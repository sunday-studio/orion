package service

import (
	"errors"
	"fmt"
	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/logstore"
	"orion/core/internal/utils"
	"time"

	"gorm.io/gorm"
)

const (
	LogAlertModeThreshold = "threshold"
	LogAlertModeAbsence   = "absence"

	logAlertHeartbeat = 10 * time.Minute // re-report an unchanged state so the monitor never looks silent
	logAlertMonitorID = "monitor-log-alert-"
)

// LogAlertService turns log conditions into incidents. Each rule owns a Core monitor
// ("Log alert: <name>"); a matching burst reports it down and the normal incident,
// grouping, outbox and delivery pipeline takes over, so a burst pages once and the
// recovery resolves it. It runs in the Worker and reads logs.db.
type LogAlertService struct {
	db        *gorm.DB
	logger    *utils.Logger
	logs      *logstore.Store
	reports   *ReportService
	incidents *IncidentService
}

func NewLogAlertService(database *gorm.DB, logger *utils.Logger, cfg *config.Config, logs *logstore.Store) *LogAlertService {
	return &LogAlertService{db: database, logger: logger, logs: logs, reports: NewReportService(database, logger, cfg), incidents: NewIncidentService(database, logger, cfg)}
}

// EvaluateAll evaluates every enabled rule once and returns how many changed state.
func (s *LogAlertService) EvaluateAll(now time.Time) (int, error) {
	var rules []db.LogAlertRule
	if err := s.db.Where("enabled = ?", true).Find(&rules).Error; err != nil {
		return 0, err
	}
	changed := 0
	var firstErr error
	for i := range rules {
		didChange, err := s.Evaluate(&rules[i], now)
		if err != nil {
			s.logger.Error("Log alert evaluation failed", "rule_id", rules[i].ID, "error", err)
			firstErr = errors.Join(firstErr, err)
			continue
		}
		if didChange {
			changed++
		}
	}
	return changed, firstErr
}

// Evaluate checks one rule and reports its monitor up or down. It returns true when the state changed.
func (s *LogAlertService) Evaluate(rule *db.LogAlertRule, now time.Time) (bool, error) {
	// EvaluateAll works from a list loaded earlier; a rule disabled or deleted since then
	// must not report again (that would revive the monitor Retire just parked).
	var live db.LogAlertRule
	if err := s.db.Select("id", "enabled").Where("id = ?", rule.ID).First(&live).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	if !live.Enabled {
		return false, nil
	}
	window := time.Duration(rule.WindowSeconds) * time.Second
	count, err := s.logs.Count(logstore.Query{AppIDs: []string{rule.AppID}, Severity: rule.Severity, Event: rule.Event, Text: rule.Text, From: now.Add(-window), To: now})
	if err != nil {
		return false, err
	}
	firing := false
	switch rule.Mode {
	case LogAlertModeAbsence:
		// A new rule gets one full window before silence counts as an outage.
		firing = count == 0 && !rule.CreatedAt.After(now.Add(-window))
	default:
		firing = count > int64(rule.Threshold)
	}

	wasFiring := rule.State == "firing"
	dueHeartbeat := rule.LastReportedAt == nil || now.Sub(*rule.LastReportedAt) >= logAlertHeartbeat
	if firing != wasFiring || dueHeartbeat {
		if err := s.report(rule, firing, count, now); err != nil {
			return false, err
		}
		rule.LastReportedAt = &now
	}
	rule.State = map[bool]string{true: "firing", false: "ok"}[firing]
	rule.LastCount = int(count)
	rule.LastEvaluatedAt = &now
	if err := s.db.Model(&db.LogAlertRule{}).Where("id = ?", rule.ID).Updates(map[string]any{
		"state": rule.State, "last_count": rule.LastCount, "last_evaluated_at": now, "last_reported_at": rule.LastReportedAt, "monitor_id": rule.MonitorID, "updated_at": time.Now().UTC(),
	}).Error; err != nil {
		return false, err
	}
	return firing != wasFiring, nil
}

func (s *LogAlertService) report(rule *db.LogAlertRule, firing bool, count int64, now time.Time) error {
	if err := s.ensureMonitor(rule); err != nil {
		return err
	}
	health := "up"
	if firing {
		health = "down"
	}
	_, err := s.reports.StoreMonitorReport(rule.MonitorID, MonitorReportPayload{
		Timestamp: now.Format(time.RFC3339),
		Health:    health,
		Metrics: map[string]any{
			"runner": "core", "type": "log-alert", "rule_id": rule.ID, "app_id": rule.AppID, "mode": rule.Mode,
			"matching_entries": count, "threshold": rule.Threshold, "window_seconds": rule.WindowSeconds,
			"summary": logAlertSummary(rule, count),
		},
	})
	return err
}

func logAlertSummary(rule *db.LogAlertRule, count int64) string {
	if rule.Mode == LogAlertModeAbsence {
		return fmt.Sprintf("%d matching log entries in the last %ds (expected at least one)", count, rule.WindowSeconds)
	}
	return fmt.Sprintf("%d matching log entries in the last %ds (threshold %d)", count, rule.WindowSeconds, rule.Threshold)
}

// ensureMonitor creates the rule's Core monitor on first use and keeps its name in sync.
func (s *LogAlertService) ensureMonitor(rule *db.LogAlertRule) error {
	id := logAlertMonitorID + rule.ID
	name := "Log alert: " + rule.Name
	var monitor db.Monitor
	err := s.db.Where("id = ?", id).First(&monitor).Error
	switch {
	case err == nil:
		updates := map[string]any{}
		if monitor.Name != name {
			updates["name"] = name
		}
		if monitor.Lifecycle != "active" {
			updates["lifecycle"] = "active"
		}
		if len(updates) > 0 {
			if err := s.db.Model(&db.Monitor{}).Where("id = ?", id).Updates(updates).Error; err != nil {
				return err
			}
		}
		rule.MonitorID = id
		return nil
	case !errors.Is(err, gorm.ErrRecordNotFound):
		return err
	}
	owner, err := s.incidents.ensureCoreOwnerNode("")
	if err != nil {
		return err
	}
	description := fmt.Sprintf("Raised by Core when logs of app %s match rule %q.", rule.AppID, rule.Name)
	rule.MonitorID = id
	return s.db.Create(&db.Monitor{
		ID: id, NodeID: owner.ID, Name: name, Description: &description, Type: "log-alert", Lifecycle: "active",
		Health: "unknown", ComputedHealth: "unknown", IncidentState: "unknown",
		ReportingIntervalSeconds: int(logAlertHeartbeat.Seconds()),
	}).Error
}

// Retire resolves any open incident for the rule and parks its monitor. Call it when a
// rule is disabled or deleted so a firing alert does not stay open forever.
func (s *LogAlertService) Retire(rule *db.LogAlertRule, now time.Time) error {
	if rule.MonitorID == "" {
		return nil
	}
	var monitor db.Monitor
	if err := s.db.Where("id = ?", rule.MonitorID).First(&monitor).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	if monitor.ActiveIncidentID != "" {
		if err := s.incidents.inTx(func(tx *IncidentService) error { return tx.resolveActiveIncident(monitor, "", "up") }); err != nil {
			return err
		}
	}
	return s.db.Model(&db.Monitor{}).Where("id = ?", monitor.ID).Updates(map[string]any{"lifecycle": "disabled", "health": "up", "computed_health": "up", "updated_at": now}).Error
}
