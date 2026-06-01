package worker

import (
	"context"
	"orion/core/internal/logging"
	"testing"
	"time"
)

func TestAppRunStopsWhenContextIsCanceled(t *testing.T) {
	database := openWorkerTestDatabase(t)
	app := NewApp(database, logging.NewLogger(), Options{HealthInterval: time.Millisecond})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if err := app.Run(ctx); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}

func TestAppUsesDefaultHealthInterval(t *testing.T) {
	database := openWorkerTestDatabase(t)
	app := NewApp(database, logging.NewLogger(), Options{})

	if app.healthInterval != defaultHealthInterval {
		t.Fatalf("healthInterval = %v, want %v", app.healthInterval, defaultHealthInterval)
	}
}

func TestAppCheckDatabasePassesForOpenDatabase(t *testing.T) {
	database := openWorkerTestDatabase(t)
	app := NewApp(database, logging.NewLogger(), Options{})

	if err := app.checkDatabase(t.Context()); err != nil {
		t.Fatalf("checkDatabase() error = %v", err)
	}
}
