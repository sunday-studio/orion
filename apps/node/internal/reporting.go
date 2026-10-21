package node

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"orion/node/internal/collector"
	"orion/node/internal/config"
	"orion/node/internal/logging"
	"orion/node/internal/state"
	"orion/node/internal/transport"
	"orion/node/internal/utils"
)

func (a *Node) runSystemMetrics(ctx context.Context) error {
	if a.isInMaintenanceMode() {
		logging.Infof("Skipping system report while node is in maintenance mode")
		return nil
	}

	metrics, err := collector.CollectWithOptions(collector.CollectOptions{IncludeLocation: a.userConfig.GeoLocation})
	if err != nil {
		return err
	}
	logging.Debugf("system metrics collected: uptime_seconds=%d cpu_percent=%.2f memory_percent=%.2f disk_percent=%.2f", metrics.UptimeSeconds, metrics.CPU.UsagePercent, metrics.Memory.UsedPercent, metrics.Disk.UsedPercent)

	report := &transport.SystemReport{
		ReportID:      utils.NewReportID(),
		KernelVersion: metrics.Kernel,
		NodeVersion:   Version,
		ConfigSummary: a.configSummary(),
		UptimeSeconds: metrics.UptimeSeconds,
		Timestamp:     metrics.Timestamp,
		CPU:           &metrics.CPU,
		Memory:        &metrics.Memory,
		Disk:          &metrics.Disk,
		Location:      metrics.Location,
	}

	if a.sampler != nil {
		appReport := a.sampler.Sample()
		report.AppResources = &appReport
	}

	nodeID := a.nodeID()
	send := func(sendCtx context.Context) error {
		return a.transport.WithContext(sendCtx).SendReport(*report, nodeID)
	}

	sendStart := time.Now()
	if err := send(ctx); err != nil {
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
	if a.hooks.SystemReport != nil {
		a.hooks.SystemReport(time.Now(), report.AppResources, time.Since(sendStart))
	}
	logging.Debugf("system report sent: node_id=%s", nodeID)

	if err := a.shipServiceLogs(ctx); err != nil {
		if transport.IsAuthError(err) {
			return err
		}
		logging.Warnf("Service log shipping failed: %v", err)
	}

	return nil
}

func (a *Node) configSummary() map[string]interface{} {
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

func (a *Node) runMonitorMetrics(ctx context.Context, monitor config.InternalStateMonitor, userMonitor config.UserMonitor) error {
	if a.isInMaintenanceMode() {
		logging.Infof("Skipping monitor report for %s while node is in maintenance mode", monitor.Name)
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

	if a.hooks.MonitorResult != nil {
		detail, value := resultDetail(result)
		a.hooks.MonitorResult(monitor.Name, string(userMonitor.Type), monitorTarget(userMonitor), result.Status, detail, value, result.Timestamp)
	}
	logging.Infof("Monitor result -> %v", monitor.Name)
	report := &transport.MonitorReport{
		ReportID:  utils.NewReportID(),
		Timestamp: result.Timestamp.Format(time.RFC3339),
		Health:    result.Status,
		Metrics:   result.Metrics,
		Error:     result.Error,
	}

	nodeID := a.nodeID()
	send := func(sendCtx context.Context) error {
		return a.transport.WithContext(sendCtx).SendMonitorReport(*report, nodeID, monitor.ID)
	}

	if err := send(ctx); err != nil {
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

	return nil
}

func (a *Node) enqueueSystemReport(report *transport.SystemReport, lastErr error) error {
	if _, err := a.stateStore.EnqueueReport(state.ReportSpoolKindSystem, a.nodeID(), "", "", report, lastErr); err != nil {
		return err
	}
	logging.Debugf("system report persisted for retry")
	return nil
}

func (a *Node) enqueueMonitorReport(monitor config.InternalStateMonitor, report *transport.MonitorReport, lastErr error) error {
	if _, err := a.stateStore.EnqueueReport(state.ReportSpoolKindMonitor, a.nodeID(), monitor.ID, monitor.Name, report, lastErr); err != nil {
		return err
	}
	logging.Debugf("monitor report persisted for retry: monitor=%s monitor_id=%s", monitor.Name, monitor.ID)
	return nil
}

func (a *Node) flushDurableSpool(ctx context.Context) error {
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

		if err := a.sendSpooledReport(ctx, item); err != nil {
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

func (a *Node) sendSpooledReport(ctx context.Context, item state.SpooledReport) error {
	transportClient := a.transport.WithContext(ctx)
	switch item.Kind {
	case state.ReportSpoolKindSystem:
		var report transport.SystemReport
		if err := json.Unmarshal(item.PayloadJSON, &report); err != nil {
			return fmt.Errorf("decode spooled system report: %w", err)
		}
		return transportClient.SendReport(report, item.NodeID)
	case state.ReportSpoolKindMonitor:
		var report transport.MonitorReport
		if err := json.Unmarshal(item.PayloadJSON, &report); err != nil {
			return fmt.Errorf("decode spooled monitor report: %w", err)
		}
		return transportClient.SendMonitorReport(report, item.NodeID, item.MonitorID)
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

// resultDetail extracts a short human reason and reading from a check result.
func resultDetail(r *collector.MonitorResult) (detail, value string) {
	var m map[string]interface{}
	switch v := r.Metrics.(type) {
	case map[string]interface{}:
		m = v
	case *map[string]interface{}:
		if v != nil {
			m = *v
		}
	}
	if s, ok := m["failure_reason"].(string); ok {
		detail = s
	}
	if detail == "" && r.Error != nil {
		detail = r.Error.Message
	}
	if p, ok := m["used_percent"].(float64); ok {
		value = fmt.Sprintf("%.0f%%", p)
	}
	return detail, value
}

// monitorTarget is the short thing a monitor watches ("nginx", "port 6379", "disk /var").
func monitorTarget(m config.UserMonitor) string {
	switch {
	case m.Systemd != nil:
		return m.Systemd.Name
	case m.Docker != nil:
		return m.Docker.Name
	case m.PM2 != nil:
		return m.PM2.AppName
	case m.TCP != nil:
		return fmt.Sprintf("port %d", m.TCP.Port)
	case m.DiskUsage != nil:
		return "disk " + m.DiskUsage.Path
	case m.ProcessRunning != nil:
		return m.ProcessRunning.Name
	case m.Website != nil:
		return m.Website.URL
	case m.HTTP != nil:
		return m.HTTP.URL
	}
	return ""
}
