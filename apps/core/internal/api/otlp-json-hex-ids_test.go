package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"orion/core/internal/metricstore"
	"orion/core/internal/tracestore"
	"orion/core/internal/utils"
	"strings"
	"testing"
	"time"
)

const (
	hexTraceID = "5b8efff798038103d269b633813fc60c"
	hexSpanID  = "eee19b7ec3c1b174"
)

// jsonExporterServer is a Core with logs, metrics and traces on.
func jsonExporterServer(t *testing.T) (*Server, *tracestore.Store) {
	t.Helper()
	server, traces := tracesServer(t, nil, 0)
	metrics, err := metricstore.Open(t.TempDir(), utils.NewLogger())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(metrics.Close)
	server.EnableMetrics(metrics)
	return server, traces
}

// The bodies below are what an http/json exporter of an OpenTelemetry SDK or the Collector sends:
// lowerCamelCase keys, 64-bit values as strings and trace and span ids as hex.
func hexLogsBody(at time.Time) string {
	return fmt.Sprintf(`{"resourceLogs":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"checkout-api"}}]},
"scopeLogs":[{"scope":{"name":"payments"},"logRecords":[{"timeUnixNano":"%d","severityNumber":17,"severityText":"ERROR",
"body":{"stringValue":"payment failed"},"traceId":"%s","spanId":"%s"}]}]}]}`, at.UnixNano(), hexTraceID, hexSpanID)
}

func hexMetricsBody(at time.Time) string {
	return fmt.Sprintf(`{"resourceMetrics":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"checkout-api"}}]},
"scopeMetrics":[{"scope":{"name":"http-server"},"metrics":[{"name":"http.server.requests","unit":"1","sum":{"aggregationTemporality":1,"isMonotonic":true,
"dataPoints":[{"startTimeUnixNano":"%d","timeUnixNano":"%d","asInt":"4","exemplars":[{"timeUnixNano":"%d","asDouble":0.5,"spanId":"%s","traceId":"%s"}]}]}}]}]}]}`,
		at.Add(-time.Minute).UnixNano(), at.UnixNano(), at.UnixNano(), hexSpanID, hexTraceID)
}

func hexTracesBody(at time.Time) string {
	return fmt.Sprintf(`{"resourceSpans":[{"resource":{"attributes":[{"key":"service.name","value":{"stringValue":"checkout-api"}}]},
"scopeSpans":[{"scope":{"name":"http-server"},"spans":[
{"traceId":"%[1]s","spanId":"%[2]s","parentSpanId":"","name":"GET /checkout","kind":2,"startTimeUnixNano":"%[3]d","endTimeUnixNano":"%[4]d","status":{"code":1}},
{"traceId":"%[1]s","spanId":"eee19b7ec3c1b175","parentSpanId":"%[2]s","name":"charge","kind":3,"startTimeUnixNano":"%[3]d","endTimeUnixNano":"%[4]d",
"links":[{"traceId":"5b8efff798038103d269b633813fc60d","spanId":"eee19b7ec3c1b176"}],"status":{"code":2,"message":"declined"}}]}]}]}`,
		hexTraceID, hexSpanID, at.UnixNano(), at.Add(200*time.Millisecond).UnixNano())
}

func TestOTLPJSONWithHexIdsIsAcceptedForAllThreeSignals(t *testing.T) {
	for _, gz := range []bool{false, true} {
		t.Run(fmt.Sprintf("gzip=%v", gz), func(t *testing.T) {
			server, traces := jsonExporterServer(t)
			appID, key := createAppWithKey(t, server, "checkout-api")
			at := time.Now().UTC().Add(-2 * time.Minute)

			// Logs: the ids are stored as the same hex.
			resp := postOTLP(server, key, "application/json", []byte(hexLogsBody(at)), gz)
			if resp.Code != http.StatusOK || !strings.HasPrefix(resp.Header().Get("Content-Type"), "application/json") {
				t.Fatalf("logs: status = %d type = %q body = %s", resp.Code, resp.Header().Get("Content-Type"), resp.Body.String())
			}
			entries := searchLogsAPI(t, server, "app_id="+appID+"&limit=10").Data.Entries
			if len(entries) != 1 {
				t.Fatalf("stored %d log entries, want 1", len(entries))
			}
			detail := performJSONRequest(t, server, http.MethodGet, "/v1/logs/"+entries[0].ID, nil, "")
			for _, want := range []string{`"trace_id":"` + hexTraceID + `"`, `"span_id":"` + hexSpanID + `"`} {
				if !strings.Contains(detail.Body.String(), want) {
					t.Errorf("log entry missing %s: %s", want, detail.Body.String())
				}
			}

			// Metrics: the exemplar's ids no longer fail the request.
			resp = postOTLPMetrics(server, key, "application/json", []byte(hexMetricsBody(at)), gz)
			if resp.Code != http.StatusOK {
				t.Fatalf("metrics: status = %d body = %s", resp.Code, resp.Body.String())
			}
			if rejected, msg := rejectedPoints(t, resp, false); rejected != 0 {
				t.Fatalf("metrics: rejected = %d (%s)", rejected, msg)
			}

			// Traces: spans, parent links and span links.
			resp = postOTLPTraces(server, key, "application/json", []byte(hexTracesBody(at)), gz)
			if resp.Code != http.StatusOK {
				t.Fatalf("traces: status = %d body = %s", resp.Code, resp.Body.String())
			}
			if rejected, msg := rejectedSpans(t, resp, false); rejected != 0 {
				t.Fatalf("traces: rejected = %d (%s)", rejected, msg)
			}
			traces.Flush(5 * time.Second)
			list := getTraces(t, server, "app_id="+appID, http.StatusOK)
			if len(list.Data.Traces) != 1 || list.Data.Traces[0].TraceID != hexTraceID || list.Data.Traces[0].RootName != "GET /checkout" ||
				list.Data.Traces[0].SpanCount != 2 || list.Data.Traces[0].ErrorCount != 1 || !list.Data.Traces[0].HasRoot {
				t.Fatalf("traces = %+v", list.Data.Traces)
			}
			one := performJSONRequest(t, server, http.MethodGet, "/v1/traces/"+hexTraceID, nil, "")
			var detailBody struct {
				Data struct {
					LogCount int64 `json:"log_count"`
					Spans    []struct {
						SpanID       string `json:"span_id"`
						ParentSpanID string `json:"parent_span_id"`
					} `json:"spans"`
				} `json:"data"`
			}
			if err := json.Unmarshal(one.Body.Bytes(), &detailBody); err != nil || len(detailBody.Data.Spans) != 2 {
				t.Fatalf("trace detail = %s (%v)", one.Body.String(), err)
			}
			child := detailBody.Data.Spans[1]
			if child.SpanID != "eee19b7ec3c1b175" || child.ParentSpanID != hexSpanID {
				t.Fatalf("child span = %+v", child)
			}
			if detailBody.Data.LogCount != 1 {
				t.Fatalf("log_count = %d, want the log that carries the same trace id", detailBody.Data.LogCount)
			}
		})
	}
}

func TestOTLPJSONWithBadIdsIsStillA400(t *testing.T) {
	server, _ := jsonExporterServer(t)
	_, key := createAppWithKey(t, server, "checkout-api")
	at := time.Now().UTC().Add(-time.Minute)
	for name, tc := range map[string]struct {
		post func(*Server, string, string, []byte, bool) *httptest.ResponseRecorder
		body string
	}{
		"logs":    {postOTLP, strings.Replace(hexLogsBody(at), hexTraceID, "not-an-id!", 1)},
		"metrics": {postOTLPMetrics, strings.Replace(hexMetricsBody(at), hexTraceID, "not-an-id!", 1)},
		"traces":  {postOTLPTraces, strings.Replace(hexTracesBody(at), hexSpanID+`","parentSpanId":""`, "not-an-id!"+`","parentSpanId":""`, 1)},
	} {
		if resp := tc.post(server, key, "application/json", []byte(tc.body), false); resp.Code != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400: %s", name, resp.Code, resp.Body.String())
		}
	}
}
