package api

import (
	"context"
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"strings"
	"testing"
	"time"
)

func TestRegisterReportListFlow(t *testing.T) {
	server := setupTestServer(t)
	registered := registerTestAgent(t, server)
	reportBody := map[string]interface{}{"uptime_seconds": 120, "timestamp": time.Now().UTC().Format(time.RFC3339), "agent_version": "dev-test", "config_summary": map[string]interface{}{"monitor_count": 1, "reporting_interval": "60s", "secret": "do-not-return"}, "cpu": map[string]interface{}{"cores": 4, "usage_percent": 12.5, "load_1": 0.1, "load_5": 0.2, "load_15": 0.3}, "memory": map[string]interface{}{"total_bytes": 1024, "used_bytes": 512, "free_bytes": 512, "available_bytes": 512, "used_percent": 50}, "disk": map[string]interface{}{"total_bytes": 2048, "used_bytes": 1024, "free_bytes": 1024, "used_percent": 50}}
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
func TestAgentTokenLifecycleFlow(t *testing.T) {
	server := setupTestServer(t)
	registered := registerTestAgent(t, server)
	registeredMonitor := registerTestMonitor(t, server, registered.Data.AgentID, registered.Data.Token)
	var storedAgent db.Agent
	if err := server.db.Where("id = ?", registered.Data.AgentID).First(&storedAgent).Error; err != nil {
		t.Fatalf("find stored agent: %v", err)
	}
	if storedAgent.Token == registered.Data.Token || storedAgent.TokenHash == "" || storedAgent.TokenVersion != 1 {
		t.Fatalf("stored agent token lifecycle fields = token:%q hash:%q version:%d", storedAgent.Token, storedAgent.TokenHash, storedAgent.TokenVersion)
	}
	statusResp := performJSONRequest(t, server, http.MethodGet, "/v1/agents/"+registered.Data.AgentID+"/token/status", nil, "")
	if statusResp.Code != http.StatusOK {
		t.Fatalf("token status = %d, body = %s", statusResp.Code, statusResp.Body.String())
	}
	assertNotContains(t, statusResp.Body.String(), registered.Data.Token)
	reissueActiveResp := performJSONRequest(t, server, http.MethodPost, "/v1/agents/"+registered.Data.AgentID+"/token/reissue", nil, "")
	if reissueActiveResp.Code != http.StatusConflict || !strings.Contains(reissueActiveResp.Body.String(), "agent_token_not_revoked") {
		t.Fatalf("active reissue status = %d, body = %s", reissueActiveResp.Code, reissueActiveResp.Body.String())
	}
	rotateResp := performJSONRequest(t, server, http.MethodPost, "/v1/agents/"+registered.Data.AgentID+"/token/rotate", nil, "")
	if rotateResp.Code != http.StatusOK {
		t.Fatalf("rotate status = %d, body = %s", rotateResp.Code, rotateResp.Body.String())
	}
	if rotateResp.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("rotate Cache-Control = %q, want no-store", rotateResp.Header().Get("Cache-Control"))
	}
	var rotated struct {
		Success bool `json:"success"`
		Data    struct {
			Token  string `json:"token"`
			Status struct {
				State        string `json:"state"`
				TokenVersion int    `json:"token_version"`
				TokenExists  bool   `json:"token_exists"`
			} `json:"status"`
		} `json:"data"`
	}
	decodeResponse(t, rotateResp, &rotated)
	if !rotated.Success || rotated.Data.Token == "" || rotated.Data.Token == registered.Data.Token || rotated.Data.Status.State != service.AgentTokenStateActive || rotated.Data.Status.TokenVersion != 2 || !rotated.Data.Status.TokenExists {
		t.Fatalf("rotate response = %+v", rotated)
	}
	oldTokenResp := performJSONRequest(t, server, http.MethodPost, "/v1/agents/"+registered.Data.AgentID+"/report", map[string]interface{}{"uptime_seconds": 300, "timestamp": time.Now().UTC().Format(time.RFC3339)}, registered.Data.Token)
	if oldTokenResp.Code != http.StatusUnauthorized {
		t.Fatalf("old token report status = %d, body = %s", oldTokenResp.Code, oldTokenResp.Body.String())
	}
	newTokenResp := performJSONRequest(t, server, http.MethodPost, "/v1/agents/"+registered.Data.AgentID+"/report", map[string]interface{}{"uptime_seconds": 301, "timestamp": time.Now().UTC().Format(time.RFC3339)}, rotated.Data.Token)
	if newTokenResp.Code != http.StatusOK {
		t.Fatalf("new token report status = %d, body = %s", newTokenResp.Code, newTokenResp.Body.String())
	}
	revokeResp := performJSONRequest(t, server, http.MethodPost, "/v1/agents/"+registered.Data.AgentID+"/token/revoke", map[string]interface{}{"reason": "lost host"}, "")
	if revokeResp.Code != http.StatusOK {
		t.Fatalf("revoke status = %d, body = %s", revokeResp.Code, revokeResp.Body.String())
	}
	var revoked struct {
		Success bool `json:"success"`
		Data    struct {
			State        string `json:"state"`
			TokenVersion int    `json:"token_version"`
			TokenExists  bool   `json:"token_exists"`
		} `json:"data"`
	}
	decodeResponse(t, revokeResp, &revoked)
	if !revoked.Success || revoked.Data.State != service.AgentTokenStateRevoked || revoked.Data.TokenVersion != 3 || revoked.Data.TokenExists {
		t.Fatalf("revoke response = %+v", revoked)
	}
	revokedTokenResp := performJSONRequest(t, server, http.MethodPost, "/v1/agents/"+registered.Data.AgentID+"/report", map[string]interface{}{"uptime_seconds": 302, "timestamp": time.Now().UTC().Format(time.RFC3339)}, rotated.Data.Token)
	if revokedTokenResp.Code != http.StatusUnauthorized || !strings.Contains(revokedTokenResp.Body.String(), "agent_token_revoked") {
		t.Fatalf("revoked token report status = %d, body = %s", revokedTokenResp.Code, revokedTokenResp.Body.String())
	}
	registerAgainResp := performJSONRequest(t, server, http.MethodPost, "/v1/register", map[string]interface{}{"machine_id": "test-machine-" + t.Name(), "name": "test-server", "os": "linux", "arch": "arm64", "reporting_interval_seconds": 60}, "")
	if registerAgainResp.Code != http.StatusConflict || !strings.Contains(registerAgainResp.Body.String(), "agent_token_revoked") {
		t.Fatalf("revoked register status = %d, body = %s", registerAgainResp.Code, registerAgainResp.Body.String())
	}
	assertNotContains(t, registerAgainResp.Body.String(), rotated.Data.Token)
	reissueResp := performJSONRequest(t, server, http.MethodPost, "/v1/agents/"+registered.Data.AgentID+"/token/reissue", nil, "")
	if reissueResp.Code != http.StatusOK {
		t.Fatalf("reissue status = %d, body = %s", reissueResp.Code, reissueResp.Body.String())
	}
	if reissueResp.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("reissue Cache-Control = %q, want no-store", reissueResp.Header().Get("Cache-Control"))
	}
	var reissued struct {
		Success bool `json:"success"`
		Data    struct {
			Token  string `json:"token"`
			Status struct {
				AgentID      string `json:"agent_id"`
				State        string `json:"state"`
				TokenVersion int    `json:"token_version"`
				TokenExists  bool   `json:"token_exists"`
			} `json:"status"`
		} `json:"data"`
	}
	decodeResponse(t, reissueResp, &reissued)
	if !reissued.Success || reissued.Data.Token == "" || reissued.Data.Token == rotated.Data.Token || reissued.Data.Status.AgentID != registered.Data.AgentID || reissued.Data.Status.State != service.AgentTokenStateActive || reissued.Data.Status.TokenVersion != 4 || !reissued.Data.Status.TokenExists {
		t.Fatalf("reissue response = %+v", reissued)
	}
	reissuedTokenResp := performJSONRequest(t, server, http.MethodPost, "/v1/agents/"+registered.Data.AgentID+"/report", map[string]interface{}{"uptime_seconds": 303, "timestamp": time.Now().UTC().Format(time.RFC3339)}, reissued.Data.Token)
	if reissuedTokenResp.Code != http.StatusOK {
		t.Fatalf("reissued token report status = %d, body = %s", reissuedTokenResp.Code, reissuedTokenResp.Body.String())
	}
	var preservedMonitor db.Monitor
	if err := server.db.Where("id = ?", registeredMonitor.Data.MonitorID).First(&preservedMonitor).Error; err != nil {
		t.Fatalf("find preserved monitor: %v", err)
	}
	if preservedMonitor.AgentID != registered.Data.AgentID {
		t.Fatalf("preserved monitor agent id = %q, want %q", preservedMonitor.AgentID, registered.Data.AgentID)
	}
	var auditCount int64
	if err := server.db.Model(&db.AuditEvent{}).Where("affected_object_type = ? AND affected_object_id = ?", "agent", registered.Data.AgentID).Count(&auditCount).Error; err != nil {
		t.Fatalf("count token audit events: %v", err)
	}
	if auditCount != 3 {
		t.Fatalf("token audit event count = %d, want 3", auditCount)
	}
}
func TestAgentServiceLogBatchFlow(t *testing.T) {
	server := setupTestServer(t)
	registered := registerTestAgent(t, server)
	batchPath := "/v1/agents/" + registered.Data.AgentID + "/logs/batch"
	batchResp := performJSONRequest(t, server, http.MethodPost, batchPath, map[string]interface{}{"entries": []map[string]interface{}{{"timestamp": "2026-05-27T20:00:00Z", "source": "agent", "stream": "jsonl", "level": "error", "component": "registration", "message": "registration failed", "fingerprint": "service-log-fp-1", "fields": map[string]interface{}{"token": "do-not-return", "attempt": 1}}, {"timestamp": "2026-05-27T20:00:00Z", "level": "error", "component": "registration", "message": "registration failed", "fingerprint": "service-log-fp-1"}}}, registered.Data.Token)
	if batchResp.Code != http.StatusOK {
		t.Fatalf("service log batch status = %d, body = %s", batchResp.Code, batchResp.Body.String())
	}
	var accepted struct {
		Success bool `json:"success"`
		Data    struct {
			Received int `json:"received"`
			Stored   int `json:"stored"`
		} `json:"data"`
	}
	decodeResponse(t, batchResp, &accepted)
	if !accepted.Success || accepted.Data.Received != 2 || accepted.Data.Stored != 1 {
		t.Fatalf("service log batch response = %+v, want received 2 stored 1", accepted)
	}
	globalResp := performJSONRequest(t, server, http.MethodGet, "/v1/logs/service?level=ERROR&q=registration", nil, "")
	if globalResp.Code != http.StatusOK {
		t.Fatalf("global service logs status = %d, body = %s", globalResp.Code, globalResp.Body.String())
	}
	assertNotContains(t, globalResp.Body.String(), registered.Data.Token)
	assertNotContains(t, globalResp.Body.String(), "do-not-return")
	var listed struct {
		Success bool `json:"success"`
		Data    struct {
			Logs []struct {
				AgentID   string `json:"agent_id"`
				AgentName string `json:"agent_name"`
				Level     string `json:"level"`
				Component string `json:"component"`
				Message   string `json:"message"`
				Fields    string `json:"fields"`
			} `json:"logs"`
			Count int64 `json:"count"`
		} `json:"data"`
	}
	decodeResponse(t, globalResp, &listed)
	if !listed.Success || listed.Data.Count != 1 || len(listed.Data.Logs) != 1 {
		t.Fatalf("service logs response = %+v, want one entry", listed)
	}
	entry := listed.Data.Logs[0]
	if entry.AgentID != registered.Data.AgentID || entry.AgentName != "test-server" || entry.Level != "ERROR" || entry.Component != "registration" || entry.Message != "registration failed" {
		t.Fatalf("service log entry = %+v, want registered agent error log", entry)
	}
	if !strings.Contains(entry.Fields, `"token":"[redacted]"`) {
		t.Fatalf("fields = %q, want redacted token", entry.Fields)
	}
	agentResp := performJSONRequest(t, server, http.MethodGet, "/v1/agents/"+registered.Data.AgentID+"/service-logs?component=registration", nil, "")
	if agentResp.Code != http.StatusOK {
		t.Fatalf("agent service logs status = %d, body = %s", agentResp.Code, agentResp.Body.String())
	}
	decodeResponse(t, agentResp, &listed)
	if listed.Data.Count != 1 || len(listed.Data.Logs) != 1 {
		t.Fatalf("agent service logs response = %+v, want one entry", listed)
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
func TestCoreWorkerDiagnosticsDoNotAffectAPIHealth(t *testing.T) {
	server := setupTestServer(t)
	server.cfg.CoreWorkerStaleSeconds = 30
	emptyResp := performJSONRequest(t, server, http.MethodGet, "/v1/diagnostics/core-worker", nil, "")
	if emptyResp.Code != http.StatusOK {
		t.Fatalf("empty diagnostics status = %d, body = %s", emptyResp.Code, emptyResp.Body.String())
	}
	var diagnostics struct {
		Success bool `json:"success"`
		Data    struct {
			API struct {
				Status  string `json:"status"`
				Service string `json:"service"`
			} `json:"api"`
			Worker struct {
				Status      string `json:"status"`
				WorkerCount int    `json:"worker_count"`
				OnlineCount int    `json:"online_count"`
				StaleCount  int    `json:"stale_count"`
				Workers     []struct {
					WorkerID string `json:"worker_id"`
					Health   string `json:"health"`
				} `json:"workers"`
			} `json:"worker"`
		} `json:"data"`
	}
	decodeResponse(t, emptyResp, &diagnostics)
	if !diagnostics.Success || diagnostics.Data.API.Status != "healthy" || diagnostics.Data.Worker.Status != "unknown" || diagnostics.Data.Worker.WorkerCount != 0 {
		t.Fatalf("empty diagnostics = %+v, want healthy API and unknown worker state", diagnostics)
	}
	now := time.Now().UTC()
	if err := server.workerDiagnosticsService.RecordHeartbeat(context.Background(), service.WorkerHeartbeat{WorkerID: "worker-fresh", Hostname: "core-host", Status: "running", Version: "test", StartedAt: now.Add(-time.Minute), LastHeartbeatAt: now}); err != nil {
		t.Fatalf("record fresh heartbeat: %v", err)
	}
	if err := server.db.Create(&db.CoreWorkerStatus{WorkerID: "worker-stale", ProcessKind: service.CoreMonitorWorkerProcessKind, Hostname: "core-host", Status: "running", Version: "test", StartedAt: now.Add(-time.Hour), LastHeartbeatAt: now.Add(-time.Minute), CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Minute)}).Error; err != nil {
		t.Fatalf("create stale heartbeat: %v", err)
	}
	staleResp := performJSONRequest(t, server, http.MethodGet, "/v1/diagnostics/core-worker", nil, "")
	if staleResp.Code != http.StatusOK {
		t.Fatalf("stale diagnostics status = %d, body = %s", staleResp.Code, staleResp.Body.String())
	}
	decodeResponse(t, staleResp, &diagnostics)
	if diagnostics.Data.Worker.Status != "degraded" || diagnostics.Data.Worker.WorkerCount != 2 || diagnostics.Data.Worker.OnlineCount != 1 || diagnostics.Data.Worker.StaleCount != 1 {
		t.Fatalf("stale diagnostics = %+v, want degraded worker state with one online and one stale", diagnostics.Data.Worker)
	}
	healthResp := performJSONRequest(t, server, http.MethodGet, "/health", nil, "")
	if healthResp.Code != http.StatusOK {
		t.Fatalf("health status = %d, body = %s, want API health to remain independent of worker diagnostics", healthResp.Code, healthResp.Body.String())
	}
}
