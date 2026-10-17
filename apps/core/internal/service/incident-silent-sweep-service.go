package service

import (
	"errors"
	"orion/core/internal/db"
	"time"

	"gorm.io/gorm"
)

// SweepSilentMonitors opens stale incidents for agent-reported monitors that
// stopped reporting. Unlike ReconcileStaleMonitors, which only runs when an
// agent sends a report, this runs from the Worker so a crashed, killed or
// unreachable agent still raises an incident. Recovery needs no sweeper work:
// the next successful report resolves the incident through the normal path.
//
// Core-managed monitors are skipped (the Worker's own scheduler and heartbeat
// reconciliation own them), as are agents in maintenance mode. Staleness is
// measured on server receive time so a skewed agent clock cannot hide an outage.
// It returns the number of monitors that were newly flagged stale.
func (s *IncidentService) SweepSilentMonitors(now time.Time) (int, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	healthConfig := DefaultHealthConfig()

	var monitors []db.Monitor
	if err := s.db.
		Where("lifecycle = ?", "active").
		Where("NOT (incident_state = ? AND active_incident_id <> '')", "stale").
		Where("id NOT IN (?)", s.db.Model(&db.CoreMonitorConfig{}).Select("monitor_id")).
		Find(&monitors).Error; err != nil {
		return 0, err
	}
	if len(monitors) == 0 {
		return 0, nil
	}

	agents := map[string]*db.Agent{}
	flagged := 0
	var firstErr error
	for _, monitor := range monitors {
		agent, err := s.sweepAgent(agents, monitor.AgentID)
		if err != nil {
			s.logger.Error("Silent monitor sweep could not load agent", "agent_id", monitor.AgentID, "error", err)
			firstErr = errors.Join(firstErr, err)
			continue
		}
		if agent == nil || agent.InMaintenance(now) {
			continue
		}

		lastHeardAt, err := s.latestMonitorReportTime(monitor)
		if err != nil {
			s.logger.Error("Silent monitor sweep could not load latest report", "monitor_id", monitor.ID, "error", err)
			firstErr = errors.Join(firstErr, err)
			continue
		}
		if now.Sub(lastHeardAt) <= staleWindow(monitor.ReportingIntervalSeconds, healthConfig) {
			continue
		}

		if err := s.openOrUpdateIncident(*agent, monitor, "", "stale", "stale"); err != nil {
			s.logger.Error("Silent monitor sweep could not open incident", "monitor_id", monitor.ID, "error", err)
			firstErr = errors.Join(firstErr, err)
			continue
		}
		flagged++
	}
	return flagged, firstErr
}

// sweepAgent returns the agent (cached for the sweep) or nil when it no longer exists.
func (s *IncidentService) sweepAgent(cache map[string]*db.Agent, agentID string) (*db.Agent, error) {
	if agent, ok := cache[agentID]; ok {
		return agent, nil
	}
	var agent db.Agent
	if err := s.db.Where("id = ?", agentID).First(&agent).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			cache[agentID] = nil
			return nil, nil
		}
		return nil, err
	}
	cache[agentID] = &agent
	return &agent, nil
}

// latestMonitorReportTime is when Core last received a report for the monitor,
// falling back to when the monitor was created if it has never reported.
func (s *IncidentService) latestMonitorReportTime(monitor db.Monitor) (time.Time, error) {
	var report db.MonitorReport
	err := s.db.Where("monitor_id = ?", monitor.ID).Order("created_at DESC").First(&report).Error
	switch {
	case err == nil:
		return report.CreatedAt, nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		return monitor.CreatedAt, nil
	default:
		return time.Time{}, err
	}
}
