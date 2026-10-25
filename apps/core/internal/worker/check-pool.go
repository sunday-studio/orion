package worker

import (
	"context"
	"time"

	"orion/core/internal/db"
	"orion/core/internal/service"
)

const (
	defaultCheckConcurrency = 16
	shutdownCheckGrace      = 15 * time.Second
)

func checkConcurrency(configured int) int {
	if configured <= 0 {
		return defaultCheckConcurrency
	}
	return configured
}

// runDueChecks claims due checks, runs them to completion and returns. The Run
// loop uses dispatchDueChecks instead so one slow target never delays the rest.
func (a *App) runDueChecks(ctx context.Context) error {
	err := a.dispatchDueChecks(ctx, ctx)
	a.inflight.Wait()
	return err
}

// dispatchDueChecks claims as many due checks as there are free slots and starts
// each in its own goroutine, bounded by the worker's concurrency. claimCtx gates
// claiming; execCtx is what the checks themselves run under.
func (a *App) dispatchDueChecks(claimCtx context.Context, execCtx context.Context) error {
	if err := claimCtx.Err(); err != nil {
		return err
	}
	free := cap(a.slots) - len(a.slots) // only this goroutine adds to slots, so this never overestimates
	if free <= 0 {
		return nil
	}
	limit := free
	if a.claimLimit > 0 && a.claimLimit < limit {
		limit = a.claimLimit
	}

	claimed, err := a.scheduler.ClaimDueCoreMonitorConfigs(service.ClaimDueCoreMonitorConfigsRequest{
		LeaseOwner:    a.workerID,
		Limit:         limit,
		LeaseDuration: a.leaseDuration,
		Now:           time.Now().UTC(),
	})
	if err != nil {
		return err
	}

	for _, monitorConfig := range claimed {
		a.slots <- struct{}{}
		a.inflight.Add(1)
		go func() {
			defer func() {
				<-a.slots
				a.inflight.Done()
			}()
			a.executeClaimedCheck(execCtx, monitorConfig)
		}()
	}
	return nil
}

// executeClaimedCheck runs one claimed check while a companion goroutine keeps
// its lease alive, so a check slower than the lease is not run twice.
func (a *App) executeClaimedCheck(ctx context.Context, monitorConfig db.CoreMonitorConfig) {
	stopRenewal := make(chan struct{})
	renewed := make(chan struct{})
	go func() {
		defer close(renewed)
		a.renewLeaseUntil(stopRenewal, monitorConfig.MonitorID)
	}()
	err := a.runClaimedCheck(ctx, monitorConfig)
	close(stopRenewal)
	<-renewed
	if err != nil {
		a.logger.Error("Core monitor check failed", "monitor_id", monitorConfig.MonitorID, "kind", monitorConfig.Kind, "error", err)
	}
}

func (a *App) renewLeaseUntil(stop <-chan struct{}, monitorID string) {
	every := max(a.leaseDuration/3, 10*time.Millisecond)
	ticker := time.NewTicker(every)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			held, err := a.scheduler.RenewCoreMonitorLease(monitorID, a.workerID, a.leaseDuration, time.Now().UTC())
			if err != nil {
				a.logger.Warn("Failed to renew Core monitor lease", "monitor_id", monitorID, "error", err)
			} else if !held {
				return // lost the lease (taken over after expiry); the result will be rejected on completion
			}
		}
	}
}

// drainChecks waits for in-flight checks, cancelling them if they exceed the
// grace period, then releases any leases still held so another worker can take
// the checks over right away.
func (a *App) drainChecks(cancelChecks context.CancelFunc) {
	done := make(chan struct{})
	go func() {
		a.inflight.Wait()
		a.background.Wait() // a dispatch pass in progress finishes before leases are released
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(shutdownCheckGrace):
		a.logger.Warn("Core monitor checks exceeded the shutdown grace period; cancelling", "grace", shutdownCheckGrace.String())
		cancelChecks()
		<-done
	}
	if released, err := a.scheduler.ReleaseCoreMonitorLeases(a.workerID); err != nil {
		a.logger.Error("Failed to release Core monitor leases on shutdown", "error", err)
	} else if released > 0 {
		a.logger.Info("Released Core monitor leases on shutdown", "count", released)
	}
}
