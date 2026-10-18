package api

import (
	"fmt"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"strings"
	"time"
)

const recentCheckCount = 24

// enrichMonitorRows fills the list/detail extras for the given monitors, one query per kind of data.
func (s *Server) enrichMonitorRows(rows []MonitorResponse, monitors []db.Monitor) {
	if len(monitors) == 0 {
		return
	}
	ids := make([]string, 0, len(monitors))
	for _, m := range monitors {
		ids = append(ids, m.ID)
	}
	configs := map[string]db.CoreMonitorConfig{}
	var cfgRows []db.CoreMonitorConfig
	s.db.Where("monitor_id IN ?", ids).Find(&cfgRows)
	for _, c := range cfgRows {
		configs[c.MonitorID] = c
	}
	agents := s.monitorAgentsByID(monitors)
	recent := s.recentMonitorChecks(ids)
	lastFailure := s.lastNonUpCheck(ids)
	for i := range rows {
		m := monitors[i]
		health := effectiveMonitorHealth(m)
		cfg, isCore := configs[m.ID]
		r := &rows[i]
		r.Kind = m.Type
		if isCore {
			r.Kind = cfg.Kind
			r.Paused, r.PausedAt, r.PausedBy = cfg.Paused, cfg.PausedAt, cfg.PausedBy
			if cfg.IntervalSeconds > 0 {
				r.IntervalSeconds = cfg.IntervalSeconds
			}
		}
		r.Target = monitorTargetLabel(m, cfg, isCore, agents[m.AgentID].Name)
		checks := recent[m.ID]
		r.RecentChecks = checkStates(checks)
		since, detail := failureFromChecks(checks)
		upSince := m.CreatedAt
		if t, ok := lastFailure[m.ID]; ok {
			upSince = t
		}
		r.StatusText, r.StatusDetail = monitorStatusFor(m, cfg, isCore, health, since, detail, upSince)
	}
}

type recentCheck struct {
	health    string
	payload   string
	createdAt time.Time
}

// recentMonitorChecks loads the newest recentCheckCount reports per monitor, newest first.
func (s *Server) recentMonitorChecks(ids []string) map[string][]recentCheck {
	var rows []struct {
		MonitorID string
		Health    string
		Payload   string
		CreatedAt time.Time
	}
	s.db.Raw(`SELECT monitor_id, health, payload, created_at FROM (
		SELECT monitor_id, health, payload, created_at, ROW_NUMBER() OVER (PARTITION BY monitor_id ORDER BY created_at DESC) AS rn
		FROM monitor_reports WHERE monitor_id IN ?) WHERE rn <= ? ORDER BY monitor_id, created_at DESC`, ids, recentCheckCount).Scan(&rows)
	out := make(map[string][]recentCheck, len(ids))
	for _, r := range rows {
		out[r.MonitorID] = append(out[r.MonitorID], recentCheck{health: r.Health, payload: r.Payload, createdAt: r.CreatedAt})
	}
	return out
}

// checkStates returns the strip oldest first.
func checkStates(newestFirst []recentCheck) []string {
	out := make([]string, 0, len(newestFirst))
	for i := len(newestFirst) - 1; i >= 0; i-- {
		switch h := newestFirst[i].health; h {
		case "up", "down", "degraded":
			out = append(out, h)
		default:
			out = append(out, "down")
		}
	}
	return out
}

// failureFromChecks finds when the current run of non-up checks began and its latest error text.
func failureFromChecks(newestFirst []recentCheck) (since *time.Time, detail string) {
	for _, c := range newestFirst {
		if c.health == "up" {
			break
		}
		t := c.createdAt
		since = &t
		if detail == "" {
			summary, _, _ := describeMonitorReport(db.MonitorReport{Health: c.health, Payload: c.payload})
			detail = summary
		}
	}
	return since, detail
}

func monitorTargetLabel(m db.Monitor, cfg db.CoreMonitorConfig, isCore bool, agentName string) string {
	if !isCore {
		if agentName != "" {
			return "Agent, " + m.Type + " on " + agentName
		}
		return "Agent, " + m.Type
	}
	switch cfg.Kind {
	case "heartbeat":
		return "Heartbeat, every " + humanInterval(cfg.IntervalSeconds)
	case "tcp":
		if t := monitorTarget(cfg); t != "" {
			return "TCP " + t
		}
	}
	return monitorTarget(cfg)
}

func humanInterval(seconds int) string {
	switch {
	case seconds%86400 == 0 && seconds >= 86400:
		return fmt.Sprintf("%dd", seconds/86400)
	case seconds%3600 == 0 && seconds >= 3600:
		return fmt.Sprintf("%dh", seconds/3600)
	case seconds%60 == 0 && seconds >= 60:
		return fmt.Sprintf("%dm", seconds/60)
	default:
		return fmt.Sprintf("%ds", seconds)
	}
}

// monitorStatusFor renders the badge ("Down 30m", "Late 4m", "Up 3d", "Paused") and detail line.
func monitorStatusFor(m db.Monitor, cfg db.CoreMonitorConfig, isCore bool, health string, since *time.Time, detail string, upSince time.Time) (text, line string) {
	now := time.Now().UTC()
	if isCore && cfg.Paused {
		return "Paused", ""
	}
	switch health {
	case "up":
		return "Up " + service.CompactDuration(now.Sub(upSince)), ""
	case "stale":
		if m.LastSuccessfulReportAt != nil {
			return "No report " + service.CompactDuration(now.Sub(*m.LastSuccessfulReportAt)), "last report " + m.LastSuccessfulReportAt.UTC().Format("15:04 UTC")
		}
		return "No report", detail
	case "down", "degraded":
		label := strings.ToUpper(health[:1]) + health[1:]
		if isCore && cfg.Kind == "heartbeat" {
			label = "Late"
		}
		if since != nil {
			return label + " " + service.CompactDuration(now.Sub(*since)), detail
		}
		return label, detail
	default:
		return "Unknown", detail
	}
}

// lastNonUpCheck is the time of each monitor's most recent failing check, for "Up 3d".
func (s *Server) lastNonUpCheck(ids []string) map[string]time.Time {
	var rows []struct {
		MonitorID string
		Last      string
	}
	s.db.Raw(`SELECT monitor_id, MAX(created_at) AS last FROM monitor_reports WHERE monitor_id IN ? AND health <> 'up' GROUP BY monitor_id`, ids).Scan(&rows)
	out := make(map[string]time.Time, len(rows))
	for _, r := range rows {
		for _, layout := range []string{"2006-01-02 15:04:05.999999999-07:00", "2006-01-02 15:04:05.999999999Z07:00", time.RFC3339Nano} {
			if t, err := time.Parse(layout, r.Last); err == nil {
				out[r.MonitorID] = t
				break
			}
		}
	}
	return out
}
