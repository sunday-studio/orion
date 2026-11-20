package tracestore

import (
	"encoding/hex"
	"fmt"
	"math"
	"strings"
	"time"

	coltrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// OTLPFormat is the wire encoding of an OTLP/HTTP request, which the response mirrors.
type OTLPFormat int

const (
	OTLPJSON OTLPFormat = iota
	OTLPProtobuf
)

// ParseOTLP decodes an OTLP/HTTP ExportTraceServiceRequest (protobuf or JSON) and maps every span
// onto the stored shape. A span that breaks a rule is reported in Parsed.Errors and skipped; only a
// request that cannot be read at all, or that carries too many spans, is an error.
func ParseOTLP(contentType string, body []byte, now time.Time) (Parsed, OTLPFormat, error) {
	request, format, err := DecodeOTLP(contentType, body)
	if err != nil {
		return Parsed{}, format, err
	}
	parsed, err := ParseOTLPRequest(request, now)
	return parsed, format, err
}

// DecodeOTLP reads an OTLP/HTTP body (protobuf or JSON) into the request message.
func DecodeOTLP(contentType string, body []byte) (*coltrace.ExportTraceServiceRequest, OTLPFormat, error) {
	mediaType := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	request := &coltrace.ExportTraceServiceRequest{}
	var format OTLPFormat
	switch mediaType {
	case "application/x-protobuf", "application/protobuf":
		format = OTLPProtobuf
		if err := proto.Unmarshal(body, request); err != nil {
			return nil, format, fmt.Errorf("%w: %v", ErrInvalidBody, err)
		}
	case "application/json":
		format = OTLPJSON
		if err := (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(body, request); err != nil {
			return nil, format, fmt.Errorf("%w: %v", ErrInvalidBody, err)
		}
	default:
		return nil, OTLPJSON, ErrUnsupportedMedia
	}
	return request, format, nil
}

// ParseOTLPRequest maps an already decoded request onto stored spans. OTLP/HTTP and OTLP/gRPC share it.
func ParseOTLPRequest(request *coltrace.ExportTraceServiceRequest, now time.Time) (Parsed, error) {
	total := 0
	for _, rs := range request.GetResourceSpans() {
		for _, ss := range rs.GetScopeSpans() {
			total += len(ss.GetSpans())
		}
	}
	if total > MaxSpansPerRequest {
		return Parsed{}, ErrTooManySpans
	}

	var result Parsed
	index := 0
	for _, rs := range request.GetResourceSpans() {
		var lim limits
		resource, _ := lim.attributes(rs.GetResource().GetAttributes(), MaxAttributesPerSpan)
		service, _ := resource["service.name"].(string)
		for _, ss := range rs.GetScopeSpans() {
			for _, raw := range ss.GetSpans() {
				span, spanErr := convertSpan(raw, ss.GetScope(), resource, service, now)
				if spanErr != nil {
					spanErr.Index = index
					result.Errors = append(result.Errors, *spanErr)
				} else {
					result.Spans = append(result.Spans, span)
				}
				index++
			}
		}
	}
	return result, nil
}

// convertSpan validates one OTLP span and builds the stored shape.
func convertSpan(raw *tracepb.Span, scope *commonpb.InstrumentationScope, resource map[string]any, service string, now time.Time) (Span, *SpanError) {
	size := proto.Size(raw)
	if size > MaxSpanBytes {
		return Span{}, &SpanError{Code: "span_too_large", Message: fmt.Sprintf("span is %d bytes, the limit is %d", size, MaxSpanBytes)}
	}
	if id := raw.GetTraceId(); len(id) != 16 || allZero(id) {
		return Span{}, &SpanError{Code: "invalid_trace_id", Message: "trace_id must be 16 bytes and not all zero"}
	}
	if id := raw.GetSpanId(); len(id) != 8 || allZero(id) {
		return Span{}, &SpanError{Code: "invalid_span_id", Message: "span_id must be 8 bytes and not all zero"}
	}
	parent := ""
	if id := raw.GetParentSpanId(); len(id) > 0 {
		if len(id) != 8 {
			return Span{}, &SpanError{Code: "invalid_parent_span_id", Message: "parent_span_id must be empty or 8 bytes"}
		}
		if !allZero(id) {
			parent = hex.EncodeToString(id)
		}
	}
	start, end := raw.GetStartTimeUnixNano(), raw.GetEndTimeUnixNano()
	switch {
	case start == 0 || start > math.MaxInt64:
		return Span{}, &SpanError{Code: "invalid_timestamp", Message: "span has no valid start time"}
	case end == 0 || end > math.MaxInt64:
		return Span{}, &SpanError{Code: "invalid_timestamp", Message: "span has no valid end time"}
	case end < start:
		return Span{}, &SpanError{Code: "invalid_timestamp", Message: "span ends before it starts"}
	case int64(start) > now.Add(maxFutureSkew).UnixNano():
		return Span{}, &SpanError{Code: "invalid_timestamp", Message: "span starts more than 1 hour in the future"}
	}

	var lim limits
	attributes, keys := lim.attributes(raw.GetAttributes(), MaxAttributesPerSpan)
	if lim.tooDeep {
		return Span{}, &SpanError{Code: "attributes_too_deep", Message: fmt.Sprintf("attribute values are nested more than %d levels", MaxAttributeDepth)}
	}
	droppedAttrs := max(len(raw.GetAttributes())-keys, 0)
	events, droppedEvents := lim.events(raw.GetEvents())
	links, droppedLinks := lim.links(raw.GetLinks())
	if lim.tooDeep {
		return Span{}, &SpanError{Code: "attributes_too_deep", Message: fmt.Sprintf("attribute values are nested more than %d levels", MaxAttributeDepth)}
	}
	if attributes == nil {
		attributes = map[string]any{}
	}
	if scope.GetName() != "" {
		attributes["otel.scope.name"] = lim.text(scope.GetName())
	}
	if scope.GetVersion() != "" {
		attributes["otel.scope.version"] = lim.text(scope.GetVersion())
	}
	for key, n := range map[string]int{"orion.dropped_attributes": droppedAttrs, "orion.dropped_events": droppedEvents, "orion.dropped_links": droppedLinks, "orion.truncated_values": lim.truncated} {
		if n > 0 {
			attributes[key] = int64(n)
		}
	}

	name := strings.TrimSpace(raw.GetName())
	if name == "" {
		name = "(unnamed)"
	}
	status := raw.GetStatus()
	code := int(status.GetCode())
	if code < StatusUnset || code > StatusError {
		code = StatusUnset
	}
	kind := int(raw.GetKind())
	if kind < 0 || kind > 5 {
		kind = 0
	}
	return Span{
		TraceID: hex.EncodeToString(raw.GetTraceId()), SpanID: hex.EncodeToString(raw.GetSpanId()), ParentSpanID: parent,
		Name: lim.text(name), Kind: kind, StartNano: int64(start), DurationNano: int64(end - start),
		StatusCode: code, StatusMessage: lim.text(status.GetMessage()), ServiceName: lim.text(service),
		Attributes: attributes, Resource: resource, Events: events, Links: links, IngestedAt: now, Size: size,
	}, nil
}

func allZero(b []byte) bool {
	for _, v := range b {
		if v != 0 {
			return false
		}
	}
	return true
}

// OTLPResponse encodes the ExportTraceServiceResponse for the given format, with a partial_success
// block when some spans were rejected (per the OTLP spec a fully accepted request has an empty
// response). The message lists up to five reasons.
func OTLPResponse(format OTLPFormat, rejected int, errs []SpanError) ([]byte, string, error) {
	response := OTLPResponseMessage(rejected, errs)
	if format == OTLPProtobuf {
		b, err := proto.Marshal(response)
		return b, "application/x-protobuf", err
	}
	b, err := protojson.Marshal(response)
	return b, "application/json", err
}

// OTLPResponseMessage builds the ExportTraceServiceResponse, the same for OTLP/HTTP and OTLP/gRPC.
func OTLPResponseMessage(rejected int, errs []SpanError) *coltrace.ExportTraceServiceResponse {
	response := &coltrace.ExportTraceServiceResponse{}
	if rejected > 0 {
		messages := make([]string, 0, len(errs))
		for _, e := range errs {
			if len(messages) == 5 {
				break
			}
			messages = append(messages, fmt.Sprintf("span %d: %s", e.Index, e.Message))
		}
		response.PartialSuccess = &coltrace.ExportTracePartialSuccess{RejectedSpans: int64(rejected), ErrorMessage: strings.Join(messages, "; ")}
	}
	return response
}
