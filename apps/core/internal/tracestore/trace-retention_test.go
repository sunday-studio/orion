package tracestore

import (
	"strings"
	"testing"
	"time"
)

func countRows(t *testing.T, s *Store, table string) int {
	t.Helper()
	var n int
	if err := s.db.Raw("SELECT COUNT(*) FROM " + table).Scan(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

func TestEffectiveDaysNeverExceedsTheInstanceLimit(t *testing.T) {
	for _, tc := range []struct{ app, limit, want int }{
		{14, 7, 7}, {3, 7, 3}, {0, 7, 7}, {14, 0, 14}, {0, 0, 0},
	} {
		if got := EffectiveDays(tc.app, tc.limit); got != tc.want {
			t.Fatalf("EffectiveDays(%d, %d) = %d, want %d", tc.app, tc.limit, got, tc.want)
		}
	}
}

func TestPruneRemovesExpiredSpansAndFixesTheirSummaries(t *testing.T) {
	s := openTestStore(t, 0)
	now := epoch.Add(10 * 24 * time.Hour)
	day := 24 * time.Hour
	// "old": entirely older than 7 days. "straddle": root is old but a late child is inside the window.
	// "fresh": inside the window. "other" belongs to an app with no policy.
	s.mustWrite(t,
		mkSpan("app", traceID(1), spanID(1), "", "old", 0, time.Second, StatusError),
		mkSpan("app", traceID(1), spanID(2), spanID(1), "old-child", time.Second, time.Second, StatusUnset),
		mkSpan("app", traceID(2), spanID(3), "", "straddle", 2*day, 8*day+time.Hour, StatusUnset),
		mkSpan("app", traceID(2), spanID(4), spanID(3), "straddle-late-child", 9*day, time.Second, StatusError),
		mkSpan("app", traceID(3), spanID(5), "", "fresh", 9*day+time.Hour, time.Second, StatusUnset),
		mkSpan("other", traceID(4), spanID(6), "", "other-old", 0, time.Second, StatusUnset),
	)
	result, err := s.Prune([]AppRetention{{AppID: "app", RetentionDays: 7}}, now)
	if err != nil {
		t.Fatal(err)
	}
	if result.Expired != 3 || result.OverLimit != 0 {
		t.Fatalf("prune result = %+v, want 3 expired (two of 'old', the straddling root)", result)
	}
	traces := listAll(t, s, Query{AppIDs: []string{"app"}})
	if len(traces) != 2 {
		t.Fatalf("traces after prune = %+v, want straddle and fresh", traces)
	}
	var straddle Trace
	for _, tr := range traces {
		if tr.TraceID == traceID(2) {
			straddle = tr
		}
	}
	// Only the late child is left: the trace is an orphan again, and its counts follow the spans that remain.
	if straddle.SpanCount != 1 || straddle.ErrorCount != 1 || straddle.HasRoot || straddle.RootName != "straddle-late-child" {
		t.Fatalf("straddling trace summary = %+v", straddle)
	}
	if n := countRows(t, s, "traces"); n != 3 {
		t.Fatalf("summary rows = %d, want 3 (straddle, fresh, the other app's)", n)
	}
	if got := listAll(t, s, Query{AppIDs: []string{"other"}}); len(got) != 1 {
		t.Fatalf("an app without a policy keeps its traces: %+v", got)
	}
}

func TestPruneTrimsTheOldestWholeTracesOverTheStorageCap(t *testing.T) {
	s := openTestStore(t, 0)
	const traceBytes = 100 << 10 // two spans of 50 KiB
	var spans []Span
	for n := 0; n < 30; n++ {
		root := mkSpan("app", traceID(500+n), spanID(n*2+1), "", "t", time.Duration(n)*time.Minute, time.Second, StatusUnset)
		child := mkSpan("app", traceID(500+n), spanID(n*2+2), spanID(n*2+1), "c", time.Duration(n)*time.Minute, time.Millisecond, StatusUnset)
		// Size is measured from the stored columns, so the weight has to be real content.
		root.StatusMessage, child.StatusMessage = strings.Repeat("x", traceBytes/2), strings.Repeat("x", traceBytes/2)
		spans = append(spans, root, child)
	}
	s.mustWrite(t, spans...) // 3000 KiB, cap is 1 MiB: at most 10 whole traces survive
	result, err := s.Prune([]AppRetention{{AppID: "app", MaxStorageMB: 1}}, epoch.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	count, bytes, _ := s.CountForApp("app")
	if bytes > 1<<20 || result.OverLimit != 60-count || count%2 != 0 {
		t.Fatalf("after trim: %d spans, %d bytes, result %+v: want under 1 MiB, whole traces only", count, bytes, result)
	}
	remaining := listAll(t, s, Query{})
	if int64(len(remaining))*2 != count {
		t.Fatalf("%d summaries for %d spans: summaries and spans must go together", len(remaining), count)
	}
	for _, tr := range remaining {
		if tr.SpanCount != 2 || !tr.Start.After(epoch.Add(19*time.Minute)) {
			t.Fatalf("kept %+v: the newest traces must survive, each complete", tr)
		}
	}
}

func TestDeleteAppRemovesSpansAndSummaries(t *testing.T) {
	s := openTestStore(t, 0)
	s.mustWrite(t,
		mkSpan("gone", traceID(1), spanID(1), "", "a", 0, time.Second, StatusUnset),
		mkSpan("gone", traceID(1), spanID(2), spanID(1), "b", 0, time.Second, StatusUnset),
		mkSpan("kept", traceID(1), spanID(3), spanID(1), "c", 0, time.Second, StatusUnset),
	)
	n, err := s.DeleteApp("gone")
	if err != nil || n != 2 {
		t.Fatalf("DeleteApp = %d, %v, want 2 spans", n, err)
	}
	if got := listAll(t, s, Query{}); len(got) != 1 || got[0].AppID != "kept" || got[0].SpanCount != 1 {
		t.Fatalf("after DeleteApp: %+v", got)
	}
	if countRows(t, s, "spans") != 1 {
		t.Fatal("the other app's span must stay")
	}
}

func TestPruneWorksInBatchesLargerThanOne(t *testing.T) {
	s := openTestStore(t, 0)
	var spans []Span
	for n := 0; n < pruneBatch*2+300; n++ {
		spans = append(spans, mkSpan("app", traceID(1000+n), spanID(n+1), "", "old", time.Duration(n)*time.Millisecond, time.Millisecond, StatusUnset))
	}
	// Written in pieces: one Enqueue is capped by the queue, and the writer batches anyway.
	for start := 0; start < len(spans); start += 1000 {
		s.mustWrite(t, spans[start:min(start+1000, len(spans))]...)
	}
	result, err := s.Prune([]AppRetention{{AppID: "app", RetentionDays: 1}}, epoch.Add(48*time.Hour))
	if err != nil || result.Expired != int64(len(spans)) {
		t.Fatalf("prune = %+v, %v, want all %d expired", result, err, len(spans))
	}
	if countRows(t, s, "spans") != 0 || countRows(t, s, "traces") != 0 {
		t.Fatal("spans and summaries must both be empty")
	}
}
