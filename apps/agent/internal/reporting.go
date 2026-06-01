package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"orion/agent/internal/collector"
	"orion/agent/internal/config"
	"orion/agent/internal/logging"
	"orion/agent/internal/state"
	"orion/agent/internal/transport"
)

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
