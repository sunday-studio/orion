package metricstore

import (
	"math"
	"strings"
	"testing"
	"time"

	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
)

func expDP(at time.Time, scale int32, count, zero uint64, posOffset int32, pos []uint64, negOffset int32, neg []uint64) *metricspb.ExponentialHistogramDataPoint {
	dp := &metricspb.ExponentialHistogramDataPoint{TimeUnixNano: uint64(at.UnixNano()), Scale: scale, Count: count, ZeroCount: zero}
	if pos != nil {
		dp.Positive = &metricspb.ExponentialHistogramDataPoint_Buckets{Offset: posOffset, BucketCounts: pos}
	}
	if neg != nil {
		dp.Negative = &metricspb.ExponentialHistogramDataPoint_Buckets{Offset: negOffset, BucketCounts: neg}
	}
	return dp
}

func expMetric(name string, temporality metricspb.AggregationTemporality, dps ...*metricspb.ExponentialHistogramDataPoint) *metricspb.Metric {
	return &metricspb.Metric{Name: name, Unit: "ms", Data: &metricspb.Metric_ExponentialHistogram{
		ExponentialHistogram: &metricspb.ExponentialHistogram{AggregationTemporality: temporality, DataPoints: dps}}}
}

func TestExponentialHistogramConvertsToExplicitBuckets(t *testing.T) {
	now := time.Now().UTC()
	for name, tc := range map[string]struct {
		dp         *metricspb.ExponentialHistogramDataPoint
		wantBounds []float64
		wantCounts []int64
	}{
		// Scale 0 has base 2: index 0 is (1,2], 1 is (2,4], 2 is (4,8].
		"positive buckets": {expDP(now, 0, 7, 0, 0, []uint64{1, 2, 4}, 0, nil), []float64{1, 2, 4, 8}, []int64{0, 1, 2, 4, 0}},
		// Index 0 on the negative side is [-2,-1); the zero bucket is a point at 0; offset 1 is (2,4].
		"negative zero and positive": {expDP(now, 0, 10, 2, 1, []uint64{5}, 0, []uint64{3}), []float64{-2, -1, 0, 2, 4}, []int64{0, 3, 2, 0, 5, 0}},
		// Scale 1 has base sqrt(2): index 2 is (2, 2.828..].
		"finer scale": {expDP(now, 1, 3, 0, 2, []uint64{3}, 0, nil), []float64{2, math.Exp2(1.5)}, []int64{0, 3, 0}},
		// Scale -1 has base 4: index 0 is (1,4], index 1 is (4,16].
		"coarser scale": {expDP(now, -1, 3, 0, 0, []uint64{1, 2}, 0, nil), []float64{1, 4, 16}, []int64{0, 1, 2, 0}},
		// Interior empty buckets stay (the neighbours need their edges); empty ones at the ends are trimmed.
		"trimmed ends and a gap": {expDP(now, 0, 3, 0, -1, []uint64{0, 1, 0, 2, 0}, 0, nil), []float64{1, 2, 4, 8}, []int64{0, 1, 0, 2, 0}},
		"zero bucket only":       {expDP(now, 3, 5, 5, 0, nil, 0, nil), []float64{0}, []int64{5, 0}},
		"nothing observed":       {expDP(now, 3, 0, 0, 0, nil, 0, nil), nil, nil},
	} {
		got := parseOne(t, now, expMetric("latency", deltaTemporality, tc.dp))
		if got.Rejected != 0 || len(got.Points) != 1 {
			t.Errorf("%s: parsed = %+v", name, got)
			continue
		}
		h := got.Points[0].Sample.Values.Histogram
		if len(h.Bounds) != len(tc.wantBounds) || len(h.Counts) != len(tc.wantCounts) {
			t.Errorf("%s: bounds %v counts %v, want %v %v", name, h.Bounds, h.Counts, tc.wantBounds, tc.wantCounts)
			continue
		}
		for i := range tc.wantBounds {
			if !near(h.Bounds[i], tc.wantBounds[i]) {
				t.Errorf("%s: bounds = %v, want %v", name, h.Bounds, tc.wantBounds)
				break
			}
		}
		for i := range tc.wantCounts {
			if h.Counts[i] != tc.wantCounts[i] {
				t.Errorf("%s: counts = %v, want %v", name, h.Counts, tc.wantCounts)
				break
			}
		}
		if h.Count != int64(tc.dp.Count) || (len(h.Counts) > 0 && totalOf(h.Counts) != h.Count) {
			t.Errorf("%s: count %d does not match counts %v", name, h.Count, h.Counts)
		}
	}
}

func TestExponentialHistogramKeepsSumMinMaxAndYieldsUsablePercentiles(t *testing.T) {
	now := time.Now().UTC()
	dp := expDP(now, 0, 7, 0, 0, []uint64{1, 2, 4}, 0, nil)
	dp.Sum, dp.Min, dp.Max = fp(40), fp(1.5), fp(7.9)
	got := parseOne(t, now, expMetric("latency", deltaTemporality, dp))
	h := *got.Points[0].Sample.Values.Histogram
	if *h.Sum != 40 || *h.Min != 1.5 || *h.Max != 7.9 {
		t.Fatalf("moments = %+v", h)
	}
	// 4 of 7 observations are in (4,8]: the median falls there.
	if p50, ok := h.Percentile(0.5); !ok || p50 <= 4 || p50 > 7.9 {
		t.Fatalf("p50 = %v, %v, want inside (4, 7.9]", p50, ok)
	}
}

func TestExponentialHistogramIsCappedByDownscaling(t *testing.T) {
	now := time.Now().UTC()
	counts := make([]uint64, 400)
	for i := range counts {
		counts[i] = 1
	}
	got := parseOne(t, now, expMetric("latency", deltaTemporality, expDP(now, 5, 400, 0, 0, counts, 0, nil)))
	if got.Rejected != 0 {
		t.Fatalf("parsed = %+v", got)
	}
	h := got.Points[0].Sample.Values.Histogram
	// 400 buckets halve to 200 and to 100 (scale 3): 101 bounds, none lost, none repeated.
	if len(h.Counts)-1 > MaxStoredBuckets+1 || len(h.Bounds) != 101 || h.Count != 400 || totalOf(h.Counts) != 400 {
		t.Fatalf("bounds %d counts total %d", len(h.Bounds), totalOf(h.Counts))
	}
	if !near(h.Bounds[0], 1) || !near(h.Bounds[1], math.Exp2(0.125)) {
		t.Fatalf("bounds start %v, want the edges of scale 3", h.Bounds[:2])
	}

	// A point that already fits is not touched.
	fits := parseOne(t, now, expMetric("latency", deltaTemporality, expDP(now, 5, 160, 0, 0, counts[:160], 0, nil)))
	if got := len(fits.Points[0].Sample.Values.Histogram.Bounds); got != 161 {
		t.Fatalf("a point of exactly %d buckets keeps them all, got %d bounds", MaxStoredBuckets, got)
	}
	// The cap counts both sides.
	both := parseOne(t, now, expMetric("latency", deltaTemporality, expDP(now, 5, 200, 0, 0, counts[:100], 0, counts[:100])))
	if h := both.Points[0].Sample.Values.Histogram; len(h.Bounds) > 2*(MaxStoredBuckets/2+1)+1 || h.Count != 200 || totalOf(h.Counts) != 200 {
		t.Fatalf("both sides: %d bounds, total %d", len(h.Bounds), totalOf(h.Counts))
	}
}

func TestDownscaleMergesNeighbouringIndexesWithNegativeOffsets(t *testing.T) {
	got := downscaleExp(expBuckets{offset: -3, counts: []int64{1, 2, 3, 4, 5}}) // indexes -3..1 become -2,-1,-1,0,0
	if got.offset != -2 || len(got.counts) != 3 || got.counts[0] != 1 || got.counts[1] != 5 || got.counts[2] != 9 {
		t.Fatalf("downscaled = %+v", got)
	}
	if got := downscaleExp(expBuckets{}); len(got.counts) != 0 {
		t.Fatalf("empty = %+v", got)
	}
}

func TestExponentialHistogramRejectsWhatItCannotConvert(t *testing.T) {
	now := time.Now().UTC()
	hugeIndex := expDP(now, 0, 1, 0, 5000, []uint64{1}, 0, nil)
	badThreshold := expDP(now, 0, 1, 1, 0, nil, 0, nil)
	badThreshold.ZeroThreshold = -1
	nanThreshold := expDP(now, 0, 1, 1, 0, nil, 0, nil)
	nanThreshold.ZeroThreshold = math.NaN()
	infSum := expDP(now, 0, 1, 1, 0, nil, 0, nil)
	infSum.Sum = fp(math.Inf(1))
	minAboveMax := expDP(now, 0, 1, 1, 0, nil, 0, nil)
	minAboveMax.Min, minAboveMax.Max = fp(3), fp(1)
	noValue := expDP(now, 0, 1, 1, 0, nil, 0, nil)
	noValue.Flags = 1
	tooWide := expDP(now, -10, 1, 0, 0, make([]uint64, 2*MaxStoredBuckets), 0, nil)
	tooWide.Positive.BucketCounts[0], tooWide.Positive.BucketCounts[len(tooWide.Positive.BucketCounts)-1] = 1, 1
	tooWide.Count = 2
	for name, tc := range map[string]struct {
		dp     *metricspb.ExponentialHistogramDataPoint
		reason string
	}{
		"scale too low":         {expDP(now, -11, 1, 1, 0, nil, 0, nil), "scale must be between"},
		"scale too high":        {expDP(now, 21, 1, 1, 0, nil, 0, nil), "scale must be between"},
		"count above buckets":   {expDP(now, 0, 9, 1, 0, []uint64{1}, 0, nil), "do not add up"},
		"count below buckets":   {expDP(now, 0, 1, 1, 0, []uint64{1}, 0, []uint64{1}), "do not add up"},
		"bucket beyond int64":   {expDP(now, 0, 1, 0, 0, []uint64{math.MaxUint64}, 0, nil), "bucket count is too large"},
		"zero count beyond":     {expDP(now, 0, 1, math.MaxUint64, 0, nil, 0, nil), "count is too large"},
		"negative threshold":    {badThreshold, "zero threshold"},
		"NaN threshold":         {nanThreshold, "zero threshold"},
		"infinite sum":          {infSum, "not a finite"},
		"min above max":         {minAboveMax, "min is greater"},
		"no recorded value":     {noValue, "no recorded value"},
		"index out of range":    {hugeIndex, "out of range"},
		"too many at min scale": {tooWide, "too many buckets"},
	} {
		got := parseOne(t, now,
			expMetric("latency", deltaTemporality, expDP(now, 0, 1, 1, 0, nil, 0, nil), tc.dp))
		if len(got.Points) != 1 || got.Rejected != 1 || !strings.Contains(strings.Join(got.Errors, ";"), tc.reason) {
			t.Errorf("%s: points %d rejected %d errors %v, want the good point kept and one rejection mentioning %q", name, len(got.Points), got.Rejected, got.Errors, tc.reason)
		}
	}
}
