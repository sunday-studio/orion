package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"orion/core/internal/db"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// The showcase data (seed-states-*.go) exists so every screen state can be reviewed without
// creating data by hand. Everything is explicit and named, not generated, and uses a one day
// reporting interval: Core treats data as stale after five intervals, so the states survive a
// review session of days instead of turning "stale" a few minutes after seeding.
const showcaseIntervalSeconds = 86400

// farFuture parks core monitor schedules so the worker never runs showcase checks (they would fail
// against example hosts, open real incidents and fire alerts) while the seed is being reviewed.
func farFuture(now time.Time) time.Time { return now.AddDate(10, 0, 0) }

func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

// check is one report in a showcase monitor's recent history.
type check struct {
	agoMin int
	health string // up | down | degraded
	ms     int
	status int
	err    string
	extra  map[string]any
}

func showcaseReports(monitorID string, runner string, now time.Time, checks []check) []db.MonitorReport {
	reports := make([]db.MonitorReport, 0, len(checks))
	for i, c := range checks {
		at := now.Add(-time.Duration(c.agoMin) * time.Minute)
		payload := map[string]any{"seed": true, "runner": runner, "duration_ms": c.ms}
		if c.status > 0 {
			payload["status_code"] = c.status
		}
		if c.err != "" {
			payload["error"] = c.err
		}
		if c.health == "up" {
			payload["ok"] = true
		}
		for k, v := range c.extra {
			payload[k] = v
		}
		reports = append(reports, db.MonitorReport{
			ID: fmt.Sprintf("seed-state-report-%s-%02d", monitorID, i), MonitorID: monitorID, Payload: mustJSON(payload),
			CollectedAt: at.Format(time.RFC3339), Health: c.health, CreatedAt: at,
		})
	}
	return reports
}

// steady returns n healthy checks spaced step minutes apart, newest first, around baseMs.
func steady(n int, step int, baseMs int, status int) []check {
	out := make([]check, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, check{agoMin: 3 + i*step, health: "up", ms: baseMs + (i*37)%60, status: status})
	}
	return out
}

func lastCheckAt(now time.Time, checks []check) *time.Time {
	for _, c := range checks {
		if c.health == "up" || c.health == "degraded" {
			t := now.Add(-time.Duration(c.agoMin) * time.Minute)
			return &t
		}
	}
	return nil
}

// rollups writes 14 days of daily uptime for a showcase monitor with the given daily down counts.
func showcaseRollups(database *gorm.DB, monitorID string, now time.Time, downByDay map[int]int) error {
	rows := make([]db.MonitorUptimeRollup, 0, 14)
	for d := 13; d >= 0; d-- {
		day := now.AddDate(0, 0, -d).Format("2006-01-02")
		total, down := 24, downByDay[d]
		rows = append(rows, db.MonitorUptimeRollup{MonitorID: monitorID, Date: day, UpCount: total - down, DownCount: down, TotalCount: total,
			UptimePercent: float64(total-down) / float64(total) * 100, CreatedAt: now, UpdatedAt: now})
	}
	return database.Create(&rows).Error
}

func agentStats(cpu, mem, disk float64) (db.CPUStats, db.MemoryStats, db.DiskStats) {
	const gb = 1024 * 1024 * 1024
	return db.CPUStats{Cores: 8, UsagePercent: cpu, Load1: cpu / 20, Load5: cpu / 24, Load15: cpu / 28},
		db.MemoryStats{TotalBytes: 16 * gb, UsedBytes: uint64(mem / 100 * 16 * gb), FreeBytes: uint64((100 - mem) / 100 * 16 * gb), AvailableBytes: uint64((100 - mem) / 100 * 16 * gb), UsedPercent: mem},
		db.DiskStats{TotalBytes: 512 * gb, UsedBytes: uint64(disk / 100 * 512 * gb), FreeBytes: uint64((100 - disk) / 100 * 512 * gb), UsedPercent: disk}
}

// showcaseAgentReports writes 12 recent system reports (newest first) with a gentle CPU wave.
func showcaseAgentReports(agent db.Agent, cpu, mem, disk float64, uptime uint64, now time.Time, lastReportAgo time.Duration) []db.AgentReport {
	reports := make([]db.AgentReport, 0, 12)
	for i := 0; i < 12; i++ {
		at := now.Add(-lastReportAgo - time.Duration(i)*time.Hour)
		c := clamp(cpu+float64((i*7)%11)-5, 1, 100)
		cs, ms, ds := agentStats(c, mem, disk)
		reports = append(reports, db.AgentReport{ID: fmt.Sprintf("seed-state-agent-report-%s-%02d", agent.ID, i), AgentID: agent.ID, CreatedAt: at, AgentVersion: "0.9.4",
			ConfigSummary: "{}", UptimeSeconds: uptime - uint64(i)*3600, Timestamp: at.Format(time.RFC3339), CPU: datatypes.NewJSONType(cs), Memory: datatypes.NewJSONType(ms), Disk: datatypes.NewJSONType(ds), Location: agent.Location})
	}
	return reports
}
