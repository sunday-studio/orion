package metricstore

import (
	"fmt"
	"orion/core/internal/utils"
	"testing"
	"time"
)

// baseMinute is an hour boundary a few hours ago, so tests never depend on where in the minute or the hour
// they run, and a step of 10 minutes or an hour starts exactly there.
func baseMinute() time.Time { return time.Now().UTC().Truncate(time.Hour).Add(-3 * time.Hour) }

func otlpBatch(at time.Time, samples ...Sample) Batch {
	return Batch{Source: SourceOTLP, AppID: "app-1", At: at, Samples: samples}
}

func refOTLP(key string) SeriesRef { return SeriesRef{Source: SourceOTLP, AppID: "app-1", Key: key} }

// sumReading is a sum data point read at time at, counting since start.
func sumReading(key string, v float64, cumulative bool, start, at time.Time) Sample {
	return Sample{Key: key, Kind: KindSum, Name: key, Metric: key,
		Values: Values{Value: fp(v)}, Temporality: &Temporality{Cumulative: cumulative, Start: start, At: at}}
}

func histReading(key string, h Histogram, cumulative bool, start, at time.Time) Sample {
	kind := KindHistogram
	if len(h.Quantiles) > 0 {
		kind = KindSummary
	}
	return Sample{Key: key, Kind: kind, Name: key, Metric: key, Values: Values{Histogram: &h}, Temporality: &Temporality{Cumulative: cumulative, Start: start, At: at}}
}

// writeAt enqueues one sample in the minute of at and waits for the writer.
func writeAt(s *Store, at time.Time, smp Sample) {
	s.Enqueue(otlpBatch(at.Truncate(time.Minute), smp))
	s.Flush()
}

func queryAll(t *testing.T, s *Store, key string, from time.Time) SeriesResult {
	t.Helper()
	res, err := s.Query(Query{Series: []SeriesRef{refOTLP(key)}, From: from, To: from.Add(6 * time.Hour)})
	if err != nil || len(res.Series) != 1 {
		t.Fatalf("query %s: %+v %v", key, res, err)
	}
	return res.Series[0]
}

func valuesOf(points []Point) []float64 {
	var out []float64
	for _, p := range points {
		if p.Value == nil {
			out = append(out, -1)
			continue
		}
		out = append(out, *p.Value)
	}
	return out
}

func sameFloats(a, b []float64) bool { return equalFloats(a, b) }

func TestDeltaOfNumbers(t *testing.T) {
	start := time.Unix(1_700_000_000, 0)
	prev := cumState{StartAt: nanos(start), Value: 100}
	for name, tc := range map[string]struct {
		start     time.Time
		value     float64
		want      float64
		wantReset bool
	}{
		"grows":                         {start, 130, 30, false},
		"unchanged":                     {start, 100, 0, false},
		"decrease is a reset":           {start, 7, 7, true},
		"new start time is a reset":     {start.Add(time.Hour), 150, 150, true},
		"new start time and a decrease": {start.Add(time.Hour), 3, 3, true},
		"start time lost":               {time.Time{}, 140, 140, true},
	} {
		got, reset := deltaOf(prev, Temporality{Cumulative: true, Start: tc.start}, Values{Value: fp(tc.value)})
		if *got.Value != tc.want || reset != tc.wantReset {
			t.Errorf("%s: delta %v reset %v, want %v %v", name, *got.Value, reset, tc.want, tc.wantReset)
		}
	}
	// An exporter that never sets a start time still gets reset detection from the value.
	unset := cumState{Value: 50}
	if got, reset := deltaOf(unset, Temporality{Cumulative: true}, Values{Value: fp(60)}); *got.Value != 10 || reset {
		t.Errorf("no start time, growing: %v %v", *got.Value, reset)
	}
	if got, reset := deltaOf(unset, Temporality{Cumulative: true}, Values{Value: fp(5)}); *got.Value != 5 || !reset {
		t.Errorf("no start time, decrease: %v %v", *got.Value, reset)
	}
}

func TestDeltaOfHistograms(t *testing.T) {
	start := time.Unix(1_700_000_000, 0)
	prev := cumState{StartAt: nanos(start), Count: 10, Sum: fp(100), Bounds: []float64{1, 2}, Counts: []int64{4, 4, 2}}
	grown := Histogram{Count: 16, Sum: fp(190), Bounds: []float64{1, 2}, Counts: []int64{5, 8, 3}, Quantiles: []Quantile{{0.5, 1}}}
	got, reset := deltaOf(prev, Temporality{Cumulative: true, Start: start}, Values{Histogram: &grown})
	d := got.Histogram
	if reset || d.Count != 6 || *d.Sum != 90 || d.Counts[0] != 1 || d.Counts[1] != 4 || d.Counts[2] != 1 || len(d.Quantiles) != 1 {
		t.Fatalf("delta = %+v reset %v", d, reset)
	}
	if grown.Counts[0] != 5 {
		t.Fatal("the reading itself must not be changed")
	}

	for name, tc := range map[string]struct {
		cur  Histogram
		time time.Time
	}{
		"count went down":    {Histogram{Count: 3, Sum: fp(1), Bounds: []float64{1, 2}, Counts: []int64{1, 1, 1}}, start},
		"a bucket went down": {Histogram{Count: 12, Sum: fp(150), Bounds: []float64{1, 2}, Counts: []int64{3, 8, 1}}, start},
		"start time changed": {Histogram{Count: 20, Sum: fp(300), Bounds: []float64{1, 2}, Counts: []int64{6, 8, 6}}, start.Add(time.Minute)},
	} {
		got, reset := deltaOf(prev, Temporality{Cumulative: true, Start: tc.time}, Values{Histogram: &tc.cur})
		if !reset || got.Histogram.Count != tc.cur.Count || totalOf(got.Histogram.Counts) != tc.cur.Count {
			t.Errorf("%s: reset %v delta %+v, want the new reading as the change", name, reset, got.Histogram)
		}
	}

	// An exponential histogram that changed scale: the old counts are moved onto the new, coarser buckets.
	old := cumState{StartAt: nanos(start), Count: 4, Bounds: []float64{1, 2, 4, 8}, Counts: []int64{0, 1, 1, 2, 0}}
	coarser := Histogram{Count: 6, Bounds: []float64{1, 4, 16}, Counts: []int64{0, 3, 3, 0}}
	got, reset = deltaOf(old, Temporality{Cumulative: true, Start: start}, Values{Histogram: &coarser})
	// Old (1,2] and (2,4] fall into the new (1,4]; old (4,8] into (4,16]: the change is 1 and 1.
	if reset || got.Histogram.Count != 2 || got.Histogram.Counts[1] != 1 || got.Histogram.Counts[2] != 1 || got.Histogram.Counts[0] != 0 {
		t.Fatalf("scale change: %+v reset %v", got.Histogram, reset)
	}

	// A sum that was not sent in one of the readings cannot be differenced.
	noSum := Histogram{Count: 11, Bounds: []float64{1, 2}, Counts: []int64{4, 5, 2}}
	if got, _ := deltaOf(prev, Temporality{Cumulative: true, Start: start}, Values{Histogram: &noSum}); got.Histogram.Sum != nil || got.Histogram.Count != 1 {
		t.Fatalf("no sum = %+v", got.Histogram)
	}
}

func TestCumulativeSumsAreStoredAsIncreases(t *testing.T) {
	s := openTestStore(t)
	base := baseMinute()
	start := base.Add(-time.Hour)
	key := "http.requests|{}"
	for i, v := range []float64{100, 130, 150, 10, 25} { // the fourth reading is a restart: it counts from zero again
		st := start
		if i >= 3 {
			st = base.Add(2*time.Minute + 30*time.Second)
		}
		writeAt(s, base.Add(time.Duration(i)*time.Minute+5*time.Second), sumReading(key, v, true, st, base.Add(time.Duration(i)*time.Minute+5*time.Second)))
	}
	series := queryAll(t, s, key, base.Add(-time.Minute))
	// The first reading only sets the baseline, so there is no row for minute 0.
	if got := valuesOf(series.Points); !sameFloats(got, []float64{30, 20, 10, 15}) {
		t.Fatalf("increases = %v, want [30 20 10 15]", got)
	}
	if series.Kind != KindSum || !series.Points[0].T.Equal(base.Add(time.Minute)) {
		t.Fatalf("series = %+v", series)
	}
	if st := s.Stats(); st.Written != 4 || st.DroppedStale != 0 {
		t.Fatalf("stats = %+v", st)
	}
}

func TestDeltaSumsAreStoredAsSentAndAddUpWithinAMinute(t *testing.T) {
	s := openTestStore(t)
	base := baseMinute()
	key := "jobs.done|{}"
	for i, v := range []float64{3, 4, 5} { // an exporter that reports every 10 seconds
		at := base.Add(time.Duration(i) * 10 * time.Second)
		writeAt(s, at, sumReading(key, v, false, at.Add(-10*time.Second), at))
	}
	writeAt(s, base.Add(time.Minute+time.Second), sumReading(key, 7, false, base, base.Add(time.Minute+time.Second)))
	series := queryAll(t, s, key, base.Add(-time.Minute))
	if got := valuesOf(series.Points); !sameFloats(got, []float64{12, 7}) {
		t.Fatalf("values = %v, want [12 7]: the first reading of a delta series counts, and a minute holds the sum", got)
	}
	// An up-down counter reports a net change, which may be negative.
	writeAt(s, base.Add(10*time.Second), sumReading("queue|{}", -4, false, base, base.Add(10*time.Second)))
	if got := valuesOf(queryAll(t, s, "queue|{}", base.Add(-time.Minute)).Points); !sameFloats(got, []float64{-4}) {
		t.Fatalf("net change = %v", got)
	}
}

func TestReadingsThatAreNotNewerAreDroppedSoNothingCountsTwice(t *testing.T) {
	s := openTestStore(t)
	base := baseMinute()
	key := "requests|{}"
	at := func(m int) time.Time { return base.Add(time.Duration(m)*time.Minute + 2*time.Second) }
	writeAt(s, at(0), sumReading(key, 100, true, base, at(0)))
	writeAt(s, at(3), sumReading(key, 160, true, base, at(3)))
	writeAt(s, at(3), sumReading(key, 160, true, base, at(3))) // a retry of the request that was written
	writeAt(s, at(1), sumReading(key, 120, true, base, at(1))) // arrived late
	writeAt(s, at(4), sumReading(key, 170, true, base, at(4)))
	if got := valuesOf(queryAll(t, s, key, base.Add(-time.Minute)).Points); !sameFloats(got, []float64{60, 10}) {
		t.Fatalf("increases = %v, want [60 10]", got)
	}
	if st := s.Stats(); st.DroppedStale != 2 || st.Written != 2 {
		t.Fatalf("stats = %+v, want 2 stale and 2 written", st)
	}

	// The same holds for delta sums and histograms: a retried delta must not double.
	dkey := "deltas|{}"
	writeAt(s, at(0), sumReading(dkey, 5, false, base, at(0)))
	writeAt(s, at(0), sumReading(dkey, 5, false, base, at(0)))
	if got := valuesOf(queryAll(t, s, dkey, base.Add(-time.Minute)).Points); !sameFloats(got, []float64{5}) {
		t.Fatalf("a retried delta = %v", got)
	}
	hkey := "histogram:lat|{}"
	h := Histogram{Count: 2, Sum: fp(3), Bounds: []float64{1}, Counts: []int64{1, 1}}
	writeAt(s, at(0), histReading(hkey, h, false, base, at(0)))
	writeAt(s, at(0), histReading(hkey, h, false, base, at(0)))
	pts := queryAll(t, s, hkey, base.Add(-time.Minute)).Points
	if len(pts) != 1 || *pts[0].Count != 2 {
		t.Fatalf("a retried histogram = %+v", pts)
	}
}

func TestStateSurvivesARestartWithoutCountingTwice(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir, utils.NewLogger())
	if err != nil {
		t.Fatal(err)
	}
	base := baseMinute()
	key := "requests|{}"
	at := func(m int) time.Time { return base.Add(time.Duration(m)*time.Minute + time.Second) }
	writeAt(s, at(0), sumReading(key, 100, true, base, at(0)))
	writeAt(s, at(1), sumReading(key, 110, true, base, at(1)))
	s.Close()

	s, err = Open(dir, utils.NewLogger())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	writeAt(s, at(1), sumReading(key, 110, true, base, at(1))) // the exporter retries across the restart
	writeAt(s, at(2), sumReading(key, 125, true, base, at(2)))
	if got := valuesOf(queryAll(t, s, key, base.Add(-time.Minute)).Points); !sameFloats(got, []float64{10, 15}) {
		t.Fatalf("increases = %v, want [10 15]: no baseline lost, nothing repeated", got)
	}
}

func TestAGapLongerThanTheActiveWindowStartsANewBaseline(t *testing.T) {
	s := openTestStore(t)
	base := baseMinute()
	key := "requests|{}"
	old := base.Add(-2 * ActiveSeriesWindow).Add(time.Second)
	writeAt(s, old, sumReading(key, 100, true, old, old))
	// 48 hours of counting went by unseen: attributing them to one minute would be a spike that never happened.
	writeAt(s, base, sumReading(key, 9000, true, old, base))
	writeAt(s, base.Add(time.Minute), sumReading(key, 9040, true, old, base.Add(time.Minute)))
	if got := valuesOf(queryAll(t, s, key, base.Add(-time.Minute)).Points); !sameFloats(got, []float64{40}) {
		t.Fatalf("increases = %v, want only [40]", got)
	}
}

func TestSeriesStateIsBoundedBySeriesAndRemovedWithThem(t *testing.T) {
	s := openTestStore(t)
	base := baseMinute()
	var samples []Sample
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("m%d|{}", i)
		samples = append(samples, sumReading(key, 1, true, base, base))
	}
	b := otlpBatch(base, samples...)
	b.SeriesCap = 3
	s.Enqueue(b)
	s.Flush()
	count := func(table string) int {
		var n int
		s.db.Raw("SELECT COUNT(*) FROM " + table).Scan(&n)
		return n
	}
	if count("series_state") != 3 || s.Stats().DroppedCap != 2 {
		t.Fatalf("state rows %d, dropped %d: a series over the cap must not hold state", count("series_state"), s.Stats().DroppedCap)
	}
	s.db.Exec("UPDATE series SET last_seen = ?", time.Now().Add(-100*24*time.Hour).Unix())
	if pr, err := s.Prune(Retention{Raw: time.Hour, Rollup: 90 * 24 * time.Hour}, time.Now()); err != nil || pr.Series != 3 {
		t.Fatalf("prune = %+v %v", pr, err)
	}
	if count("series_state") != 0 {
		t.Fatalf("state left after its series was pruned: %d", count("series_state"))
	}
}

func TestAGaugeIsStillStoredAsSentAndReplacesItsMinute(t *testing.T) {
	s := openTestStore(t)
	base := baseMinute()
	g := func(v float64) Sample {
		return Sample{Key: "mem|{}", Kind: KindGauge, Name: "mem", Metric: "mem", Values: Values{Value: fp(v)}}
	}
	writeAt(s, base, g(10))
	writeAt(s, base.Add(5*time.Second), g(30))
	series := queryAll(t, s, "mem|{}", base.Add(-time.Minute))
	if got := valuesOf(series.Points); !sameFloats(got, []float64{30}) || *series.Points[0].ValueMax != 30 {
		t.Fatalf("gauge = %v", got)
	}
}

func TestHourlyRollupOfASumKeepsTheTotalAndThePeakMinute(t *testing.T) {
	s := openTestStore(t)
	now := time.Now().UTC().Truncate(time.Hour).Add(30 * time.Minute)
	start := now.Add(-72 * time.Hour).Truncate(time.Hour)
	key := "jobs|{}"
	for h := 0; h < 72; h++ {
		for m, v := range []float64{1, 2, 3} {
			at := start.Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute)
			writeAt(s, at, sumReading(key, v, false, start, at))
		}
	}
	if _, err := s.Rollup(now); err != nil {
		t.Fatal(err)
	}
	res, err := s.Query(Query{Series: []SeriesRef{refOTLP(key)}, From: start, To: now.Add(time.Hour)})
	if err != nil || res.Resolution != "hourly" || len(res.Series[0].Points) != 72 {
		t.Fatalf("query = %+v %v", res, err)
	}
	p := res.Series[0].Points[0]
	if *p.Value != 6 || *p.ValueMax != 3 {
		t.Fatalf("an hour holds the total of its minutes (6) and its largest minute (3), got %v and %v", *p.Value, *p.ValueMax)
	}
	var all float64
	wide, _ := s.Query(Query{Series: []SeriesRef{refOTLP(key)}, From: start, To: now.Add(time.Hour), MaxPoints: 1})
	for _, w := range wide.Series[0].Points {
		all += *w.Value
	}
	if all != 72*6 {
		t.Fatalf("a wide step totals %v, want %d", all, 72*6)
	}
}
