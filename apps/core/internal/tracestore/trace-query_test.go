package tracestore

import (
	"context"
	"errors"
	"testing"
	"time"
)

// seedQueryTraces writes ten traces, one a minute apart starting at epoch:
//
//	n=0..9  root "GET /orders/n", duration 100ms*(n+1); n%3==0 has a failed child; app "web" for even n, "api" for odd n;
//	service "shop" for even n and "billing" for odd n; n==7 carries http.status_code 503 on its child.
func seedQueryTraces(t *testing.T, s *Store) {
	t.Helper()
	var spans []Span
	for n := 0; n < 10; n++ {
		app, service := "web", "shop"
		if n%2 == 1 {
			app, service = "api", "billing"
		}
		root := mkSpan(app, traceID(100+n), spanID(n*10+1), "", "GET /orders/"+string(rune('a'+n)), time.Duration(n)*time.Minute, time.Duration(n+1)*100*time.Millisecond, StatusUnset)
		root.ServiceName = service
		root.Resource = map[string]any{"deployment.environment": "prod"}
		child := mkSpan(app, traceID(100+n), spanID(n*10+2), spanID(n*10+1), "db.query", time.Duration(n)*time.Minute+time.Millisecond, time.Millisecond, StatusOK)
		child.ServiceName = service
		if n%3 == 0 {
			child.StatusCode = StatusError
		}
		child.Attributes = map[string]any{"db.system": "postgres"}
		if n == 7 {
			child.Attributes["http.status_code"] = int64(503)
		}
		spans = append(spans, root, child)
	}
	s.mustWrite(t, spans...)
}

func ids(traces []Trace) []string {
	out := make([]string, len(traces))
	for i, tr := range traces {
		out[i] = tr.TraceID
	}
	return out
}

func TestListFiltersTraces(t *testing.T) {
	s := openTestStore(t, 0)
	seedQueryTraces(t, s)
	at := func(n int) time.Time { return epoch.Add(time.Duration(n) * time.Minute) }

	cases := []struct {
		name  string
		query Query
		want  []int // trace numbers, newest first
	}{
		{"all, newest first", Query{}, []int{9, 8, 7, 6, 5, 4, 3, 2, 1, 0}},
		{"by app", Query{AppIDs: []string{"api"}}, []int{9, 7, 5, 3, 1}},
		{"by several apps", Query{AppIDs: []string{"api", "web"}}, []int{9, 8, 7, 6, 5, 4, 3, 2, 1, 0}},
		{"by service", Query{Service: "billing"}, []int{9, 7, 5, 3, 1}},
		{"errors only", Query{Status: "error"}, []int{9, 6, 3, 0}},
		{"ok only", Query{Status: "ok"}, []int{8, 7, 5, 4, 2, 1}},
		{"min duration", Query{MinDuration: 800 * time.Millisecond}, []int{9, 8, 7}},
		{"name substring", Query{Name: "orders/c"}, []int{2}},
		{"name is not a pattern", Query{Name: "orders/%"}, nil},
		{"from", Query{From: at(7)}, []int{9, 8, 7}},
		{"to", Query{To: at(2)}, []int{2, 1, 0}},
		{"from and to", Query{From: at(3), To: at(5)}, []int{5, 4, 3}},
		{"span attribute of any span", Query{Attrs: []string{"http.status_code:503"}}, []int{7}},
		{"resource attribute", Query{Attrs: []string{"deployment.environment:prod"}, Status: "error", AppIDs: []string{"web"}}, []int{6, 0}},
		{"two attributes both must match", Query{Attrs: []string{"db.system:postgres", "http.status_code:503"}}, []int{7}},
		{"attribute that nothing carries", Query{Attrs: []string{"db.system:mysql"}}, nil},
		{"filters combine", Query{AppIDs: []string{"web"}, Status: "error", MinDuration: 500 * time.Millisecond}, []int{6}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ids(listAll(t, s, tc.query))
			var want []string
			for _, n := range tc.want {
				want = append(want, traceID(100+n))
			}
			if len(got) != len(want) {
				t.Fatalf("got %v, want %v", got, want)
			}
			for i := range want {
				if got[i] != want[i] {
					t.Fatalf("got %v, want %v", got, want)
				}
			}
		})
	}
}

func TestListRejectsBadFilters(t *testing.T) {
	s := openTestStore(t, 0)
	for _, q := range []Query{
		{Status: "fine"},
		{Attrs: []string{"nocolon"}},
		{Attrs: []string{"a..b:1"}},
		{Attrs: []string{`a'); DROP TABLE spans; --:1`}},
		{Cursor: "***"},
		{Cursor: "bm90LWEtY3Vyc29y"},
	} {
		if _, _, err := s.ListTraces(q); !errors.Is(err, ErrBadFilter) {
			t.Fatalf("query %+v: err = %v, want ErrBadFilter", q, err)
		}
	}
}

func TestCursorPaginationVisitsEveryTraceOnce(t *testing.T) {
	s := openTestStore(t, 0)
	// Several traces share one start time (and one trace id lives in two apps), so the cursor
	// must break ties without skipping or repeating a row.
	var spans []Span
	for n := 0; n < 12; n++ {
		spans = append(spans, mkSpan("web", traceID(200+n), spanID(n+1), "", "same-instant", time.Duration(n/4)*time.Minute, time.Millisecond, StatusUnset))
	}
	spans = append(spans, mkSpan("api", traceID(200), spanID(99), "", "same-instant", 0, time.Millisecond, StatusUnset))
	s.mustWrite(t, spans...)

	seen := map[string]bool{}
	cursor, pages := "", 0
	for {
		page, next, err := s.ListTraces(Query{Limit: 5, Cursor: cursor})
		if err != nil {
			t.Fatal(err)
		}
		pages++
		for _, tr := range page {
			key := tr.AppID + "/" + tr.TraceID
			if seen[key] {
				t.Fatalf("%s listed twice", key)
			}
			seen[key] = true
		}
		if next == "" {
			break
		}
		cursor = next
	}
	if len(seen) != 13 || pages != 3 {
		t.Fatalf("saw %d traces over %d pages, want 13 over 3", len(seen), pages)
	}
}

func TestFacetsListServicesNamesAndStatus(t *testing.T) {
	s := openTestStore(t, 0)
	seedQueryTraces(t, s)
	f, err := s.Facets(Query{})
	if err != nil {
		t.Fatal(err)
	}
	if f.Total != 10 || len(f.Services) != 2 || len(f.Names) != 10 {
		t.Fatalf("facets = %+v", f)
	}
	if f.Services[0].Count != 5 || f.Status[0] != (Count{"ok", 6}) || f.Status[1] != (Count{"error", 4}) {
		t.Fatalf("services/status = %+v / %+v", f.Services, f.Status)
	}
	// Picking a status narrows the lists but the status counts keep showing both choices.
	f, err = s.Facets(Query{Status: "error", AppIDs: []string{"web"}})
	if err != nil || f.Total != 5 || f.Status[1].Count != 2 || len(f.Services) != 1 || f.Services[0].Value != "shop" || f.Services[0].Count != 2 {
		t.Fatalf("narrowed facets = %+v, %v", f, err)
	}
}

func TestFacetListsAreBounded(t *testing.T) {
	s := openTestStore(t, 0)
	var spans []Span
	for n := 0; n < FacetLimit+25; n++ {
		sp := mkSpan("app", traceID(300+n), spanID(n+1), "", "op-"+traceID(n), 0, time.Millisecond, StatusUnset)
		sp.ServiceName = "svc-" + traceID(n)
		spans = append(spans, sp)
	}
	s.mustWrite(t, spans...)
	f, err := s.Facets(Query{})
	if err != nil || len(f.Services) != FacetLimit || len(f.Names) != FacetLimit {
		t.Fatalf("services=%d names=%d err=%v, want both capped at %d", len(f.Services), len(f.Names), err, FacetLimit)
	}
}

func TestSummarizeCountsAndPercentiles(t *testing.T) {
	s := openTestStore(t, 0)
	var spans []Span
	// 100 rooted traces of 1..100 ms, every tenth with a failed child, plus one orphan that has no latency.
	for n := 1; n <= 100; n++ {
		spans = append(spans, mkSpan("app", traceID(400+n), spanID(n), "", "root", time.Duration(n)*time.Second, time.Duration(n)*time.Millisecond, StatusUnset))
		if n%10 == 0 {
			spans = append(spans, mkSpan("app", traceID(400+n), spanID(1000+n), spanID(n), "child", time.Duration(n)*time.Second, time.Millisecond, StatusError))
		}
	}
	spans = append(spans, mkSpan("app", traceID(999), spanID(5000), spanID(4999), "orphan", 30*time.Second, 5*time.Second, StatusUnset))
	spans = append(spans, mkSpan("other", traceID(998), spanID(6000), "", "elsewhere", 30*time.Second, time.Second, StatusError))
	s.mustWrite(t, spans...)

	got, err := s.Summarize(context.Background(), []string{"app"}, epoch, epoch.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	ms := func(n int64) int64 { return (time.Duration(n) * time.Millisecond).Nanoseconds() }
	if got.TraceCount != 101 || got.ErrorCount != 10 || got.RootedCount != 100 || got.P50 != ms(50) || got.P95 != ms(95) || got.P99 != ms(99) || got.Sampled {
		t.Fatalf("summary = %+v", got)
	}
	// The window is honoured, and an app with no traces answers with zeros.
	narrow, _ := s.Summarize(context.Background(), []string{"app"}, epoch.Add(10*time.Second), epoch.Add(20*time.Second))
	if narrow.TraceCount != 11 || narrow.P50 != ms(15) {
		t.Fatalf("narrow summary = %+v", narrow)
	}
	if none, err := s.Summarize(context.Background(), []string{"nobody"}, epoch, epoch.Add(time.Hour)); err != nil || none != (ServiceSummary{}) {
		t.Fatalf("empty summary = %+v, %v", none, err)
	}
	if none, err := s.Summarize(context.Background(), nil, epoch, epoch.Add(time.Hour)); err != nil || none != (ServiceSummary{}) {
		t.Fatalf("no apps = %+v, %v", none, err)
	}
}

func TestPercentileUsesNearestRank(t *testing.T) {
	values := []int64{10, 20, 30, 40}
	for pct, want := range map[int]int64{1: 10, 25: 10, 26: 20, 50: 20, 75: 30, 99: 40, 100: 40} {
		if got := percentile(values, pct); got != want {
			t.Fatalf("p%d = %d, want %d", pct, got, want)
		}
	}
	if percentile(nil, 50) != 0 {
		t.Fatal("percentile of nothing must be 0")
	}
}
