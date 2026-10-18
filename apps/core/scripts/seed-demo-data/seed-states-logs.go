package main

import (
	"fmt"
	"path/filepath"
	"time"

	"orion/core/internal/db"
	"orion/core/internal/logstore"
	"orion/core/internal/utils"

	"gorm.io/gorm"
)

type logAppSpec struct {
	id, name, env, description  string
	retentionDays, maxStorageMB int
	sample                      float64
	rateLimit                   int
	display, redaction          string
	lastIngestAgo               time.Duration // zero = never
}

var logAppSpecs = []logAppSpec{
	{id: "seed-app-billing-api", name: "billing-api", env: "production", description: "Payments and invoices", retentionDays: 14, maxStorageMB: 5, sample: 0.1, rateLimit: 500,
		display:   `{"title_field":"event","columns":["user.id","http.status","duration_ms"],"hidden":["internal.*","trace.flags"]}`,
		redaction: `{"rules":[{"type":"key","pattern":"card_number","replace_with":"[redacted]"},{"type":"regex","pattern":"\\b\\d{16}\\b","replace_with":"[card]"}]}`, lastIngestAgo: 2 * time.Second},
	{id: "seed-app-web-store", name: "web-store", env: "production", description: "Storefront", retentionDays: 14, sample: 1, display: `{}`, redaction: `{}`, lastIngestAgo: time.Second},
	{id: "seed-app-legacy-cron", name: "legacy-cron", env: "production", description: "Shell cron jobs, plain text only", retentionDays: 30, sample: 1, display: `{}`, redaction: `{}`, lastIngestAgo: 3 * time.Hour},
	{id: "seed-app-edge-gateway", name: "edge-gateway", env: "production", description: "API gateway with structured bodies", retentionDays: 7, maxStorageMB: 50, sample: 1, display: `{"columns":["upstream","status"]}`, redaction: `{"extra_keys":["x-internal-key"]}`, lastIngestAgo: 40 * time.Second},
	{id: "seed-app-umami", name: "umami", env: "staging", description: "Analytics, not connected yet", retentionDays: 14, sample: 1, display: `{}`, redaction: `{}`},
}

type logKeySpec struct {
	id, app, name string
	usedAgo       time.Duration // 0 = never
	revokedAgo    time.Duration // negative = in the future (rotation grace)
	createdAgoDay int
}

var logKeySpecs = []logKeySpec{
	{id: "seed-key-billing-prod", app: "billing-api", name: "production server", usedAgo: 2 * time.Second, createdAgoDay: 14},
	{id: "seed-key-billing-ci", app: "billing-api", name: "ci tests", usedAgo: 72 * time.Hour, createdAgoDay: 40},
	{id: "seed-key-billing-old", app: "billing-api", name: "old laptop", usedAgo: 11 * 24 * time.Hour, revokedAgo: 10 * 24 * time.Hour, createdAgoDay: 90},
	{id: "seed-key-billing-rotating", app: "billing-api", name: "worker (rotating)", usedAgo: time.Minute, revokedAgo: -8 * time.Minute, createdAgoDay: 30},
	{id: "seed-key-web", app: "web-store", name: "storefront", usedAgo: time.Second, createdAgoDay: 20},
	{id: "seed-key-cron", app: "legacy-cron", name: "cron host", usedAgo: 3 * time.Hour, createdAgoDay: 60},
	{id: "seed-key-edge", app: "edge-gateway", name: "gateway", usedAgo: 40 * time.Second, createdAgoDay: 9},
	{id: "seed-key-umami", app: "umami", name: "staging", createdAgoDay: 1},
}

func appIDFor(name string) string {
	for _, a := range logAppSpecs {
		if a.name == name {
			return a.id
		}
	}
	return ""
}

// openSeedLogStore opens logs.db next to the main database.
func openSeedLogStore(dbPath string) (*logstore.Store, error) {
	handle, err := db.OpenFile(filepath.Dir(dbPath), "logs.db", "warn")
	if err != nil {
		return nil, err
	}
	return logstore.OpenWith(handle, utils.NewLogger(), 0)
}

// resetSeedLogs removes previously seeded log lines (their ids start with seed-).
func resetSeedLogs(dbPath string) error {
	store, err := openSeedLogStore(dbPath)
	if err != nil {
		return err
	}
	defer store.Close()
	return store.DB().Exec("DELETE FROM log_entries WHERE id LIKE 'seed-%'").Error
}

func seedShowcaseLogs(database *gorm.DB, dbPath string, now time.Time) error {
	for _, a := range logAppSpecs {
		app := db.App{ID: a.id, Name: a.name, Slug: a.name, Environment: a.env, Description: a.description, RetentionDays: a.retentionDays, MaxStorageMB: a.maxStorageMB, DebugSampleRate: a.sample,
			RateLimitPerSecond: a.rateLimit, DisplayJSON: a.display, RedactionJSON: a.redaction, CreatedAt: now.AddDate(0, 0, -30), UpdatedAt: now}
		if a.lastIngestAgo > 0 {
			app.LastIngestAt = ptrTime(now.Add(-a.lastIngestAgo))
		}
		if err := database.Create(&app).Error; err != nil {
			return err
		}
	}
	for _, k := range logKeySpecs {
		key := db.AppIngestKey{ID: k.id, AppID: appIDFor(k.app), Name: k.name, Prefix: "orion_ik_" + sha256Hex(k.id)[:8], KeyHash: sha256Hex("seed-ingest-" + k.id), CreatedAt: now.AddDate(0, 0, -k.createdAgoDay)}
		if k.usedAgo > 0 {
			key.LastUsedAt = ptrTime(now.Add(-k.usedAgo))
		}
		if k.revokedAgo != 0 {
			key.RevokedAt = ptrTime(now.Add(-k.revokedAgo))
		}
		if err := database.Create(&key).Error; err != nil {
			return err
		}
	}
	views := []db.LogSavedView{
		{ID: "seed-view-errors", Name: "Errors, last hour", FiltersJSON: `{"severity":"error","from":"-1h"}`, CreatedBy: "admin", CreatedAt: now.AddDate(0, 0, -6)},
		{ID: "seed-view-payments", Name: "Failed payments", FiltersJSON: `{"app_id":"` + appIDFor("billing-api") + `","event":"payment.failed","from":"-24h"}`, CreatedBy: "admin", CreatedAt: now.AddDate(0, 0, -5)},
		{ID: "seed-view-agent", Name: "Agent warnings", FiltersJSON: `{"source":"agent","severity":">=warn"}`, CreatedBy: "admin", CreatedAt: now.AddDate(0, 0, -3)},
		{ID: "seed-view-cron", Name: "Cron failures", FiltersJSON: `{"app_id":"` + appIDFor("legacy-cron") + `","severity":"error"}`, CreatedBy: "ci-deploys", CreatedAt: now.AddDate(0, 0, -1)},
	}
	if err := database.Create(&views).Error; err != nil {
		return err
	}
	rules := []db.LogAlertRule{
		{ID: "seed-log-rule-silence", Name: "Cron went silent", AppID: appIDFor("legacy-cron"), Mode: "absence", Event: "", WindowSeconds: 90000, Enabled: false, State: "ok", CreatedAt: now.AddDate(0, 0, -4), UpdatedAt: now},
	}
	if err := database.Create(&rules).Error; err != nil {
		return err
	}
	if err := database.Model(&db.LogAlertRule{}).Where("id = ?", "seed-log-rule-silence").Update("enabled", false).Error; err != nil {
		return err
	}
	store, err := openSeedLogStore(dbPath)
	if err != nil {
		return err
	}
	defer store.Close()
	entries := append(append(append(billingEntries(now), webStoreEntries(now)...), cronEntries(now)...), gatewayEntries(now)...)
	entries = append(entries, internalEntries(now)...)
	for start := 0; start < len(entries); start += 1000 {
		if err := store.Enqueue(entries[start:min(start+1000, len(entries))]); err != nil {
			return fmt.Errorf("seed log entries: %w", err)
		}
		store.Flush(30 * time.Second)
	}
	store.Flush(30 * time.Second)
	return seedAgentServiceLogs(database, now)
}

// seedAgentServiceLogs gives the agent source of the unified Logs page a few lines at each level.
func seedAgentServiceLogs(database *gorm.DB, now time.Time) error {
	type line struct {
		level, component, message string
	}
	lines := []line{{"INFO", "reporter", "report accepted"}, {"INFO", "collector", "collected 5 monitors"}, {"WARN", "reporter", "slow report: latency 1204ms"}, {"WARN", "collector", "disk usage 91% on /var/lib/postgresql"},
		{"ERROR", "reporter", "report rejected: 503 from Core"}, {"INFO", "updater", "agent is up to date"}, {"ERROR", "collector", "docker socket not reachable"}, {"INFO", "collector", "retrying docker socket"}}
	var rows []db.ServiceLogEntry
	for i := 0; i < 48; i++ {
		l := lines[i%len(lines)]
		at := now.Add(-time.Duration(i*17) * time.Minute)
		rows = append(rows, db.ServiceLogEntry{ID: fmt.Sprintf("seed-svc-log-%02d", i), AgentID: showcaseAgentID("web-up"), Source: "agent", Stream: "jsonl", Level: l.level, Component: l.component, Message: l.message,
			FieldsJSON: `{}`, Raw: l.message, Fingerprint: fmt.Sprintf("seed-fp-%02d", i), OccurredAt: at, CollectedAt: at, CreatedAt: at})
	}
	return database.CreateInBatches(rows, 50).Error
}
