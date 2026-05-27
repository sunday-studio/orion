package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"time"

	"orion/agent/internal/collector"
	"orion/agent/internal/config"
	"orion/agent/internal/logging"
	"orion/agent/internal/state"
	"orion/agent/internal/transport"
)

type Agent struct {
	userConfig    *config.UserConfig
	internalState *config.InternalState
	stateStore    *state.Store
	stateMu       sync.Mutex
	transport     *transport.Client
	retryQueue    *RetryQueue
}

func New(userConfig *config.UserConfig, stateStore *state.Store, internalState *config.InternalState) *Agent {
	return &Agent{
		userConfig:    userConfig,
		transport:     transport.NewClient(userConfig.CoreURL, internalState.Token),
		internalState: internalState,
		stateStore:    stateStore,
		retryQueue:    NewRetryQueue(100),
	}
}

func NewWithStateStore(userConfig *config.UserConfig, stateStore *state.Store) (*Agent, error) {
	internalState, err := stateStore.Get()
	if err != nil {
		return nil, err
	}
	logging.Debugf("agent state loaded: agent_id=%s registered=%t monitors=%d maintenance=%t", internalState.AgentID, internalState.IsRegistered(), len(internalState.Monitors), internalState.MaintenanceMode)
	return New(userConfig, stateStore, internalState), nil
}

func (a *Agent) Run(ctx context.Context) error {
	logging.Debugf("starting agent runtime: interval=%s monitors=%d geo_location=%t", a.userConfig.Interval, len(a.userConfig.Monitors), a.userConfig.GeoLocation)
	runtimeCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	fatalErrs := make(chan error, 1)

	// Check if agent is in maintenance mode
	if a.isInMaintenanceMode() {
		reason := "No reason provided"
		if a.internalState.MaintenanceReason != nil {
			reason = *a.internalState.MaintenanceReason
		}
		logging.Infof("Agent is in maintenance mode. Reporting workers will pause until maintenance clears. Reason: %s", reason)
	}
	if err := a.flushDurableSpool(runtimeCtx); err != nil {
		if transport.IsAuthError(err) {
			return err
		}
		logging.Warnf("Initial durable report spool flush failed: %v", err)
	}

	var workers sync.WaitGroup

	// Start system metrics worker
	workers.Add(1)
	go func() {
		defer workers.Done()
		a.startSystemMetricsWorker(runtimeCtx, fatalErrs)
	}()
	workers.Add(1)
	go func() {
		defer workers.Done()
		a.startRetryQueueWorker(runtimeCtx, fatalErrs)
	}()

	// start one worker per monitor
	for _, monitor := range a.userConfig.Monitors {
		logging.Debugf("starting worker goroutine: monitor=%s type=%s interval=%s", monitor.Name, monitor.Type, monitor.Interval)
		workers.Add(1)
		go func(monitor config.UserMonitor) {
			defer workers.Done()
			a.startMonitorWorker(runtimeCtx, monitor, fatalErrs)
		}(monitor)
	}
	var runErr error
	select {
	case <-ctx.Done():
	case runErr = <-fatalErrs:
		cancel()
	}
	workers.Wait()
	if err := a.flushDurableSpool(context.Background()); err != nil {
		logging.Warnf("Durable report spool flush during shutdown failed: %v", err)
	}
	a.retryQueue.Flush(context.Background())

	logging.Infof("Agent runtime stopped")
	return runErr
}

// RunOnce runs the agent once (collects and sends all metrics, then exits)
func (a *Agent) RunOnce(ctx context.Context) error {
	logging.Infof("Running agent once (single collection cycle)")
	logging.Debugf("run once started: monitors=%d", len(a.userConfig.Monitors))
	if err := a.flushDurableSpool(ctx); err != nil {
		if transport.IsAuthError(err) {
			return err
		}
		logging.Warnf("Durable report spool flush failed before run once: %v", err)
	}

	// Run system metrics once
	if err := a.runSystemMetrics(); err != nil {
		logging.Errorf("System metrics error: %v", err)
	}

	// Run all monitors once
	for _, monitor := range a.userConfig.Monitors {
		logging.Debugf("run once collecting monitor: name=%s type=%s", monitor.Name, monitor.Type)
		internalMonitor, err := a.stateStore.GetMonitorByName(monitor.Name)
		if err != nil {
			return err
		}
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
func (a *Agent) startSystemMetricsWorker(ctx context.Context, fatalErrs chan<- error) {
	interval, err := time.ParseDuration(a.userConfig.Interval)
	if err != nil {
		logging.Errorf("Invalid system interval: %v", err)
		return
	}

	if err := a.runSystemMetrics(); err != nil {
		logging.Errorf("System metrics error: %v", err)
		if transport.IsAuthError(err) {
			reportFatalError(fatalErrs, err)
			return
		}
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	logging.Debugf("system metrics worker interval set: interval=%s", interval)

	for {
		select {
		case <-ctx.Done():
			logging.Infof("System metrics worker stopped")
			return
		case <-ticker.C:
			if err := a.runSystemMetrics(); err != nil {
				logging.Errorf("System metrics error: %v", err)
				if transport.IsAuthError(err) {
					reportFatalError(fatalErrs, err)
					return
				}
			}
		}
	}
}

func (a *Agent) startRetryQueueWorker(ctx context.Context, fatalErrs chan<- error) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	logging.Debugf("retry queue worker started: interval=30s")

	for {
		select {
		case <-ctx.Done():
			logging.Infof("Retry queue worker stopped")
			return
		case <-ticker.C:
			if err := a.flushDurableSpool(ctx); err != nil {
				logging.Errorf("Durable report spool flush failed: %v", err)
				if transport.IsAuthError(err) {
					reportFatalError(fatalErrs, err)
					return
				}
			}
			if a.retryQueue.Len() > 0 {
				logging.Infof("Flushing retry queue (%d pending)", a.retryQueue.Len())
				if err := a.retryQueue.Flush(ctx); err != nil {
					logging.Errorf("Retry queue flush failed: %v", err)
					if transport.IsAuthError(err) {
						reportFatalError(fatalErrs, err)
						return
					}
				}
			}
		}
	}
}

// Start Monitor Worker
func (a *Agent) startMonitorWorker(ctx context.Context, monitor config.UserMonitor, fatalErrs chan<- error) {
	logging.Infof("Starting monitor worker for %s...", monitor.Name)

	internalMonitor, err := a.stateStore.GetMonitorByName(monitor.Name)
	if err != nil {
		logging.Errorf("Failed to load monitor state for %s: %v", monitor.Name, err)
		return
	}

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
		if transport.IsAuthError(err) {
			reportFatalError(fatalErrs, err)
			return
		}
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	logging.Debugf("monitor worker interval set: monitor=%s interval=%s", monitor.Name, interval)

	for {
		select {
		case <-ctx.Done():
			logging.Infof("Monitor worker stopped")
			return
		case <-ticker.C:
			if err := a.runMonitorMetrics(*internalMonitor, monitor); err != nil {
				logging.Errorf("Monitor metrics error: %v", err)
				if transport.IsAuthError(err) {
					reportFatalError(fatalErrs, err)
					return
				}
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

	metrics, err := collector.CollectWithOptions(collector.CollectOptions{IncludeLocation: a.userConfig.GeoLocation})

	if err != nil {
		return err
	}
	logging.Debugf("system metrics collected: uptime_seconds=%d cpu_percent=%.2f memory_percent=%.2f disk_percent=%.2f", metrics.UptimeSeconds, metrics.CPU.UsagePercent, metrics.Memory.UsedPercent, metrics.Disk.UsedPercent)

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
		if transport.IsAuthError(err) {
			return err
		}
		if spoolErr := a.enqueueSystemReport(report, err); spoolErr != nil {
			a.retryQueue.Push(RetryItem{Name: "system-report", Send: send})
			logging.Errorf("Failed to persist system report for retry: %v", spoolErr)
			return fmt.Errorf("%w; failed to persist retry item: %v", err, spoolErr)
		}
		return err
	}
	logging.Debugf("system report sent: agent_id=%s", a.internalState.AgentID)

	if err := a.shipServiceLogs(); err != nil {
		if transport.IsAuthError(err) {
			return err
		}
		logging.Warnf("Service log shipping failed: %v", err)
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
	}
	if result == nil {
		return err
	}
	logging.Debugf("monitor metrics collected: name=%s type=%s health=%s metrics=%d has_error=%t", monitor.Name, userMonitor.Type, result.Status, metricCount(result.Metrics), result.Error != nil)

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
		if transport.IsAuthError(err) {
			return err
		}
		if spoolErr := a.enqueueMonitorReport(monitor, report, err); spoolErr != nil {
			a.retryQueue.Push(RetryItem{Name: "monitor-report:" + monitor.Name, Send: send})
			logging.Errorf("Failed to persist monitor report for retry: monitor=%s error=%v", monitor.Name, spoolErr)
			return fmt.Errorf("%w; failed to persist retry item: %v", err, spoolErr)
		}
		return err
	}
	logging.Debugf("monitor report sent: monitor=%s monitor_id=%s", monitor.Name, monitor.ID)

	return err
}

func (a *Agent) enqueueSystemReport(report *transport.SystemReport, lastErr error) error {
	if _, err := a.stateStore.EnqueueReport(state.ReportSpoolKindSystem, a.internalState.AgentID, "", "", report, lastErr); err != nil {
		return err
	}
	logging.Debugf("system report persisted for retry")
	return nil
}

func (a *Agent) enqueueMonitorReport(monitor config.InternalStateMonitor, report *transport.MonitorReport, lastErr error) error {
	if _, err := a.stateStore.EnqueueReport(state.ReportSpoolKindMonitor, a.internalState.AgentID, monitor.ID, monitor.Name, report, lastErr); err != nil {
		return err
	}
	logging.Debugf("monitor report persisted for retry: monitor=%s monitor_id=%s", monitor.Name, monitor.ID)
	return nil
}

func (a *Agent) flushDurableSpool(ctx context.Context) error {
	items, err := a.stateStore.ListDueReports(time.Now(), 100)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return nil
	}

	logging.Infof("Flushing durable report spool (%d pending)", len(items))
	var firstErr error
	for _, item := range items {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err := a.sendSpooledReport(item); err != nil {
			if transport.IsAuthError(err) {
				return err
			}
			if markErr := a.stateStore.MarkReportFailed(item.ID, err); markErr != nil {
				return markErr
			}
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if err := a.stateStore.MarkReportSent(item.ID); err != nil {
			return err
		}
		logging.Debugf("durable report spool item sent: id=%d kind=%s", item.ID, item.Kind)
	}
	return firstErr
}

func (a *Agent) sendSpooledReport(item state.SpooledReport) error {
	switch item.Kind {
	case state.ReportSpoolKindSystem:
		var report transport.SystemReport
		if err := json.Unmarshal(item.PayloadJSON, &report); err != nil {
			return fmt.Errorf("decode spooled system report: %w", err)
		}
		return a.transport.SendReport(report, item.AgentID)
	case state.ReportSpoolKindMonitor:
		var report transport.MonitorReport
		if err := json.Unmarshal(item.PayloadJSON, &report); err != nil {
			return fmt.Errorf("decode spooled monitor report: %w", err)
		}
		return a.transport.SendMonitorReport(report, item.AgentID, item.MonitorID)
	default:
		return fmt.Errorf("unsupported spooled report kind: %s", item.Kind)
	}
}

func metricCount(metrics any) int {
	if metrics == nil {
		return 0
	}
	value := reflect.ValueOf(metrics)
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return 0
		}
		value = value.Elem()
	}
	if value.Kind() == reflect.Map {
		return value.Len()
	}
	return 0
}

func reportFatalError(fatalErrs chan<- error, err error) {
	select {
	case fatalErrs <- err:
	default:
	}
}

func (a *Agent) isInMaintenanceMode() bool {
	a.refreshInternalState()

	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	return a.internalState.MaintenanceMode
}

func (a *Agent) refreshInternalState() {
	latestState, err := a.stateStore.Get()
	if err != nil {
		logging.Warnf("Failed to refresh internal state: %v", err)
		return
	}

	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	a.internalState = latestState
	a.transport.SetAuthToken(latestState.Token)
	logging.Debugf("internal state refreshed: registered=%t agent_id=%s monitors=%d maintenance=%t", latestState.IsRegistered(), latestState.AgentID, len(latestState.Monitors), latestState.MaintenanceMode)
}
