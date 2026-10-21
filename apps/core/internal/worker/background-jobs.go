package worker

import (
	"time"

	"orion/core/internal/service"
	"orion/core/internal/utils"

	"gorm.io/gorm"
)

const defaultDispatchInterval = 15 * time.Second

func dispatchInterval(configured time.Duration) time.Duration {
	if configured <= 0 {
		return defaultDispatchInterval
	}
	return configured
}

const defaultSweepInterval = 30 * time.Second

func sweepInterval(configured time.Duration) time.Duration {
	if configured <= 0 {
		return defaultSweepInterval
	}
	return configured
}

const defaultSweepBootGrace = 60 * time.Second

func sweepBootGrace(configured time.Duration) time.Duration {
	switch {
	case configured < 0:
		return 0
	case configured == 0:
		return defaultSweepBootGrace
	default:
		return configured
	}
}

// sweepSilentMonitors opens stale incidents for node monitors that stopped reporting.
func (a *App) sweepSilentMonitors() {
	// Ending expired maintenance windows is never held back by the boot grace:
	// alerting for those nodes should resume on time.
	if ended, err := service.NewNodeService(a.db, a.logger).ExpireMaintenance(time.Now().UTC()); err != nil {
		a.logger.Error("Core worker maintenance expiry failed", "error", err)
	} else if ended > 0 {
		a.logger.Info("Core worker ended maintenance windows", "count", ended)
	}
	if time.Since(a.startedAt) < a.sweepBootGrace {
		return
	}
	flagged, err := a.incidents.SweepSilentMonitors(time.Now().UTC())
	if err != nil {
		a.logger.Error("Core worker silent monitor sweep failed", "error", err)
	}
	if flagged > 0 {
		a.logger.Warn("Core worker flagged silent monitors as stale", "count", flagged)
	}
	offline, err := a.incidents.SweepSilentNodes(time.Now().UTC())
	if err != nil {
		a.logger.Error("Core worker silent node sweep failed", "error", err)
	}
	if offline > 0 {
		a.logger.Warn("Core worker flagged silent nodes as offline", "count", offline)
	}
}

// dispatchAlertsAsync runs a dispatch pass without blocking the Worker loop and never
// overlaps itself. A webhook receiver that hangs (10s per call, up to 100 calls) would
// otherwise stall check dispatch, heartbeat reconciliation and the silent sweeps.
func (a *App) dispatchAlertsAsync() {
	if !a.dispatching.CompareAndSwap(false, true) {
		return
	}
	a.background.Add(1)
	go func() {
		defer a.background.Done()
		defer a.dispatching.Store(false)
		a.dispatchAlerts()
	}()
}

// dispatchAlerts sends due alert deliveries (new, retried and delayed summaries).
func (a *App) dispatchAlerts() {
	if _, err := a.alerts.ProcessDueDeliveries(100); err != nil {
		a.logger.Error("Core worker alert dispatch failed", "error", err)
	}
}

func newLogAlerts(database *gorm.DB, logger *utils.Logger, opts Options) *service.LogAlertService {
	if opts.LogStore == nil {
		return nil
	}
	return service.NewLogAlertService(database, logger, opts.Config, opts.LogStore)
}

// evaluateLogAlerts checks every enabled log alert rule. It is a no-op when the Worker
// has no logs.db handle.
func (a *App) evaluateLogAlerts() {
	if a.logAlerts == nil {
		return
	}
	changed, err := a.logAlerts.EvaluateAll(time.Now().UTC())
	if err != nil {
		a.logger.Error("Core worker log alert evaluation failed", "error", err)
	}
	if changed > 0 {
		a.logger.Info("Core worker log alerts changed state", "count", changed)
	}
}
