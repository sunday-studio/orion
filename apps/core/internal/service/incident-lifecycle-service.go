package service

import (
	"errors"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"strings"
	"time"

	"gorm.io/gorm"
)

func (s *IncidentService) AcknowledgeIncident(incidentID string, metadata IncidentLifecycleActionMetadata) (db.Incident, error) {
	var incident db.Incident
	if err := s.db.Where("id = ?", incidentID).First(&incident).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return db.Incident{}, ErrIncidentNotFound
		}
		return db.Incident{}, err
	}
	if incident.Status == "resolved" {
		return db.Incident{}, ErrIncidentAlreadyResolved
	}
	if incident.Status == "acknowledged" {
		return incident, nil
	}

	now := time.Now().UTC()
	message := "Incident manually acknowledged"
	if err := s.db.Model(&incident).Updates(map[string]any{
		"status":        "acknowledged",
		"last_event_at": now,
		"latest_event":  message,
	}).Error; err != nil {
		return db.Incident{}, err
	}
	if err := s.createIncidentEvent(incident.ID, "incident_acknowledged", message, "", metadata); err != nil {
		return db.Incident{}, err
	}
	if err := s.db.Where("id = ?", incident.ID).First(&incident).Error; err != nil {
		return db.Incident{}, err
	}
	return incident, nil
}

func (s *IncidentService) ResolveIncident(incidentID string, metadata IncidentLifecycleActionMetadata) (db.Incident, error) {
	var incident db.Incident
	if err := s.db.Where("id = ?", incidentID).First(&incident).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return db.Incident{}, ErrIncidentNotFound
		}
		return db.Incident{}, err
	}
	if incident.Status == "resolved" {
		return incident, nil
	}

	message := "Incident manually resolved"
	if err := s.resolveIncidentRecord(&incident, message, "", "up", "manual", true, metadata); err != nil {
		return db.Incident{}, err
	}
	if err := s.db.Where("id = ?", incident.ID).First(&incident).Error; err != nil {
		return db.Incident{}, err
	}
	return incident, nil
}

func (s *IncidentService) CoverIncident(incidentID string, coveredUntil *time.Time, note string, metadata IncidentLifecycleActionMetadata) (db.Incident, error) {
	var incident db.Incident
	if err := s.db.Where("id = ?", incidentID).First(&incident).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return db.Incident{}, ErrIncidentNotFound
		}
		return db.Incident{}, err
	}
	if incident.Status == "resolved" {
		return db.Incident{}, ErrIncidentAlreadyResolved
	}

	now := time.Now().UTC()
	message := "Incident marked covered"
	cleanNote := strings.TrimSpace(note)
	metadata.Note = firstNonEmpty(metadata.Note, cleanNote)
	updates := map[string]any{
		"status":          "covered",
		"covered_at":      &now,
		"covered_until":   coveredUntil,
		"coverage_note":   cleanNote,
		"last_event_at":   now,
		"latest_event":    message,
		"resolution_kind": "",
	}
	if err := s.db.Model(&incident).
		Updates(updates).Error; err != nil {
		return db.Incident{}, err
	}
	if err := s.db.Exec("UPDATE incidents SET resolved_at = NULL WHERE id = ?", incident.ID).Error; err != nil {
		return db.Incident{}, err
	}
	if err := s.createIncidentEvent(incident.ID, "incident_covered", message, "", metadata); err != nil {
		return db.Incident{}, err
	}
	var reloaded db.Incident
	if err := s.db.Where("id = ?", incident.ID).First(&reloaded).Error; err != nil {
		return db.Incident{}, err
	}
	return reloaded, nil
}

func (s *IncidentService) ReopenIncident(incidentID string, metadata IncidentLifecycleActionMetadata) (db.Incident, error) {
	var incident db.Incident
	if err := s.db.Where("id = ?", incidentID).First(&incident).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return db.Incident{}, ErrIncidentNotFound
		}
		return db.Incident{}, err
	}
	if incident.Status == "open" || incident.Status == "acknowledged" {
		return incident, nil
	}

	now := time.Now().UTC()
	message := "Incident reopened"
	updates := map[string]any{
		"status":          "open",
		"coverage_note":   "",
		"resolution_kind": "",
		"reopened_at":     &now,
		"reopen_count":    gorm.Expr("reopen_count + ?", 1),
		"last_event_at":   now,
		"latest_event":    message,
	}
	if err := s.db.Model(&incident).
		Updates(updates).Error; err != nil {
		return db.Incident{}, err
	}
	if err := s.db.Exec("UPDATE incidents SET resolved_at = NULL, covered_at = NULL, covered_until = NULL WHERE id = ?", incident.ID).Error; err != nil {
		return db.Incident{}, err
	}
	if err := s.updateMonitorIncidentState(incident.MonitorID, incident.ID, s.reopenedIncidentState(incident.MonitorID)); err != nil {
		return db.Incident{}, err
	}
	if err := s.createIncidentEvent(incident.ID, "incident_reopened", message, "", metadata); err != nil {
		return db.Incident{}, err
	}
	if err := NewAlertService(s.db, s.logger, s.cfg).QueueIncidentNotifications(incident.ID, "incident_opened"); err != nil {
		return db.Incident{}, err
	}
	var reloaded db.Incident
	if err := s.db.Where("id = ?", incident.ID).First(&reloaded).Error; err != nil {
		return db.Incident{}, err
	}
	return reloaded, nil
}

func (s *IncidentService) ResolveMonitorRemoved(monitorID string) error {
	incident, found, err := s.findActiveIncident(monitorID)
	if err != nil {
		return err
	}
	if !found {
		return s.updateMonitorIncidentState(monitorID, "", "unknown")
	}
	message := "Monitor removed; active incident resolved"
	return s.resolveIncidentRecord(incident, message, "", "unknown", "monitor_removed", true, incidentSystemEventMetadata())
}

func (s *IncidentService) resolveIncidentRecord(incident *db.Incident, message string, monitorReportID string, incidentState string, resolutionKind string, notify bool, metadata IncidentLifecycleActionMetadata) error {
	now := time.Now().UTC()
	updates := map[string]any{
		"status":          "resolved",
		"resolved_at":     &now,
		"last_event_at":   now,
		"latest_event":    message,
		"resolution_kind": resolutionKind,
	}
	if err := s.db.Model(incident).Where("status IN ?", activeIncidentStatuses()).Updates(updates).Error; err != nil {
		return err
	}
	if err := s.clearIncidentCoverageFields(incident.ID); err != nil {
		return err
	}
	if err := s.updateMonitorIncidentState(incident.MonitorID, "", incidentState); err != nil {
		return err
	}
	if err := s.createIncidentEvent(incident.ID, "incident_resolved", message, monitorReportID, metadata); err != nil {
		return err
	}
	if !notify || (s.cfg != nil && !s.cfg.AlertRecoveryNotifications) {
		return nil
	}
	return NewAlertService(s.db, s.logger, s.cfg).QueueIncidentNotifications(incident.ID, "incident_resolved")
}

func (s *IncidentService) createIncidentEvent(incidentID string, eventType string, message string, monitorReportID string, metadata IncidentLifecycleActionMetadata) error {
	metadata = normalizedIncidentLifecycleMetadata(metadata)
	event := db.IncidentEvent{
		ID:              utils.GenerateID("incident_event"),
		IncidentID:      incidentID,
		Type:            eventType,
		Message:         message,
		MonitorReportID: monitorReportID,
		ActorType:       metadata.ActorType,
		ActorID:         metadata.ActorID,
		Note:            metadata.Note,
	}
	return s.db.Create(&event).Error
}

func incidentSystemEventMetadata() IncidentLifecycleActionMetadata {
	return IncidentLifecycleActionMetadata{
		ActorType: "system",
		ActorID:   "core",
	}
}

func normalizedIncidentLifecycleMetadata(metadata IncidentLifecycleActionMetadata) IncidentLifecycleActionMetadata {
	metadata.ActorType = strings.TrimSpace(metadata.ActorType)
	if metadata.ActorType == "" {
		metadata.ActorType = "system"
	}
	metadata.ActorID = strings.TrimSpace(metadata.ActorID)
	if metadata.ActorID == "" {
		metadata.ActorID = "core"
	}
	metadata.Note = strings.TrimSpace(metadata.Note)
	return metadata
}
