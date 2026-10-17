package soak

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"orion/core/internal/logstore"
	"orion/core/internal/utils"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestCrossProcessLoad runs the real API and Worker binaries against one data directory and
// drives them the way production does: heavy log ingest, live tails, monitor heartbeats, a
// log-alert rule the Worker evaluates, and read queries, all concurrently. It is opt-in:
//
//	ORION_CROSS_PROCESS=1 ORION_CROSS_DURATION=30m go test ./internal/soak -run TestCrossProcessLoad -timeout 40m -v
//
// The 72-hour soak is the same test with ORION_CROSS_DURATION=72h (see docs/plans/backend-hardening-load-results.md).
func TestCrossProcessLoad(t *testing.T) {
	if os.Getenv("ORION_CROSS_PROCESS") == "" {
		t.Skip("set ORION_CROSS_PROCESS=1 to run the cross-process load test")
	}
	duration := envDuration("ORION_CROSS_DURATION", 60*time.Second)
	rate := envInt("ORION_CROSS_ENTRIES_PER_SEC", 2000)
	tails := envInt("ORION_CROSS_TAILS", 5)
	heartbeats := envInt("ORION_CROSS_HEARTBEATS", 20)
	const writers, batch = 4, 50

	dir := t.TempDir()
	dataDir := filepath.Join(dir, "data")
	binDir := filepath.Join(dir, "bin")
	_ = os.MkdirAll(binDir, 0o755)
	buildBinary(t, filepath.Join(binDir, "api"), "./cmd/api")
	buildBinary(t, filepath.Join(binDir, "worker"), "./cmd/worker")

	hook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hookCalls.Add(1); w.WriteHeader(204) }))
	defer hook.Close()
	port := crossFreePort(t)
	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	env := []string{"ORION_DATA_DIR=" + dataDir, fmt.Sprintf("ORION_PORT=%d", port), "ORION_ALLOW_INSECURE_NO_AUTH=true",
		"ORION_ALERT_ALLOW_PRIVATE_WEBHOOKS=true", "ORION_CORE_MONITOR_ALLOW_PRIVATE_TARGETS=true", "ORION_LOGS_APP_RATE_PER_SECOND=1000000",
		"ORION_ALERT_COOLDOWN_SECONDS=1", "ORION_WORKER_HEARTBEAT_SECONDS=5"}
	api := startProc(t, dir, "api", filepath.Join(binDir, "api"), env)
	crossWaitHealthy(t, base)
	worker := startProc(t, dir, "worker", filepath.Join(binDir, "worker"), env)

	// Fixtures: an app + key, a webhook channel, a log-alert rule, heartbeat monitors.
	var appResp struct {
		Data struct{ App struct{ ID string } }
	}
	crossPost(t, base, "/v1/apps", map[string]any{"name": "soak-app", "retention_days": 1, "max_storage_mb": 4096}, &appResp)
	appID := appResp.Data.App.ID
	var keyResp struct {
		Data struct{ Key struct{ Key string } }
	}
	crossPost(t, base, fmt.Sprintf("/v1/apps/%s/keys", appID), map[string]any{"name": "soak"}, &keyResp)
	key := keyResp.Data.Key.Key
	crossPost(t, base, "/v1/alerts/channels", map[string]any{"name": "soak-hook", "type": "webhook", "webhook_url": hook.URL}, nil)
	crossPost(t, base, "/v1/logs/alerts", map[string]any{"name": "soak errors", "app_id": appID, "mode": "threshold", "severity": "error", "threshold": 5, "window_seconds": 60}, nil)
	var beatTokens []string
	for i := 0; i < heartbeats; i++ {
		var mr struct {
			Data struct {
				Config struct {
					HeartbeatToken string `json:"heartbeat_token"`
				}
			}
		}
		crossPost(t, base, "/v1/monitors", map[string]any{"name": fmt.Sprintf("beat-%d", i), "kind": "heartbeat", "interval_seconds": 60, "config": map[string]any{"grace_seconds": 120}}, &mr)
		beatTokens = append(beatTokens, mr.Data.Config.HeartbeatToken)
	}

	stopAt := time.Now().Add(duration)
	var stop atomic.Bool
	var wg sync.WaitGroup
	var accepted, rejected, non202, throttled atomic.Int64
	var latMu sync.Mutex
	var lats, readLats, histLats []time.Duration
	record := func(into *[]time.Duration, d time.Duration) { latMu.Lock(); *into = append(*into, d); latMu.Unlock() }
	client := &http.Client{Timeout: 30 * time.Second}

	interval := time.Duration(float64(time.Second) * float64(batch*writers) / float64(rate))
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			time.Sleep(time.Duration(w) * interval / writers) // spread writers; do not start in lockstep
			tick := time.NewTicker(interval)
			defer tick.Stop()
			seq := 0
			for range tick.C {
				if stop.Load() {
					return
				}
				var body bytes.Buffer
				for i := 0; i < batch; i++ {
					seq++
					sev := "info"
					if seq%97 == 0 {
						sev = "error"
					}
					fmt.Fprintf(&body, `{"severity":%q,"body":"request handled","event":"http.request","attributes":{"http.status_code":%d,"route":"/r/%d","user.id":%d,"writer":%d}}`+"\n", sev, 200+(seq%3)*100, seq%50, seq%5000, w)
				}
				req, _ := http.NewRequest(http.MethodPost, base+"/v1/ingest/logs", &body)
				req.Header.Set("Authorization", "Bearer "+key)
				req.Header.Set("Content-Type", "application/x-ndjson")
				start := time.Now()
				resp, err := client.Do(req)
				if err != nil {
					non202.Add(1)
					continue
				}
				var ir struct{ Accepted, Rejected int }
				_ = json.NewDecoder(resp.Body).Decode(&ir)
				resp.Body.Close()
				record(&lats, time.Since(start))
				switch resp.StatusCode {
				case 202:
					accepted.Add(int64(ir.Accepted))
					rejected.Add(int64(ir.Rejected))
				case 429, 503:
					throttled.Add(1)
				default:
					non202.Add(1)
				}
			}
		}(w)
	}

	// Live tails: long-lived SSE clients that must keep receiving while ingest runs.
	tailCounts := make([]atomic.Int64, tails)
	for i := 0; i < tails; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			resp, err := http.Get(fmt.Sprintf("%s/v1/logs/stream?app_id=%s", base, appID))
			if err != nil {
				return
			}
			defer resp.Body.Close()
			sc := bufio.NewScanner(resp.Body)
			sc.Buffer(make([]byte, 1<<20), 1<<20)
			for sc.Scan() {
				if strings.HasPrefix(sc.Text(), "data:") {
					tailCounts[i].Add(1)
				}
				if stop.Load() {
					return
				}
			}
		}(i)
	}

	// Heartbeats write the main database through the API while the Worker writes it too.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for !stop.Load() {
			for _, tok := range beatTokens {
				resp, err := client.Post(base+"/v1/heartbeats/"+tok, "application/json", strings.NewReader(`{}`))
				if err == nil {
					resp.Body.Close()
				}
			}
			time.Sleep(time.Second)
		}
	}()

	// Reads: list + histogram, timed.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for !stop.Load() {
			for _, path := range []string{"/v1/logs?app_id=" + appID + "&severity=error&limit=50", "/v1/logs/histogram?app_id=" + appID} {
				start := time.Now()
				if resp, err := client.Get(base + path); err == nil {
					resp.Body.Close()
					if strings.Contains(path, "histogram") {
						record(&histLats, time.Since(start)) // aggregates every row in the window, so it scales with volume
					} else {
						record(&readLats, time.Since(start))
					}
				}
			}
			time.Sleep(time.Second)
		}
	}()

	type sample struct {
		At                      time.Duration
		APIRSS, WorkerRSS       int
		LogsDB, LogsWAL, MainDB int64
	}
	var samples []sample
	begin := time.Now()
	sampleEvery := max(duration/60, 5*time.Second)
	for time.Now().Before(stopAt) {
		time.Sleep(min(sampleEvery, time.Until(stopAt)))
		s := sample{At: time.Since(begin).Round(time.Second), APIRSS: api.rssKB(), WorkerRSS: worker.rssKB(),
			LogsDB: crossFileSize(filepath.Join(dataDir, "logs.db")), LogsWAL: crossFileSize(filepath.Join(dataDir, "logs.db-wal")), MainDB: crossFileSize(filepath.Join(dataDir, "orion.db"))}
		samples = append(samples, s)
		t.Logf("t=%v accepted=%d api_rss=%dMB worker_rss=%dMB logs.db=%dMB wal=%dMB tails=%v", s.At, accepted.Load(), s.APIRSS/1024, s.WorkerRSS/1024, s.LogsDB>>20, s.LogsWAL>>20, tailTotals(tailCounts))
	}
	stop.Store(true)

	// Graceful shutdown with tails still attached must be prompt (BaseContext cancels streams).
	time.Sleep(2 * time.Second)
	if err := api.stop(30 * time.Second); err != nil {
		t.Errorf("API shutdown: %v", err)
	}
	if err := worker.stop(30 * time.Second); err != nil {
		t.Errorf("Worker shutdown: %v", err)
	}
	wg.Wait()

	// Everything the API acknowledged must be in logs.db, and the file must be intact.
	reader, err := logstore.OpenReader(dataDir, utils.NewLogger())
	if err != nil {
		t.Fatal(err)
	}
	stored, _, _ := reader.CountForApp(appID)
	var integrity string
	reader.DB().Raw("PRAGMA integrity_check").Scan(&integrity)

	p50, p95, p99 := pct(lats, 0.5), pct(lats, 0.95), pct(lats, 0.99)
	r95, h95 := pct(readLats, 0.95), pct(histLats, 0.95)
	t.Logf("RESULT duration=%v accepted=%d stored=%d rejected=%d throttled=%d other_errors=%d ingest p50=%v p95=%v p99=%v read p95=%v histogram p95=%v webhook_calls=%d tails=%v integrity=%s",
		duration, accepted.Load(), stored, rejected.Load(), throttled.Load(), non202.Load(), p50, p95, p99, r95, h95, hookCalls.Load(), tailTotals(tailCounts), integrity)
	if out := os.Getenv("ORION_CROSS_OUT"); out != "" {
		blob, _ := json.MarshalIndent(map[string]any{"duration": duration.String(), "accepted": accepted.Load(), "stored": stored, "throttled": throttled.Load(), "other_errors": non202.Load(),
			"ingest_p50_ms": p50.Milliseconds(), "ingest_p95_ms": p95.Milliseconds(), "ingest_p99_ms": p99.Milliseconds(), "read_p95_ms": r95.Milliseconds(), "histogram_p95_ms": h95.Milliseconds(),
			"webhook_calls": hookCalls.Load(), "tail_events": tailTotals(tailCounts), "samples": samples}, "", "  ")
		_ = os.WriteFile(out, blob, 0o644)
	}

	if stored != accepted.Load() {
		t.Errorf("stored %d rows but the API acknowledged %d", stored, accepted.Load())
	}
	if integrity != "ok" {
		t.Errorf("logs.db integrity_check = %q", integrity)
	}
	if non202.Load() > 0 {
		t.Errorf("%d ingest requests failed with a non-202/429/503 outcome", non202.Load())
	}
	if total := int64(len(lats)); total > 0 && float64(throttled.Load())/float64(total) > 0.01 {
		t.Errorf("%d of %d ingest requests were throttled at %d entries/s", throttled.Load(), total, rate)
	}
	if p95 > 250*time.Millisecond {
		t.Errorf("ingest p95 = %v, want <= 250ms", p95)
	}
	if r95 > time.Second {
		t.Errorf("read p95 = %v, want <= 1s", r95)
	}
	if h95 > 5*time.Second {
		t.Errorf("histogram p95 = %v, want <= 5s", h95)
	}
	if hookCalls.Load() == 0 {
		t.Error("the Worker never fired the log-alert webhook")
	}
	for i := range tailCounts {
		if tailCounts[i].Load() == 0 {
			t.Errorf("live tail %d received nothing", i)
		}
	}
	for _, p := range []*proc{api, worker} {
		if hits := p.logContains("database is locked", "SQLITE_BUSY", "panic:", "Dropped a batch"); len(hits) > 0 {
			t.Errorf("%s log contains %v (see %s)", p.name, hits, p.log)
		}
	}
	if duration >= 10*time.Minute && len(samples) >= 6 {
		third := len(samples) / 3
		for name, pick := range map[string]func(sample) int{"api": func(s sample) int { return s.APIRSS }, "worker": func(s sample) int { return s.WorkerRSS }} {
			early, late := avg(samples[:third], pick), avg(samples[len(samples)-third:], pick)
			if early > 0 && late > 2*early {
				t.Errorf("%s RSS grew from %dMB to %dMB over the run", name, early/1024, late/1024)
			}
		}
	}
}

var hookCalls atomic.Int64

func tailTotals(c []atomic.Int64) []int64 {
	out := make([]int64, len(c))
	for i := range c {
		out[i] = c[i].Load()
	}
	return out
}

func pct(d []time.Duration, q float64) time.Duration {
	if len(d) == 0 {
		return 0
	}
	s := append([]time.Duration(nil), d...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	return s[int(float64(len(s)-1)*q)]
}

func avg[T any](xs []T, f func(T) int) int {
	if len(xs) == 0 {
		return 0
	}
	sum := 0
	for _, x := range xs {
		sum += f(x)
	}
	return sum / len(xs)
}
