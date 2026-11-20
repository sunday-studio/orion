package metricstore

import (
	"math"
	"sort"

	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
)

func (p *parser) summary(name, unit string, resource map[string]string, dp *metricspb.SummaryDataPoint) {
	if dp.GetFlags()&uint32(metricspb.DataPointFlags_DATA_POINT_FLAGS_NO_RECORDED_VALUE_MASK) != 0 {
		p.rejectPoint(name, "data point has no recorded value")
		return
	}
	h, reason := summaryValue(dp)
	if reason != "" {
		p.rejectPoint(name, reason)
		return
	}
	// A summary has no temporality: its count and sum are cumulative, like a Prometheus summary's.
	p.distribution(name, KindSummary, unit, resource, dp.GetAttributes(), dp.GetTimeUnixNano(), dp.GetStartTimeUnixNano(), true, h)
}

// summaryValue checks a summary data point: finite sum, quantiles in [0, 1] with finite values, and no
// more than MaxSummaryQuantiles of them. It keeps the quantile values as sent and has no buckets.
func summaryValue(dp *metricspb.SummaryDataPoint) (Histogram, string) {
	if dp.GetCount() > math.MaxInt64 {
		return Histogram{}, "summary count is too large"
	}
	if !finite(dp.GetSum()) {
		return Histogram{}, "summary sum is not a finite number"
	}
	if len(dp.GetQuantileValues()) > MaxSummaryQuantiles {
		return Histogram{}, "summary has more than 20 quantiles"
	}
	sum := dp.GetSum()
	h := Histogram{Count: int64(dp.GetCount()), Sum: &sum}
	for _, qv := range dp.GetQuantileValues() {
		if !finite(qv.GetQuantile()) || qv.GetQuantile() < 0 || qv.GetQuantile() > 1 {
			return Histogram{}, "summary quantile must be a number between 0 and 1"
		}
		if !finite(qv.GetValue()) {
			return Histogram{}, "summary quantile value is not a finite number"
		}
		h.Quantiles = append(h.Quantiles, Quantile{Q: qv.GetQuantile(), V: qv.GetValue()})
	}
	sort.SliceStable(h.Quantiles, func(i, j int) bool { return h.Quantiles[i].Q < h.Quantiles[j].Q })
	return h, ""
}
