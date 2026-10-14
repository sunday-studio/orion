package worker

import "time"

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

// sweepSilentMonitors opens stale incidents for agent monitors that stopped reporting.
func (a *App) sweepSilentMonitors() {
	flagged, err := a.incidents.SweepSilentMonitors(time.Now().UTC())
	if err != nil {
		a.logger.Error("Core worker silent monitor sweep failed", "error", err)
	}
	if flagged > 0 {
		a.logger.Warn("Core worker flagged silent monitors as stale", "count", flagged)
	}
}

// dispatchAlerts sends due alert deliveries (new, retried and delayed summaries).
func (a *App) dispatchAlerts() {
	if _, err := a.alerts.ProcessDueDeliveries(100); err != nil {
		a.logger.Error("Core worker alert dispatch failed", "error", err)
	}
}
