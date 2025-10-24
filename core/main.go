package main

import (
	"log"
	"orion/core/internal/api"
	"orion/core/internal/db"
	"orion/core/internal/logging"
)

func main() {
	// Initialize logger
	logger := logging.NewLogger()
	logger.Info("Starting Orion Core Server")

	// Initialize database
	database, err := db.Initialize()
	if err != nil {
		logger.Fatal("Failed to initialize database", "error", err)
	}

	// Run migrations
	if err := db.Migrate(database); err != nil {
		logger.Fatal("Failed to run database migrations", "error", err)
	}

	// Initialize and start HTTP server
	server := api.NewServer(database, logger)
	
	logger.Info("Orion Core Server started successfully")
	if err := server.Start(":8999"); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}
