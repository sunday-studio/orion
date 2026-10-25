package main

import (
	"context"
	"log"
	"orion/core/internal/api"
	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/logging"
	"orion/core/internal/service"
	"time"

	"gorm.io/gorm"
)

// @title           Orion Core API
// @version         1.0
// @description     Orion Core API for agent management and monitoring
// @termsOfService  http://swagger.io/terms/

// @contact.name   API Support
// @contact.url    http://www.swagger.io/support
// @contact.email  support@swagger.io

// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html

// @host      localhost:8999
// @BasePath  /

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

func main() {
	logger := logging.NewLogger()
	logger.Info("Starting Orion Core Server")

	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		logger.Fatal("Invalid config", "error", err)
	}

	database, err := db.Initialize(cfg.DataDir)
	if err != nil {
		logger.Fatal("Failed to initialize database", "error", err)
	}

	if err := db.Migrate(database); err != nil {
		logger.Fatal("Failed to run database migrations", "error", err)
	}

	ctx := context.Background()

	var archive *gorm.DB
	if cfg.ArchiveEnabled && cfg.RetentionDays > 0 {
		archive, err = db.InitializeArchive(cfg.DataDir)
		if err != nil {
			logger.Fatal("Failed to initialize archive database", "error", err)
		}
	}
	db.StartRetention(ctx, database, archive, cfg.RetentionDays, logger)

	// Keep stored monitor health current (stale-data transitions) so request handlers never write.
	service.NewHealthService(database, logger).StartRefresher(ctx, 30*time.Second)

	// Initialize and start HTTP server
	server := api.NewServer(database, logger, cfg)

	logger.Info("Orion Core Server started successfully")
	if err := server.Start(":" + cfg.Port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}
