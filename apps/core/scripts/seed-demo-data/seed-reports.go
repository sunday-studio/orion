package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"orion/core/internal/db"

	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func seedAgentReports(database *gorm.DB, agent db.Agent, sc scenario, cfg seedConfig, start time.Time, now time.Time) (int, error) {
	reports := make([]db.AgentReport, 0, cfg.days)
	stop := now
	if sc.stale {
		stop = now.Add(-time.Duration(45+len(sc.key)%72) * time.Minute)
	}
	for t := start; !t.After(stop); t = t.Add(cfg.reportInterval) {
		cpu, memory, disk := systemStats(sc, t)
		report := db.AgentReport{
			ID:            fmt.Sprintf("seed-agent-report-%s-%d", agent.ID, t.Unix()),
			AgentID:       agent.ID,
			CreatedAt:     t,
			AgentVersion:  fmt.Sprintf("seed-%s", sc.key),
			ConfigSummary: mustJSON(map[string]any{"monitor_count": len(monitorTemplates), "reporting_interval": cfg.reportInterval.String(), "scenario": sc.key}),
			UptimeSeconds: uint64(math.Max(0, now.Sub(t).Seconds())) + 3600,
			Timestamp:     t.Format(time.RFC3339),
			CPU:           datatypes.NewJSONType(cpu),
			Memory:        datatypes.NewJSONType(memory),
			Disk:          datatypes.NewJSONType(disk),
			Location:      agent.Location,
		}
		reports = append(reports, report)
	}
	return bulkCreate(database, reports, 1000)
}

func seedMonitorReports(database *gorm.DB, monitor db.Monitor, sc scenario, tpl monitorTemplate, cfg seedConfig, start time.Time, now time.Time) (map[string]dayCounts, int, error) {
	reports := make([]db.MonitorReport, 0, cfg.days*24)
	counts := map[string]dayCounts{}
	stop := now
	if sc.stale {
		stop = now.Add(-time.Duration(45+len(sc.key)%72) * time.Minute)
	}
	for t := start; !t.After(stop); t = t.Add(cfg.reportInterval) {
		health := reportHealth(sc, tpl, t, now)
		payload := reportPayload(sc, tpl, health, t, now)
		report := db.MonitorReport{
			ID:          fmt.Sprintf("seed-monitor-report-%s-%d", monitor.ID, t.Unix()),
			MonitorID:   monitor.ID,
			Payload:     mustJSON(payload),
			CollectedAt: t.Format(time.RFC3339),
			Health:      health,
			CreatedAt:   t,
		}
		reports = append(reports, report)
		day := t.Format("2006-01-02")
		c := counts[day]
		switch health {
		case "up":
			c.up++
		case "down":
			c.down++
		case "degraded":
			c.degraded++
		default:
			c.unknown++
		}
		counts[day] = c
	}
	created, err := bulkCreate(database, reports, 1000)
	return counts, created, err
}

func seedRollups(database *gorm.DB, monitorID string, counts map[string]dayCounts, now time.Time) (int, error) {
	rollups := make([]db.MonitorUptimeRollup, 0, len(counts))
	for day, c := range counts {
		total := c.up + c.down + c.degraded + c.unknown
		percent := 0.0
		if total > 0 {
			percent = 100 * float64(c.up) / float64(total)
		}
		rollups = append(rollups, db.MonitorUptimeRollup{
			MonitorID:     monitorID,
			Date:          day,
			UpCount:       c.up,
			DownCount:     c.down,
			DegradedCount: c.degraded,
			UnknownCount:  c.unknown,
			TotalCount:    total,
			UptimePercent: percent,
			CreatedAt:     now,
			UpdatedAt:     now,
		})
	}
	result := database.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "monitor_id"}, {Name: "date"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"up_count", "down_count", "degraded_count", "unknown_count", "total_count", "uptime_percent", "updated_at",
		}),
	}).CreateInBatches(rollups, 1000)
	return len(rollups), result.Error
}

func currentHealth(sc scenario, tpl monitorTemplate) string {
	if sc.maintenance {
		if tpl.key == "resource" || tpl.key == "http" {
			return "degraded"
		}
		return "up"
	}
	if sc.stale {
		return "unknown"
	}
	switch scenarioBaseKey(sc) {
	case "healthy":
		return "up"
	case "degraded", "flapping", "tls", "resource", "maintenance":
		if tpl.key == "website" || tpl.key == "resource" || tpl.key == "http" {
			return "degraded"
		}
		return "up"
	case "down", "alerts":
		if tpl.key == "http" || tpl.key == "website" || tpl.key == "internal" {
			return "down"
		}
		if tpl.key == "resource" {
			return "degraded"
		}
		return "up"
	case "stale", "unknown":
		return "unknown"
	default:
		return "up"
	}
}

func reportHealth(sc scenario, tpl monitorTemplate, t time.Time, now time.Time) string {
	if sc.maintenance {
		if t.After(now.Add(-48 * time.Hour)) {
			return "degraded"
		}
		return "up"
	}
	switch scenarioBaseKey(sc) {
	case "healthy":
		if t.Hour() == 3 && t.Day()%17 == 0 {
			return "degraded"
		}
		return "up"
	case "degraded":
		if tpl.key == "resource" || tpl.key == "http" || t.Hour()%6 == 0 {
			return "degraded"
		}
		return "up"
	case "down":
		if t.After(now.Add(-36 * time.Hour)) {
			return "down"
		}
		if t.Day()%13 == 0 && t.Hour() < 3 {
			return "down"
		}
		return "up"
	case "stale":
		return "up"
	case "flapping":
		if t.Hour()%2 == 0 {
			return "down"
		}
		if t.Hour()%3 == 0 {
			return "degraded"
		}
		return "up"
	case "tls":
		if tpl.key == "website" && t.After(now.AddDate(0, 0, -14)) {
			return "degraded"
		}
		return "up"
	case "resource":
		if tpl.key == "resource" || t.Hour() >= 18 {
			return "degraded"
		}
		return "up"
	case "alerts":
		if t.After(now.Add(-24 * time.Hour)) {
			return "down"
		}
		if t.Day()%9 == 0 {
			return "down"
		}
		return "up"
	default:
		return "up"
	}
}

func reportPayload(sc scenario, tpl monitorTemplate, health string, t time.Time, now time.Time) map[string]any {
	baseKey := scenarioBaseKey(sc)
	payload := map[string]any{
		"seed":        true,
		"scenario":    sc.key,
		"monitor_key": tpl.key,
		"summary":     fmt.Sprintf("%s %s at %s", tpl.kind, health, t.Format(time.RFC3339)),
	}
	latency := 20 + (t.Hour()*7)%400
	payload["latency_ms"] = latency
	if health == "up" {
		payload["ok"] = true
	}
	if health == "degraded" {
		payload["warning"] = "threshold crossed"
		payload["latency_ms"] = latency + 900
	}
	if health == "down" {
		payload["error"] = "seeded outage"
		payload["exit_code"] = 1
	}
	switch tpl.key {
	case "http":
		payload["status_code"] = choose(health == "down", 503, 200)
		payload["expected_status"] = 200
		payload["body_contains"] = choose(health == "down", false, true)
	case "website":
		payload["status_code"] = choose(health == "down", 502, 200)
		payload["dns_lookup_ms"] = 12 + t.Hour()%40
		payload["resolved_ip"] = "203.0.113.10"
		daysRemaining := int(now.Sub(t).Hours() / 24)
		if baseKey == "tls" {
			daysRemaining = 14 - int(now.Sub(t).Hours()/24)
			if daysRemaining < 1 {
				daysRemaining = 1
			}
		} else {
			daysRemaining = 60 + daysRemaining%30
		}
		payload["tls_days_remaining"] = daysRemaining
	case "tcp":
		payload["host"] = "127.0.0.1"
		payload["port"] = 22
		payload["connected"] = health != "down"
	case "resource":
		cpu, memory, disk := systemStats(sc, t)
		payload["cpu_usage_percent"] = cpu.UsagePercent
		payload["memory_used_percent"] = memory.UsedPercent
		payload["disk_used_percent"] = disk.UsedPercent
		payload["load_1"] = cpu.Load1
	case "docker":
		payload["container_name"] = "seed-app"
		payload["state"] = choose(health == "down", "exited", "running")
		payload["restart_count"] = t.Day() % 7
	case "systemd":
		payload["unit"] = "seed.service"
		payload["active_state"] = choose(health == "down", "failed", "active")
	case "pm2":
		payload["app_name"] = "seed-api"
		payload["status"] = choose(health == "down", "errored", "online")
	case "command":
		payload["command"] = "test -f /tmp/seed-ok"
		payload["stdout"] = choose(health == "down", "", "ok")
		payload["stderr"] = choose(health == "down", "missing marker", "")
	case "internal":
		payload["ping_ok"] = health != "down"
		payload["process_port"] = 8999
		payload["process_running"] = health != "down"
	}
	return payload
}

func systemStats(sc scenario, t time.Time) (db.CPUStats, db.MemoryStats, db.DiskStats) {
	wave := float64((t.Hour() + t.Day()) % 24)
	cpuUsed := 15 + wave*2
	memUsed := 40 + wave
	diskUsed := 55 + float64(t.Day()%20)
	switch scenarioBaseKey(sc) {
	case "resource":
		cpuUsed = 88 + float64(t.Hour()%10)
		memUsed = 91
		diskUsed = 93
	case "degraded":
		cpuUsed += 25
		memUsed += 15
	}
	return db.CPUStats{
			Cores:        8,
			UsagePercent: clamp(cpuUsed, 0, 100),
			Load1:        clamp(cpuUsed/20, 0, 12),
			Load5:        clamp(cpuUsed/25, 0, 12),
			Load15:       clamp(cpuUsed/30, 0, 12),
		}, db.MemoryStats{
			TotalBytes:     16 * 1024 * 1024 * 1024,
			UsedBytes:      uint64(memUsed / 100 * 16 * 1024 * 1024 * 1024),
			FreeBytes:      uint64((100 - memUsed) / 100 * 16 * 1024 * 1024 * 1024),
			AvailableBytes: uint64((100 - memUsed) / 100 * 16 * 1024 * 1024 * 1024),
			UsedPercent:    clamp(memUsed, 0, 100),
		}, db.DiskStats{
			TotalBytes:  512 * 1024 * 1024 * 1024,
			UsedBytes:   uint64(diskUsed / 100 * 512 * 1024 * 1024 * 1024),
			FreeBytes:   uint64((100 - diskUsed) / 100 * 512 * 1024 * 1024 * 1024),
			UsedPercent: clamp(diskUsed, 0, 100),
		}
}

func templateByMonitor(monitor db.Monitor) monitorTemplate {
	for _, tpl := range monitorTemplates {
		if strings.HasSuffix(monitor.ID, "-"+tpl.key) {
			return tpl
		}
	}
	return monitorTemplate{key: "unknown", kind: monitor.Type, name: monitor.Name, intervalSec: monitor.ReportingIntervalSeconds}
}

func edgeCase(sc scenario, tpl monitorTemplate) string {
	baseKey := scenarioBaseKey(sc)
	if baseKey == "tls" && tpl.key == "website" {
		return "tls_expiring"
	}
	if baseKey == "resource" && tpl.key == "resource" {
		return "resource_threshold"
	}
	if baseKey == "flapping" {
		return "flapping"
	}
	if sc.stale {
		return "stale_server"
	}
	return sc.status
}

func scenarioBaseKey(sc scenario) string {
	key := sc.key
	if idx := strings.LastIndex(key, "-"); idx > 0 && idx+1 < len(key) {
		suffix := key[idx+1:]
		if _, err := strconv.Atoi(suffix); err == nil {
			return key[:idx]
		}
	}
	return key
}
