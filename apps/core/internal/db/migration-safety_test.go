package db

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func fileDatabase(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	return dir, filepath.Join(dir, "orion.db")
}

func TestMigrationChecksumDetectsEditedAppliedMigration(t *testing.T) {
	dir, _ := fileDatabase(t)
	database, err := Initialize(dir, "silent")
	if err != nil {
		t.Fatal(err)
	}
	if err := Migrate(database); err != nil {
		t.Fatal(err)
	}
	var stored string
	database.Raw("SELECT checksum FROM schema_migrations WHERE version = 1").Scan(&stored)
	if len(stored) != 64 {
		t.Fatalf("stored checksum = %q, want a sha256 hex digest", stored)
	}

	database.Exec("UPDATE schema_migrations SET checksum = 'tampered' WHERE version = 5")
	err = Migrate(database)
	if err == nil || !strings.Contains(err.Error(), "modified after it was applied") {
		t.Fatalf("Migrate() error = %v, want checksum mismatch", err)
	}
}

func TestMigrationBackfillsChecksumsForLegacyRows(t *testing.T) {
	dir, _ := fileDatabase(t)
	database, _ := Initialize(dir, "silent")
	if err := Migrate(database); err != nil {
		t.Fatal(err)
	}
	database.Exec("UPDATE schema_migrations SET checksum = ''")
	if err := Migrate(database); err != nil {
		t.Fatalf("Migrate() with empty legacy checksums error = %v", err)
	}
	var empty int64
	database.Raw("SELECT COUNT(*) FROM schema_migrations WHERE checksum = ''").Scan(&empty)
	if empty != 0 {
		t.Fatalf("%d migrations still lack a checksum after backfill", empty)
	}
}

// Two processes starting together must migrate exactly once.
func TestConcurrentMigratorsApplyEachMigrationOnce(t *testing.T) {
	dir, _ := fileDatabase(t)
	var wg sync.WaitGroup
	errs := make(chan error, 4)
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			database, err := Initialize(dir, "silent") // separate handle, like a separate process
			if err != nil {
				errs <- err
				return
			}
			errs <- Migrate(database)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent Migrate() error = %v", err)
		}
	}
	database, _ := Initialize(dir, "silent")
	var count, distinct int64
	database.Raw("SELECT COUNT(*), COUNT(DISTINCT version) FROM schema_migrations").Row().Scan(&count, &distinct)
	if count != distinct || count == 0 {
		t.Fatalf("schema_migrations rows = %d distinct = %d, want each applied once", count, distinct)
	}
}

func TestPendingMigrationOnExistingDatabaseIsBackedUp(t *testing.T) {
	dir, _ := fileDatabase(t)
	database, _ := Initialize(dir, "silent")
	if err := Migrate(database); err != nil {
		t.Fatal(err)
	}
	backups := func() []string {
		m, _ := filepath.Glob(filepath.Join(dir, "backups", "orion-pre-migrate-*.db"))
		return m
	}
	if len(backups()) != 0 {
		t.Fatal("a fresh database must not be backed up")
	}

	// Re-pend the idempotent Go migration to simulate an upgrade.
	for i := 0; i < 5; i++ {
		database.Exec("DELETE FROM schema_migrations WHERE version = 32")
		if err := Migrate(database); err != nil {
			t.Fatalf("Migrate() error = %v", err)
		}
		time.Sleep(1100 * time.Millisecond) // timestamped names are per second
	}
	found := backups()
	if len(found) != keepMigrationBackups {
		t.Fatalf("backups kept = %d, want %d", len(found), keepMigrationBackups)
	}
	info, _ := os.Stat(found[0])
	if info.Mode().Perm() != 0o600 || info.Size() == 0 {
		t.Fatalf("backup mode %v size %d, want non-empty 0600", info.Mode().Perm(), info.Size())
	}
}

func TestCheckHealthIsReadOnlyAndDetectsMissingMigrations(t *testing.T) {
	dir, dbPath := fileDatabase(t)
	if err := CheckHealth(t.Context(), dir); err == nil {
		t.Fatal("CheckHealth() on a missing database = nil, want error")
	}

	database, _ := Initialize(dir, "silent")
	if err := Migrate(database); err != nil {
		t.Fatal(err)
	}
	before, _ := os.Stat(dbPath)
	if err := CheckHealth(t.Context(), dir); err != nil {
		t.Fatalf("CheckHealth() on a migrated database error = %v", err)
	}
	after, _ := os.Stat(dbPath)
	if !after.ModTime().Equal(before.ModTime()) {
		t.Fatal("CheckHealth() modified the database file")
	}

	database.Exec("DELETE FROM schema_migrations WHERE version = (SELECT MAX(version) FROM schema_migrations)")
	if err := CheckHealth(t.Context(), dir); err == nil {
		t.Fatal("CheckHealth() with a missing latest migration = nil, want error")
	}
}
