package api

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"net/http"
	"net/http/httptest"
	"orion/core/internal/metricstore"
	"orion/core/internal/utils"
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

func metricsServer(t *testing.T) (*Server, *metricstore.Store) {
	t.Helper()
	server := setupLogsServer(t, nil)
	store, err := metricstore.Open(t.TempDir(), utils.NewLogger())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	server.EnableMetrics(store)
	return server, store
}

func gaugeRequest(ts time.Time, series int) *collmetrics.ExportMetricsServiceRequest {
	var metrics []*metricspb.Metric
	for i := 0; i < series; i++ {
		metrics = append(metrics, &metricspb.Metric{Name: fmt.Sprintf("app.metric.%02d", i), Unit: "1", Data: &metricspb.Metric_Gauge{Gauge: &metricspb.Gauge{
			DataPoints: []*metricspb.NumberDataPoint{{TimeUnixNano: uint64(ts.UnixNano()), Value: &metricspb.NumberDataPoint_AsDouble{AsDouble: float64(i) + 0.5}}},
		}}})
	}
	return &collmetrics.ExportMetricsServiceRequest{ResourceMetrics: []*metricspb.ResourceMetrics{{
		Resource:     &resourcepb.Resource{Attributes: []*commonpb.KeyValue{strKV("service.name", "checkout-api")}},
		ScopeMetrics: []*metricspb.ScopeMetrics{{Metrics: metrics}},
	}}}
}

func postOTLPMetrics(server *Server, key, contentType string, body []byte, gzipped bool) *httptest.ResponseRecorder {
	if gzipped {
		var buf bytes.Buffer
		w := gzip.NewWriter(&buf)
		w.Write(body)
		w.Close()
		body = buf.Bytes()
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/ingest/otlp/v1/metrics", bytes.NewReader(body))
	req.Header.Set("Content-Type", contentType)
	if gzipped {
		req.Header.Set("Content-Encoding", "gzip")
	}
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	return rawRequest(server, req)
}

func rejectedPoints(t *testing.T, resp *httptest.ResponseRecorder, protobuf bool) (int64, string) {
	t.Helper()
	out := &collmetrics.ExportMetricsServiceResponse{}
	var err error
	if protobuf {
		err = proto.Unmarshal(resp.Body.Bytes(), out)
	} else {
		err = protojson.Unmarshal(resp.Body.Bytes(), out)
	}
	if err != nil {
		t.Fatalf("response body %q: %v", resp.Body.String(), err)
	}
	return out.GetPartialSuccess().GetRejectedDataPoints(), out.GetPartialSuccess().GetErrorMessage()
}

func TestOTLPMetricsShowUpOnTheServiceOfTheApp(t *testing.T) {
	server, store := metricsServer(t)
	appID, key := createAppWithKey(t, server, "checkout-api")
	svcID := createServiceVia(t, server, gin.H{"name": "Checkout"})
	performJSONRequest(t, server, http.MethodPost, "/v1/services/"+svcID+"/links", gin.H{"kind": "app", "id": appID}, "")

	ts := time.Now().UTC().Add(-time.Minute)
	pb, _ := proto.Marshal(gaugeRequest(ts, 2))
	js, _ := protojson.Marshal(gaugeRequest(ts.Add(time.Minute), 2))
	for name, tc := range map[string]struct {
		contentType string
		body        []byte
		gz          bool
		protobuf    bool
	}{"protobuf": {"application/x-protobuf", pb, false, true}, "json gzip": {"application/json", js, true, false}} {
		resp := postOTLPMetrics(server, key, tc.contentType, tc.body, tc.gz)
		if resp.Code != http.StatusOK {
			t.Fatalf("%s: status = %d %s", name, resp.Code, resp.Body.String())
		}
		if n, msg := rejectedPoints(t, resp, tc.protobuf); n != 0 {
			t.Fatalf("%s: rejected %d (%s)", name, n, msg)
		}
	}
	store.Flush()

	var got struct {
		Data struct {
			Series []struct {
				Source string `json:"source"`
				AppID  string `json:"app_id"`
				Metric string `json:"metric"`
				Kind   string `json:"kind"`
				Unit   string `json:"unit"`
				Labels string `json:"labels"`
				Points []struct {
					Value *float64 `json:"value"`
				} `json:"points"`
			} `json:"series"`
		} `json:"data"`
	}
	resp := performJSONRequest(t, server, http.MethodGet, "/v1/services/"+svcID+"/metrics?from=-1h", nil, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("metrics status = %d %s", resp.Code, resp.Body.String())
	}
	decodeResponse(t, resp, &got)
	if len(got.Data.Series) != 2 {
		t.Fatalf("series = %+v", got.Data.Series)
	}
	first := got.Data.Series[0]
	if first.Source != "otlp" || first.AppID != appID || first.Metric != "app.metric.00" || first.Kind != "gauge" || first.Unit != "1" ||
		first.Labels != `{"resource.service.name":"checkout-api"}` || len(first.Points) != 2 || first.Points[0].Value == nil || *first.Points[0].Value != 0.5 {
		t.Fatalf("first series = %+v", first)
	}
}

func TestOTLPMetricsSeriesCapIsReportedAndRaisablePerApp(t *testing.T) {
	server, store := metricsServer(t)
	appID, key := createAppWithKey(t, server, "noisy")
	ts := time.Now().UTC()

	body, _ := proto.Marshal(gaugeRequest(ts, metricstore.DefaultSeriesCapPerApp+5))
	resp := postOTLPMetrics(server, key, "application/x-protobuf", body, false)
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d %s", resp.Code, resp.Body.String())
	}
	if n, msg := rejectedPoints(t, resp, true); n != 5 || msg == "" {
		t.Fatalf("first request: rejected %d (%q), want the 5 series over the cap of 100 with an explanation", n, msg)
	}
	store.Flush()
	// The 100 series that fit are known now; the same request is refused for the same 5.
	resp = postOTLPMetrics(server, key, "application/x-protobuf", body, false)
	if n, msg := rejectedPoints(t, resp, true); n != 5 || msg == "" {
		t.Fatalf("second request: rejected %d (%q), want 5", n, msg)
	}

	// Raising the cap on the app lets the rest in.
	if r := performJSONRequest(t, server, http.MethodPatch, "/v1/apps/"+appID, gin.H{"metric_series_cap": 200}, ""); r.Code != http.StatusOK {
		t.Fatalf("raise cap = %d %s", r.Code, r.Body.String())
	}
	server.logs.ingester.Invalidate()
	resp = postOTLPMetrics(server, key, "application/x-protobuf", body, false)
	if n, _ := rejectedPoints(t, resp, true); n != 0 {
		t.Fatalf("after raising the cap rejected %d", n)
	}
	if r := performJSONRequest(t, server, http.MethodPatch, "/v1/apps/"+appID, gin.H{"metric_series_cap": 10001}, ""); r.Code != http.StatusBadRequest {
		t.Fatalf("cap above the maximum = %d", r.Code)
	}
}

func TestOTLPMetricsRequestErrors(t *testing.T) {
	server, _ := metricsServer(t)
	_, key := createAppWithKey(t, server, "errs")
	good, _ := proto.Marshal(gaugeRequest(time.Now(), 1))

	if got := postOTLPMetrics(server, "", "application/x-protobuf", good, false).Code; got != http.StatusUnauthorized {
		t.Errorf("no key = %d, want 401", got)
	}
	if got := postOTLPMetrics(server, "orion_nope", "application/x-protobuf", good, false).Code; got != http.StatusUnauthorized {
		t.Errorf("bad key = %d, want 401", got)
	}
	if got := postOTLPMetrics(server, key, "text/plain", good, false).Code; got != http.StatusUnsupportedMediaType {
		t.Errorf("text/plain = %d, want 415", got)
	}
	if got := postOTLPMetrics(server, key, "application/json", []byte(`{"resourceMetrics": "nope"}`), false).Code; got != http.StatusBadRequest {
		t.Errorf("bad json = %d, want 400", got)
	}
	if got := postOTLPMetrics(server, key, "application/x-protobuf", []byte("\x0a\xff\xff\xff garbage"), false).Code; got != http.StatusBadRequest {
		t.Errorf("garbage protobuf = %d, want 400", got)
	}
	big, _ := proto.Marshal(gaugeRequest(time.Now(), metricstore.MaxPointsPerRequest+1))
	if got := postOTLPMetrics(server, key, "application/x-protobuf", big, true).Code; got != http.StatusRequestEntityTooLarge {
		t.Errorf("too many points = %d, want 413", got)
	}
	empty, _ := proto.Marshal(&collmetrics.ExportMetricsServiceRequest{})
	if got := postOTLPMetrics(server, key, "application/x-protobuf", empty, false).Code; got != http.StatusOK {
		t.Errorf("empty export = %d, want 200", got)
	}

	hist := &collmetrics.ExportMetricsServiceRequest{ResourceMetrics: []*metricspb.ResourceMetrics{{ScopeMetrics: []*metricspb.ScopeMetrics{{Metrics: []*metricspb.Metric{
		{Name: "latency", Data: &metricspb.Metric_Histogram{Histogram: &metricspb.Histogram{DataPoints: []*metricspb.HistogramDataPoint{{}, {}}}}},
	}}}}}}
	body, _ := proto.Marshal(hist)
	resp := postOTLPMetrics(server, key, "application/x-protobuf", body, false)
	if n, msg := rejectedPoints(t, resp, true); resp.Code != http.StatusOK || n != 2 || msg == "" {
		t.Errorf("histogram = %d, rejected %d (%q), want 200 with 2 rejected", resp.Code, n, msg)
	}
}

func TestOTLPMetricsWithoutAMetricStoreIs503(t *testing.T) {
	server := setupLogsServer(t, nil)
	_, key := createAppWithKey(t, server, "nostore")
	body, _ := proto.Marshal(gaugeRequest(time.Now(), 1))
	if got := postOTLPMetrics(server, key, "application/x-protobuf", body, false).Code; got != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", got)
	}
}

// One request can carry a day of points, each in its own minute, so it makes more batches than the
// write queue holds. It must be accepted whole rather than answered with 429 halfway.
func TestOTLPMetricsRequestSpreadOverMoreMinutesThanTheQueueHolds(t *testing.T) {
	server, store := metricsServer(t)
	_, key := createAppWithKey(t, server, "checkout-api")
	const minutes = 1300
	start := time.Now().UTC().Add(-time.Duration(minutes+5) * time.Minute)
	var points []*metricspb.NumberDataPoint
	for i := 0; i < minutes; i++ {
		at := start.Add(time.Duration(i) * time.Minute)
		points = append(points, &metricspb.NumberDataPoint{TimeUnixNano: uint64(at.UnixNano()), Value: &metricspb.NumberDataPoint_AsDouble{AsDouble: float64(i)}})
	}
	body, _ := proto.Marshal(&collmetrics.ExportMetricsServiceRequest{ResourceMetrics: []*metricspb.ResourceMetrics{{
		Resource: &resourcepb.Resource{Attributes: []*commonpb.KeyValue{strKV("service.name", "checkout-api")}},
		ScopeMetrics: []*metricspb.ScopeMetrics{{Metrics: []*metricspb.Metric{{Name: "app.backfill", Unit: "1", Data: &metricspb.Metric_Gauge{
			Gauge: &metricspb.Gauge{DataPoints: points},
		}}}}},
	}}})
	resp := postOTLPMetrics(server, key, "application/x-protobuf", body, false)
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d %s", resp.Code, resp.Body.String())
	}
	if n, msg := rejectedPoints(t, resp, true); n != 0 {
		t.Fatalf("rejected %d (%s)", n, msg)
	}
	store.Flush()
}
