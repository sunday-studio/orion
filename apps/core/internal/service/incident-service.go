package service

import (
	"encoding/json"
	"fmt"
	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/logging"
	"orion/core/internal/utils"
	"time"

	"gorm.io/gorm"
)

const (
	activeIncidentLookupSlowThreshold = 50 * time.Millisecond
	incidentReconcileSlowThreshold    = 100 * time.Millisecond
)

type IncidentService struct {
	db     *gorm.DB
	logger *logging.Logger
	cfg    *config.Config
}

func NewIncidentService(database *gorm.DB, logger *logging.Logger, cfg *config.Config) *IncidentService {
	return &IncidentService{
		db:     database,
		logger: logger,
		cfg:    cfg,
	}
}

func (s *IncidentService) ReconcileMonitorReport(monitorID string, monitorReportID string, payload MonitorReportPayload) error {
	startedAt := time.Now()
	defer func() {
		duration := time.Since(startedAt)
		if duration > incidentReconcileSlowThreshold {
			s.logger.Warn("Slow incident reconciliation", "monitor_id", monitorID, "monitor_report_id", monitorReportID, "duration_ms", duration.Milliseconds())
		}
	}()

	var monitor db.Monitor
	if err := s.db.Where("id = ?", monitorID).First(&monitor).Error; err != nil {
		return err
	}

	reportedHealth := payload.Health
	tlsExpiring := s.isTLSExpiring(payload.Metrics)
	nextIncidentState := incidentStateForReport(reportedHealth, tlsExpiring)
	if nextIncidentState == monitor.IncidentState && monitor.ActiveIncidentID == "" && nextIncidentState == "up" {
		return nil
	}

	var agent db.Agent
	if err := s.db.Where("id = ?", monitor.AgentID).First(&agent).Error; err != nil {
		return err
	}

	if reportedHealth == "up" && !tlsExpiring {
		return s.resolveActiveIncident(monitor, monitorReportID, nextIncidentState)
	}

	if agent.MaintenanceMode {
		s.logger.Info("Incident suppressed during maintenance", "monitor_id", monitorID, "agent_id", agent.ID)
		return s.updateMonitorIncidentState(monitor.ID, monitor.ActiveIncidentID, nextIncidentState)
	}

	if tlsExpiring {
		return s.openOrUpdateIncident(agent, monitor, monitorReportID, "degraded", nextIncidentState)
	}

	if reportedHealth == "down" || reportedHealth == "degraded" || reportedHealth == "stale" {
		return s.openOrUpdateIncident(agent, monitor, monitorReportID, reportedHealth, nextIncidentState)
	}

	return nil
}

func (s *IncidentService) ReconcileStaleMonitors(agentID string) error {
	var agent db.Agent
	if err := s.db.Where("id = ?", agentID).First(&agent).Error; err != nil {
		return err
	}
	if agent.MaintenanceMode {
		return nil
	}

	healthService := NewHealthService(s.db, s.logger)
	staleMonitors, err := healthService.DetectStaleMonitors(DefaultHealthConfig())
	if err != nil {
		return err
	}

	for _, monitor := range staleMonitors {
		if monitor.AgentID != agentID {
			continue
		}
		if err := s.openOrUpdateIncident(agent, monitor, "", "stale", "stale"); err != nil {
			return err
		}
	}

	return nil
}

func (s *IncidentService) openOrUpdateIncident(agent db.Agent, monitor db.Monitor, monitorReportID string, health string, incidentState string) error {
	now := time.Now().UTC()
	message := fmt.Sprintf("Monitor %s reported %s", monitor.Name, health)

	if monitor.ActiveIncidentID != "" {
		if updated, err := s.updateActiveIncidentByID(monitor.ActiveIncidentID, monitor.ID, monitorReportID, incidentSeverity(health), message, incidentState); err != nil {
			return err
		} else if updated {
			return nil
		}
	}

	incident, found, err := s.findActiveIncident(monitor.ID)
	if err != nil {
		return err
	}
	if found {
		updates := map[string]interface{}{
			"severity":      incidentSeverity(health),
			"last_event_at": now,
			"latest_event":  message,
		}
		if err := s.db.Model(incident).Updates(updates).Error; err != nil {
			return err
		}
		if err := s.updateMonitorIncidentState(monitor.ID, incident.ID, incidentState); err != nil {
			return err
		}
		return s.createIncidentEvent(incident.ID, "monitor_failed", message, monitorReportID)
	}

	newIncident := db.Incident{
		ID:                 utils.GenerateID("incident"),
		Status:             "open",
		Severity:           incidentSeverity(health),
		Title:              fmt.Sprintf("%s is %s", monitor.Name, health),
		AgentID:            agent.ID,
		MonitorID:          monitor.ID,
		OpenedAt:           now,
		LastEventAt:        now,
		LatestEvent:        message,
		NotificationStatus: "pending",
	}
	if err := s.db.Create(&newIncident).Error; err != nil {
		return err
	}

	if err := s.updateMonitorIncidentState(monitor.ID, newIncident.ID, incidentState); err != nil {
		return err
	}

	if err := s.createIncidentEvent(newIncident.ID, "incident_opened", message, monitorReportID); err != nil {
		return err
	}
	return NewAlertService(s.db, s.logger, s.cfg).QueueIncidentNotifications(newIncident.ID, "incident_opened")
}

func (s *IncidentService) resolveActiveIncident(monitor db.Monitor, monitorReportID string, incidentState string) error {
	now := time.Now().UTC()
	message := fmt.Sprintf("Monitor %s recovered", monitor.Name)

	if monitor.ActiveIncidentID != "" {
		if resolved, err := s.resolveActiveIncidentByID(monitor.ActiveIncidentID, monitor.ID, monitorReportID, message, incidentState); err != nil {
			return err
		} else if resolved {
			return nil
		}
	}

	incident, found, err := s.findActiveIncident(monitor.ID)
	if err != nil {
		return err
	}
	if !found {
		return s.updateMonitorIncidentState(monitor.ID, "", incidentState)
	}

	updates := map[string]interface{}{
		"status":        "resolved",
		"resolved_at":   &now,
		"last_event_at": now,
		"latest_event":  message,
	}
	if err := s.db.Model(incident).Updates(updates).Error; err != nil {
		return err
	}

	if err := s.updateMonitorIncidentState(monitor.ID, "", incidentState); err != nil {
		return err
	}

	if err := s.createIncidentEvent(incident.ID, "incident_resolved", message, monitorReportID); err != nil {
		return err
	}
	if s.cfg != nil && !s.cfg.AlertRecoveryNotifications {
		return nil
	}
	return NewAlertService(s.db, s.logger, s.cfg).QueueIncidentNotifications(incident.ID, "incident_resolved")
}

func (s *IncidentService) findActiveIncident(monitorID string) (*db.Incident, bool, error) {
	startedAt := time.Now()
	var incident db.Incident
	result := s.db.Where("monitor_id = ? AND status IN ?", monitorID, []string{"open", "acknowledged"}).
		Order("opened_at DESC").
		Limit(1).
		Find(&incident)
	duration := time.Since(startedAt)
	if duration > activeIncidentLookupSlowThreshold {
		s.logger.Warn("Slow active incident lookup", "monitor_id", monitorID, "duration_ms", duration.Milliseconds())
	}
	if result.Error != nil {
		return nil, false, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, false, nil
	}
	return &incident, true, nil
}

func (s *IncidentService) updateActiveIncidentByID(incidentID string, monitorID string, monitorReportID string, severity string, message string, incidentState string) (bool, error) {
	now := time.Now().UTC()
	updates := map[string]interface{}{
		"severity":      severity,
		"last_event_at": now,
		"latest_event":  message,
	}
	result := s.db.Model(&db.Incident{}).
		Where("id = ? AND monitor_id = ? AND status IN ?", incidentID, monitorID, []string{"open", "acknowledged"}).
		Updates(updates)
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected == 0 {
		if err := s.updateMonitorIncidentState(monitorID, "", incidentState); err != nil {
			return false, err
		}
		return false, nil
	}
	if err := s.updateMonitorIncidentState(monitorID, incidentID, incidentState); err != nil {
		return true, err
	}
	return true, s.createIncidentEvent(incidentID, "monitor_failed", message, monitorReportID)
}

func (s *IncidentService) resolveActiveIncidentByID(incidentID string, monitorID string, monitorReportID string, message string, incidentState string) (bool, error) {
	now := time.Now().UTC()
	updates := map[string]interface{}{
		"status":        "resolved",
		"resolved_at":   &now,
		"last_event_at": now,
		"latest_event":  message,
	}
	result := s.db.Model(&db.Incident{}).
		Where("id = ? AND monitor_id = ? AND status IN ?", incidentID, monitorID, []string{"open", "acknowledged"}).
		Updates(updates)
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected == 0 {
		if err := s.updateMonitorIncidentState(monitorID, "", incidentState); err != nil {
			return false, err
		}
		return false, nil
	}
	if err := s.updateMonitorIncidentState(monitorID, "", incidentState); err != nil {
		return true, err
	}
	if err := s.createIncidentEvent(incidentID, "incident_resolved", message, monitorReportID); err != nil {
		return true, err
	}
	if s.cfg != nil && !s.cfg.AlertRecoveryNotifications {
		return true, nil
	}
	return true, NewAlertService(s.db, s.logger, s.cfg).QueueIncidentNotifications(incidentID, "incident_resolved")
}

func (s *IncidentService) updateMonitorIncidentState(monitorID string, activeIncidentID string, incidentState string) error {
	return s.db.Model(&db.Monitor{}).Where("id = ?", monitorID).Updates(map[string]interface{}{
		"active_incident_id": activeIncidentID,
		"incident_state":     incidentState,
	}).Error
}

func (s *IncidentService) createIncidentEvent(incidentID string, eventType string, message string, monitorReportID string) error {
	event := db.IncidentEvent{
		ID:              utils.GenerateID("incident_event"),
		IncidentID:      incidentID,
		Type:            eventType,
		Message:         message,
		MonitorReportID: monitorReportID,
	}
	return s.db.Create(&event).Error
}

func incidentSeverity(health string) string {
	switch health {
	case "down", "stale":
		return "high"
	case "degraded":
		return "medium"
	default:
		return "low"
	}
}

func incidentStateForReport(reportedHealth string, tlsExpiring bool) string {
	if tlsExpiring {
		return "degraded"
	}
	switch reportedHealth {
	case "down", "degraded", "stale":
		return reportedHealth
	case "up":
		return "up"
	default:
		return "unknown"
	}
}

func (s *IncidentService) isTLSExpiring(metrics interface{}) bool {
	threshold := 14
	if s.cfg != nil {
		threshold = s.cfg.AlertTLSExpiryDays
	}
	if threshold <= 0 {
		return false
	}

	metricsMap, ok := metrics.(map[string]interface{})
	if !ok {
		return false
	}

	rawDays, exists := metricsMap["tls_days_remaining"]
	if !exists {
		return false
	}

	days, ok := numericValue(rawDays)
	return ok && days <= float64(threshold)
}

func numericValue(value interface{}) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	default:
		return 0, false
	}
}
