package worker

import (
	"context"
	"orion/core/internal/config"
	"orion/core/internal/db"
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
	delivery := db.AlertDelivery{ID: "delivery-1", IncidentID: "incident-1", EventType: "incident_opened", Channel: "legacy", Type: "email", Status: "pending", MaxAttempts: 3}
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
