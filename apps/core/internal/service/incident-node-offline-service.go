package service

import (
	"errors"
	"fmt"
	"orion/core/internal/db"
	"strings"
	"time"

	"gorm.io/gorm"
)

const nodeOfflineMonitorPrefix = "monitor-node-offline-"

func nodeOfflineMonitorID(nodeID string) string { return nodeOfflineMonitorPrefix + nodeID }

// SweepSilentNodes raises an incident for a node that has stopped reporting
// altogether and has no monitors of its own that the monitor sweep could flag
// (a host that only sends system metrics). A node with monitors is already
// covered by their stale incidents, and revoked, maintenance and Core-owner
// nodes are expected to be quiet. The incident hangs off a synthetic
// "Node connectivity" monitor so it flows through the normal incident and alert
// pipeline; ResolveNodeOffline closes it when the node reports again.
// It returns the number of nodes newly flagged.
func (s *IncidentService) SweepSilentNodes(now time.Time) (int, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	var nodes []db.Node
	if err := s.db.
		Where("token_revoked_at IS NULL AND machine_id <> ?", coreOwnerMachineID).
		Where(`NOT EXISTS (SELECT 1 FROM monitors m WHERE m.node_id = nodes.id AND m.lifecycle = 'active' AND m.id NOT LIKE ?)`, nodeOfflineMonitorPrefix+"%").
		Find(&nodes).Error; err != nil {
		return 0, err
	}

	healthConfig := DefaultHealthConfig()
	flagged := 0
	var firstErr error
	for _, node := range nodes {
		if node.InMaintenance(now) || now.Sub(node.LastSeen) <= staleWindow(node.ReportingIntervalSeconds, healthConfig) {
			continue
		}
		monitor, err := s.ensureNodeOfflineMonitor(node)
		if err != nil {
			firstErr = errors.Join(firstErr, err)
			continue
		}
		if monitor.IncidentState == "stale" && monitor.ActiveIncidentID != "" {
			continue // already open
		}
		if err := s.openOrUpdateIncident(node, monitor, "", "stale", "stale"); err != nil {
			firstErr = errors.Join(firstErr, err)
			continue
		}
		flagged++
	}
	return flagged, firstErr
}

// ResolveNodeOffline closes the node's offline incident, if any, once it reports again.
func (s *IncidentService) ResolveNodeOffline(nodeID string) error {
	var monitor db.Monitor
	err := s.db.Where("id = ?", nodeOfflineMonitorID(nodeID)).First(&monitor).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if monitor.Lifecycle != "active" && monitor.ActiveIncidentID == "" {
		return nil
	}
	return s.inTx(func(tx *IncidentService) error {
		if err := tx.resolveActiveIncident(monitor, "", "up"); err != nil {
			return err
		}
		// Parked so the monitor sweep does not flag a monitor that never reports.
		return tx.db.Model(&db.Monitor{}).Where("id = ?", monitor.ID).Updates(map[string]any{"lifecycle": "disabled", "health": "up", "computed_health": "up"}).Error
	})
}

func (s *IncidentService) ensureNodeOfflineMonitor(node db.Node) (db.Monitor, error) {
	id := nodeOfflineMonitorID(node.ID)
	var monitor db.Monitor
	err := s.db.Where("id = ?", id).First(&monitor).Error
	switch {
	case err == nil:
		if monitor.Lifecycle != "active" {
			if err := s.db.Model(&db.Monitor{}).Where("id = ?", id).Update("lifecycle", "active").Error; err != nil {
				return db.Monitor{}, err
			}
			monitor.Lifecycle = "active"
		}
		return monitor, nil
	case !errors.Is(err, gorm.ErrRecordNotFound):
		return db.Monitor{}, err
	}
	description := fmt.Sprintf("Raised by Core when node %s stops reporting. It resolves when the node reports again.", node.Name)
	monitor = db.Monitor{
		ID:                       id,
		NodeID:                   node.ID,
		Name:                     "Node connectivity: " + strings.TrimSpace(node.Name),
		Description:              &description,
		Type:                     "node-connectivity",
		System:                   true,
		Lifecycle:                "active",
		Health:                   "unknown",
		ComputedHealth:           "unknown",
		IncidentState:            "unknown",
		ReportingIntervalSeconds: max(node.ReportingIntervalSeconds, 60),
	}
	return monitor, s.db.Create(&monitor).Error
}
