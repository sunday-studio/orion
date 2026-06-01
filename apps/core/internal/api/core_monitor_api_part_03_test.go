package api

import (
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"strings"
	"testing"
	"time"
)

func TestHeartbeatMonitorTokenAndIngestRoutes(t *testing.T) {
	server := setupTestServer(t)
	httpResp := performJSONRequest(t, server, http.MethodPost, "/v1/monitors", map[string]interface{}{"name": "Convertible HTTP monitor", "kind": "http", "config": map[string]interface{}{"url": "https://example.com/health"}}, "")
	if httpResp.Code != http.StatusCreated {
		t.Fatalf("create convertible monitor status = %d, body = %s", httpResp.Code, httpResp.Body.String())
	}
	var convertible struct {
		Data struct {
			Monitor struct {
				ID string `json:"id"`
			} `json:"monitor"`
		} `json:"data"`
	}
	decodeResponse(t, httpResp, &convertible)
	convertResp := performJSONRequest(t, server, http.MethodPatch, "/v1/monitors/"+convertible.Data.Monitor.ID, map[string]interface{}{"kind": "heartbeat", "type": "heartbeat", "config": map[string]interface{}{"grace_seconds": 30}}, "")
	if convertResp.Code != http.StatusOK {
		t.Fatalf("convert heartbeat monitor status = %d, body = %s", convertResp.Code, convertResp.Body.String())
	}
	var converted struct {
		Data struct {
			Config struct {
				HeartbeatToken string `json:"heartbeat_token"`
			} `json:"config"`
		} `json:"data"`
	}
	decodeResponse(t, convertResp, &converted)
	if converted.Data.Config.HeartbeatToken == "" {
		t.Fatalf("converted heartbeat token is empty")
	}
	createResp := performJSONRequest(t, server, http.MethodPost, "/v1/monitors", map[string]interface{}{"name": "Nightly backup heartbeat", "kind": "heartbeat", "interval_seconds": 300, "config": map[string]interface{}{"grace_seconds": 60, "token": "caller-supplied-token-must-not-be-used"}}, "")
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create heartbeat monitor status = %d, body = %s", createResp.Code, createResp.Body.String())
	}
	assertNotContains(t, createResp.Body.String(), "caller-supplied-token-must-not-be-used")
	var created struct {
		Success bool `json:"success"`
		Data    struct {
			Monitor struct {
				ID string `json:"id"`
			} `json:"monitor"`
			Config struct {
				Kind           string                 `json:"kind"`
				Config         map[string]interface{} `json:"config"`
				HeartbeatToken string                 `json:"heartbeat_token"`
			} `json:"config"`
		} `json:"data"`
	}
	decodeResponse(t, createResp, &created)
	if !created.Success || created.Data.Monitor.ID == "" || created.Data.Config.Kind != "heartbeat" {
		t.Fatalf("created heartbeat = %+v, want heartbeat monitor", created)
	}
	token := created.Data.Config.HeartbeatToken
	if token == "" || token == "caller-supplied-token-must-not-be-used" {
		t.Fatalf("heartbeat token = %q, want generated token", token)
	}
	configResp := performJSONRequest(t, server, http.MethodGet, "/v1/monitors/"+created.Data.Monitor.ID+"/config", nil, "")
	if configResp.Code != http.StatusOK {
		t.Fatalf("get heartbeat config status = %d, body = %s", configResp.Code, configResp.Body.String())
	}
	assertNotContains(t, configResp.Body.String(), token)
	successResp := performRawRequest(t, server, http.MethodPost, "/v1/heartbeats/"+token+"/success", strings.NewReader(`{"job":"backup","output":"ok"}`), "")
	if successResp.Code != http.StatusOK {
		t.Fatalf("heartbeat success status = %d, body = %s", successResp.Code, successResp.Body.String())
	}
	var successSignal struct {
		Success bool `json:"success"`
		Data    struct {
			MonitorID        string    `json:"monitor_id"`
			Health           string    `json:"health"`
			ReportID         string    `json:"report_id"`
			ReceivedAt       time.Time `json:"received_at"`
			PayloadTruncated bool      `json:"payload_truncated"`
		} `json:"data"`
	}
	decodeResponse(t, successResp, &successSignal)
	if !successSignal.Success || successSignal.Data.MonitorID != created.Data.Monitor.ID || successSignal.Data.Health != "up" || successSignal.Data.ReportID == "" || successSignal.Data.PayloadTruncated {
		t.Fatalf("success signal = %+v, want up report without truncation", successSignal)
	}
	var config db.CoreMonitorConfig
	if err := server.db.Where("monitor_id = ?", created.Data.Monitor.ID).First(&config).Error; err != nil {
		t.Fatalf("find heartbeat config: %v", err)
	}
	if config.HeartbeatTokenHash == "" || strings.Contains(config.ConfigJSON, token) || config.LastSignalAt == nil || config.LastSuccessAt == nil {
		t.Fatalf("heartbeat config after success = %+v, want token hash and signal/success timestamps", config)
	}
	failurePayload := "password=super-secret token=raw-token-value " + strings.Repeat("x", heartbeatPayloadMaxBytes+200)
	failureResp := performRawRequest(t, server, http.MethodPost, "/v1/heartbeats/"+token+"/failure", strings.NewReader(failurePayload), "")
	if failureResp.Code != http.StatusOK {
		t.Fatalf("heartbeat failure status = %d, body = %s", failureResp.Code, failureResp.Body.String())
	}
	var failureSignal struct {
		Data struct {
			Health           string `json:"health"`
			PayloadTruncated bool   `json:"payload_truncated"`
		} `json:"data"`
	}
	decodeResponse(t, failureResp, &failureSignal)
	if failureSignal.Data.Health != "down" || !failureSignal.Data.PayloadTruncated {
		t.Fatalf("failure signal = %+v, want down truncated signal", failureSignal)
	}
	var failureReport db.MonitorReport
	if err := server.db.Where("monitor_id = ? AND health = ?", created.Data.Monitor.ID, "down").Order("created_at DESC").First(&failureReport).Error; err != nil {
		t.Fatalf("find failure report: %v", err)
	}
	if strings.Contains(failureReport.Payload, strings.Repeat("x", heartbeatPayloadMaxBytes+1)) || !strings.Contains(failureReport.Payload, `"payload_truncated":true`) {
		t.Fatalf("failure payload length = %d body = %.80s, want truncated payload marker", len(failureReport.Payload), failureReport.Payload)
	}
	assertNotContains(t, failureReport.Payload, "super-secret")
	assertNotContains(t, failureReport.Payload, "raw-token-value")
	assertContains(t, failureReport.Payload, "[redacted]")
	historyResp := performJSONRequest(t, server, http.MethodGet, "/v1/monitors/"+created.Data.Monitor.ID+"/history", nil, "")
	if historyResp.Code != http.StatusOK {
		t.Fatalf("heartbeat history status = %d, body = %s", historyResp.Code, historyResp.Body.String())
	}
	assertNotContains(t, historyResp.Body.String(), "super-secret")
	assertNotContains(t, historyResp.Body.String(), "raw-token-value")
	assertContains(t, historyResp.Body.String(), "[redacted]")
	if err := server.db.Where("monitor_id = ?", created.Data.Monitor.ID).First(&config).Error; err != nil {
		t.Fatalf("reload heartbeat config: %v", err)
	}
	if config.LastFailureAt == nil || config.LastSignalAt == nil {
		t.Fatalf("heartbeat config after failure = %+v, want signal/failure timestamps", config)
	}
	var incident db.Incident
	if err := server.db.Where("monitor_id = ?", created.Data.Monitor.ID).First(&incident).Error; err != nil {
		t.Fatalf("find heartbeat incident: %v", err)
	}
	if incident.Status != "open" {
		t.Fatalf("heartbeat incident = %+v, want open incident from failure report", incident)
	}
	incidentResp := performJSONRequest(t, server, http.MethodGet, "/v1/incidents/"+incident.ID, nil, "")
	if incidentResp.Code != http.StatusOK {
		t.Fatalf("heartbeat incident detail status = %d, body = %s", incidentResp.Code, incidentResp.Body.String())
	}
	assertNotContains(t, incidentResp.Body.String(), "super-secret")
	assertNotContains(t, incidentResp.Body.String(), "raw-token-value")
	assertContains(t, incidentResp.Body.String(), "[redacted]")
	assertContains(t, incidentResp.Body.String(), failureReport.ID)
	timelineResp := performJSONRequest(t, server, http.MethodGet, "/v1/incidents/"+incident.ID+"/timeline", nil, "")
	if timelineResp.Code != http.StatusOK {
		t.Fatalf("heartbeat incident timeline status = %d, body = %s", timelineResp.Code, timelineResp.Body.String())
	}
	assertNotContains(t, timelineResp.Body.String(), "super-secret")
	assertNotContains(t, timelineResp.Body.String(), "raw-token-value")
	assertContains(t, timelineResp.Body.String(), "[redacted]")
	assertContains(t, timelineResp.Body.String(), failureReport.ID)
	invalidResp := performRawRequest(t, server, http.MethodPost, "/v1/heartbeats/not-"+token+"/success", strings.NewReader("ok"), "")
	if invalidResp.Code != http.StatusUnauthorized {
		t.Fatalf("invalid heartbeat token status = %d, body = %s", invalidResp.Code, invalidResp.Body.String())
	}
	pauseResp := performJSONRequest(t, server, http.MethodPost, "/v1/monitors/"+created.Data.Monitor.ID+"/pause", nil, "")
	if pauseResp.Code != http.StatusOK {
		t.Fatalf("pause heartbeat status = %d, body = %s", pauseResp.Code, pauseResp.Body.String())
	}
	pausedResp := performRawRequest(t, server, http.MethodPost, "/v1/heartbeats/"+token+"/success", strings.NewReader("ok"), "")
	if pausedResp.Code != http.StatusUnauthorized {
		t.Fatalf("paused heartbeat token status = %d, body = %s", pausedResp.Code, pausedResp.Body.String())
	}
	resumeResp := performJSONRequest(t, server, http.MethodPost, "/v1/monitors/"+created.Data.Monitor.ID+"/resume", nil, "")
	if resumeResp.Code != http.StatusOK {
		t.Fatalf("resume heartbeat status = %d, body = %s", resumeResp.Code, resumeResp.Body.String())
	}
	deleteResp := performJSONRequest(t, server, http.MethodDelete, "/v1/monitors/"+created.Data.Monitor.ID, nil, "")
	if deleteResp.Code != http.StatusOK {
		t.Fatalf("delete heartbeat status = %d, body = %s", deleteResp.Code, deleteResp.Body.String())
	}
	deletedResp := performRawRequest(t, server, http.MethodPost, "/v1/heartbeats/"+token+"/success", strings.NewReader("ok"), "")
	if deletedResp.Code != http.StatusUnauthorized {
		t.Fatalf("deleted heartbeat token status = %d, body = %s", deletedResp.Code, deletedResp.Body.String())
	}
}
func TestCoreMonitorIncidentSeverityOverrideAppearsInResponses(t *testing.T) {
	server := setupTestServer(t)
	monitor := db.Monitor{ID: "monitor-core-severity-override", AgentID: "agent-core-severity-override", Name: "Checkout journey", Type: "synthetic", Lifecycle: "active", Health: "unknown", ComputedHealth: "unknown", ReportingIntervalSeconds: 60, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := server.db.Create(&monitor).Error; err != nil {
		t.Fatalf("create core monitor: %v", err)
	}
	if err := server.db.Create(&db.CoreMonitorConfig{MonitorID: monitor.ID, Kind: "synthetic_multi_step", ConfigJSON: `{"steps":[],"incident_severity":" Critical "}`, SecretRefJSON: `{}`, IntervalSeconds: 60, TimeoutSeconds: 10, NextRunAt: time.Now().UTC(), CreatedAt: time.Now(), UpdatedAt: time.Now()}).Error; err != nil {
		t.Fatalf("create core monitor config: %v", err)
	}
	_, err := server.reportService.StoreMonitorReport(monitor.ID, service.MonitorReportPayload{Timestamp: time.Now().UTC().Format(time.RFC3339), Health: "down", Metrics: map[string]interface{}{"runner": "core", "failure_step": "checkout"}})
	if err != nil {
		t.Fatalf("store core down report: %v", err)
	}
	var incident db.Incident
	if err := server.db.Where("monitor_id = ?", monitor.ID).First(&incident).Error; err != nil {
		t.Fatalf("find core incident: %v", err)
	}
	if incident.Severity != "critical" {
		t.Fatalf("core incident severity = %q, want critical", incident.Severity)
	}
	listResp := performJSONRequest(t, server, http.MethodGet, "/v1/incidents?monitor_id="+monitor.ID, nil, "")
	if listResp.Code != http.StatusOK {
		t.Fatalf("list core incidents status = %d, body = %s", listResp.Code, listResp.Body.String())
	}
	var listed struct {
		Success bool `json:"success"`
		Data    struct {
			Incidents []struct {
				Severity string `json:"severity"`
			} `json:"incidents"`
			Count int64 `json:"count"`
		} `json:"data"`
	}
	decodeResponse(t, listResp, &listed)
	if !listed.Success || listed.Data.Count != 1 || len(listed.Data.Incidents) != 1 || listed.Data.Incidents[0].Severity != "critical" {
		t.Fatalf("listed core incident severity = %+v, want critical", listed)
	}
	detailResp := performJSONRequest(t, server, http.MethodGet, "/v1/incidents/"+incident.ID, nil, "")
	if detailResp.Code != http.StatusOK {
		t.Fatalf("detail core incident status = %d, body = %s", detailResp.Code, detailResp.Body.String())
	}
	var detail struct {
		Success bool `json:"success"`
		Data    struct {
			Incident struct {
				Severity string `json:"severity"`
			} `json:"incident"`
		} `json:"data"`
	}
	decodeResponse(t, detailResp, &detail)
	if !detail.Success || detail.Data.Incident.Severity != "critical" {
		t.Fatalf("detail core incident severity = %+v, want critical", detail)
	}
}
func TestCoreMonitorIncidentSeverityInvalidOverrideFallsBack(t *testing.T) {
	server := setupTestServer(t)
	monitor := db.Monitor{ID: "monitor-core-severity-invalid", AgentID: "agent-core-severity-invalid", Name: "API health", Type: "http", Lifecycle: "active", Health: "unknown", ComputedHealth: "unknown", ReportingIntervalSeconds: 60, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := server.db.Create(&monitor).Error; err != nil {
		t.Fatalf("create core monitor: %v", err)
	}
	if err := server.db.Create(&db.CoreMonitorConfig{MonitorID: monitor.ID, Kind: "http", ConfigJSON: `{"url":"https://api.example.com/health","incident_severity":"urgent"}`, SecretRefJSON: `{}`, IntervalSeconds: 60, TimeoutSeconds: 10, NextRunAt: time.Now().UTC(), CreatedAt: time.Now(), UpdatedAt: time.Now()}).Error; err != nil {
		t.Fatalf("create core monitor config: %v", err)
	}
	_, err := server.reportService.StoreMonitorReport(monitor.ID, service.MonitorReportPayload{Timestamp: time.Now().UTC().Format(time.RFC3339), Health: "down", Metrics: map[string]interface{}{"runner": "core", "status_code": 500}})
	if err != nil {
		t.Fatalf("store core down report: %v", err)
	}
	var incident db.Incident
	if err := server.db.Where("monitor_id = ?", monitor.ID).First(&incident).Error; err != nil {
		t.Fatalf("find core incident: %v", err)
	}
	if incident.Severity != "high" {
		t.Fatalf("core incident severity = %q, want high fallback", incident.Severity)
	}
}
