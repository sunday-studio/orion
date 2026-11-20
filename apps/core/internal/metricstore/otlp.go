package metricstore

import (
	"errors"
	"fmt"
	"strings"
	"time"

	collmetrics "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
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
	// MaxPointsPerRequest bounds one OTLP request, the same ceiling as log records. Every data point
	// counts as one, whatever its type.
	MaxPointsPerRequest = 10_000
	maxMetricNameLen    = 200
	maxAttributes       = 20
	maxAttributeText    = 128
	maxRejectMessages   = 5
	// How far from now a data point may be: older ones are no use to a 7 day history, later ones are clock errors.
	maxPointAge  = 24 * time.Hour
	maxPointSkew = 5 * time.Minute
)

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

// parser collects the data points of one request.
type parser struct {
	now time.Time
	out OTLPParsed
}

func (p *parser) reject(n int64, reason string) {
	if n <= 0 {
		return
	}
	p.out.Rejected += n
	if len(p.out.Errors) < maxRejectMessages && !containsString(p.out.Errors, reason) {
		p.out.Errors = append(p.out.Errors, reason)
	}
}

func (p *parser) add(at time.Time, s Sample) {
	p.out.Points = append(p.out.Points, TimedSample{At: at, Sample: s})
}

// ParseOTLPMetrics decodes an OTLP/HTTP ExportMetricsServiceRequest (protobuf or JSON). Gauges, sums,
// histograms (explicit and exponential) and summaries become samples; a data point that cannot be stored
// (bad bounds, counts that do not add up, NaN or infinity, a timestamp out of range) is counted in
// Rejected with its reason and never fails the request. It is pure: cumulative readings are turned into
// per-interval changes later, by the writer, which owns the per-series state.
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

	p := &parser{now: now}
	for _, rm := range request.GetResourceMetrics() {
		resource := resourceLabels(rm.GetResource().GetAttributes())
		for _, sm := range rm.GetScopeMetrics() {
			for _, m := range sm.GetMetrics() {
				p.metric(resource, m)
			}
		}
	}
	return p.out, format, nil
}

func (p *parser) metric(resource map[string]string, m *metricspb.Metric) {
	name := strings.TrimSpace(m.GetName())
	if name == "" || len(name) > maxMetricNameLen {
		p.reject(int64(pointCount(m)), "metric name must be 1 to 200 characters")
		return
	}
	switch data := m.GetData().(type) {
	case *metricspb.Metric_Gauge:
		for _, dp := range data.Gauge.GetDataPoints() {
			p.number(numberSpec{name: name, kind: KindGauge, unit: m.GetUnit()}, resource, dp)
		}
	case *metricspb.Metric_Sum:
		sum := data.Sum
		cumulative := sum.GetAggregationTemporality() != metricspb.AggregationTemporality_AGGREGATION_TEMPORALITY_DELTA
		spec := numberSpec{name: name, kind: KindSum, unit: m.GetUnit(), monotonic: sum.GetIsMonotonic(), cumulative: cumulative, temporal: true}
		if !spec.monotonic && cumulative {
			// An up-down counter reads as a level (queue depth, open connections): keep it as a gauge.
			spec.kind, spec.temporal = KindGauge, false
		}
		for _, dp := range sum.GetDataPoints() {
			p.number(spec, resource, dp)
		}
	case *metricspb.Metric_Histogram:
		cumulative := data.Histogram.GetAggregationTemporality() != metricspb.AggregationTemporality_AGGREGATION_TEMPORALITY_DELTA
		for _, dp := range data.Histogram.GetDataPoints() {
			p.explicitHistogram(name, m.GetUnit(), resource, cumulative, dp)
		}
	case *metricspb.Metric_ExponentialHistogram:
		cumulative := data.ExponentialHistogram.GetAggregationTemporality() != metricspb.AggregationTemporality_AGGREGATION_TEMPORALITY_DELTA
		for _, dp := range data.ExponentialHistogram.GetDataPoints() {
			p.exponentialHistogram(name, m.GetUnit(), resource, cumulative, dp)
		}
	case *metricspb.Metric_Summary:
		for _, dp := range data.Summary.GetDataPoints() {
			p.summary(name, m.GetUnit(), resource, dp)
		}
	}
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
