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
		r.Maintenance = maintenanceInfo(m, time.Now().UTC())
		since, detail := failureFromChecks(checks)
		r.StatusText, r.StatusDetail = monitorStatusFor(m, cfg, isCore, health, since, detail, s.upSince(m, checks))
	}
}

type recentCheck struct {
	health    string
	payload   string
	createdAt time.Time
}

// recentMonitorChecks loads the newest recentCheckCount reports per monitor, newest first. One
// index-ordered LIMIT query per monitor keeps the cost independent of how much history exists.
func (s *Server) recentMonitorChecks(ids []string) map[string][]recentCheck {
	out := make(map[string][]recentCheck, len(ids))
	for _, id := range ids {
		var rows []db.MonitorReport
		if err := s.db.Select("health", "payload", "created_at").Where("monitor_id = ?", id).Order("created_at DESC").Limit(recentCheckCount).Find(&rows).Error; err != nil {
			continue
		}
		for _, r := range rows {
			out[id] = append(out[id], recentCheck{health: r.Health, payload: r.Payload, createdAt: r.CreatedAt})
		}
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
	if m.InMaintenance(now) {
		return "Maintenance", m.MaintenanceReason
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

// upSince estimates how long a healthy monitor has been up: the newest failing check among the
// recent ones, else the last day with failures in the daily rollups (day granularity), else its age.
func (s *Server) upSince(m db.Monitor, recent []recentCheck) time.Time {
	for _, c := range recent {
		if c.health != "up" {
			return c.createdAt
		}
	}
	var last string
	s.db.Model(&db.MonitorUptimeRollup{}).Select("MAX(date)").Where("monitor_id = ? AND (down_count > 0 OR degraded_count > 0)", m.ID).Scan(&last)
	if t, err := time.Parse("2006-01-02", last); err == nil && t.AddDate(0, 0, 1).After(m.CreatedAt) {
		return t.AddDate(0, 0, 1)
	}
	return m.CreatedAt
}

// MonitorMaintenanceInfo describes an active or scheduled maintenance window.
type MonitorMaintenanceInfo struct {
	Active   bool      `json:"active"`
	StartsAt time.Time `json:"starts_at"`
	EndsAt   time.Time `json:"ends_at"`
	Reason   string    `json:"reason,omitempty"`
}

func maintenanceInfo(m db.Monitor, now time.Time) *MonitorMaintenanceInfo {
	if m.MaintenanceEndsAt == nil || !now.Before(*m.MaintenanceEndsAt) {
		return nil
	}
	start := *m.MaintenanceEndsAt
	if m.MaintenanceStartsAt != nil {
		start = *m.MaintenanceStartsAt
	}
	return &MonitorMaintenanceInfo{Active: m.InMaintenance(now), StartsAt: start, EndsAt: *m.MaintenanceEndsAt, Reason: m.MaintenanceReason}
}
