package metricstore

import (
	"fmt"
	"orion/core/internal/utils"
	"testing"
	"time"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(t.TempDir(), utils.NewLogger())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}

func fp(v float64) *float64 { return &v }
func ip(v int64) *int64     { return &v }

func osBatch(node string, at time.Time, samples ...Sample) Batch {
	return Batch{Source: SourceOS, NodeID: node, At: at, Samples: samples}
}

func cpuSample(key string, cpu float64) Sample {
	return Sample{Key: key, Kind: "container", Name: key, Values: Values{CPUPercent: fp(cpu), MemoryBytes: ip(1 << 20)}}
}

func refOS(node, key string) SeriesRef { return SeriesRef{Source: SourceOS, NodeID: node, Key: key} }

func TestWriteAndQueryRawSamples(t *testing.T) {
	s := openTestStore(t)
	base := time.Now().UTC().Truncate(time.Minute).Add(-30 * time.Minute)
	for i := 0; i < 10; i++ {
		if err := s.Enqueue(osBatch("n1", base.Add(time.Duration(i)*time.Minute), cpuSample("container:web", float64(10+i)))); err != nil {
			t.Fatal(err)
		}
	}
	s.Flush()
	res, err := s.Query(Query{Series: []SeriesRef{refOS("n1", "container:web")}, From: base, To: base.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if res.Resolution != "raw" || res.StepSeconds != 60 || len(res.Series) != 1 || len(res.Series[0].Points) != 10 {
		t.Fatalf("result = %+v", res)
	}
	first, last := res.Series[0].Points[0], res.Series[0].Points[9]
	if *first.CPUPercent != 10 || *last.CPUPercent != 19 || !first.T.Equal(base) {
		t.Fatalf("points = %+v ... %+v", first, last)
	}
	if first.IOReadBps != nil || first.Value != nil {
		t.Fatalf("an unmeasured field must stay absent, got %+v", first)
	}
	if res.Series[0].Name != "container:web" || res.Series[0].Kind != "container" {
		t.Fatalf("series meta = %+v", res.Series[0])
	}
}

func TestSecondBatchInTheSameMinuteOverwrites(t *testing.T) {
	s := openTestStore(t)
	at := time.Now().UTC().Truncate(time.Minute).Add(-10 * time.Minute)
	s.Enqueue(osBatch("n1", at.Add(5*time.Second), cpuSample("a", 10)))
	s.Enqueue(osBatch("n1", at.Add(40*time.Second), cpuSample("a", 30)))
	s.Flush()
	res, _ := s.Query(Query{Series: []SeriesRef{refOS("n1", "a")}, From: at.Add(-time.Minute), To: at.Add(time.Minute)})
	if len(res.Series[0].Points) != 1 || *res.Series[0].Points[0].CPUPercent != 30 {
		t.Fatalf("points = %+v", res.Series[0].Points)
	}
}

func TestSeriesCapDropsTheExcessWithoutFailingTheBatch(t *testing.T) {
	s := openTestStore(t)
	var samples []Sample
	for i := 0; i < 7; i++ {
		samples = append(samples, cpuSample(fmt.Sprintf("p%d", i), 1))
	}
	b := osBatch("n1", time.Now(), samples...)
	b.SeriesCap = 5
	if err := s.Enqueue(b); err != nil {
		t.Fatal(err)
	}
	s.Flush()
	st := s.Stats()
	if st.Written != 5 || st.DroppedCap != 2 || st.BatchesFailed != 0 {
		t.Fatalf("stats = %+v", st)
	}
	// Another node has its own cap, and an already known series keeps writing at the cap.
	if err := s.Enqueue(osBatch("n2", time.Now(), cpuSample("p0", 1))); err != nil {
		t.Fatal(err)
	}
	b2 := osBatch("n1", time.Now().Add(time.Minute), cpuSample("p0", 2))
	b2.SeriesCap = 5
	s.Enqueue(b2)
	s.Flush()
	if st := s.Stats(); st.Written != 7 || st.DroppedCap != 2 {
		t.Fatalf("stats after = %+v", st)
	}
}

func TestStaleSeriesDoNotHoldACapSlot(t *testing.T) {
	s := openTestStore(t)
	b := osBatch("n1", time.Now(), cpuSample("old", 1))
	b.SeriesCap = 1
	s.Enqueue(b)
	s.Flush()
	// The old series was last seen two days ago, e.g. a container that was redeployed.
	s.db.Exec("UPDATE series SET last_seen = ?", time.Now().Add(-48*time.Hour).Unix())
	b2 := osBatch("n1", time.Now(), cpuSample("new", 1))
	b2.SeriesCap = 1
	s.Enqueue(b2)
	s.Flush()
	if st := s.Stats(); st.DroppedCap != 0 || st.Written != 2 {
		t.Fatalf("stats = %+v", st)
	}
}

func TestRollupHourlyQueryAndPrune(t *testing.T) {
	s := openTestStore(t)
	now := time.Now().UTC().Truncate(time.Hour).Add(30 * time.Minute)
	// Three days of one-minute samples for one series: cpu = hour of day, so hourly averages are exact.
	start := now.Add(-72 * time.Hour).Truncate(time.Hour)
	for m := 0; m < 72*60; m++ {
		at := start.Add(time.Duration(m) * time.Minute)
		s.Enqueue(osBatch("n1", at, cpuSample("a", float64(at.Hour()))))
		if m%500 == 0 {
			s.Flush() // keep the queue shallow
		}
	}
	s.Flush()

	hours, err := s.Rollup(now)
	if err != nil || hours < 70 {
		t.Fatalf("rollup hours = %d, %v", hours, err)
	}
	res, err := s.Query(Query{Series: []SeriesRef{refOS("n1", "a")}, From: start, To: now})
	if err != nil {
		t.Fatal(err)
	}
	if res.Resolution != "hourly" || res.StepSeconds < 3600 {
		t.Fatalf("a 3 day range should read rollups, got %s step %d", res.Resolution, res.StepSeconds)
	}
	for _, p := range res.Series[0].Points {
		if p.T.Hour() != int(*p.CPUPercent) {
			t.Fatalf("hour %s has cpu %v", p.T, *p.CPUPercent)
		}
	}

	// Prune with 24h raw retention: rolled raw rows older than that go, hourly stays.
	pr, err := s.Prune(Retention{Raw: 24 * time.Hour, Rollup: 90 * 24 * time.Hour}, now)
	if err != nil || pr.RawRows == 0 || pr.HourlyRows != 0 || pr.Series != 0 {
		t.Fatalf("prune = %+v, %v", pr, err)
	}
	var left int64
	s.db.Raw("SELECT MIN(ts) FROM samples").Scan(&left)
	if time.Unix(left, 0).Before(now.Add(-26 * time.Hour)) {
		t.Fatalf("oldest raw row is still %s", time.Unix(left, 0))
	}
	// A 90 day old series disappears with its rollups.
	s.db.Exec("UPDATE series SET last_seen = ?", now.Add(-100*24*time.Hour).Unix())
	pr, _ = s.Prune(Retention{Raw: 24 * time.Hour, Rollup: 90 * 24 * time.Hour}, now)
	var n int64
	s.db.Raw("SELECT COUNT(*) FROM series").Scan(&n)
	if pr.Series != 1 || n != 0 {
		t.Fatalf("stale series prune = %+v, series left %d", pr, n)
	}
}

func TestPruneKeepsRawRowsThatHaveNoRollupYet(t *testing.T) {
	s := openTestStore(t)
	now := time.Now().UTC()
	s.Enqueue(osBatch("n1", now.Add(-10*24*time.Hour), cpuSample("a", 1)))
	s.Flush()
	if pr, err := s.Prune(Retention{Raw: time.Hour, Rollup: 24 * time.Hour * 90}, now); err != nil || pr.RawRows != 0 {
		t.Fatalf("raw rows without a rollup must stay, got %+v, %v", pr, err)
	}
	if _, err := s.Rollup(now); err != nil {
		t.Fatal(err)
	}
	if pr, _ := s.Prune(Retention{Raw: time.Hour, Rollup: 24 * time.Hour * 90}, now); pr.RawRows != 1 {
		t.Fatalf("after the rollup the row can go, got %+v", pr)
	}
}

func TestQueryWidensTheStepToFitMaxPoints(t *testing.T) {
	s := openTestStore(t)
	base := time.Now().UTC().Truncate(time.Minute).Add(-6 * time.Hour)
	for m := 0; m < 360; m++ {
		s.Enqueue(osBatch("n1", base.Add(time.Duration(m)*time.Minute), cpuSample("a", 1)))
		if m%100 == 0 {
			s.Flush()
		}
	}
	s.Flush()
	res, _ := s.Query(Query{Series: []SeriesRef{refOS("n1", "a")}, From: base, To: base.Add(6 * time.Hour), MaxPoints: 60})
	if res.StepSeconds != 360 || len(res.Series[0].Points) > 61 {
		t.Fatalf("step %d points %d", res.StepSeconds, len(res.Series[0].Points))
	}
	if res, _ := s.Query(Query{Series: []SeriesRef{refOS("n1", "missing")}, From: base, To: base.Add(time.Hour)}); len(res.Series) != 0 {
		t.Fatalf("an unknown series should be skipped, got %+v", res.Series)
	}
}

func TestEnqueueNeverBlocksWhenTheQueueIsFull(t *testing.T) {
	// A store whose writer never runs: the second batch finds the queue full and is refused at once.
	s := &Store{jobs: make(chan job, 1), done: make(chan struct{})}
	if err := s.Enqueue(osBatch("n1", time.Now(), cpuSample("a", 1))); err != nil {
		t.Fatal(err)
	}
	if err := s.Enqueue(osBatch("n1", time.Now(), cpuSample("b", 1), cpuSample("c", 1))); err != ErrQueueFull {
		t.Fatalf("full queue = %v, want ErrQueueFull", err)
	}
	if got := s.droppedFull.Load(); got != 2 {
		t.Fatalf("dropped = %d, want the 2 samples of the refused batch", got)
	}
}

func TestEnqueueAfterCloseIsRefused(t *testing.T) {
	s := openTestStore(t)
	s.Close()
	if err := s.Enqueue(osBatch("n1", time.Now(), cpuSample("a", 1))); err != ErrQueueFull {
		t.Fatalf("enqueue after close = %v, want ErrQueueFull", err)
	}
}
