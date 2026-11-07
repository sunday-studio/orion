package metricstore

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	collmetrics "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

var (
	ErrUnsupportedMedia = errors.New("unsupported OTLP content type")
	ErrInvalidBody      = errors.New("invalid OTLP metrics request")
	ErrTooManyPoints    = errors.New("too many data points in one request")
)

const (
	// MaxPointsPerRequest bounds one OTLP request, the same ceiling as log records.
	MaxPointsPerRequest = 10_000
	maxMetricNameLen    = 200
	maxAttributes       = 20
	maxAttributeText    = 128
	maxRejectMessages   = 5
	// How far from now a data point may be: older ones are no use to a 7 day history, later ones are clock errors.
	maxPointAge  = 24 * time.Hour
	maxPointSkew = 5 * time.Minute
)

// resourceLabelKeys are the resource attributes that tell instances of one app apart. The others are
// per app and would only add noise to the series identity.
var resourceLabelKeys = []string{"service.name", "service.instance.id", "host.name"}

// OTLPFormat is the wire encoding of an OTLP/HTTP request, which the response mirrors.
type OTLPFormat int

const (
	OTLPJSON OTLPFormat = iota
	OTLPProtobuf
)

// TimedSample is one data point with its timestamp.
type TimedSample struct {
	At     time.Time
	Sample Sample
}

// OTLPParsed is what an ExportMetricsServiceRequest contributes. Rejected counts data points that were
// refused; Errors holds the first few reasons for the response.
type OTLPParsed struct {
	Points   []TimedSample
	Rejected int64
	Errors   []string
}

// ParseOTLPMetrics decodes an OTLP/HTTP ExportMetricsServiceRequest (protobuf or JSON). Gauges and
// sums become samples; histograms and summaries are counted as rejected until they are supported.
func ParseOTLPMetrics(contentType string, body []byte, now time.Time) (OTLPParsed, OTLPFormat, error) {
	mediaType := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	request := &collmetrics.ExportMetricsServiceRequest{}
	var format OTLPFormat
	switch mediaType {
	case "application/x-protobuf", "application/protobuf":
		format = OTLPProtobuf
		if err := proto.Unmarshal(body, request); err != nil {
			return OTLPParsed{}, format, fmt.Errorf("%w: %v", ErrInvalidBody, err)
		}
	case "application/json":
		format = OTLPJSON
		if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(body, request); err != nil {
			return OTLPParsed{}, format, fmt.Errorf("%w: %v", ErrInvalidBody, err)
		}
	default:
		return OTLPParsed{}, OTLPJSON, ErrUnsupportedMedia
	}

	total := 0
	for _, rm := range request.GetResourceMetrics() {
		for _, sm := range rm.GetScopeMetrics() {
			for _, m := range sm.GetMetrics() {
				total += pointCount(m)
			}
		}
	}
	if total > MaxPointsPerRequest {
		return OTLPParsed{}, format, ErrTooManyPoints
	}

	var out OTLPParsed
	reject := func(n int64, reason string) {
		out.Rejected += n
		if len(out.Errors) < maxRejectMessages && !containsString(out.Errors, reason) {
			out.Errors = append(out.Errors, reason)
		}
	}
	for _, rm := range request.GetResourceMetrics() {
		resource := resourceLabels(rm.GetResource().GetAttributes())
		for _, sm := range rm.GetScopeMetrics() {
			for _, m := range sm.GetMetrics() {
				name := strings.TrimSpace(m.GetName())
				var kind string
				var points []*metricspb.NumberDataPoint
				switch data := m.GetData().(type) {
				case *metricspb.Metric_Gauge:
					kind, points = "gauge", data.Gauge.GetDataPoints()
				case *metricspb.Metric_Sum:
					kind, points = "sum", data.Sum.GetDataPoints()
				default:
					reject(int64(pointCount(m)), fmt.Sprintf("metric %q: only gauges and sums are supported", name))
					continue
				}
				if name == "" || len(name) > maxMetricNameLen {
					reject(int64(len(points)), "metric name must be 1 to 200 characters")
					continue
				}
				for _, dp := range points {
					sample, at, reason := numberSample(name, kind, m.GetUnit(), resource, dp, now)
					if reason != "" {
						reject(1, fmt.Sprintf("metric %q: %s", name, reason))
						continue
					}
					out.Points = append(out.Points, TimedSample{At: at, Sample: sample})
				}
			}
		}
	}
	return out, format, nil
}

func pointCount(m *metricspb.Metric) int {
	switch data := m.GetData().(type) {
	case *metricspb.Metric_Gauge:
		return len(data.Gauge.GetDataPoints())
	case *metricspb.Metric_Sum:
		return len(data.Sum.GetDataPoints())
	case *metricspb.Metric_Histogram:
		return len(data.Histogram.GetDataPoints())
	case *metricspb.Metric_ExponentialHistogram:
		return len(data.ExponentialHistogram.GetDataPoints())
	case *metricspb.Metric_Summary:
		return len(data.Summary.GetDataPoints())
	}
	return 0
}

func numberSample(name, kind, unit string, resource map[string]string, dp *metricspb.NumberDataPoint, now time.Time) (Sample, time.Time, string) {
	if dp.GetFlags()&uint32(metricspb.DataPointFlags_DATA_POINT_FLAGS_NO_RECORDED_VALUE_MASK) != 0 {
		return Sample{}, time.Time{}, "data point has no recorded value"
	}
	var v float64
	switch x := dp.GetValue().(type) {
	case *metricspb.NumberDataPoint_AsDouble:
		v = x.AsDouble
	case *metricspb.NumberDataPoint_AsInt:
		v = float64(x.AsInt)
	default:
		return Sample{}, time.Time{}, "data point has no value"
	}
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return Sample{}, time.Time{}, "value is not a finite number"
	}
	at := now
	if ns := dp.GetTimeUnixNano(); ns != 0 {
		at = time.Unix(0, int64(ns)).UTC()
	}
	if at.Before(now.Add(-maxPointAge)) || at.After(now.Add(maxPointSkew)) {
		return Sample{}, time.Time{}, "timestamp is more than 24 hours old or in the future"
	}
	labels := map[string]string{}
	for k, val := range resource {
		labels["resource."+k] = val
	}
	count := 0
	for _, kv := range dp.GetAttributes() {
		if count == maxAttributes {
			break
		}
		if k := clip(kv.GetKey(), maxAttributeText); k != "" {
			labels[k] = clip(anyText(kv.GetValue()), maxAttributeText)
			count++
		}
	}
	encoded := ""
	if len(labels) > 0 {
		b, _ := json.Marshal(labels) // map keys marshal sorted, so the identity is stable
		encoded = string(b)
	}
	value := v
	return Sample{Key: name + "|" + encoded, Kind: kind, Name: name, Metric: name, Unit: clip(unit, maxAttributeText), Labels: encoded,
		Values: Values{Value: &value}}, at, ""
}

func resourceLabels(attrs []*commonpb.KeyValue) map[string]string {
	out := map[string]string{}
	for _, kv := range attrs {
		for _, want := range resourceLabelKeys {
			if kv.GetKey() == want {
				out[want] = clip(anyText(kv.GetValue()), maxAttributeText)
			}
		}
	}
	return out
}

func anyText(v *commonpb.AnyValue) string {
	switch x := v.GetValue().(type) {
	case *commonpb.AnyValue_StringValue:
		return x.StringValue
	case *commonpb.AnyValue_IntValue:
		return strconv.FormatInt(x.IntValue, 10)
	case *commonpb.AnyValue_DoubleValue:
		return strconv.FormatFloat(x.DoubleValue, 'g', -1, 64)
	case *commonpb.AnyValue_BoolValue:
		return strconv.FormatBool(x.BoolValue)
	case nil:
		return ""
	}
	b, _ := protojson.Marshal(v)
	return string(b)
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

func containsString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// OTLPMetricsResponse encodes an ExportMetricsServiceResponse in the request's format, with a
// partial_success block when some data points were refused.
func OTLPMetricsResponse(format OTLPFormat, rejected int64, message string) ([]byte, string, error) {
	response := &collmetrics.ExportMetricsServiceResponse{}
	if rejected > 0 {
		response.PartialSuccess = &collmetrics.ExportMetricsPartialSuccess{RejectedDataPoints: rejected, ErrorMessage: message}
	}
	if format == OTLPProtobuf {
		b, err := proto.Marshal(response)
		return b, "application/x-protobuf", err
	}
	b, err := protojson.Marshal(response)
	return b, "application/json", err
}
