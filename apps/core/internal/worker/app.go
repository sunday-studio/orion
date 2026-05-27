package worker

import (
	"context"
	"errors"
	"orion/core/internal/logging"
	"time"

	"gorm.io/gorm"
)

const defaultHealthInterval = 30 * time.Second

// Options configures the Core monitor worker foundation.
type Options struct {
	HealthInterval time.Duration
}

// App is the independent Core monitor worker process. Scheduling and runners
// are intentionally left to later worker milestones.
type App struct {
	db             *gorm.DB
	logger         *logging.Logger
	healthInterval time.Duration
}

// NewApp creates a worker app bound to the Core database.
func NewApp(database *gorm.DB, logger *logging.Logger, opts Options) *App {
	interval := opts.HealthInterval
	if interval <= 0 {
		interval = defaultHealthInterval
	}
	return &App{
		db:             database,
		logger:         logger,
		healthInterval: interval,
	}
}

// Run logs worker health until the context is canceled.
func (a *App) Run(ctx context.Context) error {
	a.logger.Info("Core monitor worker started", "health_interval", a.healthInterval.String())
	a.logHealth(ctx)

	ticker := time.NewTicker(a.healthInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			a.logger.Info("Core monitor worker shutting down", "reason", ctx.Err().Error())
			return nil
		case <-ticker.C:
			a.logHealth(ctx)
		}
	}
}

func (a *App) logHealth(ctx context.Context) {
	if err := a.checkDatabase(ctx); err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			a.logger.Info("Core monitor worker health check interrupted", "error", err)
			return
		}
		a.logger.Error("Core monitor worker health check failed", "database", "unavailable", "error", err)
		return
	}
	a.logger.Info("Core monitor worker health check passed", "database", "ok")
}

func (a *App) checkDatabase(ctx context.Context) error {
	sqlDB, err := a.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}
