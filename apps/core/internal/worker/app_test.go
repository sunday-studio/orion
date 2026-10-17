package worker

import (
	"context"
	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/logstore"
	"orion/core/internal/testkit"
	"orion/core/internal/utils"
	"testing"
	"time"
)

func TestAppRunStopsWhenContextIsCanceled(t *testing.T) {
	database := openWorkerTestDatabase(t)
	app := NewApp(database, utils.NewLogger(), Options{HealthInterval: time.Millisecond})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if err := app.Run(ctx); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestAppUsesDefaultHealthInterval(t *testing.T) {
	database := openWorkerTestDatabase(t)
	app := NewApp(database, utils.NewLogger(), Options{})

	if app.healthInterval != defaultHealthInterval {
		t.Fatalf("healthInterval = %v, want %v", app.healthInterval, defaultHealthInterval)
	}
}

func TestAppCheckDatabasePassesForOpenDatabase(t *testing.T) {
	database := openWorkerTestDatabase(t)
	app := NewApp(database, utils.NewLogger(), Options{})

	if err := app.checkDatabase(t.Context()); err != nil {
		t.Fatalf("checkDatabase() error = %v", err)
	}
}

func TestAppDispatchAlertsProcessesDueDeliveries(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	app := NewApp(database, utils.NewLogger(), Options{Config: &config.Config{}})
	if app.dispatchEvery != defaultDispatchInterval {
		t.Fatalf("dispatchEvery = %v, want %v", app.dispatchEvery, defaultDispatchInterval)
	}
	// A retired channel type is suppressed by the dispatcher without any network I/O,
	// which proves the Worker actually drives ProcessDueDeliveries.
	delivery := db.AlertDelivery{ID: "delivery-1", IncidentID: "incident-1", EventType: "incident_opened", Channel: "legacy", Type: "slack", Status: "pending", MaxAttempts: 3}
	if err := database.Create(&delivery).Error; err != nil {
		t.Fatalf("create delivery: %v", err)
	}

	app.dispatchAlerts()

	var got db.AlertDelivery
	if err := database.First(&got, "id = ?", "delivery-1").Error; err != nil {
		t.Fatalf("load delivery: %v", err)
	}
	if got.Status != "suppressed" {
		t.Fatalf("delivery status = %q, want suppressed", got.Status)
	}
}

func TestAppSweepSilentMonitorsOpensIncidentForQuietAgentMonitor(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	app := NewApp(database, utils.NewLogger(), Options{Config: &config.Config{}, SweepBootGrace: -1})
	if app.sweepEvery != defaultSweepInterval {
		t.Fatalf("sweepEvery = %v, want %v", app.sweepEvery, defaultSweepInterval)
	}
	old := time.Now().UTC().Add(-time.Hour)
	agent := db.Agent{ID: "agent-quiet", MachineId: "machine-quiet", Name: "quiet", OS: "linux", Arch: "amd64", Token: "t", LastSeen: old}
	monitor := db.Monitor{ID: "monitor-quiet", AgentID: agent.ID, Name: "api", Type: "http", Lifecycle: "active", Health: "up", ComputedHealth: "up", IncidentState: "up", ReportingIntervalSeconds: 60, CreatedAt: old}
	if err := database.Create(&agent).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Create(&monitor).Error; err != nil {
		t.Fatal(err)
	}

	app.sweepSilentMonitors()

	var count int64
	database.Model(&db.Incident{}).Where("monitor_id = ? AND status = ?", monitor.ID, "open").Count(&count)
	if count != 1 {
		t.Fatalf("open incidents = %d, want 1 from the Worker sweep", count)
	}
}

func TestAppSweepWaitsForBootGraceAfterStart(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	app := NewApp(database, utils.NewLogger(), Options{Config: &config.Config{}})
	if app.sweepBootGrace != defaultSweepBootGrace {
		t.Fatalf("sweepBootGrace = %v, want %v", app.sweepBootGrace, defaultSweepBootGrace)
	}
	old := time.Now().UTC().Add(-time.Hour)
	database.Create(&db.Agent{ID: "agent-boot", MachineId: "machine-boot", Name: "boot", OS: "linux", Arch: "amd64", Token: "t", LastSeen: old})
	database.Create(&db.Monitor{ID: "monitor-boot", AgentID: "agent-boot", Name: "api", Type: "http", Lifecycle: "active", Health: "up", ComputedHealth: "up", IncidentState: "up", ReportingIntervalSeconds: 60, CreatedAt: old})

	app.sweepSilentMonitors()
	var count int64
	database.Model(&db.Incident{}).Count(&count)
	if count != 0 {
		t.Fatalf("incidents during boot grace = %d, want 0 (no storm after restart)", count)
	}

	app.startedAt = time.Now().UTC().Add(-2 * time.Minute)
	app.sweepSilentMonitors()
	database.Model(&db.Incident{}).Count(&count)
	if count != 1 {
		t.Fatalf("incidents after boot grace = %d, want 1", count)
	}
}

// A receiver that hangs must not stall the Worker loop: dispatch runs beside it.
func TestDispatchAlertsAsyncDoesNotBlockAndNeverOverlaps(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	app := NewApp(database, utils.NewLogger(), Options{Config: &config.Config{}})
	delivery := db.AlertDelivery{ID: "slow", IncidentID: "i", EventType: "incident_opened", Channel: "legacy", Type: "slack", Status: "pending", MaxAttempts: 3}
	database.Create(&delivery)

	app.dispatching.Store(true) // a pass is already running
	started := time.Now()
	app.dispatchAlertsAsync() // must return immediately and not start a second pass
	if time.Since(started) > 100*time.Millisecond {
		t.Fatal("dispatchAlertsAsync blocked the caller")
	}
	app.background.Wait()
	var got db.AlertDelivery
	database.First(&got, "id = ?", "slow")
	if got.Status != "pending" {
		t.Fatalf("status = %q, want untouched: an overlapping pass must be skipped", got.Status)
	}

	app.dispatching.Store(false)
	app.dispatchAlertsAsync()
	app.background.Wait()
	database.First(&got, "id = ?", "slow")
	if got.Status != "suppressed" || app.dispatching.Load() {
		t.Fatalf("status = %q dispatching = %t, want the pass to run once and release its guard", got.Status, app.dispatching.Load())
	}
}

func TestAppEvaluatesLogAlertRulesWhenItHasALogStore(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	store, err := logstore.OpenWith(testkit.NewDB(t), utils.NewLogger(), 0)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	now := time.Now().UTC()
	database.Create(&db.App{ID: "app-1", Name: "api", Slug: "api", RetentionDays: 14, DebugSampleRate: 1, DisplayJSON: "{}", RedactionJSON: "{}", CreatedAt: now, UpdatedAt: now})
	database.Create(&db.LogAlertRule{ID: "r1", Name: "errors", AppID: "app-1", Mode: "threshold", Severity: ">=error", Threshold: 2, WindowSeconds: 300, Enabled: true, State: "ok", CreatedAt: now.Add(-time.Hour), UpdatedAt: now})
	var batch []logstore.Entry
	for i := 0; i < 5; i++ {
		batch = append(batch, logstore.Entry{ID: logstore.NewEntryID(now), AppID: "app-1", Timestamp: now, IngestedAt: now, Severity: "error", SeverityNumber: 17, Body: "boom", Size: 10})
	}
	store.Enqueue(batch)
	store.Flush(3 * time.Second)

	app := NewApp(database, utils.NewLogger(), Options{Config: &config.Config{}, LogStore: store})
	app.evaluateLogAlerts()

	var rule db.LogAlertRule
	database.First(&rule, "id = ?", "r1")
	var incidents int64
	database.Model(&db.Incident{}).Where("monitor_id = ?", "monitor-log-alert-r1").Count(&incidents)
	if rule.State != "firing" || incidents != 1 {
		t.Fatalf("rule state = %q incidents = %d, want the Worker to have raised the alert", rule.State, incidents)
	}

	// Without a log store the Worker simply skips log alerts.
	NewApp(database, utils.NewLogger(), Options{Config: &config.Config{}}).evaluateLogAlerts()
}
