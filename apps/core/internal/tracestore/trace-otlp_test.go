package tracestore

import (
	"bytes"
	"encoding/hex"
	"errors"
	"math"
	"orion/core/internal/logstore"
	"strings"
	"testing"
	"time"

	coltrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func strKV(k, v string) *commonpb.KeyValue {
	return &commonpb.KeyValue{Key: k, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: v}}}
}

func intKV(k string, v int64) *commonpb.KeyValue {
	return &commonpb.KeyValue{Key: k, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: v}}}
}

func rawID(n, size int) []byte {
	b := make([]byte, size)
	b[size-1] = byte(n)
	b[0] = 0xab
	return b
}

// okSpan is a valid span that started at epoch and ran 250ms.
func okSpan(n int) *tracepb.Span {
	return &tracepb.Span{
		TraceId: rawID(1, 16), SpanId: rawID(n, 8), Name: "GET /checkout", Kind: tracepb.Span_SPAN_KIND_SERVER,
		StartTimeUnixNano: uint64(epoch.UnixNano()), EndTimeUnixNano: uint64(epoch.Add(250 * time.Millisecond).UnixNano()),
	}
}

func request(spans ...*tracepb.Span) *coltrace.ExportTraceServiceRequest {
	return &coltrace.ExportTraceServiceRequest{ResourceSpans: []*tracepb.ResourceSpans{{
		Resource:   &resourcepb.Resource{Attributes: []*commonpb.KeyValue{strKV("service.name", "checkout-api"), strKV("deployment.environment", "prod")}},
		ScopeSpans: []*tracepb.ScopeSpans{{Scope: &commonpb.InstrumentationScope{Name: "http-server", Version: "1.2.3"}, Spans: spans}},
	}}}
}

func encodings(t *testing.T, req *coltrace.ExportTraceServiceRequest) map[string][]byte {
	t.Helper()
	pb, err := proto.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	js, err := protojson.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	return map[string][]byte{"application/x-protobuf": pb, "application/json": js}
}

func parseOne(t *testing.T, span *tracepb.Span) (Parsed, error) {
	t.Helper()
	parsed, _, err := ParseOTLP("application/x-protobuf", encodings(t, request(span))["application/x-protobuf"], epoch)
	return parsed, err
}

func TestParseMapsEveryFieldInProtobufAndJSON(t *testing.T) {
	span := okSpan(2)
	span.ParentSpanId = rawID(1, 8)
	span.Status = &tracepb.Status{Code: tracepb.Status_STATUS_CODE_ERROR, Message: "card declined"}
	span.Attributes = []*commonpb.KeyValue{strKV("http.method", "POST"), intKV("http.status_code", 502)}
	span.Events = []*tracepb.Span_Event{{TimeUnixNano: uint64(epoch.Add(time.Millisecond).UnixNano()), Name: "exception", Attributes: []*commonpb.KeyValue{strKV("exception.type", "Declined")}}}
	span.Links = []*tracepb.Span_Link{{TraceId: rawID(7, 16), SpanId: rawID(9, 8), Attributes: []*commonpb.KeyValue{strKV("kind", "retry")}}}

	for contentType, body := range encodings(t, request(span)) {
		parsed, format, err := ParseOTLP(contentType+"; charset=utf-8", body, epoch)
		if err != nil || len(parsed.Errors) != 0 || len(parsed.Spans) != 1 {
			t.Fatalf("%s: parsed = %+v, err = %v", contentType, parsed, err)
		}
		if (format == OTLPProtobuf) != (contentType == "application/x-protobuf") {
			t.Fatalf("%s: format = %v", contentType, format)
		}
		got := parsed.Spans[0]
		if got.TraceID != hex.EncodeToString(rawID(1, 16)) || got.SpanID != hex.EncodeToString(rawID(2, 8)) || got.ParentSpanID != hex.EncodeToString(rawID(1, 8)) {
			t.Fatalf("%s: ids = %q %q %q", contentType, got.TraceID, got.SpanID, got.ParentSpanID)
		}
		if got.Name != "GET /checkout" || got.Kind != 2 || got.StartNano != epoch.UnixNano() || got.DurationNano != (250*time.Millisecond).Nanoseconds() ||
			got.StatusCode != StatusError || got.StatusMessage != "card declined" || got.ServiceName != "checkout-api" {
			t.Fatalf("%s: span = %+v", contentType, got)
		}
		if got.Attributes["http.method"] != "POST" || got.Attributes["http.status_code"] != int64(502) ||
			got.Attributes["otel.scope.name"] != "http-server" || got.Attributes["otel.scope.version"] != "1.2.3" {
			t.Fatalf("%s: attributes = %+v", contentType, got.Attributes)
		}
		if got.Resource["deployment.environment"] != "prod" || got.Size <= 0 || !got.IngestedAt.Equal(epoch) {
			t.Fatalf("%s: resource/size = %+v %d", contentType, got.Resource, got.Size)
		}
		if len(got.Events) != 1 || got.Events[0].Name != "exception" || got.Events[0].Attributes["exception.type"] != "Declined" || got.Events[0].TimeUnixNano != epoch.Add(time.Millisecond).UnixNano() {
			t.Fatalf("%s: events = %+v", contentType, got.Events)
		}
		if len(got.Links) != 1 || got.Links[0].TraceID != hex.EncodeToString(rawID(7, 16)) || got.Links[0].Attributes["kind"] != "retry" {
			t.Fatalf("%s: links = %+v", contentType, got.Links)
		}
	}
}

func TestParseRejectsEachBadSpanAndKeepsTheRest(t *testing.T) {
	mutate := func(f func(s *tracepb.Span)) *tracepb.Span {
		s := okSpan(5)
		f(s)
		return s
	}
	deep := &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "x"}}
	for i := 0; i < MaxAttributeDepth+1; i++ {
		deep = &commonpb.AnyValue{Value: &commonpb.AnyValue_ArrayValue{ArrayValue: &commonpb.ArrayValue{Values: []*commonpb.AnyValue{deep}}}}
	}
	cases := []struct {
		name string
		span *tracepb.Span
		code string
	}{
		{"missing trace id", mutate(func(s *tracepb.Span) { s.TraceId = nil }), "invalid_trace_id"},
		{"short trace id", mutate(func(s *tracepb.Span) { s.TraceId = rawID(1, 8) }), "invalid_trace_id"},
		{"all-zero trace id", mutate(func(s *tracepb.Span) { s.TraceId = make([]byte, 16) }), "invalid_trace_id"},
		{"missing span id", mutate(func(s *tracepb.Span) { s.SpanId = nil }), "invalid_span_id"},
		{"long span id", mutate(func(s *tracepb.Span) { s.SpanId = rawID(1, 16) }), "invalid_span_id"},
		{"all-zero span id", mutate(func(s *tracepb.Span) { s.SpanId = make([]byte, 8) }), "invalid_span_id"},
		{"malformed parent id", mutate(func(s *tracepb.Span) { s.ParentSpanId = rawID(1, 5) }), "invalid_parent_span_id"},
		{"no start time", mutate(func(s *tracepb.Span) { s.StartTimeUnixNano = 0 }), "invalid_timestamp"},
		{"no end time", mutate(func(s *tracepb.Span) { s.EndTimeUnixNano = 0 }), "invalid_timestamp"},
		{"ends before it starts", mutate(func(s *tracepb.Span) { s.EndTimeUnixNano = s.StartTimeUnixNano - 1 }), "invalid_timestamp"},
		{"start beyond int64", mutate(func(s *tracepb.Span) { s.StartTimeUnixNano = math.MaxUint64 }), "invalid_timestamp"},
		{"starts in the future", mutate(func(s *tracepb.Span) {
			s.StartTimeUnixNano = uint64(epoch.Add(2 * time.Hour).UnixNano())
			s.EndTimeUnixNano = s.StartTimeUnixNano + 1
		}), "invalid_timestamp"},
		{"too large", mutate(func(s *tracepb.Span) { s.Attributes = hugeAttributes() }), "span_too_large"},
		{"attributes nested too deep", mutate(func(s *tracepb.Span) {
			s.Attributes = []*commonpb.KeyValue{{Key: "deep", Value: deep}}
		}), "attributes_too_deep"},
		{"events nested too deep", mutate(func(s *tracepb.Span) {
			s.Events = []*tracepb.Span_Event{{Name: "e", Attributes: []*commonpb.KeyValue{{Key: "deep", Value: deep}}}}
		}), "attributes_too_deep"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Good spans around the bad one are kept, and the error says which index failed.
			parsed, _, err := ParseOTLP("application/x-protobuf", encodings(t, request(okSpan(1), tc.span, okSpan(3)))["application/x-protobuf"], epoch)
			if err != nil {
				t.Fatal(err)
			}
			if len(parsed.Spans) != 2 || len(parsed.Errors) != 1 {
				t.Fatalf("spans = %d, errors = %+v", len(parsed.Spans), parsed.Errors)
			}
			if e := parsed.Errors[0]; e.Code != tc.code || e.Index != 1 || e.Message == "" {
				t.Fatalf("error = %+v, want code %s at index 1", e, tc.code)
			}
		})
	}
}

func hugeAttributes() []*commonpb.KeyValue {
	var kvs []*commonpb.KeyValue
	for i := 0; i < 80; i++ { // 80 x ~4 KiB of text is over the 256 KiB span limit even before truncation
		kvs = append(kvs, strKV("k"+strings.Repeat("x", i), strings.Repeat("v", 4000)))
	}
	return kvs
}

func TestParseRejectsTheWholeRequestWhenItCannotBeRead(t *testing.T) {
	valid := encodings(t, request(okSpan(1)))
	if _, _, err := ParseOTLP("text/plain", valid["application/json"], epoch); !errors.Is(err, ErrUnsupportedMedia) {
		t.Fatalf("text/plain: %v, want ErrUnsupportedMedia", err)
	}
	if _, _, err := ParseOTLP("", valid["application/json"], epoch); !errors.Is(err, ErrUnsupportedMedia) {
		t.Fatalf("no content type: %v, want ErrUnsupportedMedia", err)
	}
	if _, _, err := ParseOTLP("application/json", []byte(`{"resourceSpans": [`), epoch); !errors.Is(err, ErrInvalidBody) {
		t.Fatalf("broken json: %v, want ErrInvalidBody", err)
	}
	if _, _, err := ParseOTLP("application/x-protobuf", []byte{0xff, 0xff, 0xff, 0xff, 0x01}, epoch); !errors.Is(err, ErrInvalidBody) {
		t.Fatalf("garbage protobuf: %v, want ErrInvalidBody", err)
	}
	// The JSON of a different signal is not an error: unknown fields are ignored and nothing is stored.
	if parsed, _, err := ParseOTLP("application/json", []byte(`{"resourceLogs": []}`), epoch); err != nil || len(parsed.Spans) != 0 {
		t.Fatalf("logs request = %+v, %v", parsed, err)
	}
}

func TestParseCapsSpansPerRequest(t *testing.T) {
	spans := make([]*tracepb.Span, MaxSpansPerRequest)
	for i := range spans {
		spans[i] = okSpan(1)
	}
	if parsed, _, err := ParseOTLP("application/x-protobuf", encodings(t, request(spans...))["application/x-protobuf"], epoch); err != nil || len(parsed.Spans) != MaxSpansPerRequest {
		t.Fatalf("exactly the limit: %d spans, %v", len(parsed.Spans), err)
	}
	over := append(spans, okSpan(1))
	if _, _, err := ParseOTLP("application/x-protobuf", encodings(t, request(over...))["application/x-protobuf"], epoch); !errors.Is(err, ErrTooManySpans) {
		t.Fatalf("one over the limit: %v, want ErrTooManySpans", err)
	}
}

func TestParseTreatsUnusualButValidSpansGently(t *testing.T) {
	span := okSpan(1)
	span.Name = "  "
	span.ParentSpanId = make([]byte, 8) // an all-zero parent means no parent
	span.Kind = 99
	span.Status = &tracepb.Status{Code: 42}
	parsed, err := parseOne(t, span)
	if err != nil || len(parsed.Spans) != 1 || len(parsed.Errors) != 0 {
		t.Fatalf("parsed = %+v, %v", parsed, err)
	}
	got := parsed.Spans[0]
	if got.Name != "(unnamed)" || got.ParentSpanID != "" || got.Kind != 0 || got.StatusCode != StatusUnset {
		t.Fatalf("span = %+v", got)
	}
}

func TestParseTruncatesInsteadOfRejectingOversizedParts(t *testing.T) {
	span := okSpan(1)
	span.Attributes = []*commonpb.KeyValue{strKV("long", strings.Repeat("é", MaxValueBytes)), {Key: "nan", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_DoubleValue{DoubleValue: math.NaN()}}}}
	for i := 0; i < MaxAttributesPerSpan+10; i++ {
		span.Attributes = append(span.Attributes, intKV("n"+hex.EncodeToString([]byte{byte(i)}), int64(i)))
	}
	for i := 0; i < MaxEventsPerSpan+5; i++ {
		span.Events = append(span.Events, &tracepb.Span_Event{Name: "e"})
	}
	for i := 0; i < MaxLinksPerSpan+3; i++ {
		span.Links = append(span.Links, &tracepb.Span_Link{TraceId: rawID(1, 16), SpanId: rawID(1, 8)})
	}
	parsed, err := parseOne(t, span)
	if err != nil || len(parsed.Spans) != 1 {
		t.Fatalf("parsed = %+v, %v", parsed, err)
	}
	got := parsed.Spans[0]
	long, _ := got.Attributes["long"].(string)
	if len(long) > MaxValueBytes || len(long) < MaxValueBytes-1 || !strings.HasPrefix(long, "é") || strings.ToValidUTF8(long, "") != long {
		t.Fatalf("long value is %d bytes (valid utf-8: %v), want it cut to at most %d on a character boundary", len(long), strings.ToValidUTF8(long, "") == long, MaxValueBytes)
	}
	if got.Attributes["nan"] != "NaN" {
		t.Fatalf("a NaN double must survive as text, got %v", got.Attributes["nan"])
	}
	if len(got.Events) != MaxEventsPerSpan || len(got.Links) != MaxLinksPerSpan {
		t.Fatalf("events %d, links %d, want both capped", len(got.Events), len(got.Links))
	}
	// 128 kept attributes + scope name + scope version + the drop markers.
	if got.Attributes["orion.dropped_events"] != int64(5) || got.Attributes["orion.dropped_links"] != int64(3) || got.Attributes["orion.truncated_values"] != int64(1) || got.Attributes["orion.dropped_attributes"] != int64(12) {
		t.Fatalf("drop markers = %+v", got.Attributes)
	}
	// The attribute map must be storable: a NaN left as a number would make json.Marshal fail and lose them all.
	if encodeJSON(got.Attributes) == "" {
		t.Fatal("attributes cannot be encoded as JSON")
	}
}

func TestResponseMirrorsTheRequestFormat(t *testing.T) {
	errs := []SpanError{{Index: 1, Code: "invalid_span_id", Message: "bad id"}}
	for _, format := range []OTLPFormat{OTLPProtobuf, OTLPJSON} {
		body, contentType, err := OTLPResponse(format, 1, errs)
		if err != nil {
			t.Fatal(err)
		}
		out := &coltrace.ExportTraceServiceResponse{}
		if format == OTLPProtobuf {
			err = proto.Unmarshal(body, out)
			if contentType != "application/x-protobuf" {
				t.Fatalf("content type = %s", contentType)
			}
		} else {
			err = protojson.Unmarshal(body, out)
			if contentType != "application/json" {
				t.Fatalf("content type = %s", contentType)
			}
		}
		if err != nil || out.GetPartialSuccess().GetRejectedSpans() != 1 || !strings.Contains(out.GetPartialSuccess().GetErrorMessage(), "span 1: bad id") {
			t.Fatalf("format %v: %+v, %v", format, out, err)
		}
	}
	// Nothing rejected: an empty response, as the OTLP spec asks.
	body, _, _ := OTLPResponse(OTLPProtobuf, 0, nil)
	if len(body) != 0 {
		t.Fatalf("clean protobuf response = %x, want empty", body)
	}
	body, _, _ = OTLPResponse(OTLPJSON, 0, nil)
	if !bytes.Equal(body, []byte("{}")) {
		t.Fatalf("clean json response = %s, want {}", body)
	}
	// The reason list is bounded.
	many := make([]SpanError, 20)
	for i := range many {
		many[i] = SpanError{Index: i, Message: "m"}
	}
	body, _, _ = OTLPResponse(OTLPJSON, 20, many)
	out := &coltrace.ExportTraceServiceResponse{}
	_ = protojson.Unmarshal(body, out)
	if got := strings.Count(out.GetPartialSuccess().GetErrorMessage(), "span "); got != 5 {
		t.Fatalf("message lists %d reasons, want 5", got)
	}
}

func TestRedactHidesSecretsEverywhereASpanCarriesThem(t *testing.T) {
	span := okSpan(1)
	span.Attributes = []*commonpb.KeyValue{strKV("http.request.header.authorization", "Bearer abc"), strKV("http.route", "/pay")}
	span.Events = []*tracepb.Span_Event{{Name: "login", Attributes: []*commonpb.KeyValue{strKV("password", "hunter2")}}}
	span.Links = []*tracepb.Span_Link{{TraceId: rawID(1, 16), SpanId: rawID(1, 8), Attributes: []*commonpb.KeyValue{strKV("api_key", "k")}}}
	req := request(span)
	req.ResourceSpans[0].Resource.Attributes = append(req.ResourceSpans[0].Resource.Attributes, strKV("db.password", "pw"))
	parsed, _, err := ParseOTLP("application/json", encodings(t, req)["application/json"], epoch)
	if err != nil {
		t.Fatal(err)
	}
	rules := logstore.NewRedactorRules(nil, false, []logstore.Rule{{Type: "regex", Pattern: `card \d+`, ReplaceWith: "card ****"}})
	parsed.Spans[0].StatusMessage = "declined card 4242"
	Redact(parsed.Spans, rules)
	got := parsed.Spans[0]
	if got.Attributes["http.request.header.authorization"] != "[redacted]" || got.Attributes["http.route"] != "/pay" {
		t.Fatalf("attributes = %+v", got.Attributes)
	}
	if got.Events[0].Attributes["password"] != "[redacted]" || got.Links[0].Attributes["api_key"] != "[redacted]" || got.Resource["db.password"] != "[redacted]" {
		t.Fatalf("events/links/resource = %+v %+v %+v", got.Events, got.Links, got.Resource)
	}
	if got.StatusMessage != "declined card ****" {
		t.Fatalf("status message = %q", got.StatusMessage)
	}
}
