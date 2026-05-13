package agent

import (
	"context"
	"sync"
	"time"

	"orion/agent/internal/collector"
	"orion/agent/internal/config"
	"orion/agent/internal/logging"
	"orion/agent/internal/transport"
)

type Agent struct {
	userConfig    *config.UserConfig
	internalState *config.InternalState
	statePath     string
	stateMu       sync.Mutex
	transport     *transport.Client
	retryQueue    *RetryQueue
}

func New(userConfig *config.UserConfig, internalState *config.InternalState) *Agent {
	return NewWithStatePath(userConfig, internalState, "")
}

func NewWithStatePath(userConfig *config.UserConfig, internalState *config.InternalState, statePath string) *Agent {
	return &Agent{
		userConfig:    userConfig,
		transport:     transport.NewClient(userConfig.CoreURL, internalState.Token),
		internalState: internalState,
		statePath:     statePath,
		retryQueue:    NewRetryQueue(100),
	}
}

func (a *Agent) Run(ctx context.Context) error {
	// Check if agent is in maintenance mode
	if a.isInMaintenanceMode() {
		reason := "No reason provided"
		if a.internalState.MaintenanceReason != nil {
			reason = *a.internalState.MaintenanceReason
		}
		logging.Infof("Agent is in maintenance mode. Reporting workers will pause until maintenance clears. Reason: %s", reason)
	}

	// Start system metrics worker
	go a.startSystemMetricsWorker(ctx)
	go a.startRetryQueueWorker(ctx)

	// start one worker per monitor
	for _, monitor := range a.userConfig.Monitors {
		go a.startMonitorWorker(ctx, monitor)
	}
	<-ctx.Done()

	logging.Infof("Agent runtime stopped")
	return nil
}

// RunOnce runs the agent once (collects and sends all metrics, then exits)
func (a *Agent) RunOnce(ctx context.Context) error {
	logging.Infof("Running agent once (single collection cycle)")

	// Run system metrics once
	if err := a.runSystemMetrics(); err != nil {
		logging.Errorf("System metrics error: %v", err)
	}

	// Run all monitors once
	for _, monitor := range a.userConfig.Monitors {
		internalMonitor := a.internalState.GetMonitorByName(monitor.Name)
		if internalMonitor == nil {
			logging.Warnf("Monitor not found in internal state: %s", monitor.Name)
			continue
		}

		if err := a.runMonitorMetrics(*internalMonitor, monitor); err != nil {
			logging.Errorf("Monitor metrics error for %s: %v", monitor.Name, err)
		}
	}

	logging.Infof("Single run completed")
	return nil
}

// System Metrics Worker
func (a *Agent) startSystemMetricsWorker(ctx context.Context) {
	interval, err := time.ParseDuration(a.userConfig.Interval)
	if err != nil {
		logging.Errorf("Invalid system interval: %v", err)
		return
	}

	if err := a.runSystemMetrics(); err != nil {
		logging.Errorf("System metrics error: %v", err)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logging.Infof("System metrics worker stopped")
			return
		case <-ticker.C:
			if err := a.runSystemMetrics(); err != nil {
				logging.Errorf("System metrics error: %v", err)
			}
		}
	}
}

func (a *Agent) startRetryQueueWorker(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logging.Infof("Retry queue worker stopped")
			return
		case <-ticker.C:
			if a.retryQueue.Len() > 0 {
				logging.Infof("Flushing retry queue (%d pending)", a.retryQueue.Len())
				a.retryQueue.Flush(ctx)
			}
		}
	}
}

// Start Monitor Worker
func (a *Agent) startMonitorWorker(ctx context.Context, monitor config.UserMonitor) {
	logging.Infof("Starting monitor worker for %s...", monitor.Name)

	internalMonitor := a.internalState.GetMonitorByName(monitor.Name)

	if internalMonitor == nil {
		logging.Errorf("Monitor not found in internal state: %s", monitor.Name)
		return
	}

	interval, err := time.ParseDuration(monitor.Interval)

	if err != nil {
		logging.Errorf("Invalid monitor interval: %v", err)
		return
	}

	if err := a.runMonitorMetrics(*internalMonitor, monitor); err != nil {
		logging.Errorf("Monitor metrics error: %v", err)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logging.Infof("Monitor worker stopped")
			return
		case <-ticker.C:
			if err := a.runMonitorMetrics(*internalMonitor, monitor); err != nil {
				logging.Errorf("Monitor metrics error: %v", err)
			}

		}
	}
}

// Run System Metrics
func (a *Agent) runSystemMetrics() error {
	if a.isInMaintenanceMode() {
		logging.Infof("Skipping system report while agent is in maintenance mode")
		return nil
	}

	metrics, err := collector.Collect()

	if err != nil {
		return err
	}

	report := &transport.SystemReport{
		KernelVersion: metrics.Kernel,
		AgentVersion:  Version,
		ConfigSummary: a.configSummary(),
		UptimeSeconds: metrics.UptimeSeconds,
		Timestamp:     metrics.Timestamp,
		CPU:           &metrics.CPU,
		Memory:        &metrics.Memory,
		Disk:          &metrics.Disk,
		Location:      metrics.Location,
	}

	send := func(context.Context) error {
		return a.transport.SendReport(*report, a.internalState.AgentID)
	}

	if err := send(context.Background()); err != nil {
		a.retryQueue.Push(RetryItem{Name: "system-report", Send: send})
		return err
	}

	return nil
}

func (a *Agent) configSummary() map[string]interface{} {
	monitorTypes := make(map[string]int)
	for _, monitor := range a.userConfig.Monitors {
		monitorTypes[string(monitor.Type)]++
	}

	return map[string]interface{}{
		"reporting_interval": a.userConfig.Interval,
		"monitor_count":      len(a.userConfig.Monitors),
		"monitor_types":      monitorTypes,
	}
}

func (a *Agent) runMonitorMetrics(monitor config.InternalStateMonitor, userMonitor config.UserMonitor) error {
	if a.isInMaintenanceMode() {
		logging.Infof("Skipping monitor report for %s while agent is in maintenance mode", monitor.Name)
		return nil
	}

	result, err := collector.CollectMonitorReport(monitor, userMonitor)
	if err != nil {
		logging.Errorf("Monitor check error: %v", err)
		return err
	}

	logging.Infof("Monitor result -> %v", monitor.Name)
	report := &transport.MonitorReport{
		Timestamp: result.Timestamp.Format(time.RFC3339),
		Health:    result.Status,
		Metrics:   result.Metrics,
		Error:     result.Error,
	}

	send := func(context.Context) error {
		return a.transport.SendMonitorReport(*report, a.internalState.AgentID, monitor.ID)
	}

	if err := send(context.Background()); err != nil {
		a.retryQueue.Push(RetryItem{Name: "monitor-report:" + monitor.Name, Send: send})
		return err
	}

	return nil
}

func (a *Agent) isInMaintenanceMode() bool {
	a.refreshInternalState()

	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	return a.internalState.MaintenanceMode
}

func (a *Agent) refreshInternalState() {
	if a.statePath == "" {
		return
	}

	latestState, err := config.LoadInternalState(a.statePath)
	if err != nil {
		logging.Warnf("Failed to refresh internal state: %v", err)
		return
	}

	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	a.internalState = latestState
	a.transport.SetAuthToken(latestState.Token)
}
