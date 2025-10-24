package db

import (
	"orion/core/internal/logging"
	"os"
	"path/filepath"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Initialize creates and returns a new database connection
func Initialize() (*gorm.DB, error) {
	// Create data directory if it doesn't exist
	dataDir := "data"
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}

	// SQLite database path
	dbPath := filepath.Join(dataDir, "orion.db")
	
	// Configure GORM logger
	gormLogger := logger.Default.LogMode(logger.Info)

	// Connect to SQLite database
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return nil, err
	}

	// Configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	// Set connection pool settings
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)

	return db, nil
}

// Migrate runs database migrations
func Migrate(db *gorm.DB) error {
	log := logging.NewLogger()
	
	log.Info("Running database migrations")
	
	// Auto-migrate the schema
	if err := db.AutoMigrate(&Agent{}, &Report{}); err != nil {
		log.Error("Migration failed", "error", err)
		return err
	}

	log.Info("Database migrations completed successfully")
	return nil
}
