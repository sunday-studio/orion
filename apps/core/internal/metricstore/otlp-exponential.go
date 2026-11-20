package metricstore

import (
	"math"
	"sort"

	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
)

const (
	minExponentialScale = -10
	maxExponentialScale = 20
)

func (p *parser) exponentialHistogram(name, unit string, resource map[string]string, cumulative bool, dp *metricspb.ExponentialHistogramDataPoint) {
	if dp.GetFlags()&uint32(metricspb.DataPointFlags_DATA_POINT_FLAGS_NO_RECORDED_VALUE_MASK) != 0 {
		p.rejectPoint(name, "data point has no recorded value")
		return
	}
	h, reason := exponentialHistogramValue(dp)
	if reason != "" {
		p.rejectPoint(name, reason)
		return
	}
	p.distribution(name, KindHistogram, unit, resource, dp.GetAttributes(), dp.GetTimeUnixNano(), dp.GetStartTimeUnixNano(), cumulative, h)
}

// expBuckets is one side (positive or negative) of an exponential histogram: counts[k] belongs to the
// bucket with index offset+k.
//
// The index is an int64 here although OTLP sends an int32: every sum and shift on it (trim, downscale,
// edge lookup) then stays exact, and expSide rejects a side whose last index would leave int32.
type expBuckets struct {
	offset int64
	counts []int64
}

const errBucketIndex = "exponential histogram bucket index is out of range"

// exponentialHistogramValue converts an exponential data point to the stored explicit shape, so one
// reader handles both. Bucket index i of scale s covers (base^i, base^(i+1)] with base = 2^(2^-s); the
// negative side mirrors it. The zero bucket covers [-zero_threshold, zero_threshold]. The counts must
// add up to the count. A point with more than MaxStoredBuckets buckets is downscaled (adjacent bucket
// pairs merge, which is what lowering the scale by one does) until it fits; counts never change, only
// resolution does. The bounds of two readings may differ when the scale does; Histogram.Merge and the
// cumulative conversion handle that.
func exponentialHistogramValue(dp *metricspb.ExponentialHistogramDataPoint) (Histogram, string) {
	scale := dp.GetScale()
	if scale < minExponentialScale || scale > maxExponentialScale {
		return Histogram{}, "exponential histogram scale must be between -10 and 20"
	}
	if dp.GetCount() > math.MaxInt64 || dp.GetZeroCount() > math.MaxInt64 {
		return Histogram{}, "histogram count is too large"
	}
	zeroThreshold := dp.GetZeroThreshold()
	if !finite(zeroThreshold) || zeroThreshold < 0 {
		return Histogram{}, "exponential histogram zero threshold must be a finite number of at least 0"
	}
	pos, reason := expSide(dp.GetPositive().GetOffset(), dp.GetPositive().GetBucketCounts())
	if reason != "" {
		return Histogram{}, reason
	}
	neg, reason := expSide(dp.GetNegative().GetOffset(), dp.GetNegative().GetBucketCounts())
	if reason != "" {
		return Histogram{}, reason
	}
	total := int64(dp.GetZeroCount())
	for _, c := range append(append([]int64(nil), pos.counts...), neg.counts...) {
		if c > math.MaxInt64-total {
			return Histogram{}, "histogram bucket count is too large"
		}
		total += c
	}
	if total != int64(dp.GetCount()) {
		return Histogram{}, "exponential histogram bucket counts do not add up to its count"
	}
	h := Histogram{Count: total}
	if reason := fillMoments(&h, dp.Sum, dp.Min, dp.Max); reason != "" {
		return Histogram{}, reason
	}
	pos, neg = trimExp(pos), trimExp(neg)
	for len(pos.counts)+len(neg.counts) > MaxStoredBuckets && scale > minExponentialScale {
		pos, neg, scale = downscaleExp(pos), downscaleExp(neg), scale-1
	}
	if len(pos.counts)+len(neg.counts) > MaxStoredBuckets {
		return Histogram{}, "exponential histogram has too many buckets"
	}
	bounds, counts, ok := expToExplicit(scale, zeroThreshold, int64(dp.GetZeroCount()), pos, neg)
	if !ok {
		return Histogram{}, errBucketIndex
	}
	h.Bounds, h.Counts = bounds, counts
	if len(bounds) == 0 {
		h.Counts = nil
	}
	return h, ""
}

func expSide(offset int32, counts []uint64) (expBuckets, string) {
	if len(counts) > 0 && int64(offset)+int64(len(counts))-1 > math.MaxInt32 {
		return expBuckets{}, errBucketIndex
	}
	out := expBuckets{offset: int64(offset), counts: make([]int64, len(counts))}
	for i, c := range counts {
		if c > math.MaxInt64 {
			return expBuckets{}, "histogram bucket count is too large"
		}
		out.counts[i] = int64(c)
	}
	return out, ""
}

// trimExp drops empty buckets at both ends, which exporters leave in.
func trimExp(b expBuckets) expBuckets {
	lo, hi := 0, len(b.counts)
	for lo < hi && b.counts[lo] == 0 {
		lo++
	}
	for hi > lo && b.counts[hi-1] == 0 {
		hi--
	}
	return expBuckets{offset: b.offset + int64(lo), counts: b.counts[lo:hi]}
}

// downscaleExp merges the buckets two by two: index i of the old scale becomes i>>1 of the new one.
func downscaleExp(b expBuckets) expBuckets {
	if len(b.counts) == 0 {
		return b
	}
	first := b.offset >> 1
	last := (b.offset + int64(len(b.counts)) - 1) >> 1
	out := expBuckets{offset: first, counts: make([]int64, last-first+1)}
	for k, c := range b.counts {
		out.counts[((b.offset+int64(k))>>1)-first] += c
	}
	return out
}

// expEdge is the boundary base^index of an exponential histogram of the given scale.
func expEdge(index int64, scale int32) float64 {
	return math.Exp2(math.Ldexp(float64(index), -int(scale)))
}

type expInterval struct {
	lo, hi float64
	count  int64
}

// expToExplicit lists the intervals of the three parts in ascending order and turns them into bounds and
// counts: every interval's count lands in the bucket that ends at its upper edge.
func expToExplicit(scale int32, zeroThreshold float64, zeroCount int64, pos, neg expBuckets) ([]float64, []int64, bool) {
	var intervals []expInterval
	for k := len(neg.counts) - 1; k >= 0; k-- { // the most negative bucket has the highest index
		idx := neg.offset + int64(k)
		intervals = append(intervals, expInterval{lo: -expEdge(idx+1, scale), hi: -expEdge(idx, scale), count: neg.counts[k]})
	}
	if zeroCount > 0 {
		intervals = append(intervals, expInterval{lo: -zeroThreshold, hi: zeroThreshold, count: zeroCount})
	}
	for k, c := range pos.counts {
		idx := pos.offset + int64(k)
		intervals = append(intervals, expInterval{lo: expEdge(idx, scale), hi: expEdge(idx+1, scale), count: c})
	}
	if len(intervals) == 0 {
		return nil, nil, true
	}
	edges := make([]float64, 0, 2*len(intervals))
	for _, iv := range intervals {
		if !finite(iv.lo) || !finite(iv.hi) || iv.lo > iv.hi {
			return nil, nil, false
		}
		edges = append(edges, iv.lo, iv.hi)
	}
	sort.Float64s(edges)
	bounds := edges[:0:0]
	for i, e := range edges {
		if i == 0 || e != edges[i-1] {
			bounds = append(bounds, e)
		}
	}
	if len(bounds) > MaxExplicitBounds {
		return nil, nil, false
	}
	counts := make([]int64, len(bounds)+1)
	for _, iv := range intervals {
		counts[sort.SearchFloat64s(bounds, iv.hi)] += iv.count
	}
	return bounds, counts, true
}
