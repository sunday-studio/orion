package api

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"orion/core/internal/service"
)

func reportWithAppResources(t *testing.T, server *Server, nodeID, token string, app map[string]any) {
	t.Helper()
	body := map[string]any{"uptime_seconds": 120, "timestamp": time.Now().UTC().Format(time.RFC3339),
		"cpu": map[string]any{"cores": 4, "usage_percent": 12.5}, "memory": map[string]any{"total_bytes": 1024, "used_bytes": 512, "used_percent": 50},
		"disk": map[string]any{"total_bytes": 2048, "used_bytes": 1024, "used_percent": 50}}
	if app != nil {
		body["app_resources"] = app
	}
	if resp := performJSONRequest(t, server, http.MethodPost, "/v1/nodes/"+nodeID+"/report", body, token); resp.Code != http.StatusOK {
		t.Fatalf("report status = %d: %s", resp.Code, resp.Body.String())
	}
}

type appResourcesResp struct {
	Data struct {
		AppResources *service.NodeAppResourcesView `json:"app_resources"`
	} `json:"data"`
}

func TestAppResourcesAreStoredAsTheLatestReportAndReplacedNotAccumulated(t *testing.T) {
	server := setupTestServer(t)
	node := registerTestNode(t, server)
	path := "/v1/nodes/" + node.Data.NodeID + "/app-resources"

	fetch := func() *service.NodeAppResourcesView {
		var r appResourcesResp // a fresh value each time: decoding into a used one merges fields
		decodeResponse(t, performJSONRequest(t, server, http.MethodGet, path, nil, ""), &r)
		return r.Data.AppResources
	}
	if got := fetch(); got != nil {
		t.Fatalf("before any report: %+v", got)
	}

	reportWithAppResources(t, server, node.Data.NodeID, node.Data.Token, map[string]any{
		"interval_seconds": 60.1,
		"series": []map[string]any{
			{"kind": "container", "key": "container:abc", "name": "db", "cpu_percent": 1.5, "memory_bytes": 52428800, "storage_bytes": 1073741824},
			{"kind": "path", "key": "path:/var/lib/app", "name": "/var/lib/app", "storage_bytes": 2048, "storage_capped": true},
			{"kind": "docker", "key": "docker:images", "name": "Docker images", "storage_bytes": 9999},
		},
		"notes": []string{"cgroup v2 not found"},
	})
	v := fetch()
	if v == nil || len(v.Series) != 3 || v.IntervalSeconds != 60.1 || len(v.Notes) != 1 {
		t.Fatalf("first report: %+v", v)
	}
	by := map[string]service.AppResourceSeries{}
	for _, s := range v.Series {
		by[s.Key] = s
	}
	if c := by["container:abc"]; c.CPUPercent == nil || *c.CPUPercent != 1.5 || c.MemoryBytes == nil || *c.MemoryBytes != 52428800 || c.NetRxBps != nil {
		t.Fatalf("container: %+v (an unmeasured field stays absent, not zero)", c)
	}
	if p := by["path:/var/lib/app"]; !p.StorageCapped || p.StorageBytes == nil || *p.StorageBytes != 2048 {
		t.Fatalf("path: %+v", p)
	}

	reportWithAppResources(t, server, node.Data.NodeID, node.Data.Token, map[string]any{
		"interval_seconds": 60,
		"series":           []map[string]any{{"kind": "container", "key": "container:abc", "name": "db", "cpu_percent": 9.0}},
	})
	v = fetch()
	if len(v.Series) != 1 || v.Series[0].CPUPercent == nil || *v.Series[0].CPUPercent != 9.0 || v.Series[0].MemoryBytes != nil || len(v.Notes) != 0 {
		t.Fatalf("a thing that is no longer reported disappears and fields are replaced, not merged: %+v", v)
	}

	reportWithAppResources(t, server, node.Data.NodeID, node.Data.Token, nil)
	if got := fetch(); got == nil || len(got.Series) != 1 {
		t.Fatalf("a report without the section leaves the last measurements alone: %+v", got)
	}
}

func TestOddSeriesAreDroppedWithoutRejectingTheReport(t *testing.T) {
	server := setupTestServer(t)
	node := registerTestNode(t, server)
	long := strings.Repeat("x", 400)
	reportWithAppResources(t, server, node.Data.NodeID, node.Data.Token, map[string]any{
		"series": []map[string]any{
			{"kind": "weird", "key": "weird:1", "name": "bad kind"},
			{"kind": "unit", "key": "", "name": "no key"},
			{"kind": "unit", "key": "unit:a", "name": long, "cpu_percent": -3.0, "pids": -1},
			{"kind": "unit", "key": "unit:a", "name": "duplicate"},
		},
	})
	var got appResourcesResp
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/nodes/"+node.Data.NodeID+"/app-resources", nil, ""), &got)
	v := got.Data.AppResources
	if v == nil || len(v.Series) != 1 || len([]rune(v.Series[0].Name)) != 300 || v.Series[0].CPUPercent != nil || v.Series[0].Pids != nil {
		t.Fatalf("series = %+v", v)
	}
}

func TestAppResourcesForAnUnknownNodeIs404(t *testing.T) {
	server := setupTestServer(t)
	if resp := performJSONRequest(t, server, http.MethodGet, "/v1/nodes/nope/app-resources", nil, ""); resp.Code != http.StatusNotFound {
		t.Fatalf("status = %d", resp.Code)
	}
}

func TestAMalformedAppResourcesSectionNeverCostsTheHostReport(t *testing.T) {
	server := setupTestServer(t)
	node := registerTestNode(t, server)
	many := make([]map[string]any, 0, 250)
	for i := 0; i < 250; i++ {
		many = append(many, map[string]any{"kind": "process", "key": "process:p" + strings.Repeat("x", i%5) + string(rune('a'+i%26)) + string(rune('a'+i/26)), "name": "p"})
	}
	sections := map[string]any{
		"negative uint":   map[string]any{"series": []map[string]any{{"kind": "unit", "key": "unit:a", "name": "a", "memory_bytes": -5, "cpu_percent": 2.5}}},
		"string number":   map[string]any{"interval_seconds": "soon", "series": []map[string]any{{"kind": "unit", "key": "unit:a", "name": "a", "cpu_percent": "high", "pids": "many"}}},
		"huge number":     map[string]any{"series": []map[string]any{{"kind": "path", "key": "path:/x", "name": "/x", "storage_bytes": 1.8e19}}},
		"series not list": map[string]any{"series": "nope"},
		"not an object":   []int{1, 2, 3},
		"over the cap":    map[string]any{"series": many},
	}
	for name, section := range sections {
		body := map[string]any{"uptime_seconds": 120, "timestamp": time.Now().UTC().Format(time.RFC3339),
			"cpu": map[string]any{"cores": 4, "usage_percent": 12.5}, "memory": map[string]any{"total_bytes": 1024, "used_bytes": 512, "used_percent": 50},
			"disk": map[string]any{"total_bytes": 2048, "used_bytes": 1024, "used_percent": 50}, "app_resources": section}
		if resp := performJSONRequest(t, server, http.MethodPost, "/v1/nodes/"+node.Data.NodeID+"/report", body, node.Data.Token); resp.Code != http.StatusOK {
			t.Fatalf("%s: report status = %d: %s (an odd section must never turn into a rejected host report)", name, resp.Code, resp.Body.String())
		}
	}
	reportWithAppResources(t, server, node.Data.NodeID, node.Data.Token, map[string]any{"series": many}) // after the loop: map order is random
	var r appResourcesResp
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/nodes/"+node.Data.NodeID+"/app-resources", nil, ""), &r)
	if v := r.Data.AppResources; v == nil || len(v.Series) != 200 {
		t.Fatalf("a report of 250 series keeps the first 200: %+v", v)
	}

	reportWithAppResources(t, server, node.Data.NodeID, node.Data.Token, map[string]any{"series": []map[string]any{
		{"kind": "path", "key": "path:/huge", "name": "/huge", "storage_bytes": 1.8e19},
		{"kind": "unit", "key": "unit:ok", "name": "ok", "memory_bytes": 1024}}})
	var huge appResourcesResp
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/nodes/"+node.Data.NodeID+"/app-resources", nil, ""), &huge)
	got := map[string]service.AppResourceSeries{}
	for _, x := range huge.Data.AppResources.Series {
		got[x.Key] = x
	}
	if h := got["path:/huge"]; h.StorageBytes == nil || *h.StorageBytes != 1<<63-1 || got["unit:ok"].MemoryBytes == nil {
		t.Fatalf("a huge count is clamped and stored, and its neighbours are kept: %+v", got)
	}

	reportWithAppResources(t, server, node.Data.NodeID, node.Data.Token, sections["negative uint"].(map[string]any))
	var r2 appResourcesResp
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/nodes/"+node.Data.NodeID+"/app-resources", nil, ""), &r2)
	s := r2.Data.AppResources.Series[0]
	if s.MemoryBytes != nil || s.CPUPercent == nil || *s.CPUPercent != 2.5 {
		t.Fatalf("a bad field is absent while the good ones are kept: %+v", s)
	}
}
