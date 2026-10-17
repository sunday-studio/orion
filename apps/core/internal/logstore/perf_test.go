package logstore

import (
	"fmt"
	"os"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"
)

func envNumber(name string, fallback int) int {
	if v, err := strconv.Atoi(os.Getenv(name)); err == nil && v > 0 {
		return v
	}
	return fallback
}

// Plan gate: sustained ingest with p95 under 100 ms and nothing dropped. Default is a
// short run; ORION_LOGS_PERF_ENTRIES_PER_SEC / ORION_LOGS_PERF_SECONDS scale it.
func TestSustainedIngestThroughputAndLatency(t *testing.T) {
	perSecond := envNumber("ORION_LOGS_PERF_ENTRIES_PER_SEC", 2000)
	seconds := envNumber("ORION_LOGS_PERF_SECONDS", 5)
	store := newTestStore(t, 1_000_000)

	const producers, batch = 10, 25
	ticks := perSecond / (producers * batch) // batches per producer per second
	var mu sync.Mutex
	var latencies []time.Duration
	var wg sync.WaitGroup
	start := time.Now()
	for p := 0; p < producers; p++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ticker := time.NewTicker(time.Second / time.Duration(max(ticks, 1)))
			defer ticker.Stop()
			for n := 0; n < ticks*seconds; n++ {
				<-ticker.C
				entries := make([]Entry, batch)
				for i := range entries {
					entries[i] = mkEntry(fmt.Sprintf("app-%d", p%3), n*batch+i, time.Now().UTC())
				}
				t0 := time.Now()
				if err := store.Enqueue(entries); err != nil {
					t.Errorf("enqueue under sustained load: %v", err)
					return
				}
				mu.Lock()
				latencies = append(latencies, time.Since(t0))
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	store.Flush(60 * time.Second)
	elapsed := time.Since(start)

	want := int64(producers * batch * ticks * seconds)
	st := store.Stats()
	slices.Sort(latencies)
	p95 := latencies[len(latencies)*95/100]
	t.Logf("%d entries in %v (%.0f/s): written=%d dropped=%d enqueue p95=%v", want, elapsed.Round(time.Millisecond), float64(want)/elapsed.Seconds(), st.Written, st.Dropped, p95)
	if st.Written != want || st.Dropped != 0 {
		t.Fatalf("written=%d dropped=%d, want all %d written", st.Written, st.Dropped, want)
	}
	if p95 > 100*time.Millisecond {
		t.Fatalf("ingest p95 = %v, want under 100ms", p95)
	}
}

// Plan gate: filtered list under 300 ms at volume. ORION_LOGS_PERF_ROWS scales it (10M for the full gate).
func TestFilteredListLatencyAtVolume(t *testing.T) {
	rows := envNumber("ORION_LOGS_PERF_ROWS", 300_000)
	store := newTestStore(t, 0)
	base := epoch.Add(-24 * time.Hour).UnixNano()
	step := int64(24*time.Hour) / int64(rows)
	sql := fmt.Sprintf(`WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i+1 FROM n WHERE i < %d)
		INSERT INTO log_entries (id, app_id, ts, ingested_at, severity, severity_number, event, body, attributes, size)
		SELECT 'log_' || printf('%%012d', i), 'app-' || (i %% 5), %d + i * %d, 0,
		       CASE WHEN i %% 50 = 0 THEN 'error' ELSE 'info' END, CASE WHEN i %% 50 = 0 THEN 17 ELSE 9 END,
		       CASE WHEN i %% 7 = 0 THEN 'order.created' ELSE '' END, 'request handled ' || i,
		       '{"http.status_code":' || (200 + (i %% 3) * 100) || ',"shard":' || (i %% 4) || '}', 100 FROM n`, rows, base, step)
	if err := store.DB().Exec(sql).Error; err != nil {
		t.Fatal(err)
	}
	t.Logf("seeded %d rows", rows)

	slowest := time.Duration(0)
	for name, q := range map[string]Query{
		"newest page":       {Limit: 50},
		"one app":           {AppIDs: []string{"app-2"}, Limit: 50},
		"app + errors":      {AppIDs: []string{"app-0"}, Severity: ">=error", Limit: 50},
		"app + event":       {AppIDs: []string{"app-1"}, Event: "order.created", Limit: 50},
		"app + time window": {AppIDs: []string{"app-3"}, From: epoch.Add(-2 * time.Hour), To: epoch.Add(-time.Hour), Limit: 50},
		"deep page":         {AppIDs: []string{"app-4"}, Limit: 200},
		"attribute filter":  {AppIDs: []string{"app-4"}, Attrs: []string{"http.status_code:400"}, From: epoch.Add(-3 * time.Hour), Limit: 50},
	} {
		t0 := time.Now()
		got, _, err := store.List(q)
		d := time.Since(t0)
		if err != nil || len(got) == 0 {
			t.Fatalf("%s: rows=%d err=%v", name, len(got), err)
		}
		t.Logf("%-18s %v (%d rows)", name, d.Round(time.Microsecond), len(got))
		slowest = max(slowest, d)
	}
	if slowest > 300*time.Millisecond {
		t.Fatalf("slowest filtered list = %v, want under 300ms", slowest)
	}
}

// Plan gate: pruning never blocks ingest for more than a second.
func TestPruneDoesNotBlockIngest(t *testing.T) {
	store := newTestStore(t, 1_000_000)
	old := epoch.AddDate(0, 0, -60).UnixNano()
	sql := fmt.Sprintf(`WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i+1 FROM n WHERE i < 150000)
		INSERT INTO log_entries (id, app_id, ts, ingested_at, severity, severity_number, body, size)
		SELECT 'old_' || i, 'a', %d + i, 0, 'info', 9, 'old', 100 FROM n`, old)
	if err := store.DB().Exec(sql).Error; err != nil {
		t.Fatal(err)
	}

	done := make(chan PruneResult, 1)
	go func() {
		r, _ := store.Prune([]AppRetention{{AppID: "a", RetentionDays: 14}}, epoch)
		done <- r
	}()
	var worst time.Duration
	deadline := time.After(60 * time.Second)
	for finished := false; !finished; {
		select {
		case r := <-done:
			if r.Expired != 150000 {
				t.Fatalf("expired = %d, want 150000", r.Expired)
			}
			finished = true
		case <-deadline:
			t.Fatal("prune did not finish")
		default:
			t0 := time.Now()
			if err := store.Enqueue([]Entry{mkEntry("b", 1, time.Now().UTC())}); err != nil {
				t.Fatalf("ingest refused during prune: %v", err)
			}
			store.Flush(5 * time.Second) // time to durable, i.e. the writer got the lock
			worst = max(worst, time.Since(t0))
			time.Sleep(5 * time.Millisecond)
		}
	}
	t.Logf("worst enqueue-to-written during a 150k-row prune: %v", worst)
	if worst > time.Second {
		t.Fatalf("ingest was blocked for %v during prune, want under 1s", worst)
	}
}
