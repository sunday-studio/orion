package service

import (
	"errors"
	"fmt"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"time"

	"gorm.io/gorm"
)

func (s *IncidentService) openOrUpdateIncident(agent db.Agent, monitor db.Monitor, monitorReportID string, health string, incidentState string) error {
	now := time.Now().UTC()
	message := incidentMessage(agent, monitor, health)
	severity := s.monitorIncidentSeverity(agent, monitor, health)
	impactedComponents, err := s.impactedComponentsForMonitorIncident(agent, monitor, health)
	if err != nil {
		return err
	}

	if monitor.ActiveIncidentID != "" {
		if updated, err := s.updateActiveIncidentByID(monitor.ActiveIncidentID, monitor.ID, monitorReportID, severity, message, incidentState, impactedComponents); err != nil {
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
		if handled, err := s.handleCoveredIncidentFailure(incident, monitor.ID, monitorReportID, incidentState, now); err != nil {
			return err
		} else if handled {
			return nil
		}

		updates := map[string]any{
			"severity":      severity,
			"last_event_at": now,
			"latest_event":  message,
		}
		if len(impactedComponents) > 0 {
			updates["impacted_components"] = encodeIncidentComponentImpacts(impactedComponents)
		}
		if err := s.db.Model(incident).Updates(updates).Error; err != nil {
			return err
		}
		if err := s.updateMonitorIncidentState(monitor.ID, incident.ID, incidentState); err != nil {
			return err
		}
		return s.createIncidentEvent(incident.ID, "monitor_failed", message, monitorReportID, incidentSystemEventMetadata())
	}

	newIncident := db.Incident{
		ID:                 utils.GenerateID("incident"),
		Status:             "open",
		Severity:           severity,
		Title:              incidentTitle(agent, monitor, health),
		AgentID:            agent.ID,
		MonitorID:          monitor.ID,
		ImpactedComponents: encodeIncidentComponentImpacts(impactedComponents),
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

	if err := s.createIncidentEvent(newIncident.ID, "incident_opened", message, monitorReportID, incidentSystemEventMetadata()); err != nil {
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

	updates := map[string]any{
		"status":          "resolved",
		"resolved_at":     &now,
		"last_event_at":   now,
		"latest_event":    message,
		"resolution_kind": "recovered",
	}
	if err := s.db.Model(incident).Updates(updates).Error; err != nil {
		return err
	}
	if err := s.clearIncidentCoverageFields(incident.ID); err != nil {
		return err
	}

	if err := s.updateMonitorIncidentState(monitor.ID, "", incidentState); err != nil {
		return err
	}

	if err := s.createIncidentEvent(incident.ID, "incident_resolved", message, monitorReportID, incidentSystemEventMetadata()); err != nil {
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
	result := s.db.Where("monitor_id = ? AND status IN ?", monitorID, activeIncidentStatuses()).
		Order("opened_at DESC").
		Limit(1).
		Find(&incident)
	duration := time.Since(startedAt)
	found := result.RowsAffected > 0
	s.diagnostics.RecordActiveIncidentLookup(duration, found, result.Error)
	if duration > activeIncidentLookupSlowThreshold {
		s.diagnostics.RecordSlowOperation("active_incident_lookup", monitorID, duration)
		s.logger.Warn("Slow active incident lookup", "monitor_id", monitorID, "duration_ms", duration.Milliseconds())
	}
	if result.Error != nil {
		return nil, false, result.Error
	}
	if !found {
		return nil, false, nil
	}
	return &incident, true, nil
}

func (s *IncidentService) updateActiveIncidentByID(incidentID string, monitorID string, monitorReportID string, severity string, message string, incidentState string, impactedComponents []db.IncidentComponentImpact) (bool, error) {
	now := time.Now().UTC()
	var incident db.Incident
	result := s.db.Where("id = ? AND monitor_id = ? AND status IN ?", incidentID, monitorID, activeIncidentStatuses()).
		First(&incident)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		if err := s.updateMonitorIncidentState(monitorID, "", incidentState); err != nil {
			return false, err
		}
		return false, nil
	}
	if result.Error != nil {
		return false, result.Error
	}
	if handled, err := s.handleCoveredIncidentFailure(&incident, monitorID, monitorReportID, incidentState, now); err != nil {
		return false, err
	} else if handled {
		return true, nil
	}

	updates := map[string]any{
		"severity":      severity,
		"last_event_at": now,
		"latest_event":  message,
	}
	if len(impactedComponents) > 0 {
		updates["impacted_components"] = encodeIncidentComponentImpacts(impactedComponents)
	}
	result = s.db.Model(&db.Incident{}).
		Where("id = ? AND monitor_id = ? AND status IN ?", incidentID, monitorID, activeIncidentStatuses()).
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
	return true, s.createIncidentEvent(incidentID, "monitor_failed", message, monitorReportID, incidentSystemEventMetadata())
}

func (s *IncidentService) resolveActiveIncidentByID(incidentID string, monitorID string, monitorReportID string, message string, incidentState string) (bool, error) {
	now := time.Now().UTC()
	updates := map[string]any{
		"status":          "resolved",
		"resolved_at":     &now,
		"last_event_at":   now,
		"latest_event":    message,
		"resolution_kind": "recovered",
	}
	result := s.db.Model(&db.Incident{}).
		Where("id = ? AND monitor_id = ? AND status IN ?", incidentID, monitorID, activeIncidentStatuses()).
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
	if err := s.clearIncidentCoverageFields(incidentID); err != nil {
		return true, err
	}
	if err := s.updateMonitorIncidentState(monitorID, "", incidentState); err != nil {
		return true, err
	}
	if err := s.createIncidentEvent(incidentID, "incident_resolved", message, monitorReportID, incidentSystemEventMetadata()); err != nil {
		return true, err
	}
	if s.cfg != nil && !s.cfg.AlertRecoveryNotifications {
		return true, nil
	}
	return true, NewAlertService(s.db, s.logger, s.cfg).QueueIncidentNotifications(incidentID, "incident_resolved")
}

func (s *IncidentService) handleCoveredIncidentFailure(incident *db.Incident, monitorID string, monitorReportID string, incidentState string, now time.Time) (bool, error) {
	if incident.Status != "covered" {
		return false, nil
	}
	if incidentCoverageActive(*incident, now) {
		message := "Incident coverage suppressed failing monitor report"
		if err := s.db.Model(incident).Updates(map[string]any{
			"last_event_at": now,
			"latest_event":  message,
		}).Error; err != nil {
			return false, err
		}
		if err := s.updateMonitorIncidentState(monitorID, incident.ID, incidentState); err != nil {
			return false, err
		}
		if err := s.createIncidentEvent(incident.ID, "incident_coverage_suppressed", message, monitorReportID, incidentSystemEventMetadata()); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, s.expireIncidentCoverage(incident, monitorReportID, now)
}

func incidentCoverageActive(incident db.Incident, now time.Time) bool {
	if incident.Status != "covered" {
		return false
	}
	return incident.CoveredUntil == nil || now.Before(incident.CoveredUntil.UTC())
}

func (s *IncidentService) expireIncidentCoverage(incident *db.Incident, monitorReportID string, now time.Time) error {
	message := "Incident coverage expired"
	if err := s.db.Model(incident).Updates(map[string]any{
		"status":          "open",
		"coverage_note":   "",
		"resolution_kind": "",
		"last_event_at":   now,
		"latest_event":    message,
	}).Error; err != nil {
		return err
	}
	if err := s.clearIncidentCoverageFields(incident.ID); err != nil {
		return err
	}
	if err := s.createIncidentEvent(incident.ID, "incident_coverage_expired", message, monitorReportID, incidentSystemEventMetadata()); err != nil {
		return err
	}
	incident.Status = "open"
	incident.CoveredAt = nil
	incident.CoveredUntil = nil
	incident.CoverageNote = ""
	incident.ResolutionKind = ""
	incident.LastEventAt = now
	incident.LatestEvent = message
	return nil
}

func (s *IncidentService) clearIncidentCoverageFields(incidentID string) error {
	return s.db.Exec("UPDATE incidents SET covered_at = NULL, covered_until = NULL, coverage_note = '' WHERE id = ?", incidentID).Error
}

func (s *IncidentService) updateMonitorIncidentState(monitorID string, activeIncidentID string, incidentState string) error {
	return s.db.Model(&db.Monitor{}).Where("id = ?", monitorID).Updates(map[string]any{
		"active_incident_id": activeIncidentID,
		"incident_state":     incidentState,
	}).Error
}
