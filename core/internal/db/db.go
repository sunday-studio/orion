package db

import (
	"orion/core/internal/logging"
	"os"
	"path/filepath"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Initialize() (*gorm.DB, error) {
	dataDir := "data"
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

// Migrate runs database migrations using GORM AutoMigrate
// For production, consider using golang-migrate via RunMigrations()
func Migrate(db *gorm.DB) error {
	log := logging.NewLogger()

	log.Info("Running database migrations (AutoMigrate)")

	if err := db.AutoMigrate(&Agent{}, &AgentReport{}, &Monitor{}, &MonitorReport{}); err != nil {
		log.Error("Migration failed", "error", err)
		return err
	}

	log.Info("Database migrations completed successfully")
	return nil
}

// MigrateWithFiles runs migrations from SQL files using golang-migrate
// This is the preferred method for production deployments
func MigrateWithFiles(db *gorm.DB, migrationsPath string, logger *logging.Logger) error {
	// Try to run file-based migrations first
	if err := RunMigrations(db, migrationsPath, logger); err != nil {
		logger.Warn("File-based migrations failed, falling back to AutoMigrate", "error", err)
		// Fall back to AutoMigrate if file migrations fail (e.g., migrations directory doesn't exist)
		return Migrate(db)
	}
	return nil
}
