package agent

import (
	"context"
	"time"

	"orion/agent/internal/collector"
	"orion/agent/internal/config"
	"orion/agent/internal/logging"
	"orion/agent/internal/transport"
)

type Agent struct {
	userConfig    *config.UserConfig
	internalState *config.InternalState
	transport     *transport.Client
}

func New(userConfig *config.UserConfig, internalState *config.InternalState) *Agent {
	return &Agent{
		userConfig:    userConfig,
		transport:     transport.NewClient(userConfig.CoreURL, internalState.Token),
		internalState: internalState,
	}
}

func (a *Agent) Run(ctx context.Context) error {
	// Check if agent is in maintenance mode
	if a.internalState.MaintenanceMode {
		reason := "No reason provided"
		if a.internalState.MaintenanceReason != nil {
			reason = *a.internalState.MaintenanceReason
		}
		logging.Infof("Agent is in maintenance mode. Pausing reporting. Reason: %s", reason)
		// Just wait for context cancellation
		<-ctx.Done()
		logging.Infof("Agent runtime stopped (maintenance mode)")
		return nil
	}

	// Start system metrics worker
	go a.startSystemMetricsWorker(ctx)

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
	metrics, err := collector.Collect()

	if err != nil {
		return err
	}

	report := &transport.SystemReport{
		KernelVersion: metrics.Kernel,
		UptimeSeconds: metrics.UptimeSeconds,
		Timestamp:     metrics.Timestamp,
		CPU:           &metrics.CPU,
		Memory:        &metrics.Memory,
		Disk:          &metrics.Disk,
		Location:      metrics.Location,
	}

	if err := a.transport.SendReport(*report, a.internalState.AgentID); err != nil {
		return err
	}

	return nil
}

func (a *Agent) runMonitorMetrics(monitor config.InternalStateMonitor, userMonitor config.UserMonitor) error {
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

	if err := a.transport.SendMonitorReport(*report, a.internalState.AgentID, monitor.ID); err != nil {
		return err
	}

	return nil
}
