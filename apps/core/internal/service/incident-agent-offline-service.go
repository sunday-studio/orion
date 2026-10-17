package service

import (
	"errors"
	"fmt"
	"orion/core/internal/db"
	"strings"
	"time"

	"gorm.io/gorm"
)

const agentOfflineMonitorPrefix = "monitor-agent-offline-"

func agentOfflineMonitorID(agentID string) string { return agentOfflineMonitorPrefix + agentID }

// SweepSilentAgents raises an incident for an agent that has stopped reporting
// altogether and has no monitors of its own that the monitor sweep could flag
// (a host that only sends system metrics). An agent with monitors is already
// covered by their stale incidents, and revoked, maintenance and Core-owner
// agents are expected to be quiet. The incident hangs off a synthetic
// "Agent connectivity" monitor so it flows through the normal incident and alert
// pipeline; ResolveAgentOffline closes it when the agent reports again.
// It returns the number of agents newly flagged.
func (s *IncidentService) SweepSilentAgents(now time.Time) (int, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	var agents []db.Agent
	if err := s.db.
		Where("token_revoked_at IS NULL AND machine_id <> ?", coreOwnerMachineID).
		Where(`NOT EXISTS (SELECT 1 FROM monitors m WHERE m.agent_id = agents.id AND m.lifecycle = 'active' AND m.id NOT LIKE ?)`, agentOfflineMonitorPrefix+"%").
		Find(&agents).Error; err != nil {
		return 0, err
	}

	healthConfig := DefaultHealthConfig()
	flagged := 0
	var firstErr error
	for _, agent := range agents {
		if agent.InMaintenance(now) || now.Sub(agent.LastSeen) <= staleWindow(agent.ReportingIntervalSeconds, healthConfig) {
			continue
		}
		monitor, err := s.ensureAgentOfflineMonitor(agent)
		if err != nil {
			firstErr = errors.Join(firstErr, err)
			continue
		}
		if monitor.IncidentState == "stale" && monitor.ActiveIncidentID != "" {
			continue // already open
		}
		if err := s.openOrUpdateIncident(agent, monitor, "", "stale", "stale"); err != nil {
			firstErr = errors.Join(firstErr, err)
			continue
		}
		flagged++
	}
	return flagged, firstErr
}

// ResolveAgentOffline closes the agent's offline incident, if any, once it reports again.
func (s *IncidentService) ResolveAgentOffline(agentID string) error {
	var monitor db.Monitor
	err := s.db.Where("id = ?", agentOfflineMonitorID(agentID)).First(&monitor).Error
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

func (s *IncidentService) ensureAgentOfflineMonitor(agent db.Agent) (db.Monitor, error) {
	id := agentOfflineMonitorID(agent.ID)
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
	description := fmt.Sprintf("Raised by Core when agent %s stops reporting. It resolves when the agent reports again.", agent.Name)
	monitor = db.Monitor{
		ID:                       id,
		AgentID:                  agent.ID,
		Name:                     "Agent connectivity: " + strings.TrimSpace(agent.Name),
		Description:              &description,
		Type:                     "agent-connectivity",
		System:                   true,
		Lifecycle:                "active",
		Health:                   "unknown",
		ComputedHealth:           "unknown",
		IncidentState:            "unknown",
		ReportingIntervalSeconds: max(agent.ReportingIntervalSeconds, 60),
	}
	return monitor, s.db.Create(&monitor).Error
}
