package api

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	collogs "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func strKV(k, v string) *commonpb.KeyValue {
	return &commonpb.KeyValue{Key: k, Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: v}}}
}

func otlpRequest(ts time.Time) *collogs.ExportLogsServiceRequest {
	return &collogs.ExportLogsServiceRequest{ResourceLogs: []*logspb.ResourceLogs{{
		Resource: &resourcepb.Resource{Attributes: []*commonpb.KeyValue{strKV("service.name", "checkout-api"), strKV("deployment.environment", "production")}},
		ScopeLogs: []*logspb.ScopeLogs{{
			Scope: &commonpb.InstrumentationScope{Name: "payments", Version: "1.2.0"},
			LogRecords: []*logspb.LogRecord{
				{
					TimeUnixNano: uint64(ts.UnixNano()), SeverityNumber: logspb.SeverityNumber_SEVERITY_NUMBER_ERROR,
					Body:    &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "payment failed"}},
					TraceId: []byte{0x4b, 0xf9, 0x2f, 0x35, 0x77, 0xb3, 0x4d, 0xa6, 0xa3, 0xce, 0x92, 0x9d, 0x0e, 0x0e, 0x47, 0x36},
					SpanId:  []byte{0x00, 0xf0, 0x67, 0xaa, 0x0b, 0xa9, 0x02, 0xb7},
					Attributes: []*commonpb.KeyValue{
						strKV("event.name", "payment.failed"), strKV("password", "hunter2"),
						{Key: "order_id", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_IntValue{IntValue: 8841}}},
						{Key: "retryable", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_BoolValue{BoolValue: false}}},
					},
				},
				{ObservedTimeUnixNano: uint64(ts.UnixNano()), SeverityText: "WARN", Body: &commonpb.AnyValue{Value: &commonpb.AnyValue_KvlistValue{KvlistValue: &commonpb.KeyValueList{Values: []*commonpb.KeyValue{strKV("reason", "slow")}}}}},
				{}, // no body and no event: rejected, the rest still accepted
			},
		}},
	}}}
}

func postOTLP(server *Server, key, contentType string, body []byte, gzipped bool) *httptest.ResponseRecorder {
	if gzipped {
		var buf bytes.Buffer
		w := gzip.NewWriter(&buf)
		w.Write(body)
		w.Close()
		body = buf.Bytes()
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/ingest/otlp/v1/logs", bytes.NewReader(body))
	req.Header.Set("Content-Type", contentType)
	if gzipped {
		req.Header.Set("Content-Encoding", "gzip")
	}
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	return rawRequest(server, req)
}

func TestOTLPIngestMapsRecordsForProtobufJSONAndGzip(t *testing.T) {
	ts := time.Now().UTC().Add(-time.Minute)
	protoBody, _ := proto.Marshal(otlpRequest(ts))
	jsonBody, _ := protojson.Marshal(otlpRequest(ts))

	for name, tc := range map[string]struct {
		contentType string
		body        []byte
		gz          bool
	}{"protobuf": {"application/x-protobuf", protoBody, false}, "protobuf gzip": {"application/x-protobuf", protoBody, true}, "json": {"application/json", jsonBody, false}, "json gzip": {"application/json; charset=utf-8", jsonBody, true}} {
		t.Run(name, func(t *testing.T) {
			server := setupLogsServer(t, nil)
			appID, key := createAppWithKey(t, server, "otel-app")
			resp := postOTLP(server, key, tc.contentType, tc.body, tc.gz)
			if resp.Code != http.StatusOK {
				t.Fatalf("status = %d body = %s", resp.Code, resp.Body.String())
			}
			// The response mirrors the request encoding and reports the rejected record (partial success).
			out := &collogs.ExportLogsServiceResponse{}
			if strings.HasPrefix(tc.contentType, "application/json") {
				if err := protojson.Unmarshal(resp.Body.Bytes(), out); err != nil {
					t.Fatalf("response is not OTLP JSON: %v %s", err, resp.Body.String())
				}
			} else if err := proto.Unmarshal(resp.Body.Bytes(), out); err != nil {
				t.Fatalf("response is not OTLP protobuf: %v", err)
			}
			if out.GetPartialSuccess().GetRejectedLogRecords() != 1 {
				t.Fatalf("partial_success = %+v, want 1 rejected record", out.GetPartialSuccess())
			}

			entries := searchLogsAPI(t, server, "app_id="+appID+"&limit=10").Data.Entries
			if len(entries) != 2 {
				t.Fatalf("stored %d entries, want 2", len(entries))
			}
			failed, warn := entries[0], entries[1]
			if failed.Severity == "warn" {
				failed, warn = warn, failed
			}
			if failed.Severity != "error" || failed.Event != "payment.failed" || failed.Body != "payment failed" {
				t.Fatalf("error record = %+v", failed)
			}
			if failed.Attributes["order_id"] != float64(8841) || failed.Attributes["retryable"] != false || failed.Attributes["otel.scope.name"] != "payments" {
				t.Fatalf("attributes = %v", failed.Attributes)
			}
			if failed.Attributes["password"] != "[redacted]" {
				t.Fatalf("password = %v, want redacted for OTLP just like native ingest", failed.Attributes["password"])
			}
			if warn.Severity != "warn" || !strings.Contains(warn.Body, "slow") {
				t.Fatalf("warn record = %+v, want severity from severity_text and a structured body", warn)
			}
			one := performJSONRequest(t, server, http.MethodGet, "/v1/logs/"+failed.ID, nil, "")
			for _, want := range []string{`"trace_id":"4bf92f3577b34da6a3ce929d0e0e4736"`, `"span_id":"00f067aa0ba902b7"`, `"service.name":"checkout-api"`} {
				if !strings.Contains(one.Body.String(), want) {
					t.Errorf("entry detail missing %s: %s", want, one.Body.String())
				}
			}
		})
	}
}

func TestOTLPIngestErrors(t *testing.T) {
	server := setupLogsServer(t, nil)
	_, key := createAppWithKey(t, server, "otel-errors")
	good, _ := proto.Marshal(otlpRequest(time.Now()))
	if got := postOTLP(server, "", "application/x-protobuf", good, false).Code; got != http.StatusUnauthorized {
		t.Errorf("no key status = %d, want 401", got)
	}
	if got := postOTLP(server, key, "text/plain", good, false).Code; got != http.StatusUnsupportedMediaType {
		t.Errorf("text/plain status = %d, want 415", got)
	}
	if got := postOTLP(server, key, "application/x-protobuf", []byte("\x0a\xff\xff\xff garbage"), false).Code; got != http.StatusBadRequest {
		t.Errorf("garbage protobuf status = %d, want 400", got)
	}
	if got := postOTLP(server, key, "application/json", []byte(`{"resourceLogs": "nope"}`), false).Code; got != http.StatusBadRequest {
		t.Errorf("bad json status = %d, want 400", got)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/ingest/otlp/v1/logs", strings.NewReader("not gzip"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("Authorization", "Bearer "+key)
	if got := rawRequest(server, req).Code; got != http.StatusBadRequest {
		t.Errorf("fake gzip status = %d, want 400", got)
	}
	// An empty request is a valid no-op in OTLP.
	empty, _ := proto.Marshal(&collogs.ExportLogsServiceRequest{})
	if got := postOTLP(server, key, "application/x-protobuf", empty, false).Code; got != http.StatusOK {
		t.Errorf("empty export status = %d, want 200", got)
	}
}

func TestNativeIngestAcceptsGzip(t *testing.T) {
	server := setupLogsServer(t, nil)
	appID, key := createAppWithKey(t, server, "gz")
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	w.Write([]byte(`[{"body":"a"},{"body":"b"}]`))
	w.Close()
	req := httptest.NewRequest(http.MethodPost, "/v1/ingest/logs", &buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("Authorization", "Bearer "+key)
	if got := rawRequest(server, req).Code; got != http.StatusAccepted {
		t.Fatalf("gzip native status = %d, want 202", got)
	}
	if n := len(searchLogsAPI(t, server, "app_id="+appID).Data.Entries); n != 2 {
		t.Fatalf("stored %d, want 2", n)
	}
	// A gzip bomb is capped at the same size as an uncompressed body.
	var bomb bytes.Buffer
	bw := gzip.NewWriter(&bomb)
	bw.Write([]byte(`{"body":"` + strings.Repeat("a", 3<<20) + `"}`))
	bw.Close()
	req = httptest.NewRequest(http.MethodPost, "/v1/ingest/logs", &bomb)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("Authorization", "Bearer "+key)
	if got := rawRequest(server, req).Code; got != http.StatusRequestEntityTooLarge {
		t.Fatalf("decompressed 3 MiB status = %d, want 413", got)
	}
}
