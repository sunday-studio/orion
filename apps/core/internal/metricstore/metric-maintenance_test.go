package metricstore

import (
	"testing"
	"time"
)

func countRows(t *testing.T, s *Store, table string) int64 {
	t.Helper()
	var n int64
	if err := s.db.Raw("SELECT COUNT(*) FROM " + table).Scan(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

func TestASeriesThatReportsAgainAfterBeingPrunedIsRecreated(t *testing.T) {
	s := openTestStore(t)
	now := time.Now().UTC()
	s.Enqueue(osBatch("n1", now.Add(-200*24*time.Hour), cpuSample("job", 1)))
	s.Flush()
	// Make the series look 200 days idle, in the table and in the writer's cache (the writer is idle after Flush).
	old := now.Add(-200 * 24 * time.Hour).Unix()
	s.db.Exec("UPDATE series SET last_seen = ?", old)
	for k, c := range s.cache {
		c.lastSeen = old
		s.cache[k] = c
	}
	// The series has been idle past the rollup retention, so Prune removes it while the writer still caches its id.
	pr, err := s.Prune(Retention{Raw: 24 * time.Hour, Rollup: 90 * 24 * time.Hour}, now)
	if err != nil || pr.Series != 1 {
		t.Fatalf("prune = %+v, %v", pr, err)
	}
	s.Enqueue(osBatch("n1", now, cpuSample("job", 7)))
	s.Flush()

	res, err := s.Query(Query{Series: []SeriesRef{refOS("n1", "job")}, From: now.Add(-time.Hour), To: now.Add(time.Hour)})
	if err != nil || len(res.Series) != 1 || len(res.Series[0].Points) != 1 || *res.Series[0].Points[0].CPUPercent != 7 {
		t.Fatalf("query after re-report = %+v, %v", res, err)
	}
	var orphans int64
	s.db.Raw("SELECT COUNT(*) FROM samples WHERE series_id NOT IN (SELECT id FROM series)").Scan(&orphans)
	if orphans != 0 {
		t.Fatalf("%d samples were written under a series id that no longer exists", orphans)
	}
	if st := s.Stats(); st.BatchesFailed != 0 {
		t.Fatalf("stats = %+v", st)
	}
}

func TestLateSamplesAreSummarisedAgainAndSurvivePrune(t *testing.T) {
	s := openTestStore(t)
	now := time.Now().UTC().Truncate(time.Hour).Add(30 * time.Minute)
	hour := now.Add(-5 * time.Hour).Truncate(time.Hour)
	s.Enqueue(osBatch("n1", hour.Add(10*time.Minute), cpuSample("a", 10)))
	s.Flush()
	if _, err := s.Rollup(now); err != nil {
		t.Fatal(err)
	}
	hourly := func() (n int64, cpu float64) {
		s.db.Raw("SELECT COALESCE(SUM(n), 0), COALESCE(MAX(cpu_avg), 0) FROM samples_hourly WHERE hour = ?", hour.Unix()).Row().Scan(&n, &cpu)
		return
	}
	if n, cpu := hourly(); n != 1 || cpu != 10 {
		t.Fatalf("first rollup: n=%d cpu=%v", n, cpu)
	}

	// An exporter retries after an outage and delivers a point of that hour five hours late.
	s.Enqueue(osBatch("n1", hour.Add(40*time.Minute), cpuSample("a", 30)))
	s.Flush()
	if got := countRows(t, s, "dirty_hours"); got != 1 {
		t.Fatalf("dirty hours = %d, want the late hour marked", got)
	}
	// Prune must not delete a raw row whose hour is waiting to be summarised again.
	if pr, err := s.Prune(Retention{Raw: time.Hour, Rollup: 90 * 24 * time.Hour}, now); err != nil || pr.RawRows != 0 {
		t.Fatalf("prune before the re-rollup = %+v, %v; the late row has no summary yet", pr, err)
	}
	if _, err := s.Rollup(now); err != nil {
		t.Fatal(err)
	}
	if n, cpu := hourly(); n != 2 || cpu != 20 {
		t.Fatalf("after the re-rollup: n=%d cpu=%v, want both points averaged", n, cpu)
	}
	if got := countRows(t, s, "dirty_hours"); got != 0 {
		t.Fatalf("dirty hours left = %d", got)
	}
	if pr, _ := s.Prune(Retention{Raw: time.Hour, Rollup: 90 * 24 * time.Hour}, now); pr.RawRows != 2 {
		t.Fatalf("prune after the re-rollup = %+v, want both raw rows gone", pr)
	}
}

func TestHourlyAveragesAreWeightedByTheRowsTheyCover(t *testing.T) {
	s := openTestStore(t)
	now := time.Now().UTC().Truncate(time.Hour).Add(30 * time.Minute)
	full := now.Add(-70 * time.Hour).Truncate(time.Hour)
	// One hour with 60 samples at 10, the next with a single sample at 100: the 2h average is (600+100)/61, not 55.
	for m := 0; m < 60; m++ {
		s.Enqueue(osBatch("n1", full.Add(time.Duration(m)*time.Minute), cpuSample("a", 10)))
	}
	s.Enqueue(osBatch("n1", full.Add(time.Hour), cpuSample("a", 100)))
	s.Flush()
	if _, err := s.Rollup(now); err != nil {
		t.Fatal(err)
	}
	res, err := s.Query(Query{Series: []SeriesRef{refOS("n1", "a")}, From: full, To: full.Add(2 * time.Hour), MaxPoints: 1})
	if err != nil {
		t.Fatal(err)
	}
	if res.Resolution != "raw" {
		t.Fatalf("a 2 hour range reads raw rows: %s", res.Resolution)
	}
	// Force the hourly reader on the same data by asking over a long range with a coarse step.
	res, err = s.Query(Query{Series: []SeriesRef{refOS("n1", "a")}, From: full, To: full.Add(72 * time.Hour), MaxPoints: 1})
	if err != nil || len(res.Series) != 1 || res.Resolution != "hourly" {
		t.Fatalf("query = %+v, %v", res, err)
	}
	var avg float64
	for _, p := range res.Series[0].Points {
		if p.CPUPercent != nil {
			avg = *p.CPUPercent
		}
	}
	want := 700.0 / 61
	if avg < want-0.01 || avg > want+0.01 {
		t.Fatalf("hourly average = %v, want %v", avg, want)
	}
}

func TestRollupDoesNotScanTheWholeTable(t *testing.T) {
	s := openTestStore(t)
	var plan []struct {
		Detail string `gorm:"column:detail"`
	}
	if err := s.db.Raw("EXPLAIN QUERY PLAN "+rollupSQL, 1, 0, 3600).Scan(&plan).Error; err != nil {
		t.Fatal(err)
	}
	for _, row := range plan {
		if row.Detail == "SCAN samples" {
			t.Fatalf("the rollup query scans the whole samples table: %v", plan)
		}
	}
}

func TestSeriesCacheStaysBounded(t *testing.T) {
	s := openTestStore(t)
	for i := 0; i < maxCachedSeries+3; i++ {
		s.remember(seriesKey{source: SourceOS, key: string(rune(i%1000)) + string(rune(i/1000))}, cachedSeries{id: int64(i)})
	}
	if len(s.cache) > maxCachedSeries {
		t.Fatalf("cache grew to %d", len(s.cache))
	}
}

func TestQueryTruncatesToMaxSeries(t *testing.T) {
	s := openTestStore(t)
	now := time.Now().UTC()
	var samples []Sample
	var refs []SeriesRef
	for i := 0; i < 5; i++ {
		key := string(rune('a' + i))
		samples = append(samples, cpuSample(key, 1))
		refs = append(refs, refOS("n1", key))
	}
	s.Enqueue(osBatch("n1", now, samples...))
	s.Flush()
	res, err := s.Query(Query{Series: refs, From: now.Add(-time.Hour), To: now.Add(time.Hour), MaxSeries: 3})
	if err != nil || len(res.Series) != 3 || !res.Truncated {
		t.Fatalf("result = %d series, truncated %v, %v", len(res.Series), res.Truncated, err)
	}
	if res, _ := s.Query(Query{Series: refs, From: now.Add(-time.Hour), To: now.Add(time.Hour)}); len(res.Series) != 5 || res.Truncated {
		t.Fatalf("default limit = %d series, truncated %v", len(res.Series), res.Truncated)
	}
}
