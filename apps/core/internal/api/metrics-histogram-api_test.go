package api

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	collmetrics "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func fptr(v float64) *float64 { return &v }

func exportMetrics(metrics ...*metricspb.Metric) *collmetrics.ExportMetricsServiceRequest {
	return &collmetrics.ExportMetricsServiceRequest{ResourceMetrics: []*metricspb.ResourceMetrics{{
		Resource:     &resourcepb.Resource{Attributes: []*commonpb.KeyValue{strKV("service.name", "checkout-api")}},
		ScopeMetrics: []*metricspb.ScopeMetrics{{Metrics: metrics}},
	}}}
}

// latencyHistogram is a delta histogram in milliseconds: 100 requests spread evenly over 1..100.
func latencyHistogram(at time.Time) *metricspb.Metric {
	return &metricspb.Metric{Name: "http.server.duration", Unit: "ms", Data: &metricspb.Metric_Histogram{Histogram: &metricspb.Histogram{
		AggregationTemporality: metricspb.AggregationTemporality_AGGREGATION_TEMPORALITY_DELTA,
		DataPoints: []*metricspb.HistogramDataPoint{{
			TimeUnixNano:   uint64(at.UnixNano()),
			Attributes:     []*commonpb.KeyValue{strKV("route", "/pay")},
			Count:          100,
			Sum:            fptr(5050),
			Min:            fptr(1),
			Max:            fptr(100),
			ExplicitBounds: []float64{10, 20, 50, 100},
			BucketCounts:   []uint64{10, 10, 30, 50, 0},
		}},
	}}}
}

func requestsCounter(at time.Time, value int64) *metricspb.Metric {
	return &metricspb.Metric{Name: "http.server.requests", Data: &metricspb.Metric_Sum{Sum: &metricspb.Sum{
		IsMonotonic:            true,
		AggregationTemporality: metricspb.AggregationTemporality_AGGREGATION_TEMPORALITY_CUMULATIVE,
		DataPoints:             []*metricspb.NumberDataPoint{{TimeUnixNano: uint64(at.UnixNano()), Value: &metricspb.NumberDataPoint_AsInt{AsInt: value}}},
	}}}
}

type histogramSeriesBody struct {
	Data struct {
		Series []struct {
			Source string `json:"source"`
			Metric string `json:"metric"`
			Kind   string `json:"kind"`
			Unit   string `json:"unit"`
			Points []struct {
				T         time.Time `json:"t"`
				Value     *float64  `json:"value"`
				Count     *int64    `json:"count"`
				Sum       *float64  `json:"sum"`
				Avg       *float64  `json:"avg"`
				Min       *float64  `json:"min"`
				P50       *float64  `json:"p50"`
				P95       *float64  `json:"p95"`
				P99       *float64  `json:"p99"`
				Quantiles []struct {
					Quantile float64 `json:"quantile"`
					Value    float64 `json:"value"`
				} `json:"quantiles"`
			} `json:"points"`
		} `json:"series"`
	} `json:"data"`
}

func TestOTLPHistogramsShowUpWithPercentilesOnTheServiceOfTheApp(t *testing.T) {
	server, store := metricsServer(t)
	appID, key := createAppWithKey(t, server, "checkout-api")
	svcID := createServiceVia(t, server, gin.H{"name": "Checkout"})
	performJSONRequest(t, server, http.MethodPost, "/v1/services/"+svcID+"/links", gin.H{"kind": "app", "id": appID}, "")

	ts := time.Now().UTC().Add(-10 * time.Minute)
	// Protobuf first: a histogram, a cumulative counter (its first reading is a baseline) and a bad histogram.
	bad := latencyHistogram(ts)
	bad.Name = "bad.histogram"
	bad.GetHistogram().DataPoints[0].ExplicitBounds = []float64{20, 10, 50, 100}
	req := exportMetrics(latencyHistogram(ts), requestsCounter(ts, 1000), bad)
	resp := postOTLPMetrics(server, key, "application/x-protobuf", mustProto(req), false)
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d %s", resp.Code, resp.Body.String())
	}
	if n, msg := rejectedPoints(t, resp, true); n != 1 || !strings.Contains(msg, "bad.histogram") || !strings.Contains(msg, "strictly increasing") {
		t.Fatalf("rejected %d (%q), want the bad histogram counted with its reason", n, msg)
	}
	// JSON and gzip a minute later: another interval of the histogram, and the counter at 1030.
	next := ts.Add(time.Minute)
	js, _ := protojson.Marshal(exportMetrics(latencyHistogram(next), requestsCounter(next, 1030)))
	resp = postOTLPMetrics(server, key, "application/json", js, true)
	if n, msg := rejectedPoints(t, resp, false); resp.Code != http.StatusOK || n != 0 {
		t.Fatalf("second request: %d, rejected %d (%s)", resp.Code, n, msg)
	}
	// A retry of that request must not count twice.
	postOTLPMetrics(server, key, "application/json", js, true)
	store.Flush()

	var got histogramSeriesBody
	resp = performJSONRequest(t, server, http.MethodGet, metricsHistoryPath(svcID), nil, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("metrics status = %d %s", resp.Code, resp.Body.String())
	}
	decodeResponse(t, resp, &got)
	byMetric := map[string]int{}
	for i, s := range got.Data.Series {
		byMetric[s.Metric] = i
	}
	if len(got.Data.Series) != 2 {
		t.Fatalf("series = %+v, want the histogram and the counter (the bad histogram has none)", got.Data.Series)
	}

	hist := got.Data.Series[byMetric["http.server.duration"]]
	if hist.Source != "otlp" || hist.Kind != "histogram" || hist.Unit != "ms" || len(hist.Points) != 2 {
		t.Fatalf("histogram series = %+v", hist)
	}
	p := hist.Points[0]
	if p.Count == nil || *p.Count != 100 || p.Sum == nil || *p.Sum != 5050 || p.Avg == nil || *p.Avg != 50.5 || p.Min == nil || *p.Min != 1 {
		t.Fatalf("point = %+v", p)
	}
	if p.P50 == nil || p.P95 == nil || p.P99 == nil || *p.P50 != 50 || *p.P95 != 95 || *p.P99 != 99 {
		t.Fatalf("percentiles = %v %v %v, want 50 95 99 from the buckets", p.P50, p.P95, p.P99)
	}
	if p.Value != nil || p.Quantiles != nil {
		t.Fatalf("a histogram point has no value or summary quantiles: %+v", p)
	}

	counter := got.Data.Series[byMetric["http.server.requests"]]
	if counter.Kind != "sum" || len(counter.Points) != 1 || counter.Points[0].Value == nil || *counter.Points[0].Value != 30 || counter.Points[0].Count != nil || counter.Points[0].P50 != nil {
		t.Fatalf("counter series = %+v, want one point holding the increase of 30 (the first reading is a baseline, the retry is dropped)", counter)
	}
}

func TestOTLPExponentialHistogramsAndSummariesAreStoredAndRead(t *testing.T) {
	server, store := metricsServer(t)
	appID, key := createAppWithKey(t, server, "checkout-api")
	svcID := createServiceVia(t, server, gin.H{"name": "Checkout"})
	performJSONRequest(t, server, http.MethodPost, "/v1/services/"+svcID+"/links", gin.H{"kind": "app", "id": appID}, "")

	at := time.Now().UTC().Add(-5 * time.Minute)
	exponential := &metricspb.Metric{Name: "rpc.duration", Unit: "ms", Data: &metricspb.Metric_ExponentialHistogram{ExponentialHistogram: &metricspb.ExponentialHistogram{
		AggregationTemporality: metricspb.AggregationTemporality_AGGREGATION_TEMPORALITY_DELTA,
		DataPoints: []*metricspb.ExponentialHistogramDataPoint{{
			TimeUnixNano: uint64(at.UnixNano()), Scale: 0, Count: 7, Sum: fptr(40), Min: fptr(1.5), Max: fptr(7.9),
			Positive: &metricspb.ExponentialHistogramDataPoint_Buckets{Offset: 0, BucketCounts: []uint64{1, 2, 4}},
		}},
	}}}
	summary := &metricspb.Metric{Name: "rpc.latency", Unit: "ms", Data: &metricspb.Metric_Summary{Summary: &metricspb.Summary{DataPoints: []*metricspb.SummaryDataPoint{
		{TimeUnixNano: uint64(at.UnixNano()), Count: 10, Sum: 100, QuantileValues: []*metricspb.SummaryDataPoint_ValueAtQuantile{{Quantile: 0.5, Value: 9}, {Quantile: 0.99, Value: 30}}},
		{TimeUnixNano: uint64(at.Add(time.Minute).UnixNano()), Count: 30, Sum: 400, QuantileValues: []*metricspb.SummaryDataPoint_ValueAtQuantile{{Quantile: 0.5, Value: 12}, {Quantile: 0.99, Value: 40}}},
	}}}}
	resp := postOTLPMetrics(server, key, "application/x-protobuf", mustProto(exportMetrics(exponential, summary)), false)
	if n, msg := rejectedPoints(t, resp, true); resp.Code != http.StatusOK || n != 0 {
		t.Fatalf("status %d rejected %d (%s)", resp.Code, n, msg)
	}
	store.Flush()

	var got histogramSeriesBody
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, metricsHistoryPath(svcID), nil, ""), &got)
	byMetric := map[string]int{}
	for i, s := range got.Data.Series {
		byMetric[s.Metric] = i + 1
	}
	exp := got.Data.Series[byMetric["rpc.duration"]-1]
	if exp.Kind != "histogram" || len(exp.Points) != 1 || *exp.Points[0].Count != 7 || exp.Points[0].P50 == nil || *exp.Points[0].P50 <= 4 {
		t.Fatalf("exponential series = %+v", exp)
	}
	sum := got.Data.Series[byMetric["rpc.latency"]-1]
	if sum.Kind != "summary" || len(sum.Points) != 1 {
		t.Fatalf("summary series = %+v, want one point: the first reading is a baseline", sum)
	}
	sp := sum.Points[0]
	if *sp.Count != 20 || *sp.Sum != 300 || *sp.Avg != 15 || sp.P50 == nil || *sp.P50 != 12 || *sp.P99 != 40 || sp.P95 != nil || len(sp.Quantiles) != 2 {
		t.Fatalf("summary point = %+v: count and sum are the change, quantiles are as sent, p95 was not sent", sp)
	}
}

func TestOTLPMetricsWithOutOfOrderMinutesInOneRequestAreAllKept(t *testing.T) {
	server, store := metricsServer(t)
	appID, key := createAppWithKey(t, server, "checkout-api")
	svcID := createServiceVia(t, server, gin.H{"name": "Checkout"})
	performJSONRequest(t, server, http.MethodPost, "/v1/services/"+svcID+"/links", gin.H{"kind": "app", "id": appID}, "")
	at := time.Now().UTC().Add(-20 * time.Minute)
	// Newest first inside the request: 1000 (t0), then 1050 (t0+2m) listed before 1020 (t0+1m).
	counter := &metricspb.Metric{Name: "jobs", Data: &metricspb.Metric_Sum{Sum: &metricspb.Sum{IsMonotonic: true,
		AggregationTemporality: metricspb.AggregationTemporality_AGGREGATION_TEMPORALITY_CUMULATIVE,
		DataPoints: []*metricspb.NumberDataPoint{
			{TimeUnixNano: uint64(at.UnixNano()), Value: &metricspb.NumberDataPoint_AsInt{AsInt: 1000}},
			{TimeUnixNano: uint64(at.Add(2 * time.Minute).UnixNano()), Value: &metricspb.NumberDataPoint_AsInt{AsInt: 1050}},
			{TimeUnixNano: uint64(at.Add(time.Minute).UnixNano()), Value: &metricspb.NumberDataPoint_AsInt{AsInt: 1020}},
		}}}}
	if resp := postOTLPMetrics(server, key, "application/x-protobuf", mustProto(exportMetrics(counter)), false); resp.Code != http.StatusOK {
		t.Fatalf("status = %d %s", resp.Code, resp.Body.String())
	}
	store.Flush()
	var got histogramSeriesBody
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, metricsHistoryPath(svcID), nil, ""), &got)
	pts := got.Data.Series[0].Points
	if len(pts) != 2 || *pts[0].Value != 20 || *pts[1].Value != 30 {
		t.Fatalf("points = %+v, want increases of 20 and 30", pts)
	}
}

func mustProto(m proto.Message) []byte {
	b, err := proto.Marshal(m)
	if err != nil {
		panic(err)
	}
	return b
}
