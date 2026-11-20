package tracestore

import (
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

// otlpJSONTraces is an ExportTraceServiceRequest as an OpenTelemetry SDK or the Collector
// writes it with the http/json protocol: lowerCamelCase keys, hex trace and span ids,
// 64-bit values as strings, enums as numbers or names and attribute value oneofs.
// The ids and the shape follow the examples of the OTLP specification.
const otlpJSONTraces = `{
  "resourceSpans": [{
    "resource": {"attributes": [
      {"key": "service.name", "value": {"stringValue": "checkout-api"}},
      {"key": "telemetry.sdk.language", "value": {"stringValue": "nodejs"}}
    ]},
    "scopeSpans": [{
      "scope": {"name": "http-server", "version": "1.2.3"},
      "spans": [{
        "traceId": "5b8efff798038103d269b633813fc60c",
        "spanId": "eee19b7ec3c1b174",
        "parentSpanId": "eee19b7ec3c1b173",
        "name": "GET /checkout",
        "kind": 2,
        "startTimeUnixNano": "1544712660000000000",
        "endTimeUnixNano": "1544712661000000000",
        "attributes": [
          {"key": "http.status_code", "value": {"intValue": "502"}},
          {"key": "http.method", "value": {"stringValue": "GET"}},
          {"key": "retry", "value": {"boolValue": false}}
        ],
        "events": [{"timeUnixNano": "1544712660500000000", "name": "exception", "attributes": [{"key": "exception.type", "value": {"stringValue": "Declined"}}]}],
        "links": [{"traceId": "5b8efff798038103d269b633813fc60d", "spanId": "eee19b7ec3c1b175", "attributes": []}],
        "status": {"message": "card declined", "code": 2}
      }, {
        "traceId": "5b8efff798038103d269b633813fc60c",
        "spanId": "EEE19B7EC3C1B173",
        "parentSpanId": "",
        "name": "root",
        "kind": "SPAN_KIND_SERVER",
        "startTimeUnixNano": "1544712659000000000",
        "endTimeUnixNano": "1544712662000000000",
        "status": {"code": "STATUS_CODE_OK"}
      }]
    }]
  }]
}`

func TestParseReadsHexIdsOfOTLPJSON(t *testing.T) {
	parsed, format, err := ParseOTLP("application/json", []byte(otlpJSONTraces), epoch)
	if err != nil || format != OTLPJSON || len(parsed.Errors) != 0 || len(parsed.Spans) != 2 {
		t.Fatalf("parsed = %+v, format = %v, err = %v", parsed, format, err)
	}
	child, root := parsed.Spans[0], parsed.Spans[1]
	if child.TraceID != "5b8efff798038103d269b633813fc60c" || child.SpanID != "eee19b7ec3c1b174" || child.ParentSpanID != "eee19b7ec3c1b173" {
		t.Fatalf("child ids = %q %q %q", child.TraceID, child.SpanID, child.ParentSpanID)
	}
	if root.SpanID != "eee19b7ec3c1b173" || root.ParentSpanID != "" || root.Kind != 2 || root.StatusCode != StatusOK {
		t.Fatalf("root = %+v (an uppercase hex id is the same id, an empty parent means none)", root)
	}
	if len(child.Links) != 1 || child.Links[0].TraceID != "5b8efff798038103d269b633813fc60d" || child.Links[0].SpanID != "eee19b7ec3c1b175" {
		t.Fatalf("links = %+v", child.Links)
	}
	if child.Name != "GET /checkout" || child.StatusCode != StatusError || child.StatusMessage != "card declined" ||
		child.Attributes["http.status_code"] != int64(502) || child.ServiceName != "checkout-api" || len(child.Events) != 1 {
		t.Fatalf("child = %+v", child)
	}
}

func TestParseKeepsReadingBase64IdsOfOTLPJSON(t *testing.T) {
	// What protojson itself writes, and what Core accepted before hex ids were read.
	b64 := func(hexID string) string {
		raw := make([]byte, len(hexID)/2)
		for i := range raw {
			var v byte
			for _, c := range hexID[2*i : 2*i+2] {
				v = v<<4 | byte(strings.IndexRune("0123456789abcdef", c))
			}
			raw[i] = v
		}
		return base64.StdEncoding.EncodeToString(raw)
	}
	body := strings.NewReplacer(
		"5b8efff798038103d269b633813fc60c", b64("5b8efff798038103d269b633813fc60c"),
		"5b8efff798038103d269b633813fc60d", b64("5b8efff798038103d269b633813fc60d"),
		"eee19b7ec3c1b174", b64("eee19b7ec3c1b174"),
		"eee19b7ec3c1b175", b64("eee19b7ec3c1b175"),
		"eee19b7ec3c1b173", b64("eee19b7ec3c1b173"),
		"EEE19B7EC3C1B173", b64("eee19b7ec3c1b173"),
	).Replace(otlpJSONTraces)
	if body == otlpJSONTraces || strings.Contains(body, "5b8efff7") {
		t.Fatalf("test body was not converted")
	}
	parsed, _, err := ParseOTLP("application/json", []byte(body), epoch)
	if err != nil || len(parsed.Errors) != 0 || len(parsed.Spans) != 2 {
		t.Fatalf("parsed = %+v, err = %v", parsed, err)
	}
	if parsed.Spans[0].TraceID != "5b8efff798038103d269b633813fc60c" || parsed.Spans[0].ParentSpanID != "eee19b7ec3c1b173" || parsed.Spans[1].SpanID != "eee19b7ec3c1b173" {
		t.Fatalf("ids = %+v", parsed.Spans)
	}
}

func TestParseStillRejectsMalformedIdsOfOTLPJSON(t *testing.T) {
	bad := func(id string) string {
		return strings.Replace(otlpJSONTraces, `"spanId": "eee19b7ec3c1b174"`, `"spanId": "`+id+`"`, 1)
	}
	// A value with characters outside hex and base64: the request does not decode.
	if _, _, err := ParseOTLP("application/json", []byte(bad("eee19b7ec3c1b1!!")), epoch); !errors.Is(err, ErrInvalidBody) {
		t.Fatalf("err = %v, want ErrInvalidBody", err)
	}
	// 16 characters that are not hex read as base64 of 12 bytes: that span is refused, the other kept.
	parsed, _, err := ParseOTLP("application/json", []byte(bad("xyz19b7ec3c1b174")), epoch)
	if err != nil || len(parsed.Errors) != 1 || parsed.Errors[0].Code != "invalid_span_id" || len(parsed.Spans) != 1 {
		t.Fatalf("parsed = %+v, err = %v", parsed, err)
	}
}
