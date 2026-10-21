// Package soak runs Core end to end in one process (API, Worker, file-backed WAL
// SQLite) against simulated nodes, injected outages and a flaky webhook receiver,
// then checks the product promise: every outage produces exactly one "opened" and
// one "resolved" alert, with no database lock errors. The same harness backs the
// short CI run and the 72h staging soak (see docs/plans/backend-hardening-load-results.md).
package soak

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"orion/core/internal/api"
	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"orion/core/internal/worker"
	"os"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

// Config sizes a run. Zero values take the defaults noted.
type Config struct {
	Nodes           int           // simulated nodes (default 10)
	MonitorsPerNode int           // monitors per node (default 5)
	ReportInterval  time.Duration // per-monitor reporting interval (default 1s)
	Duration        time.Duration // how long nodes report (default 10s)
	Outages         int           // injected outages, each on its own node (default Nodes/2)
	OutageLength    time.Duration // how long each outage lasts (default 3 x ReportInterval)
	WebhookFailRate float64       // fraction of webhook calls answered 500 (default 0.05)
	Seed            int64
	DataDir         string // empty = temp dir
	FastBackoff     bool   // make delivery retries due immediately instead of after minutes
}

// minDuration is the shortest run in which every outage can start, last OutageLength,
// and recover with two more healthy reports before the nodes stop.
func (c Config) minDuration() time.Duration {
	return c.OutageLength + 4*c.ReportInterval
}

func (c *Config) defaults() {
	if c.Nodes <= 0 {
		c.Nodes = 10
	}
	if c.MonitorsPerNode <= 0 {
		c.MonitorsPerNode = 5
	}
	if c.ReportInterval <= 0 {
		c.ReportInterval = time.Second
	}
	if c.Duration <= 0 {
		c.Duration = 10 * time.Second
	}
	if c.Outages <= 0 {
		c.Outages = max(c.Nodes/2, 1)
	}
	if c.Outages > c.Nodes {
		c.Outages = c.Nodes
	}
	if c.OutageLength <= 0 {
		c.OutageLength = 2 * c.ReportInterval
	}
	if c.WebhookFailRate == 0 {
		c.WebhookFailRate = 0.05
	}
	if c.Seed == 0 {
		c.Seed = 1
	}
}

// Outage is one injected failure: the monitor reports down for a window, then up.
type Outage struct {
	NodeID, MonitorID string
	Start, End        time.Duration // offsets from the start of the run
}

// Result holds measurements and invariant violations from a run.
type Result struct {
	Reports       int64
	ReportErrors  int64
	P50, P95, P99 time.Duration
	SQLiteBusy    int64
	// ServerLatency is what Core measured per route in milliseconds, excluding client-side queueing.
	ServerLatency    map[string]int64
	DatabaseBytes    int64
	GoroutinesBefore int
	GoroutinesAfter  int
	WebhookCalls     int64
	WebhookFailures  int64
	Outages          []Outage
	Violations       []string
}

type nodeSim struct {
	id, token string
	monitors  []string
}

type receiver struct {
	failRate float64
	mu       sync.Mutex
	rng      *rand.Rand
	ok       map[string]int // "event|incident" -> successful deliveries
	calls    atomic.Int64
	failures atomic.Int64
}

func (r *receiver) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.calls.Add(1)
	body, _ := io.ReadAll(req.Body)
	r.mu.Lock()
	fail := r.rng.Float64() < r.failRate
	if !fail {
		var payload struct {
			EventType string `json:"event_type"`
			Incident  struct {
				ID string `json:"id"`
			} `json:"incident"`
		}
		if json.Unmarshal(body, &payload) == nil && payload.Incident.ID != "" {
			r.ok[payload.EventType+"|"+payload.Incident.ID]++
		}
	}
	r.mu.Unlock()
	if fail {
		r.failures.Add(1)
		http.Error(w, "injected failure", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Run executes the scenario and returns the measurements and any violations.
func Run(ctx context.Context, cfg Config) (*Result, error) {
	cfg.defaults()
	if cfg.Duration < cfg.minDuration() {
		return nil, fmt.Errorf("duration %v is too short for outages of %v at a %v interval; need at least %v", cfg.Duration, cfg.OutageLength, cfg.ReportInterval, cfg.minDuration())
	}
	dataDir := cfg.DataDir
	if dataDir == "" {
		var err error
		if dataDir, err = os.MkdirTemp("", "orion-soak-"); err != nil {
			return nil, err
		}
		defer os.RemoveAll(dataDir)
	}

	database, err := db.Initialize(dataDir, "silent")
	if err != nil {
		return nil, err
	}
	if err := db.Migrate(database); err != nil {
		return nil, err
	}
	sqlDB, _ := database.DB()
	defer sqlDB.Close()

	logger := utils.NewLoggerWithWriter(io.Discard)
	hook := &receiver{failRate: cfg.WebhookFailRate, rng: rand.New(rand.NewSource(cfg.Seed)), ok: map[string]int{}}
	hookServer := httptest.NewServer(hook)
	defer hookServer.Close()

	coreCfg := &config.Config{DataDir: dataDir, AlertRecoveryNotifications: true, AlertAllowPrivateWebhooks: true, AlertCooldownSeconds: 0, CoreWorkerStaleSeconds: 60}
	if err := database.Create(&db.AlertChannel{ID: "ch-soak", Name: "soak", Type: "webhook", Enabled: true, WebhookURL: hookServer.URL, SubscribedEvents: db.EncodeAlertEvents(db.DefaultAlertEvents())}).Error; err != nil {
		return nil, fmt.Errorf("create alert channel: %w", err)
	}
	apiServer := api.NewServer(database, logger, coreCfg)
	coreServer := httptest.NewServer(apiServer.Handler())
	defer coreServer.Close()

	workerCtx, stopWorker := context.WithCancel(ctx)
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		_ = worker.NewApp(database, logger, worker.Options{
			WorkerID: "soak-worker", Config: coreCfg, DispatchInterval: 100 * time.Millisecond, SweepBootGrace: -1,
		}).Run(workerCtx)
	}()

	result := &Result{GoroutinesBefore: runtime.NumGoroutine()}
	// Reuse connections like real nodes do; the default transport keeps only two idle
	// connections per host, which would turn this into a connection-churn benchmark.
	transport := &http.Transport{MaxIdleConns: 4096, MaxIdleConnsPerHost: 4096, IdleConnTimeout: 90 * time.Second}
	defer transport.CloseIdleConnections()
	client := &http.Client{Timeout: 10 * time.Second, Transport: transport}

	nodes, err := registerFleet(client, coreServer.URL, cfg)
	if err != nil {
		stopWorker()
		return nil, err
	}
	outages := planOutages(nodes, cfg)
	result.Outages = outages

	var latencies []time.Duration
	var latMu sync.Mutex
	started := time.Now()
	runCtx, stopNodes := context.WithTimeout(ctx, cfg.Duration)
	defer stopNodes()
	var wg sync.WaitGroup
	for _, a := range nodes {
		for _, monitorID := range a.monitors {
			wg.Add(1)
			go func() {
				defer wg.Done()
				simulateMonitor(runCtx, client, coreServer.URL, a, monitorID, cfg, outages, started, result, &latencies, &latMu)
			}()
		}
	}
	wg.Wait()

	waitForDrain(database, 60*time.Second, cfg.FastBackoff)
	stopWorker()
	<-workerDone

	latMu.Lock()
	slices.Sort(latencies)
	result.P50, result.P95, result.P99 = percentile(latencies, 0.50), percentile(latencies, 0.95), percentile(latencies, 0.99)
	latMu.Unlock()
	result.WebhookCalls, result.WebhookFailures = hook.calls.Load(), hook.failures.Load()
	measureDatabase(ctx, database, apiServer, dataDir, result)
	result.GoroutinesAfter = runtime.NumGoroutine()
	verify(database, hook, result)
	return result, nil
}

func registerFleet(client *http.Client, base string, cfg Config) ([]nodeSim, error) {
	nodes := make([]nodeSim, 0, cfg.Nodes)
	for i := 0; i < cfg.Nodes; i++ {
		var reg struct {
			Data struct {
				NodeID string `json:"node_id"`
				Token  string `json:"token"`
			} `json:"data"`
		}
		if err := postJSON(client, base+"/v1/register", "", map[string]any{
			"machine_id": fmt.Sprintf("soak-machine-%03d", i), "name": fmt.Sprintf("soak-%03d", i), "os": "linux", "arch": "amd64", "reporting_interval_seconds": int(cfg.ReportInterval.Seconds()),
		}, &reg); err != nil {
			return nil, fmt.Errorf("register node %d: %w", i, err)
		}
		a := nodeSim{id: reg.Data.NodeID, token: reg.Data.Token}
		for m := 0; m < cfg.MonitorsPerNode; m++ {
			var mon struct {
				Data struct {
					MonitorID string `json:"monitor_id"`
				} `json:"data"`
			}
			if err := postJSON(client, base+"/v1/nodes/"+a.id+"/register-monitor", a.token, map[string]any{
				"node_id": a.id, "name": fmt.Sprintf("monitor-%02d", m), "description": "soak monitor", "type": "http-healthcheck", "last_checked": time.Now().UTC().Format(time.RFC3339), "reporting_interval_seconds": max(int(cfg.ReportInterval.Seconds()), 1),
			}, &mon); err != nil {
				return nil, fmt.Errorf("register monitor %d/%d: %w", i, m, err)
			}
			a.monitors = append(a.monitors, mon.Data.MonitorID)
		}
		nodes = append(nodes, a)
	}
	return nodes, nil
}

// planOutages picks distinct nodes (alert grouping folds same-node incidents into
// one page, so one outage per node keeps "one alert per outage" well defined).
func planOutages(nodes []nodeSim, cfg Config) []Outage {
	rng := rand.New(rand.NewSource(cfg.Seed))
	order := rng.Perm(len(nodes))
	window := cfg.Duration - cfg.minDuration()
	outages := make([]Outage, 0, cfg.Outages)
	for i := 0; i < cfg.Outages; i++ {
		a := nodes[order[i]]
		start := cfg.ReportInterval + time.Duration(rng.Int63n(int64(max(window, time.Millisecond))))
		outages = append(outages, Outage{NodeID: a.id, MonitorID: a.monitors[rng.Intn(len(a.monitors))], Start: start, End: start + cfg.OutageLength})
	}
	return outages
}

func simulateMonitor(ctx context.Context, client *http.Client, base string, a nodeSim, monitorID string, cfg Config, outages []Outage, started time.Time, result *Result, latencies *[]time.Duration, mu *sync.Mutex) {
	// Stagger each monitor's phase across the interval, like real nodes that booted
	// at different moments. The ticker must start after the jitter, otherwise every
	// monitor fires in the same instant each interval (a thundering herd).
	jitter := time.Duration(rand.Int63n(int64(cfg.ReportInterval)))
	select {
	case <-ctx.Done():
		return
	case <-time.After(jitter):
	}
	ticker := time.NewTicker(cfg.ReportInterval)
	defer ticker.Stop()
	for {
		elapsed := time.Since(started)
		health := "up"
		for _, o := range outages {
			if o.MonitorID == monitorID && elapsed >= o.Start && elapsed < o.End {
				health = "down"
			}
		}
		t0 := time.Now()
		err := postJSON(client, base+"/v1/nodes/"+a.id+"/"+monitorID+"/report", a.token, map[string]any{
			"report_id": fmt.Sprintf("%s-%d", monitorID, time.Now().UnixNano()), "timestamp": time.Now().UTC().Format(time.RFC3339), "health": health, "metrics": map[string]any{"soak": true},
		}, nil)
		elapsedCall := time.Since(t0)
		atomic.AddInt64(&result.Reports, 1)
		if err != nil {
			atomic.AddInt64(&result.ReportErrors, 1)
		}
		mu.Lock()
		*latencies = append(*latencies, elapsedCall)
		mu.Unlock()
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func postJSON(client *http.Client, url string, token string, body any, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("%s: status %d: %s", url, resp.StatusCode, bytes.TrimSpace(data))
	}
	if out != nil {
		return json.Unmarshal(data, out)
	}
	return nil
}

func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	return sorted[min(int(float64(len(sorted))*p), len(sorted)-1)]
}
