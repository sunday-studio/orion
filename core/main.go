package main

import (
	"log"
	"orion/core/internal/api"
	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/logging"
)

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

	// Initialize and start HTTP server
	server := api.NewServer(database, logger, cfg)

	logger.Info("Orion Core Server started successfully")
	if err := server.Start(":" + cfg.Port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}
