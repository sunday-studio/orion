package db

import (
	"orion/core/internal/utils"
	"path/filepath"
	"testing"
)

// Two processes upgrading in the same instant (API and Worker) must not collide on the backup file name.
func TestMigrationBackupsDoNotCollideWithinOneSecond(t *testing.T) {
	dir := t.TempDir()
	database, err := OpenFile(dir, "orion.db", "warn")
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := database.DB()
	for i := 0; i < 5; i++ {
		if err := backupBeforeMigration(sqlDB, 1, utils.NewLogger()); err != nil {
			t.Fatalf("backup %d: %v", i, err)
		}
	}
	files, _ := filepath.Glob(filepath.Join(dir, "backups", "orion-pre-migrate-*.db"))
	if len(files) != keepMigrationBackups {
		t.Fatalf("kept %d backups, want %d", len(files), keepMigrationBackups)
	}
}
