package startup

import (
	"fmt"
	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/logging"
	"os"

	"gorm.io/gorm"
)

// LoadConfig loads dotenv values when present, then validates Core runtime config.
func LoadConfig(dotEnvPath string) (*config.Config, error) {
	if err := config.LoadDotEnv(dotEnvPath); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("load dotenv: %w", err)
	}

	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}
	return cfg, nil
}

// OpenMigratedDatabase opens the Core SQLite database and applies embedded migrations.
func OpenMigratedDatabase(cfg *config.Config) (*gorm.DB, error) {
	database, err := db.Initialize(cfg.DataDir)
	if err != nil {
		return nil, fmt.Errorf("initialize database: %w", err)
	}

	if err := db.Migrate(database); err != nil {
		return nil, fmt.Errorf("run database migrations: %w", err)
	}
	return database, nil
}

// CloseDatabase closes the underlying SQL handle and logs close failures.
func CloseDatabase(database *gorm.DB, logger *logging.Logger) {
	if database == nil {
		return
	}
	sqlDB, err := database.DB()
	if err != nil {
		logger.Warn("Failed to get database handle for close", "error", err)
		return
	}
	if err := sqlDB.Close(); err != nil {
		logger.Warn("Failed to close database", "error", err)
	}
}
