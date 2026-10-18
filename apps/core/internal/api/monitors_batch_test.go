package api

import (
	"net/http"
	"net/http/httptest"
	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestMonitorMaintenanceHoldsIncidentsAndBulkActions(t *testing.T) {
	server := setupTestServer(t)
	agent := registerTestAgent(t, server)
	a := createHTTPMonitor(t, server, "api-https", nil)
	b := createHTTPMonitor(t, server, "web-https", nil)
	c := createHTTPMonitor(t, server, "pg-tcp", gin.H{"kind": "tcp", "config": gin.H{"host": "db-02.example.com", "port": 5432}})

	// Bulk maintenance on a + b; the window shows on the rows and validation holds.
	r := performJSONRequest(t, server, http.MethodPost, "/v1/monitors/bulk", gin.H{"ids": []string{a.Data.Monitor.ID, b.Data.Monitor.ID, "missing"}, "action": "maintenance", "maintenance": gin.H{"duration_seconds": 3600, "reason": "Database upgrade"}}, "")
	var bulk struct {
		Data struct {
			Results   []BulkMonitorResult `json:"results"`
			Succeeded int                 `json:"succeeded"`
			Failed    int                 `json:"failed"`
		} `json:"data"`
	}
	decodeResponse(t, r, &bulk)
	if r.Code != http.StatusOK || bulk.Data.Succeeded != 2 || bulk.Data.Failed != 1 || bulk.Data.Results[2].OK {
		t.Fatalf("bulk maintenance = %d %+v", r.Code, bulk.Data)
	}
	var listed struct {
		Data struct {
			Monitors []MonitorResponse `json:"monitors"`
		} `json:"data"`
	}
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/monitors?q=api-https", nil, ""), &listed)
	m := listed.Data.Monitors[0]
	if m.Maintenance == nil || !m.Maintenance.Active || m.Maintenance.Reason != "Database upgrade" || m.StatusText != "Maintenance" {
		t.Fatalf("maintenance row = %+v", m)
	}
	for name, body := range map[string]gin.H{
		"no end":      {"ids": []string{c.Data.Monitor.ID}, "action": "maintenance", "maintenance": gin.H{"reason": "x"}},
		"no settings": {"ids": []string{c.Data.Monitor.ID}, "action": "maintenance"},
		"bad action":  {"ids": []string{c.Data.Monitor.ID}, "action": "explode"},
		"empty ids":   {"ids": []string{}, "action": "pause"},
	} {
		if r := performJSONRequest(t, server, http.MethodPost, "/v1/monitors/bulk", body, ""); r.Code != http.StatusBadRequest {
			t.Errorf("%s = %d", name, r.Code)
		}
	}
	if r := performJSONRequest(t, server, http.MethodPut, "/v1/monitors/"+c.Data.Monitor.ID+"/maintenance", gin.H{"duration_seconds": 90 * 86400}, ""); r.Code != http.StatusBadRequest {
		t.Errorf("90 day window = %d", r.Code)
	}

	// A failing report during maintenance opens no incident; after clearing, it does.
	var incidents int64
	failing := func() {
		server.db.Model(&db.Monitor{}).Where("id = ?", a.Data.Monitor.ID).Update("agent_id", agent.Data.AgentID)
		payload := service.MonitorReportPayload{Timestamp: time.Now().UTC().Format(time.RFC3339), Health: "down", Error: "boom"}
		if _, err := server.reportService.StoreMonitorReport(a.Data.Monitor.ID, payload); err != nil {
			t.Fatal(err)
		}
	}
	failing()
	server.db.Model(&db.Incident{}).Where("monitor_id = ?", a.Data.Monitor.ID).Count(&incidents)
	if incidents != 0 {
		t.Fatalf("incident opened during maintenance: %d", incidents)
	}
	if r := performJSONRequest(t, server, http.MethodDelete, "/v1/monitors/"+a.Data.Monitor.ID+"/maintenance", nil, ""); r.Code != http.StatusOK {
		t.Fatalf("clear = %d", r.Code)
	}
	failing()
	server.db.Model(&db.Incident{}).Where("monitor_id = ?", a.Data.Monitor.ID).Count(&incidents)
	if incidents != 1 {
		t.Fatalf("incident after maintenance cleared: %d, want 1", incidents)
	}

	// Pause, resume, clone, delete in bulk.
	ids := []string{b.Data.Monitor.ID, c.Data.Monitor.ID}
	for _, action := range []string{"pause", "resume", "clone"} {
		decodeResponse(t, performJSONRequest(t, server, http.MethodPost, "/v1/monitors/bulk", gin.H{"ids": ids, "action": action}, ""), &bulk)
		if bulk.Data.Succeeded != 2 {
			t.Fatalf("bulk %s = %+v", action, bulk.Data)
		}
		if action == "clone" && (bulk.Data.Results[0].NewID == "" || bulk.Data.Results[0].NewID == ids[0]) {
			t.Fatalf("clone result = %+v", bulk.Data.Results)
		}
	}
	decodeResponse(t, performJSONRequest(t, server, http.MethodPost, "/v1/monitors/bulk", gin.H{"ids": ids, "action": "delete", "keep_history": false}, ""), &bulk)
	var left int64
	server.db.Model(&db.Monitor{}).Where("id IN ?", ids).Count(&left)
	if bulk.Data.Succeeded != 2 || left != 0 {
		t.Fatalf("bulk delete = %+v left=%d", bulk.Data, left)
	}
	// Non-Core monitors cannot be paused here.
	server.db.Create(&db.Monitor{ID: "agent-mon", Type: "disk", Name: "disk", AgentID: agent.Data.AgentID, Lifecycle: "active", Health: "up", ComputedHealth: "up"})
	decodeResponse(t, performJSONRequest(t, server, http.MethodPost, "/v1/monitors/bulk", gin.H{"ids": []string{"agent-mon"}, "action": "pause"}, ""), &bulk)
	if bulk.Data.Failed != 1 || !strings.Contains(bulk.Data.Results[0].Error, "Core-managed") {
		t.Fatalf("pause agent monitor = %+v", bulk.Data)
	}
}

func TestTestRunDoesNotRecordAnything(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/bad" {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()
	server := setupTestServerWithConfig(t, &config.Config{CoreMonitorAllowPrivateTargets: true})
	registerTestAgent(t, server)

	run := func(path string, body any) (int, MonitorTestRunResponse) {
		resp := performJSONRequest(t, server, http.MethodPost, path, body, "")
		var out struct {
			Data MonitorTestRunResponse `json:"data"`
		}
		if resp.Code == http.StatusOK {
			decodeResponse(t, resp, &out)
		}
		return resp.Code, out.Data
	}
	code, ok := run("/v1/monitors/test-run", gin.H{"kind": "http", "config": gin.H{"url": upstream.URL + "/ok", "method": "GET"}})
	if code != http.StatusOK || !ok.Passed || ok.Status != "passed" || ok.Expected == nil || ok.StatusCode == nil || *ok.StatusCode != 200 || len(ok.Notes) == 0 {
		t.Fatalf("passing unsaved run = %d %+v", code, ok)
	}
	code, bad := run("/v1/monitors/test-run", gin.H{"kind": "http", "config": gin.H{"url": upstream.URL + "/bad", "method": "GET"}})
	if code != http.StatusOK || bad.Passed || bad.Status != "failed" || bad.StatusCode == nil || *bad.StatusCode != 502 || bad.Actual.Title == "" {
		t.Fatalf("failing unsaved run = %d %+v", code, bad)
	}
	// Saved monitor variant, and nothing is written to the real database.
	saved := createHTTPMonitor(t, server, "bad-upstream", gin.H{"config": gin.H{"url": upstream.URL + "/bad", "method": "GET"}})
	code, again := run("/v1/monitors/"+saved.Data.Monitor.ID+"/test-run", nil)
	if code != http.StatusOK || again.Passed {
		t.Fatalf("saved run = %d %+v", code, again)
	}
	var reports, incidents int64
	server.db.Model(&db.MonitorReport{}).Count(&reports)
	server.db.Model(&db.Incident{}).Count(&incidents)
	if reports != 0 || incidents != 0 {
		t.Fatalf("test runs stored reports=%d incidents=%d", reports, incidents)
	}
	if code, _ := run("/v1/monitors/test-run", gin.H{"kind": "http", "config": gin.H{"url": "ftp://x"}}); code != http.StatusBadRequest {
		t.Fatalf("invalid config = %d", code)
	}
	hb := createHTTPMonitor(t, server, "beat", gin.H{"kind": "heartbeat", "interval_seconds": 3600, "config": gin.H{"grace_seconds": 60}})
	if code, _ := run("/v1/monitors/"+hb.Data.Monitor.ID+"/test-run", nil); code != http.StatusBadRequest {
		t.Fatalf("heartbeat test run = %d", code)
	}
	if code, _ := run("/v1/monitors/nope/test-run", nil); code != http.StatusNotFound {
		t.Fatalf("unknown = %d", code)
	}
}

func TestAgentMonitorDefinitionsAreConfiguredPulledAndReportWaitingThenReporting(t *testing.T) {
	server := setupTestServer(t)
	agent := registerTestAgent(t, server)
	id := agent.Data.AgentID
	path := "/v1/agents/" + id + "/monitor-definitions"

	resp := performJSONRequest(t, server, http.MethodPost, path, gin.H{"name": "Disk usage", "check_type": "disk", "params": gin.H{"path": "/var", "threshold_percent": 85}, "interval_seconds": 30, "fail_after": 3}, "")
	var created struct {
		Data struct {
			Definition AgentMonitorDefinitionResponse `json:"definition"`
		} `json:"data"`
	}
	decodeResponse(t, resp, &created)
	d := created.Data.Definition
	if resp.Code != http.StatusCreated || d.State != "waiting" || d.CheckType != "disk" || d.Params["path"] != "/var" || d.Params["threshold_percent"].(float64) != 85 {
		t.Fatalf("create = %d %+v", resp.Code, d)
	}
	if r := performJSONRequest(t, server, http.MethodPost, path, gin.H{"name": "Disk usage", "check_type": "memory"}, ""); r.Code != http.StatusConflict {
		t.Fatalf("duplicate name = %d", r.Code)
	}
	for name, body := range map[string]gin.H{
		"bad type":     {"name": "x", "check_type": "gpu"},
		"bad path":     {"name": "x", "check_type": "disk", "params": gin.H{"path": "../etc"}},
		"bad process":  {"name": "x", "check_type": "process", "params": gin.H{"process_name": "a;rm -rf /"}},
		"bad port":     {"name": "x", "check_type": "port", "params": gin.H{"port": 70000}},
		"bad interval": {"name": "x", "check_type": "memory", "interval_seconds": 2},
	} {
		if r := performJSONRequest(t, server, http.MethodPost, path, body, ""); r.Code != http.StatusBadRequest {
			t.Errorf("%s = %d, want 400", name, r.Code)
		}
	}
	if r := performJSONRequest(t, server, http.MethodPost, "/v1/agents/nope/monitor-definitions", gin.H{"name": "x", "check_type": "memory"}, ""); r.Code != http.StatusNotFound {
		t.Errorf("unknown agent = %d", r.Code)
	}

	// The agent pulls with its own token; another agent's token cannot read it.
	pulled := performJSONRequest(t, server, http.MethodPost, path+"/sync", nil, agent.Data.Token)
	if pulled.Code != http.StatusOK || !strings.Contains(pulled.Body.String(), `"check_type":"disk"`) {
		t.Fatalf("pull = %d %s", pulled.Code, pulled.Body.String())
	}
	if r := performJSONRequest(t, server, http.MethodPost, path+"/sync", nil, "wrong"); r.Code != http.StatusUnauthorized {
		t.Fatalf("pull with a bad token = %d", r.Code)
	}

	// The agent registers a monitor with that name and reports: state becomes reporting.
	server.db.Create(&db.Monitor{ID: "mon-disk", Type: "disk", Name: "Disk usage", AgentID: id, Lifecycle: "active", Health: "up", ComputedHealth: "up"})
	var list struct {
		Data struct {
			Definitions []AgentMonitorDefinitionResponse `json:"definitions"`
		} `json:"data"`
	}
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, path, nil, ""), &list)
	if list.Data.Definitions[0].State != "waiting" || list.Data.Definitions[0].MonitorID != "mon-disk" {
		t.Fatalf("registered but silent = %+v", list.Data.Definitions[0])
	}
	addChecks(server, "mon-disk", "up")
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, path, nil, ""), &list)
	if list.Data.Definitions[0].State != "reporting" || list.Data.Definitions[0].FirstReport == nil {
		t.Fatalf("after first report = %+v", list.Data.Definitions[0])
	}
	if r := performJSONRequest(t, server, http.MethodDelete, path+"/"+d.ID, nil, ""); r.Code != http.StatusOK {
		t.Fatalf("delete = %d", r.Code)
	}
	if r := performJSONRequest(t, server, http.MethodDelete, path+"/"+d.ID, nil, ""); r.Code != http.StatusNotFound {
		t.Fatalf("second delete = %d", r.Code)
	}
}
