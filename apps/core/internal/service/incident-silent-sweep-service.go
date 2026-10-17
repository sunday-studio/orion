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

	// One statement finds every candidate with the time Core last heard from it,
	// instead of one round trip per monitor, so the sweep stays cheap at thousands
	// of monitors and does not queue ingestion behind it.
	var rows []struct {
		db.Monitor   `gorm:"embedded"`
		LastReportAt *string
	}
	if err := s.db.Model(&db.Monitor{}).
		Select("monitors.*, (SELECT MAX(created_at) FROM monitor_reports WHERE monitor_reports.monitor_id = monitors.id) AS last_report_at").
		Where("lifecycle = ?", "active").
		Where("NOT (incident_state = ? AND active_incident_id <> '')", "stale").
		Where("id NOT IN (?)", s.db.Model(&db.CoreMonitorConfig{}).Select("monitor_id")).
		Scan(&rows).Error; err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}

	agents := map[string]*db.Agent{}
	flagged := 0
	var firstErr error
	for _, row := range rows {
		monitor := row.Monitor
		agent, err := s.sweepAgent(agents, monitor.AgentID)
		if err != nil {
			s.logger.Error("Silent monitor sweep could not load agent", "agent_id", monitor.AgentID, "error", err)
			firstErr = errors.Join(firstErr, err)
			continue
		}
		if agent == nil || agent.InMaintenance(now) {
			continue
		}

		lastHeardAt := lastHeardTime(monitor, row.LastReportAt)
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

// sqliteTimeLayouts are the formats SQLite stores GORM timestamps in.
var sqliteTimeLayouts = []string{"2006-01-02 15:04:05.999999999-07:00", "2006-01-02 15:04:05.999999999 -0700 MST", time.RFC3339Nano}

// lastHeardTime is when Core last received a report for the monitor, falling back
// to when the monitor was created if it has never reported (or the stored time is unreadable).
func lastHeardTime(monitor db.Monitor, lastReportAt *string) time.Time {
	if lastReportAt != nil {
		for _, layout := range sqliteTimeLayouts {
			if parsed, err := time.Parse(layout, *lastReportAt); err == nil {
				return parsed
			}
		}
	}
	return monitor.CreatedAt
}
