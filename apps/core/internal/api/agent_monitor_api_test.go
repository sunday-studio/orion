package api

import (
	"net/http"
	"testing"
	"time"

	"orion/core/internal/db"
)

func TestRegisterMonitorReportHistoryFlow(t *testing.T) {
	server := setupTestServer(t)
	registered := registerTestAgent(t, server)
	registeredMonitor := registerTestMonitor(t, server, registered.Data.AgentID, registered.Data.Token)

	monitorReportBody := map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"health":    "up",
		"metrics": map[string]interface{}{
			"response_time_ms": 45,
			"status_code":      200,
		},
	}
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
	if listedMonitors.Data.Monitors[0].ID != registeredMonitor.Data.MonitorID ||
		listedMonitors.Data.Monitors[0].Name != "homepage" ||
		listedMonitors.Data.Monitors[0].Health != "up" {
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
	registerMonitorBody := map[string]interface{}{
		"name":                       "route-scoped-monitor",
		"description":                description,
		"type":                       "http-healthcheck",
		"last_checked":               time.Now().UTC().Format(time.RFC3339),
		"reporting_interval_seconds": 30,
	}
	registerMonitorResp := performJSONRequest(
		t,
		server,
		http.MethodPost,
		"/v1/agents/"+registered.Data.AgentID+"/register-monitor",
		registerMonitorBody,
		registered.Data.Token,
	)
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
	duplicateResp := performJSONRequest(
		t,
		server,
		http.MethodPost,
		"/v1/agents/"+registered.Data.AgentID+"/register-monitor",
		registerMonitorBody,
		registered.Data.Token,
	)
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

	unregisterResp := performJSONRequest(
		t,
		server,
		http.MethodPost,
		"/v1/agents/"+registered.Data.AgentID+"/unregister-monitor",
		map[string]interface{}{"monitor_id": registeredMonitor.Data.MonitorID},
		registered.Data.Token,
	)
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
	secondAgent := db.Agent{
		ID:        "agent-register-monitor-other",
		MachineId: "machine-register-monitor-other",
		Name:      "other register agent",
		OS:        "linux",
		Arch:      "arm64",
		Token:     "token-register-monitor-other",
		LastSeen:  time.Now(),
	}
	if err := server.db.Create(&secondAgent).Error; err != nil {
		t.Fatalf("create second agent: %v", err)
	}

	description := "wrong agent monitor"
	registerMonitorBody := map[string]interface{}{
		"agent_id":                   secondAgent.ID,
		"name":                       "wrong-agent-monitor",
		"description":                description,
		"type":                       "http-healthcheck",
		"last_checked":               time.Now().UTC().Format(time.RFC3339),
		"reporting_interval_seconds": 30,
	}

	registerMonitorResp := performJSONRequest(
		t,
		server,
		http.MethodPost,
		"/v1/agents/"+firstAgent.Data.AgentID+"/register-monitor",
		registerMonitorBody,
		firstAgent.Data.Token,
	)
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
	secondAgent := db.Agent{
		ID:        "agent-report-monitor-other",
		MachineId: "machine-report-monitor-other",
		Name:      "other report agent",
		OS:        "linux",
		Arch:      "arm64",
		Token:     "token-report-monitor-other",
		LastSeen:  time.Now(),
	}
	if err := server.db.Create(&secondAgent).Error; err != nil {
		t.Fatalf("create second agent: %v", err)
	}
	secondMonitor := db.Monitor{
		ID:                       "monitor-report-other-agent",
		AgentID:                  secondAgent.ID,
		Name:                     "other agent monitor",
		Type:                     "http",
		Lifecycle:                "active",
		Health:                   "up",
		ComputedHealth:           "up",
		ReportingIntervalSeconds: 60,
	}
	if err := server.db.Create(&secondMonitor).Error; err != nil {
		t.Fatalf("create second monitor: %v", err)
	}

	reportResp := performJSONRequest(
		t,
		server,
		http.MethodPost,
		"/v1/agents/"+firstAgent.Data.AgentID+"/"+secondMonitor.ID+"/report",
		map[string]interface{}{
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"health":    "down",
			"metrics":   map[string]interface{}{},
		},
		firstAgent.Data.Token,
	)
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
	if err := server.db.Model(&db.Monitor{}).
		Where("id = ?", registeredMonitor.Data.MonitorID).
		Updates(map[string]interface{}{
			"health":                  "down",
			"computed_health":         "up",
			"last_health_computation": staleComputation,
		}).Error; err != nil {
		t.Fatalf("update monitor health cache: %v", err)
	}

	report := db.MonitorReport{
		ID:          "monitor-report-computed-detail",
		MonitorID:   registeredMonitor.Data.MonitorID,
		Payload:     "{}",
		CollectedAt: time.Now().UTC().Format(time.RFC3339),
		Health:      "down",
		CreatedAt:   time.Now(),
	}
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

	if err := server.db.Model(&db.Monitor{}).
		Where("id = ?", registeredMonitor.Data.MonitorID).
		Updates(map[string]interface{}{
			"computed_health":         "up",
			"last_health_computation": time.Now(),
		}).Error; err != nil {
		t.Fatalf("prime monitor health cache: %v", err)
	}

	reportPath := "/v1/agents/" + registered.Data.AgentID + "/" + registeredMonitor.Data.MonitorID + "/report"
	reportResp := performJSONRequest(t, server, http.MethodPost, reportPath, map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"health":    "down",
		"metrics":   map[string]interface{}{"status_code": 500},
	}, registered.Data.Token)
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

	for _, path := range []string{
		"/v1/agents/agent-missing/health",
		"/v1/agents/agent-missing/reports",
		"/v1/agents/agent-missing/monitors",
	} {
		resp := performJSONRequest(t, server, http.MethodGet, path, nil, "")
		if resp.Code != http.StatusNotFound {
			t.Fatalf("%s status = %d, body = %s, want 404", path, resp.Code, resp.Body.String())
		}
	}
}

func TestAgentHealthReturnsStoredMonitorCountsForStaleAgent(t *testing.T) {
	server := setupTestServer(t)
	agent := db.Agent{
		ID:        "agent-stale-health-counts",
		MachineId: "machine-stale-health-counts",
		Name:      "stale health counts",
		OS:        "linux",
		Arch:      "arm64",
		Token:     "token-stale-health-counts",
		LastSeen:  time.Now().Add(-30 * time.Minute),
	}
	if err := server.db.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}

	monitors := []db.Monitor{
		{
			ID:        "monitor-stale-up",
			AgentID:   agent.ID,
			Name:      "stale up",
			Type:      "http",
			Lifecycle: "active",
			Health:    "up",
		},
		{
			ID:        "monitor-stale-down",
			AgentID:   agent.ID,
			Name:      "stale down",
			Type:      "http",
			Lifecycle: "active",
			Health:    "down",
		},
		{
			ID:        "monitor-stale-degraded",
			AgentID:   agent.ID,
			Name:      "stale degraded",
			Type:      "http",
			Lifecycle: "active",
			Health:    "degraded",
		},
	}
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

func TestAgentHealthSplitsAvailabilityFromMonitorRollup(t *testing.T) {
	server := setupTestServer(t)
	agent := db.Agent{
		ID:        "agent-split-health",
		MachineId: "machine-split-health",
		Name:      "split health",
		OS:        "linux",
		Arch:      "arm64",
		Token:     "token-split-health",
		LastSeen:  time.Now(),
	}
	if err := server.db.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}

	monitors := []db.Monitor{
		{ID: "monitor-split-up", AgentID: agent.ID, Name: "split up", Type: "http", Lifecycle: "active", Health: "up", ReportingIntervalSeconds: 60},
		{ID: "monitor-split-down", AgentID: agent.ID, Name: "split down", Type: "http", Lifecycle: "active", Health: "down", ReportingIntervalSeconds: 60},
	}
	if err := server.db.Create(&monitors).Error; err != nil {
		t.Fatalf("create monitors: %v", err)
	}

	now := time.Now().UTC()
	reports := []db.MonitorReport{
		{ID: "report-split-up", MonitorID: "monitor-split-up", Payload: "{}", CollectedAt: now.Format(time.RFC3339), Health: "up", CreatedAt: now},
		{ID: "report-split-down", MonitorID: "monitor-split-down", Payload: "{}", CollectedAt: now.Format(time.RFC3339), Health: "down", CreatedAt: now},
	}
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
	agent := db.Agent{
		ID:        "agent-system-health-stale",
		MachineId: "machine-system-health-stale",
		Name:      "system health stale",
		OS:        "linux",
		Arch:      "arm64",
		Token:     "token-system-health-stale",
		LastSeen:  time.Now(),
	}
	if err := server.db.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}

	monitors := []db.Monitor{
		{
			ID:        "monitor-system-health-up",
			AgentID:   agent.ID,
			Name:      "fresh monitor",
			Type:      "http",
			Lifecycle: "active",
			Health:    "up",
		},
		{
			ID:        "monitor-system-health-stale",
			AgentID:   agent.ID,
			Name:      "stale monitor",
			Type:      "http",
			Lifecycle: "active",
			Health:    "up",
		},
	}
	if err := server.db.Create(&monitors).Error; err != nil {
		t.Fatalf("create monitors: %v", err)
	}

	freshTime := time.Now().UTC()
	oldTime := freshTime.Add(-30 * time.Minute)
	reports := []db.MonitorReport{
		{
			ID:          "monitor-report-system-health-up",
			MonitorID:   "monitor-system-health-up",
			Payload:     "{}",
			CollectedAt: freshTime.Format(time.RFC3339),
			Health:      "up",
			CreatedAt:   freshTime,
		},
		{
			ID:          "monitor-report-system-health-stale",
			MonitorID:   "monitor-system-health-stale",
			Payload:     "{}",
			CollectedAt: oldTime.Format(time.RFC3339),
			Health:      "up",
			CreatedAt:   oldTime,
		},
	}
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
	agent := db.Agent{
		ID:        "agent-monitor-filter-counts",
		MachineId: "machine-monitor-filter-counts",
		Name:      "monitor filter counts",
		OS:        "linux",
		Arch:      "arm64",
		Token:     "token-monitor-filter-counts",
		LastSeen:  time.Now(),
	}
	if err := server.db.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}

	monitors := []db.Monitor{
		{
			ID:        "monitor-filter-up",
			AgentID:   agent.ID,
			Name:      "up",
			Type:      "http",
			Lifecycle: "active",
			Health:    "up",
		},
		{
			ID:        "monitor-filter-down",
			AgentID:   agent.ID,
			Name:      "down",
			Type:      "http",
			Lifecycle: "active",
			Health:    "down",
		},
		{
			ID:        "monitor-filter-disabled-down",
			AgentID:   agent.ID,
			Name:      "disabled down",
			Type:      "http",
			Lifecycle: "disabled",
			Health:    "down",
		},
	}
	if err := server.db.Create(&monitors).Error; err != nil {
		t.Fatalf("create monitors: %v", err)
	}
	reportTime := time.Now().UTC()
	reports := []db.MonitorReport{
		{
			ID:          "monitor-report-filter-up",
			MonitorID:   "monitor-filter-up",
			Payload:     "{}",
			CollectedAt: reportTime.Format(time.RFC3339),
			Health:      "up",
			CreatedAt:   reportTime,
		},
		{
			ID:          "monitor-report-filter-down",
			MonitorID:   "monitor-filter-down",
			Payload:     "{}",
			CollectedAt: reportTime.Format(time.RFC3339),
			Health:      "down",
			CreatedAt:   reportTime,
		},
		{
			ID:          "monitor-report-filter-disabled-down",
			MonitorID:   "monitor-filter-disabled-down",
			Payload:     "{}",
			CollectedAt: reportTime.Format(time.RFC3339),
			Health:      "down",
			CreatedAt:   reportTime,
		},
	}
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
	agent := db.Agent{
		ID:        "agent-all-monitor-summary",
		MachineId: "machine-all-monitor-summary",
		Name:      "all monitor summary",
		OS:        "linux",
		Arch:      "arm64",
		Token:     "token-all-monitor-summary",
		LastSeen:  time.Now(),
	}
	if err := server.db.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}

	monitors := []db.Monitor{
		{
			ID:        "monitor-all-up",
			AgentID:   agent.ID,
			Name:      "all up",
			Type:      "http",
			Lifecycle: "active",
			Health:    "up",
		},
		{
			ID:        "monitor-all-down",
			AgentID:   agent.ID,
			Name:      "all down",
			Type:      "tcp",
			Lifecycle: "active",
			Health:    "down",
		},
		{
			ID:        "monitor-all-stale",
			AgentID:   agent.ID,
			Name:      "all stale",
			Type:      "http",
			Lifecycle: "active",
			Health:    "up",
		},
	}
	if err := server.db.Create(&monitors).Error; err != nil {
		t.Fatalf("create monitors: %v", err)
	}

	freshTime := time.Now().UTC()
	oldTime := freshTime.Add(-30 * time.Minute)
	reports := []db.MonitorReport{
		{
			ID:          "monitor-report-all-up",
			MonitorID:   "monitor-all-up",
			Payload:     "{}",
			CollectedAt: freshTime.Format(time.RFC3339),
			Health:      "up",
			CreatedAt:   freshTime,
		},
		{
			ID:          "monitor-report-all-down",
			MonitorID:   "monitor-all-down",
			Payload:     "{}",
			CollectedAt: freshTime.Format(time.RFC3339),
			Health:      "down",
			CreatedAt:   freshTime,
		},
		{
			ID:          "monitor-report-all-stale",
			MonitorID:   "monitor-all-stale",
			Payload:     "{}",
			CollectedAt: oldTime.Format(time.RFC3339),
			Health:      "up",
			CreatedAt:   oldTime,
		},
	}
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

func TestListAllMonitorsFiltersCanonicalMonitorTypeAliases(t *testing.T) {
	server := setupTestServer(t)
	agent := db.Agent{
		ID:        "agent-monitor-type-aliases",
		MachineId: "machine-monitor-type-aliases",
		Name:      "monitor type aliases",
		OS:        "linux",
		Arch:      "arm64",
		Token:     "token-monitor-type-aliases",
		LastSeen:  time.Now(),
	}
	if err := server.db.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}

	monitors := []db.Monitor{
		{
			ID:        "monitor-docker-container-type",
			AgentID:   agent.ID,
			Name:      "docker container type",
			Type:      "docker-container",
			Lifecycle: "active",
			Health:    "up",
		},
		{
			ID:        "monitor-systemd-service-type",
			AgentID:   agent.ID,
			Name:      "systemd service type",
			Type:      "systemd-service",
			Lifecycle: "active",
			Health:    "up",
		},
	}
	if err := server.db.Create(&monitors).Error; err != nil {
		t.Fatalf("create monitors: %v", err)
	}

	for _, tc := range []struct {
		query string
		want  string
	}{
		{query: "docker-container", want: "monitor-docker-container-type"},
		{query: "docker", want: "monitor-docker-container-type"},
		{query: "systemd-service", want: "monitor-systemd-service-type"},
		{query: "systemd", want: "monitor-systemd-service-type"},
	} {
		resp := performJSONRequest(t, server, http.MethodGet, "/v1/monitors?type="+tc.query, nil, "")
		if resp.Code != http.StatusOK {
			t.Fatalf("type %q monitor list status = %d, body = %s", tc.query, resp.Code, resp.Body.String())
		}
		var listed struct {
			Success bool `json:"success"`
			Data    struct {
				Monitors []struct {
					ID   string `json:"id"`
					Type string `json:"type"`
				} `json:"monitors"`
				Count int64 `json:"count"`
			} `json:"data"`
		}
		decodeResponse(t, resp, &listed)
		if !listed.Success || listed.Data.Count != 1 || listed.Data.Monitors[0].ID != tc.want {
			t.Fatalf("type %q monitor list = %+v, want %s", tc.query, listed, tc.want)
		}
	}
}

func TestListAllMonitorsFiltersByOwnerAndSource(t *testing.T) {
	server := setupTestServer(t)
	agent := db.Agent{
		ID:        "agent-monitor-owner-filter",
		MachineId: "machine-monitor-owner-filter",
		Name:      "Customer App Agent",
		OS:        "linux",
		Arch:      "arm64",
		Token:     "token-monitor-owner-filter",
		LastSeen:  time.Now(),
	}
	coreOwner := db.Agent{
		ID:        "agent-core-monitor-owner-filter",
		MachineId: "core",
		Name:      "Orion Core",
		OS:        "linux",
		Arch:      "arm64",
		Token:     "token-core-monitor-owner-filter",
		LastSeen:  time.Now(),
	}
	if err := server.db.Create(&[]db.Agent{agent, coreOwner}).Error; err != nil {
		t.Fatalf("create owner filter agents: %v", err)
	}

	monitors := []db.Monitor{
		{
			ID:        "monitor-agent-owner-filter",
			AgentID:   agent.ID,
			Name:      "Agent API check",
			Type:      "http-healthcheck",
			Lifecycle: "active",
			Health:    "up",
		},
		{
			ID:        "monitor-core-owner-filter",
			AgentID:   coreOwner.ID,
			Name:      "Core API check",
			Type:      "http",
			Lifecycle: "active",
			Health:    "up",
		},
	}
	if err := server.db.Create(&monitors).Error; err != nil {
		t.Fatalf("create owner filter monitors: %v", err)
	}
	if err := server.db.Create(&db.CoreMonitorConfig{
		MonitorID:       "monitor-core-owner-filter",
		Kind:            "http",
		ConfigJSON:      "{}",
		SecretRefJSON:   "{}",
		IntervalSeconds: 60,
		TimeoutSeconds:  10,
		NextRunAt:       time.Now().Add(time.Minute),
	}).Error; err != nil {
		t.Fatalf("create core monitor config: %v", err)
	}

	for _, tc := range []struct {
		query     string
		wantID    string
		wantOwner string
		wantName  string
	}{
		{query: "owner_kind=agent", wantID: "monitor-agent-owner-filter", wantOwner: "agent", wantName: "Customer App Agent"},
		{query: "owner_kind=core", wantID: "monitor-core-owner-filter", wantOwner: "core", wantName: "Orion Core"},
		{query: "source=agent", wantID: "monitor-agent-owner-filter", wantOwner: "agent", wantName: "Customer App Agent"},
		{query: "source=core", wantID: "monitor-core-owner-filter", wantOwner: "core", wantName: "Orion Core"},
		{query: "owner_name=Customer", wantID: "monitor-agent-owner-filter", wantOwner: "agent", wantName: "Customer App Agent"},
		{query: "owner_name=Orion", wantID: "monitor-core-owner-filter", wantOwner: "core", wantName: "Orion Core"},
	} {
		resp := performJSONRequest(t, server, http.MethodGet, "/v1/monitors?"+tc.query, nil, "")
		if resp.Code != http.StatusOK {
			t.Fatalf("%s monitor list status = %d, body = %s", tc.query, resp.Code, resp.Body.String())
		}
		var listed struct {
			Success bool `json:"success"`
			Data    struct {
				Monitors []struct {
					ID        string `json:"id"`
					OwnerKind string `json:"owner_kind"`
					OwnerName string `json:"owner_name"`
					Source    string `json:"source"`
				} `json:"monitors"`
				Count int64 `json:"count"`
			} `json:"data"`
		}
		decodeResponse(t, resp, &listed)
		if !listed.Success || listed.Data.Count != 1 || len(listed.Data.Monitors) != 1 {
			t.Fatalf("%s listed monitors = %+v, want exactly one monitor", tc.query, listed)
		}
		got := listed.Data.Monitors[0]
		if got.ID != tc.wantID || got.OwnerKind != tc.wantOwner || got.OwnerName != tc.wantName || got.Source != tc.wantOwner {
			t.Fatalf("%s monitor = %+v, want id %s owner/source %s name %s", tc.query, got, tc.wantID, tc.wantOwner, tc.wantName)
		}
	}
}

func TestListAllMonitorsFiltersByComputedHealth(t *testing.T) {
	server := setupTestServer(t)
	agent := db.Agent{
		ID:        "agent-computed-monitor-filter",
		MachineId: "machine-computed-monitor-filter",
		Name:      "computed monitor filter",
		OS:        "linux",
		Arch:      "arm64",
		Token:     "token-computed-monitor-filter",
		LastSeen:  time.Now(),
	}
	if err := server.db.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}

	monitor := db.Monitor{
		ID:                       "monitor-computed-degraded-filter",
		AgentID:                  agent.ID,
		Name:                     "computed degraded filter",
		Type:                     "http",
		Lifecycle:                "active",
		Health:                   "up",
		ReportingIntervalSeconds: 60,
		CreatedAt:                time.Now(),
	}
	if err := server.db.Create(&monitor).Error; err != nil {
		t.Fatalf("create monitor: %v", err)
	}

	now := time.Now().UTC()
	reports := []db.MonitorReport{
		{ID: "report-computed-filter-1", MonitorID: monitor.ID, Payload: "{}", CollectedAt: now.Format(time.RFC3339), Health: "up", CreatedAt: now},
		{ID: "report-computed-filter-2", MonitorID: monitor.ID, Payload: "{}", CollectedAt: now.Add(-1 * time.Minute).Format(time.RFC3339), Health: "up", CreatedAt: now.Add(-1 * time.Minute)},
		{ID: "report-computed-filter-3", MonitorID: monitor.ID, Payload: "{}", CollectedAt: now.Add(-2 * time.Minute).Format(time.RFC3339), Health: "down", CreatedAt: now.Add(-2 * time.Minute)},
		{ID: "report-computed-filter-4", MonitorID: monitor.ID, Payload: "{}", CollectedAt: now.Add(-3 * time.Minute).Format(time.RFC3339), Health: "down", CreatedAt: now.Add(-3 * time.Minute)},
		{ID: "report-computed-filter-5", MonitorID: monitor.ID, Payload: "{}", CollectedAt: now.Add(-4 * time.Minute).Format(time.RFC3339), Health: "up", CreatedAt: now.Add(-4 * time.Minute)},
	}
	if err := server.db.Create(&reports).Error; err != nil {
		t.Fatalf("create reports: %v", err)
	}

	resp := performJSONRequest(t, server, http.MethodGet, "/v1/monitors?health=degraded", nil, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("computed degraded monitor list status = %d, body = %s", resp.Code, resp.Body.String())
	}
	var listed struct {
		Success bool `json:"success"`
		Data    struct {
			Monitors []struct {
				ID             string `json:"id"`
				Health         string `json:"health"`
				ComputedHealth string `json:"computed_health"`
			} `json:"monitors"`
			Count int64 `json:"count"`
		} `json:"data"`
	}
	decodeResponse(t, resp, &listed)
	if !listed.Success || listed.Data.Count != 1 || listed.Data.Monitors[0].ID != monitor.ID || listed.Data.Monitors[0].Health != "degraded" || listed.Data.Monitors[0].ComputedHealth != "degraded" {
		t.Fatalf("computed degraded monitor list = %+v, want computed degraded monitor", listed)
	}
}
