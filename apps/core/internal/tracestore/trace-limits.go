package tracestore

import (
	"encoding/base64"
	"encoding/hex"
	"math"
	"strconv"
	"unicode/utf8"

	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

// limits converts OTLP values into plain JSON-ready values while enforcing the attribute limits.
// It counts what it had to cut so the span can say so, and notes values nested too deep.
type limits struct {
	truncated int  // text values cut to MaxValueBytes
	tooDeep   bool // a value nested deeper than MaxAttributeDepth
}

// text cuts s to MaxValueBytes on a character boundary.
func (l *limits) text(s string) string {
	if len(s) <= MaxValueBytes {
		return s
	}
	l.truncated++
	cut := MaxValueBytes
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}

// attributes keeps the first max pairs, in the order sent, and returns them with how many it kept.
func (l *limits) attributes(kvs []*commonpb.KeyValue, max int) (map[string]any, int) {
	if len(kvs) == 0 {
		return nil, 0
	}
	out := make(map[string]any, min(len(kvs), max))
	kept := 0
	for _, kv := range kvs {
		if kept >= max {
			break
		}
		if kv.GetKey() == "" {
			continue
		}
		out[l.text(kv.GetKey())] = l.value(kv.GetValue(), 1)
		kept++
	}
	return out, kept
}

func (l *limits) value(v *commonpb.AnyValue, depth int) any {
	if v == nil {
		return nil
	}
	if depth > MaxAttributeDepth {
		l.tooDeep = true
		return nil
	}
	switch value := v.GetValue().(type) {
	case *commonpb.AnyValue_StringValue:
		return l.text(value.StringValue)
	case *commonpb.AnyValue_BoolValue:
		return value.BoolValue
	case *commonpb.AnyValue_IntValue:
		return value.IntValue
	case *commonpb.AnyValue_DoubleValue:
		// JSON has no NaN or infinity; one such value must not lose the whole attribute map.
		if math.IsNaN(value.DoubleValue) || math.IsInf(value.DoubleValue, 0) {
			return strconv.FormatFloat(value.DoubleValue, 'g', -1, 64)
		}
		return value.DoubleValue
	case *commonpb.AnyValue_BytesValue:
		return l.text(base64.StdEncoding.EncodeToString(value.BytesValue))
	case *commonpb.AnyValue_ArrayValue:
		items := make([]any, 0, min(len(value.ArrayValue.GetValues()), MaxAttributesPerSpan))
		for _, item := range value.ArrayValue.GetValues() {
			if len(items) >= MaxAttributesPerSpan {
				break
			}
			items = append(items, l.value(item, depth+1))
		}
		return items
	case *commonpb.AnyValue_KvlistValue:
		nested := make(map[string]any, min(len(value.KvlistValue.GetValues()), MaxAttributesPerSpan))
		for _, kv := range value.KvlistValue.GetValues() {
			if len(nested) >= MaxAttributesPerSpan {
				break
			}
			if kv.GetKey() != "" {
				nested[l.text(kv.GetKey())] = l.value(kv.GetValue(), depth+1)
			}
		}
		return nested
	}
	return nil
}

// events keeps the first MaxEventsPerSpan events and returns how many it dropped.
func (l *limits) events(raw []*tracepb.Span_Event) ([]Event, int) {
	if len(raw) == 0 {
		return nil, 0
	}
	out := make([]Event, 0, min(len(raw), MaxEventsPerSpan))
	for _, e := range raw {
		if len(out) >= MaxEventsPerSpan {
			break
		}
		attrs, _ := l.attributes(e.GetAttributes(), MaxAttributesPerEvent)
		out = append(out, Event{TimeUnixNano: clampNano(e.GetTimeUnixNano()), Name: l.text(e.GetName()), Attributes: attrs})
	}
	return out, len(raw) - len(out)
}

// links keeps the first MaxLinksPerSpan links and returns how many it dropped.
func (l *limits) links(raw []*tracepb.Span_Link) ([]Link, int) {
	if len(raw) == 0 {
		return nil, 0
	}
	out := make([]Link, 0, min(len(raw), MaxLinksPerSpan))
	for _, link := range raw {
		if len(out) >= MaxLinksPerSpan {
			break
		}
		attrs, _ := l.attributes(link.GetAttributes(), MaxAttributesPerEvent)
		out = append(out, Link{TraceID: hex.EncodeToString(link.GetTraceId()), SpanID: hex.EncodeToString(link.GetSpanId()), Attributes: attrs})
	}
	return out, len(raw) - len(out)
}

func clampNano(v uint64) int64 {
	if v > math.MaxInt64 {
		return 0
	}
	return int64(v)
}
