package worker

import (
	"context"
	"encoding/json"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"time"
)

func (a *App) reconcileMissedHeartbeats(ctx context.Context, now time.Time) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}

	var configs []db.CoreMonitorConfig
	if err := a.db.
		Preload("Monitor").
		Joins("JOIN monitors ON monitors.id = core_monitor_configs.monitor_id").
		Where("core_monitor_configs.kind = ?", "heartbeat").
		Where("core_monitor_configs.paused = ?", false).
		Where("monitors.lifecycle = ?", "active").
		Find(&configs).Error; err != nil {
		return err
	}

	for _, monitorConfig := range configs {
		if err := ctx.Err(); err != nil {
			return err
		}
		if !heartbeatSignalIsMissed(monitorConfig, now) {
			continue
		}
		if heartbeatFailureIsCurrent(monitorConfig) {
			continue
		}
		if err := a.storeMissedHeartbeatReport(monitorConfig, now); err != nil {
			return err
		}
	}
	return nil
}

// heartbeatReferenceTime is when the countdown to a missed heartbeat starts: the
// last signal, or for a heartbeat that has never pinged, when it was created.
func heartbeatReferenceTime(config db.CoreMonitorConfig) time.Time {
	if config.LastSignalAt != nil {
		return config.LastSignalAt.UTC()
	}
	return config.CreatedAt.UTC()
}

func heartbeatSignalIsMissed(config db.CoreMonitorConfig, now time.Time) bool {
	reference := heartbeatReferenceTime(config)
	if reference.IsZero() {
		return false
	}
	interval := time.Duration(config.IntervalSeconds) * time.Second
	if interval <= 0 {
		interval = defaultHeartbeatInterval
	}
	grace := time.Duration(heartbeatGraceSeconds(config.ConfigJSON)) * time.Second
	return now.After(reference.Add(interval + grace))
}

func heartbeatFailureIsCurrent(config db.CoreMonitorConfig) bool {
	if config.LastFailureAt == nil {
		return false
	}
	if config.LastSignalAt == nil {
		// Never pinged: one missed-heartbeat report per outage, not one per tick.
		return true
	}
	return !config.LastFailureAt.UTC().Before(config.LastSignalAt.UTC())
}

func heartbeatGraceSeconds(configJSON string) int {
	var cfg struct {
		GraceSeconds int `json:"grace_seconds"`
	}
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil || cfg.GraceSeconds < 0 {
		return 0
	}
	return cfg.GraceSeconds
}

func (a *App) storeMissedHeartbeatReport(monitorConfig db.CoreMonitorConfig, now time.Time) error {
	intervalSeconds := monitorConfig.IntervalSeconds
	if intervalSeconds <= 0 {
		intervalSeconds = int(defaultHeartbeatInterval.Seconds())
	}
	graceSeconds := heartbeatGraceSeconds(monitorConfig.ConfigJSON)
	reference := heartbeatReferenceTime(monitorConfig)
	missedAfter := reference.Add(time.Duration(intervalSeconds+graceSeconds) * time.Second)
	lastSignal := "never"
	if monitorConfig.LastSignalAt != nil {
		lastSignal = monitorConfig.LastSignalAt.UTC().Format(time.RFC3339)
	}
	payload := service.MonitorReportPayload{
		Timestamp: now.Format(time.RFC3339),
		Health:    "stale",
		Metrics: map[string]any{
			"runner":           "core",
			"type":             "heartbeat",
			"failure_stage":    "missed_signal",
			"last_signal_at":   lastSignal,
			"missed_after":     missedAfter.Format(time.RFC3339),
			"interval_seconds": intervalSeconds,
			"grace_seconds":    graceSeconds,
		},
	}
	if _, err := a.reports.StoreMonitorReport(monitorConfig.MonitorID, payload); err != nil {
		return err
	}
	return a.db.Model(&db.CoreMonitorConfig{}).
		Where("monitor_id = ?", monitorConfig.MonitorID).
		Updates(map[string]any{
			"last_failure_at": now,
			"updated_at":      now,
		}).Error
}
