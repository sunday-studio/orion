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
	ErrNodeNotFound = errors.New("server not found")
	ErrNodeInvalid  = errors.New("invalid server details")
	// decommissionedHistoryDays is how long reports stay after a server is removed with keep_history.
	decommissionedHistoryDays = 90
)

const (
	maxNodeNameLength = 100
	maxNodeLabels     = 20
	maxNodeNotes      = 2000
)

var labelPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,39}=[A-Za-z0-9_.:/@+ -]{0,80}$`)

// NodeDetailsInput is the PATCH payload; nil fields are left alone.
type NodeDetailsInput struct {
	Name   *string
	Labels *[]string
	Notes  *string
}

// DecodeNodeLabels reads the stored label array.
func DecodeNodeLabels(raw string) []string {
	out := []string{}
	_ = json.Unmarshal([]byte(raw), &out)
	return out
}

// UpdateNodeDetails changes name, labels and notes.
func (s *NodeService) UpdateNodeDetails(nodeID string, in NodeDetailsInput) (*db.Node, error) {
	var node db.Node
	if err := s.db.Where("id = ? AND (deleted_at IS NULL OR deleted_at = ?)", nodeID, time.Time{}).First(&node).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNodeNotFound
		}
		return nil, err
	}
	updates := map[string]any{}
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" || len(name) > maxNodeNameLength {
			return nil, fmt.Errorf("%w: name is required (max %d characters)", ErrNodeInvalid, maxNodeNameLength)
		}
		updates["name"] = name
	}
	if in.Labels != nil {
		if len(*in.Labels) > maxNodeLabels {
			return nil, fmt.Errorf("%w: at most %d labels", ErrNodeInvalid, maxNodeLabels)
		}
		seen := map[string]bool{}
		clean := make([]string, 0, len(*in.Labels))
		for _, label := range *in.Labels {
			label = strings.TrimSpace(label)
			if !labelPattern.MatchString(label) {
				return nil, fmt.Errorf("%w: label %q must look like key=value", ErrNodeInvalid, label)
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
		if len(notes) > maxNodeNotes {
			return nil, fmt.Errorf("%w: notes are too long (max %d characters)", ErrNodeInvalid, maxNodeNotes)
		}
		updates["notes"] = notes
	}
	if len(updates) > 0 {
		if err := s.db.Model(&db.Node{}).Where("id = ?", node.ID).Updates(updates).Error; err != nil {
			return nil, err
		}
	}
	err := s.db.Where("id = ?", node.ID).First(&node).Error
	return &node, err
}

// DecommissionResult says what removing a server did.
type DecommissionResult struct {
	MonitorsRemoved int  `json:"monitors_removed"`
	IncidentsClosed int  `json:"incidents_closed"`
	HistoryKept     bool `json:"history_kept"`
}

// DecommissionNode revokes the server's token and removes it. With keepHistory the reports stay
// for decommissionedHistoryDays and are then purged by retention; without it everything goes now.
func (s *NodeService) DecommissionNode(nodeID string, keepHistory bool, actor NodeTokenActionInput) (*DecommissionResult, error) {
	result := &DecommissionResult{HistoryKept: keepHistory}
	now := time.Now().UTC()
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var node db.Node
		if err := tx.Where("id = ? AND (deleted_at IS NULL OR deleted_at = ?)", nodeID, time.Time{}).First(&node).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNodeNotFound
			}
			return err
		}
		if node.System {
			return fmt.Errorf("%w: Core's own system node cannot be removed", ErrNodeInvalid)
		}
		var monitorIDs []string
		tx.Model(&db.Monitor{}).Where("node_id = ? AND lifecycle <> ?", node.ID, "deleted").Pluck("id", &monitorIDs)
		result.MonitorsRemoved = len(monitorIDs)
		if err := tx.Where("node_id = ?", node.ID).Delete(&db.NodeDiscovery{}).Error; err != nil { // a removed server keeps no scan of its host
			return err
		}
		if err := tx.Where("node_id = ?", node.ID).Delete(&db.NodeDiscoveryRequest{}).Error; err != nil {
			return err
		}
		if err := tx.Where("node_id = ?", node.ID).Delete(&db.NodeAppResource{}).Error; err != nil {
			return err
		}
		if err := tx.Where("node_id = ?", node.ID).Delete(&db.NodeAppResourceReport{}).Error; err != nil {
			return err
		}

		// Close what is open, so nothing keeps paging for a server that no longer exists.
		var openIncidents []string
		tx.Model(&db.Incident{}).Where("node_id = ? AND status IN ?", node.ID, []string{"open", "acknowledged", "covered"}).Pluck("id", &openIncidents)
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
		if err := tx.Model(&db.Node{}).Where("id = ?", node.ID).Updates(map[string]any{
			"token": revokedNodeTokenMarker(node.ID, node.TokenVersion+1), "token_hash": "", "token_version": normalizedNodeTokenVersion(node.TokenVersion) + 1,
			"token_revoked_at": now, "token_revocation_reason": reason, "previous_token_hash": "", "previous_token_expires_at": nil,
			// Frees the machine id so the same host can be added again later.
			"machine_id": fmt.Sprintf("%s#removed-%s", node.MachineId, node.ID), "deleted_at": now,
		}).Error; err != nil {
			return err
		}
		if err := tx.Model(&db.Monitor{}).Where("node_id = ?", node.ID).Updates(map[string]any{"lifecycle": "deleted", "deleted_at": now}).Error; err != nil {
			return err
		}
		// A removed server must not keep a public status page component stale or down.
		if err := tx.Where("resource_type = ? AND resource_id IN ?", "monitor", monitorIDsOrNone(monitorIDs)).Delete(&db.StatusPageComponentMapping{}).Error; err != nil {
			return err
		}
		if err := tx.Where("resource_type = ? AND resource_id = ?", "node", node.ID).Delete(&db.StatusPageComponentMapping{}).Error; err != nil {
			return err
		}
		if !keepHistory {
			if err := purgeNodeHistory(tx, []string{node.ID}); err != nil {
				return err
			}
		}
		return recordNodeTokenAuditEvent(tx, "node_decommissioned", node.ID, actor, normalizedNodeTokenVersion(node.TokenVersion)+1)
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// purgeNodeHistory deletes reports, monitor reports, rollups, monitors and the node rows.
func purgeNodeHistory(tx *gorm.DB, nodeIDs []string) error {
	if len(nodeIDs) == 0 {
		return nil
	}
	monitors := tx.Model(&db.Monitor{}).Select("id").Where("node_id IN ?", nodeIDs)
	steps := []*gorm.DB{
		tx.Where("node_id IN ?", nodeIDs).Delete(&db.ServiceLogEntry{}),
		tx.Where("monitor_id IN (?)", monitors).Delete(&db.IncidentCandidateDismissal{}),
		tx.Where("resource_type = ? AND resource_id IN (?)", "monitor", monitors).Delete(&db.StatusPageComponentMapping{}),
		tx.Where("resource_type = ? AND resource_id IN ?", "node", nodeIDs).Delete(&db.StatusPageComponentMapping{}),
		tx.Where("monitor_id IN (?)", monitors).Delete(&db.MonitorReport{}),
		tx.Where("monitor_id IN (?)", monitors).Delete(&db.MonitorUptimeRollup{}),
		tx.Where("node_id IN ?", nodeIDs).Delete(&db.NodeReport{}),
		tx.Where("node_id IN ?", nodeIDs).Delete(&db.Monitor{}),
		tx.Where("id IN ?", nodeIDs).Delete(&db.Node{}),
	}
	for _, step := range steps {
		if step.Error != nil {
			return step.Error
		}
	}
	return nil
}

// PurgeDecommissionedNodes removes servers whose kept history has aged out.
func PurgeDecommissionedNodes(database *gorm.DB, logger *utils.Logger, now time.Time) (int64, error) {
	cutoff := now.AddDate(0, 0, -decommissionedHistoryDays)
	var ids []string
	if err := database.Model(&db.Node{}).Where("deleted_at <> ? AND deleted_at < ?", time.Time{}, cutoff).Limit(200).Pluck("id", &ids).Error; err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}
	if err := database.Transaction(func(tx *gorm.DB) error { return purgeNodeHistory(tx, ids) }); err != nil {
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
