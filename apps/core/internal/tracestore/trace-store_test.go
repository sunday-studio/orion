package tracestore

import (
	"context"
	"fmt"
	"orion/core/internal/testkit"
	"orion/core/internal/utils"
	"strings"
	"testing"
	"time"
)

// epoch is a fixed instant, so no test depends on the wall clock or a minute boundary.
var epoch = time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)

func openTestStore(t *testing.T, queueMax int) *Store {
	t.Helper()
	store, err := OpenWith(testkit.NewDB(t), utils.NewLogger(), queueMax)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	return store
}

// mkSpan builds a span that started offset after epoch and lasted dur.
func mkSpan(app, trace, id, parent, name string, offset, dur time.Duration, status int) Span {
	return Span{
		AppID: app, TraceID: trace, SpanID: id, ParentSpanID: parent, Name: name, Kind: 2,
		StartNano: epoch.Add(offset).UnixNano(), DurationNano: dur.Nanoseconds(), StatusCode: status,
		ServiceName: "checkout", IngestedAt: epoch, Size: 200,
	}
}

func (s *Store) mustWrite(t *testing.T, spans ...Span) {
	t.Helper()
	if err := s.Enqueue(spans); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	s.Flush(10 * time.Second)
}

func traceID(n int) string { return fmt.Sprintf("%032x", n) }

func spanID(n int) string { return fmt.Sprintf("%016x", n) }

func listAll(t *testing.T, s *Store, q Query) []Trace {
	t.Helper()
	q.Limit = MaxPageSize
	out, _, err := s.ListTraces(q)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	return out
}

func TestWriterBuildsTheTraceSummary(t *testing.T) {
	s := openTestStore(t, 0)
	tid := traceID(1)
	s.mustWrite(t,
		mkSpan("app", tid, spanID(1), "", "GET /checkout", 0, 300*time.Millisecond, StatusUnset),
		mkSpan("app", tid, spanID(2), spanID(1), "db.query", 10*time.Millisecond, 50*time.Millisecond, StatusError),
		mkSpan("app", tid, spanID(3), spanID(1), "render", 100*time.Millisecond, 20*time.Millisecond, StatusOK),
	)
	traces := listAll(t, s, Query{})
	if len(traces) != 1 {
		t.Fatalf("traces = %+v, want one", traces)
	}
	got := traces[0]
	if got.TraceID != tid || got.RootName != "GET /checkout" || got.ServiceName != "checkout" || !got.HasRoot ||
		got.SpanCount != 3 || got.ErrorCount != 1 || got.DurationNs != (300*time.Millisecond).Nanoseconds() || !got.Start.Equal(epoch) {
		t.Fatalf("summary = %+v", got)
	}
}

func TestOutOfOrderArrivalProducesTheSameSummary(t *testing.T) {
	s := openTestStore(t, 0)
	tid := traceID(2)
	// Children first, in separate writes: an orphan trace is stored as it arrived.
	s.mustWrite(t, mkSpan("app", tid, spanID(2), spanID(1), "db.query", 20*time.Millisecond, 40*time.Millisecond, StatusError))
	orphan := listAll(t, s, Query{})[0]
	if orphan.HasRoot || orphan.RootName != "db.query" || orphan.SpanCount != 1 || orphan.ErrorCount != 1 {
		t.Fatalf("orphan summary = %+v", orphan)
	}
	s.mustWrite(t, mkSpan("app", tid, spanID(3), spanID(1), "render", 70*time.Millisecond, 10*time.Millisecond, StatusOK))
	if got := listAll(t, s, Query{})[0]; got.HasRoot || got.SpanCount != 2 || got.DurationNs != (60*time.Millisecond).Nanoseconds() {
		t.Fatalf("two orphans: %+v (extent from first start to last end is 60ms)", got)
	}
	// The root arrives last and takes over the summary.
	s.mustWrite(t, mkSpan("app", tid, spanID(1), "", "GET /pay", 0, 100*time.Millisecond, StatusUnset))
	traces := listAll(t, s, Query{})
	if len(traces) != 1 {
		t.Fatalf("traces = %+v, want one", traces)
	}
	got := traces[0]
	if !got.HasRoot || got.RootName != "GET /pay" || got.SpanCount != 3 || got.ErrorCount != 1 || got.DurationNs != (100*time.Millisecond).Nanoseconds() || !got.Start.Equal(epoch) {
		t.Fatalf("summary after the root = %+v", got)
	}
}

func TestSummaryIsTheSameForEveryArrivalOrder(t *testing.T) {
	spans := []Span{
		mkSpan("app", traceID(3), spanID(1), "", "root", 0, 90*time.Millisecond, StatusUnset),
		mkSpan("app", traceID(3), spanID(2), spanID(1), "a", 5*time.Millisecond, 10*time.Millisecond, StatusError),
		mkSpan("app", traceID(3), spanID(3), spanID(2), "b", 6*time.Millisecond, 5*time.Millisecond, StatusError),
	}
	orders := [][]int{{0, 1, 2}, {2, 1, 0}, {1, 0, 2}, {2, 0, 1}}
	var want *Trace
	for _, order := range orders {
		s := openTestStore(t, 0)
		for _, i := range order {
			s.mustWrite(t, spans[i])
		}
		got := listAll(t, s, Query{})[0]
		if want == nil {
			want = &got
		} else if got != *want {
			t.Fatalf("order %v: %+v, want %+v", order, got, *want)
		}
	}
}

func TestRepeatedSpansAreNotCountedTwice(t *testing.T) {
	s := openTestStore(t, 0)
	tid := traceID(4)
	batch := []Span{
		mkSpan("app", tid, spanID(1), "", "root", 0, time.Second, StatusUnset),
		mkSpan("app", tid, spanID(2), spanID(1), "child", 0, time.Millisecond, StatusError),
	}
	s.mustWrite(t, batch...)
	s.mustWrite(t, batch...) // an exporter retry resends the same spans
	got := listAll(t, s, Query{})[0]
	if got.SpanCount != 2 || got.ErrorCount != 1 {
		t.Fatalf("summary after a resend = %+v, want 2 spans and 1 error", got)
	}
	if st := s.Stats(); st.Written != 2 || st.Duplicates != 2 {
		t.Fatalf("stats = %+v, want written 2 and duplicates 2", st)
	}
}

func TestTheSameTraceIdInTwoAppsKeepsOneSummaryPerApp(t *testing.T) {
	s := openTestStore(t, 0)
	tid := traceID(5)
	s.mustWrite(t,
		mkSpan("web", tid, spanID(1), "", "GET /", 0, 100*time.Millisecond, StatusUnset),
		mkSpan("api", tid, spanID(2), spanID(1), "handle", 10*time.Millisecond, 50*time.Millisecond, StatusUnset),
	)
	if got := listAll(t, s, Query{}); len(got) != 2 {
		t.Fatalf("traces = %+v, want one per app", got)
	}
	api := listAll(t, s, Query{AppIDs: []string{"api"}})
	if len(api) != 1 || api[0].HasRoot || api[0].SpanCount != 1 {
		t.Fatalf("api summary = %+v, want its own orphan span", api)
	}
	detail, err := s.GetTrace(context.Background(), tid)
	if err != nil || detail.Total != 2 || len(detail.Spans) != 2 || detail.Spans[0].SpanID != spanID(1) {
		t.Fatalf("detail = %+v, %v: want both apps' spans in start order", detail, err)
	}
}

func TestEnqueueRefusesWhenTheQueueIsFull(t *testing.T) {
	s := openTestStore(t, 5)
	var spans []Span
	for i := 0; i < 6; i++ {
		spans = append(spans, mkSpan("app", traceID(6), spanID(i+1), "", "s", 0, time.Millisecond, StatusUnset))
	}
	if err := s.Enqueue(spans); err != ErrQueueFull {
		t.Fatalf("Enqueue over the budget = %v, want ErrQueueFull", err)
	}
	if q, max := s.QueueDepth(); q != 0 || max != 5 {
		t.Fatalf("queue depth = %d/%d after a refused batch", q, max)
	}
	if err := s.Enqueue(spans[:5]); err != nil {
		t.Fatalf("Enqueue within the budget = %v", err)
	}
	s.Flush(5 * time.Second)
	s.Close()
	if err := s.Enqueue(spans[:1]); err != ErrQueueFull {
		t.Fatalf("Enqueue after Close = %v, want ErrQueueFull", err)
	}
}

// PRAGMA max_page_count makes SQLite fail writes with the same SQLITE_FULL a full volume produces.
func TestStorageFullRefusesKeepsDataAndRecovers(t *testing.T) {
	old := storageFullBackoff
	storageFullBackoff = 200 * time.Millisecond
	t.Cleanup(func() { storageFullBackoff = old })

	database := testkit.NewDB(t)
	sqlDB, _ := database.DB()
	sqlDB.SetMaxOpenConns(1) // max_page_count is per connection; one connection makes it deterministic
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(0)
	s, err := OpenWith(database, utils.NewLogger(), 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)

	big := strings.Repeat("x", 1500)
	mk := func(from, n int) []Span {
		var out []Span
		for i := from; i < from+n; i++ {
			sp := mkSpan("app", traceID(i), spanID(i), "", "span", time.Duration(i)*time.Second, time.Millisecond, StatusUnset)
			sp.StatusMessage = big
			out = append(out, sp)
		}
		return out
	}
	s.mustWrite(t, mk(1, 500)...)
	before, _, _ := s.CountForApp("app")
	if before != 500 {
		t.Fatalf("seeded %d spans, want 500", before)
	}
	var pages int
	database.Raw("PRAGMA page_count").Scan(&pages)
	if err := database.Exec(fmt.Sprintf("PRAGMA max_page_count = %d", pages)).Error; err != nil {
		t.Fatal(err)
	}
	if err := s.Enqueue(mk(1000, 2000)); err != nil {
		t.Fatalf("the first oversized batch is accepted into the queue: %v", err)
	}
	s.Flush(10 * time.Second)
	if st := s.Stats(); st.Dropped == 0 || !st.StorageFull {
		t.Fatalf("stats after a full disk = %+v, want dropped and storage_full", st)
	}
	if err := s.Enqueue(mk(5000, 1)); err != ErrStorageFull {
		t.Fatalf("Enqueue while full = %v, want ErrStorageFull", err)
	}
	if after, _, _ := s.CountForApp("app"); after != before {
		t.Fatalf("spans %d -> %d: a failed batch must not partially commit", before, after)
	}
	var traceRows int
	database.Raw("SELECT COUNT(*) FROM traces").Scan(&traceRows)
	if traceRows != 500 {
		t.Fatalf("trace summaries = %d, want 500 (the summary rolls back with the spans)", traceRows)
	}

	database.Exec("PRAGMA max_page_count = 1073741823")
	time.Sleep(storageFullBackoff + 50*time.Millisecond)
	s.mustWrite(t, mk(6000, 10)...)
	if final, _, _ := s.CountForApp("app"); final != before+10 || s.Stats().StorageFull {
		t.Fatalf("spans=%d (want %d) stats=%+v: should have recovered", final, before+10, s.Stats())
	}
}
