package logstore

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

// otlpJSONLogs is an ExportLogsServiceRequest as an OpenTelemetry SDK or the Collector writes it
// with the http/json protocol (see the OTLP specification): hex trace and span ids, 64-bit
// values as strings, severity as a number or an enum name, attribute value oneofs.
func otlpJSONLogs(at time.Time, traceID, spanID string) string {
	return fmt.Sprintf(`{
  "resourceLogs": [{
    "resource": {"attributes": [{"key": "service.name", "value": {"stringValue": "checkout-api"}}]},
    "scopeLogs": [{
      "scope": {"name": "payments", "version": "1.0.0"},
      "logRecords": [{
        "timeUnixNano": "%[1]d",
        "observedTimeUnixNano": "%[1]d",
        "severityNumber": 17,
        "severityText": "ERROR",
        "body": {"stringValue": "payment failed"},
        "attributes": [
          {"key": "order_id", "value": {"intValue": "8841"}},
          {"key": "retryable", "value": {"boolValue": false}}
        ],
        "droppedAttributesCount": 0,
        "flags": 1,
        "traceId": "%[2]s",
        "spanId": "%[3]s"
      }, {
        "timeUnixNano": "%[1]d",
        "severityNumber": "SEVERITY_NUMBER_WARN",
        "body": {"stringValue": "slow"},
        "traceId": "",
        "spanId": ""
      }]
    }]
  }]
}`, at.UnixNano(), traceID, spanID)
}

func TestParseOTLPReadsHexIdsOfOTLPJSON(t *testing.T) {
	now := time.Now().UTC()
	body := otlpJSONLogs(now.Add(-time.Minute), "5b8efff798038103d269b633813fc60c", "eee19b7ec3c1b174")
	parsed, format, err := ParseOTLP("application/json", []byte(body), now)
	if err != nil || format != OTLPJSON || len(parsed.Errors) != 0 || len(parsed.Entries) != 2 {
		t.Fatalf("parsed = %+v, format = %v, err = %v", parsed, format, err)
	}
	failed, warn := parsed.Entries[0], parsed.Entries[1]
	if failed.TraceID != "5b8efff798038103d269b633813fc60c" || failed.SpanID != "eee19b7ec3c1b174" {
		t.Fatalf("ids = %q %q", failed.TraceID, failed.SpanID)
	}
	if failed.Severity != "error" || failed.Body != "payment failed" || failed.Attributes["order_id"] != int64(8841) {
		t.Fatalf("record = %+v", failed)
	}
	if warn.TraceID != "" || warn.SpanID != "" || warn.Severity != "warn" {
		t.Fatalf("a record with empty ids keeps none: %+v", warn)
	}
}

func TestParseOTLPKeepsReadingBase64IdsOfOTLPJSON(t *testing.T) {
	now := time.Now().UTC()
	// What protojson writes for the ids 4bf92f3577b34da6a3ce929d0e0e4736 and 00f067aa0ba902b7.
	body := otlpJSONLogs(now.Add(-time.Minute), "S/kvNXezTaajzpKdDg5HNg==", "APBnqgupArc=")
	parsed, _, err := ParseOTLP("application/json", []byte(body), now)
	if err != nil || len(parsed.Entries) != 2 || parsed.Entries[0].TraceID != "4bf92f3577b34da6a3ce929d0e0e4736" || parsed.Entries[0].SpanID != "00f067aa0ba902b7" {
		t.Fatalf("parsed = %+v, err = %v", parsed.Entries, err)
	}
}

func TestParseOTLPRejectsIdsThatAreNeitherHexNorBase64(t *testing.T) {
	now := time.Now().UTC()
	body := otlpJSONLogs(now.Add(-time.Minute), "not-an-id!", "eee19b7ec3c1b174")
	if _, _, err := ParseOTLP("application/json", []byte(body), now); !errors.Is(err, ErrInvalidBody) {
		t.Fatalf("err = %v, want ErrInvalidBody", err)
	}
}
