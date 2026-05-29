package main

import (
	"context"
	"fmt"
	"log"
	"orion/core/internal/config"
	"orion/core/internal/logging"
	"orion/core/internal/service"
	"orion/core/internal/startup"
	"orion/core/internal/worker"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

var version = "dev"

const playwrightRunnerEnv = "ORION_PLAYWRIGHT_RUNNER"

func main() {
	logger := logging.NewLogger()

	cfg, err := startup.LoadConfig(".env")
	if err != nil {
		logger.Fatal("Failed to load config", "error", err)
	}

	if len(os.Args) > 1 && strings.EqualFold(os.Args[1], "healthcheck") {
		if err := runHealthcheck(cfg); err != nil {
			log.Fatal(err)
		}
		return
	}

	logger.Info("Starting Orion Core monitor worker", "worker_id", cfg.CoreWorkerID)
	if strings.TrimSpace(os.Getenv(playwrightRunnerEnv)) == "" {
		logger.Warn("Playwright transaction runtime is not configured", "env", playwrightRunnerEnv, "behavior", "playwright monitors report runtime_unavailable until a runner executable is configured", "docs", "docs/deployment/core-docker.md#playwright-transaction-runtime")
	} else {
		logger.Info("Playwright transaction runtime configured", "env", playwrightRunnerEnv)
	}

	database, err := startup.OpenMigratedDatabase(cfg)
	if err != nil {
		logger.Fatal("Failed to initialize database", "error", err)
	}
	defer startup.CloseDatabase(database, logger)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	diagnostics := service.NewWorkerDiagnosticsService(database, logger)
	heartbeatDone := make(chan struct{})
	go func() {
		defer close(heartbeatDone)
		if err := runHeartbeatLoop(ctx, diagnostics, cfg, logger); err != nil {
			logger.Error("Core monitor worker diagnostics stopped", "error", err)
			stop()
		}
	}()

	app := worker.NewApp(database, logger, worker.Options{
		WorkerID: cfg.CoreWorkerID,
		Config:   cfg,
	})
	if err := app.Run(ctx); err != nil {
		logger.Fatal("Core monitor worker stopped with error", "error", err)
	}
	<-heartbeatDone
}

func runHealthcheck(cfg *config.Config) error {
	database, err := startup.OpenMigratedDatabase(cfg)
	if err != nil {
		return err
	}
	sqlDB, err := database.DB()
	if err != nil {
		return fmt.Errorf("get database handle: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("ping database: %w", err)
	}
	return nil
}

func runHeartbeatLoop(ctx context.Context, diagnostics *service.WorkerDiagnosticsService, cfg *config.Config, logger *logging.Logger) error {
	hostname, _ := os.Hostname()
	startedAt := time.Now().UTC()
	interval := time.Duration(cfg.CoreWorkerHeartbeatSeconds) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	writeHeartbeat := func(writeCtx context.Context, status string) error {
		return diagnostics.RecordHeartbeat(writeCtx, service.WorkerHeartbeat{
			WorkerID:        cfg.CoreWorkerID,
			Hostname:        hostname,
			Status:          status,
			Version:         version,
			StartedAt:       startedAt,
			LastHeartbeatAt: time.Now().UTC(),
		})
	}

	if err := writeHeartbeat(ctx, "running"); err != nil {
		return err
	}
	logger.Info("Core monitor worker diagnostics heartbeat started", "worker_id", cfg.CoreWorkerID, "interval_seconds", cfg.CoreWorkerHeartbeatSeconds)

	for {
		select {
		case <-ctx.Done():
			stopCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			if err := writeHeartbeat(stopCtx, "stopping"); err != nil {
				logger.Warn("Failed to write stopping heartbeat", "error", err)
			}
			cancel()
			logger.Info("Core monitor worker stopped", "worker_id", cfg.CoreWorkerID)
			return nil
		case <-ticker.C:
			if err := writeHeartbeat(ctx, "running"); err != nil {
				logger.Error("Failed to write Core monitor worker heartbeat", "error", err)
				return err
			}
		}
	}
}
