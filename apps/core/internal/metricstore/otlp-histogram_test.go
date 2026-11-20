package metricstore

import (
	"math"
	"strings"
	"testing"
	"time"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const (
	cumulativeTemporality = metricspb.AggregationTemporality_AGGREGATION_TEMPORALITY_CUMULATIVE
	deltaTemporality      = metricspb.AggregationTemporality_AGGREGATION_TEMPORALITY_DELTA
)

func explicitDP(at time.Time, bounds []float64, counts []uint64, count uint64, sum *float64, attrs ...*commonpb.KeyValue) *metricspb.HistogramDataPoint {
	return &metricspb.HistogramDataPoint{TimeUnixNano: uint64(at.UnixNano()), Count: count, Sum: sum, ExplicitBounds: bounds, BucketCounts: counts, Attributes: attrs}
}

func histogramMetric(name string, temporality metricspb.AggregationTemporality, dps ...*metricspb.HistogramDataPoint) *metricspb.Metric {
	return &metricspb.Metric{Name: name, Unit: "ms", Data: &metricspb.Metric_Histogram{Histogram: &metricspb.Histogram{AggregationTemporality: temporality, DataPoints: dps}}}
}

func parseOne(t *testing.T, now time.Time, metrics ...*metricspb.Metric) OTLPParsed {
	t.Helper()
	got, _, err := ParseOTLPMetrics("application/x-protobuf", mustMarshal(request([]*commonpb.KeyValue{strAttr("service.name", "api")}, metrics...)), now)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestParseExplicitHistogramIsStoredAsSentInBothEncodings(t *testing.T) {
	now := time.Now().UTC()
	dp := explicitDP(now, []float64{5, 10, 50}, []uint64{1, 2, 3, 4}, 10, fp(321), strAttr("route", "/pay"))
	dp.Min, dp.Max = fp(0.5), fp(80)
	dp.StartTimeUnixNano = uint64(now.Add(-time.Hour).UnixNano())
	req := request(nil, histogramMetric("http.server.duration", deltaTemporality, dp))
	for name, body := range map[string][]byte{"protobuf": mustMarshal(req), "json": mustJSON(req)} {
		ct := map[string]string{"protobuf": "application/x-protobuf", "json": "application/json"}[name]
		got, _, err := ParseOTLPMetrics(ct, body, now)
		if err != nil || got.Rejected != 0 || len(got.Points) != 1 {
			t.Fatalf("%s: %+v %v", name, got, err)
		}
		s := got.Points[0].Sample
		h := s.Values.Histogram
		if s.Kind != KindHistogram || s.Unit != "ms" || s.Metric != "http.server.duration" || !strings.HasPrefix(s.Key, "histogram:http.server.duration|") ||
			s.Labels != `{"route":"/pay"}` || s.Values.Value != nil {
			t.Fatalf("%s: sample = %+v", name, s)
		}
		if h.Count != 10 || *h.Sum != 321 || *h.Min != 0.5 || *h.Max != 80 || !equalFloats(h.Bounds, []float64{5, 10, 50}) || totalOf(h.Counts) != 10 || h.Counts[3] != 4 {
			t.Fatalf("%s: histogram = %+v", name, h)
		}
		if s.Temporality == nil || s.Temporality.Cumulative || s.Temporality.Start.IsZero() {
			t.Fatalf("%s: temporality = %+v", name, s.Temporality)
		}
	}
}

func TestParseHistogramTemporalityAndOptionalFields(t *testing.T) {
	now := time.Now().UTC()
	withMinMax := func() *metricspb.HistogramDataPoint {
		dp := explicitDP(now, []float64{1}, []uint64{1, 1}, 2, fp(3))
		dp.Min, dp.Max = fp(0.5), fp(2)
		return dp
	}
	got := parseOne(t, now,
		histogramMetric("cumulative", cumulativeTemporality, withMinMax()),
		histogramMetric("unspecified", metricspb.AggregationTemporality_AGGREGATION_TEMPORALITY_UNSPECIFIED, withMinMax()),
		histogramMetric("delta", deltaTemporality, withMinMax()),
		histogramMetric("no sum", deltaTemporality, explicitDP(now, []float64{1}, []uint64{1, 1}, 2, nil)),
		histogramMetric("count and sum only", deltaTemporality, explicitDP(now, nil, nil, 4, fp(8))),
		histogramMetric("empty interval", deltaTemporality, explicitDP(now, []float64{1}, []uint64{0, 0}, 0, nil)),
	)
	if got.Rejected != 0 || len(got.Points) != 6 {
		t.Fatalf("parsed = %+v", got)
	}
	by := map[string]Sample{}
	for _, p := range got.Points {
		by[p.Sample.Name] = p.Sample
	}
	if !by["cumulative"].Temporality.Cumulative || !by["unspecified"].Temporality.Cumulative || by["delta"].Temporality.Cumulative {
		t.Fatal("cumulative and unspecified are cumulative, delta is not")
	}
	if h := by["cumulative"].Values.Histogram; h.Min != nil || h.Max != nil {
		t.Fatalf("a cumulative min and max are the lifetime's, not the interval's: %+v", h)
	}
	if h := by["delta"].Values.Histogram; h.Min == nil || *h.Max != 2 {
		t.Fatalf("a delta keeps min and max: %+v", h)
	}
	if h := by["no sum"].Values.Histogram; h.Sum != nil {
		t.Fatalf("a sum that was not sent stays absent: %+v", h)
	}
	if h := by["count and sum only"].Values.Histogram; h.Count != 4 || len(h.Bounds) != 0 || h.Counts != nil {
		t.Fatalf("count and sum only = %+v", h)
	}
}

func TestParseHistogramRejectsBadDataPointsAndKeepsTheGoodOnes(t *testing.T) {
	now := time.Now().UTC()
	nan, inf := math.NaN(), math.Inf(1)
	tooMany := make([]float64, MaxExplicitBounds+1)
	tooManyCounts := make([]uint64, MaxExplicitBounds+2)
	for i := range tooMany {
		tooMany[i] = float64(i)
	}
	noValue := explicitDP(now, []float64{1}, []uint64{1, 0}, 1, nil)
	noValue.Flags = 1
	minAboveMax := explicitDP(now, []float64{1}, []uint64{1, 0}, 1, nil)
	minAboveMax.Min, minAboveMax.Max = fp(5), fp(2)
	nanMin := explicitDP(now, []float64{1}, []uint64{1, 0}, 1, nil)
	nanMin.Min = fp(nan)

	for name, tc := range map[string]struct {
		dp     *metricspb.HistogramDataPoint
		reason string
	}{
		"bounds going down":         {explicitDP(now, []float64{5, 1}, []uint64{1, 0, 0}, 1, nil), "strictly increasing"},
		"equal bounds":              {explicitDP(now, []float64{1, 1}, []uint64{1, 0, 0}, 1, nil), "strictly increasing"},
		"NaN bound":                 {explicitDP(now, []float64{nan}, []uint64{1, 0}, 1, nil), "bucket bound is not a finite"},
		"infinite bound":            {explicitDP(now, []float64{1, inf}, []uint64{1, 0, 0}, 1, nil), "bucket bound is not a finite"},
		"one count too few":         {explicitDP(now, []float64{1, 2}, []uint64{1, 0}, 1, nil), "one more bucket count"},
		"counts without bounds":     {explicitDP(now, nil, []uint64{1, 2}, 3, nil), "one more bucket count"},
		"bounds without counts":     {explicitDP(now, []float64{1}, nil, 0, nil), "one more bucket count"},
		"counts do not add up":      {explicitDP(now, []float64{1}, []uint64{1, 1}, 5, nil), "do not add up"},
		"count above the sum":       {explicitDP(now, []float64{1}, []uint64{1, 1}, 1, nil), "do not add up"},
		"count beyond int64":        {explicitDP(now, nil, nil, math.MaxUint64, nil), "count is too large"},
		"bucket count beyond int64": {explicitDP(now, []float64{1}, []uint64{math.MaxUint64, 0}, 1, nil), "bucket count is too large"},
		"counts overflow together":  {explicitDP(now, []float64{1}, []uint64{math.MaxInt64, math.MaxInt64}, 1, nil), "bucket count is too large"},
		"NaN sum":                   {explicitDP(now, []float64{1}, []uint64{1, 0}, 1, fp(nan)), "not a finite"},
		"infinite sum":              {explicitDP(now, []float64{1}, []uint64{1, 0}, 1, fp(inf)), "not a finite"},
		"NaN min":                   {nanMin, "not a finite"},
		"min above max":             {minAboveMax, "min is greater"},
		"no recorded value":         {noValue, "no recorded value"},
		"too many bounds":           {explicitDP(now, tooMany, tooManyCounts, 0, nil), "more than 512"},
		"ancient":                   {explicitDP(now.Add(-48*time.Hour), []float64{1}, []uint64{1, 0}, 1, nil), "24 hours"},
		"in the future":             {explicitDP(now.Add(time.Hour), []float64{1}, []uint64{1, 0}, 1, nil), "24 hours"},
	} {
		got := parseOne(t, now,
			histogramMetric("latency", deltaTemporality, explicitDP(now, []float64{1}, []uint64{1, 1}, 2, fp(3)), tc.dp))
		if len(got.Points) != 1 || got.Rejected != 1 || len(got.Errors) != 1 || !strings.Contains(got.Errors[0], tc.reason) {
			t.Errorf("%s: points %d rejected %d errors %v, want the good point kept and one rejection mentioning %q", name, len(got.Points), got.Rejected, got.Errors, tc.reason)
		}
	}
	// A bad name rejects every data point of the metric.
	got := parseOne(t, now, histogramMetric("", deltaTemporality, explicitDP(now, nil, nil, 0, nil), explicitDP(now, nil, nil, 0, nil)))
	if got.Rejected != 2 || len(got.Points) != 0 {
		t.Fatalf("unnamed histogram: %+v", got)
	}
}

func TestParseSumsDecideWhatIsStoredAsChangesAndWhatAsALevel(t *testing.T) {
	now := time.Now().UTC()
	sum := func(name string, monotonic bool, temporality metricspb.AggregationTemporality, v float64) *metricspb.Metric {
		return &metricspb.Metric{Name: name, Data: &metricspb.Metric_Sum{Sum: &metricspb.Sum{IsMonotonic: monotonic, AggregationTemporality: temporality,
			DataPoints: []*metricspb.NumberDataPoint{dpDouble(now, v)}}}}
	}
	got := parseOne(t, now,
		sum("counter.cumulative", true, cumulativeTemporality, 5),
		sum("counter.delta", true, deltaTemporality, 5),
		sum("updown.cumulative", false, cumulativeTemporality, -3),
		sum("updown.delta", false, deltaTemporality, -3),
		sum("counter.negative", true, deltaTemporality, -1),
		sum("counter.negative.cumulative", true, cumulativeTemporality, -1),
	)
	if got.Rejected != 2 || len(got.Points) != 4 {
		t.Fatalf("parsed = %+v", got)
	}
	by := map[string]Sample{}
	for _, p := range got.Points {
		by[p.Sample.Name] = p.Sample
	}
	if s := by["counter.cumulative"]; s.Kind != KindSum || s.Temporality == nil || !s.Temporality.Cumulative {
		t.Errorf("monotonic cumulative sum = %+v", s)
	}
	if s := by["counter.delta"]; s.Kind != KindSum || s.Temporality == nil || s.Temporality.Cumulative {
		t.Errorf("monotonic delta sum = %+v", s)
	}
	if s := by["updown.cumulative"]; s.Kind != KindGauge || s.Temporality != nil {
		t.Errorf("a non-monotonic cumulative sum is a level, stored like a gauge: %+v", s)
	}
	if s := by["updown.delta"]; s.Kind != KindSum || s.Temporality == nil || s.Temporality.Cumulative {
		t.Errorf("a non-monotonic delta sum is a net change: %+v", s)
	}
}

func TestParseSummaryKeepsItsQuantilesAndHasNoBuckets(t *testing.T) {
	now := time.Now().UTC()
	nan := math.NaN()
	summary := func(dp *metricspb.SummaryDataPoint) *metricspb.Metric {
		return &metricspb.Metric{Name: "rpc.latency", Unit: "ms", Data: &metricspb.Metric_Summary{Summary: &metricspb.Summary{DataPoints: []*metricspb.SummaryDataPoint{dp}}}}
	}
	qv := func(q, v float64) *metricspb.SummaryDataPoint_ValueAtQuantile {
		return &metricspb.SummaryDataPoint_ValueAtQuantile{Quantile: q, Value: v}
	}
	base := func(qs ...*metricspb.SummaryDataPoint_ValueAtQuantile) *metricspb.SummaryDataPoint {
		return &metricspb.SummaryDataPoint{TimeUnixNano: uint64(now.UnixNano()), Count: 100, Sum: 4000, QuantileValues: qs}
	}
	got := parseOne(t, now, summary(base(qv(0.99, 90), qv(0.5, 20), qv(0.95, 70))))
	if got.Rejected != 0 || len(got.Points) != 1 {
		t.Fatalf("parsed = %+v", got)
	}
	s := got.Points[0].Sample
	h := s.Values.Histogram
	if s.Kind != KindSummary || !strings.HasPrefix(s.Key, "summary:") || h.Count != 100 || *h.Sum != 4000 || len(h.Bounds) != 0 || len(h.Counts) != 0 ||
		len(h.Quantiles) != 3 || h.Quantiles[0] != (Quantile{0.5, 20}) || h.Quantiles[2] != (Quantile{0.99, 90}) {
		t.Fatalf("summary sample = %+v", s)
	}
	if !s.Temporality.Cumulative {
		t.Fatal("a summary's count and sum are cumulative")
	}

	many := base()
	for i := 0; i <= MaxSummaryQuantiles; i++ {
		many.QuantileValues = append(many.QuantileValues, qv(float64(i)/100, 1))
	}
	badSum := base()
	badSum.Sum = math.Inf(1)
	noValue := base()
	noValue.Flags = 1
	hugeCount := base()
	hugeCount.Count = math.MaxUint64
	for name, tc := range map[string]struct {
		dp     *metricspb.SummaryDataPoint
		reason string
	}{
		"quantile above 1":      {base(qv(1.5, 1)), "between 0 and 1"},
		"negative quantile":     {base(qv(-0.1, 1)), "between 0 and 1"},
		"NaN quantile":          {base(qv(nan, 1)), "between 0 and 1"},
		"NaN quantile value":    {base(qv(0.5, nan)), "quantile value is not a finite"},
		"infinite value":        {base(qv(0.5, math.Inf(-1))), "quantile value is not a finite"},
		"infinite sum":          {badSum, "sum is not a finite"},
		"too many quantiles":    {many, "more than 20 quantiles"},
		"no recorded value":     {noValue, "no recorded value"},
		"count beyond int64":    {hugeCount, "count is too large"},
		"timestamp in the past": {&metricspb.SummaryDataPoint{TimeUnixNano: uint64(now.Add(-72 * time.Hour).UnixNano()), Count: 1, Sum: 1}, "24 hours"},
	} {
		got := parseOne(t, now, summary(tc.dp))
		if len(got.Points) != 0 || got.Rejected != 1 || !strings.Contains(strings.Join(got.Errors, ";"), tc.reason) {
			t.Errorf("%s: %+v, want one rejection mentioning %q", name, got, tc.reason)
		}
	}
}

func TestParseRejectionsAreCountedPerDataPointNotPerRequest(t *testing.T) {
	now := time.Now().UTC()
	bad := explicitDP(now, []float64{2, 1}, []uint64{1, 0, 0}, 1, nil)
	got := parseOne(t, now,
		histogramMetric("a", deltaTemporality, bad, bad, bad),
		gauge("ok", "", dpDouble(now, 1)),
	)
	if got.Rejected != 3 || len(got.Points) != 1 || len(got.Errors) != 1 {
		t.Fatalf("parsed = %+v: three rejected data points share one reason", got)
	}
}

func mustJSON(m proto.Message) []byte {
	b, err := protojson.Marshal(m)
	if err != nil {
		panic(err)
	}
	return b
}
