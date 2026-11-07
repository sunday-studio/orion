package metricstore

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"testing"
	"time"
)

// TestLoadAtTheSeriesCap is the measured gate for S4 (service-correlation-plan.md section 4.3). It
// is opt-in because it writes millions of rows:
//
//	ORION_METRIC_LOAD_NODES=1 go test ./internal/metricstore -run TestLoadAtTheSeriesCap -v -timeout 30m
//
// Each node reports 200 series (the cap) every minute for ORION_METRIC_LOAD_DAYS days (default 7).
func TestLoadAtTheSeriesCap(t *testing.T) {
	nodesEnv := os.Getenv("ORION_METRIC_LOAD_NODES")
	if nodesEnv == "" {
		t.Skip("set ORION_METRIC_LOAD_NODES to run the metrics.db load measurement")
	}
	nodes, _ := strconv.Atoi(nodesEnv)
	days := 7
	if v, err := strconv.Atoi(os.Getenv("ORION_METRIC_LOAD_DAYS")); err == nil && v > 0 {
		days = v
	}
	s := openTestStore(t)
	end := time.Now().UTC().Truncate(time.Hour).Add(30 * time.Minute)
	start := end.Add(-time.Duration(days) * 24 * time.Hour)
	minutes := days * 24 * 60

	mk := func(node, minute int) Batch {
		b := Batch{Source: SourceOS, NodeID: fmt.Sprintf("node-%d", node), At: start.Add(time.Duration(minute) * time.Minute)}
		for k := 0; k < DefaultSeriesCapPerNode; k++ {
			cpu := float64((minute+k)%100) + 0.5
			rx, tx := float64(minute%1000)*10, float64(k)*100
			mem, pids := int64(1<<28+k*4096+minute), int64(10+k%5)
			b.Samples = append(b.Samples, Sample{Key: fmt.Sprintf("container:%03d", k), Kind: "container", Name: fmt.Sprintf("service-%03d", k),
				Values: Values{CPUPercent: &cpu, MemoryBytes: &mem, NetRxBps: &rx, NetTxBps: &tx, Pids: &pids}})
		}
		return b
	}

	t0 := time.Now()
	var perBatch []time.Duration
	for m := 0; m < minutes; m++ {
		for n := 0; n < nodes; n++ {
			b := mk(n, m)
			s.Enqueue(b)
			if m >= minutes-50 { // time the last 50 minutes one at a time: the steady-state cost of one report
				at := time.Now()
				s.Flush()
				perBatch = append(perBatch, time.Since(at))
			}
		}
		if m%50 == 0 {
			s.Flush()
		}
	}
	s.Flush()
	write := time.Since(t0)
	rows := int64(minutes) * int64(nodes) * DefaultSeriesCapPerNode
	st := s.Stats()
	sort.Slice(perBatch, func(i, j int) bool { return perBatch[i] < perBatch[j] })
	t.Logf("write: %d rows in %s (%.0f rows/s); dropped by cap %d, queue drops %d", rows, write.Round(time.Millisecond), float64(rows)/write.Seconds(), st.DroppedCap, st.DroppedFull)
	t.Logf("one 200-series report (writer, incl. flush): p50 %s, p95 %s", perBatch[len(perBatch)/2].Round(time.Microsecond), perBatch[len(perBatch)*95/100].Round(time.Microsecond))
	t.Logf("size after write: %.1f MiB (%.0f bytes/row)", float64(st.DatabaseBytes)/(1<<20), float64(st.DatabaseBytes)/float64(rows))

	t1 := time.Now()
	hours, err := s.Rollup(end)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("rollup: %d hours in %s", hours, time.Since(t1).Round(time.Millisecond))
	t.Logf("size after rollup: %.1f MiB", float64(s.Stats().DatabaseBytes)/(1<<20))
	// The steady state: the next hourly run re-reads the last hour and rolls the new one.
	tSteady := time.Now()
	if _, err := s.Rollup(end.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	t.Logf("steady-state hourly rollup pass: %s", time.Since(tSteady).Round(time.Millisecond))

	ref := func(n, k int) SeriesRef {
		return SeriesRef{Source: SourceOS, NodeID: fmt.Sprintf("node-%d", n), Key: fmt.Sprintf("container:%03d", k)}
	}
	timeQuery := func(name string, q Query) {
		var d []time.Duration
		for i := 0; i < 20; i++ {
			at := time.Now()
			res, err := s.Query(q)
			if err != nil {
				t.Fatal(err)
			}
			d = append(d, time.Since(at))
			if i == 0 {
				pts := 0
				for _, sr := range res.Series {
					pts += len(sr.Points)
				}
				t.Logf("query %-34s resolution=%s step=%ds series=%d points=%d", name, res.Resolution, res.StepSeconds, len(res.Series), pts)
			}
		}
		sort.Slice(d, func(i, j int) bool { return d[i] < d[j] })
		t.Logf("query %-34s p50 %s, max %s", name, d[len(d)/2].Round(time.Microsecond), d[len(d)-1].Round(time.Microsecond))
	}
	timeQuery("1 series, 24h (raw)", Query{Series: []SeriesRef{ref(0, 7)}, From: end.Add(-24 * time.Hour), To: end})
	timeQuery("1 series, whole range (hourly)", Query{Series: []SeriesRef{ref(0, 7)}, From: start, To: end})
	var twenty []SeriesRef
	for k := 0; k < 20; k++ {
		twenty = append(twenty, ref(0, k))
	}
	timeQuery("20 series, 6h (raw)", Query{Series: twenty, From: end.Add(-6 * time.Hour), To: end})
	timeQuery("20 series, whole range (hourly)", Query{Series: twenty, From: start, To: end})

	t2 := time.Now()
	pr, err := s.Prune(Retention{Raw: time.Duration(days) * 12 * time.Hour, Rollup: 90 * 24 * time.Hour}, end)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("prune (raw kept %.1f days): removed %d raw rows in %s", float64(days)/2, pr.RawRows, time.Since(t2).Round(time.Millisecond))
	t3 := time.Now()
	pr, _ = s.Prune(Retention{Raw: time.Duration(days) * 12 * time.Hour, Rollup: 90 * 24 * time.Hour}, end)
	t.Logf("prune with nothing to remove: %s (series %d)", time.Since(t3).Round(time.Millisecond), nodes*DefaultSeriesCapPerNode)
	t.Logf("size after prune: %.1f MiB", float64(s.Stats().DatabaseBytes)/(1<<20))
}
