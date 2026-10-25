package db

import (
	"fmt"
	"log"
	"orion/core/internal/logging"
	"os"
	"path/filepath"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// sqliteDSN builds a DSN with WAL, a busy timeout so contended writers wait
// instead of failing, and immediate write transactions to avoid lock-upgrade deadlocks.
func sqliteDSN(path string) string {
	return fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL&_txlock=immediate", path)
}

func open(path string) (*gorm.DB, error) {
	gormLogger := logger.New(
		log.New(os.Stdout, "", log.LstdFlags),
		logger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true,
		},
	)

	db, err := gorm.Open(sqlite.Open(sqliteDSN(path)), &gorm.Config{Logger: gormLogger})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	// WAL allows concurrent readers alongside one writer; extra connections beyond
	// a handful only queue on the write lock.
	sqlDB.SetMaxOpenConns(8)
	sqlDB.SetMaxIdleConns(8)

	return db, nil
}

func Initialize(dataDir string) (*gorm.DB, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}
	return open(filepath.Join(dataDir, "orion.db"))
}

// InitializeArchive opens the cold store that receives reports aged out of the main DB.
func InitializeArchive(dataDir string) (*gorm.DB, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}
	archive, err := open(filepath.Join(dataDir, "archive.db"))
	if err != nil {
		return nil, err
	}
	if err := archive.AutoMigrate(&AgentReport{}, &MonitorReport{}); err != nil {
		return nil, err
	}
	return archive, nil
}

// Migrate runs database migrations using GORM AutoMigrate.
func Migrate(db *gorm.DB) error {
	log := logging.NewLogger()

	log.Info("Running database migrations (AutoMigrate)")

	// Superseded by the composite (id, created_at) indexes; the old single-column
	// ones are redundant prefixes and only slow down writes.
	for _, legacy := range []struct {
		model any
		name  string
	}{
		{&MonitorReport{}, "idx_monitor_reports_monitor_id"},
		{&AgentReport{}, "idx_agent_reports_agent_id"},
	} {
		if db.Migrator().HasIndex(legacy.model, legacy.name) {
			if err := db.Migrator().DropIndex(legacy.model, legacy.name); err != nil {
				log.Error("Failed to drop legacy index", "index", legacy.name, "error", err)
			}
		}
	}

	if err := db.AutoMigrate(&Agent{}, &AgentReport{}, &Monitor{}, &MonitorReport{}, &MonitorDailyRollup{}); err != nil {
		log.Error("Migration failed", "error", err)
		return err
	}

	log.Info("Database migrations completed successfully")
	return nil
}

// MigrateWithFiles runs migrations from SQL files using golang-migrate.
// File-based migrations are not yet implemented; it falls back to AutoMigrate.
func MigrateWithFiles(db *gorm.DB, migrationsPath string, logger *logging.Logger) error {
	// TODO: wire up golang-migrate when migration files exist
	logger.Info("File-based migrations not configured, using AutoMigrate")
	return Migrate(db)
}
