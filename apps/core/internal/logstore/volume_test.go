package logstore

import (
	"fmt"
	"testing"
	"time"

	"orion/core/internal/testkit"
	"orion/core/internal/utils"
)

func sized(app string, i int, at time.Time, size int) Entry {
	e := mkEntry(app, i, at)
	e.Size = size
	return e
}

// fastRefresh lets a stale snapshot refresh at once, so tests need not wait out the minimum age.
func fastRefresh(s *Store) {
	s.vol.mu.Lock()
	s.vol.fast = true
	s.vol.mu.Unlock()
}

func statsOf(t *testing.T, s *Store, app string) AppStats {
	t.Helper()
	all, err := s.StatsByApp(epoch)
	if err != nil {
		t.Fatal(err)
	}
	return all[app]
}

func TestStatsFollowWritesWithoutRescanning(t *testing.T) {
	store := newTestStore(t, 0)
	store.Enqueue([]Entry{sized("a", 0, epoch, 100), sized("a", 1, epoch, 150), sized("b", 0, epoch, 10)})
	store.Flush(5 * time.Second)

	if st := statsOf(t, store, "a"); st.Entries != 2 || st.Bytes != 250 {
		t.Fatalf("app a = %+v, want 2 entries and 250 bytes", st)
	}
	// The snapshot is built. Later writes must show up at once, with no new scan (a scan only starts
	// when the snapshot is old or stale, and neither holds here).
	store.Enqueue([]Entry{sized("a", 2, epoch, 50), sized("c", 0, epoch, 7)})
	store.Flush(5 * time.Second)
	if st := statsOf(t, store, "a"); st.Entries != 3 || st.Bytes != 300 {
		t.Fatalf("app a after a write = %+v, want 3 entries and 300 bytes", st)
	}
	if st := statsOf(t, store, "c"); st.Entries != 1 || st.Bytes != 7 {
		t.Fatalf("new app c = %+v, want 1 entry and 7 bytes", st)
	}
	one, err := store.StatsForApp("a", epoch)
	if err != nil || one.Entries != 3 || one.Bytes != 300 {
		t.Fatalf("StatsForApp(a) = %+v, %v", one, err)
	}
	if u, err := store.Usage(epoch); err != nil || u.Entries != 5 || u.Bytes != 317 {
		t.Fatalf("Usage = %+v, %v, want 5 entries and 317 bytes", u, err)
	}
}

func TestStatsStayCorrectWhenARepeatedClientIDIsIgnored(t *testing.T) {
	store := newTestStore(t, 0)
	fastRefresh(store)
	first := sized("a", 0, epoch, 100)
	first.ClientID = "same"
	store.Enqueue([]Entry{first})
	store.Flush(5 * time.Second)
	statsOf(t, store, "a") // builds the snapshot

	again := sized("a", 1, epoch, 100)
	again.ClientID = "same"
	store.Enqueue([]Entry{again, sized("a", 2, epoch, 40)})
	store.Flush(5 * time.Second)

	// The batch had an ignored row, so the snapshot is stale and refreshes; wait for that scan.
	deadline := time.Now().Add(5 * time.Second)
	for {
		st := statsOf(t, store, "a")
		if st.Entries == 2 && st.Bytes == 140 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("app a = %+v, want 2 entries and 140 bytes (the repeated id stored once)", st)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestStatsDropADeletedAppAtOnceAndRefreshAfterPrune(t *testing.T) {
	store := newTestStore(t, 0)
	fastRefresh(store)
	var old []Entry
	for i := 0; i < 5; i++ {
		old = append(old, sized("a", i, epoch.AddDate(0, 0, -30), 10))
	}
	store.Enqueue(append(old, sized("a", 9, epoch, 10), sized("b", 0, epoch, 20)))
	store.Flush(5 * time.Second)
	if st := statsOf(t, store, "a"); st.Entries != 6 {
		t.Fatalf("app a = %+v, want 6", st)
	}

	if _, err := store.DeleteApp("b"); err != nil {
		t.Fatal(err)
	}
	if st := statsOf(t, store, "b"); st.Entries != 0 || st.Bytes != 0 {
		t.Fatalf("deleted app b still shows %+v", st)
	}

	if _, err := store.Prune([]AppRetention{{AppID: "a", RetentionDays: 14}}, epoch); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if st := statsOf(t, store, "a"); st.Entries == 1 && st.Bytes == 10 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("app a = %+v after prune, want the 1 recent entry", statsOf(t, store, "a"))
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestFacetsCountSeveritiesAppsAndEvents(t *testing.T) {
	store := newTestStore(t, 0)
	var batch []Entry
	for i := 0; i < 30; i++ {
		e := mkEntry("busy", i, epoch.Add(time.Duration(i)*time.Second))
		e.Severity, e.SeverityNumber = "error", 17
		if i%3 != 0 {
			e.Severity, e.SeverityNumber = "info", 9
		}
		if i%2 == 0 {
			e.Event = "checkout.failed"
		}
		batch = append(batch, e)
	}
	for i := 0; i < 5; i++ {
		e := mkEntry("quiet", i, epoch.Add(time.Duration(i)*time.Second))
		e.Event = "job.done"
		batch = append(batch, e)
	}
	// An entry a long time before the range must not count.
	batch = append(batch, mkEntry("busy", 99, epoch.AddDate(0, 0, -3)))
	store.Enqueue(batch)
	store.Flush(5 * time.Second)

	for _, ranged := range []bool{false, true} {
		q := Query{}
		if ranged {
			q.From, q.To = epoch.Add(-time.Hour), epoch.Add(time.Hour)
		}
		f, err := store.Facets(q, 10)
		if err != nil {
			t.Fatal(err)
		}
		wantTotal := int64(35)
		if !ranged {
			wantTotal = 36
		}
		if f.Total != wantTotal {
			t.Fatalf("ranged=%v total = %d, want %d", ranged, f.Total, wantTotal)
		}
		if f.Severity["error"] != 10 {
			t.Fatalf("ranged=%v error count = %d, want 10", ranged, f.Severity["error"])
		}
		if len(f.Apps) != 2 || f.Apps[0].Value != "busy" || f.Apps[1].Value != "quiet" || f.Apps[1].Count != 5 {
			t.Fatalf("ranged=%v apps = %+v, want busy then quiet(5)", ranged, f.Apps)
		}
		if f.EventsDistinct != 2 || f.Events[0] != (Count{Value: "checkout.failed", Count: 15}) || f.Events[1] != (Count{Value: "job.done", Count: 5}) {
			t.Fatalf("ranged=%v events = %+v (%d distinct)", ranged, f.Events, f.EventsDistinct)
		}
	}
	// With an app filter the counts cover that app only.
	f, err := store.Facets(Query{AppIDs: []string{"quiet"}, From: epoch.Add(-time.Hour), To: epoch.Add(time.Hour)}, 10)
	if err != nil || f.Total != 5 || len(f.Apps) != 1 {
		t.Fatalf("app-filtered facets = %+v, %v", f, err)
	}
}

func TestFacetsOverManyAppsReturnTheFiftyBusiest(t *testing.T) {
	store := newTestStore(t, 0)
	var batch []Entry
	for a := 0; a < 60; a++ {
		for i := 0; i <= a; i++ {
			batch = append(batch, mkEntry(fmt.Sprintf("app-%02d", a), i, epoch))
		}
	}
	store.Enqueue(batch)
	store.Flush(10 * time.Second)
	f, err := store.Facets(Query{From: epoch.Add(-time.Hour), To: epoch.Add(time.Hour)}, 10)
	if err != nil || len(f.Apps) != 50 || f.Apps[0].Value != "app-59" || f.Apps[49].Value != "app-10" {
		t.Fatalf("apps = %d first %v err %v, want the 50 busiest, app-59 first and app-10 last", len(f.Apps), f.Apps[0], err)
	}
}

func TestAScanThatStartedBeforeADeleteLeavesTheSnapshotStale(t *testing.T) {
	store := newTestStore(t, 0)
	store.Enqueue([]Entry{sized("a", 0, epoch, 10)})
	store.Flush(5 * time.Second)
	statsOf(t, store, "a") // first scan done
	c := &store.vol

	scan := func(deleteDuringScan bool) bool {
		c.mu.Lock()
		if c.loading != nil {
			c.mu.Unlock()
			t.Fatal("a scan is still running")
		}
		store.startVolumeScanLocked() // the scan goroutine waits for c.mu once its query has run
		done := c.loading
		if deleteDuringScan {
			c.stale, c.gen = true, c.gen+1 // what volumeShrunk does while the scan is in flight
		}
		c.mu.Unlock()
		<-done
		c.mu.Lock()
		defer c.mu.Unlock()
		return c.stale
	}

	// The scan began before the delete, so its answer may still hold the deleted rows.
	if !scan(true) {
		t.Fatal("a scan that began before a delete cleared the stale mark")
	}
	// With nothing happening during it, a scan clears a stale snapshot.
	c.mu.Lock()
	c.stale = true
	c.mu.Unlock()
	if scan(false) {
		t.Fatal("a scan with no delete during it left the snapshot stale")
	}
}

func TestASavedSnapshotAnswersAtOnceAfterARestart(t *testing.T) {
	database := testkit.NewDB(t)
	first, err := OpenWith(database, utils.NewLogger(), 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(first.Close)
	first.Enqueue([]Entry{sized("a", 0, epoch, 10), sized("a", 1, epoch, 20), sized("a", 2, epoch, 30)})
	first.Flush(5 * time.Second)
	statsOf(t, first, "a") // the first scan, which also saves the snapshot

	deadline := time.Now().Add(5 * time.Second)
	for {
		var n int64
		database.Raw("SELECT COUNT(*) FROM log_app_volume").Scan(&n)
		if n == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the snapshot was not saved")
		}
		time.Sleep(10 * time.Millisecond)
	}

	// The rows go behind the first store's back, then a second store starts on the same file, as after a
	// restart. It must answer from what was saved without scanning first, and then correct itself.
	database.Exec("DELETE FROM log_entries")
	second, err := OpenWith(database, utils.NewLogger(), 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(second.Close)
	fastRefresh(second)
	if st := statsOf(t, second, "a"); st.Entries != 3 || st.Bytes != 60 {
		t.Fatalf("after a restart app a = %+v, want the saved 3 entries and 60 bytes at once", st)
	}
	deadline = time.Now().Add(5 * time.Second)
	for statsOf(t, second, "a").Entries != 0 {
		if time.Now().After(deadline) {
			t.Fatal("the saved snapshot was never refreshed")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestWithoutASnapshotASlowFirstScanGetsAnEstimateNotAHang(t *testing.T) {
	store := newTestStore(t, 0)
	store.Enqueue([]Entry{sized("a", 0, epoch, 100), sized("a", 1, epoch, 100), sized("a", 2, epoch, 100), sized("b", 0, epoch, 50)})
	store.Flush(5 * time.Second)

	// Pretend the first scan is still running and will not end during the test.
	store.vol.mu.Lock()
	store.vol.loading = make(chan struct{})
	store.vol.mu.Unlock()

	started := time.Now()
	all, err := store.StatsByApp(epoch)
	if err != nil {
		t.Fatal(err)
	}
	if took := time.Since(started); took < volumeFirstWait || took > volumeFirstWait+3*time.Second {
		t.Fatalf("the reader waited %v, want about %v and then an answer", took, volumeFirstWait)
	}
	if a := all["a"]; a.Entries != 3 || a.Bytes != 300 {
		t.Fatalf("estimate for a = %+v, want 3 entries and 300 bytes", a)
	}
	if b := all["b"]; b.Entries != 1 || b.Bytes != 50 {
		t.Fatalf("estimate for b = %+v, want 1 entry and 50 bytes", b)
	}
	store.vol.mu.Lock()
	ready := store.vol.ready
	store.vol.mu.Unlock()
	if ready {
		t.Fatal("an estimate must not be kept as the snapshot")
	}
}
