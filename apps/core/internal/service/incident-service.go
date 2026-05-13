package service

import (
	"errors"
	"fmt"
	"orion/core/internal/db"
	"orion/core/internal/logging"
	"orion/core/internal/utils"
	"time"

	"gorm.io/gorm"
)

type IncidentService struct {
	db     *gorm.DB
	logger *logging.Logger
}

func NewIncidentService(database *gorm.DB, logger *logging.Logger) *IncidentService {
	return &IncidentService{
		db:     database,
		logger: logger,
	}
}

func (s *IncidentService) ReconcileMonitorReport(monitorID string, monitorReportID string, health string) error {
	var monitor db.Monitor
	if err := s.db.Where("id = ?", monitorID).First(&monitor).Error; err != nil {
		return err
	}

	var agent db.Agent
	if err := s.db.Where("id = ?", monitor.AgentID).First(&agent).Error; err != nil {
		return err
	}

	if health == "up" {
		return s.resolveActiveIncident(monitor, monitorReportID)
	}

	if agent.MaintenanceMode {
		s.logger.Info("Incident suppressed during maintenance", "monitor_id", monitorID, "agent_id", agent.ID)
		return nil
	}

	if health == "down" || health == "degraded" || health == "stale" {
		return s.openOrUpdateIncident(agent, monitor, monitorReportID, health)
	}

	return nil
}

func (s *IncidentService) openOrUpdateIncident(agent db.Agent, monitor db.Monitor, monitorReportID string, health string) error {
	now := time.Now().UTC()
	message := fmt.Sprintf("Monitor %s reported %s", monitor.Name, health)

	var incident db.Incident
	err := s.db.Where("monitor_id = ? AND status IN ?", monitor.ID, []string{"open", "acknowledged"}).
		Order("opened_at DESC").
		First(&incident).Error
	if err == nil {
		updates := map[string]interface{}{
			"severity":      incidentSeverity(health),
			"last_event_at": now,
			"latest_event":  message,
		}
		if err := s.db.Model(&incident).Updates(updates).Error; err != nil {
			return err
		}
		return s.createIncidentEvent(incident.ID, "monitor_failed", message, monitorReportID)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	incident = db.Incident{
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
	if err := s.db.Create(&incident).Error; err != nil {
		return err
	}

	return s.createIncidentEvent(incident.ID, "incident_opened", message, monitorReportID)
}

func (s *IncidentService) resolveActiveIncident(monitor db.Monitor, monitorReportID string) error {
	now := time.Now().UTC()
	message := fmt.Sprintf("Monitor %s recovered", monitor.Name)

	var incident db.Incident
	err := s.db.Where("monitor_id = ? AND status IN ?", monitor.ID, []string{"open", "acknowledged"}).
		Order("opened_at DESC").
		First(&incident).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}

	updates := map[string]interface{}{
		"status":        "resolved",
		"resolved_at":   &now,
		"last_event_at": now,
		"latest_event":  message,
	}
	if err := s.db.Model(&incident).Updates(updates).Error; err != nil {
		return err
	}

	return s.createIncidentEvent(incident.ID, "incident_resolved", message, monitorReportID)
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
