package service

import (
	"errors"
	"fmt"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"strings"
	"time"

	"gorm.io/gorm"
)

var (
	ErrIncidentInvalid        = errors.New("invalid incident")
	ErrIncidentMonitorMissing = errors.New("monitor not found")
	ErrIncidentAgentMissing   = errors.New("server not found")
	ErrIncidentAlreadyActive  = errors.New("this monitor already has an active incident")
)

const (
	maxIncidentTitleLength   = 200
	maxIncidentSummaryLength = 4000
	maxIncidentNoteLength    = 4000
)

// nextIncidentNumber returns the next human incident number. Creation always runs inside a
// transaction (BEGIN IMMEDIATE profile), so numbers are handed out one at a time; the unique
// index on number is the backstop.
func (s *IncidentService) nextIncidentNumber() int {
	var next int
	s.db.Raw("SELECT COALESCE(MAX(number), 0) + 1 FROM incidents").Scan(&next)
	return next
}

// DeclareIncidentInput is what an operator types in the Declare incident dialog.
type DeclareIncidentInput struct {
	Title     string
	Severity  string
	MonitorID string
	AgentID   string
	Summary   string
	Notify    bool
}

// DeclareIncident opens an incident by hand for something Orion has not noticed.
func (s *IncidentService) DeclareIncident(in DeclareIncidentInput, actor IncidentLifecycleActionMetadata) (db.Incident, error) {
	title := strings.TrimSpace(in.Title)
	summary := strings.TrimSpace(in.Summary)
	switch {
	case title == "" || len(title) > maxIncidentTitleLength:
		return db.Incident{}, fmt.Errorf("%w: title is required (max %d characters)", ErrIncidentInvalid, maxIncidentTitleLength)
	case len(summary) > maxIncidentSummaryLength:
		return db.Incident{}, fmt.Errorf("%w: summary is too long", ErrIncidentInvalid)
	}
	severity := strings.ToLower(strings.TrimSpace(in.Severity))
	switch severity {
	case "low", "medium", "high", "critical":
	default:
		return db.Incident{}, fmt.Errorf("%w: severity must be low, medium, high or critical", ErrIncidentInvalid)
	}
	monitorID, agentID := strings.TrimSpace(in.MonitorID), strings.TrimSpace(in.AgentID)
	if monitorID == "" && agentID == "" {
		return db.Incident{}, fmt.Errorf("%w: choose an affected monitor or server", ErrIncidentInvalid)
	}

	var created db.Incident
	err := s.inTx(func(tx *IncidentService) error {
		if monitorID != "" {
			var monitor db.Monitor
			if err := tx.db.Where("id = ?", monitorID).First(&monitor).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return ErrIncidentMonitorMissing
				}
				return err
			}
			agentID = monitor.AgentID
			var active int64
			tx.db.Model(&db.Incident{}).Where("monitor_id = ? AND status IN ?", monitorID, activeIncidentStatuses()).Count(&active)
			if active > 0 {
				return ErrIncidentAlreadyActive
			}
		} else if err := tx.db.Where("id = ?", agentID).First(&db.Agent{}).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrIncidentAgentMissing
			}
			return err
		}
		now := time.Now().UTC()
		created = db.Incident{
			ID: utils.GenerateID("incident"), Number: tx.nextIncidentNumber(), Status: "open", Severity: severity, Title: title,
			AgentID: agentID, MonitorID: monitorID, ImpactedComponents: "[]", Summary: summary, Source: "manual",
			OpenedAt: now, LastEventAt: now, LatestEvent: firstNonEmpty(summary, "Incident declared by "+actor.ActorID), NotificationStatus: "pending",
		}
		if err := tx.db.Create(&created).Error; err != nil {
			return err
		}
		if err := tx.createIncidentEvent(created.ID, "incident_declared", "Incident declared manually", "", actor); err != nil {
			return err
		}
		if in.Notify {
			return NewAlertService(tx.db, tx.logger, tx.cfg).QueueIncidentNotifications(created.ID, "incident_opened")
		}
		return nil
	})
	return created, err
}

// AddIncidentNote records a free-text note on the timeline.
func (s *IncidentService) AddIncidentNote(incidentID, text string, actor IncidentLifecycleActionMetadata) error {
	text = strings.TrimSpace(text)
	if text == "" || len(text) > maxIncidentNoteLength {
		return fmt.Errorf("%w: note text is required (max %d characters)", ErrIncidentInvalid, maxIncidentNoteLength)
	}
	var count int64
	s.db.Model(&db.Incident{}).Where("id = ?", incidentID).Count(&count)
	if count == 0 {
		return ErrIncidentNotFound
	}
	actor.Note = text
	return s.createIncidentEvent(incidentID, "incident_note", "Note added", "", actor)
}

// SetIncidentResponder assigns (or clears, with an empty name) the person on point.
func (s *IncidentService) SetIncidentResponder(incidentID, name string, actor IncidentLifecycleActionMetadata) (db.Incident, error) {
	name = strings.TrimSpace(name)
	if len(name) > 120 {
		return db.Incident{}, fmt.Errorf("%w: responder name is too long", ErrIncidentInvalid)
	}
	var incident db.Incident
	if err := s.db.Where("id = ?", incidentID).First(&incident).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return db.Incident{}, ErrIncidentNotFound
		}
		return db.Incident{}, err
	}
	now := time.Now().UTC()
	updates := map[string]any{"responder": name, "responder_at": &now}
	if name == "" {
		updates["responder_at"] = nil
	}
	if err := s.db.Model(&incident).Updates(updates).Error; err != nil {
		return db.Incident{}, err
	}
	message := "Responder set to " + name
	if name == "" {
		message = "Responder cleared"
	}
	if err := s.createIncidentEvent(incident.ID, "incident_responder_changed", message, "", actor); err != nil {
		return db.Incident{}, err
	}
	err := s.db.Where("id = ?", incident.ID).First(&incident).Error
	return incident, err
}
