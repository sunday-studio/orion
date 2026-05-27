package main

import (
	"context"
	"orion/core/internal/api"
	"orion/core/internal/logging"
	"orion/core/internal/service"
	"orion/core/internal/startup"
	"os/signal"
	"syscall"
	"time"
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

	cfg, err := startup.LoadConfig(".env")
	if err != nil {
		logger.Fatal("Failed to load config", "error", err)
	}

	database, err := startup.OpenMigratedDatabase(cfg)
	if err != nil {
		logger.Fatal("Failed to initialize database", "error", err)
	}
	defer startup.CloseDatabase(database, logger)

	// Initialize and start HTTP server
	server := api.NewServer(database, logger, cfg)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	lifecycleScheduler := service.NewDataLifecycleSchedulerService(database, logger, cfg.DataDir, time.Duration(cfg.DataLifecycleSchedulerSeconds)*time.Second)
	go func() {
		if err := lifecycleScheduler.Run(ctx); err != nil {
			logger.Error("Data lifecycle scheduler stopped", "error", err)
		}
	}()

	logger.Info("Orion Core Server started successfully")
	if err := server.Start(ctx, ":"+cfg.Port); err != nil {
		logger.Fatal("Failed to start server", "error", err)
	}
}
