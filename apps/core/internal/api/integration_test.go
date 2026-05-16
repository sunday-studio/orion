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
			} `json:"monitor"`
		} `json:"data"`
	}
	decodeResponse(t, detailResp, &detail)
	if !detail.Success || detail.Data.ComputedHealth != "down" || detail.Data.Monitor.ComputedHealth != "down" {
		t.Fatalf("monitor detail health = %+v, want both computed health fields down", detail)
	}
}

func TestMonitorHistoryReturnsNotFoundForUnknownMonitor(t *testing.T) {
	server := setupTestServer(t)

	historyResp := performJSONRequest(t, server, http.MethodGet, "/v1/monitors/monitor-missing/history", nil, "")
	if historyResp.Code != http.StatusNotFound {
		t.Fatalf("monitor history status = %d, body = %s, want 404", historyResp.Code, historyResp.Body.String())
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

func TestAlertReadEndpointsRedactConfiguration(t *testing.T) {
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
		AlertChannels: []config.AlertChannelConfig{
			{
				Name:       "ops-webhook",
				Type:       "webhook",
				Enabled:    true,
				WebhookURL: "https://secret.example.com/hook",
			},
			{
				Name:         "ops-email",
				Type:         "email",
				Enabled:      false,
				EmailTo:      "ops@example.com",
				EmailFrom:    "orion@example.com",
				SMTPHost:     "smtp.example.com",
				SMTPPort:     587,
				SMTPUsername: "mailer",
				SMTPPassword: "secret-password",
			},
		},
	})

	delivery := db.AlertDelivery{
		ID:         "alert-delivery-test",
		IncidentID: "incident-test",
		EventType:  "incident_opened",
		Channel:    "ops-webhook",
		Type:       "webhook",
		Status:     "failed",
		Error:      "post https://secret.example.com/hook: connection refused",
	}
	if err := server.db.Create(&delivery).Error; err != nil {
		t.Fatalf("create alert delivery: %v", err)
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
	assertNotContains(t, channelsResp.Body.String(), "secret.example.com")
	assertNotContains(t, channelsResp.Body.String(), "secret-password")

	var channels struct {
		Success bool `json:"success"`
		Data    struct {
			Channels []struct {
				Name               string `json:"name"`
				Type               string `json:"type"`
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
	if !channels.Data.Channels[0].WebhookConfigured || channels.Data.Channels[0].LastDeliveryStatus != "failed" {
		t.Fatalf("webhook channel response = %+v, want redacted webhook with last failed status", channels.Data.Channels[0])
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

	rulesResp := performJSONRequest(t, server, http.MethodGet, "/v1/alerts/rules", nil, "")
	if rulesResp.Code != http.StatusOK {
		t.Fatalf("rules status = %d, body = %s", rulesResp.Code, rulesResp.Body.String())
	}
	assertNotContains(t, rulesResp.Body.String(), "secret.example.com")
	assertNotContains(t, rulesResp.Body.String(), "secret-password")
}

func registerTestAgent(t *testing.T, server *Server) struct {
	Success bool `json:"success"`
	Data    struct {
		AgentID string `json:"agent_id"`
		Token   string `json:"token"`
	} `json:"data"`
} {
	t.Helper()

	registerBody := map[string]string{
		"machine_id": "test-machine-" + t.Name(),
		"name":       "test-server",
		"os":         "linux",
		"arch":       "arm64",
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
