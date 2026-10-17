package db

import (
	"database/sql"
	"fmt"
	"orion/core/internal/utils"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const keepMigrationBackups = 3

// backupBeforeMigration writes a consistent copy of the database (VACUUM INTO)
// to <data dir>/backups before pending migrations are applied to an existing
// database, and keeps only the newest few. The file name includes nanoseconds and the
// PID so two processes upgrading at the same moment (API and Worker) never collide. In-memory databases are skipped.
func backupBeforeMigration(db *sql.DB, firstPendingVersion int, logger *utils.Logger) error {
	dbPath, err := databaseFilePath(db)
	if err != nil || dbPath == "" {
		return err
	}
	backupDir := filepath.Join(filepath.Dir(dbPath), "backups")
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		return fmt.Errorf("create migration backup dir: %w", err)
	}
	target := filepath.Join(backupDir, fmt.Sprintf("orion-pre-migrate-%06d-%s-%d.db", firstPendingVersion, time.Now().UTC().Format("20060102T150405.000000000Z"), os.Getpid()))
	if _, err := db.Exec(fmt.Sprintf("VACUUM INTO '%s'", strings.ReplaceAll(target, "'", "''"))); err != nil {
		return fmt.Errorf("back up database before migration: %w", err)
	}
	if err := os.Chmod(target, 0o600); err != nil {
		return err
	}
	logger.Info("Backed up database before migration", "path", target)
	pruneMigrationBackups(backupDir, logger)
	return nil
}

func databaseFilePath(db *sql.DB) (string, error) {
	rows, err := db.Query("PRAGMA database_list")
	if err != nil {
		return "", err
	}
	defer rows.Close()
	for rows.Next() {
		var seq int
		var name, file string
		if err := rows.Scan(&seq, &name, &file); err != nil {
			return "", err
		}
		if name == "main" {
			return file, rows.Err()
		}
	}
	return "", rows.Err()
}

func pruneMigrationBackups(dir string, logger *utils.Logger) {
	matches, err := filepath.Glob(filepath.Join(dir, "orion-pre-migrate-*.db"))
	if err != nil || len(matches) <= keepMigrationBackups {
		return
	}
	sort.Strings(matches) // names embed a sortable timestamp
	for _, old := range matches[:len(matches)-keepMigrationBackups] {
		if err := os.Remove(old); err != nil {
			logger.Warn("Failed to remove old migration backup", "path", old, "error", err)
		}
	}
}
