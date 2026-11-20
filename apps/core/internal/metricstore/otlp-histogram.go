package metricstore

import (
	"math"

	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
)

func (p *parser) explicitHistogram(name, unit string, resource map[string]string, cumulative bool, dp *metricspb.HistogramDataPoint) {
	if dp.GetFlags()&uint32(metricspb.DataPointFlags_DATA_POINT_FLAGS_NO_RECORDED_VALUE_MASK) != 0 {
		p.rejectPoint(name, "data point has no recorded value")
		return
	}
	h, reason := explicitHistogramValue(dp)
	if reason != "" {
		p.rejectPoint(name, reason)
		return
	}
	p.distribution(name, KindHistogram, unit, resource, dp.GetAttributes(), dp.GetTimeUnixNano(), dp.GetStartTimeUnixNano(), cumulative, h)
}

// explicitHistogramValue checks an explicit-bucket data point and returns it as stored. Bounds must be
// finite and strictly ascending, there must be one more bucket count than bounds (none at all is allowed
// for a histogram that only reports count and sum), the counts must add up to the count, and sum, min and
// max must be finite numbers.
func explicitHistogramValue(dp *metricspb.HistogramDataPoint) (Histogram, string) {
	bounds, counts := dp.GetExplicitBounds(), dp.GetBucketCounts()
	if len(bounds) > MaxExplicitBounds {
		return Histogram{}, "histogram has more than 512 bucket bounds"
	}
	for i, b := range bounds {
		if !finite(b) {
			return Histogram{}, "histogram bucket bound is not a finite number"
		}
		if i > 0 && b <= bounds[i-1] {
			return Histogram{}, "histogram bucket bounds must be strictly increasing"
		}
	}
	if dp.GetCount() > math.MaxInt64 {
		return Histogram{}, "histogram count is too large"
	}
	h := Histogram{Count: int64(dp.GetCount())}
	if len(counts) > 0 || len(bounds) > 0 {
		if len(counts) != len(bounds)+1 {
			return Histogram{}, "histogram needs one more bucket count than bucket bounds"
		}
		total, ok := sumCounts(counts)
		if !ok {
			return Histogram{}, "histogram bucket count is too large"
		}
		if total != h.Count {
			return Histogram{}, "histogram bucket counts do not add up to its count"
		}
		h.Bounds = append([]float64(nil), bounds...)
		h.Counts = make([]int64, len(counts))
		for i, c := range counts {
			h.Counts[i] = int64(c)
		}
	}
	if reason := fillMoments(&h, dp.Sum, dp.Min, dp.Max); reason != "" {
		return Histogram{}, reason
	}
	return h, ""
}

// sumCounts adds bucket counts, failing when one or the total does not fit an int64.
func sumCounts(counts []uint64) (int64, bool) {
	var total uint64
	for _, c := range counts {
		if c > math.MaxInt64 || total > math.MaxInt64-c {
			return 0, false
		}
		total += c
	}
	return int64(total), true
}

// fillMoments copies the optional sum, min and max of a data point after checking them.
func fillMoments(h *Histogram, sum, lowest, highest *float64) string {
	for _, v := range []*float64{sum, lowest, highest} {
		if v != nil && !finite(*v) {
			return "histogram sum, min or max is not a finite number"
		}
	}
	if lowest != nil && highest != nil && *lowest > *highest {
		return "histogram min is greater than its max"
	}
	h.Sum, h.Min, h.Max = cp(sum), cp(lowest), cp(highest)
	if h.Count == 0 {
		h.Min, h.Max = nil, nil
	}
	return ""
}
