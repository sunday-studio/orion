package main

import (
	"path/filepath"
	"testing"
	"time"

	"orion/core/internal/db"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func seedForTest(t *testing.T, database *gorm.DB) {
	t.Helper()
	cfg := seedConfig{dbPath: filepath.Join(t.TempDir(), "x.db"), days: 3, agents: 10, reportInterval: 6 * time.Hour, resetSeed: true, updateSettings: true}
	if err := resetSeedData(database); err != nil {
		t.Fatalf("reset: %v", err)
	}
	if _, err := seed(database, cfg); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func count(t *testing.T, database *gorm.DB, query string, args ...any) int {
	t.Helper()
	var n int
	if err := database.Raw(query, args...).Scan(&n).Error; err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

// TestSeedCoversEveryState is the review contract: each screen state the console renders has data,
// and seeding twice (reset in between) neither fails nor duplicates anything.
func TestSeedCoversEveryState(t *testing.T) {
	database, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(database); err != nil {
		t.Fatal(err)
	}
	seedForTest(t, database)
	first := count(t, database, "SELECT COUNT(*) FROM monitors")
	seedForTest(t, database)
	if again := count(t, database, "SELECT COUNT(*) FROM monitors"); again != first {
		t.Fatalf("second seed changed the monitor count: %d -> %d", first, again)
	}

	atLeast := func(name string, query string, min int, args ...any) {
		t.Helper()
		if n := count(t, database, query, args...); n < min {
			t.Errorf("%s: %d, want at least %d", name, n, min)
		}
	}
	// Servers.
	atLeast("labels and notes", "SELECT COUNT(*) FROM agents WHERE id LIKE 'seed-state-%' AND labels <> '[]' AND notes <> ''", 3)
	atLeast("maintenance server", "SELECT COUNT(*) FROM agents WHERE id LIKE 'seed-state-%' AND maintenance_mode = 1", 1)
	atLeast("rotation grace token", "SELECT COUNT(*) FROM agents WHERE previous_token_hash <> '' AND previous_token_expires_at > ?", 1, time.Now().UTC())
	atLeast("revoked token", "SELECT COUNT(*) FROM agents WHERE token_revoked_at IS NOT NULL AND token_revocation_reason <> '' AND deleted_at IS NOT NULL AND id LIKE 'seed-state-%'", 1)
	atLeast("enrollment states", "SELECT COUNT(DISTINCT CASE WHEN claimed_at IS NOT NULL THEN 'c' WHEN cancelled_at IS NOT NULL THEN 'x' WHEN expires_at < ? THEN 'e' ELSE 'w' END) FROM enrollment_sessions", 4, time.Now().UTC())
	// Monitors.
	atLeast("monitor kinds", "SELECT COUNT(DISTINCT kind) FROM core_monitor_configs WHERE monitor_id LIKE 'seed-state-%'", 12)
	atLeast("agent monitor types", "SELECT COUNT(DISTINCT type) FROM monitors WHERE id LIKE 'seed-state-monitor-%'", 7)
	atLeast("paused", "SELECT COUNT(*) FROM core_monitor_configs WHERE paused = 1", 1)
	atLeast("active maintenance window", "SELECT COUNT(*) FROM monitors WHERE maintenance_starts_at <= ? AND maintenance_ends_at > ?", 1, time.Now().UTC(), time.Now().UTC())
	atLeast("scheduled maintenance window", "SELECT COUNT(*) FROM monitors WHERE maintenance_starts_at > ?", 1, time.Now().UTC())
	atLeast("runbook", "SELECT COUNT(*) FROM monitors WHERE runbook_url <> ''", 3)
	atLeast("timeouts", "SELECT COUNT(*) FROM monitor_reports WHERE payload LIKE '%deadline exceeded%'", 2)
	atLeast("never reported", "SELECT COUNT(*) FROM monitors WHERE id LIKE 'seed-state-%' AND lifecycle = 'active' AND id NOT IN (SELECT monitor_id FROM monitor_reports)", 3)
	atLeast("heartbeat never signaled", "SELECT COUNT(*) FROM core_monitor_configs WHERE kind = 'heartbeat' AND last_signal_at IS NULL", 1)
	atLeast("agent monitor definitions", "SELECT COUNT(*) FROM agent_monitor_definitions WHERE id LIKE 'seed-state-%'", 4)
	// Incidents.
	for _, status := range []string{"open", "acknowledged", "covered", "resolved"} {
		atLeast("incident "+status, "SELECT COUNT(*) FROM incidents WHERE id LIKE 'seed-state-%' AND status = ?", 1, status)
	}
	for _, sev := range []string{"critical", "high", "medium", "low"} {
		atLeast("severity "+sev, "SELECT COUNT(*) FROM incidents WHERE id LIKE 'seed-state-%' AND severity = ?", 1, sev)
	}
	atLeast("resolution kinds", "SELECT COUNT(DISTINCT resolution_kind) FROM incidents WHERE id LIKE 'seed-state-%' AND resolution_kind <> ''", 4)
	atLeast("reopened", "SELECT COUNT(*) FROM incidents WHERE id LIKE 'seed-state-%' AND reopen_count > 0", 1)
	atLeast("declared", "SELECT COUNT(*) FROM incidents WHERE id LIKE 'seed-state-%' AND source = 'manual'", 2)
	atLeast("covered by another incident", "SELECT COUNT(*) FROM incidents WHERE covered_by_incident_id <> ''", 1)
	atLeast("dismissed candidate", "SELECT COUNT(*) FROM incident_candidate_dismissals WHERE monitor_id LIKE 'seed-state-%'", 1)
	atLeast("notes", "SELECT COUNT(*) FROM incident_events WHERE id LIKE 'seed-state-%' AND type = 'incident_note'", 3)
	// Alerts.
	atLeast("disabled channel", "SELECT COUNT(*) FROM alert_channels WHERE id LIKE 'seed-state-%' AND enabled = 0", 1)
	atLeast("digest channel", "SELECT COUNT(*) FROM alert_channels WHERE email_delivery_mode = 'digest_15m'", 1)
	atLeast("recipient states", "SELECT COUNT(DISTINCT status) FROM alert_channel_recipients", 3)
	for _, status := range []string{"sent", "failed", "pending", "suppressed", "cooldown"} {
		atLeast("delivery "+status, "SELECT COUNT(*) FROM alert_deliveries WHERE id LIKE 'seed-state-%' AND status = ?", 1, status)
	}
	atLeast("retried then sent", "SELECT COUNT(*) FROM alert_deliveries WHERE id LIKE 'seed-state-%' AND status = 'sent' AND attempt_count > 1", 2)
	atLeast("exhausted retries", "SELECT COUNT(*) FROM alert_deliveries WHERE id LIKE 'seed-state-%' AND status = 'failed' AND attempt_count >= max_attempts", 1)
	atLeast("disabled rule", "SELECT COUNT(*) FROM alert_routes WHERE enabled = 0", 1)
	atLeast("suppress rule", "SELECT COUNT(*) FROM alert_routes WHERE suppress = 1", 1)
	atLeast("rule cooldown", "SELECT COUNT(*) FROM alert_routes WHERE cooldown_seconds > 0", 2)
	atLeast("disabled label route", "SELECT COUNT(*) FROM alert_label_routes WHERE enabled = 0", 1)
	// Status pages.
	for _, domain := range []string{"pending", "verified", "failed"} {
		atLeast("domain "+domain, "SELECT COUNT(*) FROM status_pages WHERE custom_domain <> '' AND domain_status = ?", 1, domain)
	}
	for _, vis := range []string{"draft", "public", "unlisted"} {
		atLeast("page "+vis, "SELECT COUNT(*) FROM status_pages WHERE visibility = ?", 1, vis)
	}
	atLeast("page without components", "SELECT COUNT(*) FROM status_pages WHERE id NOT IN (SELECT status_page_id FROM status_page_components)", 1)
	atLeast("component statuses", "SELECT COUNT(DISTINCT manual_status) FROM status_page_components WHERE manual_status <> ''", 5)
	atLeast("hidden component", "SELECT COUNT(*) FROM status_page_components WHERE visible = 0", 1)
	for _, status := range []string{"investigating", "identified", "monitoring", "resolved", "scheduled"} {
		atLeast("public incident "+status, "SELECT COUNT(*) FROM status_page_incidents WHERE public_status = ? AND visibility = 'published'", 1, status)
	}
	atLeast("draft and private public incidents", "SELECT COUNT(DISTINCT visibility) FROM status_page_incidents WHERE visibility IN ('draft','private')", 2)
	atLeast("incident linked to internal", "SELECT COUNT(*) FROM status_page_incidents WHERE internal_incident_id <> ''", 1)
	now := time.Now().UTC()
	atLeast("maintenance in progress", "SELECT COUNT(*) FROM status_page_maintenance WHERE starts_at <= ? AND ends_at > ?", 1, now, now)
	atLeast("maintenance scheduled", "SELECT COUNT(*) FROM status_page_maintenance WHERE starts_at > ?", 1, now)
	atLeast("maintenance completed", "SELECT COUNT(*) FROM status_page_maintenance WHERE ends_at <= ?", 1, now)
	for _, state := range []string{"confirmed", "pending", "unsubscribed", "disabled", "bounced"} {
		atLeast("subscriber "+state, "SELECT COUNT(*) FROM status_page_subscribers WHERE state = ?", 1, state)
	}
	// Nothing seeded may be picked up by the worker the moment it starts.
	if n := count(t, database, "SELECT COUNT(*) FROM core_monitor_configs WHERE monitor_id LIKE 'seed-state-%' AND next_run_at < ?", now.AddDate(1, 0, 0)); n != 0 {
		t.Errorf("%d showcase core monitors are due within a year; the worker would run them", n)
	}
	if n := count(t, database, "SELECT COUNT(*) FROM alert_deliveries WHERE id LIKE 'seed-%' AND status IN ('pending','failed') AND attempt_count < max_attempts AND (next_attempt_at IS NULL OR next_attempt_at < ?)", now.Add(time.Hour)); n != 0 {
		t.Errorf("%d seeded deliveries are due within the hour; the worker would send them", n)
	}
}
