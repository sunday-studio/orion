// Package tracestore implements product traces: OTLP span ingest into a separate
// traces.db through a bounded asynchronous writer, a per-trace summary kept by that
// writer, queries and per-app retention. See docs/product-traces.md and
// docs/plans/opentelemetry-support.md.
package tracestore

import (
	"errors"
	"time"
)

// Limits that keep one request, one span and one read bounded.
const (
	// MaxSpansPerRequest keeps one request within what the queue can ever accept.
	MaxSpansPerRequest = 10_000
	// MaxSpanBytes is the encoded size above which one span is rejected.
	MaxSpanBytes = 256 << 10
	// MaxAttributesPerSpan caps the attributes of a span and of its resource; extras are dropped.
	MaxAttributesPerSpan = 128
	// MaxAttributesPerEvent caps the attributes of one event or link.
	MaxAttributesPerEvent = 32
	// MaxEventsPerSpan and MaxLinksPerSpan cap the events and links kept per span.
	MaxEventsPerSpan = 128
	MaxLinksPerSpan  = 128
	// MaxAttributeDepth is how deep array and map attribute values may nest.
	MaxAttributeDepth = 8
	// MaxValueBytes truncates any text value (attribute, event name, status message).
	MaxValueBytes = 4096
	// MaxSpansPerTraceRead bounds what one trace detail read returns.
	MaxSpansPerTraceRead = 5000

	maxFutureSkew = time.Hour
)

// Span status codes, as OTLP numbers them.
const (
	StatusUnset = 0
	StatusOK    = 1
	StatusError = 2
)

// Request-level failures. Per-span problems are SpanErrors instead.
var (
	// ErrUnsupportedMedia is returned for a content type that is not OTLP protobuf or JSON.
	ErrUnsupportedMedia = errors.New("unsupported content type")
	// ErrInvalidBody is returned when the request is not a parseable ExportTraceServiceRequest.
	ErrInvalidBody = errors.New("request body is not a valid ExportTraceServiceRequest")
	// ErrTooManySpans is returned when one request carries more than MaxSpansPerRequest spans.
	ErrTooManySpans = errors.New("too many spans in one request")
	// ErrBadFilter marks a filter the caller got wrong (maps to HTTP 400).
	ErrBadFilter = errors.New("invalid trace filter")
)

// Event is a point-in-time annotation on a span.
type Event struct {
	TimeUnixNano int64          `json:"time_unix_nano"`
	Name         string         `json:"name"`
	Attributes   map[string]any `json:"attributes,omitempty"`
}

// Link points at a span of another (or the same) trace.
type Link struct {
	TraceID    string         `json:"trace_id"`
	SpanID     string         `json:"span_id"`
	Attributes map[string]any `json:"attributes,omitempty"`
}

// Span is one normalised span, ready to store. Ids are lower-case hex.
type Span struct {
	AppID         string
	TraceID       string
	SpanID        string
	ParentSpanID  string // empty for a root span
	Name          string
	Kind          int // OTLP SpanKind number, 0 to 5
	StartNano     int64
	DurationNano  int64
	StatusCode    int
	StatusMessage string
	// ServiceName is the resource's service.name, denormalised for the trace summary and filters.
	ServiceName string
	Attributes  map[string]any
	Resource    map[string]any
	Events      []Event
	Links       []Link
	IngestedAt  time.Time
	Size        int
}

// EndNano is when the span ended.
func (s Span) EndNano() int64 { return s.StartNano + s.DurationNano }

// SpanError explains why one span was rejected.
type SpanError struct {
	Index   int    `json:"index"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *SpanError) Error() string { return e.Code + ": " + e.Message }

// Parsed is the result of reading one ingest request: valid spans plus per-span errors.
type Parsed struct {
	Spans  []Span
	Errors []SpanError
}

// Trace is the stored summary of the spans one app received for a trace id.
type Trace struct {
	AppID       string
	TraceID     string
	RootName    string
	ServiceName string
	Start       time.Time
	DurationNs  int64
	SpanCount   int64
	ErrorCount  int64
	// HasRoot is false while only orphan spans arrived: the name, service and start then come from the earliest span.
	HasRoot bool
}

// KindName is the lower-case OTLP span kind.
func KindName(kind int) string {
	switch kind {
	case 1:
		return "internal"
	case 2:
		return "server"
	case 3:
		return "client"
	case 4:
		return "producer"
	case 5:
		return "consumer"
	}
	return "unspecified"
}

// StatusName is unset, ok or error.
func StatusName(code int) string {
	switch code {
	case StatusOK:
		return "ok"
	case StatusError:
		return "error"
	}
	return "unset"
}

// Redactor rewrites decoded JSON (maps, lists, strings) before storage. logstore.Redactor implements it.
type Redactor interface {
	Redact(sample any) any
}
