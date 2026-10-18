package main

import (
	"fmt"
	"hash/fnv"
	"time"

	"orion/core/internal/db"

	"gorm.io/gorm"
)

// agentMonitorSpec is a monitor reported by an agent on a showcase server.
type agentMonitorSpec struct {
	server, key, name, typ string
	health                 string // up | degraded | down | stale | unknown
	checks                 []check
	runbook                string
	maintenance            string // "", active, scheduled
}

// coreMonitorSpec is a check Core runs itself (or receives, for heartbeats).
type coreMonitorSpec struct {
	key, name, kind string
	config          map[string]any
	intervalSec     int
	timeoutSec      int
	health          string
	checks          []check
	runbook         string
	paused          bool
	maintenance     string
	signalAgoMin    int // heartbeat: minutes since the last ping, 0 = never
}

func spike(base []check, at int, ms int) []check {
	out := append([]check{}, base...)
	out[at].ms = ms
	return out
}

func timeouts(base []check, at ...int) []check {
	out := append([]check{}, base...)
	for _, i := range at {
		out[i] = check{agoMin: out[i].agoMin, health: "down", ms: 10000, err: "context deadline exceeded"}
	}
	return out
}

func flapping(n int) []check {
	out := make([]check, 0, n)
	for i := 0; i < n; i++ {
		c := check{agoMin: 3 + i*20, health: "up", ms: 180 + i*11, status: 200}
		if i%2 == 1 && i > 0 {
			c = check{agoMin: 3 + i*20, health: "down", ms: 5000, status: 503}
		}
		out = append(out, c)
	}
	return out
}

func downSince(n int, status int, errMsg string) []check {
	out := make([]check, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, check{agoMin: 3 + i*15, health: "down", ms: 5000, status: status, err: errMsg})
	}
	return append(out, steady(8, 30, 120, 200)[:0]...)
}

func staleChecks(agoDays int) []check {
	base := agoDays * 24 * 60
	out := make([]check, 0, 6)
	for i := 0; i < 6; i++ {
		out = append(out, check{agoMin: base + i*60, health: "up", ms: 140 + i*9, status: 200})
	}
	return out
}

var agentMonitorSpecs = []agentMonitorSpec{
	{server: "web-up", key: "site", name: "web-prod-01 Website", typ: "website", health: "up", runbook: "https://wiki.example.com/runbooks/website", checks: spike(steady(24, 60, 140, 200), 5, 2900)},
	{server: "web-up", key: "tcp", name: "web-prod-01 TLS port", typ: "tcp", health: "up", checks: steady(12, 60, 18, 0)},
	{server: "web-up", key: "nginx", name: "nginx container", typ: "docker-container", health: "up", checks: steady(12, 60, 9, 0)},
	{server: "web-up", key: "disk", name: "Disk /", typ: "resource-threshold", health: "up", checks: steady(12, 60, 5, 0)},
	{server: "web-up", key: "ntp", name: "chrony service", typ: "systemd-service", health: "up", checks: steady(12, 60, 4, 0)},
	{server: "api-degraded", key: "api", name: "api-prod-02 Health", typ: "http-healthcheck", health: "degraded", runbook: "https://wiki.example.com/runbooks/api-latency", checks: flapping(20)},
	{server: "api-degraded", key: "worker", name: "api worker (pm2)", typ: "pm2", health: "up", checks: steady(12, 60, 11, 0)},
	{server: "api-degraded", key: "mem", name: "Memory", typ: "resource-threshold", health: "degraded", checks: append([]check{{agoMin: 3, health: "up", ms: 6}, {agoMin: 63, health: "down", ms: 6, err: "memory 96%"}, {agoMin: 123, health: "down", ms: 6, err: "memory 97%"}, {agoMin: 183, health: "up", ms: 6}, {agoMin: 243, health: "down", ms: 6, err: "memory 95%"}, {agoMin: 303, health: "up", ms: 6}}, steady(6, 60, 6, 0)...)},
	{server: "db-down", key: "pg", name: "Postgres port", typ: "tcp", health: "down", runbook: "https://wiki.example.com/runbooks/postgres-down", checks: downSince(14, 0, "connection refused")},
	{server: "db-down", key: "pgsvc", name: "postgresql service", typ: "systemd-service", health: "down", checks: downSince(14, 0, "unit failed")},
	{server: "db-down", key: "backup", name: "Nightly backup check", typ: "command", health: "down", checks: downSince(6, 0, "exit status 2")},
	{server: "cache-maint", key: "redis", name: "Redis port", typ: "tcp", health: "up", maintenance: "active", checks: steady(12, 60, 5, 0)},
	{server: "cache-maint", key: "redis2", name: "Redis service", typ: "systemd-service", health: "up", checks: steady(12, 60, 4, 0)},
	{server: "batch-stale", key: "cron", name: "Batch runner", typ: "command", health: "stale", checks: staleChecks(40)},
	{server: "batch-stale", key: "queue", name: "Batch queue port", typ: "tcp", health: "stale", checks: staleChecks(40)},
	{server: "new-unknown", key: "web", name: "edge-new-01 Website", typ: "website", health: "unknown"},
	{server: "new-unknown", key: "tcp", name: "edge-new-01 SSH", typ: "tcp", health: "unknown"},
	{server: "disk-full", key: "disk", name: "Disk /data", typ: "resource-threshold", health: "degraded", checks: append([]check{{agoMin: 3, health: "down", ms: 5, err: "disk 94%"}}, steady(11, 60, 5, 0)...)},
	{server: "idle-low", key: "pm2", name: "dev tools (pm2)", typ: "pm2", health: "up", checks: steady(12, 60, 8, 0)},
	{server: "idle-low", key: "cmd", name: "Dev DB reachable", typ: "command", health: "up", checks: steady(12, 60, 120, 0)},
	{server: "token-grace", key: "q", name: "Queue broker port", typ: "tcp", health: "up", checks: steady(12, 60, 6, 0)},
	{server: "token-rotated", key: "ci", name: "CI runner service", typ: "systemd-service", health: "up", checks: steady(12, 60, 5, 0)},
	{server: "token-revoked", key: "app", name: "Legacy app health", typ: "http-healthcheck", health: "up", checks: steady(12, 60, 260, 200)},
}

func upHealthCfg(kind string, extra map[string]any) map[string]any {
	cfg := map[string]any{"url": "https://" + kind + ".example.com/health", "method": "GET", "expected_status": 200}
	for k, v := range extra {
		cfg[k] = v
	}
	return cfg
}

var coreMonitorSpecs = []coreMonitorSpec{
	{key: "http-up", name: "Marketing site", kind: "http", config: map[string]any{"url": "https://www.example.com/", "method": "GET", "expected_status": 200}, intervalSec: 60, timeoutSec: 10, health: "up", runbook: "https://wiki.example.com/runbooks/marketing", checks: spike(steady(24, 60, 210, 200), 3, 3400)},
	{key: "http-down", name: "Checkout API", kind: "http", config: map[string]any{"url": "https://api.example.com/v1/checkout/health", "method": "GET", "expected_status": 200}, intervalSec: 60, timeoutSec: 10, health: "down", runbook: "https://wiki.example.com/runbooks/checkout", checks: append(timeouts(downSince(4, 503, ""), 1, 2), steady(20, 60, 190, 200)...)},
	{key: "auth-down", name: "Auth service", kind: "http", config: map[string]any{"url": "https://auth.example.com/health", "method": "GET", "expected_status": 200}, intervalSec: 60, timeoutSec: 10, health: "down", runbook: "https://wiki.example.com/runbooks/auth", checks: downSince(5, 500, "")},
	{key: "http-slow", name: "Reports API", kind: "http", config: map[string]any{"url": "https://reports.example.com/health", "method": "GET", "expected_status": 200}, intervalSec: 300, timeoutSec: 10, health: "degraded", checks: flapping(20)},
	{key: "keyword", name: "Pricing page keyword", kind: "http_keyword", config: map[string]any{"url": "https://www.example.com/pricing", "method": "GET", "keyword": "per month"}, intervalSec: 300, timeoutSec: 10, health: "up", checks: steady(12, 60, 260, 200)},
	{key: "tcp-up", name: "Postgres primary", kind: "tcp", config: map[string]any{"host": "db.internal.example.com", "port": 5432}, intervalSec: 60, timeoutSec: 5, health: "up", checks: steady(12, 60, 14, 0)},
	{key: "tcp-down", name: "SMTP relay", kind: "tcp", config: map[string]any{"host": "smtp.internal.example.com", "port": 587}, intervalSec: 60, timeoutSec: 5, health: "down", checks: downSince(8, 0, "connection refused")},
	{key: "hb-ok", name: "Nightly backup", kind: "heartbeat", config: map[string]any{}, intervalSec: 86400, timeoutSec: 3600, health: "up", signalAgoMin: 120, checks: steady(7, 1440, 0, 0)},
	{key: "hb-late", name: "Hourly invoice sync", kind: "heartbeat", config: map[string]any{}, intervalSec: 3600, timeoutSec: 600, health: "down", signalAgoMin: 190, checks: append([]check{{agoMin: 190, health: "up", ms: 0}}, steady(6, 60, 0, 0)[:0]...)},
	{key: "hb-wait", name: "Weekly report job", kind: "heartbeat", config: map[string]any{}, intervalSec: 604800, timeoutSec: 3600, health: "unknown"},
	{key: "dns", name: "example.com A record", kind: "dns", config: map[string]any{"host": "www.example.com", "record_type": "A", "expected": "203.0.113.10"}, intervalSec: 300, timeoutSec: 5, health: "up", checks: steady(12, 60, 22, 0)},
	{key: "tls", name: "TLS certificate www", kind: "tls", config: map[string]any{"host": "www.example.com", "port": 443}, intervalSec: 3600, timeoutSec: 10, health: "up", checks: spikeExtra(steady(6, 360, 90, 0), "tls_days_remaining", 9)},
	{key: "udp", name: "DNS resolver (UDP)", kind: "udp", config: map[string]any{"host": "10.0.0.2", "port": 53}, intervalSec: 300, timeoutSec: 5, health: "up", checks: steady(8, 60, 4, 0)},
	{key: "ping", name: "Gateway ping", kind: "ping", config: map[string]any{"host": "gw.example.com"}, intervalSec: 60, timeoutSec: 5, health: "up", checks: steady(12, 60, 3, 0)},
	{key: "domain", name: "example.com domain expiry", kind: "domain_expiration", config: map[string]any{"host": "example.com"}, intervalSec: 86400, timeoutSec: 10, health: "up", checks: spikeExtra(steady(4, 1440, 300, 0), "days_remaining", 21)},
	{key: "api", name: "Orders API request", kind: "api_request", config: map[string]any{"url": "https://api.example.com/v1/orders", "method": "POST", "expected_status": 201}, intervalSec: 300, timeoutSec: 10, health: "up", checks: steady(12, 60, 340, 201)},
	{key: "smtp", name: "Mail server SMTP", kind: "smtp", config: map[string]any{"host": "mail.example.com", "port": 587}, intervalSec: 300, timeoutSec: 10, health: "up", checks: steady(8, 60, 180, 0)},
	{key: "synthetic", name: "Signup flow (multi-step)", kind: "synthetic", config: map[string]any{"steps": 3}, intervalSec: 900, timeoutSec: 30, health: "up", checks: steady(8, 60, 2400, 0)},
	{key: "playwright", name: "Checkout browser test", kind: "playwright", config: map[string]any{"browser": "chromium"}, intervalSec: 1800, timeoutSec: 60, health: "up", checks: steady(8, 60, 5200, 0)},
	{key: "paused", name: "Staging API (paused)", kind: "http", config: map[string]any{"url": "https://staging.example.com/health", "method": "GET", "expected_status": 200}, intervalSec: 60, timeoutSec: 10, health: "up", paused: true, checks: steady(8, 60, 230, 200)},
	{key: "maint-active", name: "Billing API (maintenance now)", kind: "http", config: map[string]any{"url": "https://billing.example.com/health", "method": "GET", "expected_status": 200}, intervalSec: 60, timeoutSec: 10, health: "down", maintenance: "active", checks: downSince(6, 503, "")},
	{key: "maint-soon", name: "Docs site (maintenance scheduled)", kind: "http", config: map[string]any{"url": "https://docs.example.com/", "method": "GET", "expected_status": 200}, intervalSec: 120, timeoutSec: 10, health: "up", maintenance: "scheduled", checks: steady(12, 60, 300, 200)},
	{key: "stale", name: "Legacy status endpoint", kind: "http", config: map[string]any{"url": "https://legacy.example.com/status", "method": "GET", "expected_status": 200}, intervalSec: 300, timeoutSec: 10, health: "stale", checks: staleChecks(30)},
}

func spikeExtra(base []check, key string, value any) []check {
	out := append([]check{}, base...)
	for i := range out {
		out[i].extra = map[string]any{key: value}
	}
	return out
}

func maintenanceWindow(kind string, now time.Time) (*time.Time, *time.Time, string) {
	switch kind {
	case "active":
		return ptrTime(now.Add(-30 * time.Minute)), ptrTime(now.Add(90 * time.Minute)), "Database migration"
	case "scheduled":
		return ptrTime(now.Add(26 * time.Hour)), ptrTime(now.Add(28 * time.Hour)), "Docs platform upgrade"
	}
	return nil, nil, ""
}

func baseMonitor(id, name, typ, agentID, health string, interval int, checks []check, now time.Time) db.Monitor {
	created := now.AddDate(0, 0, -60)
	m := db.Monitor{ID: id, Type: typ, Name: name, AgentID: agentID, ReportingIntervalSeconds: interval, ComputedHealth: health, LastHealthComputation: ptrTime(now.Add(-time.Minute)),
		Lifecycle: "active", Health: health, IncidentState: incidentState(health), LastSuccessfulReportAt: lastCheckAt(now, checks), CreatedAt: created, UpdatedAt: now}
	if len(checks) == 0 {
		m.CreatedAt, m.UpdatedAt = now.Add(-10*time.Minute), now.Add(-10*time.Minute)
		m.ComputedHealth, m.Health, m.LastHealthComputation = "unknown", "unknown", nil
	}
	return m
}

func seedShowcaseMonitors(database *gorm.DB, agents map[string]db.Agent, now time.Time) ([]db.Monitor, error) {
	var created []db.Monitor
	save := func(m db.Monitor, runner string, checks []check) error {
		if err := database.Create(&m).Error; err != nil {
			return err
		}
		created = append(created, m)
		if len(checks) > 0 {
			if err := database.CreateInBatches(showcaseReports(m.ID, runner, now, checks), 100).Error; err != nil {
				return err
			}
		}
		if len(checks) == 0 {
			return nil
		}
		return showcaseRollups(database, m.ID, now, rollupFailures(m.ID, m.Health))
	}
	for _, spec := range agentMonitorSpecs {
		agent := agents[spec.server]
		m := baseMonitor("seed-state-monitor-"+spec.server+"-"+spec.key, spec.name, spec.typ, agent.ID, spec.health, showcaseIntervalSeconds, spec.checks, now)
		m.RunbookURL = spec.runbook
		m.MaintenanceStartsAt, m.MaintenanceEndsAt, m.MaintenanceReason = maintenanceWindow(spec.maintenance, now)
		if err := save(m, "agent", spec.checks); err != nil {
			return nil, err
		}
	}
	for _, spec := range showcaseServers {
		if !spec.removed {
			continue
		}
		deleted := baseMonitor("seed-state-monitor-removed-"+spec.key, "old-worker-07 Worker", "pm2", showcaseAgentID(spec.key), "unknown", showcaseIntervalSeconds, nil, now)
		deleted.Lifecycle, deleted.DeletedAt = "deleted", now.AddDate(0, 0, -20)
		if err := database.Create(&deleted).Error; err != nil {
			return nil, err
		}
	}
	for _, spec := range coreMonitorSpecs {
		m := baseMonitor("seed-state-core-"+spec.key, spec.name, spec.kind, "seed-agent-core", spec.health, spec.intervalSec, spec.checks, now)
		m.RunbookURL = spec.runbook
		m.MaintenanceStartsAt, m.MaintenanceEndsAt, m.MaintenanceReason = maintenanceWindow(spec.maintenance, now)
		if spec.health == "stale" {
			m.ReportingIntervalSeconds = showcaseIntervalSeconds
		}
		runner := "core"
		if spec.kind == "heartbeat" {
			runner = "heartbeat"
		}
		if err := save(m, runner, spec.checks); err != nil {
			return nil, err
		}
		cfg := db.CoreMonitorConfig{MonitorID: m.ID, Kind: spec.kind, ConfigJSON: mustJSON(spec.config), SecretRefJSON: "{}", IntervalSeconds: spec.intervalSec, TimeoutSeconds: spec.timeoutSec,
			Paused: spec.paused, NextRunAt: farFuture(now), CreatedAt: m.CreatedAt, UpdatedAt: now}
		if spec.kind == "heartbeat" {
			cfg.HeartbeatTokenHash, cfg.HeartbeatTokenPrefix = sha256Hex("seed-heartbeat-"+spec.key), fmt.Sprintf("hb_%s", spec.key[:2])
			if spec.health == "down" { // a missed ping already recorded, so the worker does not file another
				cfg.LastFailureAt = ptrTime(now.Add(-time.Duration(spec.signalAgoMin-60) * time.Minute))
			}
			if spec.signalAgoMin > 0 {
				cfg.LastSignalAt, cfg.LastSuccessAt = ptrTime(now.Add(-time.Duration(spec.signalAgoMin)*time.Minute)), ptrTime(now.Add(-time.Duration(spec.signalAgoMin)*time.Minute))
			}
		} else if len(spec.checks) > 0 {
			cfg.LastRunAt = ptrTime(now.Add(-time.Duration(spec.checks[0].agoMin) * time.Minute))
			cfg.LastSuccessAt = lastCheckAt(now, spec.checks)
		}
		if spec.paused {
			cfg.PausedAt, cfg.PausedBy = ptrTime(now.AddDate(0, 0, -2)), "admin"
		}
		if err := database.Create(&cfg).Error; err != nil {
			return nil, err
		}
	}
	return created, seedAgentMonitorDefinitions(database, now)
}

// seedAgentMonitorDefinitions covers the Agent monitor setup states: reporting and waiting.
func seedAgentMonitorDefinitions(database *gorm.DB, now time.Time) error {
	defs := []db.AgentMonitorDefinition{
		{ID: "seed-state-def-disk", AgentID: showcaseAgentID("web-up"), Name: "Disk /", CheckType: "disk", Params: mustJSON(map[string]any{"path": "/", "max_used_percent": 90}), IntervalSeconds: 60, FailAfter: 3, CreatedBy: "admin", CreatedAt: now.AddDate(0, 0, -30)},
		{ID: "seed-state-def-mem", AgentID: showcaseAgentID("api-degraded"), Name: "Memory", CheckType: "memory", Params: mustJSON(map[string]any{"max_used_percent": 90}), IntervalSeconds: 30, FailAfter: 3, CreatedBy: "admin", CreatedAt: now.AddDate(0, 0, -10)},
		{ID: "seed-state-def-port", AgentID: showcaseAgentID("db-down"), Name: "Postgres port", CheckType: "port", Params: mustJSON(map[string]any{"host": "127.0.0.1", "port": 5432}), IntervalSeconds: 30, FailAfter: 2, CreatedBy: "admin", CreatedAt: now.AddDate(0, 0, -45)},
		{ID: "seed-state-def-proc", AgentID: showcaseAgentID("web-up"), Name: "nginx process", CheckType: "process", Params: mustJSON(map[string]any{"name": "nginx", "min_count": 1}), IntervalSeconds: 30, FailAfter: 3, CreatedBy: "admin", CreatedAt: now.Add(-4 * time.Minute)},
	}
	return database.Create(&defs).Error
}

// rollupFailures gives each monitor its own failure history, so "Up 3d" / "Up 9d" / "Up 60d" differ.
func rollupFailures(id, health string) map[int]int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(id))
	v := int(h.Sum32())
	out := map[int]int{}
	if health == "down" || health == "degraded" {
		out[0] = 3 + v%5
	}
	switch v % 4 {
	case 0: // clean history
	case 1:
		out[2+v%11] = 1 + v%4
	default:
		out[2+v%11], out[1+(v/7)%12] = 2, 1
	}
	return out
}
