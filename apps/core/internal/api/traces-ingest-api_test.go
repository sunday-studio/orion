package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"orion/core/internal/config"
	"orion/core/internal/tracestore"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

func TestOTLPTracesNeedAValidIngestKey(t *testing.T) {
	server, _ := tracesServer(t, nil, 0)
	appID, key := createAppWithKey(t, server, "checkout-api")
	body, contentType := marshalTraceRequest(t, otlpTraceRequest("svc", otlpSpan(1, 1, 0, "root", time.Now(), time.Millisecond, false)), true)

	for name, bearer := range map[string]string{"no key": "", "unknown key": "orion_ik_0000000000000000000000000000000000000000", "not an ingest key": "abc"} {
		resp := postOTLPTraces(server, bearer, contentType, body, false)
		if resp.Code != http.StatusUnauthorized || resp.Header().Get("WWW-Authenticate") == "" {
			t.Fatalf("%s: status = %d, want 401 with WWW-Authenticate", name, resp.Code)
		}
	}
	if resp := postOTLPTraces(server, key, contentType, body, false); resp.Code != http.StatusOK {
		t.Fatalf("valid key status = %d body = %s", resp.Code, resp.Body.String())
	}
	server.traces.Flush(5 * time.Second)
	if list := getTraces(t, server, "app_id="+appID, http.StatusOK); len(list.Data.Traces) != 1 {
		t.Fatalf("traces of the app = %+v, want the one just sent", list.Data.Traces)
	}
}

func TestOTLPTracesAreStoredFromProtobufJSONAndGzip(t *testing.T) {
	server, store := tracesServer(t, nil, 0)
	_, key := createAppWithKey(t, server, "checkout-api")
	now := time.Now().UTC()
	for i, c := range []struct {
		protobuf, gzipped bool
	}{{true, false}, {false, false}, {true, true}, {false, true}} {
		req := otlpTraceRequest("checkout-api", otlpSpan(10+i, 1, 0, "POST /pay", now.Add(-time.Minute), 120*time.Millisecond, false))
		body, contentType := marshalTraceRequest(t, req, c.protobuf)
		resp := postOTLPTraces(server, key, contentType, body, c.gzipped)
		if resp.Code != http.StatusOK {
			t.Fatalf("case %+v: status = %d body = %s", c, resp.Code, resp.Body.String())
		}
		// The response mirrors the request encoding; a clean request has an empty partial_success.
		if want := map[bool]string{true: "application/x-protobuf", false: "application/json"}[c.protobuf]; !strings.HasPrefix(resp.Header().Get("Content-Type"), want) {
			t.Fatalf("case %+v: response type = %q, want %s", c, resp.Header().Get("Content-Type"), want)
		}
		if rejected, _ := rejectedSpans(t, resp, c.protobuf); rejected != 0 {
			t.Fatalf("case %+v: rejected = %d", c, rejected)
		}
	}
	store.Flush(5 * time.Second)
	list := getTraces(t, server, "", http.StatusOK)
	if len(list.Data.Traces) != 4 {
		t.Fatalf("traces = %d, want 4", len(list.Data.Traces))
	}
	if first := list.Data.Traces[0]; first.AppName != "checkout-api" || first.ServiceName != "checkout-api" || first.RootName != "POST /pay" || first.DurationNs != (120*time.Millisecond).Nanoseconds() || first.Status != "ok" || !first.HasRoot {
		t.Fatalf("first trace = %+v", first)
	}
}

func TestOTLPTracesReportRejectedSpansAndStoreTheRest(t *testing.T) {
	server, store := tracesServer(t, nil, 0)
	appID, key := createAppWithKey(t, server, "checkout-api")
	now := time.Now().UTC()
	bad := otlpSpan(1, 2, 1, "bad", now, time.Millisecond, false)
	bad.TraceId = []byte{1, 2, 3} // not 16 bytes
	reversed := otlpSpan(1, 3, 1, "reversed", now, time.Millisecond, false)
	reversed.EndTimeUnixNano = reversed.StartTimeUnixNano - 1

	for _, protobuf := range []bool{true, false} {
		req := otlpTraceRequest("checkout-api", otlpSpan(1, 1, 0, "root", now, time.Second, false), bad, reversed)
		body, contentType := marshalTraceRequest(t, req, protobuf)
		resp := postOTLPTraces(server, key, contentType, body, false)
		if resp.Code != http.StatusOK {
			t.Fatalf("protobuf=%v status = %d body = %s", protobuf, resp.Code, resp.Body.String())
		}
		rejected, message := rejectedSpans(t, resp, protobuf)
		if rejected != 2 || !strings.Contains(message, "span 1:") || !strings.Contains(message, "trace_id") || !strings.Contains(message, "span 2:") || !strings.Contains(message, "ends before it starts") {
			t.Fatalf("protobuf=%v: rejected = %d message = %q", protobuf, rejected, message)
		}
	}
	store.Flush(5 * time.Second)
	if n, _, _ := store.CountForApp(appID); n != 1 { // the valid root, once: the second request resent it
		t.Fatalf("stored spans = %d, want only the valid root", n)
	}

	// A request whose spans are all invalid is still answered 200 with partial_success and stores nothing.
	body, contentType := marshalTraceRequest(t, otlpTraceRequest("svc", bad), true)
	resp := postOTLPTraces(server, key, contentType, body, false)
	if rejected, _ := rejectedSpans(t, resp, true); resp.Code != http.StatusOK || rejected != 1 {
		t.Fatalf("all-invalid request: status = %d rejected = %d", resp.Code, rejected)
	}
}

func TestOTLPTracesRequestLevelFailures(t *testing.T) {
	server, _ := tracesServer(t, nil, 0)
	_, key := createAppWithKey(t, server, "checkout-api")
	valid, _ := marshalTraceRequest(t, otlpTraceRequest("svc", otlpSpan(1, 1, 0, "root", time.Now(), time.Millisecond, false)), false)
	now := time.Now()
	many := make([]*tracepb.Span, tracestore.MaxSpansPerRequest+1)
	for i := range many {
		many[i] = otlpSpan(1, 1, 0, "s", now, time.Millisecond, false)
	}
	manyBody, _ := marshalTraceRequest(t, otlpTraceRequest("svc", many...), true)

	cases := []struct {
		name        string
		contentType string
		body        []byte
		gzipped     bool
		want        int
	}{
		{"wrong content type", "text/plain", valid, false, http.StatusUnsupportedMediaType},
		{"broken json", "application/json", []byte(`{"resourceSpans":[`), false, http.StatusBadRequest},
		{"broken protobuf", "application/x-protobuf", []byte{0xff, 0xff, 0xff, 0xff, 0x01}, false, http.StatusBadRequest},
		{"over 10,000 spans", "application/x-protobuf", manyBody, true, http.StatusRequestEntityTooLarge},
		{"body over the 5 MiB cap", "application/json", bytes.Repeat([]byte(" "), 6<<20), false, http.StatusRequestEntityTooLarge},
	}
	for _, tc := range cases {
		if resp := postOTLPTraces(server, key, tc.contentType, tc.body, tc.gzipped); resp.Code != tc.want {
			t.Fatalf("%s: status = %d, want %d (%s)", tc.name, resp.Code, tc.want, resp.Body.String())
		}
	}
	// A body that says it is gzip but is not.
	req := httptest.NewRequest(http.MethodPost, "/v1/ingest/otlp/v1/traces", strings.NewReader("not gzip"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("Authorization", "Bearer "+key)
	if resp := rawRequest(server, req); resp.Code != http.StatusBadRequest {
		t.Fatalf("bad gzip: status = %d, want 400", resp.Code)
	}
}

func TestOTLPTracesAnswer503WhenTracesAreOff(t *testing.T) {
	server := setupLogsServer(t, nil) // logs only
	_, key := createAppWithKey(t, server, "checkout-api")
	body, contentType := marshalTraceRequest(t, otlpTraceRequest("svc", otlpSpan(1, 1, 0, "root", time.Now(), time.Millisecond, false)), true)
	if resp := postOTLPTraces(server, key, contentType, body, false); resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("ingest status = %d, want 503", resp.Code)
	}
	if resp := performJSONRequest(t, server, http.MethodGet, "/v1/traces", nil, ""); resp.Code != http.StatusServiceUnavailable {
		t.Fatalf("list status = %d, want 503", resp.Code)
	}
	// Without a valid key the answer is still 401, not a hint that traces are off.
	if resp := postOTLPTraces(server, "", contentType, body, false); resp.Code != http.StatusUnauthorized {
		t.Fatalf("no key status = %d, want 401", resp.Code)
	}
}

func TestOTLPTracesShareTheAppRateLimit(t *testing.T) {
	server, store := tracesServer(t, &config.Config{LogsAppRatePerSecond: 5}, 0)
	_, key := createAppWithKey(t, server, "noisy")
	var spans []*tracepb.Span
	for i := 1; i <= 10; i++ {
		spans = append(spans, otlpSpan(1, i, 0, "s", time.Now(), time.Millisecond, false))
	}
	body, contentType := marshalTraceRequest(t, otlpTraceRequest("svc", spans...), true)
	if resp := postOTLPTraces(server, key, contentType, body, false); resp.Code != http.StatusOK {
		t.Fatalf("first burst status = %d, want 200", resp.Code)
	}
	resp := postOTLPTraces(server, key, contentType, body, false)
	if resp.Code != http.StatusTooManyRequests || resp.Header().Get("Retry-After") == "" {
		t.Fatalf("second burst status = %d Retry-After = %q, want 429 with Retry-After", resp.Code, resp.Header().Get("Retry-After"))
	}
	// The limiter is the one logs use: a log burst right after is refused too.
	if got := ingest(server, key, "application/json", `{"body":"x"}`).Code; got != http.StatusTooManyRequests {
		t.Fatalf("log after the span burst status = %d, want 429 (shared budget)", got)
	}
	store.Flush(5 * time.Second)
}

func TestOTLPTracesAnswer429WhenTheWriteQueueIsFull(t *testing.T) {
	server, _ := tracesServer(t, nil, 2)
	_, key := createAppWithKey(t, server, "checkout-api")
	var spans []*tracepb.Span
	for i := 1; i <= 3; i++ {
		spans = append(spans, otlpSpan(1, i, 0, "s", time.Now(), time.Millisecond, false))
	}
	body, contentType := marshalTraceRequest(t, otlpTraceRequest("svc", spans...), true)
	resp := postOTLPTraces(server, key, contentType, body, false)
	if resp.Code != http.StatusTooManyRequests || resp.Header().Get("Retry-After") != "1" {
		t.Fatalf("status = %d Retry-After = %q, want 429 and 1", resp.Code, resp.Header().Get("Retry-After"))
	}
}

func TestOTLPTracesApplyTheAppsRedactionRules(t *testing.T) {
	server, store := tracesServer(t, nil, 0)
	_, key := createAppWithKey(t, server, "checkout-api")
	span := otlpSpan(1, 1, 0, "login", time.Now(), time.Millisecond, true)
	span.Attributes = []*commonpb.KeyValue{strKV("user.password", "hunter2"), strKV("http.route", "/login")}
	span.Events = []*tracepb.Span_Event{{Name: "attempt", Attributes: []*commonpb.KeyValue{strKV("api_key", "k-123")}}}
	sendTraces(t, server, store, key, span)

	resp := performJSONRequest(t, server, http.MethodGet, "/v1/traces/"+traceHex(1), nil, "")
	body := resp.Body.String()
	if resp.Code != http.StatusOK || strings.Contains(body, "hunter2") || strings.Contains(body, "k-123") || !strings.Contains(body, "[redacted]") || !strings.Contains(body, "/login") {
		t.Fatalf("status = %d, body = %s: secrets must be redacted, other values kept", resp.Code, body)
	}
}

func TestTraceRoutesNeedTheAdminLoginButIngestDoesNot(t *testing.T) {
	cfg := &config.Config{FrontendAuthOn: true, AdminUsername: "admin", AdminPassword: "correct-password", JWTSecret: "test-secret-test-secret-test-1234"}
	server, store := tracesServer(t, cfg, 0)
	token := loginStatusPageTestAdmin(t, server)

	create := performJSONRequest(t, server, http.MethodPost, "/v1/apps", gin.H{"name": "checkout-api"}, token)
	if create.Code != http.StatusCreated {
		t.Fatalf("create app = %d %s", create.Code, create.Body.String())
	}
	var app struct {
		Data struct{ App struct{ ID string } }
	}
	decodeResponse(t, create, &app)
	keyResp := performJSONRequest(t, server, http.MethodPost, "/v1/apps/"+app.Data.App.ID+"/keys", gin.H{"name": "k"}, token)
	var created struct {
		Data struct{ Key struct{ Key string } }
	}
	decodeResponse(t, keyResp, &created)

	sendTraces(t, server, store, created.Data.Key.Key, otlpSpan(1, 1, 0, "root", time.Now(), time.Millisecond, false))
	for _, path := range []string{"/v1/traces", "/v1/traces/facets", "/v1/traces/" + traceHex(1), "/v1/services/svc-1/traces/summary"} {
		if resp := performJSONRequest(t, server, http.MethodGet, path, nil, ""); resp.Code != http.StatusUnauthorized {
			t.Fatalf("GET %s without a login = %d, want 401", path, resp.Code)
		}
		if resp := performJSONRequest(t, server, http.MethodGet, path, nil, created.Data.Key.Key); resp.Code != http.StatusUnauthorized {
			t.Fatalf("GET %s with an ingest key = %d, want 401: ingest keys cannot read", path, resp.Code)
		}
	}
	if resp := performJSONRequest(t, server, http.MethodGet, "/v1/traces", nil, token); resp.Code != http.StatusOK {
		t.Fatalf("GET /v1/traces with a login = %d, want 200", resp.Code)
	}
}
