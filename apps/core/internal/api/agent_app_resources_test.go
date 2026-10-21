package api

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"orion/core/internal/service"
)

func reportWithAppResources(t *testing.T, server *Server, agentID, token string, app map[string]any) {
	t.Helper()
	body := map[string]any{"uptime_seconds": 120, "timestamp": time.Now().UTC().Format(time.RFC3339),
		"cpu": map[string]any{"cores": 4, "usage_percent": 12.5}, "memory": map[string]any{"total_bytes": 1024, "used_bytes": 512, "used_percent": 50},
		"disk": map[string]any{"total_bytes": 2048, "used_bytes": 1024, "used_percent": 50}}
	if app != nil {
		body["app_resources"] = app
	}
	if resp := performJSONRequest(t, server, http.MethodPost, "/v1/agents/"+agentID+"/report", body, token); resp.Code != http.StatusOK {
		t.Fatalf("report status = %d: %s", resp.Code, resp.Body.String())
	}
}

type appResourcesResp struct {
	Data struct {
		AppResources *service.AgentAppResourcesView `json:"app_resources"`
	} `json:"data"`
}

func TestAppResourcesAreStoredAsTheLatestReportAndReplacedNotAccumulated(t *testing.T) {
	server := setupTestServer(t)
	agent := registerTestAgent(t, server)
	path := "/v1/agents/" + agent.Data.AgentID + "/app-resources"

	fetch := func() *service.AgentAppResourcesView {
		var r appResourcesResp // a fresh value each time: decoding into a used one merges fields
		decodeResponse(t, performJSONRequest(t, server, http.MethodGet, path, nil, ""), &r)
		return r.Data.AppResources
	}
	if got := fetch(); got != nil {
		t.Fatalf("before any report: %+v", got)
	}

	reportWithAppResources(t, server, agent.Data.AgentID, agent.Data.Token, map[string]any{
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

	reportWithAppResources(t, server, agent.Data.AgentID, agent.Data.Token, map[string]any{
		"interval_seconds": 60,
		"series":           []map[string]any{{"kind": "container", "key": "container:abc", "name": "db", "cpu_percent": 9.0}},
	})
	v = fetch()
	if len(v.Series) != 1 || v.Series[0].CPUPercent == nil || *v.Series[0].CPUPercent != 9.0 || v.Series[0].MemoryBytes != nil || len(v.Notes) != 0 {
		t.Fatalf("a thing that is no longer reported disappears and fields are replaced, not merged: %+v", v)
	}

	reportWithAppResources(t, server, agent.Data.AgentID, agent.Data.Token, nil)
	if got := fetch(); got == nil || len(got.Series) != 1 {
		t.Fatalf("a report without the section leaves the last measurements alone: %+v", got)
	}
}

func TestOddSeriesAreDroppedWithoutRejectingTheReport(t *testing.T) {
	server := setupTestServer(t)
	agent := registerTestAgent(t, server)
	long := strings.Repeat("x", 400)
	reportWithAppResources(t, server, agent.Data.AgentID, agent.Data.Token, map[string]any{
		"series": []map[string]any{
			{"kind": "weird", "key": "weird:1", "name": "bad kind"},
			{"kind": "unit", "key": "", "name": "no key"},
			{"kind": "unit", "key": "unit:a", "name": long, "cpu_percent": -3.0, "pids": -1},
			{"kind": "unit", "key": "unit:a", "name": "duplicate"},
		},
	})
	var got appResourcesResp
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/agents/"+agent.Data.AgentID+"/app-resources", nil, ""), &got)
	v := got.Data.AppResources
	if v == nil || len(v.Series) != 1 || len([]rune(v.Series[0].Name)) != 300 || v.Series[0].CPUPercent != nil || v.Series[0].Pids != nil {
		t.Fatalf("series = %+v", v)
	}
}

func TestAppResourcesForAnUnknownAgentIs404(t *testing.T) {
	server := setupTestServer(t)
	if resp := performJSONRequest(t, server, http.MethodGet, "/v1/agents/nope/app-resources", nil, ""); resp.Code != http.StatusNotFound {
		t.Fatalf("status = %d", resp.Code)
	}
}
