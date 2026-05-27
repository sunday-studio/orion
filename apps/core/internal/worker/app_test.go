package worker

import (
	"context"
	"orion/core/internal/logging"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAppRunStopsWhenContextIsCanceled(t *testing.T) {
	database := openWorkerTestDatabase(t)
	app := NewApp(database, logging.NewLogger(), Options{HealthInterval: time.Millisecond})
	ctx, cancel := context.WithCancel(context.Background())
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

	if err := app.checkDatabase(context.Background()); err != nil {
		t.Fatalf("checkDatabase() error = %v", err)
	}
}

func openWorkerTestDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	database, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	return database
}
