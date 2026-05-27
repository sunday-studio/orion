package main

import (
	"context"
	"orion/core/internal/logging"
	"orion/core/internal/startup"
	"orion/core/internal/worker"
	"os/signal"
	"syscall"
)

func main() {
	logger := logging.NewLogger()
	logger.Info("Starting Orion Core monitor worker")

	cfg, err := startup.LoadConfig(".env")
	if err != nil {
		logger.Fatal("Failed to load config", "error", err)
	}

	database, err := startup.OpenMigratedDatabase(cfg)
	if err != nil {
		logger.Fatal("Failed to initialize database", "error", err)
	}
	defer startup.CloseDatabase(database, logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	app := worker.NewApp(database, logger, worker.Options{})
	if err := app.Run(ctx); err != nil {
		logger.Fatal("Core monitor worker stopped with error", "error", err)
	}
}
