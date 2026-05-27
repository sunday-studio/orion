package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/logging"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRegisterReportListFlow(t *testing.T) {
	server := setupTestServer(t)
	registered := registerTestAgent(t, server)

	reportBody := map[string]interface{}{
		"uptime_seconds": 120,
		"timestamp":      time.Now().UTC().Format(time.RFC3339),
		"agent_version":  "dev-test",
		"config_summary": map[string]interface{}{
			"monitor_count":      1,
			"reporting_interval": "60s",
			"secret":             "do-not-return",
		},
		"cpu": map[string]interface{}{
			"cores":         4,
			"usage_percent": 12.5,
			"load_1":        0.1,
			"load_5":        0.2,
			"load_15":       0.3,
		},
		"memory": map[string]interface{}{
			"total_bytes":     1024,
			"used_bytes":      512,
			"free_bytes":      512,
			"available_bytes": 512,
			"used_percent":    50,
		},
		"disk": map[string]interface{}{
			"total_bytes":  2048,
			"used_bytes":   1024,
			"free_bytes":   1024,
			"used_percent": 50,
		},
	}
	reportPath := "/v1/agents/" + registered.Data.AgentID + "/report"
	reportResp := performJSONRequest(t, server, http.MethodPost, reportPath, reportBody, registered.Data.Token)
	if reportResp.Code != http.StatusOK {
		t.Fatalf("report status = %d, body = %s", reportResp.Code, reportResp.Body.String())
	}

	listResp := performJSONRequest(t, server, http.MethodGet, "/v1/agents", nil, "")
	if listResp.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listResp.Code, listResp.Body.String())
	}
	assertFrontendResponseDoesNotExposeAgentSecrets(t, listResp.Body.String(), registered.Data.Token)

	var listed struct {
		Success bool `json:"success"`
		Data    struct {
			Agents []struct {
				ID            string  `json:"id"`
				Name          string  `json:"name"`
				UptimeSeconds *uint64 `json:"uptime_seconds"`
			} `json:"agents"`
			Count int64 `json:"count"`
		} `json:"data"`
	}
	decodeResponse(t, listResp, &listed)
	if !listed.Success {
		t.Fatalf("list response was not successful: %+v", listed)
	}
	if listed.Data.Count != 1 || len(listed.Data.Agents) != 1 {
		t.Fatalf("list returned count=%d len=%d, want 1 agent", listed.Data.Count, len(listed.Data.Agents))
	}
	if listed.Data.Agents[0].ID != registered.Data.AgentID || listed.Data.Agents[0].Name != "test-server" {
		t.Fatalf("list returned wrong agent: %+v", listed.Data.Agents[0])
	}
	if listed.Data.Agents[0].UptimeSeconds == nil || *listed.Data.Agents[0].UptimeSeconds != 120 {
		t.Fatalf("list did not include latest uptime: %+v", listed.Data.Agents[0].UptimeSeconds)
	}

	detailResp := performJSONRequest(t, server, http.MethodGet, "/v1/agents/"+registered.Data.AgentID, nil, "")
	if detailResp.Code != http.StatusOK {
		t.Fatalf("detail status = %d, body = %s", detailResp.Code, detailResp.Body.String())
	}
	assertFrontendResponseDoesNotExposeAgentSecrets(t, detailResp.Body.String(), registered.Data.Token)
	assertNotContains(t, detailResp.Body.String(), "do-not-return")

	reportsResp := performJSONRequest(t, server, http.MethodGet, "/v1/agents/"+registered.Data.AgentID+"/reports", nil, "")
	if reportsResp.Code != http.StatusOK {
		t.Fatalf("reports status = %d, body = %s", reportsResp.Code, reportsResp.Body.String())
	}
	assertFrontendResponseDoesNotExposeAgentSecrets(t, reportsResp.Body.String(), registered.Data.Token)
	assertNotContains(t, reportsResp.Body.String(), "do-not-return")

	var reports struct {
		Success bool `json:"success"`
		Data    struct {
			Reports []struct {
				ConfigSummary struct {
					MonitorCount      int    `json:"monitor_count"`
					ReportingInterval string `json:"reporting_interval"`
				} `json:"config_summary"`
			} `json:"reports"`
		} `json:"data"`
	}
	decodeResponse(t, reportsResp, &reports)
	if !reports.Success || len(reports.Data.Reports) != 1 {
		t.Fatalf("reports response = %+v, want one report", reports)
	}
	if reports.Data.Reports[0].ConfigSummary.MonitorCount != 1 || reports.Data.Reports[0].ConfigSummary.ReportingInterval != "60s" {
		t.Fatalf("config summary response = %+v, want whitelisted summary", reports.Data.Reports[0].ConfigSummary)
	}

	var storedReport db.AgentReport
	if err := server.db.Where("agent_id = ?", registered.Data.AgentID).First(&storedReport).Error; err != nil {
		t.Fatalf("find stored agent report: %v", err)
	}
	if storedReport.AgentVersion != "dev-test" {
		t.Fatalf("agent_version = %q, want dev-test", storedReport.AgentVersion)
	}
	if !strings.Contains(storedReport.ConfigSummary, `"monitor_count":1`) {
		t.Fatalf("config_summary = %q, want monitor_count", storedReport.ConfigSummary)
	}
	var storedAgent db.Agent
	if err := server.db.Where("id = ?", registered.Data.AgentID).First(&storedAgent).Error; err != nil {
		t.Fatalf("find stored agent: %v", err)
	}
	if storedAgent.ReportingIntervalSeconds != 60 {
		t.Fatalf("agent reporting interval = %d, want 60", storedAgent.ReportingIntervalSeconds)
	}
}

func TestHealthCheckResponse(t *testing.T) {
	server := setupTestServer(t)

	healthResp := performJSONRequest(t, server, http.MethodGet, "/health", nil, "")
	if healthResp.Code != http.StatusOK {
		t.Fatalf("health status = %d, body = %s", healthResp.Code, healthResp.Body.String())
	}

	var health struct {
		Status   string `json:"status"`
		Service  string `json:"service"`
		Database string `json:"database"`
	}
	decodeResponse(t, healthResp, &health)
	if health.Status != "healthy" || health.Service != "orion-core" || health.Database != "ok" {
		t.Fatalf("health response = %+v, want healthy orion-core ok", health)
	}
	if healthResp.Header().Get("X-Request-ID") == "" {
		t.Fatalf("health response missing X-Request-ID header")
	}
}

func TestLoginRequiresConfiguredFrontendAuth(t *testing.T) {
	server := setupTestServer(t)

	loginResp := performJSONRequest(t, server, http.MethodPost, "/v1/auth/login", map[string]string{
		"username": "admin",
		"password": "password",
	}, "")
	if loginResp.Code != http.StatusUnauthorized {
		t.Fatalf("login status = %d, body = %s, want 401 when auth is not configured", loginResp.Code, loginResp.Body.String())
	}
}

func TestLoginReturnsTokenForValidConfiguredCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	database, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.Migrate(database); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	server := NewServer(database, logging.NewLogger(), &config.Config{
		FrontendAuthOn: true,
		AdminUsername:  "admin",
		AdminPassword:  "correct-password",
		JWTSecret:      "test-secret",
	})

	badResp := performJSONRequest(t, server, http.MethodPost, "/v1/auth/login", map[string]string{
		"username": "admin",
		"password": "wrong-password",
	}, "")
	if badResp.Code != http.StatusUnauthorized {
		t.Fatalf("bad login status = %d, body = %s, want 401", badResp.Code, badResp.Body.String())
	}
	assertNotContains(t, badResp.Body.String(), "correct-password")

	goodResp := performJSONRequest(t, server, http.MethodPost, "/v1/auth/login", map[string]string{
		"username": "admin",
		"password": "correct-password",
	}, "")
	if goodResp.Code != http.StatusOK {
		t.Fatalf("good login status = %d, body = %s", goodResp.Code, goodResp.Body.String())
	}
	assertNotContains(t, goodResp.Body.String(), "correct-password")

	var login struct {
		Success bool `json:"success"`
		Data    struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	decodeResponse(t, goodResp, &login)
	if !login.Success || login.Data.Token == "" {
		t.Fatalf("login response = %+v, want token", login)
	}
}

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

func TestMaintenanceSuppressesIncidentCandidates(t *testing.T) {
	server := setupTestServer(t)
	registered := registerTestAgent(t, server)
	registeredMonitor := registerTestMonitor(t, server, registered.Data.AgentID, registered.Data.Token)

	reportBody := map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"health":    "down",
		"metrics":   map[string]interface{}{},
		"error": map[string]string{
			"message": "connection refused",
		},
	}
	reportPath := "/v1/agents/" + registered.Data.AgentID + "/" + registeredMonitor.Data.MonitorID + "/report"
	reportResp := performJSONRequest(t, server, http.MethodPost, reportPath, reportBody, registered.Data.Token)
	if reportResp.Code != http.StatusOK {
		t.Fatalf("monitor report status = %d, body = %s", reportResp.Code, reportResp.Body.String())
	}

	beforeMaintenance := performJSONRequest(t, server, http.MethodGet, "/v1/incidents/candidates", nil, "")
	assertIncidentCandidateCount(t, beforeMaintenance, 1)

	maintenanceResp := performJSONRequest(
		t,
		server,
		http.MethodPut,
		"/v1/agents/"+registered.Data.AgentID+"/maintenance",
		map[string]bool{"maintenance_mode": true},
		registered.Data.Token,
	)
	if maintenanceResp.Code != http.StatusOK {
		t.Fatalf("maintenance status = %d, body = %s", maintenanceResp.Code, maintenanceResp.Body.String())
	}

	afterMaintenance := performJSONRequest(t, server, http.MethodGet, "/v1/incidents/candidates", nil, "")
	assertIncidentCandidateCount(t, afterMaintenance, 0)

	disableMaintenanceResp := performJSONRequest(
		t,
		server,
		http.MethodPut,
		"/v1/agents/"+registered.Data.AgentID+"/maintenance",
		map[string]bool{"maintenance_mode": false},
		registered.Data.Token,
	)
	if disableMaintenanceResp.Code != http.StatusOK {
		t.Fatalf("disable maintenance status = %d, body = %s", disableMaintenanceResp.Code, disableMaintenanceResp.Body.String())
	}

	afterMaintenanceDisabled := performJSONRequest(t, server, http.MethodGet, "/v1/incidents/candidates", nil, "")
	assertIncidentCandidateCount(t, afterMaintenanceDisabled, 1)
}

func TestIncidentCandidatesIncludeStaleMonitors(t *testing.T) {
	server := setupTestServer(t)
	agent := db.Agent{
		ID:        "agent-stale-candidate",
		MachineId: "machine-stale-candidate",
		Name:      "stale candidate",
		OS:        "linux",
		Arch:      "arm64",
		Token:     "token-stale-candidate",
		LastSeen:  time.Now(),
	}
	if err := server.db.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}

	monitor := db.Monitor{
		ID:                       "monitor-stale-candidate",
		AgentID:                  agent.ID,
		Name:                     "stale monitor",
		Type:                     "http",
		Lifecycle:                "active",
		Health:                   "up",
		ComputedHealth:           "up",
		ReportingIntervalSeconds: 60,
	}
	if err := server.db.Create(&monitor).Error; err != nil {
		t.Fatalf("create monitor: %v", err)
	}

	oldTimestamp := time.Now().Add(-30 * time.Minute)
	report := db.MonitorReport{
		ID:          "monitor-report-stale-candidate",
		MonitorID:   monitor.ID,
		Payload:     "{}",
		CollectedAt: oldTimestamp.UTC().Format(time.RFC3339),
		Health:      "up",
		CreatedAt:   oldTimestamp,
	}
	if err := server.db.Create(&report).Error; err != nil {
		t.Fatalf("create monitor report: %v", err)
	}

	candidatesResp := performJSONRequest(t, server, http.MethodGet, "/v1/incidents/candidates", nil, "")
	if candidatesResp.Code != http.StatusOK {
		t.Fatalf("incident candidates status = %d, body = %s", candidatesResp.Code, candidatesResp.Body.String())
	}

	var candidates struct {
		Success bool `json:"success"`
		Data    struct {
			Candidates []struct {
				MonitorID string `json:"monitor_id"`
				Health    string `json:"health"`
				Severity  string `json:"severity"`
				IssueType string `json:"issue_type"`
			} `json:"candidates"`
			Count int `json:"count"`
		} `json:"data"`
	}
	decodeResponse(t, candidatesResp, &candidates)
	if !candidates.Success || candidates.Data.Count != 1 || len(candidates.Data.Candidates) != 1 {
		t.Fatalf("incident candidates = %+v, want one stale candidate", candidates)
	}
	candidate := candidates.Data.Candidates[0]
	if candidate.MonitorID != monitor.ID || candidate.Health != "stale" || candidate.Severity != "high" || candidate.IssueType != "stale_data" {
		t.Fatalf("stale candidate = %+v, want monitor stale high stale_data", candidate)
	}
}

func TestMonitorReportsOpenAndResolveIncident(t *testing.T) {
	server := setupTestServer(t)
	registered := registerTestAgent(t, server)
	registeredMonitor := registerTestMonitor(t, server, registered.Data.AgentID, registered.Data.Token)
	reportPath := "/v1/agents/" + registered.Data.AgentID + "/" + registeredMonitor.Data.MonitorID + "/report"

	downResp := performJSONRequest(t, server, http.MethodPost, reportPath, map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"health":    "down",
		"metrics": map[string]interface{}{
			"failure_reason": "connection refused",
		},
	}, registered.Data.Token)
	if downResp.Code != http.StatusOK {
		t.Fatalf("down report status = %d, body = %s", downResp.Code, downResp.Body.String())
	}

	var incident db.Incident
	if err := server.db.Where("monitor_id = ?", registeredMonitor.Data.MonitorID).First(&incident).Error; err != nil {
		t.Fatalf("find incident: %v", err)
	}
	if incident.Status != "open" || incident.Severity != "high" || incident.NotificationStatus != "pending" {
		t.Fatalf("incident = %+v, want open high pending", incident)
	}
	assertAlertDelivery(t, server, incident.ID, "incident_opened", "suppressed")
	assertMonitorIncidentState(t, server, registeredMonitor.Data.MonitorID, incident.ID, "down")

	downAgainResp := performJSONRequest(t, server, http.MethodPost, reportPath, map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"health":    "down",
		"metrics":   map[string]interface{}{},
	}, registered.Data.Token)
	if downAgainResp.Code != http.StatusOK {
		t.Fatalf("second down report status = %d, body = %s", downAgainResp.Code, downAgainResp.Body.String())
	}
	var incidentCount int64
	if err := server.db.Model(&db.Incident{}).Where("monitor_id = ?", registeredMonitor.Data.MonitorID).Count(&incidentCount).Error; err != nil {
		t.Fatalf("count incidents: %v", err)
	}
	if incidentCount != 1 {
		t.Fatalf("incident count = %d, want 1", incidentCount)
	}

	upResp := performJSONRequest(t, server, http.MethodPost, reportPath, map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"health":    "up",
		"metrics": map[string]interface{}{
			"status_code": 200,
		},
	}, registered.Data.Token)
	if upResp.Code != http.StatusOK {
		t.Fatalf("up report status = %d, body = %s", upResp.Code, upResp.Body.String())
	}

	if err := server.db.Where("id = ?", incident.ID).First(&incident).Error; err != nil {
		t.Fatalf("reload incident: %v", err)
	}
	if incident.Status != "resolved" || incident.ResolvedAt == nil {
		t.Fatalf("incident = %+v, want resolved with resolved_at", incident)
	}
	assertAlertDelivery(t, server, incident.ID, "incident_resolved", "suppressed")
	assertMonitorIncidentState(t, server, registeredMonitor.Data.MonitorID, "", "up")

	var eventCount int64
	if err := server.db.Model(&db.IncidentEvent{}).Where("incident_id = ?", incident.ID).Count(&eventCount).Error; err != nil {
		t.Fatalf("count incident events: %v", err)
	}
	if eventCount != 3 {
		t.Fatalf("incident event count = %d, want 3", eventCount)
	}
}

func TestListIncidentsReturnsPersistedActiveIncidents(t *testing.T) {
	server := setupTestServer(t)
	registered := registerTestAgent(t, server)
	registeredMonitor := registerTestMonitor(t, server, registered.Data.AgentID, registered.Data.Token)

	reportPath := "/v1/agents/" + registered.Data.AgentID + "/" + registeredMonitor.Data.MonitorID + "/report"
	reportResp := performJSONRequest(t, server, http.MethodPost, reportPath, map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"health":    "down",
		"metrics":   map[string]interface{}{"status_code": 500},
	}, registered.Data.Token)
	if reportResp.Code != http.StatusOK {
		t.Fatalf("monitor report status = %d, body = %s", reportResp.Code, reportResp.Body.String())
	}

	incidentsResp := performJSONRequest(t, server, http.MethodGet, "/v1/incidents", nil, "")
	if incidentsResp.Code != http.StatusOK {
		t.Fatalf("incidents status = %d, body = %s", incidentsResp.Code, incidentsResp.Body.String())
	}

	var listed struct {
		Success bool `json:"success"`
		Data    struct {
			Incidents []struct {
				Status      string `json:"status"`
				AgentID     string `json:"agent_id"`
				AgentName   string `json:"agent_name"`
				MonitorName string `json:"monitor_name"`
			} `json:"incidents"`
			Count int64 `json:"count"`
		} `json:"data"`
	}
	decodeResponse(t, incidentsResp, &listed)
	if !listed.Success || listed.Data.Count != 1 || len(listed.Data.Incidents) != 1 {
		t.Fatalf("incidents response = %+v, want one active incident", listed)
	}
	if listed.Data.Incidents[0].Status != "open" ||
		listed.Data.Incidents[0].AgentName != "test-server" ||
		listed.Data.Incidents[0].MonitorName != "homepage" {
		t.Fatalf("incident row = %+v, want open homepage on test-server", listed.Data.Incidents[0])
	}

	filteredResp := performJSONRequest(t, server, http.MethodGet, "/v1/incidents?agent_id="+registered.Data.AgentID, nil, "")
	if filteredResp.Code != http.StatusOK {
		t.Fatalf("filtered incidents status = %d, body = %s", filteredResp.Code, filteredResp.Body.String())
	}
	var filtered struct {
		Success bool `json:"success"`
		Data    struct {
			Incidents []struct {
				AgentID string `json:"agent_id"`
			} `json:"incidents"`
			Count int64 `json:"count"`
		} `json:"data"`
	}
	decodeResponse(t, filteredResp, &filtered)
	if !filtered.Success || filtered.Data.Count != 1 || filtered.Data.Incidents[0].AgentID != registered.Data.AgentID {
		t.Fatalf("filtered incidents response = %+v, want one incident for agent %s", filtered, registered.Data.AgentID)
	}

	monitorFilteredResp := performJSONRequest(t, server, http.MethodGet, "/v1/incidents?monitor_id="+registeredMonitor.Data.MonitorID, nil, "")
	if monitorFilteredResp.Code != http.StatusOK {
		t.Fatalf("monitor filtered incidents status = %d, body = %s", monitorFilteredResp.Code, monitorFilteredResp.Body.String())
	}
	var monitorFiltered struct {
		Data struct {
			Incidents []struct {
				MonitorName string `json:"monitor_name"`
			} `json:"incidents"`
			Count int64 `json:"count"`
		} `json:"data"`
	}
	decodeResponse(t, monitorFilteredResp, &monitorFiltered)
	if monitorFiltered.Data.Count != 1 || monitorFiltered.Data.Incidents[0].MonitorName != "homepage" {
		t.Fatalf("monitor filtered incidents = %+v, want homepage incident", monitorFiltered)
	}

	noMatchResp := performJSONRequest(t, server, http.MethodGet, "/v1/incidents?agent_id=agent-no-match", nil, "")
	if noMatchResp.Code != http.StatusOK {
		t.Fatalf("no match incidents status = %d, body = %s", noMatchResp.Code, noMatchResp.Body.String())
	}
	var noMatch struct {
		Data struct {
			Count int64 `json:"count"`
		} `json:"data"`
	}
	decodeResponse(t, noMatchResp, &noMatch)
	if noMatch.Data.Count != 0 {
		t.Fatalf("no match incident count = %d, want 0", noMatch.Data.Count)
	}

	highSeverityReviewResp := performJSONRequest(t, server, http.MethodGet, "/v1/incidents?needs_review=true", nil, "")
	if highSeverityReviewResp.Code != http.StatusOK {
		t.Fatalf("high severity review incidents status = %d, body = %s", highSeverityReviewResp.Code, highSeverityReviewResp.Body.String())
	}
	var highSeverityReview struct {
		Data struct {
			Count     int64 `json:"count"`
			Incidents []struct {
				Severity string `json:"severity"`
			} `json:"incidents"`
		} `json:"data"`
	}
	decodeResponse(t, highSeverityReviewResp, &highSeverityReview)
	if highSeverityReview.Data.Count != 1 || highSeverityReview.Data.Incidents[0].Severity != "high" {
		t.Fatalf("high severity review incidents = %+v, want high severity incident", highSeverityReview)
	}

	if err := server.db.Model(&db.Incident{}).
		Where("monitor_id = ?", registeredMonitor.Data.MonitorID).
		Update("notification_status", "failed").Error; err != nil {
		t.Fatalf("mark incident notification failed: %v", err)
	}

	needsReviewResp := performJSONRequest(t, server, http.MethodGet, "/v1/incidents?needs_review=true", nil, "")
	if needsReviewResp.Code != http.StatusOK {
		t.Fatalf("needs review incidents status = %d, body = %s", needsReviewResp.Code, needsReviewResp.Body.String())
	}
	var needsReview struct {
		Data struct {
			Count     int64 `json:"count"`
			Incidents []struct {
				NotificationStatus string `json:"notification_status"`
			} `json:"incidents"`
		} `json:"data"`
	}
	decodeResponse(t, needsReviewResp, &needsReview)
	if needsReview.Data.Count != 1 || needsReview.Data.Incidents[0].NotificationStatus != "failed" {
		t.Fatalf("needs review incidents = %+v, want failed notification incident", needsReview)
	}
}

func TestIncidentDetailAndTimelineEndpoints(t *testing.T) {
	server := setupTestServer(t)
	registered := registerTestAgent(t, server)
	registeredMonitor := registerTestMonitor(t, server, registered.Data.AgentID, registered.Data.Token)

	reportPath := "/v1/agents/" + registered.Data.AgentID + "/" + registeredMonitor.Data.MonitorID + "/report"
	downResp := performJSONRequest(t, server, http.MethodPost, reportPath, map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"health":    "down",
		"metrics":   map[string]interface{}{"status_code": 500},
	}, registered.Data.Token)
	if downResp.Code != http.StatusOK {
		t.Fatalf("down report status = %d, body = %s", downResp.Code, downResp.Body.String())
	}

	var incident db.Incident
	if err := server.db.Where("monitor_id = ?", registeredMonitor.Data.MonitorID).First(&incident).Error; err != nil {
		t.Fatalf("find incident: %v", err)
	}

	detailResp := performJSONRequest(t, server, http.MethodGet, "/v1/incidents/"+incident.ID, nil, "")
	if detailResp.Code != http.StatusOK {
		t.Fatalf("incident detail status = %d, body = %s", detailResp.Code, detailResp.Body.String())
	}
	var detail struct {
		Success bool `json:"success"`
		Data    struct {
			Incident struct {
				ID          string `json:"id"`
				AgentName   string `json:"agent_name"`
				MonitorName string `json:"monitor_name"`
			} `json:"incident"`
			Timeline []struct {
				Type   string `json:"type"`
				Source string `json:"source"`
			} `json:"timeline"`
			AlertDeliveries []struct {
				Status string `json:"status"`
			} `json:"alert_deliveries"`
			MonitorReports []struct {
				Health string `json:"health"`
			} `json:"monitor_reports"`
		} `json:"data"`
	}
	decodeResponse(t, detailResp, &detail)
	if !detail.Success || detail.Data.Incident.ID != incident.ID {
		t.Fatalf("incident detail response = %+v, want incident %s", detail, incident.ID)
	}
	if detail.Data.Incident.AgentName != "test-server" || detail.Data.Incident.MonitorName != "homepage" {
		t.Fatalf("incident names = %+v, want agent and monitor names", detail.Data.Incident)
	}
	if len(detail.Data.Timeline) < 2 || len(detail.Data.AlertDeliveries) == 0 || len(detail.Data.MonitorReports) == 0 {
		t.Fatalf("incident detail linked data missing: %+v", detail.Data)
	}

	timelineResp := performJSONRequest(t, server, http.MethodGet, "/v1/incidents/"+incident.ID+"/timeline", nil, "")
	if timelineResp.Code != http.StatusOK {
		t.Fatalf("incident timeline status = %d, body = %s", timelineResp.Code, timelineResp.Body.String())
	}
	var timeline struct {
		Success bool `json:"success"`
		Data    struct {
			Timeline []struct {
				Source string `json:"source"`
			} `json:"timeline"`
			Count int `json:"count"`
		} `json:"data"`
	}
	decodeResponse(t, timelineResp, &timeline)
	if !timeline.Success || timeline.Data.Count < 2 {
		t.Fatalf("timeline response = %+v, want incident and alert events", timeline)
	}
}

func TestListOrionEvents(t *testing.T) {
	server := setupTestServer(t)
	registered := registerTestAgent(t, server)
	registeredMonitor := registerTestMonitor(t, server, registered.Data.AgentID, registered.Data.Token)

	reportPath := "/v1/agents/" + registered.Data.AgentID + "/" + registeredMonitor.Data.MonitorID + "/report"
	reportResp := performJSONRequest(t, server, http.MethodPost, reportPath, map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"health":    "down",
		"metrics":   map[string]interface{}{},
	}, registered.Data.Token)
	if reportResp.Code != http.StatusOK {
		t.Fatalf("monitor report status = %d, body = %s", reportResp.Code, reportResp.Body.String())
	}

	eventsResp := performJSONRequest(t, server, http.MethodGet, "/v1/events?limit=20", nil, "")
	if eventsResp.Code != http.StatusOK {
		t.Fatalf("events status = %d, body = %s", eventsResp.Code, eventsResp.Body.String())
	}
	var listed struct {
		Success bool `json:"success"`
		Data    struct {
			Events []struct {
				Type   string `json:"type"`
				Source string `json:"source"`
			} `json:"events"`
			Count int `json:"count"`
		} `json:"data"`
	}
	decodeResponse(t, eventsResp, &listed)
	if !listed.Success || listed.Data.Count == 0 || len(listed.Data.Events) == 0 {
		t.Fatalf("events response = %+v, want events", listed)
	}
	foundIncidentEvent := false
	for _, event := range listed.Data.Events {
		if event.Source == "incident_event" {
			foundIncidentEvent = true
			break
		}
	}
	if !foundIncidentEvent {
		t.Fatalf("events response = %+v, want incident event", listed.Data.Events)
	}

	pagedResp := performJSONRequest(t, server, http.MethodGet, "/v1/events?limit=1&offset=0", nil, "")
	if pagedResp.Code != http.StatusOK {
		t.Fatalf("paged events status = %d, body = %s", pagedResp.Code, pagedResp.Body.String())
	}
	var paged struct {
		Success bool `json:"success"`
		Data    struct {
			Events []struct {
				Type string `json:"type"`
			} `json:"events"`
			Count      int `json:"count"`
			Pagination struct {
				TotalItems int64 `json:"total_items"`
			} `json:"pagination"`
		} `json:"data"`
	}
	decodeResponse(t, pagedResp, &paged)
	if !paged.Success || len(paged.Data.Events) != 1 {
		t.Fatalf("paged events response = %+v, want one returned event", paged)
	}
	if paged.Data.Count <= len(paged.Data.Events) || paged.Data.Pagination.TotalItems != int64(paged.Data.Count) {
		t.Fatalf("paged event count = %+v, want total count larger than returned rows", paged.Data)
	}

	filteredResp := performJSONRequest(t, server, http.MethodGet, "/v1/events?source=incident_event&type=incident_opened&q=homepage&limit=20", nil, "")
	if filteredResp.Code != http.StatusOK {
		t.Fatalf("filtered events status = %d, body = %s", filteredResp.Code, filteredResp.Body.String())
	}
	var filtered struct {
		Success bool `json:"success"`
		Data    struct {
			Events []struct {
				Type    string `json:"type"`
				Source  string `json:"source"`
				Message string `json:"message"`
			} `json:"events"`
			Count int `json:"count"`
		} `json:"data"`
	}
	decodeResponse(t, filteredResp, &filtered)
	if !filtered.Success || filtered.Data.Count == 0 || len(filtered.Data.Events) == 0 {
		t.Fatalf("filtered events response = %+v, want filtered events", filtered)
	}
	for _, event := range filtered.Data.Events {
		if event.Source != "incident_event" || event.Type != "incident_opened" || !strings.Contains(strings.ToLower(event.Message), "homepage") {
			t.Fatalf("filtered event = %+v, want matching source, type, and search", event)
		}
	}
}

func TestMaintenanceSuppressesAutomaticIncidentOpen(t *testing.T) {
	server := setupTestServer(t)
	registered := registerTestAgent(t, server)
	registeredMonitor := registerTestMonitor(t, server, registered.Data.AgentID, registered.Data.Token)

	maintenanceResp := performJSONRequest(
		t,
		server,
		http.MethodPut,
		"/v1/agents/"+registered.Data.AgentID+"/maintenance",
		map[string]bool{"maintenance_mode": true},
		registered.Data.Token,
	)
	if maintenanceResp.Code != http.StatusOK {
		t.Fatalf("maintenance status = %d, body = %s", maintenanceResp.Code, maintenanceResp.Body.String())
	}

	reportPath := "/v1/agents/" + registered.Data.AgentID + "/" + registeredMonitor.Data.MonitorID + "/report"
	downResp := performJSONRequest(t, server, http.MethodPost, reportPath, map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"health":    "down",
		"metrics":   map[string]interface{}{},
	}, registered.Data.Token)
	if downResp.Code != http.StatusOK {
		t.Fatalf("down report status = %d, body = %s", downResp.Code, downResp.Body.String())
	}

	var incidentCount int64
	if err := server.db.Model(&db.Incident{}).Where("monitor_id = ?", registeredMonitor.Data.MonitorID).Count(&incidentCount).Error; err != nil {
		t.Fatalf("count incidents: %v", err)
	}
	if incidentCount != 0 {
		t.Fatalf("incident count = %d, want 0", incidentCount)
	}
}

func TestTLSExpiryMetricOpensAndResolvesIncident(t *testing.T) {
	server := setupTestServer(t)
	registered := registerTestAgent(t, server)
	registeredMonitor := registerTestMonitor(t, server, registered.Data.AgentID, registered.Data.Token)
	reportPath := "/v1/agents/" + registered.Data.AgentID + "/" + registeredMonitor.Data.MonitorID + "/report"

	expiringResp := performJSONRequest(t, server, http.MethodPost, reportPath, map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"health":    "up",
		"metrics": map[string]interface{}{
			"tls_days_remaining": 3,
		},
	}, registered.Data.Token)
	if expiringResp.Code != http.StatusOK {
		t.Fatalf("expiring TLS report status = %d, body = %s", expiringResp.Code, expiringResp.Body.String())
	}

	var incident db.Incident
	if err := server.db.Where("monitor_id = ?", registeredMonitor.Data.MonitorID).First(&incident).Error; err != nil {
		t.Fatalf("find incident: %v", err)
	}
	if incident.Status != "open" || incident.Severity != "medium" {
		t.Fatalf("incident = %+v, want open medium", incident)
	}

	healthyResp := performJSONRequest(t, server, http.MethodPost, reportPath, map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"health":    "up",
		"metrics": map[string]interface{}{
			"tls_days_remaining": 90,
		},
	}, registered.Data.Token)
	if healthyResp.Code != http.StatusOK {
		t.Fatalf("healthy TLS report status = %d, body = %s", healthyResp.Code, healthyResp.Body.String())
	}

	if err := server.db.Where("id = ?", incident.ID).First(&incident).Error; err != nil {
		t.Fatalf("reload incident: %v", err)
	}
	if incident.Status != "resolved" {
		t.Fatalf("incident status = %q, want resolved", incident.Status)
	}
}

func TestAgentReportOpensStaleMonitorIncident(t *testing.T) {
	server := setupTestServer(t)
	registered := registerTestAgent(t, server)
	registeredMonitor := registerTestMonitor(t, server, registered.Data.AgentID, registered.Data.Token)

	if err := server.db.Model(&db.Monitor{}).
		Where("id = ?", registeredMonitor.Data.MonitorID).
		Update("created_at", time.Now().Add(-20*time.Minute)).Error; err != nil {
		t.Fatalf("age monitor: %v", err)
	}

	reportBody := map[string]interface{}{
		"uptime_seconds": 120,
		"timestamp":      time.Now().UTC().Format(time.RFC3339),
		"cpu":            map[string]interface{}{},
		"memory":         map[string]interface{}{},
		"disk":           map[string]interface{}{},
	}
	reportPath := "/v1/agents/" + registered.Data.AgentID + "/report"
	reportResp := performJSONRequest(t, server, http.MethodPost, reportPath, reportBody, registered.Data.Token)
	if reportResp.Code != http.StatusOK {
		t.Fatalf("agent report status = %d, body = %s", reportResp.Code, reportResp.Body.String())
	}

	var incident db.Incident
	if err := server.db.Where("monitor_id = ?", registeredMonitor.Data.MonitorID).First(&incident).Error; err != nil {
		t.Fatalf("find incident: %v", err)
	}
	if incident.Status != "open" || incident.Severity != "high" {
		t.Fatalf("incident = %+v, want open high stale incident", incident)
	}
}

func TestDataLifecycleSettingsFlow(t *testing.T) {
	server := setupTestServer(t)

	getResp := performJSONRequest(t, server, http.MethodGet, "/v1/settings/data-lifecycle", nil, "")
	if getResp.Code != http.StatusOK {
		t.Fatalf("get settings status = %d, body = %s", getResp.Code, getResp.Body.String())
	}

	var settingsResp struct {
		Success bool `json:"success"`
		Data    struct {
			Settings struct {
				RawReportHotDays  int    `json:"raw_report_hot_days"`
				ArchiveRawReports bool   `json:"archive_raw_reports"`
				ArchiveDir        string `json:"archive_dir"`
				RollupsEnabled    bool   `json:"rollups_enabled"`
				ArchiveSchedule   string `json:"archive_schedule"`
			} `json:"settings"`
		} `json:"data"`
	}
	decodeResponse(t, getResp, &settingsResp)
	if !settingsResp.Success || settingsResp.Data.Settings.RawReportHotDays != 90 {
		t.Fatalf("settings response = %+v, want default hot days", settingsResp)
	}

	updateResp := performJSONRequest(t, server, http.MethodPut, "/v1/settings/data-lifecycle", map[string]interface{}{
		"raw_report_hot_days":   120,
		"archive_raw_reports":   true,
		"archive_dir":           "data/archive",
		"rollups_enabled":       true,
		"rollup_retention_days": nil,
		"archive_schedule":      "manual",
	}, "")
	if updateResp.Code != http.StatusOK {
		t.Fatalf("update settings status = %d, body = %s", updateResp.Code, updateResp.Body.String())
	}

	decodeResponse(t, updateResp, &settingsResp)
	if settingsResp.Data.Settings.RawReportHotDays != 120 || settingsResp.Data.Settings.ArchiveSchedule != "manual" {
		t.Fatalf("updated settings = %+v, want updated values", settingsResp.Data.Settings)
	}
}

func setupTestServer(t *testing.T) *Server {
	t.Helper()
	gin.SetMode(gin.TestMode)

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	database, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.Migrate(database); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	return NewServer(database, logging.NewLogger(), &config.Config{AlertRecoveryNotifications: true, AlertTLSExpiryDays: 14})
}

func TestDataLifecycleActionsFlow(t *testing.T) {
	server := setupTestServer(t)
	registered := registerTestAgent(t, server)
	registeredMonitor := registerTestMonitor(t, server, registered.Data.AgentID, registered.Data.Token)

	reportTime := time.Date(2026, 5, 12, 10, 0, 0, 0, time.UTC)
	reportPath := "/v1/agents/" + registered.Data.AgentID + "/" + registeredMonitor.Data.MonitorID + "/report"
	for _, health := range []string{"up", "down"} {
		resp := performJSONRequest(t, server, http.MethodPost, reportPath, map[string]interface{}{
			"timestamp": reportTime.Format(time.RFC3339),
			"health":    health,
			"metrics":   map[string]interface{}{},
		}, registered.Data.Token)
		if resp.Code != http.StatusOK {
			t.Fatalf("monitor report status = %d, body = %s", resp.Code, resp.Body.String())
		}
	}
	if err := server.db.Model(&db.MonitorReport{}).Where("monitor_id = ?", registeredMonitor.Data.MonitorID).Update("created_at", reportTime).Error; err != nil {
		t.Fatalf("set monitor report created_at: %v", err)
	}

	rollupResp := performJSONRequest(t, server, http.MethodPost, "/v1/settings/data-lifecycle/actions/rollup", map[string]string{
		"date": "2026-05-12",
	}, "")
	if rollupResp.Code != http.StatusOK {
		t.Fatalf("rollup status = %d, body = %s", rollupResp.Code, rollupResp.Body.String())
	}

	var rollup db.MonitorUptimeRollup
	if err := server.db.Where("monitor_id = ? AND date = ?", registeredMonitor.Data.MonitorID, "2026-05-12").First(&rollup).Error; err != nil {
		t.Fatalf("find rollup: %v", err)
	}
	if rollup.UpCount != 1 || rollup.DownCount != 1 || rollup.TotalCount != 2 {
		t.Fatalf("rollup = %+v, want one up, one down, two total", rollup)
	}

	updateResp := performJSONRequest(t, server, http.MethodPut, "/v1/settings/data-lifecycle", map[string]interface{}{
		"raw_report_hot_days":   0,
		"archive_raw_reports":   true,
		"archive_dir":           t.TempDir(),
		"rollups_enabled":       true,
		"rollup_retention_days": nil,
		"archive_schedule":      "manual",
	}, "")
	if updateResp.Code != http.StatusBadRequest {
		t.Fatalf("invalid settings status = %d, body = %s", updateResp.Code, updateResp.Body.String())
	}

	updateResp = performJSONRequest(t, server, http.MethodPut, "/v1/settings/data-lifecycle", map[string]interface{}{
		"raw_report_hot_days":   1,
		"archive_raw_reports":   true,
		"archive_dir":           t.TempDir(),
		"rollups_enabled":       true,
		"rollup_retention_days": nil,
		"archive_schedule":      "manual",
	}, "")
	if updateResp.Code != http.StatusOK {
		t.Fatalf("settings update status = %d, body = %s", updateResp.Code, updateResp.Body.String())
	}

	archiveResp := performJSONRequest(t, server, http.MethodPost, "/v1/settings/data-lifecycle/actions/archive", nil, "")
	if archiveResp.Code != http.StatusOK {
		t.Fatalf("archive status = %d, body = %s", archiveResp.Code, archiveResp.Body.String())
	}
}

func TestCORSPreflightAllowsConsoleFetchHeaders(t *testing.T) {
	server := setupTestServer(t)

	req := httptest.NewRequest(http.MethodOptions, "/v1/agents", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	req.Header.Set("Access-Control-Request-Method", http.MethodGet)
	req.Header.Set("Access-Control-Request-Headers", "authorization,content-type,cache-control,pragma")
	recorder := httptest.NewRecorder()

	server.router.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:5173" {
		t.Fatalf("allow origin = %q, want localhost Vite origin", got)
	}
	allowHeaders := strings.ToLower(recorder.Header().Get("Access-Control-Allow-Headers"))
	for _, header := range []string{"authorization", "content-type", "cache-control", "pragma"} {
		if !strings.Contains(allowHeaders, header) {
			t.Fatalf("allow headers = %q, missing %q", allowHeaders, header)
		}
	}
}

func TestAlertReadEndpointsShowWebhookURLAndRedactSecrets(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	database, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.Migrate(database); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	server := NewServer(database, logging.NewLogger(), &config.Config{
		AlertRecoveryNotifications: true,
		AlertTLSExpiryDays:         14,
		AlertCooldownSeconds:       300,
	})
	if err := server.db.Create(&db.AlertChannel{
		ID:         "alert-channel-webhook",
		Name:       "ops-webhook",
		Type:       "webhook",
		Enabled:    true,
		WebhookURL: "https://secret.example.com/hook",
	}).Error; err != nil {
		t.Fatalf("create webhook channel: %v", err)
	}
	if err := server.db.Create(&db.AlertChannel{
		ID:           "alert-channel-email",
		Name:         "ops-email",
		Type:         "email",
		Enabled:      false,
		EmailTo:      "ops@example.com",
		EmailFrom:    "orion@example.com",
		SMTPHost:     "smtp.example.com",
		SMTPPort:     587,
		SMTPUsername: "mailer",
		SMTPPassword: "secret-password",
	}).Error; err != nil {
		t.Fatalf("create email channel: %v", err)
	}

	delivery := db.AlertDelivery{
		ID:           "alert-delivery-test",
		IncidentID:   "incident-test",
		EventType:    "incident_opened",
		Channel:      "ops-webhook",
		Type:         "webhook",
		Status:       "failed",
		Error:        "post https://secret.example.com/hook: connection refused",
		AttemptCount: 1,
		MaxAttempts:  3,
	}
	if err := server.db.Create(&delivery).Error; err != nil {
		t.Fatalf("create alert delivery: %v", err)
	}
	attemptTime := time.Now().UTC()
	if err := server.db.Create(&db.AlertDeliveryAttempt{
		ID:              "alert-delivery-attempt-test",
		AlertDeliveryID: delivery.ID,
		AttemptNumber:   1,
		Status:          "failed",
		Stage:           "http_request",
		Error:           "post https://secret.example.com/hook: connection refused",
		StartedAt:       attemptTime,
		CompletedAt:     &attemptTime,
	}).Error; err != nil {
		t.Fatalf("create alert delivery attempt: %v", err)
	}
	secondDelivery := db.AlertDelivery{
		ID:         "alert-delivery-sent",
		IncidentID: "incident-other",
		EventType:  "incident_resolved",
		Channel:    "ops-email",
		Type:       "email",
		Status:     "sent",
	}
	if err := server.db.Create(&secondDelivery).Error; err != nil {
		t.Fatalf("create second alert delivery: %v", err)
	}

	channelsResp := performJSONRequest(t, server, http.MethodGet, "/v1/alerts/channels", nil, "")
	if channelsResp.Code != http.StatusOK {
		t.Fatalf("channels status = %d, body = %s", channelsResp.Code, channelsResp.Body.String())
	}
	assertNotContains(t, channelsResp.Body.String(), "secret-password")

	var channels struct {
		Success bool `json:"success"`
		Data    struct {
			Channels []struct {
				Name               string `json:"name"`
				Type               string `json:"type"`
				WebhookURL         string `json:"webhook_url"`
				WebhookConfigured  bool   `json:"webhook_configured"`
				LastDeliveryStatus string `json:"last_delivery_status"`
			} `json:"channels"`
			Count int `json:"count"`
		} `json:"data"`
	}
	decodeResponse(t, channelsResp, &channels)
	if !channels.Success || channels.Data.Count != 2 || len(channels.Data.Channels) != 2 {
		t.Fatalf("channels response = %+v, want two channels", channels)
	}
	var webhookChannel struct {
		Name               string `json:"name"`
		Type               string `json:"type"`
		WebhookURL         string `json:"webhook_url"`
		WebhookConfigured  bool   `json:"webhook_configured"`
		LastDeliveryStatus string `json:"last_delivery_status"`
	}
	for _, channel := range channels.Data.Channels {
		if channel.Name == "ops-webhook" {
			webhookChannel = channel
			break
		}
	}
	if webhookChannel.WebhookURL != "https://secret.example.com/hook" || !webhookChannel.WebhookConfigured || webhookChannel.LastDeliveryStatus != "failed" {
		t.Fatalf("webhook channel response = %+v, want webhook URL with last failed status", webhookChannel)
	}

	deliveriesResp := performJSONRequest(t, server, http.MethodGet, "/v1/alerts/deliveries?limit=10", nil, "")
	if deliveriesResp.Code != http.StatusOK {
		t.Fatalf("deliveries status = %d, body = %s", deliveriesResp.Code, deliveriesResp.Body.String())
	}
	assertNotContains(t, deliveriesResp.Body.String(), "secret.example.com")
	if !strings.Contains(deliveriesResp.Body.String(), "delivery failed; check Core logs") {
		t.Fatalf("delivery error was not sanitized: %s", deliveriesResp.Body.String())
	}

	filteredDeliveriesResp := performJSONRequest(t, server, http.MethodGet, "/v1/alerts/deliveries?status=failed&incident_id=incident-test", nil, "")
	if filteredDeliveriesResp.Code != http.StatusOK {
		t.Fatalf("filtered deliveries status = %d, body = %s", filteredDeliveriesResp.Code, filteredDeliveriesResp.Body.String())
	}
	var filteredDeliveries struct {
		Success bool `json:"success"`
		Data    struct {
			Deliveries []struct {
				IncidentID string `json:"incident_id"`
				Status     string `json:"status"`
				Error      string `json:"error"`
				Attempts   []struct {
					Status string `json:"status"`
					Stage  string `json:"stage"`
					Error  string `json:"error"`
				} `json:"attempts"`
			} `json:"deliveries"`
			Count int64 `json:"count"`
		} `json:"data"`
	}
	decodeResponse(t, filteredDeliveriesResp, &filteredDeliveries)
	if !filteredDeliveries.Success || filteredDeliveries.Data.Count != 1 || len(filteredDeliveries.Data.Deliveries) != 1 {
		t.Fatalf("filtered deliveries response = %+v, want one delivery", filteredDeliveries)
	}
	filteredDelivery := filteredDeliveries.Data.Deliveries[0]
	if filteredDelivery.IncidentID != "incident-test" || filteredDelivery.Status != "failed" || filteredDelivery.Error != "delivery failed; check Core logs" {
		t.Fatalf("filtered delivery = %+v, want sanitized failed incident-test delivery", filteredDelivery)
	}
	if len(filteredDelivery.Attempts) != 1 || filteredDelivery.Attempts[0].Status != "failed" || filteredDelivery.Attempts[0].Stage != "http_request" || filteredDelivery.Attempts[0].Error != "delivery failed; check Core logs" {
		t.Fatalf("filtered delivery attempts = %+v, want sanitized http_request failure", filteredDelivery.Attempts)
	}

	destinationFilteredResp := performJSONRequest(t, server, http.MethodGet, "/v1/alerts/deliveries?type=email&channel=ops-email&event_type=incident_resolved", nil, "")
	if destinationFilteredResp.Code != http.StatusOK {
		t.Fatalf("destination filtered deliveries status = %d, body = %s", destinationFilteredResp.Code, destinationFilteredResp.Body.String())
	}
	var destinationFiltered struct {
		Success bool `json:"success"`
		Data    struct {
			Deliveries []struct {
				Channel   string `json:"channel"`
				Type      string `json:"type"`
				EventType string `json:"event_type"`
				Status    string `json:"status"`
			} `json:"deliveries"`
			Count int64 `json:"count"`
		} `json:"data"`
	}
	decodeResponse(t, destinationFilteredResp, &destinationFiltered)
	if !destinationFiltered.Success || destinationFiltered.Data.Count != 1 || len(destinationFiltered.Data.Deliveries) != 1 {
		t.Fatalf("destination filtered deliveries response = %+v, want one email delivery", destinationFiltered)
	}
	destinationDelivery := destinationFiltered.Data.Deliveries[0]
	if destinationDelivery.Channel != "ops-email" || destinationDelivery.Type != "email" || destinationDelivery.EventType != "incident_resolved" || destinationDelivery.Status != "sent" {
		t.Fatalf("destination filtered delivery = %+v, want sent ops-email incident_resolved delivery", destinationDelivery)
	}

	rulesResp := performJSONRequest(t, server, http.MethodGet, "/v1/alerts/rules", nil, "")
	if rulesResp.Code != http.StatusOK {
		t.Fatalf("rules status = %d, body = %s", rulesResp.Code, rulesResp.Body.String())
	}
	assertNotContains(t, rulesResp.Body.String(), "secret.example.com")
	assertNotContains(t, rulesResp.Body.String(), "secret-password")
}

func TestAlertChannelTestEndpointSendsConfiguredWebhook(t *testing.T) {
	server := setupTestServer(t)
	webhookPayloads := make(chan map[string]interface{}, 1)
	webhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("webhook method = %s, want POST", r.Method)
		}
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode webhook payload: %v", err)
		}
		webhookPayloads <- payload
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(webhookServer.Close)

	createResp := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/channels", gin.H{
		"name":        "ops-webhook",
		"type":        "webhook",
		"enabled":     false,
		"webhook_url": webhookServer.URL,
	}, "")
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create channel status = %d, body = %s", createResp.Code, createResp.Body.String())
	}
	var created struct {
		Data struct {
			Channel struct {
				ID string `json:"id"`
			} `json:"channel"`
		} `json:"data"`
	}
	decodeResponse(t, createResp, &created)

	testResp := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/channels/"+created.Data.Channel.ID+"/test", nil, "")
	if testResp.Code != http.StatusOK {
		t.Fatalf("test channel status = %d, body = %s", testResp.Code, testResp.Body.String())
	}
	var tested struct {
		Success bool `json:"success"`
		Data    struct {
			Delivery struct {
				IncidentID string `json:"incident_id"`
				EventType  string `json:"event_type"`
				Channel    string `json:"channel"`
				Type       string `json:"type"`
				Status     string `json:"status"`
				Error      string `json:"error"`
			} `json:"delivery"`
		} `json:"data"`
	}
	decodeResponse(t, testResp, &tested)
	if !tested.Success || tested.Data.Delivery.IncidentID != "alert-channel-test" || tested.Data.Delivery.EventType != "test" || tested.Data.Delivery.Channel != "ops-webhook" || tested.Data.Delivery.Type != "webhook" || tested.Data.Delivery.Status != "sent" || tested.Data.Delivery.Error != "" {
		t.Fatalf("test delivery response = %+v, want sent webhook test delivery", tested)
	}

	select {
	case payload := <-webhookPayloads:
		if payload["event_type"] != "test" {
			t.Fatalf("webhook payload = %+v, want test event", payload)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for webhook test payload")
	}

	var stored db.AlertDelivery
	if err := server.db.Where("incident_id = ? AND event_type = ? AND channel = ?", "alert-channel-test", "test", "ops-webhook").First(&stored).Error; err != nil {
		t.Fatalf("find stored test delivery: %v", err)
	}
	if stored.Status != "sent" {
		t.Fatalf("stored test delivery status = %q, want sent", stored.Status)
	}
}

func TestAlertChannelWriteEndpointsPersistWebhookConfiguration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	database, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.Migrate(database); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	server := NewServer(database, logging.NewLogger(), &config.Config{})
	createResp := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/channels", gin.H{
		"name":              "ops-webhook",
		"type":              "webhook",
		"enabled":           true,
		"webhook_url":       "https://secret.example.com/hook",
		"subscribed_events": []string{db.AlertEventIncidentOpened},
	}, "")
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create channel status = %d, body = %s", createResp.Code, createResp.Body.String())
	}
	var created struct {
		Data struct {
			Channel struct {
				ID                string   `json:"id"`
				Name              string   `json:"name"`
				WebhookURL        string   `json:"webhook_url"`
				WebhookConfigured bool     `json:"webhook_configured"`
				SubscribedEvents  []string `json:"subscribed_events"`
			} `json:"channel"`
		} `json:"data"`
	}
	decodeResponse(t, createResp, &created)
	if created.Data.Channel.ID == "" || created.Data.Channel.Name != "ops-webhook" || created.Data.Channel.WebhookURL != "https://secret.example.com/hook" || !created.Data.Channel.WebhookConfigured {
		t.Fatalf("created channel = %+v, want webhook channel", created.Data.Channel)
	}
	if got := created.Data.Channel.SubscribedEvents; len(got) != 1 || got[0] != db.AlertEventIncidentOpened {
		t.Fatalf("created subscribed_events = %#v, want incident_opened", got)
	}

	updateResp := performJSONRequest(t, server, http.MethodPatch, "/v1/alerts/channels/"+created.Data.Channel.ID, gin.H{
		"name":              "critical-webhook",
		"enabled":           false,
		"webhook_url":       "https://alerts.example.com/critical",
		"subscribed_events": []string{db.AlertEventIncidentOpened, db.AlertEventIncidentResolved},
	}, "")
	if updateResp.Code != http.StatusOK {
		t.Fatalf("update channel status = %d, body = %s", updateResp.Code, updateResp.Body.String())
	}

	var stored db.AlertChannel
	if err := server.db.Where("id = ?", created.Data.Channel.ID).First(&stored).Error; err != nil {
		t.Fatalf("find updated channel: %v", err)
	}
	if stored.Name != "critical-webhook" || stored.Enabled {
		t.Fatalf("stored channel = %+v, want renamed disabled channel", stored)
	}
	if stored.WebhookURL != "https://alerts.example.com/critical" {
		t.Fatalf("stored webhook url = %q, want updated webhook url", stored.WebhookURL)
	}
	if got := db.DecodeAlertEvents(stored.SubscribedEvents); len(got) != 2 || got[0] != db.AlertEventIncidentOpened || got[1] != db.AlertEventIncidentResolved {
		t.Fatalf("stored subscribed_events = %#v, want opened and resolved", got)
	}

	deleteResp := performJSONRequest(t, server, http.MethodDelete, "/v1/alerts/channels/"+created.Data.Channel.ID, nil, "")
	if deleteResp.Code != http.StatusOK {
		t.Fatalf("delete channel status = %d, body = %s", deleteResp.Code, deleteResp.Body.String())
	}
	var count int64
	if err := server.db.Model(&db.AlertChannel{}).Count(&count).Error; err != nil {
		t.Fatalf("count alert channels: %v", err)
	}
	if count != 0 {
		t.Fatalf("alert channel count = %d, want 0", count)
	}
}

func TestAlertRouteWriteAndDryRunEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	database, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.Migrate(database); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	server := NewServer(database, logging.NewLogger(), &config.Config{})
	if err := server.db.Create(&db.AlertChannel{
		ID:         "channel-ops-webhook",
		Name:       "ops-webhook",
		Type:       "webhook",
		Enabled:    true,
		WebhookURL: "https://alerts.example.com/hook",
	}).Error; err != nil {
		t.Fatalf("create alert channel: %v", err)
	}

	createResp := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/routes", gin.H{
		"name":        "critical route",
		"priority":    10,
		"event_types": []string{db.AlertEventIncidentOpened},
		"severities":  []string{"high"},
		"channel_ids": []string{"channel-ops-webhook"},
	}, "")
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create route status = %d, body = %s", createResp.Code, createResp.Body.String())
	}
	var created struct {
		Data struct {
			Route struct {
				ID         string   `json:"id"`
				Name       string   `json:"name"`
				EventTypes []string `json:"event_types"`
				ChannelIDs []string `json:"channel_ids"`
			} `json:"route"`
		} `json:"data"`
	}
	decodeResponse(t, createResp, &created)
	if created.Data.Route.ID == "" || created.Data.Route.Name != "critical route" || len(created.Data.Route.ChannelIDs) != 1 {
		t.Fatalf("created route = %+v, want route with channel", created.Data.Route)
	}

	dryRunResp := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/routes/dry-run", gin.H{
		"event_type": db.AlertEventIncidentOpened,
		"severity":   "high",
	}, "")
	if dryRunResp.Code != http.StatusOK {
		t.Fatalf("dry-run status = %d, body = %s", dryRunResp.Code, dryRunResp.Body.String())
	}
	var dryRun struct {
		Data struct {
			DryRun struct {
				Suppressed       bool `json:"suppressed"`
				RouteEvaluations []struct {
					Matched bool `json:"matched"`
				} `json:"route_evaluations"`
				DestinationDecisions []struct {
					ChannelName string `json:"channel_name"`
					Status      string `json:"status"`
				} `json:"destination_decisions"`
			} `json:"dry_run"`
		} `json:"data"`
	}
	decodeResponse(t, dryRunResp, &dryRun)
	if dryRun.Data.DryRun.Suppressed || len(dryRun.Data.DryRun.RouteEvaluations) != 1 || !dryRun.Data.DryRun.RouteEvaluations[0].Matched {
		t.Fatalf("dry-run route evaluations = %+v, want one matched non-suppressed route", dryRun.Data.DryRun)
	}
	if len(dryRun.Data.DryRun.DestinationDecisions) != 1 || dryRun.Data.DryRun.DestinationDecisions[0].ChannelName != "ops-webhook" || dryRun.Data.DryRun.DestinationDecisions[0].Status != "pending" {
		t.Fatalf("dry-run destinations = %+v, want pending ops-webhook", dryRun.Data.DryRun.DestinationDecisions)
	}

	var deliveryCount int64
	if err := server.db.Model(&db.AlertDelivery{}).Count(&deliveryCount).Error; err != nil {
		t.Fatalf("count deliveries: %v", err)
	}
	if deliveryCount != 0 {
		t.Fatalf("delivery count = %d, want dry-run to avoid side effects", deliveryCount)
	}
}

func TestAlertSMTPServiceAndEmailDestinationEndpoints(t *testing.T) {
	server := setupTestServer(t)

	createServiceResp := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/smtp-services", gin.H{
		"name":       "Primary SMTP",
		"enabled":    true,
		"host":       "smtp.example.com",
		"port":       587,
		"username":   "mailer",
		"password":   "secret-password",
		"from_email": "orion@example.com",
	}, "")
	if createServiceResp.Code != http.StatusCreated {
		t.Fatalf("create smtp service status = %d, body = %s", createServiceResp.Code, createServiceResp.Body.String())
	}
	assertNotContains(t, createServiceResp.Body.String(), "secret-password")

	var createdService struct {
		Data struct {
			SMTPService struct {
				ID                 string `json:"id"`
				Name               string `json:"name"`
				Host               string `json:"host"`
				Port               int    `json:"port"`
				UsernameConfigured bool   `json:"username_configured"`
				PasswordConfigured bool   `json:"password_configured"`
			} `json:"smtp_service"`
		} `json:"data"`
	}
	decodeResponse(t, createServiceResp, &createdService)
	if createdService.Data.SMTPService.ID == "" || createdService.Data.SMTPService.Name != "Primary SMTP" ||
		createdService.Data.SMTPService.Host != "smtp.example.com" || createdService.Data.SMTPService.Port != 587 ||
		!createdService.Data.SMTPService.UsernameConfigured || !createdService.Data.SMTPService.PasswordConfigured {
		t.Fatalf("created smtp service = %+v, want redacted configured service", createdService.Data.SMTPService)
	}

	createDestinationResp := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/email-destinations", gin.H{
		"smtp_service_id":   createdService.Data.SMTPService.ID,
		"name":              "Ops Email",
		"enabled":           true,
		"email_to":          "ops@example.com",
		"subscribed_events": []string{db.AlertEventIncidentOpened},
	}, "")
	if createDestinationResp.Code != http.StatusCreated {
		t.Fatalf("create email destination status = %d, body = %s", createDestinationResp.Code, createDestinationResp.Body.String())
	}

	var createdDestination struct {
		Data struct {
			EmailDestination struct {
				ID               string   `json:"id"`
				SMTPServiceID    string   `json:"smtp_service_id"`
				SMTPServiceName  string   `json:"smtp_service_name"`
				Name             string   `json:"name"`
				EmailTo          string   `json:"email_to"`
				SubscribedEvents []string `json:"subscribed_events"`
			} `json:"email_destination"`
		} `json:"data"`
	}
	decodeResponse(t, createDestinationResp, &createdDestination)
	if createdDestination.Data.EmailDestination.SMTPServiceID != createdService.Data.SMTPService.ID ||
		createdDestination.Data.EmailDestination.SMTPServiceName != "Primary SMTP" ||
		createdDestination.Data.EmailDestination.Name != "Ops Email" ||
		createdDestination.Data.EmailDestination.EmailTo != "ops@example.com" ||
		len(createdDestination.Data.EmailDestination.SubscribedEvents) != 1 ||
		createdDestination.Data.EmailDestination.SubscribedEvents[0] != db.AlertEventIncidentOpened {
		t.Fatalf("created email destination = %+v, want linked opened-only destination", createdDestination.Data.EmailDestination)
	}

	listServicesResp := performJSONRequest(t, server, http.MethodGet, "/v1/alerts/smtp-services", nil, "")
	if listServicesResp.Code != http.StatusOK {
		t.Fatalf("list smtp services status = %d, body = %s", listServicesResp.Code, listServicesResp.Body.String())
	}
	assertNotContains(t, listServicesResp.Body.String(), "secret-password")

	deleteServiceResp := performJSONRequest(t, server, http.MethodDelete, "/v1/alerts/smtp-services/"+createdService.Data.SMTPService.ID, nil, "")
	if deleteServiceResp.Code != http.StatusConflict {
		t.Fatalf("delete referenced smtp service status = %d, body = %s, want 409", deleteServiceResp.Code, deleteServiceResp.Body.String())
	}

	deleteDestinationResp := performJSONRequest(t, server, http.MethodDelete, "/v1/alerts/email-destinations/"+createdDestination.Data.EmailDestination.ID, nil, "")
	if deleteDestinationResp.Code != http.StatusOK {
		t.Fatalf("delete email destination status = %d, body = %s", deleteDestinationResp.Code, deleteDestinationResp.Body.String())
	}
	deleteServiceResp = performJSONRequest(t, server, http.MethodDelete, "/v1/alerts/smtp-services/"+createdService.Data.SMTPService.ID, nil, "")
	if deleteServiceResp.Code != http.StatusOK {
		t.Fatalf("delete smtp service status = %d, body = %s", deleteServiceResp.Code, deleteServiceResp.Body.String())
	}
}

func registerTestAgent(t *testing.T, server *Server) struct {
	Success bool `json:"success"`
	Data    struct {
		AgentID string `json:"agent_id"`
		Token   string `json:"token"`
	} `json:"data"`
} {
	t.Helper()

	registerBody := map[string]interface{}{
		"machine_id":                 "test-machine-" + t.Name(),
		"name":                       "test-server",
		"os":                         "linux",
		"arch":                       "arm64",
		"reporting_interval_seconds": 60,
	}
	registerResp := performJSONRequest(t, server, http.MethodPost, "/v1/register", registerBody, "")
	if registerResp.Code != http.StatusOK {
		t.Fatalf("register status = %d, body = %s", registerResp.Code, registerResp.Body.String())
	}

	var registered struct {
		Success bool `json:"success"`
		Data    struct {
			AgentID string `json:"agent_id"`
			Token   string `json:"token"`
		} `json:"data"`
	}
	decodeResponse(t, registerResp, &registered)
	if !registered.Success || registered.Data.AgentID == "" || registered.Data.Token == "" {
		t.Fatalf("registration response missing agent identity: %+v", registered)
	}

	return registered
}

func registerTestMonitor(t *testing.T, server *Server, agentID string, token string) struct {
	Success bool `json:"success"`
	Data    struct {
		MonitorID string `json:"monitor_id"`
	} `json:"data"`
} {
	t.Helper()

	description := "Checks the homepage"
	registerMonitorBody := map[string]interface{}{
		"agent_id":                   agentID,
		"name":                       "homepage",
		"description":                description,
		"type":                       "http-healthcheck",
		"last_checked":               time.Now().UTC().Format(time.RFC3339),
		"reporting_interval_seconds": 30,
	}
	registerMonitorPath := "/v1/agents/" + agentID + "/register-monitor"
	registerMonitorResp := performJSONRequest(t, server, http.MethodPost, registerMonitorPath, registerMonitorBody, token)
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
		t.Fatalf("registration response missing monitor identity: %+v", registeredMonitor)
	}

	return registeredMonitor
}

func assertIncidentCandidateCount(t *testing.T, response *httptest.ResponseRecorder, want int) {
	t.Helper()

	if response.Code != http.StatusOK {
		t.Fatalf("incident candidates status = %d, body = %s", response.Code, response.Body.String())
	}

	var candidates struct {
		Success bool `json:"success"`
		Data    struct {
			Count int `json:"count"`
		} `json:"data"`
	}
	decodeResponse(t, response, &candidates)
	if !candidates.Success || candidates.Data.Count != want {
		t.Fatalf("incident candidate count = %+v, want %d", candidates, want)
	}
}

func assertAlertDelivery(t *testing.T, server *Server, incidentID string, eventType string, wantStatus string) {
	t.Helper()

	var delivery db.AlertDelivery
	if err := server.db.Where("incident_id = ? AND event_type = ?", incidentID, eventType).First(&delivery).Error; err != nil {
		t.Fatalf("find alert delivery: %v", err)
	}
	if delivery.Status != wantStatus {
		t.Fatalf("alert delivery status = %q, want %q", delivery.Status, wantStatus)
	}
}

func assertMonitorIncidentState(t *testing.T, server *Server, monitorID string, wantActiveIncidentID string, wantIncidentState string) {
	t.Helper()

	var monitor db.Monitor
	if err := server.db.Where("id = ?", monitorID).First(&monitor).Error; err != nil {
		t.Fatalf("find monitor: %v", err)
	}
	if monitor.ActiveIncidentID != wantActiveIncidentID || monitor.IncidentState != wantIncidentState {
		t.Fatalf("monitor incident state = active %q state %q, want active %q state %q", monitor.ActiveIncidentID, monitor.IncidentState, wantActiveIncidentID, wantIncidentState)
	}
}

func assertFrontendResponseDoesNotExposeAgentSecrets(t *testing.T, body string, token string) {
	t.Helper()

	if strings.Contains(body, token) {
		t.Fatalf("frontend response exposed agent token: %s", body)
	}
	if strings.Contains(body, `"token"`) {
		t.Fatalf("frontend response exposed token field: %s", body)
	}
	if strings.Contains(body, `"machine_id"`) {
		t.Fatalf("frontend response exposed machine_id field: %s", body)
	}
}

func assertNotContains(t *testing.T, body string, value string) {
	t.Helper()

	if strings.Contains(body, value) {
		t.Fatalf("response exposed %q: %s", value, body)
	}
}

func performJSONRequest(t *testing.T, server *Server, method string, path string, body interface{}, token string) *httptest.ResponseRecorder {
	t.Helper()

	var payload bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&payload).Encode(body); err != nil {
			t.Fatalf("encode request body: %v", err)
		}
	}

	req := httptest.NewRequest(method, path, &payload)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, req)
	return recorder
}

func decodeResponse(t *testing.T, response *httptest.ResponseRecorder, target interface{}) {
	t.Helper()

	if err := json.Unmarshal(response.Body.Bytes(), target); err != nil {
		t.Fatalf("decode response %q: %v", response.Body.String(), err)
	}
}
