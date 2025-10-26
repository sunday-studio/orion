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

func Migrate(db *gorm.DB) error {
	log := logging.NewLogger()

	log.Info("Running database migrations")

	if err := db.AutoMigrate(&Agent{}, &AgentReport{}, &Application{}, &ApplicationReport{}); err != nil {
		log.Error("Migration failed", "error", err)
		return err
	}

	log.Info("Database migrations completed successfully")
	return nil
}
