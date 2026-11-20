package metricstore

import (
	"fmt"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"

	metricspb "go.opentelemetry.io/proto/otlp/metrics/v1"
)

// numberSpec says how the data points of one gauge or sum metric are stored.
type numberSpec struct {
	name, kind, unit string
	monotonic        bool
	cumulative       bool
	// temporal is set for sums that are stored as per-interval changes: the writer converts a cumulative
	// reading and de-duplicates both. A gauge, and an up-down counter read as a level, are stored as sent.
	temporal bool
}

func (p *parser) rejectPoint(name, reason string) {
	p.reject(1, fmt.Sprintf("metric %q: %s", name, reason))
}

func (p *parser) number(spec numberSpec, resource map[string]string, dp *metricspb.NumberDataPoint) {
	if dp.GetFlags()&uint32(metricspb.DataPointFlags_DATA_POINT_FLAGS_NO_RECORDED_VALUE_MASK) != 0 {
		p.rejectPoint(spec.name, "data point has no recorded value")
		return
	}
	var v float64
	switch x := dp.GetValue().(type) {
	case *metricspb.NumberDataPoint_AsDouble:
		v = x.AsDouble
	case *metricspb.NumberDataPoint_AsInt:
		v = float64(x.AsInt)
	default:
		p.rejectPoint(spec.name, "data point has no value")
		return
	}
	if !finite(v) {
		p.rejectPoint(spec.name, "value is not a finite number")
		return
	}
	if spec.temporal && spec.monotonic && v < 0 {
		p.rejectPoint(spec.name, "a monotonic sum cannot have a negative value")
		return
	}
	at, reason := pointTime(dp.GetTimeUnixNano(), p.now)
	if reason != "" {
		p.rejectPoint(spec.name, reason)
		return
	}
	key, labels := seriesIdentity(spec.name, spec.kind, resource, dp.GetAttributes())
	value := v
	sample := Sample{Key: key, Kind: spec.kind, Name: spec.name, Metric: spec.name, Unit: clip(spec.unit, maxAttributeText), Labels: labels,
		Values: Values{Value: &value}}
	if spec.temporal {
		sample.Temporality = &Temporality{Cumulative: spec.cumulative, Start: startTime(dp.GetStartTimeUnixNano()), At: at}
	}
	p.add(at, sample)
}

// distribution stores a validated histogram or summary as a sample.
func (p *parser) distribution(name, kind, unit string, resource map[string]string, attrs []*commonpb.KeyValue, timeNs, startNs uint64, cumulative bool, h Histogram) {
	at, reason := pointTime(timeNs, p.now)
	if reason != "" {
		p.rejectPoint(name, reason)
		return
	}
	if cumulative {
		h.Min, h.Max = nil, nil // a cumulative min and max cover the whole life of the process, not the interval
	}
	key, labels := seriesIdentity(name, kind, resource, attrs)
	p.add(at, Sample{Key: key, Kind: kind, Name: name, Metric: name, Unit: clip(unit, maxAttributeText), Labels: labels,
		Values:      Values{Histogram: &h},
		Temporality: &Temporality{Cumulative: cumulative, Start: startTime(startNs), At: at}})
}
