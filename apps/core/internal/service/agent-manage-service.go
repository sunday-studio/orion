package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"regexp"
	"strings"
	"time"

	"gorm.io/gorm"
)

var (
	ErrAgentNotFound = errors.New("server not found")
	ErrAgentInvalid  = errors.New("invalid server details")
	// decommissionedHistoryDays is how long reports stay after a server is removed with keep_history.
	decommissionedHistoryDays = 90
)

const (
	maxAgentNameLength = 100
	maxAgentLabels     = 20
	maxAgentNotes      = 2000
)

var labelPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,39}=[A-Za-z0-9_.:/@+ -]{0,80}$`)

// AgentDetailsInput is the PATCH payload; nil fields are left alone.
type AgentDetailsInput struct {
	Name   *string
	Labels *[]string
	Notes  *string
}

// DecodeAgentLabels reads the stored label array.
func DecodeAgentLabels(raw string) []string {
	out := []string{}
	_ = json.Unmarshal([]byte(raw), &out)
	return out
}

// UpdateAgentDetails changes name, labels and notes.
func (s *AgentService) UpdateAgentDetails(agentID string, in AgentDetailsInput) (*db.Agent, error) {
	var agent db.Agent
	if err := s.db.Where("id = ? AND (deleted_at IS NULL OR deleted_at = ?)", agentID, time.Time{}).First(&agent).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAgentNotFound
		}
		return nil, err
	}
	updates := map[string]any{}
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" || len(name) > maxAgentNameLength {
			return nil, fmt.Errorf("%w: name is required (max %d characters)", ErrAgentInvalid, maxAgentNameLength)
		}
		updates["name"] = name
	}
	if in.Labels != nil {
		if len(*in.Labels) > maxAgentLabels {
			return nil, fmt.Errorf("%w: at most %d labels", ErrAgentInvalid, maxAgentLabels)
		}
		seen := map[string]bool{}
		clean := make([]string, 0, len(*in.Labels))
		for _, label := range *in.Labels {
			label = strings.TrimSpace(label)
			if !labelPattern.MatchString(label) {
				return nil, fmt.Errorf("%w: label %q must look like key=value", ErrAgentInvalid, label)
			}
			if !seen[label] {
				seen[label] = true
				clean = append(clean, label)
			}
		}
		raw, _ := json.Marshal(clean)
		updates["labels"] = string(raw)
	}
	if in.Notes != nil {
		notes := strings.TrimSpace(*in.Notes)
		if len(notes) > maxAgentNotes {
			return nil, fmt.Errorf("%w: notes are too long (max %d characters)", ErrAgentInvalid, maxAgentNotes)
		}
		updates["notes"] = notes
	}
	if len(updates) > 0 {
		if err := s.db.Model(&db.Agent{}).Where("id = ?", agent.ID).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	err := s.db.Where("id = ?", agent.ID).First(&agent).Error
	return &agent, err
}

// DecommissionResult says what removing a server did.
type DecommissionResult struct {
	MonitorsRemoved int  `json:"monitors_removed"`
	IncidentsClosed int  `json:"incidents_closed"`
	HistoryKept     bool `json:"history_kept"`
}

// DecommissionAgent revokes the server's token and removes it. With keepHistory the reports stay
// for decommissionedHistoryDays and are then purged by retention; without it everything goes now.
func (s *AgentService) DecommissionAgent(agentID string, keepHistory bool, actor AgentTokenActionInput) (*DecommissionResult, error) {
	result := &DecommissionResult{HistoryKept: keepHistory}
	now := time.Now().UTC()
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var agent db.Agent
		if err := tx.Where("id = ? AND (deleted_at IS NULL OR deleted_at = ?)", agentID, time.Time{}).First(&agent).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrAgentNotFound
			}
			return err
		}
		if agent.System {
			return fmt.Errorf("%w: Core's own system agent cannot be removed", ErrAgentInvalid)
		}
		var monitorIDs []string
		tx.Model(&db.Monitor{}).Where("agent_id = ? AND lifecycle <> ?", agent.ID, "deleted").Pluck("id", &monitorIDs)
		result.MonitorsRemoved = len(monitorIDs)
		if err := tx.Where("agent_id = ?", agent.ID).Delete(&db.AgentDiscovery{}).Error; err != nil { // a removed server keeps no scan of its host
			return err
		}
		if err := tx.Where("agent_id = ?", agent.ID).Delete(&db.AgentDiscoveryRequest{}).Error; err != nil {
			return err
		}
		if err := tx.Where("agent_id = ?", agent.ID).Delete(&db.AgentAppResource{}).Error; err != nil {
			return err
		}
		if err := tx.Where("agent_id = ?", agent.ID).Delete(&db.AgentAppResourceReport{}).Error; err != nil {
			return err
		}

		// Close what is open, so nothing keeps paging for a server that no longer exists.
		var openIncidents []string
		tx.Model(&db.Incident{}).Where("agent_id = ? AND status IN ?", agent.ID, []string{"open", "acknowledged", "covered"}).Pluck("id", &openIncidents)
		if len(openIncidents) > 0 {
			if err := tx.Model(&db.Incident{}).Where("id IN ?", openIncidents).
				Updates(map[string]any{"status": "resolved", "resolved_at": now, "resolution_kind": "decommissioned", "last_event_at": now, "latest_event": "Server removed"}).Error; err != nil {
				return err
			}
			// Nothing may page for a server that no longer exists.
			if err := tx.Model(&db.AlertDelivery{}).Where("incident_id IN ? AND status IN ?", openIncidents, []string{"pending", "failed"}).
				Updates(map[string]any{"status": "suppressed", "error": "server removed", "locked_until": nil}).Error; err != nil {
				return err
			}
			for i, incidentID := range openIncidents {
				event := db.IncidentEvent{ID: utils.GenerateID("incident_event") + fmt.Sprintf("-%d", i), IncidentID: incidentID, Type: "incident_resolved", Message: "Server removed", ActorType: "user", ActorID: firstNonEmpty(actor.ActorID, "admin"), CreatedAt: now}
				if err := tx.Create(&event).Error; err != nil {
					return err
				}
			}
		}
		result.IncidentsClosed = len(openIncidents)
		if len(monitorIDs) > 0 {
			tx.Model(&db.Monitor{}).Where("id IN ?", monitorIDs).Updates(map[string]any{"active_incident_id": "", "incident_state": "unknown"})
		}

		reason := "Server removed"
		if err := tx.Model(&db.Agent{}).Where("id = ?", agent.ID).Updates(map[string]any{
			"token": revokedAgentTokenMarker(agent.ID, agent.TokenVersion+1), "token_hash": "", "token_version": normalizedAgentTokenVersion(agent.TokenVersion) + 1,
			"token_revoked_at": now, "token_revocation_reason": reason, "previous_token_hash": "", "previous_token_expires_at": nil,
			// Frees the machine id so the same host can be added again later.
			"machine_id": fmt.Sprintf("%s#removed-%s", agent.MachineId, agent.ID), "deleted_at": now,
		}).Error; err != nil {
			return err
		}
		if err := tx.Model(&db.Monitor{}).Where("agent_id = ?", agent.ID).Updates(map[string]any{"lifecycle": "deleted", "deleted_at": now}).Error; err != nil {
			return err
		}
		// A removed server must not keep a public status page component stale or down.
		if err := tx.Where("resource_type = ? AND resource_id IN ?", "monitor", monitorIDsOrNone(monitorIDs)).Delete(&db.StatusPageComponentMapping{}).Error; err != nil {
			return err
		}
		if err := tx.Where("resource_type = ? AND resource_id = ?", "agent", agent.ID).Delete(&db.StatusPageComponentMapping{}).Error; err != nil {
			return err
		}
		if !keepHistory {
			if err := purgeAgentHistory(tx, []string{agent.ID}); err != nil {
				return err
			}
		}
		return recordAgentTokenAuditEvent(tx, "agent_decommissioned", agent.ID, actor, normalizedAgentTokenVersion(agent.TokenVersion)+1)
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// purgeAgentHistory deletes reports, monitor reports, rollups, monitors and the agent rows.
func purgeAgentHistory(tx *gorm.DB, agentIDs []string) error {
	if len(agentIDs) == 0 {
		return nil
	}
	monitors := tx.Model(&db.Monitor{}).Select("id").Where("agent_id IN ?", agentIDs)
	steps := []*gorm.DB{
		tx.Where("agent_id IN ?", agentIDs).Delete(&db.ServiceLogEntry{}),
		tx.Where("monitor_id IN (?)", monitors).Delete(&db.IncidentCandidateDismissal{}),
		tx.Where("resource_type = ? AND resource_id IN (?)", "monitor", monitors).Delete(&db.StatusPageComponentMapping{}),
		tx.Where("resource_type = ? AND resource_id IN ?", "agent", agentIDs).Delete(&db.StatusPageComponentMapping{}),
		tx.Where("monitor_id IN (?)", monitors).Delete(&db.MonitorReport{}),
		tx.Where("monitor_id IN (?)", monitors).Delete(&db.MonitorUptimeRollup{}),
		tx.Where("agent_id IN ?", agentIDs).Delete(&db.AgentReport{}),
		tx.Where("agent_id IN ?", agentIDs).Delete(&db.Monitor{}),
		tx.Where("id IN ?", agentIDs).Delete(&db.Agent{}),
	}
	for _, step := range steps {
		if step.Error != nil {
			return step.Error
		}
	}
	return nil
}

// PurgeDecommissionedAgents removes servers whose kept history has aged out.
func PurgeDecommissionedAgents(database *gorm.DB, logger *utils.Logger, now time.Time) (int64, error) {
	cutoff := now.AddDate(0, 0, -decommissionedHistoryDays)
	var ids []string
	if err := database.Model(&db.Agent{}).Where("deleted_at <> ? AND deleted_at < ?", time.Time{}, cutoff).Limit(200).Pluck("id", &ids).Error; err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}
	if err := database.Transaction(func(tx *gorm.DB) error { return purgeAgentHistory(tx, ids) }); err != nil {
		return 0, err
	}
	logger.Info("Purged decommissioned servers", "count", len(ids))
	return int64(len(ids)), nil
}

func monitorIDsOrNone(ids []string) []string {
	if len(ids) == 0 {
		return []string{""}
	}
	return ids
}
