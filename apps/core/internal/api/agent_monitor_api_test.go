package api

import (
	"net/http"
	"orion/core/internal/db"
	"testing"
	"time"
)

func TestRegisterMonitorReportHistoryFlow(t *testing.T) {
	server := setupTestServer(t)
	registered := registerTestAgent(t, server)
	registeredMonitor := registerTestMonitor(t, server, registered.Data.AgentID, registered.Data.Token)
	monitorReportBody := map[string]interface{}{"timestamp": time.Now().UTC().Format(time.RFC3339), "health": "up", "metrics": map[string]interface{}{"response_time_ms": 45, "status_code": 200}}
	monitorReportPath := "/v1/agents/" + registered.Data.AgentID + "/" + registeredMonitor.Data.MonitorID + "/report"
	monitorReportResp := performJSONRequest(t, server, http.MethodPost, monitorReportPath, monitorReportBody, registered.Data.Token)
	if monitorReportResp.Code != http.StatusOK {
		t.Fatalf("monitor report status = %d, body = %s", monitorReportResp.Code, monitorReportResp.Body.String())
	}
	listMonitorsResp := performJSONRequest(t, server, http.MethodGet, "/v1/agents/"+registered.Data.AgentID+"/monitors", nil, "")
	if listMonitorsResp.Code != http.StatusOK {
		t.Fatalf("list monitors status = %d, body = %s", listMonitorsResp.Code, listMonitorsResp.Body.String())
	}
	var listedMonitors struct {
		Success bool `json:"success"`
		Data    struct {
			Monitors []struct {
				ID     string `json:"id"`
				Name   string `json:"name"`
				Health string `json:"health"`
			} `json:"monitors"`
			Count int64 `json:"count"`
		} `json:"data"`
	}
	decodeResponse(t, listMonitorsResp, &listedMonitors)
	if !listedMonitors.Success || listedMonitors.Data.Count != 1 || len(listedMonitors.Data.Monitors) != 1 {
		t.Fatalf("list monitors returned unexpected payload: %+v", listedMonitors)
	}
	if listedMonitors.Data.Monitors[0].ID != registeredMonitor.Data.MonitorID || listedMonitors.Data.Monitors[0].Name != "homepage" || listedMonitors.Data.Monitors[0].Health != "up" {
		t.Fatalf("list monitors returned wrong monitor: %+v", listedMonitors.Data.Monitors[0])
	}
	historyResp := performJSONRequest(t, server, http.MethodGet, "/v1/monitors/"+registeredMonitor.Data.MonitorID+"/history", nil, "")
	if historyResp.Code != http.StatusOK {
		t.Fatalf("monitor history status = %d, body = %s", historyResp.Code, historyResp.Body.String())
	}
	var history struct {
		Success bool `json:"success"`
		Data    struct {
			Reports []struct {
				MonitorID string `json:"monitor_id"`
				Health    string `json:"health"`
				Payload   string `json:"payload"`
			} `json:"reports"`
			Count int64 `json:"count"`
		} `json:"data"`
	}
	decodeResponse(t, historyResp, &history)
	if !history.Success || history.Data.Count != 1 || len(history.Data.Reports) != 1 {
		t.Fatalf("history returned unexpected payload: %+v", history)
	}
	if history.Data.Reports[0].MonitorID != registeredMonitor.Data.MonitorID || history.Data.Reports[0].Health != "up" {
		t.Fatalf("history returned wrong report: %+v", history.Data.Reports[0])
	}
}
func TestRegisterAndUnregisterMonitorUseRouteAgentID(t *testing.T) {
	server := setupTestServer(t)
	registered := registerTestAgent(t, server)
	description := "route scoped monitor"
	registerMonitorBody := map[string]interface{}{"name": "route-scoped-monitor", "description": description, "type": "http-healthcheck", "last_checked": time.Now().UTC().Format(time.RFC3339), "reporting_interval_seconds": 30}
	registerMonitorResp := performJSONRequest(t, server, http.MethodPost, "/v1/agents/"+registered.Data.AgentID+"/register-monitor", registerMonitorBody, registered.Data.Token)
	if registerMonitorResp.Code != http.StatusOK {
		t.Fatalf("register monitor status = %d, body = %s", registerMonitorResp.Code, registerMonitorResp.Body.String())
	}
	var registeredMonitor struct {
		Success bool `json:"success"`
		Data    struct {
			MonitorID string `json:"monitor_id"`
		} `json:"data"`
	}
	decodeResponse(t, registerMonitorResp, &registeredMonitor)
	if !registeredMonitor.Success || registeredMonitor.Data.MonitorID == "" {
		t.Fatalf("register monitor response = %+v, want monitor id", registeredMonitor)
	}
	var monitor db.Monitor
	if err := server.db.Where("id = ?", registeredMonitor.Data.MonitorID).First(&monitor).Error; err != nil {
		t.Fatalf("find monitor: %v", err)
	}
	if monitor.AgentID != registered.Data.AgentID {
		t.Fatalf("monitor agent id = %q, want route agent id %q", monitor.AgentID, registered.Data.AgentID)
	}
	registerMonitorBody["reporting_interval_seconds"] = 45
	updatedDescription := "updated route scoped monitor"
	registerMonitorBody["description"] = updatedDescription
	registerMonitorBody["type"] = "tcp"
	duplicateResp := performJSONRequest(t, server, http.MethodPost, "/v1/agents/"+registered.Data.AgentID+"/register-monitor", registerMonitorBody, registered.Data.Token)
	if duplicateResp.Code != http.StatusOK {
		t.Fatalf("duplicate register monitor status = %d, body = %s", duplicateResp.Code, duplicateResp.Body.String())
	}
	var duplicateMonitor struct {
		Success bool `json:"success"`
		Data    struct {
			MonitorID string `json:"monitor_id"`
		} `json:"data"`
	}
	decodeResponse(t, duplicateResp, &duplicateMonitor)
	if duplicateMonitor.Data.MonitorID != registeredMonitor.Data.MonitorID {
		t.Fatalf("duplicate monitor id = %q, want existing %q", duplicateMonitor.Data.MonitorID, registeredMonitor.Data.MonitorID)
	}
	if err := server.db.Where("id = ?", registeredMonitor.Data.MonitorID).First(&monitor).Error; err != nil {
		t.Fatalf("reload monitor after duplicate register: %v", err)
	}
	if monitor.ReportingIntervalSeconds != 45 {
		t.Fatalf("monitor interval = %d, want updated 45", monitor.ReportingIntervalSeconds)
	}
	if monitor.Description == nil || *monitor.Description != updatedDescription {
		t.Fatalf("monitor description = %v, want updated description", monitor.Description)
	}
	if monitor.Type != "tcp" {
		t.Fatalf("monitor type = %q, want tcp", monitor.Type)
	}
	unregisterResp := performJSONRequest(t, server, http.MethodPost, "/v1/agents/"+registered.Data.AgentID+"/unregister-monitor", map[string]interface{}{"monitor_id": registeredMonitor.Data.MonitorID}, registered.Data.Token)
	if unregisterResp.Code != http.StatusOK {
		t.Fatalf("unregister monitor status = %d, body = %s", unregisterResp.Code, unregisterResp.Body.String())
	}
	if err := server.db.Where("id = ?", registeredMonitor.Data.MonitorID).First(&monitor).Error; err != nil {
		t.Fatalf("reload monitor: %v", err)
	}
	if monitor.Lifecycle != "deleted" {
		t.Fatalf("monitor lifecycle = %q, want deleted", monitor.Lifecycle)
	}
}
func TestAgentCannotRegisterMonitorForDifferentAgent(t *testing.T) {
	server := setupTestServer(t)
	firstAgent := registerTestAgent(t, server)
	secondAgent := db.Agent{ID: "agent-register-monitor-other", MachineId: "machine-register-monitor-other", Name: "other register agent", OS: "linux", Arch: "arm64", Token: "token-register-monitor-other", LastSeen: time.Now()}
	if err := server.db.Create(&secondAgent).Error; err != nil {
		t.Fatalf("create second agent: %v", err)
	}
	description := "wrong agent monitor"
	registerMonitorBody := map[string]interface{}{"agent_id": secondAgent.ID, "name": "wrong-agent-monitor", "description": description, "type": "http-healthcheck", "last_checked": time.Now().UTC().Format(time.RFC3339), "reporting_interval_seconds": 30}
	registerMonitorResp := performJSONRequest(t, server, http.MethodPost, "/v1/agents/"+firstAgent.Data.AgentID+"/register-monitor", registerMonitorBody, firstAgent.Data.Token)
	if registerMonitorResp.Code != http.StatusBadRequest {
		t.Fatalf("register monitor status = %d, body = %s, want 400", registerMonitorResp.Code, registerMonitorResp.Body.String())
	}
	var count int64
	if err := server.db.Model(&db.Monitor{}).Where("agent_id = ? AND name = ?", secondAgent.ID, "wrong-agent-monitor").Count(&count).Error; err != nil {
		t.Fatalf("count monitors: %v", err)
	}
	if count != 0 {
		t.Fatalf("monitor count = %d, want no cross-agent monitor", count)
	}
}
func TestAgentCannotReportForAnotherAgentsMonitor(t *testing.T) {
	server := setupTestServer(t)
	firstAgent := registerTestAgent(t, server)
	secondAgent := db.Agent{ID: "agent-report-monitor-other", MachineId: "machine-report-monitor-other", Name: "other report agent", OS: "linux", Arch: "arm64", Token: "token-report-monitor-other", LastSeen: time.Now()}
	if err := server.db.Create(&secondAgent).Error; err != nil {
		t.Fatalf("create second agent: %v", err)
	}
	secondMonitor := db.Monitor{ID: "monitor-report-other-agent", AgentID: secondAgent.ID, Name: "other agent monitor", Type: "http", Lifecycle: "active", Health: "up", ComputedHealth: "up", ReportingIntervalSeconds: 60}
	if err := server.db.Create(&secondMonitor).Error; err != nil {
		t.Fatalf("create second monitor: %v", err)
	}
	reportResp := performJSONRequest(t, server, http.MethodPost, "/v1/agents/"+firstAgent.Data.AgentID+"/"+secondMonitor.ID+"/report", map[string]interface{}{"timestamp": time.Now().UTC().Format(time.RFC3339), "health": "down", "metrics": map[string]interface{}{}}, firstAgent.Data.Token)
	if reportResp.Code != http.StatusUnauthorized {
		t.Fatalf("cross-agent monitor report status = %d, body = %s, want 401", reportResp.Code, reportResp.Body.String())
	}
	var count int64
	if err := server.db.Model(&db.MonitorReport{}).Where("monitor_id = ?", secondMonitor.ID).Count(&count).Error; err != nil {
		t.Fatalf("count monitor reports: %v", err)
	}
	if count != 0 {
		t.Fatalf("monitor report count = %d, want no cross-agent report", count)
	}
}
func TestMonitorDetailReturnsConsistentComputedHealth(t *testing.T) {
	server := setupTestServer(t)
	registered := registerTestAgent(t, server)
	registeredMonitor := registerTestMonitor(t, server, registered.Data.AgentID, registered.Data.Token)
	staleComputation := time.Now().Add(-10 * time.Minute)
	if err := server.db.Model(&db.Monitor{}).Where("id = ?", registeredMonitor.Data.MonitorID).Updates(map[string]interface{}{"health": "down", "computed_health": "up", "last_health_computation": staleComputation}).Error; err != nil {
		t.Fatalf("update monitor health cache: %v", err)
	}
	report := db.MonitorReport{ID: "monitor-report-computed-detail", MonitorID: registeredMonitor.Data.MonitorID, Payload: "{}", CollectedAt: time.Now().UTC().Format(time.RFC3339), Health: "down", CreatedAt: time.Now()}
	if err := server.db.Create(&report).Error; err != nil {
		t.Fatalf("create monitor report: %v", err)
	}
	detailResp := performJSONRequest(t, server, http.MethodGet, "/v1/monitors/"+registeredMonitor.Data.MonitorID, nil, "")
	if detailResp.Code != http.StatusOK {
		t.Fatalf("monitor detail status = %d, body = %s", detailResp.Code, detailResp.Body.String())
	}
	var detail struct {
		Success bool `json:"success"`
		Data    struct {
			ComputedHealth string `json:"computed_health"`
			Monitor        struct {
				ComputedHealth string `json:"computed_health"`
				AgentName      string `json:"agent_name"`
			} `json:"monitor"`
		} `json:"data"`
	}
	decodeResponse(t, detailResp, &detail)
	if !detail.Success || detail.Data.ComputedHealth != "down" || detail.Data.Monitor.ComputedHealth != "down" {
		t.Fatalf("monitor detail health = %+v, want both computed health fields down", detail)
	}
	if detail.Data.Monitor.AgentName != "test-server" {
		t.Fatalf("monitor detail agent_name = %q, want test-server", detail.Data.Monitor.AgentName)
	}
}
func TestMonitorReportInvalidatesComputedHealthCache(t *testing.T) {
	server := setupTestServer(t)
	registered := registerTestAgent(t, server)
	registeredMonitor := registerTestMonitor(t, server, registered.Data.AgentID, registered.Data.Token)
	if err := server.db.Model(&db.Monitor{}).Where("id = ?", registeredMonitor.Data.MonitorID).Updates(map[string]interface{}{"computed_health": "up", "last_health_computation": time.Now()}).Error; err != nil {
		t.Fatalf("prime monitor health cache: %v", err)
	}
	reportPath := "/v1/agents/" + registered.Data.AgentID + "/" + registeredMonitor.Data.MonitorID + "/report"
	reportResp := performJSONRequest(t, server, http.MethodPost, reportPath, map[string]interface{}{"timestamp": time.Now().UTC().Format(time.RFC3339), "health": "down", "metrics": map[string]interface{}{"status_code": 500}}, registered.Data.Token)
	if reportResp.Code != http.StatusOK {
		t.Fatalf("monitor report status = %d, body = %s", reportResp.Code, reportResp.Body.String())
	}
	var incident db.Incident
	if err := server.db.Where("monitor_id = ?", registeredMonitor.Data.MonitorID).First(&incident).Error; err != nil {
		t.Fatalf("find list incident: %v", err)
	}
	var monitor db.Monitor
	if err := server.db.Where("id = ?", registeredMonitor.Data.MonitorID).First(&monitor).Error; err != nil {
		t.Fatalf("reload monitor: %v", err)
	}
	if monitor.Health != "down" || monitor.ComputedHealth != "down" {
		t.Fatalf("monitor health = %q computed = %q, want down/down", monitor.Health, monitor.ComputedHealth)
	}
}
func TestMonitorHistoryReturnsNotFoundForUnknownMonitor(t *testing.T) {
	server := setupTestServer(t)
	historyResp := performJSONRequest(t, server, http.MethodGet, "/v1/monitors/monitor-missing/history", nil, "")
	if historyResp.Code != http.StatusNotFound {
		t.Fatalf("monitor history status = %d, body = %s, want 404", historyResp.Code, historyResp.Body.String())
	}
}
func TestAgentScopedReadEndpointsReturnNotFoundForMissingAgent(t *testing.T) {
	server := setupTestServer(t)
	for _, path := range []string{"/v1/agents/agent-missing/health", "/v1/agents/agent-missing/reports", "/v1/agents/agent-missing/monitors"} {
		resp := performJSONRequest(t, server, http.MethodGet, path, nil, "")
		if resp.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d, body = %s, want 404", path, resp.Code, resp.Body.String())
		}
	}
}
func TestAgentHealthReturnsStoredMonitorCountsForStaleAgent(t *testing.T) {
	server := setupTestServer(t)
	agent := db.Agent{ID: "agent-stale-health-counts", MachineId: "machine-stale-health-counts", Name: "stale health counts", OS: "linux", Arch: "arm64", Token: "token-stale-health-counts", LastSeen: time.Now().Add(-30 * time.Minute)}
	if err := server.db.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	monitors := []db.Monitor{{ID: "monitor-stale-up", AgentID: agent.ID, Name: "stale up", Type: "http", Lifecycle: "active", Health: "up"}, {ID: "monitor-stale-down", AgentID: agent.ID, Name: "stale down", Type: "http", Lifecycle: "active", Health: "down"}, {ID: "monitor-stale-degraded", AgentID: agent.ID, Name: "stale degraded", Type: "http", Lifecycle: "active", Health: "degraded"}}
	if err := server.db.Create(&monitors).Error; err != nil {
		t.Fatalf("create monitors: %v", err)
	}
	healthResp := performJSONRequest(t, server, http.MethodGet, "/v1/agents/"+agent.ID+"/health", nil, "")
	if healthResp.Code != http.StatusOK {
		t.Fatalf("health status = %d, body = %s", healthResp.Code, healthResp.Body.String())
	}
	var health struct {
		Success bool `json:"success"`
		Data    struct {
			OverallHealth string `json:"overall_health"`
			UpCount       int    `json:"up_count"`
			DownCount     int    `json:"down_count"`
			DegradedCount int    `json:"degraded_count"`
		} `json:"data"`
	}
	decodeResponse(t, healthResp, &health)
	if !health.Success || health.Data.OverallHealth != "stale" {
		t.Fatalf("health response = %+v, want stale", health)
	}
	if health.Data.UpCount != 1 || health.Data.DownCount != 1 || health.Data.DegradedCount != 1 {
		t.Fatalf("health counts = %+v, want 1/1/1", health.Data)
	}
}
