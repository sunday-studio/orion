package db

import (
	"orion/core/internal/logging"
	"os"
	"path/filepath"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Initialize(dataDir string) (*gorm.DB, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}

	dbPath := filepath.Join(dataDir, "orion.db")

	gormLogger := logger.Default.LogMode(logger.Info)

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)

	return db, nil
}

// Migrate runs database migrations using GORM AutoMigrate.
func Migrate(db *gorm.DB) error {
	log := logging.NewLogger()

	log.Info("Running database migrations (AutoMigrate)")

	if err := db.AutoMigrate(&Agent{}, &AgentReport{}, &Monitor{}, &MonitorReport{}, &Incident{}, &IncidentEvent{}); err != nil {
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
