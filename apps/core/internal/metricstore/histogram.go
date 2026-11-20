package metricstore

import (
	"encoding/json"
	"math"
	"sort"
)

const (
	// MaxExplicitBounds is how many bucket bounds an explicit-bucket histogram data point may carry;
	// more is rejected. Typical exporters send 10 to 30.
	MaxExplicitBounds = 512
	// MaxStoredBuckets caps the buckets of a converted exponential histogram data point (negative and
	// positive together). Above it the point is downscaled, which halves its buckets and loses
	// resolution, never counts.
	MaxStoredBuckets = 160
	// maxMergedBounds caps the bounds of a histogram that is merged from rows with different bounds.
	maxMergedBounds = 2 * MaxStoredBuckets
	// MaxSummaryQuantiles bounds the quantiles one summary data point may report.
	MaxSummaryQuantiles = 20
)

// Quantile is one quantile of a summary: Q is in [0, 1].
type Quantile struct {
	Q float64 `json:"q"`
	V float64 `json:"v"`
}

// Histogram is the stored shape of a distribution, whatever its source. Counts[i] is the number of
// observations in (Bounds[i-1], Bounds[i]]; the first bucket reaches down to -Inf and the last up to
// +Inf, so Counts has one more entry than Bounds. A histogram without buckets (an exporter that sends
// only count and sum) has empty Bounds and Counts. A summary has Quantiles and no buckets. A field the
// exporter did not send is nil, never zero.
type Histogram struct {
	Count     int64
	Sum       *float64
	Min       *float64
	Max       *float64
	Bounds    []float64
	Counts    []int64
	Quantiles []Quantile
}

func (h Histogram) clone() Histogram {
	out := h
	out.Sum, out.Min, out.Max = cp(h.Sum), cp(h.Min), cp(h.Max)
	out.Bounds = append([]float64(nil), h.Bounds...)
	out.Counts = append([]int64(nil), h.Counts...)
	out.Quantiles = append([]Quantile(nil), h.Quantiles...)
	return out
}

func cp(p *float64) *float64 {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

// mean is the average of the observations, or ok=false when it is not known.
func (h Histogram) mean() (float64, bool) {
	if h.Sum == nil || h.Count <= 0 {
		return 0, false
	}
	return *h.Sum / float64(h.Count), true
}

// extent returns the lowest and highest value the observations may have, for each bucket: lo[i] to
// hi[i]. The two open ends use Min and Max when known; without them they are as wide as nothing: the
// first bucket starts at its bound (or at 0 when that bound is positive, as latencies do), the last
// ends at its bound.
func (h Histogram) extent() (lo, hi []float64) {
	n := len(h.Bounds)
	lo, hi = make([]float64, n+1), make([]float64, n+1)
	for i := 0; i <= n; i++ {
		if i > 0 {
			lo[i] = h.Bounds[i-1]
		}
		if i < n {
			hi[i] = h.Bounds[i]
		}
	}
	mean, hasMean := h.mean()
	switch {
	case h.Min != nil && (n == 0 || *h.Min < h.Bounds[0]):
		lo[0] = *h.Min
	case n > 0 && h.Bounds[0] > 0:
		lo[0] = 0
	case n > 0:
		lo[0] = h.Bounds[0]
	case hasMean:
		lo[0] = mean
	}
	switch {
	case h.Max != nil && (n == 0 || *h.Max > h.Bounds[n-1]):
		hi[n] = *h.Max
	case n > 0:
		hi[n] = h.Bounds[n-1]
	case hasMean:
		hi[n] = mean
	}
	if hi[n] < lo[n] {
		hi[n] = lo[n]
	}
	return lo, hi
}

// Percentile returns the q-th quantile (0 < q <= 1) by linear interpolation inside the bucket that holds
// the rank. ok is false when the histogram has no observations or no buckets.
func (h Histogram) Percentile(q float64) (float64, bool) {
	var total int64
	for _, c := range h.Counts {
		total += c
	}
	if total <= 0 || len(h.Bounds) == 0 || len(h.Counts) != len(h.Bounds)+1 || q <= 0 || q > 1 {
		return 0, false
	}
	lo, hi := h.extent()
	rank := q * float64(total)
	var cum float64
	for i, c := range h.Counts {
		if c == 0 {
			continue
		}
		if cum+float64(c) >= rank {
			frac := (rank - cum) / float64(c)
			return h.clamp(lo[i] + (hi[i]-lo[i])*frac), true
		}
		cum += float64(c)
	}
	return h.clamp(hi[len(hi)-1]), true // only reachable through rounding at q = 1
}

func (h Histogram) clamp(v float64) float64 {
	if h.Min != nil && v < *h.Min {
		v = *h.Min
	}
	if h.Max != nil && v > *h.Max {
		v = *h.Max
	}
	return v
}

// Merge adds b to h. Equal bounds add bucket by bucket (exact). Different bounds, which happen when an
// exponential histogram changed its scale or an exporter was reconfigured, are brought to their union
// (thinned to maxMergedBounds) and each source bucket is split over the new buckets in proportion to
// the overlap, assuming a uniform spread inside a bucket. Sum is kept only when both sides have it.
func (h *Histogram) Merge(b Histogram) {
	if b.Count <= 0 {
		return
	}
	if h.Count <= 0 {
		q := h.Quantiles
		*h = b.clone()
		h.Quantiles = q
		return
	}
	switch {
	case equalFloats(h.Bounds, b.Bounds) && len(h.Counts) == len(b.Counts):
		for i := range h.Counts {
			h.Counts[i] += b.Counts[i]
		}
	default:
		bounds := unionBounds(h.Bounds, b.Bounds)
		counts := rebucket(*h, bounds)
		for i, c := range rebucket(b, bounds) {
			counts[i] += c
		}
		h.Bounds, h.Counts = bounds, counts
	}
	h.Count += b.Count
	if h.Sum != nil && b.Sum != nil {
		s := *h.Sum + *b.Sum
		h.Sum = &s
	} else {
		h.Sum = nil
	}
	h.Min, h.Max = minPtr(h.Min, b.Min), maxPtr(h.Max, b.Max)
}

func minPtr(a, b *float64) *float64 {
	switch {
	case a == nil:
		return cp(b)
	case b == nil || *a <= *b:
		return a
	}
	return cp(b)
}

func maxPtr(a, b *float64) *float64 {
	switch {
	case a == nil:
		return cp(b)
	case b == nil || *a >= *b:
		return a
	}
	return cp(b)
}

func equalFloats(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// unionBounds merges two ascending bound lists, thinning the result to maxMergedBounds by keeping every
// k-th bound (coarser buckets, same counts).
func unionBounds(a, b []float64) []float64 {
	out := make([]float64, 0, len(a)+len(b))
	i, j := 0, 0
	for i < len(a) || j < len(b) {
		var v float64
		switch {
		case j >= len(b) || (i < len(a) && a[i] < b[j]):
			v, i = a[i], i+1
		case i >= len(a) || b[j] < a[i]:
			v, j = b[j], j+1
		default:
			v, i, j = a[i], i+1, j+1
		}
		out = append(out, v)
	}
	if len(out) > maxMergedBounds {
		stride := (len(out) + maxMergedBounds - 1) / maxMergedBounds
		thin := make([]float64, 0, maxMergedBounds)
		for k := stride - 1; k < len(out); k += stride {
			thin = append(thin, out[k])
		}
		out = thin
	}
	return out
}

// rebucket distributes the counts of h over dst bounds. A source bucket is spread uniformly over its
// extent; the whole count of a bucket lands in the destination buckets it overlaps and rounding keeps
// each source bucket's total exact. A bucket of no width (or a histogram without bounds and without a
// min and max) is placed at its single point.
func rebucket(h Histogram, dst []float64) []int64 {
	out := make([]int64, len(dst)+1)
	counts := h.Counts
	if len(counts) == 0 {
		if h.Count <= 0 {
			return out
		}
		counts = []int64{h.Count}
	}
	src := h
	if len(h.Counts) == 0 {
		src.Bounds = nil
	}
	lo, hi := src.extent()
	for i, c := range counts {
		if c <= 0 || i >= len(lo) {
			continue
		}
		l, u := lo[i], hi[i]
		if u <= l {
			out[sort.SearchFloat64s(dst, u)] += c
			continue
		}
		var cumFrac float64
		var given int64
		first := sort.SearchFloat64s(dst, l) // the bucket that holds l
		for j := first; j <= len(dst); j++ {
			lower := math.Inf(-1)
			if j > 0 {
				lower = dst[j-1]
			}
			if lower >= u {
				break
			}
			upper := math.Inf(1)
			if j < len(dst) {
				upper = dst[j]
			}
			width := math.Min(u, upper) - math.Max(l, lower)
			if width <= 0 {
				continue
			}
			cumFrac += width / (u - l)
			target := int64(math.Round(cumFrac * float64(c)))
			if target > c {
				target = c
			}
			out[j] += target - given
			given = target
		}
		out[min(sort.SearchFloat64s(dst, u), len(dst))] += c - given // rounding left-overs go to the top bucket
	}
	return out
}

func encodeFloats(v []float64) string {
	if len(v) == 0 {
		return ""
	}
	b, _ := json.Marshal(v)
	return string(b)
}

func encodeInts(v []int64) string {
	if len(v) == 0 {
		return ""
	}
	b, _ := json.Marshal(v)
	return string(b)
}

func encodeQuantiles(v []Quantile) string {
	if len(v) == 0 {
		return ""
	}
	b, _ := json.Marshal(v)
	return string(b)
}

func decodeFloats(s string) []float64 {
	var out []float64
	if s != "" {
		_ = json.Unmarshal([]byte(s), &out)
	}
	return out
}

func decodeInts(s string) []int64 {
	var out []int64
	if s != "" {
		_ = json.Unmarshal([]byte(s), &out)
	}
	return out
}

func decodeQuantiles(s string) []Quantile {
	var out []Quantile
	if s != "" {
		_ = json.Unmarshal([]byte(s), &out)
	}
	return out
}

// quantileMean averages a summary's quantiles over rows, weighting each row by its observation count
// (equally when no row has any). Quantiles of a summary cannot be merged exactly; this is a
// representative value for a chart, not a quantile of the union.
type quantileMean struct {
	sum, weight, plain map[float64]float64
	n                  map[float64]float64
}

func newQuantileMean() *quantileMean {
	return &quantileMean{sum: map[float64]float64{}, weight: map[float64]float64{}, plain: map[float64]float64{}, n: map[float64]float64{}}
}

func (m *quantileMean) add(count int64, qs []Quantile) {
	for _, q := range qs {
		m.sum[q.Q] += q.V * float64(count)
		m.weight[q.Q] += float64(count)
		m.plain[q.Q] += q.V
		m.n[q.Q]++
	}
}

func (m *quantileMean) result() []Quantile {
	out := make([]Quantile, 0, len(m.n))
	for q, n := range m.n {
		v := m.plain[q] / n
		if m.weight[q] > 0 {
			v = m.sum[q] / m.weight[q]
		}
		out = append(out, Quantile{Q: q, V: v})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Q < out[j].Q })
	return out
}
