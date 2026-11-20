package metricstore

import (
	"math"
	"math/rand"
	"sort"
	"strings"
	"testing"
	"time"

	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
)

// rangeAt is a fixed instant: these tests never read the clock.
var rangeAt = time.Date(2026, 3, 10, 12, 0, 30, 0, time.UTC)

func nonZero(n int) []uint64 {
	out := make([]uint64, n)
	for i := range out {
		out[i] = 1
	}
	return out
}

// checkConverted asserts the invariants of a converted point: ascending unique bounds, one more count
// than bounds, and counts that add up to the point's count.
func checkConverted(t *testing.T, label string, h Histogram) {
	t.Helper()
	if !sort.Float64sAreSorted(h.Bounds) {
		t.Errorf("%s: bounds not ascending: %v", label, h.Bounds)
	}
	for i := 1; i < len(h.Bounds); i++ {
		if h.Bounds[i] == h.Bounds[i-1] {
			t.Errorf("%s: duplicate bound %v", label, h.Bounds[i])
		}
	}
	if len(h.Bounds) == 0 {
		if len(h.Counts) != 0 {
			t.Errorf("%s: counts without bounds: %v", label, h.Counts)
		}
		return
	}
	if len(h.Counts) != len(h.Bounds)+1 {
		t.Errorf("%s: %d counts for %d bounds", label, len(h.Counts), len(h.Bounds))
	}
	var total int64
	for _, c := range h.Counts {
		if c < 0 {
			t.Errorf("%s: negative bucket count %d", label, c)
		}
		total += c
	}
	if total != h.Count {
		t.Errorf("%s: buckets add up to %d, count is %d", label, total, h.Count)
	}
}

func TestExponentialOffsetNearInt32LimitIsRejectedNotPanicked(t *testing.T) {
	// 200 non-zero buckets starting 100 below MaxInt32: the last index would not fit an int32. This
	// used to overflow the index arithmetic and panic in make().
	dp := expDP(rangeAt, 0, 200, 0, 2147483547, nonZero(200), 0, nil)
	if _, reason := exponentialHistogramValue(dp); !strings.Contains(reason, "bucket index is out of range") {
		t.Fatalf("reason = %q, want bucket index out of range", reason)
	}
	got := parseOne(t, rangeAt, expMetric("latency", deltaTemporality, expDP(rangeAt, 0, 1, 1, 0, nil, 0, nil), dp))
	if len(got.Points) != 1 || got.Rejected != 1 || !strings.Contains(strings.Join(got.Errors, ";"), "bucket index is out of range") {
		t.Fatalf("points %d rejected %d errors %v: the bad point is rejected on its own, the good one kept", len(got.Points), got.Rejected, got.Errors)
	}
	// The same through the negative side, and with a count that needs a downscale.
	neg := expDP(rangeAt, 0, 200, 0, 0, nil, 2147483600, nonZero(200))
	if _, reason := exponentialHistogramValue(neg); !strings.Contains(reason, "out of range") {
		t.Fatalf("negative side reason = %q", reason)
	}
}

func TestExponentialExtremeIndexesNeverPanicAndAreRejectedPerPoint(t *testing.T) {
	counts := func(n int) []uint64 { return nonZero(n) }
	for name, tc := range map[string]struct {
		dp     *metricspb.ExponentialHistogramDataPoint
		reason string // empty means the point converts
	}{
		"max offset one bucket":           {expDP(rangeAt, 0, 1, 0, math.MaxInt32, counts(1), 0, nil), "out of range"}, // 2^(2^31) is infinite
		"max offset two buckets":          {expDP(rangeAt, 0, 2, 0, math.MaxInt32, counts(2), 0, nil), "out of range"},
		"min offset":                      {expDP(rangeAt, 0, 1, 0, math.MinInt32, counts(1), 0, nil), ""}, // edges underflow to 0: still a valid point
		"min offset negative side":        {expDP(rangeAt, 0, 1, 0, 0, nil, math.MinInt32, counts(1)), ""},
		"offset near max at scale 20":     {expDP(rangeAt, 20, 100, 0, 2147483000, counts(100), 0, nil), "out of range"},
		"offset near max downscaled":      {expDP(rangeAt, 0, 300, 0, 2147483000, counts(300), 0, nil), "out of range"},
		"offset near min downscaled":      {expDP(rangeAt, 0, 300, 0, math.MinInt32, counts(300), 0, nil), ""},
		"both sides near the top":         {expDP(rangeAt, 5, 400, 0, 2147483547, counts(200), 2147483547, counts(200)), "out of range"},
		"scale -10 with huge index":       {expDP(rangeAt, -10, 1, 0, 2147000000, counts(1), 0, nil), "out of range"},
		"scale 20 with a modest index":    {expDP(rangeAt, 20, 1, 0, 1<<20, counts(1), 0, nil), ""},
		"scale 20 with an infinite index": {expDP(rangeAt, 20, 1, 0, 1<<30, counts(1), 0, nil), "out of range"},
		"negative buckets only":           {expDP(rangeAt, 3, 4, 0, 0, nil, -40, counts(4)), ""},
		"zero count only at extreme scale": {
			expDP(rangeAt, -10, 3, 3, 0, nil, 0, nil), "",
		},
	} {
		h, reason := exponentialHistogramValue(tc.dp)
		if tc.reason == "" {
			if reason != "" {
				t.Errorf("%s: rejected: %s", name, reason)
				continue
			}
			checkConverted(t, name, h)
			continue
		}
		if !strings.Contains(reason, tc.reason) {
			t.Errorf("%s: reason = %q, want it to mention %q", name, reason, tc.reason)
		}
		got := parseOne(t, rangeAt, expMetric("latency", deltaTemporality, expDP(rangeAt, 0, 1, 1, 0, nil, 0, nil), tc.dp))
		if len(got.Points) != 1 || got.Rejected != 1 {
			t.Errorf("%s: points %d rejected %d: want the good point kept and this one rejected", name, len(got.Points), got.Rejected)
		}
	}
}

func TestExponentialHugeCountsAreRejectedWithoutOverflow(t *testing.T) {
	for name, dp := range map[string]*metricspb.ExponentialHistogramDataPoint{
		"two max buckets":  expDP(rangeAt, 0, math.MaxInt64, 0, 0, []uint64{math.MaxInt64, math.MaxInt64}, 0, nil),
		"zero plus bucket": expDP(rangeAt, 0, math.MaxInt64, math.MaxInt64, 0, []uint64{1}, 0, nil),
		"pos plus neg":     expDP(rangeAt, 0, math.MaxInt64, 0, 0, []uint64{math.MaxInt64}, 0, []uint64{math.MaxInt64}),
		"count beyond":     expDP(rangeAt, 0, math.MaxUint64, 0, 0, []uint64{1}, 0, nil),
	} {
		if _, reason := exponentialHistogramValue(dp); reason == "" {
			t.Errorf("%s: converted, want a rejection", name)
		}
	}
	// A single bucket holding MaxInt64 is fine.
	h, reason := exponentialHistogramValue(expDP(rangeAt, 0, math.MaxInt64, 0, 0, []uint64{math.MaxInt64}, 0, nil))
	if reason != "" {
		t.Fatalf("one maximal bucket rejected: %s", reason)
	}
	checkConverted(t, "one maximal bucket", h)
}

func TestDownscaleExpDoesNotOverflowAtTheIndexLimits(t *testing.T) {
	top := downscaleExp(expBuckets{offset: math.MaxInt32 - 3, counts: []int64{1, 2, 3, 4}})
	if len(top.counts) != 3 && len(top.counts) != 2 {
		t.Fatalf("top = %+v", top)
	}
	var total int64
	for _, c := range top.counts {
		total += c
	}
	if total != 10 || top.offset != (math.MaxInt32-3)>>1 {
		t.Fatalf("top = %+v: counts must be kept and the offset halved", top)
	}
	bottom := downscaleExp(expBuckets{offset: math.MinInt32, counts: []int64{1, 2, 3}})
	if bottom.offset != math.MinInt32>>1 || len(bottom.counts) != 2 || bottom.counts[0] != 3 || bottom.counts[1] != 3 {
		t.Fatalf("bottom = %+v", bottom)
	}
	trimmed := trimExp(expBuckets{offset: math.MaxInt32 - 2, counts: []int64{0, 0, 5}})
	if trimmed.offset != math.MaxInt32 || len(trimmed.counts) != 1 {
		t.Fatalf("trimmed = %+v: the offset must not wrap", trimmed)
	}
}

// TestExponentialRandomExtremesNeverPanic throws seeded extreme offsets, scales and counts at the
// converter. The only claim is "no panic, and anything accepted is a well-formed histogram".
func TestExponentialRandomExtremesNeverPanic(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	offsets := []int32{math.MinInt32, math.MinInt32 + 1, -1 << 30, -5000, -1, 0, 1, 5000, 1 << 30, math.MaxInt32 - 300, math.MaxInt32 - 1, math.MaxInt32}
	scales := []int32{math.MinInt32, -11, -10, -1, 0, 1, 5, 20, 21, math.MaxInt32}
	pick := func(n int) []uint64 {
		out := make([]uint64, n)
		for i := range out {
			switch rng.Intn(4) {
			case 0:
				out[i] = 0
			case 1:
				out[i] = uint64(rng.Intn(5))
			case 2:
				out[i] = math.MaxInt64
			default:
				out[i] = math.MaxUint64
			}
		}
		return out
	}
	accepted := 0
	for i := 0; i < 4000; i++ {
		var pos, neg []uint64
		if rng.Intn(4) > 0 {
			pos = pick(rng.Intn(400))
		}
		if rng.Intn(3) == 0 {
			neg = pick(rng.Intn(400))
		}
		dp := expDP(rangeAt, scales[rng.Intn(len(scales))], 0, uint64(rng.Intn(3)), offsets[rng.Intn(len(offsets))], pos, offsets[rng.Intn(len(offsets))], neg)
		var sum int64
		okSum := true
		for _, c := range append(append([]uint64{dp.ZeroCount}, pos...), neg...) {
			if c > math.MaxInt64 || sum > math.MaxInt64-int64(c) {
				okSum = false
				break
			}
			sum += int64(c)
		}
		if okSum && rng.Intn(4) > 0 {
			dp.Count = uint64(sum) // usually consistent, so conversion goes deep
		} else {
			dp.Count = rng.Uint64()
		}
		h, reason := exponentialHistogramValue(dp)
		if reason != "" {
			continue
		}
		accepted++
		checkConverted(t, "random", h)
	}
	if accepted == 0 {
		t.Fatal("no random point converted: the table no longer exercises the success path")
	}
}

func FuzzExponentialHistogramValue(f *testing.F) {
	f.Add(int32(0), int32(0), int32(0), uint64(3), uint64(1), uint64(2), uint64(0))
	f.Add(int32(math.MaxInt32-3), int32(math.MinInt32), int32(20), uint64(3), uint64(1), uint64(1), uint64(1))
	f.Add(int32(math.MinInt32), int32(math.MaxInt32), int32(-10), uint64(math.MaxInt64), uint64(math.MaxInt64), uint64(0), uint64(0))
	f.Fuzz(func(t *testing.T, posOffset, negOffset, scale int32, a, b, c, zero uint64) {
		pos := []uint64{a, b, c, a}
		neg := []uint64{c, b}
		dp := expDP(rangeAt, scale, a+b+c+a+c+b+zero, zero, posOffset, pos, negOffset, neg)
		if h, reason := exponentialHistogramValue(dp); reason == "" {
			checkConverted(t, "fuzz", h)
		}
	})
}
