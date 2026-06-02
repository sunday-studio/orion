package api

import (
	"net/http"
	"orion/core/internal/db"
	"testing"
	"time"
)

func TestAgentHealthSplitsAvailabilityFromMonitorRollup(t *testing.T) {
	server := setupTestServer(t)
	agent := db.Agent{ID: "agent-split-health", MachineId: "machine-split-health", Name: "split health", OS: "linux", Arch: "arm64", Token: "token-split-health", LastSeen: time.Now()}
	if err := server.db.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	monitors := []db.Monitor{{ID: "monitor-split-up", AgentID: agent.ID, Name: "split up", Type: "http", Lifecycle: "active", Health: "up", ReportingIntervalSeconds: 60}, {ID: "monitor-split-down", AgentID: agent.ID, Name: "split down", Type: "http", Lifecycle: "active", Health: "down", ReportingIntervalSeconds: 60}}
	if err := server.db.Create(&monitors).Error; err != nil {
		t.Fatalf("create monitors: %v", err)
	}
	now := time.Now().UTC()
	reports := []db.MonitorReport{{ID: "report-split-up", MonitorID: "monitor-split-up", Payload: "{}", CollectedAt: now.Format(time.RFC3339), Health: "up", CreatedAt: now}, {ID: "report-split-down", MonitorID: "monitor-split-down", Payload: "{}", CollectedAt: now.Format(time.RFC3339), Health: "down", CreatedAt: now}}
	if err := server.db.Create(&reports).Error; err != nil {
		t.Fatalf("create reports: %v", err)
	}
	resp := performJSONRequest(t, server, http.MethodGet, "/v1/agents/"+agent.ID+"/health", nil, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("health status = %d, body = %s", resp.Code, resp.Body.String())
	}
	var health struct {
		Success bool `json:"success"`
		Data    struct {
			OverallHealth      string `json:"overall_health"`
			AvailabilityHealth string `json:"availability_health"`
			MonitorHealth      string `json:"monitor_health"`
			StatusReason       string `json:"status_reason"`
			UpCount            int    `json:"up_count"`
			DownCount          int    `json:"down_count"`
			TotalCount         int    `json:"total_count"`
		} `json:"data"`
	}
	decodeResponse(t, resp, &health)
	if !health.Success {
		t.Fatalf("health response = %+v, want success", health)
	}
	if health.Data.AvailabilityHealth != "up" || health.Data.MonitorHealth != "degraded" || health.Data.OverallHealth != "degraded" {
		t.Fatalf("health response = %+v, want live degraded agent", health.Data)
	}
	if health.Data.UpCount != 1 || health.Data.DownCount != 1 || health.Data.TotalCount != 2 {
		t.Fatalf("health counts = %+v, want 1 up, 1 down, 2 total", health.Data)
	}
	if health.Data.StatusReason == "" {
		t.Fatalf("status reason is empty")
	}
}
func TestSystemHealthSeparatesStaleMonitorCounts(t *testing.T) {
	server := setupTestServer(t)
	agent := db.Agent{ID: "agent-system-health-stale", MachineId: "machine-system-health-stale", Name: "system health stale", OS: "linux", Arch: "arm64", Token: "token-system-health-stale", LastSeen: time.Now()}
	if err := server.db.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	monitors := []db.Monitor{{ID: "monitor-system-health-up", AgentID: agent.ID, Name: "fresh monitor", Type: "http", Lifecycle: "active", Health: "up"}, {ID: "monitor-system-health-stale", AgentID: agent.ID, Name: "stale monitor", Type: "http", Lifecycle: "active", Health: "up"}}
	if err := server.db.Create(&monitors).Error; err != nil {
		t.Fatalf("create monitors: %v", err)
	}
	freshTime := time.Now().UTC()
	oldTime := freshTime.Add(-30 * time.Minute)
	reports := []db.MonitorReport{{ID: "monitor-report-system-health-up", MonitorID: "monitor-system-health-up", Payload: "{}", CollectedAt: freshTime.Format(time.RFC3339), Health: "up", CreatedAt: freshTime}, {ID: "monitor-report-system-health-stale", MonitorID: "monitor-system-health-stale", Payload: "{}", CollectedAt: oldTime.Format(time.RFC3339), Health: "up", CreatedAt: oldTime}}
	if err := server.db.Create(&reports).Error; err != nil {
		t.Fatalf("create reports: %v", err)
	}
	resp := performJSONRequest(t, server, http.MethodGet, "/v1/health/summary", nil, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("health summary status = %d, body = %s", resp.Code, resp.Body.String())
	}
	var summary struct {
		Success bool `json:"success"`
		Data    struct {
			OverallHealth string `json:"overall_health"`
			Monitors      struct {
				Up      int `json:"up"`
				Unknown int `json:"unknown"`
				Stale   int `json:"stale"`
			} `json:"monitors"`
		} `json:"data"`
	}
	decodeResponse(t, resp, &summary)
	if !summary.Success || summary.Data.OverallHealth != "stale" {
		t.Fatalf("summary = %+v, want overall stale", summary)
	}
	if summary.Data.Monitors.Up != 1 || summary.Data.Monitors.Stale != 1 || summary.Data.Monitors.Unknown != 0 {
		t.Fatalf("monitor counts = %+v, want up 1 stale 1 unknown 0", summary.Data.Monitors)
	}
	issuesResp := performJSONRequest(t, server, http.MethodGet, "/v1/health/issues", nil, "")
	if issuesResp.Code != http.StatusOK {
		t.Fatalf("health issues status = %d, body = %s", issuesResp.Code, issuesResp.Body.String())
	}
	var issues struct {
		Success bool `json:"success"`
		Data    struct {
			Issues []struct {
				MonitorID string `json:"monitor_id"`
				Health    string `json:"health"`
				IssueType string `json:"issue_type"`
			} `json:"issues"`
			Count int `json:"count"`
		} `json:"data"`
	}
	decodeResponse(t, issuesResp, &issues)
	if !issues.Success || issues.Data.Count != 1 || len(issues.Data.Issues) != 1 {
		t.Fatalf("health issues = %+v, want one stale issue", issues)
	}
	issue := issues.Data.Issues[0]
	if issue.MonitorID != "monitor-system-health-stale" || issue.Health != "stale" || issue.IssueType != "stale_data" {
		t.Fatalf("health issue = %+v, want stale monitor issue", issue)
	}
}
func TestListMonitorsCountMatchesFilters(t *testing.T) {
	server := setupTestServer(t)
	agent := db.Agent{ID: "agent-monitor-filter-counts", MachineId: "machine-monitor-filter-counts", Name: "monitor filter counts", OS: "linux", Arch: "arm64", Token: "token-monitor-filter-counts", LastSeen: time.Now()}
	if err := server.db.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	monitors := []db.Monitor{{ID: "monitor-filter-up", AgentID: agent.ID, Name: "up", Type: "http", Lifecycle: "active", Health: "up"}, {ID: "monitor-filter-down", AgentID: agent.ID, Name: "down", Type: "http", Lifecycle: "active", Health: "down"}, {ID: "monitor-filter-disabled-down", AgentID: agent.ID, Name: "disabled down", Type: "http", Lifecycle: "disabled", Health: "down"}}
	if err := server.db.Create(&monitors).Error; err != nil {
		t.Fatalf("create monitors: %v", err)
	}
	reportTime := time.Now().UTC()
	reports := []db.MonitorReport{{ID: "monitor-report-filter-up", MonitorID: "monitor-filter-up", Payload: "{}", CollectedAt: reportTime.Format(time.RFC3339), Health: "up", CreatedAt: reportTime}, {ID: "monitor-report-filter-down", MonitorID: "monitor-filter-down", Payload: "{}", CollectedAt: reportTime.Format(time.RFC3339), Health: "down", CreatedAt: reportTime}, {ID: "monitor-report-filter-disabled-down", MonitorID: "monitor-filter-disabled-down", Payload: "{}", CollectedAt: reportTime.Format(time.RFC3339), Health: "down", CreatedAt: reportTime}}
	if err := server.db.Create(&reports).Error; err != nil {
		t.Fatalf("create monitor reports: %v", err)
	}
	downResp := performJSONRequest(t, server, http.MethodGet, "/v1/agents/"+agent.ID+"/monitors?health=down", nil, "")
	if downResp.Code != http.StatusOK {
		t.Fatalf("down monitors status = %d, body = %s", downResp.Code, downResp.Body.String())
	}
	var listed struct {
		Success bool `json:"success"`
		Data    struct {
			Monitors []struct {
				ID        string `json:"id"`
				Health    string `json:"health"`
				Lifecycle string `json:"lifecycle"`
			} `json:"monitors"`
			Count int64 `json:"count"`
		} `json:"data"`
	}
	decodeResponse(t, downResp, &listed)
	if !listed.Success || listed.Data.Count != 1 || len(listed.Data.Monitors) != 1 {
		t.Fatalf("filtered active down monitors = %+v, want one row and count one", listed)
	}
	if listed.Data.Monitors[0].Health != "down" || listed.Data.Monitors[0].Lifecycle != "active" {
		t.Fatalf("filtered monitor = %+v, want active down", listed.Data.Monitors[0])
	}
	disabledResp := performJSONRequest(t, server, http.MethodGet, "/v1/agents/"+agent.ID+"/monitors?health=down&lifecycle=disabled", nil, "")
	if disabledResp.Code != http.StatusOK {
		t.Fatalf("disabled monitors status = %d, body = %s", disabledResp.Code, disabledResp.Body.String())
	}
	decodeResponse(t, disabledResp, &listed)
	if !listed.Success || listed.Data.Count != 1 || len(listed.Data.Monitors) != 1 {
		t.Fatalf("filtered disabled down monitors = %+v, want one row and count one", listed)
	}
	if listed.Data.Monitors[0].Health != "down" || listed.Data.Monitors[0].Lifecycle != "disabled" {
		t.Fatalf("filtered monitor = %+v, want disabled down", listed.Data.Monitors[0])
	}
}
func TestListAllMonitorsAndSummaryUseDerivedStaleState(t *testing.T) {
	server := setupTestServer(t)
	agent := db.Agent{ID: "agent-all-monitor-summary", MachineId: "machine-all-monitor-summary", Name: "all monitor summary", OS: "linux", Arch: "arm64", Token: "token-all-monitor-summary", LastSeen: time.Now()}
	if err := server.db.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	monitors := []db.Monitor{{ID: "monitor-all-up", AgentID: agent.ID, Name: "all up", Type: "http", Lifecycle: "active", Health: "up"}, {ID: "monitor-all-down", AgentID: agent.ID, Name: "all down", Type: "tcp", Lifecycle: "active", Health: "down"}, {ID: "monitor-all-stale", AgentID: agent.ID, Name: "all stale", Type: "http", Lifecycle: "active", Health: "up"}}
	if err := server.db.Create(&monitors).Error; err != nil {
		t.Fatalf("create monitors: %v", err)
	}
	freshTime := time.Now().UTC()
	oldTime := freshTime.Add(-30 * time.Minute)
	reports := []db.MonitorReport{{ID: "monitor-report-all-up", MonitorID: "monitor-all-up", Payload: "{}", CollectedAt: freshTime.Format(time.RFC3339), Health: "up", CreatedAt: freshTime}, {ID: "monitor-report-all-down", MonitorID: "monitor-all-down", Payload: "{}", CollectedAt: freshTime.Format(time.RFC3339), Health: "down", CreatedAt: freshTime}, {ID: "monitor-report-all-stale", MonitorID: "monitor-all-stale", Payload: "{}", CollectedAt: oldTime.Format(time.RFC3339), Health: "up", CreatedAt: oldTime}}
	if err := server.db.Create(&reports).Error; err != nil {
		t.Fatalf("create reports: %v", err)
	}
	summaryResp := performJSONRequest(t, server, http.MethodGet, "/v1/monitors/summary", nil, "")
	if summaryResp.Code != http.StatusOK {
		t.Fatalf("monitor summary status = %d, body = %s", summaryResp.Code, summaryResp.Body.String())
	}
	var summary struct {
		Success bool `json:"success"`
		Data    struct {
			Total   int64 `json:"total"`
			Up      int64 `json:"up"`
			Down    int64 `json:"down"`
			Stale   int64 `json:"stale"`
			Unknown int64 `json:"unknown"`
		} `json:"data"`
	}
	decodeResponse(t, summaryResp, &summary)
	if !summary.Success || summary.Data.Total != 3 || summary.Data.Up != 1 || summary.Data.Down != 1 || summary.Data.Stale != 1 || summary.Data.Unknown != 0 {
		t.Fatalf("monitor summary = %+v, want total 3 up 1 down 1 stale 1 unknown 0", summary)
	}
	downResp := performJSONRequest(t, server, http.MethodGet, "/v1/monitors?health=down", nil, "")
	if downResp.Code != http.StatusOK {
		t.Fatalf("down monitor list status = %d, body = %s", downResp.Code, downResp.Body.String())
	}
	var listed struct {
		Success bool `json:"success"`
		Data    struct {
			Monitors []struct {
				ID        string `json:"id"`
				AgentName string `json:"agent_name"`
				Health    string `json:"health"`
			} `json:"monitors"`
			Count int64 `json:"count"`
		} `json:"data"`
	}
	decodeResponse(t, downResp, &listed)
	if !listed.Success || listed.Data.Count != 1 || listed.Data.Monitors[0].ID != "monitor-all-down" || listed.Data.Monitors[0].Health != "down" || listed.Data.Monitors[0].AgentName != agent.Name {
		t.Fatalf("down monitor list = %+v, want fresh down monitor with agent name", listed)
	}
	upResp := performJSONRequest(t, server, http.MethodGet, "/v1/monitors?health=up", nil, "")
	if upResp.Code != http.StatusOK {
		t.Fatalf("up monitor list status = %d, body = %s", upResp.Code, upResp.Body.String())
	}
	decodeResponse(t, upResp, &listed)
	if !listed.Success || listed.Data.Count != 1 || listed.Data.Monitors[0].ID != "monitor-all-up" {
		t.Fatalf("up monitor list = %+v, want only fresh up monitor", listed)
	}
	typeResp := performJSONRequest(t, server, http.MethodGet, "/v1/monitors?type=tcp", nil, "")
	if typeResp.Code != http.StatusOK {
		t.Fatalf("type monitor list status = %d, body = %s", typeResp.Code, typeResp.Body.String())
	}
	decodeResponse(t, typeResp, &listed)
	if !listed.Success || listed.Data.Count != 1 || listed.Data.Monitors[0].ID != "monitor-all-down" {
		t.Fatalf("type monitor list = %+v, want only tcp monitor", listed)
	}
	staleResp := performJSONRequest(t, server, http.MethodGet, "/v1/monitors?health=stale", nil, "")
	if staleResp.Code != http.StatusOK {
		t.Fatalf("stale monitor list status = %d, body = %s", staleResp.Code, staleResp.Body.String())
	}
	decodeResponse(t, staleResp, &listed)
	if !listed.Success || listed.Data.Count != 1 || listed.Data.Monitors[0].ID != "monitor-all-stale" || listed.Data.Monitors[0].Health != "stale" {
		t.Fatalf("stale monitor list = %+v, want stale monitor with stale health", listed)
	}
	detailResp := performJSONRequest(t, server, http.MethodGet, "/v1/monitors/monitor-all-stale", nil, "")
	if detailResp.Code != http.StatusOK {
		t.Fatalf("stale monitor detail status = %d, body = %s", detailResp.Code, detailResp.Body.String())
	}
	var detail struct {
		Success bool `json:"success"`
		Data    struct {
			Monitor struct {
				ID             string `json:"id"`
				Health         string `json:"health"`
				ComputedHealth string `json:"computed_health"`
			} `json:"monitor"`
			ComputedHealth string `json:"computed_health"`
		} `json:"data"`
	}
	decodeResponse(t, detailResp, &detail)
	if !detail.Success || detail.Data.Monitor.ID != "monitor-all-stale" || detail.Data.Monitor.Health != "stale" || detail.Data.Monitor.ComputedHealth != "stale" || detail.Data.ComputedHealth != "stale" {
		t.Fatalf("stale monitor detail = %+v, want stale health", detail)
	}
}
