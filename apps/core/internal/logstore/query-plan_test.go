package logstore

import (
	"strings"
	"testing"
	"time"
)

// planOf returns SQLite's query plan for the entries query of q, as one string.
func planOf(t *testing.T, s *Store, q Query) string {
	t.Helper()
	where, args, err := q.where()
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct{ Detail string }
	if err := s.db.Raw("EXPLAIN QUERY PLAN SELECT * FROM log_entries"+where+" ORDER BY ts DESC, id DESC LIMIT 26", args...).Scan(&rows).Error; err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, r := range rows {
		out = append(out, r.Detail)
	}
	return strings.Join(out, "; ")
}

// A lookup by trace or request id must use its index. The indexes are partial (`<> ”`), and SQLite only
// uses one when the query says so too; without it the lookup scans every stored line.
func TestTraceAndRequestLookupsUseTheirIndexes(t *testing.T) {
	store := newTestStore(t, 0)
	var batch []Entry
	for i := 0; i < 20; i++ {
		e := mkEntry("a", i, epoch.Add(time.Duration(i)*time.Second))
		if i%2 == 0 {
			e.TraceID, e.RequestID = "trace-1", "req-1"
		}
		batch = append(batch, e)
	}
	store.Enqueue(batch)
	store.Flush(5 * time.Second)

	for _, tc := range []struct {
		name  string
		q     Query
		index string
	}{
		{"trace", Query{TraceID: "trace-1"}, "idx_log_entries_trace"},
		{"request", Query{RequestID: "req-1"}, "idx_log_entries_request"},
		{"trace with a range", Query{TraceID: "trace-1", From: epoch.Add(-time.Hour), To: epoch.Add(time.Hour)}, "idx_log_entries_trace"},
	} {
		if plan := planOf(t, store, tc.q); !strings.Contains(plan, tc.index) {
			t.Errorf("%s lookup plan = %q, want it to use %s", tc.name, plan, tc.index)
		}
	}

	got, _, err := store.List(Query{TraceID: "trace-1", Limit: 50})
	if err != nil || len(got) != 10 {
		t.Fatalf("List by trace id = %d, %v, want the 10 lines of the trace", len(got), err)
	}
	got, _, err = store.List(Query{RequestID: "nope", Limit: 50})
	if err != nil || len(got) != 0 {
		t.Fatalf("List by an unknown request id = %d, %v, want none", len(got), err)
	}
	first, _, err := store.List(Query{TraceID: "trace-1", Limit: 1})
	if err != nil || len(first) != 1 {
		t.Fatal(err)
	}
	by, total, related, err := store.Related(first[0], 5)
	if err != nil || by != "request_id" || total != 10 || len(related) != 5 {
		t.Fatalf("Related = %q total %d len %d err %v, want request_id, 10, 5", by, total, len(related), err)
	}
}

// The event facet of an app over a short range must not read all of the app's history.
func TestEventFacetUsesTheTimeRangeIndex(t *testing.T) {
	store := newTestStore(t, 0)
	store.Enqueue([]Entry{mkEntry("a", 0, epoch)})
	store.Flush(5 * time.Second)
	for _, tc := range []struct {
		name  string
		q     Query
		index string
	}{
		{"with an app", Query{AppIDs: []string{"a"}, From: epoch.Add(-time.Hour), To: epoch.Add(time.Hour)}, "idx_log_entries_app_ts"},
		{"without an app", Query{From: epoch.Add(-time.Hour), To: epoch.Add(time.Hour)}, "idx_log_entries_ts"},
	} {
		where, args, err := tc.q.where()
		if err != nil {
			t.Fatal(err)
		}
		sql, sqlArgs := eventCountsSQL(tc.q, where, args)
		var rows []struct{ Detail string }
		if err := store.db.Raw("EXPLAIN QUERY PLAN "+sql, sqlArgs...).Scan(&rows).Error; err != nil {
			t.Fatal(err)
		}
		var plan []string
		for _, r := range rows {
			plan = append(plan, r.Detail)
		}
		if joined := strings.Join(plan, "; "); !strings.Contains(joined, tc.index+" (") {
			t.Errorf("%s: plan = %q, want it to search %s", tc.name, joined, tc.index)
		}
	}
}
