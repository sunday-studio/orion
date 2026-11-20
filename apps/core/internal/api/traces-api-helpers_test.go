package api

import (
	"bytes"
	"compress/gzip"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"orion/core/internal/config"
	"orion/core/internal/testkit"
	"orion/core/internal/tracestore"
	"orion/core/internal/utils"
	"testing"
	"time"

	coltrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// tracesServer is a Core with product logs and traces enabled and no admin auth.
func tracesServer(t *testing.T, cfg *config.Config, queueMax int) (*Server, *tracestore.Store) {
	t.Helper()
	server := setupLogsServer(t, cfg)
	store, err := tracestore.OpenWith(testkit.NewDB(t), utils.NewLogger(), queueMax)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	server.EnableTraces(store)
	return server, store
}

func traceIDBytes(n int) []byte {
	b := make([]byte, 16)
	b[0], b[15] = 0xcd, byte(n)
	return b
}

func spanIDBytes(n int) []byte {
	b := make([]byte, 8)
	b[0], b[7] = 0xef, byte(n)
	return b
}

func traceHex(n int) string { return hex.EncodeToString(traceIDBytes(n)) }

// otlpSpan builds a span of trace traceN; parent 0 makes it a root. It started at start and lasted dur.
func otlpSpan(traceN, spanN, parent int, name string, start time.Time, dur time.Duration, failed bool) *tracepb.Span {
	span := &tracepb.Span{
		TraceId: traceIDBytes(traceN), SpanId: spanIDBytes(spanN), Name: name, Kind: tracepb.Span_SPAN_KIND_SERVER,
		StartTimeUnixNano: uint64(start.UnixNano()), EndTimeUnixNano: uint64(start.Add(dur).UnixNano()),
	}
	if parent != 0 {
		span.ParentSpanId = spanIDBytes(parent)
	}
	if failed {
		span.Status = &tracepb.Status{Code: tracepb.Status_STATUS_CODE_ERROR, Message: "boom"}
	}
	return span
}

func otlpTraceRequest(service string, spans ...*tracepb.Span) *coltrace.ExportTraceServiceRequest {
	return &coltrace.ExportTraceServiceRequest{ResourceSpans: []*tracepb.ResourceSpans{{
		Resource:   &resourcepb.Resource{Attributes: []*commonpb.KeyValue{strKV("service.name", service)}},
		ScopeSpans: []*tracepb.ScopeSpans{{Scope: &commonpb.InstrumentationScope{Name: "test"}, Spans: spans}},
	}}}
}

func marshalTraceRequest(t *testing.T, req *coltrace.ExportTraceServiceRequest, protobuf bool) ([]byte, string) {
	t.Helper()
	if protobuf {
		b, err := proto.Marshal(req)
		if err != nil {
			t.Fatal(err)
		}
		return b, "application/x-protobuf"
	}
	b, err := protojson.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	return b, "application/json"
}

func postOTLPTraces(server *Server, key, contentType string, body []byte, gzipped bool) *httptest.ResponseRecorder {
	if gzipped {
		var buf bytes.Buffer
		w := gzip.NewWriter(&buf)
		w.Write(body)
		w.Close()
		body = buf.Bytes()
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/ingest/otlp/v1/traces", bytes.NewReader(body))
	req.Header.Set("Content-Type", contentType)
	if gzipped {
		req.Header.Set("Content-Encoding", "gzip")
	}
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	return rawRequest(server, req)
}

// sendTraces posts a request as protobuf and fails the test unless it is accepted.
func sendTraces(t *testing.T, server *Server, store *tracestore.Store, key string, spans ...*tracepb.Span) {
	t.Helper()
	body, contentType := marshalTraceRequest(t, otlpTraceRequest("checkout-api", spans...), true)
	if resp := postOTLPTraces(server, key, contentType, body, false); resp.Code != http.StatusOK {
		t.Fatalf("ingest status = %d body = %s", resp.Code, resp.Body.String())
	}
	store.Flush(10 * time.Second)
}

func rejectedSpans(t *testing.T, resp *httptest.ResponseRecorder, protobuf bool) (int64, string) {
	t.Helper()
	out := &coltrace.ExportTraceServiceResponse{}
	var err error
	if protobuf {
		err = proto.Unmarshal(resp.Body.Bytes(), out)
	} else {
		err = protojson.Unmarshal(resp.Body.Bytes(), out)
	}
	if err != nil {
		t.Fatalf("response body %q: %v", resp.Body.String(), err)
	}
	return out.GetPartialSuccess().GetRejectedSpans(), out.GetPartialSuccess().GetErrorMessage()
}

type traceListBody struct {
	Data struct {
		Traces []struct {
			TraceID     string `json:"trace_id"`
			AppID       string `json:"app_id"`
			AppName     string `json:"app_name"`
			ServiceName string `json:"service_name"`
			RootName    string `json:"root_name"`
			DurationNs  int64  `json:"duration_ns"`
			SpanCount   int64  `json:"span_count"`
			ErrorCount  int64  `json:"error_count"`
			HasRoot     bool   `json:"has_root"`
			Status      string `json:"status"`
		} `json:"traces"`
		NextCursor string `json:"next_cursor"`
	} `json:"data"`
}

func getTraces(t *testing.T, server *Server, query string, wantStatus int) traceListBody {
	t.Helper()
	resp := performJSONRequest(t, server, http.MethodGet, "/v1/traces?"+query, nil, "")
	if resp.Code != wantStatus {
		t.Fatalf("GET /v1/traces?%s = %d, want %d: %s", query, resp.Code, wantStatus, resp.Body.String())
	}
	var out traceListBody
	if resp.Code == http.StatusOK {
		if err := json.Unmarshal(resp.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
	}
	return out
}

func traceRootNames(list traceListBody) []string {
	names := make([]string, 0, len(list.Data.Traces))
	for _, tr := range list.Data.Traces {
		names = append(names, tr.RootName)
	}
	return names
}
