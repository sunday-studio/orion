package metricstore

import (
	"errors"
	"fmt"
	"math"
	"testing"
	"time"

	collmetrics "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func strAttr(k, v string) *commonpb.KeyValue {
	return &commonpb.KeyValue{Key: k, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: v}}}
}

func dpDouble(at time.Time, v float64, attrs ...*commonpb.KeyValue) *metricspb.NumberDataPoint {
	return &metricspb.NumberDataPoint{TimeUnixNano: uint64(at.UnixNano()), Value: &metricspb.NumberDataPoint_AsDouble{AsDouble: v}, Attributes: attrs}
}

func gauge(name, unit string, points ...*metricspb.NumberDataPoint) *metricspb.Metric {
	return &metricspb.Metric{Name: name, Unit: unit, Data: &metricspb.Metric_Gauge{Gauge: &metricspb.Gauge{DataPoints: points}}}
}

func request(resourceAttrs []*commonpb.KeyValue, metrics ...*metricspb.Metric) *collmetrics.ExportMetricsServiceRequest {
	return &collmetrics.ExportMetricsServiceRequest{ResourceMetrics: []*metricspb.ResourceMetrics{{
		Resource:     &resourcepb.Resource{Attributes: resourceAttrs},
		ScopeMetrics: []*metricspb.ScopeMetrics{{Metrics: metrics}},
	}}}
}

func TestParseOTLPMetricsGaugesAndSumsInBothEncodings(t *testing.T) {
	now := time.Now().UTC()
	req := request([]*commonpb.KeyValue{strAttr("service.name", "checkout-api"), strAttr("telemetry.sdk.language", "go")},
		gauge("process.memory.usage", "By", dpDouble(now, 1048576), dpDouble(now.Add(-time.Minute), 2097152)),
		&metricspb.Metric{Name: "http.server.requests", Data: &metricspb.Metric_Sum{Sum: &metricspb.Sum{DataPoints: []*metricspb.NumberDataPoint{
			{TimeUnixNano: uint64(now.UnixNano()), Value: &metricspb.NumberDataPoint_AsInt{AsInt: 42}, Attributes: []*commonpb.KeyValue{strAttr("route", "/pay"), strAttr("method", "GET")}},
		}}}},
	)
	pb, _ := proto.Marshal(req)
	js, _ := protojson.Marshal(req)
	for name, tc := range map[string]struct {
		contentType string
		body        []byte
		format      OTLPFormat
	}{"protobuf": {"application/x-protobuf", pb, OTLPProtobuf}, "json": {"application/json; charset=utf-8", js, OTLPJSON}} {
		t.Run(name, func(t *testing.T) {
			got, format, err := ParseOTLPMetrics(tc.contentType, tc.body, now)
			if err != nil || format != tc.format || got.Rejected != 0 || len(got.Points) != 3 {
				t.Fatalf("parsed = %+v, %v, %v", got, format, err)
			}
			mem := got.Points[0].Sample
			if mem.Metric != "process.memory.usage" || mem.Kind != "gauge" || mem.Unit != "By" || *mem.Values.Value != 1048576 ||
				mem.Labels != `{"resource.service.name":"checkout-api"}` {
				t.Fatalf("gauge sample = %+v (only identifying resource attributes become labels)", mem)
			}
			if got.Points[0].Sample.Key != got.Points[1].Sample.Key {
				t.Fatalf("two points of one series must share a key")
			}
			sum := got.Points[2].Sample
			if sum.Kind != "sum" || *sum.Values.Value != 42 || sum.Labels != `{"method":"GET","resource.service.name":"checkout-api","route":"/pay"}` {
				t.Fatalf("sum sample = %+v (labels must be sorted so the identity is stable)", sum)
			}
		})
	}
}

func TestParseOTLPMetricsRejectsWhatItCannotStoreWithoutFailingTheRequest(t *testing.T) {
	now := time.Now().UTC()
	bad := &metricspb.NumberDataPoint{TimeUnixNano: uint64(now.UnixNano())} // no value
	noValue := &metricspb.NumberDataPoint{TimeUnixNano: uint64(now.UnixNano()), Value: &metricspb.NumberDataPoint_AsDouble{AsDouble: 1}, Flags: 1}
	req := request(nil,
		gauge("ok", "", dpDouble(now, 1)),
		gauge("nan", "", dpDouble(now, math.NaN()), dpDouble(now, math.Inf(1))),
		gauge("ancient", "", dpDouble(now.Add(-48*time.Hour), 1)),
		gauge("future", "", dpDouble(now.Add(time.Hour), 1)),
		gauge("novalue", "", bad, noValue),
		gauge("", "", dpDouble(now, 1)),
		&metricspb.Metric{Name: "latency", Data: &metricspb.Metric_Histogram{Histogram: &metricspb.Histogram{DataPoints: []*metricspb.HistogramDataPoint{{}, {}}}}},
	)
	body, _ := proto.Marshal(req)
	got, _, err := ParseOTLPMetrics("application/x-protobuf", body, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Points) != 1 || got.Points[0].Sample.Metric != "ok" || got.Rejected != 9 {
		t.Fatalf("points %d rejected %d errors %v, want 1 stored and 9 rejected", len(got.Points), got.Rejected, got.Errors)
	}
	if len(got.Errors) == 0 || len(got.Errors) > maxRejectMessages {
		t.Fatalf("errors = %v, want a few distinct reasons", got.Errors)
	}
}

func TestParseOTLPMetricsRequestErrors(t *testing.T) {
	now := time.Now().UTC()
	if _, _, err := ParseOTLPMetrics("text/plain", []byte("x"), now); !errors.Is(err, ErrUnsupportedMedia) {
		t.Fatalf("media type: %v", err)
	}
	if _, _, err := ParseOTLPMetrics("application/json", []byte("{not json"), now); !errors.Is(err, ErrInvalidBody) {
		t.Fatalf("bad json: %v", err)
	}
	if _, _, err := ParseOTLPMetrics("application/x-protobuf", []byte{0xff, 0xff, 0xff}, now); !errors.Is(err, ErrInvalidBody) {
		t.Fatalf("bad protobuf: %v", err)
	}
	points := make([]*metricspb.NumberDataPoint, MaxPointsPerRequest+1)
	for i := range points {
		points[i] = dpDouble(now, 1)
	}
	body, _ := proto.Marshal(request(nil, gauge("big", "", points...)))
	if _, _, err := ParseOTLPMetrics("application/x-protobuf", body, now); !errors.Is(err, ErrTooManyPoints) {
		t.Fatalf("too many points: %v", err)
	}
	empty, _ := proto.Marshal(&collmetrics.ExportMetricsServiceRequest{})
	if got, _, err := ParseOTLPMetrics("application/x-protobuf", empty, now); err != nil || len(got.Points) != 0 {
		t.Fatalf("empty request = %+v, %v", got, err)
	}
}

func TestOTLPMetricsResponseCarriesPartialSuccess(t *testing.T) {
	for _, format := range []OTLPFormat{OTLPJSON, OTLPProtobuf} {
		body, ct, err := OTLPMetricsResponse(format, 3, "limit reached")
		if err != nil {
			t.Fatal(err)
		}
		resp := &collmetrics.ExportMetricsServiceResponse{}
		if format == OTLPProtobuf {
			err = proto.Unmarshal(body, resp)
		} else {
			err = protojson.Unmarshal(body, resp)
		}
		if err != nil || resp.GetPartialSuccess().GetRejectedDataPoints() != 3 || resp.GetPartialSuccess().GetErrorMessage() != "limit reached" || ct == "" {
			t.Fatalf("format %d: %+v %q %v", format, resp, ct, err)
		}
		clean, _, _ := OTLPMetricsResponse(format, 0, "")
		if format == OTLPProtobuf && len(clean) != 0 {
			t.Fatalf("a fully accepted request needs an empty protobuf response, got %d bytes", len(clean))
		}
	}
}

func TestAllowSeriesAdmitsKnownSeriesAndNewOnesUnderTheCap(t *testing.T) {
	s := openTestStore(t)
	now := time.Now().UTC()
	known := func(key string) Sample {
		return Sample{Key: key, Kind: "gauge", Name: key, Metric: key, Values: Values{Value: fp(1)}}
	}
	s.Enqueue(Batch{Source: SourceOTLP, AppID: "app-1", At: now, Samples: []Sample{known("a"), known("b")}})
	s.Flush()

	allowed, err := s.AllowSeries(SourceOTLP, "", "app-1", []string{"a", "c", "d", "e"}, 3)
	if err != nil {
		t.Fatal(err)
	}
	if !allowed["a"] || !allowed["c"] || allowed["d"] || allowed["e"] {
		t.Fatalf("allowed = %v: a is known, c is the one free slot, d and e are over the cap of 3", allowed)
	}
	// Another app has its own budget.
	other, _ := s.AllowSeries(SourceOTLP, "", "app-2", []string{"a", "b", "c", "d"}, 3)
	if !other["a"] || !other["b"] || !other["c"] || other["d"] {
		t.Fatalf("other app = %v", other)
	}
	if def, _ := s.AllowSeries(SourceOTLP, "", "app-3", make([]string, 0), 0); len(def) != 0 {
		t.Fatalf("no keys = %v", def)
	}
}

func TestOTLPAttributeLimitDoesNotDependOnSendOrder(t *testing.T) {
	now := time.Now().UTC()
	var forward, backward []*commonpb.KeyValue
	for i := 0; i < maxAttributes+5; i++ {
		forward = append(forward, strAttr(fmt.Sprintf("attr.%02d", i), "v"))
	}
	for i := len(forward) - 1; i >= 0; i-- {
		backward = append(backward, forward[i])
	}
	a, _, _ := ParseOTLPMetrics("application/x-protobuf", mustMarshal(request(nil, gauge("m", "", dpDouble(now, 1, forward...)))), now)
	b, _, _ := ParseOTLPMetrics("application/x-protobuf", mustMarshal(request(nil, gauge("m", "", dpDouble(now, 1, backward...)))), now)
	if len(a.Points) != 1 || len(b.Points) != 1 || a.Points[0].Sample.Key != b.Points[0].Sample.Key {
		t.Fatalf("one data point mapped to two series depending on attribute order:\n%s\n%s", a.Points[0].Sample.Key, b.Points[0].Sample.Key)
	}
}

func mustMarshal(m proto.Message) []byte {
	b, err := proto.Marshal(m)
	if err != nil {
		panic(err)
	}
	return b
}
