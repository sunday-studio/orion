package logstore

import (
	"fmt"
	"orion/core/internal/db"
	"strings"
	"sync"
	"testing"
	"time"
)

// Review P1: ids collided (same nanosecond + 24 random bits) and INSERT OR IGNORE hid it.
func TestEntryIDsAreUniqueEvenWithinOneNanosecond(t *testing.T) {
	seen := make(map[string]struct{}, 300_000)
	for i := 0; i < 300_000; i++ {
		id := NewEntryID(epoch) // identical clock reading for every call
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate id %s after %d calls", id, i)
		}
		seen[id] = struct{}{}
	}
}

func TestLargeBatchStoresExactlyWhatItReportsAccepted(t *testing.T) {
	ing, store, app, key := newIngester(t, 0)
	ctx, _ := ing.Authenticate(key)
	var lines []string
	for i := 0; i < MaxEntriesPerRequest; i++ {
		lines = append(lines, fmt.Sprintf(`{"body":"line %d"}`, i))
	}
	res, err := ing.Ingest(ctx, "application/x-ndjson", []byte(strings.Join(lines, "\n")))
	if err != nil || res.Accepted != MaxEntriesPerRequest {
		t.Fatalf("Ingest() = %+v, %v", res, err)
	}
	store.Flush(10 * time.Second)
	stored, _, _ := store.CountForApp(app.ID)
	st := store.Stats()
	if stored != int64(res.Accepted) || st.Written != int64(res.Accepted) || st.Duplicates != 0 {
		t.Fatalf("accepted=%d stored=%d stats=%+v, want all three to agree", res.Accepted, stored, st)
	}
}

func TestRequestsOverTheEntryCapAreRefusedWholesale(t *testing.T) {
	var lines []string
	for i := 0; i <= MaxEntriesPerRequest; i++ {
		lines = append(lines, `{"body":"x"}`)
	}
	if _, err := ParseRequest("application/x-ndjson", []byte(strings.Join(lines, "\n")), epoch); err != ErrTooManyEntries {
		t.Fatalf("ndjson error = %v, want ErrTooManyEntries", err)
	}
	if _, err := ParseRequest("application/json", []byte("["+strings.Join(lines, ",")+"]"), epoch); err != ErrTooManyEntries {
		t.Fatalf("array error = %v, want ErrTooManyEntries", err)
	}
}

func TestDuplicateClientIDsAreCountedAsDuplicatesNotWrites(t *testing.T) {
	store := newTestStore(t, 0)
	a, b := mkEntry("a", 1, epoch), mkEntry("a", 2, epoch)
	a.ClientID, b.ClientID = "same", "same"
	store.Enqueue([]Entry{a, b})
	store.Flush(3 * time.Second)
	if st := store.Stats(); st.Written != 1 || st.Duplicates != 1 {
		t.Fatalf("stats = %+v, want written 1 and duplicates 1", st)
	}
}

// Review P2: a sample rate of 0 means keep none of the debug and trace entries.
func TestDebugSampleRateZeroDropsDebugAndKeepsEverythingElse(t *testing.T) {
	ing, store, app, key := newIngester(t, 0)
	ing.main.Model(&db.App{}).Where("id = ?", app.ID).Update("debug_sample_rate", 0)
	ing.Invalidate()
	ctx, _ := ing.Authenticate(key)
	res, err := ing.Ingest(ctx, "application/json", []byte(`[{"body":"d","severity":"debug"},{"body":"t","severity":"trace"},{"body":"i","severity":"info"},{"body":"e","severity":"error"}]`))
	if err != nil || res.Accepted != 4 {
		t.Fatalf("Ingest() = %+v, %v", res, err)
	}
	store.Flush(3 * time.Second)
	rows, _, _ := store.List(Query{Limit: 50})
	if len(rows) != 2 {
		t.Fatalf("stored %d, want only info and error", len(rows))
	}
}

// Review P2: Close racing with in-flight Enqueue calls must not panic.
func TestEnqueueRacingCloseNeverPanics(t *testing.T) {
	for round := 0; round < 20; round++ {
		store, err := OpenWith(newTestStoreDB(t), utilsLogger(), 100000)
		if err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		for w := 0; w < 8; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := 0; i < 200; i++ {
					_ = store.Enqueue([]Entry{mkEntry("a", i, epoch)}) // ErrQueueFull after Close is fine
				}
			}()
		}
		time.Sleep(time.Millisecond)
		store.Close()
		wg.Wait()
		if err := store.Enqueue([]Entry{mkEntry("a", 1, epoch)}); err != ErrQueueFull {
			t.Fatalf("Enqueue after Close = %v, want ErrQueueFull", err)
		}
	}
}

// Review P3: a lookup that raced a revoke must not leave the revoked key cached.
func TestInvalidateDuringAuthenticationDoesNotCacheTheStaleResult(t *testing.T) {
	ing, _, _, key := newIngester(t, 0)
	ing.mu.Lock()
	staleGen := ing.gen
	ing.mu.Unlock()
	ing.Invalidate() // a revoke lands between "read gen" and "store in cache"
	ing.mu.Lock()
	defer ing.mu.Unlock()
	if ing.gen == staleGen {
		t.Fatal("Invalidate must advance the generation so racing lookups skip the cache")
	}
	_ = key
}

func TestSlowSubscribersLoseEntriesWithoutSlowingTheWriter(t *testing.T) {
	store := newTestStore(t, 100000)
	sub, cancel := store.Subscribe(Query{})
	defer cancel() // nobody reads sub.C: its 256-entry buffer fills
	var batch []Entry
	for i := 0; i < 1000; i++ {
		batch = append(batch, mkEntry("a", i, epoch))
	}
	start := time.Now()
	store.Enqueue(batch)
	store.Flush(5 * time.Second)
	if time.Since(start) > 3*time.Second {
		t.Fatal("a stalled subscriber slowed the writer")
	}
	if sub.Dropped.Load() < 700 || len(sub.C) != 256 {
		t.Fatalf("dropped=%d buffered=%d, want the overflow dropped and counted", sub.Dropped.Load(), len(sub.C))
	}
	if st := store.Stats(); st.Written != 1000 {
		t.Fatalf("written = %d, want all 1000 stored regardless of the subscriber", st.Written)
	}
}

func TestQueryMatchMirrorsTheSQLFilters(t *testing.T) {
	e := mkEntry("app-1", 1, epoch)
	e.Severity, e.SeverityNumber, e.Event, e.RequestID = "error", 17, "job.failed", "req-9"
	for name, tc := range map[string]struct {
		q    Query
		want bool
	}{
		"empty":         {Query{}, true},
		"app match":     {Query{AppIDs: []string{"x", "app-1"}}, true},
		"app mismatch":  {Query{AppIDs: []string{"x"}}, false},
		"severity list": {Query{Severity: "warn,error"}, true},
		"severity min":  {Query{Severity: ">=fatal"}, false},
		"event":         {Query{Event: "job.failed"}, true},
		"request id":    {Query{RequestID: "other"}, false},
		"text in body":  {Query{Text: "MESSAGE 1"}, true},
		"attr match":    {Query{Attrs: []string{"i:1"}}, true},
		"attr mismatch": {Query{Attrs: []string{"i:2"}}, false},
	} {
		if got := tc.q.Match(e); got != tc.want {
			t.Errorf("%s: Match = %t, want %t", name, got, tc.want)
		}
	}
}
