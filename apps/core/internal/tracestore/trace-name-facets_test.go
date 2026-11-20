package tracestore

import (
	"testing"
	"time"
)

// seedNameTraces writes three traces of app "web":
//
//	rooted:   root "GET /checkout" with a child "db.query"
//	rootless: orphans "queue.process" (earliest) and "cache.get"; the root never arrived
//	failing:  rootless with one failed orphan "payment.charge"
func seedNameTraces(t *testing.T, s *Store) {
	t.Helper()
	s.mustWrite(t,
		mkSpan("web", traceID(1), spanID(1), "", "GET /checkout", 0, 100*time.Millisecond, StatusUnset),
		mkSpan("web", traceID(1), spanID(2), spanID(1), "db.query", time.Millisecond, time.Millisecond, StatusUnset),
		mkSpan("web", traceID(2), spanID(21), spanID(20), "cache.get", 2*time.Minute+time.Second, time.Millisecond, StatusUnset),
		mkSpan("web", traceID(2), spanID(22), spanID(20), "queue.process", 2*time.Minute, 50*time.Millisecond, StatusUnset),
		mkSpan("web", traceID(3), spanID(31), spanID(30), "payment.charge", 4*time.Minute, 20*time.Millisecond, StatusError),
	)
}

func TestNameFilterMatchesWhatTheListShowsAsTheTraceName(t *testing.T) {
	s := openTestStore(t, 0)
	seedNameTraces(t, s)

	// The list names a rootless trace after its earliest span, and the filter follows the same name.
	all := listAll(t, s, Query{})
	shown := map[string]string{}
	for _, tr := range all {
		shown[tr.TraceID] = tr.RootName
	}
	if shown[traceID(1)] != "GET /checkout" || shown[traceID(2)] != "queue.process" || shown[traceID(3)] != "payment.charge" {
		t.Fatalf("names shown = %v", shown)
	}

	for _, tc := range []struct {
		name  string
		query Query
		want  []string
	}{
		{"root span name", Query{Name: "checkout"}, []string{traceID(1)}},
		{"earliest span of a rootless trace", Query{Name: "queue.process"}, []string{traceID(2)}},
		{"substring of it, any case", Query{Name: "QUEUE"}, []string{traceID(2)}},
		{"rootless trace of one span", Query{Name: "payment"}, []string{traceID(3)}},
		{"a later span of a rootless trace is not its name", Query{Name: "cache.get"}, nil},
		{"a child of a rooted trace is not its name", Query{Name: "db.query"}, nil},
		{"name with the other filters", Query{Name: "payment", Status: "error", AppIDs: []string{"web"}}, []string{traceID(3)}},
		{"name that excludes by status", Query{Name: "queue", Status: "error"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := ids(listAll(t, s, tc.query))
			if len(got) != len(tc.want) || (len(got) == 1 && got[0] != tc.want[0]) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			// The facets count the same traces as the search.
			facets, err := s.Facets(tc.query)
			if err != nil || facets.Total != int64(len(tc.want)) {
				t.Fatalf("facets total = %d (%v), want %d", facets.Total, err, len(tc.want))
			}
		})
	}
}

func TestNameFilterFollowsTheRootWhenItArrivesLate(t *testing.T) {
	s := openTestStore(t, 0)
	s.mustWrite(t, mkSpan("web", traceID(5), spanID(2), spanID(1), "db.query", time.Millisecond, time.Millisecond, StatusUnset))
	if got := ids(listAll(t, s, Query{Name: "db.query"})); len(got) != 1 {
		t.Fatalf("rootless trace not found by its earliest span: %v", got)
	}
	s.mustWrite(t, mkSpan("web", traceID(5), spanID(1), "", "GET /late", 0, 50*time.Millisecond, StatusUnset))
	if got := ids(listAll(t, s, Query{Name: "db.query"})); len(got) != 0 {
		t.Fatalf("once the root arrived the trace is named after it, got %v", got)
	}
	if got := ids(listAll(t, s, Query{Name: "late"})); len(got) != 1 {
		t.Fatalf("trace not found by its root: %v", got)
	}
}

func TestFacetTotalFollowsTheStatusFilterAndStatusCountsDoNot(t *testing.T) {
	s := openTestStore(t, 0)
	seedQueryTraces(t, s) // 10 traces: 4 with a failed span, 6 without; 5 for app web, of which 2 failed

	cases := []struct {
		name            string
		query           Query
		total, ok, errs int64
		services        int
	}{
		{"no status", Query{}, 10, 6, 4, 2},
		{"error", Query{Status: "error"}, 4, 6, 4, 2},
		{"ok", Query{Status: "ok"}, 6, 6, 4, 2},
		{"error in one app", Query{Status: "error", AppIDs: []string{"web"}}, 2, 3, 2, 1},
		{"ok in one app", Query{Status: "ok", AppIDs: []string{"web"}}, 3, 3, 2, 1},
		{"error with a duration", Query{Status: "error", MinDuration: 800 * time.Millisecond}, 1, 2, 1, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f, err := s.Facets(tc.query)
			if err != nil {
				t.Fatal(err)
			}
			if f.Total != tc.total || f.Status[0] != (Count{"ok", tc.ok}) || f.Status[1] != (Count{"error", tc.errs}) || len(f.Services) != tc.services {
				t.Fatalf("facets = %+v", f)
			}
			// Total is what the search returns, and the service counts add up to it.
			listed := int64(len(listAll(t, s, tc.query)))
			var byService int64
			for _, c := range f.Services {
				byService += c.Count
			}
			if f.Total != listed || byService != listed {
				t.Fatalf("total = %d, listed = %d, services add up to %d", f.Total, listed, byService)
			}
		})
	}
}
