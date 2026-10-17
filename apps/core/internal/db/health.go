package db

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// CheckHealth verifies the database is reachable and fully migrated without
// writing anything: no migrations, no chmod, no schema changes. Container
// healthchecks use it so polling every 30s never takes a write lock or migrates.
func CheckHealth(ctx context.Context, dataDir string) error {
	dbPath := filepath.Join(dataDir, "orion.db")
	handle, err := sql.Open("sqlite3", "file:"+dbPath+"?mode=ro&_busy_timeout=2000")
	if err != nil {
		return err
	}
	defer handle.Close()

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := handle.PingContext(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}

	var applied int
	if err := handle.QueryRowContext(ctx, "SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&applied); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if want := latestMigrationVersion(); applied < want {
		return fmt.Errorf("database schema is at version %d, expected %d", applied, want)
	}
	return nil
}

func latestMigrationVersion() int {
	latest := 0
	if migrations, err := loadMigrations("migrations"); err == nil {
		for _, m := range migrations {
			latest = max(latest, m.version)
		}
	}
	for _, m := range goMigrations() {
		latest = max(latest, m.version)
	}
	return latest
}
