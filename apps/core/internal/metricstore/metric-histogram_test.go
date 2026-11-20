package metricstore

import (
	"fmt"
	"math"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"testing"
	"time"
)

// latencyBounds are the bounds of the histograms in these tests, in milliseconds.
var latencyBounds = []float64{10, 20, 50, 100}

// observe builds the delta histogram of the given observations over latencyBounds.
func observe(values ...float64) Histogram {
	h := Histogram{Count: int64(len(values)), Bounds: latencyBounds, Counts: make([]int64, len(latencyBounds)+1)}
	var sum float64
	for _, v := range values {
		sum += v
		i := 0
		for i < len(latencyBounds) && v > latencyBounds[i] {
			i++
		}
		h.Counts[i]++
		if h.Min == nil || v < *h.Min {
			h.Min = fp(v)
		}
		if h.Max == nil || v > *h.Max {
			h.Max = fp(v)
		}
	}
	h.Sum = &sum
	return h
}

// spread returns n observations evenly spread over (0, top].
func spread(n int, top float64) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = top * float64(i+1) / float64(n)
	}
	return out
}

const latencyKey = "histogram:http.server.duration|{}"

func TestHistogramPointsCarryCountSumAvgAndPercentiles(t *testing.T) {
	s := openTestStore(t)
	base := baseMinute()
	// Minute 0: 100 observations over (0,100]; minute 1: 100 observations, all slow.
	writeAt(s, base, histReading(latencyKey, observe(spread(100, 100)...), false, base, base))
	slow := make([]float64, 100)
	for i := range slow {
		slow[i] = 90
	}
	writeAt(s, base.Add(time.Minute), histReading(latencyKey, observe(slow...), false, base, base.Add(time.Minute)))

	res, err := s.Query(Query{Series: []SeriesRef{refOTLP(latencyKey)}, From: base, To: base.Add(10 * time.Minute)})
	if err != nil || len(res.Series) != 1 {
		t.Fatalf("query = %+v %v", res, err)
	}
	series := res.Series[0]
	if series.Kind != KindHistogram || len(series.Points) != 2 || res.Resolution != "raw" {
		t.Fatalf("series = %+v", series)
	}
	p := series.Points[0]
	if *p.Count != 100 || !near(*p.Sum, 5050) || !near(*p.Avg, 50.5) || *p.Min != 1 || *p.Max != 100 {
		t.Fatalf("point 0 = %+v", p)
	}
	// 1..100 evenly in buckets (0,10] (10,20] (20,50] (50,100]: interpolation inside them gives the true quantiles.
	if !near(*p.P50, 50) || !near(*p.P95, 95) || !near(*p.P99, 99) {
		t.Fatalf("percentiles = %v %v %v", *p.P50, *p.P95, *p.P99)
	}
	if p.Value != nil || p.CPUPercent != nil || p.Quantiles != nil {
		t.Fatalf("a histogram point carries no gauge value: %+v", p)
	}
	// A narrower step merges the two minutes: the merged distribution, not an average of percentiles.
	merged, _ := s.Query(Query{Series: []SeriesRef{refOTLP(latencyKey)}, From: base, To: base.Add(10 * time.Minute), MaxPoints: 1})
	mp := merged.Series[0].Points
	if len(mp) != 1 || *mp[0].Count != 200 || !near(*mp[0].Avg, (5050+9000)/200.0) {
		t.Fatalf("merged = %+v", mp)
	}
	wantP95, _ := mergedHistogram(observe(spread(100, 100)...), observe(slow...)).Percentile(0.95)
	if !near(*mp[0].P95, wantP95) || *mp[0].P95 <= *series.Points[0].P95 {
		t.Fatalf("merged p95 = %v, want %v from the combined buckets", *mp[0].P95, wantP95)
	}
}

func mergedHistogram(hs ...Histogram) Histogram {
	var out Histogram
	for _, h := range hs {
		out.Merge(h)
	}
	return out
}

func TestHistogramFieldsThatWereNotMeasuredAreAbsent(t *testing.T) {
	s := openTestStore(t)
	base := baseMinute()
	noSum := Histogram{Count: 3, Bounds: []float64{1}, Counts: []int64{2, 1}}
	writeAt(s, base, histReading(latencyKey, noSum, false, base, base))
	empty := Histogram{Count: 0, Bounds: []float64{1}, Counts: []int64{0, 0}}
	writeAt(s, base.Add(time.Minute), histReading(latencyKey, empty, false, base, base.Add(time.Minute)))
	bucketless := Histogram{Count: 4, Sum: fp(8)}
	writeAt(s, base.Add(2*time.Minute), histReading(latencyKey, bucketless, false, base, base.Add(2*time.Minute)))

	pts := queryAll(t, s, latencyKey, base).Points
	if len(pts) != 3 {
		t.Fatalf("points = %+v", pts)
	}
	if *pts[0].Count != 3 || pts[0].Sum != nil || pts[0].Avg != nil || pts[0].Min != nil || pts[0].P50 == nil {
		t.Fatalf("no sum sent: %+v", pts[0])
	}
	if pts[1].Count == nil || *pts[1].Count != 0 || pts[1].Avg != nil || pts[1].P50 != nil || pts[1].P99 != nil {
		t.Fatalf("an interval without observations has a count of 0 and nothing else: %+v", pts[1])
	}
	if *pts[2].Count != 4 || *pts[2].Avg != 2 || pts[2].P50 != nil || pts[2].P95 != nil {
		t.Fatalf("count and sum only: %+v", pts[2])
	}
}

func TestDeltaHistogramsAddUpWithinAMinute(t *testing.T) {
	s := openTestStore(t)
	base := baseMinute()
	for i := 0; i < 3; i++ {
		at := base.Add(time.Duration(i) * 10 * time.Second)
		writeAt(s, at, histReading(latencyKey, observe(5, 15, 60), false, base, at))
	}
	pts := queryAll(t, s, latencyKey, base).Points
	if len(pts) != 1 || *pts[0].Count != 9 || !near(*pts[0].Sum, 240) || *pts[0].Min != 5 || *pts[0].Max != 60 {
		t.Fatalf("points = %+v", pts)
	}
}

func TestCumulativeHistogramsBecomePerMinuteCounts(t *testing.T) {
	s := openTestStore(t)
	base := baseMinute()
	start := base.Add(-time.Hour)
	cumulative := observe()
	total := []float64{}
	for i, batch := range [][]float64{{5, 15}, {25, 25, 70}, {8}} {
		total = append(total, batch...)
		cumulative = observe(total...)
		at := base.Add(time.Duration(i) * time.Minute)
		writeAt(s, at, histReading(latencyKey, cumulative, true, start, at))
	}
	pts := queryAll(t, s, latencyKey, base).Points
	// The first reading is the baseline: 2 observations before it are not counted. Then 3 and 1.
	if len(pts) != 2 || *pts[0].Count != 3 || *pts[1].Count != 1 || !near(*pts[0].Sum, 120) || !near(*pts[1].Sum, 8) {
		t.Fatalf("points = %+v", pts)
	}
	if p50 := *pts[0].P50; p50 <= 20 || p50 > 50 {
		t.Fatalf("the interval's own median is in (20,50]: %v", p50)
	}
	// The exporter restarts: its histogram starts again and the new reading is the interval's content.
	at := base.Add(3 * time.Minute)
	writeAt(s, at, histReading(latencyKey, observe(12, 12), true, at.Add(-time.Second), at))
	pts = queryAll(t, s, latencyKey, base).Points
	if len(pts) != 3 || *pts[2].Count != 2 {
		t.Fatalf("after a restart = %+v", pts)
	}
}

func TestSummariesKeepTheirQuantilesAndNeverClaimBuckets(t *testing.T) {
	s := openTestStore(t)
	base := baseMinute()
	key := "summary:rpc.latency|{}"
	reading := func(count int64, sum, p50, p99 float64, at time.Time) Sample {
		return histReading(key, Histogram{Count: count, Sum: fp(sum), Quantiles: []Quantile{{0.5, p50}, {0.99, p99}}}, true, base.Add(-time.Hour), at)
	}
	writeAt(s, base, reading(100, 1000, 9, 30, base))
	writeAt(s, base.Add(time.Minute), reading(160, 1900, 12, 40, base.Add(time.Minute)))
	writeAt(s, base.Add(2*time.Minute), reading(200, 2300, 8, 20, base.Add(2*time.Minute)))
	series := queryAll(t, s, key, base)
	if series.Kind != KindSummary || len(series.Points) != 2 {
		t.Fatalf("series = %+v", series)
	}
	p := series.Points[0]
	if *p.Count != 60 || !near(*p.Sum, 900) || !near(*p.Avg, 15) || *p.P50 != 12 || *p.P99 != 40 || p.P95 != nil || len(p.Quantiles) != 2 {
		t.Fatalf("point = %+v: count and sum are changes, quantiles as sent, p95 was not reported", p)
	}
	// Two minutes in one step: quantiles are averaged, weighted by observations (60 and 40).
	merged, _ := s.Query(Query{Series: []SeriesRef{refOTLP(key)}, From: base, To: base.Add(10 * time.Minute), MaxPoints: 1})
	mp := merged.Series[0].Points[0]
	if *mp.Count != 100 || !near(*mp.P50, (12*60+8*40)/100.0) {
		t.Fatalf("merged = %+v", mp)
	}
	var buckets int
	s.db.Raw("SELECT COUNT(*) FROM histogram_samples WHERE bounds != '' OR counts != ''").Scan(&buckets)
	if buckets != 0 {
		t.Fatalf("a summary stored %d rows with buckets", buckets)
	}
}

// Three days of one-minute histograms for one series. Every hour holds the same 120 observations (two
// minutes of 60), except that one hour came from an exporter with other bounds, and the last hour is slow.
func fillThreeDays(t *testing.T, s *Store, now time.Time) (start time.Time, all []Histogram, times []int64) {
	t.Helper()
	start = now.Add(-72 * time.Hour).Truncate(time.Hour)
	for h := 0; h < 72; h++ {
		for m := 0; m < 2; m++ {
			at := start.Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute)
			hist := observe(spread(60, 100)...)
			if h == 71 {
				hist = observe(spread(60, 400)...)
			}
			if h == 30 {
				hist = Histogram{Count: 60, Sum: fp(3000), Bounds: []float64{25, 75}, Counts: []int64{15, 30, 15}, Min: fp(1), Max: fp(100)}
			}
			all, times = append(all, hist), append(times, at.Unix())
			writeAt(s, at, histReading(latencyKey, hist, false, start, at))
		}
	}
	return start, all, times
}

func TestHourlyRollupMergesBucketsSoLongRangesKeepTrueQuantiles(t *testing.T) {
	s := openTestStore(t)
	now := time.Now().UTC().Truncate(time.Hour).Add(30 * time.Minute)
	start, all, times := fillThreeDays(t, s, now)
	hours, err := s.Rollup(now)
	if err != nil || hours < 70 {
		t.Fatalf("rollup = %d, %v", hours, err)
	}
	to := now.Add(time.Hour)
	res, err := s.Query(Query{Series: []SeriesRef{refOTLP(latencyKey)}, From: start, To: to, MaxPoints: 2000})
	if err != nil || res.Resolution != "hourly" || len(res.Series) != 1 {
		t.Fatalf("query = %+v %v", res, err)
	}
	pts := res.Series[0].Points
	if len(pts) != 72 {
		t.Fatalf("hourly points = %d, want 72", len(pts))
	}
	var count int64
	for _, p := range pts {
		count += *p.Count
	}
	if count != 72*120 {
		t.Fatalf("hourly counts add up to %d, want %d", count, 72*120)
	}
	// An ordinary hour holds two minutes of the same distribution: the same percentiles as one minute.
	one := observe(spread(60, 100)...)
	for _, q := range []float64{0.5, 0.95, 0.99} {
		want, _ := one.Percentile(q)
		got := map[float64]*float64{0.5: pts[0].P50, 0.95: pts[0].P95, 0.99: pts[0].P99}[q]
		if !near(*got, want) {
			t.Errorf("hour 0 p%v = %v, want %v", q, *got, want)
		}
	}
	// Three days become one or two points: the step widens, and the percentiles of a point come from
	// merging the buckets of all its hours, which equals merging the raw minutes of the same step (the oracle).
	wide, err := s.Query(Query{Series: []SeriesRef{refOTLP(latencyKey)}, From: start, To: to, MaxPoints: 1})
	if err != nil || len(wide.Series[0].Points) == 0 || len(wide.Series[0].Points) > 2 {
		t.Fatalf("wide = %+v %v", wide, err)
	}
	var wideCount int64
	for _, w := range wide.Series[0].Points {
		oracle := Histogram{}
		for i, h := range all {
			if times[i]/int64(wide.StepSeconds)*int64(wide.StepSeconds) == w.T.Unix() {
				oracle.Merge(h)
			}
		}
		if *w.Count != oracle.Count {
			t.Fatalf("step %s: count %d, oracle %d", w.T, *w.Count, oracle.Count)
		}
		wideCount += *w.Count
		for q, got := range map[float64]*float64{0.5: w.P50, 0.95: w.P95, 0.99: w.P99} {
			want, _ := oracle.Percentile(q)
			if math.Abs(*got-want) > 0.01*math.Max(want, 1) {
				t.Errorf("step %s: p%v = %v, oracle %v", w.T, q, *got, want)
			}
		}
	}
	if wideCount != 72*120 {
		t.Fatalf("the wide points count %d observations, want %d", wideCount, 72*120)
	}
	// The hour with different bounds still counts every observation, and the slow last hour shows.
	if *pts[30].Count != 120 || *pts[71].P99 <= *pts[0].P99*2 {
		t.Fatalf("hour 30 count %d, last hour p99 %v vs first %v", *pts[30].Count, *pts[71].P99, *pts[0].P99)
	}
}

func TestPruneRemovesHistogramRowsStateAndRollupsLikeSamples(t *testing.T) {
	s := openTestStore(t)
	now := time.Now().UTC().Truncate(time.Hour).Add(30 * time.Minute)
	_, _, _ = fillThreeDays(t, s, now)
	if _, err := s.Rollup(now); err != nil {
		t.Fatal(err)
	}
	count := func(q string) int64 {
		var n int64
		s.db.Raw(q).Scan(&n)
		return n
	}
	rawBefore, hourlyBefore := count("SELECT COUNT(*) FROM histogram_samples"), count("SELECT COUNT(*) FROM histogram_hourly")
	if rawBefore != 144 || hourlyBefore < 70 || count("SELECT COUNT(*) FROM series_state") != 1 {
		t.Fatalf("before: raw %d hourly %d", rawBefore, hourlyBefore)
	}
	pr, err := s.Prune(Retention{Raw: 24 * time.Hour, Rollup: 48 * time.Hour}, now)
	if err != nil || pr.RawRows == 0 || pr.HourlyRows == 0 {
		t.Fatalf("prune = %+v, %v", pr, err)
	}
	var oldestRaw, oldestHour int64
	s.db.Raw("SELECT MIN(ts) FROM histogram_samples").Scan(&oldestRaw)
	s.db.Raw("SELECT MIN(hour) FROM histogram_hourly").Scan(&oldestHour)
	if time.Unix(oldestRaw, 0).Before(now.Add(-26*time.Hour)) || time.Unix(oldestHour, 0).Before(now.Add(-49*time.Hour)) {
		t.Fatalf("oldest raw %s hourly %s survived", time.Unix(oldestRaw, 0), time.Unix(oldestHour, 0))
	}
	// A series that stopped reporting goes away with every row it owned.
	s.db.Exec("UPDATE series SET last_seen = ?", now.Add(-100*24*time.Hour).Unix())
	if pr, _ := s.Prune(Retention{Raw: 24 * time.Hour, Rollup: 48 * time.Hour}, now); pr.Series != 1 {
		t.Fatalf("stale prune = %+v", pr)
	}
	for _, table := range []string{"series", "histogram_samples", "histogram_hourly", "series_state"} {
		if n := count("SELECT COUNT(*) FROM " + table); n != 0 {
			t.Errorf("%s still has %d rows", table, n)
		}
	}
}

func TestPruneKeepsHistogramRowsThatHaveNoRollupYet(t *testing.T) {
	s := openTestStore(t)
	now := time.Now().UTC()
	at := now.Add(-10 * 24 * time.Hour)
	writeAt(s, at, histReading(latencyKey, observe(5), false, at, at))
	if pr, err := s.Prune(Retention{Raw: time.Hour, Rollup: 90 * 24 * time.Hour}, now); err != nil || pr.RawRows != 0 {
		t.Fatalf("prune before the rollup = %+v %v", pr, err)
	}
	if _, err := s.Rollup(now); err != nil {
		t.Fatal(err)
	}
	if pr, _ := s.Prune(Retention{Raw: time.Hour, Rollup: 90 * 24 * time.Hour}, now); pr.RawRows != 1 {
		t.Fatalf("prune after the rollup = %+v", pr)
	}
}

func TestALateHistogramReadingMarksItsHourForANewRollup(t *testing.T) {
	s := openTestStore(t)
	now := time.Now().UTC().Truncate(time.Hour).Add(30 * time.Minute)
	hour := now.Add(-5 * time.Hour).Truncate(time.Hour)
	writeAt(s, hour.Add(time.Minute), histReading(latencyKey, observe(5, 5), false, hour, hour.Add(time.Minute)))
	if _, err := s.Rollup(now); err != nil {
		t.Fatal(err)
	}
	writeAt(s, hour.Add(5*time.Minute), histReading(latencyKey, observe(15), false, hour, hour.Add(5*time.Minute)))
	if _, err := s.Rollup(now); err != nil {
		t.Fatal(err)
	}
	var n int64
	s.db.Raw("SELECT count FROM histogram_hourly WHERE hour = ?", hour.Unix()).Scan(&n)
	if n != 3 {
		t.Fatalf("the hourly row counts %d observations, want 3 after the late one", n)
	}
}

func TestHistogramSeriesCountAgainstTheSeriesCapAndTheyAreListedByApp(t *testing.T) {
	s := openTestStore(t)
	base := baseMinute()
	var samples []Sample
	for i := 0; i < 4; i++ {
		samples = append(samples, histReading(fmt.Sprintf("histogram:m%d|{}", i), observe(5), false, base, base))
	}
	b := otlpBatch(base, samples...)
	b.SeriesCap = 3
	s.Enqueue(b)
	s.Flush()
	if st := s.Stats(); st.Written != 3 || st.DroppedCap != 1 {
		t.Fatalf("stats = %+v", st)
	}
	res, _ := s.Query(Query{AppIDs: []string{"app-1"}, From: base, To: base.Add(time.Hour)})
	if len(res.Series) != 3 || len(res.Series[0].Points) != 1 {
		t.Fatalf("by app = %+v", res)
	}
}

func TestOpeningAnOlderDatabaseAddsTheNewTablesAndDropsOnlyOldSumHistory(t *testing.T) {
	dir := t.TempDir()
	old, err := db.OpenFile(dir, metricsDatabaseFileName, "warn")
	if err != nil {
		t.Fatal(err)
	}
	// The tables of the release before histograms, with an OTLP sum (cumulative values), an OTLP gauge and an OS series.
	for _, stmt := range []string{
		`CREATE TABLE series (id INTEGER PRIMARY KEY, source TEXT NOT NULL, node_id TEXT NOT NULL DEFAULT '', app_id TEXT NOT NULL DEFAULT '', key TEXT NOT NULL,
  kind TEXT NOT NULL DEFAULT '', name TEXT NOT NULL DEFAULT '', metric TEXT NOT NULL DEFAULT '', unit TEXT NOT NULL DEFAULT '', labels TEXT NOT NULL DEFAULT '',
  first_seen INTEGER NOT NULL, last_seen INTEGER NOT NULL)`,
		`CREATE UNIQUE INDEX idx_series_ref ON series(source, node_id, app_id, key)`,
		`CREATE TABLE samples (series_id INTEGER NOT NULL, ts INTEGER NOT NULL, cpu_percent REAL, memory_bytes INTEGER, io_read_bps REAL, io_write_bps REAL,
  net_rx_bps REAL, net_tx_bps REAL, pids INTEGER, storage_bytes INTEGER, value REAL, PRIMARY KEY (series_id, ts)) WITHOUT ROWID`,
		`CREATE TABLE samples_hourly (series_id INTEGER NOT NULL, hour INTEGER NOT NULL, n INTEGER NOT NULL, cpu_avg REAL, cpu_max REAL, mem_avg REAL, mem_max REAL,
  io_read_avg REAL, io_write_avg REAL, net_rx_avg REAL, net_tx_avg REAL, pids_max INTEGER, storage_max INTEGER, value_avg REAL, value_max REAL,
  PRIMARY KEY (series_id, hour)) WITHOUT ROWID`,
		`CREATE TABLE meta (k TEXT PRIMARY KEY, v INTEGER NOT NULL) WITHOUT ROWID`,
		`CREATE TABLE dirty_hours (series_id INTEGER NOT NULL, hour INTEGER NOT NULL, PRIMARY KEY (series_id, hour)) WITHOUT ROWID`,
		`INSERT INTO series (id, source, app_id, key, kind, first_seen, last_seen) VALUES (1, 'otlp', 'app-1', 'requests|{}', 'sum', 1, 2), (2, 'otlp', 'app-1', 'mem|{}', 'gauge', 1, 2), (3, 'os', 'app-1', 'p', 'process', 1, 2)`,
		`INSERT INTO samples (series_id, ts, value) VALUES (1, 60, 1000), (1, 120, 1100), (2, 60, 5), (3, 60, 7)`,
		`INSERT INTO samples_hourly (series_id, hour, n, value_avg, value_max) VALUES (1, 0, 2, 1050, 1100), (2, 0, 1, 5, 5)`,
		`INSERT INTO dirty_hours (series_id, hour) VALUES (1, 0), (2, 0)`,
	} {
		if err := old.Exec(stmt).Error; err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	s, err := OpenWith(old, utils.NewLogger())
	if err != nil {
		t.Fatalf("an older file must open: %v", err)
	}
	count := func(q string) int64 {
		var n int64
		s.db.Raw(q).Scan(&n)
		return n
	}
	if count("SELECT COUNT(*) FROM samples WHERE series_id = 1") != 0 || count("SELECT COUNT(*) FROM samples_hourly WHERE series_id = 1") != 0 ||
		count("SELECT COUNT(*) FROM dirty_hours WHERE series_id = 1") != 0 {
		t.Fatal("the cumulative history of the old sum series must be removed")
	}
	if count("SELECT COUNT(*) FROM samples WHERE series_id IN (2, 3)") != 2 || count("SELECT COUNT(*) FROM samples_hourly WHERE series_id = 2") != 1 ||
		count("SELECT COUNT(*) FROM series") != 3 {
		t.Fatal("gauge and OS history, and every series, must stay")
	}
	// The new tables are usable on the upgraded file.
	base := baseMinute()
	writeAt(s, base, histReading(latencyKey, observe(5), false, base, base))
	writeAt(s, base, sumReading("requests|{}", 4, false, base, base))
	if got := valuesOf(queryAll(t, s, "requests|{}", base.Add(-time.Minute)).Points); !sameFloats(got, []float64{4}) {
		t.Fatalf("sum after the upgrade = %v", got)
	}
	s.Close()

	// A second start is a no-op: what was written since is not removed again.
	again, err := Open(dir, utils.NewLogger())
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	if got := valuesOf(queryAll(t, again, "requests|{}", base.Add(-time.Minute)).Points); !sameFloats(got, []float64{4}) {
		t.Fatalf("sum after the second start = %v", got)
	}
}
