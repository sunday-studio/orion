package logstore

import (
	"testing"
	"time"
)

func sevEntry(app string, i int, ts time.Time, severity string) Entry {
	e := mkEntry(app, i, ts)
	e.Severity = severity
	return e
}

func countsByKey(t *testing.T, store *Store, apps []string, from, to time.Time) map[string]int64 {
	t.Helper()
	rows, err := store.MinuteCounts(apps, from, to)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]int64{}
	for _, r := range rows {
		out[r.AppID+"|"+r.Minute.Format("15:04")+"|"+r.Severity] += r.Count
	}
	return out
}

func TestMinuteCountsFollowTheWriter(t *testing.T) {
	store := newTestStore(t, 0)
	base := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	batch := []Entry{
		sevEntry("app-1", 0, base.Add(5*time.Second), "error"),
		sevEntry("app-1", 1, base.Add(50*time.Second), "error"),
		sevEntry("app-1", 2, base.Add(51*time.Second), "info"),
		sevEntry("app-1", 3, base.Add(70*time.Second), "error"),
		sevEntry("app-2", 4, base.Add(5*time.Second), "warn"),
	}
	if err := store.Enqueue(batch); err != nil {
		t.Fatal(err)
	}
	store.Flush(5 * time.Second)
	// A second write into the same minute adds to the row.
	store.Enqueue([]Entry{sevEntry("app-1", 5, base.Add(20*time.Second), "error")})
	store.Flush(5 * time.Second)

	got := countsByKey(t, store, []string{"app-1", "app-2"}, base, base.Add(time.Hour))
	want := map[string]int64{"app-1|10:00|error": 3, "app-1|10:00|info": 1, "app-1|10:01|error": 1, "app-2|10:00|warn": 1}
	if len(got) != len(want) {
		t.Fatalf("counts = %v, want %v", got, want)
	}
	for k, n := range want {
		if got[k] != n {
			t.Fatalf("counts = %v, want %v", got, want)
		}
	}
	// Only the requested apps and the half-open range are returned.
	if only := countsByKey(t, store, []string{"app-2"}, base, base.Add(time.Hour)); len(only) != 1 {
		t.Fatalf("app-2 only = %v", only)
	}
	if none := countsByKey(t, store, []string{"app-1"}, base.Add(2*time.Minute), base.Add(time.Hour)); len(none) != 0 {
		t.Fatalf("after the data = %v", none)
	}
	if rows, err := store.MinuteCounts(nil, base, base.Add(time.Hour)); err != nil || len(rows) != 0 {
		t.Fatalf("no apps = %v, %v", rows, err)
	}
}

func TestMinuteCountsIgnoreRepeatedClientIDs(t *testing.T) {
	store := newTestStore(t, 0)
	base := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	first := sevEntry("app-1", 0, base, "error")
	first.ClientID = "evt-1"
	store.Enqueue([]Entry{first})
	store.Flush(5 * time.Second)

	// The client retries: same client id, new server id. The row is ignored and must not count again.
	retry := sevEntry("app-1", 1, base, "error")
	retry.ClientID = "evt-1"
	fresh := sevEntry("app-1", 2, base, "error")
	store.Enqueue([]Entry{retry, fresh})
	store.Flush(5 * time.Second)

	got := countsByKey(t, store, []string{"app-1"}, base, base.Add(time.Hour))
	if got["app-1|10:00|error"] != 2 {
		t.Fatalf("counts = %v, want 2 (one retry ignored)", got)
	}
	if lines, _, _ := store.CountForApp("app-1"); lines != 2 {
		t.Fatalf("stored lines = %d", lines)
	}
}

func TestMinuteCountsArePrunedWithRetentionAndDeletedWithTheApp(t *testing.T) {
	store := newTestStore(t, 0)
	now := time.Now().UTC().Truncate(time.Minute)
	store.Enqueue([]Entry{
		sevEntry("app-1", 0, now.AddDate(0, 0, -10), "error"),
		sevEntry("app-1", 1, now.Add(-time.Minute), "error"),
		sevEntry("app-2", 2, now.Add(-time.Minute), "error"),
	})
	store.Flush(5 * time.Second)

	if _, err := store.Prune([]AppRetention{{AppID: "app-1", RetentionDays: 7}}, now); err != nil {
		t.Fatal(err)
	}
	got := countsByKey(t, store, []string{"app-1", "app-2"}, now.AddDate(0, 0, -30), now)
	if len(got) != 2 {
		t.Fatalf("after prune = %v, want only the recent rows", got)
	}
	if _, err := store.DeleteApp("app-1"); err != nil {
		t.Fatal(err)
	}
	if got := countsByKey(t, store, []string{"app-1", "app-2"}, now.AddDate(0, 0, -30), now); len(got) != 1 {
		t.Fatalf("after delete = %v, want only app-2", got)
	}
}
