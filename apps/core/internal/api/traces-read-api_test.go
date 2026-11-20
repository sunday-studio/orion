package api

import (
	"encoding/json"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

// seedReadTraces stores, for one app, traces that started at now-10m, now-3h and now-30h (far from every
// boundary the tests use) and returns their start offsets from now.
func seedReadTraces(t *testing.T, server *Server, key string) {
	t.Helper()
	now := time.Now().UTC()
	checkout := otlpSpan(1, 1, 0, "POST /checkout", now.Add(-10*time.Minute), 800*time.Millisecond, false)
	checkoutDB := otlpSpan(1, 2, 1, "db.query", now.Add(-10*time.Minute+time.Millisecond), 5*time.Millisecond, true)
	checkoutDB.Attributes = []*commonpb.KeyValue{strKV("db.system", "postgres")}
	checkoutDB.Events = []*tracepb.Span_Event{{TimeUnixNano: uint64(now.Add(-10*time.Minute + 2*time.Millisecond).UnixNano()), Name: "exception"}}
	slow := otlpSpan(2, 3, 0, "GET /report", now.Add(-3*time.Hour), 4*time.Second, false)
	old := otlpSpan(3, 4, 0, "GET /health", now.Add(-30*time.Hour), time.Millisecond, false)
	sendTraces(t, server, server.traces, key, checkout, checkoutDB, slow, old)
}

func TestListTracesFiltersAndRelativeTimes(t *testing.T) {
	server, _ := tracesServer(t, nil, 0)
	appID, key := createAppWithKey(t, server, "checkout-api")
	seedReadTraces(t, server, key)
	rfc := func(d time.Duration) string { return url.QueryEscape(time.Now().UTC().Add(d).Format(time.RFC3339)) }

	cases := []struct {
		name  string
		query string
		want  []string
	}{
		{"everything, newest first", "", []string{"POST /checkout", "GET /report", "GET /health"}},
		{"relative from", "from=-1h", []string{"POST /checkout"}},
		{"relative window", "from=-4h&to=-2h", []string{"GET /report"}},
		{"relative to", "to=-1h", []string{"GET /report", "GET /health"}},
		{"days", "from=-2d&to=-1d", []string{"GET /health"}},
		{"rfc 3339 from", "from=" + rfc(-5*time.Hour), []string{"POST /checkout", "GET /report"}},
		{"from now keyword to", "from=-1h&to=now", []string{"POST /checkout"}},
		{"app id", "app_id=" + appID + "&from=-1h", []string{"POST /checkout"}},
		{"unknown app", "app_id=nope", nil},
		{"service name", "service=checkout-api&from=-4h", []string{"POST /checkout", "GET /report"}},
		{"other service", "service=billing", nil},
		{"status error", "status=error", []string{"POST /checkout"}},
		{"status ok", "status=ok", []string{"GET /report", "GET /health"}},
		{"min duration with unit", "min_duration=2s", []string{"GET /report"}},
		{"min duration in ms", "min_duration=500", []string{"POST /checkout", "GET /report"}},
		{"name substring", "name=report", []string{"GET /report"}},
		{"attribute of a child span", "attr=db.system:postgres", []string{"POST /checkout"}},
		{"attribute that is absent", "attr=db.system:mysql", nil},
		{"combined", "status=ok&min_duration=1s&from=-4h", []string{"GET /report"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := traceRootNames(getTraces(t, server, tc.query, http.StatusOK))
			if !reflect.DeepEqual(got, tc.want) && !(len(got) == 0 && len(tc.want) == 0) {
				t.Fatalf("GET /v1/traces?%s = %v, want %v", tc.query, got, tc.want)
			}
		})
	}
}

func TestListTracesRejectsBadFilters(t *testing.T) {
	server, _ := tracesServer(t, nil, 0)
	for _, query := range []string{
		"status=great", "min_duration=slow", "min_duration=-5s", "from=yesterday", "to=soon", "attr=nocolon", "attr=a..b:1", "cursor=%25%25%25", "from=-1x",
	} {
		getTraces(t, server, query, http.StatusBadRequest)
	}
	getTraces(t, server, "service_id=svc-nope", http.StatusNotFound)
}

func TestListTracesPaginatesWithACursor(t *testing.T) {
	server, store := tracesServer(t, nil, 0)
	_, key := createAppWithKey(t, server, "checkout-api")
	now := time.Now().UTC().Add(-time.Hour)
	var spans []*tracepb.Span
	for i := 1; i <= 7; i++ {
		spans = append(spans, otlpSpan(i, i, 0, "trace "+strconv.Itoa(i), now.Add(time.Duration(i)*time.Second), time.Millisecond, false))
	}
	sendTraces(t, server, store, key, spans...)

	var names []string
	cursor := ""
	for pages := 0; pages < 10; pages++ {
		list := getTraces(t, server, "limit=3&cursor="+url.QueryEscape(cursor), http.StatusOK)
		names = append(names, traceRootNames(list)...)
		if list.Data.NextCursor == "" {
			break
		}
		cursor = list.Data.NextCursor
	}
	want := []string{"trace 7", "trace 6", "trace 5", "trace 4", "trace 3", "trace 2", "trace 1"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("paged names = %v, want %v", names, want)
	}
	if n := len(getTraces(t, server, "limit=1000", http.StatusOK).Data.Traces); n != 7 {
		t.Fatalf("an oversized limit returned %d traces", n)
	}
}

func TestListTracesByServiceUsesTheLinkedApps(t *testing.T) {
	server, store := tracesServer(t, nil, 0)
	appA, keyA := createAppWithKey(t, server, "web")
	_, keyB := createAppWithKey(t, server, "worker")
	now := time.Now().UTC().Add(-time.Minute)
	sendTraces(t, server, store, keyA, otlpSpan(1, 1, 0, "from web", now, time.Millisecond, false))
	sendTraces(t, server, store, keyB, otlpSpan(2, 2, 0, "from worker", now, time.Millisecond, false))
	svc := createServiceVia(t, server, gin.H{"name": "Shop"})
	empty := createServiceVia(t, server, gin.H{"name": "Nothing linked"})
	performJSONRequest(t, server, http.MethodPost, "/v1/services/"+svc+"/links", gin.H{"kind": "app", "id": appA}, "")

	if got := traceRootNames(getTraces(t, server, "service_id="+svc, http.StatusOK)); !reflect.DeepEqual(got, []string{"from web"}) {
		t.Fatalf("service traces = %v, want only the linked app's", got)
	}
	if got := getTraces(t, server, "service_id="+empty, http.StatusOK); len(got.Data.Traces) != 0 {
		t.Fatalf("a service without apps listed %v", traceRootNames(got))
	}
	// app_id narrows within the service: an app that is not linked yields nothing.
	if got := getTraces(t, server, "service_id="+svc+"&app_id=nope", http.StatusOK); len(got.Data.Traces) != 0 {
		t.Fatalf("unlinked app within a service listed %v", traceRootNames(got))
	}
}

type traceDetailBody struct {
	Data struct {
		TraceID     string `json:"trace_id"`
		RootName    string `json:"root_name"`
		ServiceName string `json:"service_name"`
		DurationNs  int64  `json:"duration_ns"`
		HasRoot     bool   `json:"has_root"`
		SpanCount   int64  `json:"span_count"`
		ErrorCount  int64  `json:"error_count"`
		Status      string `json:"status"`
		Truncated   bool   `json:"truncated"`
		LogCount    int64  `json:"log_count"`
		Spans       []struct {
			SpanID        string         `json:"span_id"`
			ParentSpanID  string         `json:"parent_span_id"`
			AppName       string         `json:"app_name"`
			Name          string         `json:"name"`
			Kind          string         `json:"kind"`
			StartOffsetNs int64          `json:"start_offset_ns"`
			DurationNs    int64          `json:"duration_ns"`
			Status        string         `json:"status"`
			StatusMessage string         `json:"status_message"`
			Attributes    map[string]any `json:"attributes"`
			Events        []struct {
				Name     string `json:"name"`
				OffsetNs int64  `json:"offset_ns"`
			} `json:"events"`
		} `json:"spans"`
	} `json:"data"`
}

func TestGetTraceReturnsSpansInOrderWithLogCount(t *testing.T) {
	server, store := tracesServer(t, nil, 0)
	_, key := createAppWithKey(t, server, "checkout-api")
	seedReadTraces(t, server, key)
	store.Flush(5 * time.Second)

	// Two product log lines carry the trace id; one carries another.
	for _, line := range []string{
		`{"body":"charging card","trace_id":"` + traceHex(1) + `"}`,
		`{"body":"charged","trace_id":"` + traceHex(1) + `"}`,
		`{"body":"other","trace_id":"` + traceHex(2) + `"}`,
	} {
		if resp := ingest(server, key, "application/json", line); resp.Code != http.StatusAccepted {
			t.Fatalf("log ingest = %d", resp.Code)
		}
	}
	server.logs.store.Flush(5 * time.Second)

	for _, id := range []string{traceHex(1), strings.ToUpper(traceHex(1))} {
		resp := performJSONRequest(t, server, http.MethodGet, "/v1/traces/"+url.PathEscape(id), nil, "")
		if resp.Code != http.StatusOK {
			t.Fatalf("GET trace %q = %d %s", id, resp.Code, resp.Body.String())
		}
		var got traceDetailBody
		if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		d := got.Data
		if d.TraceID != traceHex(1) || d.RootName != "POST /checkout" || d.ServiceName != "checkout-api" || !d.HasRoot || d.SpanCount != 2 || d.ErrorCount != 1 ||
			d.Status != "error" || d.Truncated || d.LogCount != 2 || d.DurationNs != (800*time.Millisecond).Nanoseconds() {
			t.Fatalf("trace = %+v", d)
		}
		if len(d.Spans) != 2 || d.Spans[0].Name != "POST /checkout" || d.Spans[1].Name != "db.query" {
			t.Fatalf("spans = %+v, want root then child", d.Spans)
		}
		root, child := d.Spans[0], d.Spans[1]
		if root.StartOffsetNs != 0 || root.Kind != "server" || root.AppName != "checkout-api" || root.ParentSpanID != "" ||
			child.ParentSpanID != root.SpanID || child.StartOffsetNs != time.Millisecond.Nanoseconds() || child.Status != "error" || child.StatusMessage != "boom" ||
			child.Attributes["db.system"] != "postgres" || len(child.Events) != 1 || child.Events[0].OffsetNs != (2*time.Millisecond).Nanoseconds() {
			t.Fatalf("root = %+v child = %+v", root, child)
		}
	}
}

func TestGetTraceErrors(t *testing.T) {
	server, store := tracesServer(t, nil, 0)
	_, key := createAppWithKey(t, server, "checkout-api")
	// A trace of orphans only: the root never arrived.
	sendTraces(t, server, store, key, otlpSpan(5, 7, 6, "late child", time.Now().Add(-time.Minute), 30*time.Millisecond, false))

	if resp := performJSONRequest(t, server, http.MethodGet, "/v1/traces/"+traceHex(99), nil, ""); resp.Code != http.StatusNotFound {
		t.Fatalf("unknown trace = %d, want 404", resp.Code)
	}
	for _, bad := range []string{"abc", "zz" + traceHex(5)[2:], traceHex(5) + "00"} {
		if resp := performJSONRequest(t, server, http.MethodGet, "/v1/traces/"+bad, nil, ""); resp.Code != http.StatusBadRequest {
			t.Fatalf("trace id %q = %d, want 400", bad, resp.Code)
		}
	}
	var got traceDetailBody
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/traces/"+traceHex(5), nil, ""), &got)
	if got.Data.HasRoot || got.Data.RootName != "late child" || got.Data.DurationNs != (30*time.Millisecond).Nanoseconds() {
		t.Fatalf("orphan trace = %+v: it is shown as it arrived", got.Data)
	}
}

func TestTraceFacets(t *testing.T) {
	server, _ := tracesServer(t, nil, 0)
	_, key := createAppWithKey(t, server, "checkout-api")
	seedReadTraces(t, server, key)
	type facets struct {
		Data struct {
			Total    int64 `json:"total"`
			Services []struct {
				Value string `json:"value"`
				Count int64  `json:"count"`
			} `json:"services"`
			Names []struct {
				Value string `json:"value"`
			} `json:"names"`
			Status []struct {
				Value string `json:"value"`
				Count int64  `json:"count"`
			} `json:"status"`
		} `json:"data"`
	}
	get := func(query string) facets {
		resp := performJSONRequest(t, server, http.MethodGet, "/v1/traces/facets?"+query, nil, "")
		if resp.Code != http.StatusOK {
			t.Fatalf("facets %q = %d %s", query, resp.Code, resp.Body.String())
		}
		var out facets
		decodeResponse(t, resp, &out)
		return out
	}
	// With no from, the last 24 hours count: the 30 hour old trace is outside.
	f := get("")
	if f.Data.Total != 2 || len(f.Data.Services) != 1 || f.Data.Services[0].Value != "checkout-api" || f.Data.Services[0].Count != 2 || len(f.Data.Names) != 2 {
		t.Fatalf("default facets = %+v", f.Data)
	}
	if len(f.Data.Status) != 2 || f.Data.Status[0].Count != 1 || f.Data.Status[1].Count != 1 {
		t.Fatalf("status counts = %+v", f.Data.Status)
	}
	if f := get("from=-2d"); f.Data.Total != 3 {
		t.Fatalf("facets over two days total = %d, want 3", f.Data.Total)
	}
	if f := get("status=error&from=-2d"); f.Data.Total != 3 || len(f.Data.Names) != 1 || f.Data.Names[0].Value != "POST /checkout" {
		t.Fatalf("narrowed facets = %+v (status counts keep both choices, lists follow the filter)", f.Data)
	}
	if resp := performJSONRequest(t, server, http.MethodGet, "/v1/traces/facets?status=great", nil, ""); resp.Code != http.StatusBadRequest {
		t.Fatalf("bad status = %d, want 400", resp.Code)
	}
	if resp := performJSONRequest(t, server, http.MethodGet, "/v1/traces/facets?service_id=svc-nope", nil, ""); resp.Code != http.StatusNotFound {
		t.Fatalf("unknown service = %d, want 404", resp.Code)
	}
}

func TestServiceTraceSummary(t *testing.T) {
	server, store := tracesServer(t, nil, 0)
	appID, key := createAppWithKey(t, server, "checkout-api")
	svc := createServiceVia(t, server, gin.H{"name": "Checkout"})
	idle := createServiceVia(t, server, gin.H{"name": "Idle"})
	performJSONRequest(t, server, http.MethodPost, "/v1/services/"+svc+"/links", gin.H{"kind": "app", "id": appID}, "")

	now := time.Now().UTC().Add(-20 * time.Minute)
	var spans []*tracepb.Span
	for i := 1; i <= 20; i++ { // 20 traces of 100ms..2000ms, the last 4 with a failed child
		spans = append(spans, otlpSpan(i, i, 0, "GET /", now.Add(time.Duration(i)*time.Second), time.Duration(i)*100*time.Millisecond, false))
		if i > 16 {
			spans = append(spans, otlpSpan(i, 100+i, i, "db", now.Add(time.Duration(i)*time.Second), time.Millisecond, true))
		}
	}
	sendTraces(t, server, store, key, spans...)
	sendTraces(t, server, store, key, otlpSpan(50, 150, 149, "orphan", now, 9*time.Second, false)) // no root: counted, not in the latency

	type summary struct {
		Data struct {
			ServiceID   string   `json:"service_id"`
			AppIDs      []string `json:"app_ids"`
			TraceCount  int64    `json:"trace_count"`
			ErrorCount  int64    `json:"error_count"`
			ErrorRate   float64  `json:"error_rate"`
			RootedCount int64    `json:"rooted_count"`
			P50Ns       *int64   `json:"p50_ns"`
			P95Ns       *int64   `json:"p95_ns"`
			P99Ns       *int64   `json:"p99_ns"`
			Sampled     bool     `json:"sampled"`
		} `json:"data"`
	}
	get := func(id, query string, want int) summary {
		resp := performJSONRequest(t, server, http.MethodGet, "/v1/services/"+id+"/traces/summary"+query, nil, "")
		if resp.Code != want {
			t.Fatalf("summary %s%s = %d %s", id, query, resp.Code, resp.Body.String())
		}
		var out summary
		if want == http.StatusOK {
			decodeResponse(t, resp, &out)
		}
		return out
	}
	d := get(svc, "?from=-1h", http.StatusOK).Data
	ms := func(n int) int64 { return (time.Duration(n) * time.Millisecond).Nanoseconds() }
	if d.ServiceID != svc || len(d.AppIDs) != 1 || d.TraceCount != 21 || d.ErrorCount != 4 || d.RootedCount != 20 || d.Sampled {
		t.Fatalf("counts = %+v", d)
	}
	if d.ErrorRate < 0.19 || d.ErrorRate > 0.191 {
		t.Fatalf("error rate = %v, want 4/21", d.ErrorRate)
	}
	if d.P50Ns == nil || *d.P50Ns != ms(1000) || *d.P95Ns != ms(1900) || *d.P99Ns != ms(2000) {
		t.Fatalf("percentiles = %v %v %v, want 1000ms 1900ms 2000ms", d.P50Ns, d.P95Ns, d.P99Ns)
	}
	// The default window is the last hour; a window that misses the data answers zeros and null percentiles.
	if d := get(svc, "", http.StatusOK).Data; d.TraceCount != 21 {
		t.Fatalf("default window count = %d", d.TraceCount)
	}
	if d := get(svc, "?from=-3h&to=-2h", http.StatusOK).Data; d.TraceCount != 0 || d.ErrorRate != 0 || d.P50Ns != nil {
		t.Fatalf("empty window = %+v", d)
	}
	if d := get(idle, "", http.StatusOK).Data; d.TraceCount != 0 || len(d.AppIDs) != 0 || d.P99Ns != nil {
		t.Fatalf("service without apps = %+v", d)
	}
	get("svc-nope", "", http.StatusNotFound)
	get(svc, "?from=-1h&to=-2h", http.StatusBadRequest)
	get(svc, "?from=-8d", http.StatusBadRequest)
	get(svc, "?from=soon", http.StatusBadRequest)
}

func TestDeletingAnAppRemovesItsTraces(t *testing.T) {
	server, store := tracesServer(t, nil, 0)
	appID, key := createAppWithKey(t, server, "short-lived")
	sendTraces(t, server, store, key, otlpSpan(1, 1, 0, "root", time.Now().Add(-time.Minute), time.Millisecond, false))
	if resp := performJSONRequest(t, server, http.MethodDelete, "/v1/apps/"+appID, nil, ""); resp.Code != http.StatusOK {
		t.Fatalf("delete app = %d %s", resp.Code, resp.Body.String())
	}
	deadline := time.Now().Add(5 * time.Second) // the removal runs in the background
	for time.Now().Before(deadline) {
		if n, _, _ := store.CountForApp(appID); n == 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("the deleted app's spans are still stored")
}
