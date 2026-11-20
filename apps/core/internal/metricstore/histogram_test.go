package metricstore

import (
	"math"
	"testing"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func totalOf(counts []int64) int64 {
	var n int64
	for _, c := range counts {
		n += c
	}
	return n
}

func TestPercentileOfAKnownDistribution(t *testing.T) {
	// 100 observations spread evenly over 1..100 in buckets of width 10: the interpolated percentiles
	// are the true ones.
	uniform := Histogram{Count: 100, Bounds: []float64{10, 20, 30, 40, 50, 60, 70, 80, 90}, Counts: []int64{10, 10, 10, 10, 10, 10, 10, 10, 10, 10},
		Min: fp(1), Max: fp(100)}
	skewed := Histogram{Count: 100, Bounds: []float64{10, 20, 50, 100}, Counts: []int64{10, 40, 30, 15, 5}, Min: fp(1), Max: fp(500)}
	noMin := Histogram{Count: 100, Bounds: []float64{10, 20, 50, 100}, Counts: []int64{10, 40, 30, 15, 5}}
	for name, tc := range map[string]struct {
		h    Histogram
		q    float64
		want float64
		ok   bool
	}{
		"uniform p50":                    {uniform, 0.5, 50, true},
		"uniform p95":                    {uniform, 0.95, 95, true},
		"uniform p99":                    {uniform, 0.99, 99, true},
		"uniform p100 is the max":        {uniform, 1, 100, true},
		"skewed p5 starts at the min":    {skewed, 0.05, 5.5, true},
		"skewed p25 inside a bucket":     {skewed, 0.25, 13.75, true},
		"skewed p50 on a bucket edge":    {skewed, 0.5, 20, true},
		"skewed p95 on a bucket edge":    {skewed, 0.95, 100, true},
		"skewed p99 reaches up to max":   {skewed, 0.99, 420, true},
		"no min starts the bucket at 0":  {noMin, 0.05, 5, true},
		"no max stops at the last bound": {noMin, 0.99, 100, true},
		"no observations":                {Histogram{Count: 0, Bounds: []float64{1}, Counts: []int64{0, 0}}, 0.5, 0, false},
		"no buckets":                     {Histogram{Count: 5, Sum: fp(10)}, 0.5, 0, false},
		"bad quantile":                   {uniform, 0, 0, false},
		"counts of the wrong length":     {Histogram{Count: 1, Bounds: []float64{1, 2}, Counts: []int64{1}}, 0.5, 0, false},
	} {
		got, ok := tc.h.Percentile(tc.q)
		if ok != tc.ok || (ok && !near(got, tc.want)) {
			t.Errorf("%s: got %v, %v want %v, %v", name, got, ok, tc.want, tc.ok)
		}
	}
}

func TestPercentileIsClampedToTheObservedRange(t *testing.T) {
	// Every observation was 7: the bucket is (0, 10] but no value can be outside min..max.
	h := Histogram{Count: 4, Bounds: []float64{10}, Counts: []int64{4, 0}, Min: fp(7), Max: fp(7)}
	for _, q := range []float64{0.5, 0.95, 0.99} {
		if got, ok := h.Percentile(q); !ok || got != 7 {
			t.Errorf("p%v = %v, %v, want 7", q, got, ok)
		}
	}
}

func TestMergeWithEqualBoundsAddsBuckets(t *testing.T) {
	a := Histogram{Count: 3, Sum: fp(9), Min: fp(1), Max: fp(5), Bounds: []float64{2, 4}, Counts: []int64{1, 1, 1}}
	b := Histogram{Count: 2, Sum: fp(11), Min: fp(0.5), Max: fp(9), Bounds: []float64{2, 4}, Counts: []int64{0, 1, 1}}
	a.Merge(b)
	if a.Count != 5 || *a.Sum != 20 || *a.Min != 0.5 || *a.Max != 9 || totalOf(a.Counts) != 5 || a.Counts[0] != 1 || a.Counts[1] != 2 || a.Counts[2] != 2 {
		t.Fatalf("merged = %+v", a)
	}
	// A side without a sum makes the merged sum unknown rather than wrong.
	c := Histogram{Count: 1, Bounds: []float64{2, 4}, Counts: []int64{1, 0, 0}}
	a.Merge(c)
	if a.Sum != nil || a.Count != 6 {
		t.Fatalf("sum after merging a point without one = %+v", a)
	}
	// Merging into or with an empty histogram changes nothing and loses nothing.
	var empty Histogram
	empty.Merge(b)
	if empty.Count != 2 || len(empty.Counts) != 3 {
		t.Fatalf("empty.Merge = %+v", empty)
	}
	empty.Merge(Histogram{})
	if empty.Count != 2 {
		t.Fatalf("merge of nothing = %+v", empty)
	}
}

func TestMergeWithDifferentBoundsKeepsEveryCount(t *testing.T) {
	a := Histogram{Count: 10, Bounds: []float64{10, 20}, Counts: []int64{4, 6, 0}, Min: fp(0), Max: fp(20)}
	b := Histogram{Count: 7, Bounds: []float64{5, 10, 30}, Counts: []int64{1, 2, 3, 1}, Min: fp(1), Max: fp(40)}
	a.Merge(b)
	if a.Count != 17 || totalOf(a.Counts) != 17 || len(a.Counts) != len(a.Bounds)+1 {
		t.Fatalf("merged = %+v", a)
	}
	for i := 1; i < len(a.Bounds); i++ {
		if a.Bounds[i] <= a.Bounds[i-1] {
			t.Fatalf("bounds not ascending: %v", a.Bounds)
		}
	}
	// a's bucket (0,10] had 4: 2 of them in (0,5], 2 in (5,10] under a uniform spread; b added 1 and 2.
	if a.Bounds[0] != 5 || a.Counts[0] != 3 {
		t.Fatalf("first bucket = bound %v count %d, want bound 5 holding 2 of a and 1 of b", a.Bounds[0], a.Counts[0])
	}
}

func TestMergeThinsAWideUnionAndStillConservesCounts(t *testing.T) {
	var ab, bb []float64
	var ac, bc []int64
	for i := 0; i < 300; i++ {
		ab, bb = append(ab, float64(2*i+1)), append(bb, float64(2*i+2))
	}
	for i := 0; i <= 300; i++ {
		ac, bc = append(ac, 1), append(bc, 2)
	}
	a := Histogram{Count: 301, Bounds: ab, Counts: ac}
	a.Merge(Histogram{Count: 602, Bounds: bb, Counts: bc})
	if len(a.Bounds) > maxMergedBounds || totalOf(a.Counts) != 903 || a.Count != 903 {
		t.Fatalf("bounds %d total %d", len(a.Bounds), totalOf(a.Counts))
	}
}

func TestRebucketPointMassesAndOpenEnds(t *testing.T) {
	// A histogram of count and sum only is placed at its mean.
	got := rebucket(Histogram{Count: 4, Sum: fp(60)}, []float64{10, 20, 30})
	if got[1] != 4 || totalOf(got) != 4 {
		t.Fatalf("mean 15 should land in (10,20]: %v", got)
	}
	if got := rebucket(Histogram{Count: 0}, []float64{1}); totalOf(got) != 0 {
		t.Fatalf("empty = %v", got)
	}
}

func TestQuantileMeanWeighsRowsByTheirCount(t *testing.T) {
	m := newQuantileMean()
	m.add(10, []Quantile{{0.5, 100}})
	m.add(30, []Quantile{{0.5, 200}, {0.99, 900}})
	got := m.result()
	if len(got) != 2 || !near(got[0].V, 175) || got[1].Q != 0.99 || got[1].V != 900 {
		t.Fatalf("quantiles = %+v", got)
	}
	idle := newQuantileMean()
	idle.add(0, []Quantile{{0.5, 4}})
	idle.add(0, []Quantile{{0.5, 8}})
	if got := idle.result(); !near(got[0].V, 6) {
		t.Fatalf("rows without observations average evenly: %+v", got)
	}
}
