package main

import (
	"context"
	"orion/core/internal/api"
	"orion/core/internal/logstore"
	"orion/core/internal/metricstore"
	"orion/core/internal/service"
	"orion/core/internal/shared"
	"orion/core/internal/utils"
	"os/signal"
	"syscall"
	"time"
)

// @title           Orion Core API
// @version         1.0
// @description     Orion Core API for server management and monitoring
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

// @securityDefinitions.apikey IngestKey
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and an app ingest key (orion_ik_...). Write-only; created per app.

func main() {
	logger := utils.NewLogger()
	logger.Info("Starting Orion Core Server")

	cfg, err := shared.LoadConfig(".env")
	if err != nil {
		logger.Fatal("Failed to load config", "error", err)
	}

	if err := cfg.ValidateAPIAuth(); err != nil {
		logger.Fatal("Refusing to start", "error", err)
	}
	if cfg.RunsWithoutAuth() {
		logger.Warn("INSECURE: admin API authentication is disabled via ORION_ALLOW_INSECURE_NO_AUTH; anyone who can reach this port can manage nodes, alerts and incidents")
	}

	if cfg.EnrollmentToken == "" {
		logger.Warn("ORION_ENROLLMENT_TOKEN is not set: any client that can reach /v1/register can enroll a node (registration is rate limited only)")
	}

	database, err := shared.OpenMigratedDatabase(cfg)
	if err != nil {
		logger.Fatal("Failed to initialize database", "error", err)
	}
	defer shared.CloseDatabase(database, logger)

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

	logStore, err := logstore.Open(cfg.DataDir, logger, cfg.LogsQueueMax)
	if err != nil {
		logger.Fatal("Failed to open the product logs database", "error", err)
	}
	server.EnableProductLogs(logStore)
	logger.AttachSink(logstore.NewCoreLogSink(logStore, "api")) // Internal logs
	go logstore.RunRetention(ctx, logStore, database, logger, time.Hour, logstore.AppRetention{AppID: logstore.InternalAppID, RetentionDays: cfg.InternalLogRetentionDays, MaxStorageMB: cfg.InternalLogMaxStorageMB})

	// Resource history is a bonus: without metrics.db Core still stores latest values and answers 503 on history.
	metricStore, err := metricstore.Open(cfg.DataDir, logger)
	if err != nil {
		logger.Warn("Metrics database unavailable; resource history is disabled", "error", err)
	} else {
		server.EnableMetrics(metricStore)
		lifecycle := service.NewSettingsService(database, logger, cfg.DataDir)
		go metricstore.RunMaintenance(ctx, metricStore, func() metricstore.Retention {
			return service.MetricRetention(lifecycle)
		}, logger, time.Hour)
	}

	selfMonitor := service.NewSelfMonitorService(database, logger, cfg, server.RuntimeDiagnostics())
	go selfMonitor.Run(ctx)

	listenAddr := cfg.ListenAddress()
	if cfg.RunsWithoutAuth() {
		logger.Warn("INSECURE: binding to loopback only because admin authentication is disabled", "address", listenAddr)
	}

	logger.Info("Orion Core Server started successfully")
	startErr := server.Start(ctx, listenAddr)
	if metricStore != nil {
		metricStore.Close() // drain queued samples before exit
	}
	logStore.Close() // drain queued log entries before exit
	if startErr != nil {
		logger.Fatal("Failed to start server", "error", startErr)
	}
}
