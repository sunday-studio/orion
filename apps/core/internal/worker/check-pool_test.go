package worker

import (
	"context"
	"fmt"
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func seedHTTPMonitors(t *testing.T, app *App, names []string, urlFor func(string) string, nextRun time.Time, interval int) {
	t.Helper()
	for _, name := range names {
		insertWorkerMonitor(t, app.db, name)
		insertWorkerCoreMonitorConfig(t, app.db, db.CoreMonitorConfig{
			MonitorID:       name,
			Kind:            "http",
			ConfigJSON:      fmt.Sprintf(`{"url":%q,"expected_status":204}`, urlFor(name)),
			IntervalSeconds: interval,
			TimeoutSeconds:  5,
			NextRunAt:       nextRun,
		})
	}
}

func slowFastClient(slow time.Duration, current, peak *atomic.Int32, calls *sync.Map) *http.Client {
	return &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		n := current.Add(1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		defer current.Add(-1)
		actual, _ := calls.LoadOrStore(r.URL.Path, new(atomic.Int32))
		actual.(*atomic.Int32).Add(1)
		if r.URL.Path != "/fast" {
			time.Sleep(slow)
		}
		return workerHTTPResponse(http.StatusNoContent), nil
	})}
}

// ORB-023: slow targets run in parallel up to a bound; a fast check is not stuck behind them.
func TestSlowTargetsDoNotDelayFastCheckAndConcurrencyIsBounded(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	insertWorkerCoreOwner(t, database)
	var current, peak atomic.Int32
	var calls sync.Map
	app := NewApp(database, utils.NewLogger(), Options{
		WorkerID: "pool", CheckInterval: 20 * time.Millisecond, LeaseDuration: time.Minute,
		Concurrency: 6, HTTPClient: slowFastClient(250*time.Millisecond, &current, &peak, &calls), SweepBootGrace: -1,
	})
	old := time.Now().UTC().Add(-time.Hour)
	var slowNames []string
	for i := 0; i < 18; i++ {
		slowNames = append(slowNames, fmt.Sprintf("slow-%02d", i))
	}
	seedHTTPMonitors(t, app, slowNames, func(n string) string { return "https://example.com/" + n }, old.Add(time.Minute), 300)
	seedHTTPMonitors(t, app, []string{"fast"}, func(string) string { return "https://example.com/fast" }, old, 1) // due first, then every 1s

	ctx, cancel := context.WithTimeout(context.Background(), 2500*time.Millisecond)
	defer cancel()
	if err := app.Run(ctx); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	fastCalls, _ := calls.Load("/fast")
	if got := fastCalls.(*atomic.Int32).Load(); got < 2 {
		t.Fatalf("fast check ran %d times in 2.5s, want at least 2 (it was stuck behind slow targets)", got)
	}
	if peak.Load() > 6 {
		t.Fatalf("peak concurrent checks = %d, want at most the configured 6", peak.Load())
	}
	if peak.Load() < 3 {
		t.Fatalf("peak concurrent checks = %d, want checks to actually run in parallel", peak.Load())
	}
	var slowDone int
	for _, name := range slowNames {
		if _, ok := calls.Load("/" + name); ok {
			slowDone++
		}
	}
	if slowDone != 18 {
		t.Fatalf("%d of 18 slow monitors ran, want all (3 waves of 6)", slowDone)
	}
}

// A check slower than its lease keeps the lease alive and is not run twice.
func TestLeaseIsRenewedWhileLongCheckRuns(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	insertWorkerCoreOwner(t, database)
	var current, peak atomic.Int32
	var calls sync.Map
	app := NewApp(database, utils.NewLogger(), Options{
		WorkerID: "renew", LeaseDuration: 150 * time.Millisecond,
		HTTPClient: slowFastClient(700*time.Millisecond, &current, &peak, &calls),
	})
	seedHTTPMonitors(t, app, []string{"long"}, func(n string) string { return "https://example.com/" + n }, time.Now().UTC().Add(-time.Minute), 300)

	if err := app.dispatchDueChecks(t.Context(), t.Context()); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 6; i++ { // 6 x 150ms spans several lease durations while the check runs
		time.Sleep(150 * time.Millisecond)
		if err := app.dispatchDueChecks(t.Context(), t.Context()); err != nil {
			t.Fatal(err)
		}
	}
	app.inflight.Wait()

	got, _ := calls.Load("/long")
	if n := got.(*atomic.Int32).Load(); n != 1 {
		t.Fatalf("long check ran %d times, want exactly 1 (lease must be renewed)", n)
	}
}

func TestShutdownWaitsForInflightChecksAndReleasesLeases(t *testing.T) {
	database := openWorkerMigratedTestDatabase(t)
	insertWorkerCoreOwner(t, database)
	var current, peak atomic.Int32
	var calls sync.Map
	app := NewApp(database, utils.NewLogger(), Options{
		WorkerID: "shutdown", CheckInterval: 20 * time.Millisecond, LeaseDuration: time.Minute,
		HTTPClient: slowFastClient(300*time.Millisecond, &current, &peak, &calls), SweepBootGrace: -1,
	})
	seedHTTPMonitors(t, app, []string{"inflight"}, func(n string) string { return "https://example.com/" + n }, time.Now().UTC().Add(-time.Minute), 300)
	// A second monitor whose lease this worker holds but never started (claimed, then crashed/stopped).
	seedHTTPMonitors(t, app, []string{"orphan"}, func(n string) string { return "https://example.com/" + n }, time.Now().UTC().Add(time.Hour), 300)
	expires := time.Now().UTC().Add(time.Hour)
	database.Model(&db.CoreMonitorConfig{}).Where("monitor_id = ?", "orphan").Updates(map[string]any{"lease_owner": "shutdown", "lease_expires_at": &expires})

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond) // the check has started and is mid-flight
		cancel()
	}()
	if err := app.Run(ctx); err != nil {
		t.Fatal(err)
	}

	inflight := loadWorkerCoreMonitorConfig(t, database, "inflight")
	if inflight.LastRunAt == nil || inflight.LeaseOwner != "" {
		t.Fatalf("in-flight check = last_run %v lease %q, want it completed during shutdown grace", inflight.LastRunAt, inflight.LeaseOwner)
	}
	report := loadWorkerMonitorReport(t, database, "inflight")
	if report.Health != "up" {
		t.Fatalf("report health = %q, want up (not a failure caused by shutdown cancel)", report.Health)
	}
	orphan := loadWorkerCoreMonitorConfig(t, database, "orphan")
	if orphan.LeaseOwner != "" || orphan.LeaseExpiresAt != nil {
		t.Fatalf("orphan lease = %q/%v, want released on shutdown", orphan.LeaseOwner, orphan.LeaseExpiresAt)
	}
}
