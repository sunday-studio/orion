package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/logging"
	"orion/core/internal/service"

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
	if !rotated.Success || rotated.Data.Token == "" || rotated.Data.Token == registered.Data.Token ||
		rotated.Data.Status.State != service.AgentTokenStateActive || rotated.Data.Status.TokenVersion != 2 || !rotated.Data.Status.TokenExists {
		t.Fatalf("rotate response = %+v", rotated)
	}

	oldTokenResp := performJSONRequest(t, server, http.MethodPost, "/v1/agents/"+registered.Data.AgentID+"/report", map[string]interface{}{
		"uptime_seconds": 300,
		"timestamp":      time.Now().UTC().Format(time.RFC3339),
	}, registered.Data.Token)
	if oldTokenResp.Code != http.StatusUnauthorized {
		t.Fatalf("old token report status = %d, body = %s", oldTokenResp.Code, oldTokenResp.Body.String())
	}

	newTokenResp := performJSONRequest(t, server, http.MethodPost, "/v1/agents/"+registered.Data.AgentID+"/report", map[string]interface{}{
		"uptime_seconds": 301,
		"timestamp":      time.Now().UTC().Format(time.RFC3339),
	}, rotated.Data.Token)
	if newTokenResp.Code != http.StatusOK {
		t.Fatalf("new token report status = %d, body = %s", newTokenResp.Code, newTokenResp.Body.String())
	}

	revokeResp := performJSONRequest(t, server, http.MethodPost, "/v1/agents/"+registered.Data.AgentID+"/token/revoke", map[string]interface{}{
		"reason": "lost host",
	}, "")
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

	revokedTokenResp := performJSONRequest(t, server, http.MethodPost, "/v1/agents/"+registered.Data.AgentID+"/report", map[string]interface{}{
		"uptime_seconds": 302,
		"timestamp":      time.Now().UTC().Format(time.RFC3339),
	}, rotated.Data.Token)
	if revokedTokenResp.Code != http.StatusUnauthorized || !strings.Contains(revokedTokenResp.Body.String(), "agent_token_revoked") {
		t.Fatalf("revoked token report status = %d, body = %s", revokedTokenResp.Code, revokedTokenResp.Body.String())
	}

	registerAgainResp := performJSONRequest(t, server, http.MethodPost, "/v1/register", map[string]interface{}{
		"machine_id":                 "test-machine-" + t.Name(),
		"name":                       "test-server",
		"os":                         "linux",
		"arch":                       "arm64",
		"reporting_interval_seconds": 60,
	}, "")
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
	if !reissued.Success || reissued.Data.Token == "" || reissued.Data.Token == rotated.Data.Token ||
		reissued.Data.Status.AgentID != registered.Data.AgentID || reissued.Data.Status.State != service.AgentTokenStateActive ||
		reissued.Data.Status.TokenVersion != 4 || !reissued.Data.Status.TokenExists {
		t.Fatalf("reissue response = %+v", reissued)
	}

	reissuedTokenResp := performJSONRequest(t, server, http.MethodPost, "/v1/agents/"+registered.Data.AgentID+"/report", map[string]interface{}{
		"uptime_seconds": 303,
		"timestamp":      time.Now().UTC().Format(time.RFC3339),
	}, reissued.Data.Token)
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
	batchResp := performJSONRequest(t, server, http.MethodPost, batchPath, map[string]interface{}{
		"entries": []map[string]interface{}{
			{
				"timestamp":   "2026-05-27T20:00:00Z",
				"source":      "agent",
				"stream":      "jsonl",
				"level":       "error",
				"component":   "registration",
				"message":     "registration failed",
				"fingerprint": "service-log-fp-1",
				"fields": map[string]interface{}{
					"token":   "do-not-return",
					"attempt": 1,
				},
			},
			{
				"timestamp":   "2026-05-27T20:00:00Z",
				"level":       "error",
				"component":   "registration",
				"message":     "registration failed",
				"fingerprint": "service-log-fp-1",
			},
		},
	}, registered.Data.Token)
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
	if err := server.workerDiagnosticsService.RecordHeartbeat(context.Background(), service.WorkerHeartbeat{
		WorkerID:        "worker-fresh",
		Hostname:        "core-host",
		Status:          "running",
		Version:         "test",
		StartedAt:       now.Add(-time.Minute),
		LastHeartbeatAt: now,
	}); err != nil {
		t.Fatalf("record fresh heartbeat: %v", err)
	}
	if err := server.db.Create(&db.CoreWorkerStatus{
		WorkerID:        "worker-stale",
		ProcessKind:     service.CoreMonitorWorkerProcessKind,
		Hostname:        "core-host",
		Status:          "running",
		Version:         "test",
		StartedAt:       now.Add(-time.Hour),
		LastHeartbeatAt: now.Add(-time.Minute),
		CreatedAt:       now.Add(-time.Hour),
		UpdatedAt:       now.Add(-time.Minute),
	}).Error; err != nil {
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

type diagnosticLatency struct {
	Count int64 `json:"count"`
	P50   int64 `json:"p50"`
	P95   int64 `json:"p95"`
	P99   int64 `json:"p99"`
	Max   int64 `json:"max"`
}

type diagnosticWrites struct {
	Count      int64 `json:"count"`
	ErrorCount int64 `json:"error_count"`
	P50        int64 `json:"p50"`
	P95        int64 `json:"p95"`
	P99        int64 `json:"p99"`
	Max        int64 `json:"max"`
}

type diagnosticOperation struct {
	Count      int64 `json:"count"`
	ErrorCount int64 `json:"error_count"`
	P50        int64 `json:"p50"`
	P95        int64 `json:"p95"`
	P99        int64 `json:"p99"`
	Max        int64 `json:"max"`
}

type diagnosticLookup struct {
	Count     int64 `json:"count"`
	MissCount int64 `json:"miss_count"`
	P50       int64 `json:"p50"`
	P95       int64 `json:"p95"`
	P99       int64 `json:"p99"`
	Max       int64 `json:"max"`
}

type diagnosticSQLite struct {
	BusyTotal     int64 `json:"busy_total"`
	DatabaseBytes int64 `json:"database_bytes"`
	PageCount     int64 `json:"page_count"`
	PageSizeBytes int64 `json:"page_size_bytes"`
	FreelistCount int64 `json:"freelist_count"`
}

func TestCoreDiagnosticsReportsIngestionMetrics(t *testing.T) {
	server := setupTestServer(t)
	registered := registerTestAgent(t, server)
	registeredMonitor := registerTestMonitor(t, server, registered.Data.AgentID, registered.Data.Token)

	agentReportResp := performJSONRequest(t, server, http.MethodPost, "/v1/agents/"+registered.Data.AgentID+"/report", map[string]interface{}{
		"uptime_seconds": 60,
		"timestamp":      time.Now().UTC().Format(time.RFC3339),
		"cpu":            map[string]interface{}{"usage_percent": 10},
		"memory":         map[string]interface{}{"used_percent": 20},
		"disk":           map[string]interface{}{"used_percent": 30},
	}, registered.Data.Token)
	if agentReportResp.Code != http.StatusOK {
		t.Fatalf("agent report status = %d, body = %s", agentReportResp.Code, agentReportResp.Body.String())
	}

	monitorReportResp := performJSONRequest(t, server, http.MethodPost, "/v1/agents/"+registered.Data.AgentID+"/"+registeredMonitor.Data.MonitorID+"/report", map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"health":    "down",
		"metrics": map[string]interface{}{
			"failure_reason": "diagnostics test failure",
		},
	}, registered.Data.Token)
	if monitorReportResp.Code != http.StatusOK {
		t.Fatalf("monitor report status = %d, body = %s", monitorReportResp.Code, monitorReportResp.Body.String())
	}

	healthResp := performJSONRequest(t, server, http.MethodGet, "/health", nil, "")
	if healthResp.Code != http.StatusOK {
		t.Fatalf("health status = %d, body = %s", healthResp.Code, healthResp.Body.String())
	}

	diagnosticsResp := performJSONRequest(t, server, http.MethodGet, "/v1/diagnostics/core", nil, "")
	if diagnosticsResp.Code != http.StatusOK {
		t.Fatalf("core diagnostics status = %d, body = %s", diagnosticsResp.Code, diagnosticsResp.Body.String())
	}

	var diagnostics struct {
		Success bool `json:"success"`
		Data    struct {
			API struct {
				Status  string `json:"status"`
				Service string `json:"service"`
			} `json:"api"`
			Metrics struct {
				Status         string                       `json:"status"`
				UptimeSeconds  int64                        `json:"uptime_seconds"`
				Requests       map[string]map[string]int64  `json:"requests"`
				Ingestion      map[string]diagnosticLatency `json:"ingestion_latency_ms"`
				ReportWrites   map[string]diagnosticWrites  `json:"report_writes"`
				Reconciliation diagnosticOperation          `json:"incident_reconciliation"`
				Lookup         diagnosticLookup             `json:"active_incident_lookup"`
				SQLite         diagnosticSQLite             `json:"sqlite"`
				SlowOperations []map[string]interface{}     `json:"slow_operations"`
			} `json:"metrics"`
		} `json:"data"`
	}
	decodeResponse(t, diagnosticsResp, &diagnostics)
	if !diagnostics.Success || diagnostics.Data.API.Status != "healthy" || diagnostics.Data.Metrics.Status != "healthy" {
		t.Fatalf("diagnostics status = %+v, want healthy response", diagnostics)
	}
	if diagnostics.Data.Metrics.Requests["/v1/agents/:agent_id/report"]["200"] != 1 {
		t.Fatalf("agent report request counts = %+v, want one 200", diagnostics.Data.Metrics.Requests)
	}
	if diagnostics.Data.Metrics.Requests["/v1/agents/:agent_id/:monitor_id/report"]["200"] != 1 {
		t.Fatalf("monitor report request counts = %+v, want one 200", diagnostics.Data.Metrics.Requests)
	}
	if diagnostics.Data.Metrics.Requests["/health"]["200"] != 1 {
		t.Fatalf("global request counts = %+v, want health request", diagnostics.Data.Metrics.Requests)
	}
	if diagnostics.Data.Metrics.Ingestion["agent"].Count != 1 || diagnostics.Data.Metrics.Ingestion["monitor"].Count != 1 {
		t.Fatalf("ingestion metrics = %+v, want agent and monitor samples", diagnostics.Data.Metrics.Ingestion)
	}
	if diagnostics.Data.Metrics.ReportWrites["agent"].Count != 1 || diagnostics.Data.Metrics.ReportWrites["monitor"].Count != 1 {
		t.Fatalf("report write metrics = %+v, want agent and monitor writes", diagnostics.Data.Metrics.ReportWrites)
	}
	if diagnostics.Data.Metrics.Reconciliation.Count != 2 {
		t.Fatalf("incident reconciliation metrics = %+v, want agent stale and monitor reconciliation", diagnostics.Data.Metrics.Reconciliation)
	}
	if diagnostics.Data.Metrics.Lookup.Count == 0 {
		t.Fatalf("active incident lookup metrics = %+v, want lookup timing", diagnostics.Data.Metrics.Lookup)
	}
	if diagnostics.Data.Metrics.SQLite.DatabaseBytes <= 0 || diagnostics.Data.Metrics.SQLite.PageCount <= 0 {
		t.Fatalf("sqlite diagnostics = %+v, want database size and page count", diagnostics.Data.Metrics.SQLite)
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

func TestManualIncidentActionsAcknowledgeAndResolve(t *testing.T) {
	server := setupTestServer(t)
	registered := registerTestAgent(t, server)
	registeredMonitor := registerTestMonitor(t, server, registered.Data.AgentID, registered.Data.Token)
	reportPath := "/v1/agents/" + registered.Data.AgentID + "/" + registeredMonitor.Data.MonitorID + "/report"

	downResp := performJSONRequest(t, server, http.MethodPost, reportPath, map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"health":    "down",
		"metrics": map[string]interface{}{
			"failure_reason": "manual action test failure",
		},
	}, registered.Data.Token)
	if downResp.Code != http.StatusOK {
		t.Fatalf("down report status = %d, body = %s", downResp.Code, downResp.Body.String())
	}

	var incident db.Incident
	if err := server.db.Where("monitor_id = ?", registeredMonitor.Data.MonitorID).First(&incident).Error; err != nil {
		t.Fatalf("find incident: %v", err)
	}
	incidentID := incident.ID

	coveredUntil := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	coverResp := performJSONRequest(t, server, http.MethodPost, "/v1/incidents/"+incidentID+"/cover", map[string]interface{}{
		"covered_until": coveredUntil.Format(time.RFC3339),
		"note":          "Known deploy window",
	}, "")
	if coverResp.Code != http.StatusOK {
		t.Fatalf("cover incident status = %d, body = %s", coverResp.Code, coverResp.Body.String())
	}
	incident = db.Incident{}
	if err := server.db.Where("id = ?", incidentID).First(&incident).Error; err != nil {
		t.Fatalf("reload covered incident: %v", err)
	}
	if incident.Status != "covered" || incident.CoveredAt == nil || incident.CoveredUntil == nil || incident.CoverageNote != "Known deploy window" || incident.LatestEvent != "Incident marked covered" {
		t.Fatalf("covered incident = %+v, want covered lifecycle fields", incident)
	}
	assertIncidentEvent(t, server, incident.ID, "incident_covered", "Incident marked covered")
	assertMonitorIncidentState(t, server, registeredMonitor.Data.MonitorID, incident.ID, "down")

	reopenCoveredResp := performJSONRequest(t, server, http.MethodPost, "/v1/incidents/"+incident.ID+"/reopen", nil, "")
	if reopenCoveredResp.Code != http.StatusOK {
		t.Fatalf("reopen covered incident status = %d, body = %s", reopenCoveredResp.Code, reopenCoveredResp.Body.String())
	}
	incident = db.Incident{}
	if err := server.db.Where("id = ?", incidentID).First(&incident).Error; err != nil {
		t.Fatalf("reload reopened covered incident: %v", err)
	}
	if incident.Status != "open" || incident.CoveredAt != nil || incident.CoveredUntil != nil || incident.CoverageNote != "" || incident.ReopenedAt == nil || incident.ReopenCount != 1 || incident.LatestEvent != "Incident reopened" {
		t.Fatalf("reopened covered incident = %+v, want open with cleared coverage fields", incident)
	}
	assertIncidentEvent(t, server, incident.ID, "incident_reopened", "Incident reopened")
	assertMonitorIncidentState(t, server, registeredMonitor.Data.MonitorID, incident.ID, "down")

	ackResp := performJSONRequest(t, server, http.MethodPost, "/v1/incidents/"+incident.ID+"/acknowledge", nil, "")
	if ackResp.Code != http.StatusOK {
		t.Fatalf("acknowledge incident status = %d, body = %s", ackResp.Code, ackResp.Body.String())
	}
	incident = db.Incident{}
	if err := server.db.Where("id = ?", incidentID).First(&incident).Error; err != nil {
		t.Fatalf("reload acknowledged incident: %v", err)
	}
	if incident.Status != "acknowledged" || incident.LatestEvent != "Incident manually acknowledged" {
		t.Fatalf("acknowledged incident = %+v, want acknowledged manual latest event", incident)
	}
	assertIncidentEvent(t, server, incident.ID, "incident_acknowledged", "Incident manually acknowledged")
	assertMonitorIncidentState(t, server, registeredMonitor.Data.MonitorID, incident.ID, "down")

	resolveResp := performJSONRequest(t, server, http.MethodPost, "/v1/incidents/"+incident.ID+"/resolve", nil, "")
	if resolveResp.Code != http.StatusOK {
		t.Fatalf("resolve incident status = %d, body = %s", resolveResp.Code, resolveResp.Body.String())
	}
	incident = db.Incident{}
	if err := server.db.Where("id = ?", incidentID).First(&incident).Error; err != nil {
		t.Fatalf("reload resolved incident: %v", err)
	}
	if incident.Status != "resolved" || incident.ResolvedAt == nil || incident.ResolutionKind != "manual" || incident.LatestEvent != "Incident manually resolved" {
		t.Fatalf("resolved incident = %+v, want resolved manual latest event", incident)
	}
	assertIncidentEvent(t, server, incident.ID, "incident_resolved", "Incident manually resolved")
	assertMonitorIncidentState(t, server, registeredMonitor.Data.MonitorID, "", "up")

	reopenResolvedResp := performJSONRequest(t, server, http.MethodPost, "/v1/incidents/"+incident.ID+"/reopen", nil, "")
	if reopenResolvedResp.Code != http.StatusOK {
		t.Fatalf("reopen resolved incident status = %d, body = %s", reopenResolvedResp.Code, reopenResolvedResp.Body.String())
	}
	incident = db.Incident{}
	if err := server.db.Where("id = ?", incidentID).First(&incident).Error; err != nil {
		t.Fatalf("reload reopened resolved incident: %v", err)
	}
	if incident.Status != "open" || incident.ResolvedAt != nil || incident.ResolutionKind != "" || incident.ReopenCount != 2 {
		t.Fatalf("reopened resolved incident = %+v, want open with cleared resolution fields", incident)
	}
	assertMonitorIncidentState(t, server, registeredMonitor.Data.MonitorID, incident.ID, "down")

	resolveAgainResp := performJSONRequest(t, server, http.MethodPost, "/v1/incidents/"+incident.ID+"/resolve", nil, "")
	if resolveAgainResp.Code != http.StatusOK {
		t.Fatalf("resolve reopened incident status = %d, body = %s", resolveAgainResp.Code, resolveAgainResp.Body.String())
	}

	ackResolvedResp := performJSONRequest(t, server, http.MethodPost, "/v1/incidents/"+incident.ID+"/acknowledge", nil, "")
	if ackResolvedResp.Code != http.StatusBadRequest {
		t.Fatalf("acknowledge resolved incident status = %d, want 400, body = %s", ackResolvedResp.Code, ackResolvedResp.Body.String())
	}

	coverResolvedResp := performJSONRequest(t, server, http.MethodPost, "/v1/incidents/"+incident.ID+"/cover", map[string]interface{}{}, "")
	if coverResolvedResp.Code != http.StatusBadRequest {
		t.Fatalf("cover resolved incident status = %d, want 400, body = %s", coverResolvedResp.Code, coverResolvedResp.Body.String())
	}
}

func TestCoveredIncidentSuppressesFailuresUntilRecoveryOrExpiry(t *testing.T) {
	server := setupTestServer(t)
	startedAt := time.Date(2026, 5, 28, 3, 0, 0, 0, time.UTC)
	coveredMonitor := seedCoreNoiseMonitor(t, server, "monitor-core-covered-suppression", 0, 0, 0)

	storeCoreConfirmationReport(t, server, coveredMonitor.ID, "down", startedAt)
	var incident db.Incident
	if err := server.db.Where("monitor_id = ?", coveredMonitor.ID).First(&incident).Error; err != nil {
		t.Fatalf("find covered suppression incident: %v", err)
	}

	coveredUntil := time.Now().UTC().Add(time.Hour).Truncate(time.Second)
	coverResp := performJSONRequest(t, server, http.MethodPost, "/v1/incidents/"+incident.ID+"/cover", map[string]interface{}{
		"covered_until": coveredUntil.Format(time.RFC3339),
		"note":          "Known upstream outage",
	}, "")
	if coverResp.Code != http.StatusOK {
		t.Fatalf("cover incident status = %d, body = %s", coverResp.Code, coverResp.Body.String())
	}

	storeCoreConfirmationReport(t, server, coveredMonitor.ID, "down", startedAt.Add(time.Minute))
	assertCoreIncidentCount(t, server, coveredMonitor.ID, 1)
	incident = db.Incident{}
	if err := server.db.Where("monitor_id = ?", coveredMonitor.ID).First(&incident).Error; err != nil {
		t.Fatalf("reload suppressed incident: %v", err)
	}
	if incident.Status != "covered" || incident.CoveredAt == nil || incident.CoveredUntil == nil || incident.CoverageNote != "Known upstream outage" || incident.LatestEvent != "Incident coverage suppressed failing monitor report" {
		t.Fatalf("suppressed incident = %+v, want covered with suppression event", incident)
	}
	assertMonitorIncidentState(t, server, coveredMonitor.ID, incident.ID, "down")
	assertIncidentEventCount(t, server, incident.ID, "incident_coverage_suppressed", 1)
	assertIncidentEventCount(t, server, incident.ID, "monitor_failed", 0)

	storeCoreConfirmationReport(t, server, coveredMonitor.ID, "up", startedAt.Add(2*time.Minute))
	incident = db.Incident{}
	if err := server.db.Where("monitor_id = ?", coveredMonitor.ID).First(&incident).Error; err != nil {
		t.Fatalf("reload recovered covered incident: %v", err)
	}
	if incident.Status != "resolved" || incident.ResolvedAt == nil || incident.CoveredAt != nil || incident.CoveredUntil != nil || incident.CoverageNote != "" || incident.ResolutionKind != "recovered" {
		t.Fatalf("recovered covered incident = %+v, want resolved with cleared coverage", incident)
	}
	assertMonitorIncidentState(t, server, coveredMonitor.ID, "", "up")
	assertIncidentEvent(t, server, incident.ID, "incident_resolved", "Monitor "+coveredMonitor.Name+" recovered")

	expiringMonitor := seedCoreNoiseMonitor(t, server, "monitor-core-covered-expiry", 0, 0, 0)
	storeCoreConfirmationReport(t, server, expiringMonitor.ID, "down", startedAt)
	var expiringIncident db.Incident
	if err := server.db.Where("monitor_id = ?", expiringMonitor.ID).First(&expiringIncident).Error; err != nil {
		t.Fatalf("find expiring coverage incident: %v", err)
	}

	expiredUntil := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	expiredCoverResp := performJSONRequest(t, server, http.MethodPost, "/v1/incidents/"+expiringIncident.ID+"/cover", map[string]interface{}{
		"covered_until": expiredUntil.Format(time.RFC3339),
		"note":          "Expired maintenance cover",
	}, "")
	if expiredCoverResp.Code != http.StatusOK {
		t.Fatalf("cover expiring incident status = %d, body = %s", expiredCoverResp.Code, expiredCoverResp.Body.String())
	}

	storeCoreConfirmationReport(t, server, expiringMonitor.ID, "down", startedAt.Add(time.Minute))
	assertCoreIncidentCount(t, server, expiringMonitor.ID, 1)
	expiringIncident = db.Incident{}
	if err := server.db.Where("monitor_id = ?", expiringMonitor.ID).First(&expiringIncident).Error; err != nil {
		t.Fatalf("reload expired coverage incident: %v", err)
	}
	if expiringIncident.Status != "open" || expiringIncident.CoveredAt != nil || expiringIncident.CoveredUntil != nil || expiringIncident.CoverageNote != "" || expiringIncident.LatestEvent == "Incident coverage expired" {
		t.Fatalf("expired coverage incident = %+v, want reopened failure with cleared coverage", expiringIncident)
	}
	assertMonitorIncidentState(t, server, expiringMonitor.ID, expiringIncident.ID, "down")
	assertIncidentEventCount(t, server, expiringIncident.ID, "incident_coverage_expired", 1)
	assertIncidentEventCount(t, server, expiringIncident.ID, "monitor_failed", 1)
}

func TestUnregisterMonitorResolvesActiveIncidentPath(t *testing.T) {
	server := setupTestServer(t)
	registered := registerTestAgent(t, server)
	registeredMonitor := registerTestMonitor(t, server, registered.Data.AgentID, registered.Data.Token)

	downResp := performJSONRequest(t, server, http.MethodPost, "/v1/agents/"+registered.Data.AgentID+"/"+registeredMonitor.Data.MonitorID+"/report", map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"health":    "down",
		"metrics": map[string]interface{}{
			"failure_reason": "removed monitor failure",
		},
	}, registered.Data.Token)
	if downResp.Code != http.StatusOK {
		t.Fatalf("down report status = %d, body = %s", downResp.Code, downResp.Body.String())
	}

	var incident db.Incident
	if err := server.db.Where("monitor_id = ?", registeredMonitor.Data.MonitorID).First(&incident).Error; err != nil {
		t.Fatalf("find incident: %v", err)
	}
	assertMonitorIncidentState(t, server, registeredMonitor.Data.MonitorID, incident.ID, "down")

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

	if err := server.db.Where("id = ?", incident.ID).First(&incident).Error; err != nil {
		t.Fatalf("reload incident: %v", err)
	}
	if incident.Status != "resolved" || incident.ResolvedAt == nil || incident.LatestEvent != "Monitor removed; active incident resolved" {
		t.Fatalf("incident after monitor unregister = %+v, want resolved monitor removed event", incident)
	}
	assertIncidentEvent(t, server, incident.ID, "incident_resolved", "Monitor removed; active incident resolved")
	assertMonitorIncidentState(t, server, registeredMonitor.Data.MonitorID, "", "unknown")
}

func TestUnregisterMonitorClearsStaleIncidentPathWithoutDuplicateEvent(t *testing.T) {
	server := setupTestServer(t)
	registered := registerTestAgent(t, server)
	registeredMonitor := registerTestMonitor(t, server, registered.Data.AgentID, registered.Data.Token)
	now := time.Now().UTC()
	incident := db.Incident{
		ID:                 "incident-stale-active-path",
		Status:             "resolved",
		Severity:           "high",
		Title:              "Resolved stale active path",
		AgentID:            registered.Data.AgentID,
		MonitorID:          registeredMonitor.Data.MonitorID,
		OpenedAt:           now.Add(-30 * time.Minute),
		ResolvedAt:         &now,
		LastEventAt:        now,
		LatestEvent:        "Already resolved",
		NotificationStatus: "suppressed",
	}
	if err := server.db.Create(&incident).Error; err != nil {
		t.Fatalf("create resolved incident: %v", err)
	}
	if err := server.db.Model(&db.Monitor{}).Where("id = ?", registeredMonitor.Data.MonitorID).Updates(map[string]interface{}{
		"active_incident_id": incident.ID,
		"incident_state":     "down",
	}).Error; err != nil {
		t.Fatalf("set stale monitor incident path: %v", err)
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

	assertMonitorIncidentState(t, server, registeredMonitor.Data.MonitorID, "", "unknown")

	var resolvedEventCount int64
	if err := server.db.Model(&db.IncidentEvent{}).
		Where("incident_id = ? AND type = ?", incident.ID, "incident_resolved").
		Count(&resolvedEventCount).Error; err != nil {
		t.Fatalf("count resolved events: %v", err)
	}
	if resolvedEventCount != 0 {
		t.Fatalf("resolved event count = %d, want 0 duplicate events", resolvedEventCount)
	}
}

func TestCoreWorkerReportsOpenAndResolveIncident(t *testing.T) {
	server := setupTestServer(t)
	monitor := db.Monitor{
		ID:                       "monitor-core-worker-report",
		AgentID:                  "agent-core-worker",
		Name:                     "Core API health",
		Type:                     "http",
		Lifecycle:                "active",
		Health:                   "unknown",
		ComputedHealth:           "unknown",
		ReportingIntervalSeconds: 30,
		CreatedAt:                time.Now(),
		UpdatedAt:                time.Now(),
	}
	if err := server.db.Create(&monitor).Error; err != nil {
		t.Fatalf("create core monitor: %v", err)
	}

	downReportID, err := server.reportService.StoreMonitorReport(monitor.ID, service.MonitorReportPayload{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Health:    "down",
		Metrics: map[string]interface{}{
			"runner":       "core",
			"status_code":  500,
			"duration_ms":  125,
			"failure_step": "http",
		},
	})
	if err != nil {
		t.Fatalf("store core down report: %v", err)
	}
	if downReportID == nil || *downReportID == "" {
		t.Fatalf("down report id = %v, want generated id", downReportID)
	}

	var coreOwner db.Agent
	if err := server.db.Where("id = ?", "agent-core-worker").First(&coreOwner).Error; err != nil {
		t.Fatalf("find generated core owner: %v", err)
	}
	if coreOwner.MachineId != "core" || coreOwner.Name != "Orion Core" {
		t.Fatalf("core owner = %+v, want Orion Core owner row", coreOwner)
	}

	var incident db.Incident
	if err := server.db.Where("monitor_id = ?", monitor.ID).First(&incident).Error; err != nil {
		t.Fatalf("find core incident: %v", err)
	}
	if incident.Status != "open" || incident.Severity != "high" || incident.AgentID != coreOwner.ID {
		t.Fatalf("core incident = %+v, want open high incident owned by core", incident)
	}
	if incident.Title != "Core monitor down: Core API health" {
		t.Fatalf("core incident title = %q, want Core monitor down title", incident.Title)
	}
	assertAlertDelivery(t, server, incident.ID, "incident_opened", "suppressed")
	assertMonitorIncidentState(t, server, monitor.ID, incident.ID, "down")

	var storedMonitor db.Monitor
	if err := server.db.Where("id = ?", monitor.ID).First(&storedMonitor).Error; err != nil {
		t.Fatalf("reload core monitor: %v", err)
	}
	if storedMonitor.Health != "down" || storedMonitor.ComputedHealth != "down" {
		t.Fatalf("core monitor health = stored %q computed %q, want down/down", storedMonitor.Health, storedMonitor.ComputedHealth)
	}

	listResp := performJSONRequest(t, server, http.MethodGet, "/v1/incidents?monitor_id="+monitor.ID, nil, "")
	if listResp.Code != http.StatusOK {
		t.Fatalf("list core incidents status = %d, body = %s", listResp.Code, listResp.Body.String())
	}
	var listed struct {
		Success bool `json:"success"`
		Data    struct {
			Incidents []struct {
				AgentName   string `json:"agent_name"`
				MonitorName string `json:"monitor_name"`
				Severity    string `json:"severity"`
			} `json:"incidents"`
			Count int64 `json:"count"`
		} `json:"data"`
	}
	decodeResponse(t, listResp, &listed)
	if !listed.Success || listed.Data.Count != 1 || listed.Data.Incidents[0].AgentName != "Orion Core" || listed.Data.Incidents[0].MonitorName != "Core API health" || listed.Data.Incidents[0].Severity != "high" {
		t.Fatalf("listed core incidents = %+v, want Orion Core incident", listed)
	}

	upReportID, err := server.reportService.StoreMonitorReport(monitor.ID, service.MonitorReportPayload{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Health:    "up",
		Metrics: map[string]interface{}{
			"runner":      "core",
			"status_code": 200,
			"duration_ms": 48,
		},
	})
	if err != nil {
		t.Fatalf("store core up report: %v", err)
	}
	if upReportID == nil || *upReportID == "" {
		t.Fatalf("up report id = %v, want generated id", upReportID)
	}

	if err := server.db.Where("id = ?", incident.ID).First(&incident).Error; err != nil {
		t.Fatalf("reload core incident: %v", err)
	}
	if incident.Status != "resolved" || incident.ResolvedAt == nil {
		t.Fatalf("core incident = %+v, want resolved with resolved_at", incident)
	}
	assertAlertDelivery(t, server, incident.ID, "incident_resolved", "suppressed")
	assertMonitorIncidentState(t, server, monitor.ID, "", "up")

	if err := server.db.Where("id = ?", monitor.ID).First(&storedMonitor).Error; err != nil {
		t.Fatalf("reload recovered core monitor: %v", err)
	}
	if storedMonitor.Health != "up" || storedMonitor.LastSuccessfulReportAt == nil {
		t.Fatalf("recovered core monitor = %+v, want up with last_successful_report_at", storedMonitor)
	}
}

func TestCoreMonitorConfirmationPeriodDefersTransientFailure(t *testing.T) {
	server := setupTestServer(t)
	monitor := seedCoreConfirmationMonitor(t, server, "monitor-core-confirm-transient", 60, 0)
	startedAt := time.Date(2026, 5, 28, 1, 0, 0, 0, time.UTC)

	storeCoreConfirmationReport(t, server, monitor.ID, "down", startedAt)
	assertCoreIncidentCount(t, server, monitor.ID, 0)
	assertMonitorIncidentState(t, server, monitor.ID, "", "down")

	storeCoreConfirmationReport(t, server, monitor.ID, "up", startedAt.Add(30*time.Second))
	assertCoreIncidentCount(t, server, monitor.ID, 0)
	assertMonitorIncidentState(t, server, monitor.ID, "", "up")
}

func TestCoreMonitorConfirmationPeriodOpensSustainedFailure(t *testing.T) {
	server := setupTestServer(t)
	monitor := seedCoreConfirmationMonitor(t, server, "monitor-core-confirm-sustained", 60, 0)
	startedAt := time.Date(2026, 5, 28, 1, 10, 0, 0, time.UTC)

	storeCoreConfirmationReport(t, server, monitor.ID, "down", startedAt)
	assertCoreIncidentCount(t, server, monitor.ID, 0)

	storeCoreConfirmationReport(t, server, monitor.ID, "down", startedAt.Add(61*time.Second))

	var incident db.Incident
	if err := server.db.Where("monitor_id = ?", monitor.ID).First(&incident).Error; err != nil {
		t.Fatalf("find sustained core incident: %v", err)
	}
	if incident.Status != "open" {
		t.Fatalf("sustained incident = %+v, want open", incident)
	}
	assertMonitorIncidentState(t, server, monitor.ID, incident.ID, "down")
}

func TestCoreMonitorConfirmationCheckCountOpensAfterConsecutiveFailures(t *testing.T) {
	server := setupTestServer(t)
	monitor := seedCoreConfirmationMonitor(t, server, "monitor-core-confirm-count", 0, 2)
	startedAt := time.Date(2026, 5, 28, 1, 20, 0, 0, time.UTC)

	storeCoreConfirmationReport(t, server, monitor.ID, "down", startedAt)
	assertCoreIncidentCount(t, server, monitor.ID, 0)

	storeCoreConfirmationReport(t, server, monitor.ID, "down", startedAt.Add(10*time.Second))

	var incident db.Incident
	if err := server.db.Where("monitor_id = ?", monitor.ID).First(&incident).Error; err != nil {
		t.Fatalf("find count-confirmed core incident: %v", err)
	}
	if incident.Status != "open" {
		t.Fatalf("count-confirmed incident = %+v, want open", incident)
	}
	assertMonitorIncidentState(t, server, monitor.ID, incident.ID, "down")
}

func TestCoreMonitorRecoveryPeriodDefersBlipsAndResolvesSustainedRecovery(t *testing.T) {
	server := setupTestServer(t)
	monitor := seedCoreNoiseMonitor(t, server, "monitor-core-recovery-period", 0, 0, 60)
	startedAt := time.Date(2026, 5, 28, 1, 30, 0, 0, time.UTC)

	storeCoreConfirmationReport(t, server, monitor.ID, "down", startedAt)
	var incident db.Incident
	if err := server.db.Where("monitor_id = ?", monitor.ID).First(&incident).Error; err != nil {
		t.Fatalf("find recovery incident: %v", err)
	}
	if incident.Status != "open" {
		t.Fatalf("recovery incident = %+v, want open", incident)
	}
	assertMonitorIncidentState(t, server, monitor.ID, incident.ID, "down")

	storeCoreConfirmationReport(t, server, monitor.ID, "up", startedAt.Add(30*time.Second))
	if err := server.db.Where("id = ?", incident.ID).First(&incident).Error; err != nil {
		t.Fatalf("reload recovering incident: %v", err)
	}
	if incident.Status != "open" || incident.ResolvedAt != nil {
		t.Fatalf("short recovery incident = %+v, want still open", incident)
	}
	assertMonitorIncidentState(t, server, monitor.ID, incident.ID, "recovering")
	assertIncidentEventCount(t, server, incident.ID, "incident_resolved", 0)

	storeCoreConfirmationReport(t, server, monitor.ID, "down", startedAt.Add(40*time.Second))
	if err := server.db.Where("id = ?", incident.ID).First(&incident).Error; err != nil {
		t.Fatalf("reload relapse incident: %v", err)
	}
	if incident.Status != "open" || incident.ResolvedAt != nil {
		t.Fatalf("relapse incident = %+v, want still open", incident)
	}
	assertMonitorIncidentState(t, server, monitor.ID, incident.ID, "down")

	storeCoreConfirmationReport(t, server, monitor.ID, "up", startedAt.Add(50*time.Second))
	assertMonitorIncidentState(t, server, monitor.ID, incident.ID, "recovering")

	storeCoreConfirmationReport(t, server, monitor.ID, "up", startedAt.Add(111*time.Second))
	if err := server.db.Where("id = ?", incident.ID).First(&incident).Error; err != nil {
		t.Fatalf("reload resolved recovery incident: %v", err)
	}
	if incident.Status != "resolved" || incident.ResolvedAt == nil {
		t.Fatalf("sustained recovery incident = %+v, want resolved", incident)
	}
	assertMonitorIncidentState(t, server, monitor.ID, "", "up")
}

func TestCoreMonitorMaintenanceWindowsSuppressIncidents(t *testing.T) {
	server := setupTestServer(t)
	windowStart := time.Date(2026, 5, 28, 2, 0, 0, 0, time.UTC)
	windowEnd := windowStart.Add(30 * time.Minute)

	monitor := seedCoreMaintenanceMonitor(t, server, "monitor-core-maintenance-window", windowStart, windowEnd)
	storeCoreConfirmationReport(t, server, monitor.ID, "down", windowStart.Add(-time.Minute))
	assertCoreIncidentCount(t, server, monitor.ID, 1)

	var incident db.Incident
	if err := server.db.Where("monitor_id = ?", monitor.ID).First(&incident).Error; err != nil {
		t.Fatalf("find maintenance incident: %v", err)
	}
	assertMonitorIncidentState(t, server, monitor.ID, incident.ID, "down")
	var deliveryCountBeforeMaintenance int64
	if err := server.db.Model(&db.AlertDelivery{}).Joins("JOIN incidents ON incidents.id = alert_deliveries.incident_id").Where("incidents.monitor_id = ?", monitor.ID).Count(&deliveryCountBeforeMaintenance).Error; err != nil {
		t.Fatalf("count pre-maintenance alert deliveries: %v", err)
	}

	storeCoreConfirmationReport(t, server, monitor.ID, "up", windowStart.Add(time.Minute))
	assertCoreIncidentCount(t, server, monitor.ID, 1)
	if err := server.db.Where("id = ?", incident.ID).First(&incident).Error; err != nil {
		t.Fatalf("reload maintenance incident: %v", err)
	}
	if incident.Status != "open" || incident.ResolvedAt != nil {
		t.Fatalf("maintenance recovery incident = %+v, want still open", incident)
	}
	assertMonitorIncidentState(t, server, monitor.ID, incident.ID, "maintenance")
	assertIncidentEventCount(t, server, incident.ID, "incident_resolved", 0)

	storeCoreConfirmationReport(t, server, monitor.ID, "down", windowStart.Add(2*time.Minute))
	assertCoreIncidentCount(t, server, monitor.ID, 1)
	assertMonitorIncidentState(t, server, monitor.ID, incident.ID, "maintenance")
	var activeReportCount int64
	if err := server.db.Model(&db.MonitorReport{}).Where("monitor_id = ?", monitor.ID).Count(&activeReportCount).Error; err != nil {
		t.Fatalf("count maintenance reports: %v", err)
	}
	if activeReportCount != 3 {
		t.Fatalf("maintenance report count = %d, want preserved report", activeReportCount)
	}
	var activeDeliveryCount int64
	if err := server.db.Model(&db.AlertDelivery{}).Joins("JOIN incidents ON incidents.id = alert_deliveries.incident_id").Where("incidents.monitor_id = ?", monitor.ID).Count(&activeDeliveryCount).Error; err != nil {
		t.Fatalf("count maintenance alert deliveries: %v", err)
	}
	if activeDeliveryCount != deliveryCountBeforeMaintenance {
		t.Fatalf("maintenance alert delivery count = %d, want unchanged %d", activeDeliveryCount, deliveryCountBeforeMaintenance)
	}

	storeCoreConfirmationReport(t, server, monitor.ID, "up", windowEnd.Add(time.Second))
	if err := server.db.Where("id = ?", incident.ID).First(&incident).Error; err != nil {
		t.Fatalf("reload resolved maintenance incident: %v", err)
	}
	if incident.Status != "resolved" || incident.ResolvedAt == nil {
		t.Fatalf("exited maintenance incident = %+v, want resolved", incident)
	}
	assertMonitorIncidentState(t, server, monitor.ID, "", "up")

	storeCoreConfirmationReport(t, server, monitor.ID, "down", windowEnd.Add(2*time.Second))
	assertCoreIncidentCount(t, server, monitor.ID, 2)
}

func TestCoreMonitorManagementLifecycle(t *testing.T) {
	server := setupTestServerWithConfig(t, &config.Config{
		AlertRecoveryNotifications:     true,
		AlertTLSExpiryDays:             14,
		CoreMonitorAllowPrivateTargets: true,
	})
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("ready"))
	}))
	defer target.Close()

	createResp := performJSONRequest(t, server, http.MethodPost, "/v1/monitors", map[string]interface{}{
		"name":                        "Core API probe",
		"description":                 "managed by core",
		"kind":                        "http",
		"interval_seconds":            30,
		"timeout_seconds":             2,
		"confirmation_period_seconds": 60,
		"confirmation_check_count":    3,
		"recovery_period_seconds":     45,
		"config": map[string]interface{}{
			"url":               target.URL + "/health",
			"expected_status":   http.StatusAccepted,
			"authorization":     "Bearer should-not-leak",
			"required_contains": []string{"ready"},
		},
		"secret_refs": map[string]interface{}{
			"header": "secret-ref",
		},
	}, "")
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create core monitor status = %d, body = %s", createResp.Code, createResp.Body.String())
	}
	assertNotContains(t, createResp.Body.String(), "should-not-leak")
	assertNotContains(t, createResp.Body.String(), "secret-ref")

	var created struct {
		Success bool `json:"success"`
		Data    struct {
			Monitor struct {
				ID        string `json:"id"`
				OwnerKind string `json:"owner_kind"`
				Source    string `json:"source"`
			} `json:"monitor"`
			Config struct {
				Kind                      string                 `json:"kind"`
				ConfirmationPeriodSeconds int                    `json:"confirmation_period_seconds"`
				ConfirmationCheckCount    int                    `json:"confirmation_check_count"`
				RecoveryPeriodSeconds     int                    `json:"recovery_period_seconds"`
				Config                    map[string]interface{} `json:"config"`
				SecretRefs                map[string]interface{} `json:"secret_refs"`
			} `json:"config"`
		} `json:"data"`
	}
	decodeResponse(t, createResp, &created)
	if !created.Success || created.Data.Monitor.ID == "" {
		t.Fatalf("create response = %+v, want monitor id", created)
	}
	monitorID := created.Data.Monitor.ID
	if created.Data.Monitor.OwnerKind != "core" || created.Data.Monitor.Source != "core" {
		t.Fatalf("owner fields = %+v, want core/core", created.Data.Monitor)
	}
	if created.Data.Config.Kind != "http" || created.Data.Config.Config["authorization"] != "[redacted]" || created.Data.Config.SecretRefs["header"] != "[redacted]" {
		t.Fatalf("redacted config = %+v, secret refs = %+v", created.Data.Config.Config, created.Data.Config.SecretRefs)
	}
	if created.Data.Config.ConfirmationPeriodSeconds != 60 || created.Data.Config.ConfirmationCheckCount != 3 || created.Data.Config.RecoveryPeriodSeconds != 45 {
		t.Fatalf("noise config = %+v, want 60s confirmation, 3 checks, and 45s recovery", created.Data.Config)
	}

	configResp := performJSONRequest(t, server, http.MethodGet, "/v1/monitors/"+monitorID+"/config", nil, "")
	if configResp.Code != http.StatusOK {
		t.Fatalf("get config status = %d, body = %s", configResp.Code, configResp.Body.String())
	}
	assertNotContains(t, configResp.Body.String(), "should-not-leak")

	updateResp := performJSONRequest(t, server, http.MethodPatch, "/v1/monitors/"+monitorID, map[string]interface{}{
		"name":                        "Core API probe updated",
		"interval_seconds":            45,
		"confirmation_period_seconds": 30,
		"confirmation_check_count":    2,
		"recovery_period_seconds":     20,
		"config": map[string]interface{}{
			"url":             target.URL + "/health",
			"expected_status": http.StatusAccepted,
			"token":           "patch-secret",
		},
	}, "")
	if updateResp.Code != http.StatusOK {
		t.Fatalf("update status = %d, body = %s", updateResp.Code, updateResp.Body.String())
	}
	assertNotContains(t, updateResp.Body.String(), "patch-secret")
	var updated db.Monitor
	if err := server.db.Where("id = ?", monitorID).First(&updated).Error; err != nil {
		t.Fatalf("find updated monitor: %v", err)
	}
	if updated.Name != "Core API probe updated" || updated.ReportingIntervalSeconds != 45 {
		t.Fatalf("updated monitor = %+v, want updated name and interval", updated)
	}
	var updatedConfig db.CoreMonitorConfig
	if err := server.db.Where("monitor_id = ?", monitorID).First(&updatedConfig).Error; err != nil {
		t.Fatalf("find updated core monitor config: %v", err)
	}
	if updatedConfig.ConfirmationPeriodSeconds != 30 || updatedConfig.ConfirmationCheckCount != 2 || updatedConfig.RecoveryPeriodSeconds != 20 {
		t.Fatalf("updated noise config = %+v, want 30s confirmation, 2 checks, and 20s recovery", updatedConfig)
	}

	pauseResp := performJSONRequest(t, server, http.MethodPost, "/v1/monitors/"+monitorID+"/pause", nil, "")
	if pauseResp.Code != http.StatusOK {
		t.Fatalf("pause status = %d, body = %s", pauseResp.Code, pauseResp.Body.String())
	}
	var paused db.CoreMonitorConfig
	if err := server.db.Where("monitor_id = ?", monitorID).First(&paused).Error; err != nil {
		t.Fatalf("find paused core monitor config: %v", err)
	}
	if !paused.Paused {
		t.Fatalf("paused = false, want true")
	}

	resumeResp := performJSONRequest(t, server, http.MethodPost, "/v1/monitors/"+monitorID+"/resume", nil, "")
	if resumeResp.Code != http.StatusOK {
		t.Fatalf("resume status = %d, body = %s", resumeResp.Code, resumeResp.Body.String())
	}
	var resumed db.CoreMonitorConfig
	if err := server.db.Where("monitor_id = ?", monitorID).First(&resumed).Error; err != nil {
		t.Fatalf("find resumed core monitor config: %v", err)
	}
	if resumed.Paused || resumed.NextRunAt.After(time.Now().UTC().Add(2*time.Second)) {
		t.Fatalf("resumed config = %+v, want unpaused and due", resumed)
	}

	testResp := performJSONRequest(t, server, http.MethodPost, "/v1/monitors/"+monitorID+"/test", nil, "")
	if testResp.Code != http.StatusOK {
		t.Fatalf("test-now status = %d, body = %s", testResp.Code, testResp.Body.String())
	}
	var reportCount int64
	if err := server.db.Model(&db.MonitorReport{}).Where("monitor_id = ?", monitorID).Count(&reportCount).Error; err != nil {
		t.Fatalf("count test-now reports: %v", err)
	}
	if reportCount != 1 {
		t.Fatalf("test-now report count = %d, want 1", reportCount)
	}
	var tested db.CoreMonitorConfig
	if err := server.db.Where("monitor_id = ?", monitorID).First(&tested).Error; err != nil {
		t.Fatalf("find tested core monitor config: %v", err)
	}
	if tested.LastRunAt == nil || tested.LastSuccessAt == nil {
		t.Fatalf("tested config = %+v, want last run and success", tested)
	}

	deleteResp := performJSONRequest(t, server, http.MethodDelete, "/v1/monitors/"+monitorID, nil, "")
	if deleteResp.Code != http.StatusOK {
		t.Fatalf("delete status = %d, body = %s", deleteResp.Code, deleteResp.Body.String())
	}
	var deleted db.Monitor
	if err := server.db.Where("id = ?", monitorID).First(&deleted).Error; err != nil {
		t.Fatalf("find deleted monitor: %v", err)
	}
	if deleted.Lifecycle != "deleted" {
		t.Fatalf("deleted lifecycle = %q, want deleted", deleted.Lifecycle)
	}
}

func TestCoreMonitorManagementRejectsUnsupportedKind(t *testing.T) {
	server := setupTestServer(t)
	resp := performJSONRequest(t, server, http.MethodPost, "/v1/monitors", map[string]interface{}{
		"name":   "Unsupported",
		"kind":   "coffee",
		"config": map[string]interface{}{},
	}, "")
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("unsupported kind status = %d, body = %s", resp.Code, resp.Body.String())
	}
}

func TestCoreMonitorManagementRejectsInvalidConfig(t *testing.T) {
	server := setupTestServer(t)

	createResp := performJSONRequest(t, server, http.MethodPost, "/v1/monitors", map[string]interface{}{
		"name":   "Missing URL",
		"kind":   "http",
		"config": map[string]interface{}{},
	}, "")
	if createResp.Code != http.StatusBadRequest {
		t.Fatalf("invalid create status = %d, body = %s", createResp.Code, createResp.Body.String())
	}

	validResp := performJSONRequest(t, server, http.MethodPost, "/v1/monitors", map[string]interface{}{
		"name": "Valid URL",
		"kind": "http",
		"config": map[string]interface{}{
			"url": "https://example.com/health",
		},
	}, "")
	if validResp.Code != http.StatusCreated {
		t.Fatalf("valid create status = %d, body = %s", validResp.Code, validResp.Body.String())
	}
	var created struct {
		Data struct {
			Monitor struct {
				ID string `json:"id"`
			} `json:"monitor"`
		} `json:"data"`
	}
	decodeResponse(t, validResp, &created)

	updateResp := performJSONRequest(t, server, http.MethodPatch, "/v1/monitors/"+created.Data.Monitor.ID, map[string]interface{}{
		"config": map[string]interface{}{
			"url": "ftp://example.com/health",
		},
	}, "")
	if updateResp.Code != http.StatusBadRequest {
		t.Fatalf("invalid update status = %d, body = %s", updateResp.Code, updateResp.Body.String())
	}

	if err := server.db.Model(&db.CoreMonitorConfig{}).
		Where("monitor_id = ?", created.Data.Monitor.ID).
		Update("config_json", `{"url":"ftp://example.com/health"}`).Error; err != nil {
		t.Fatalf("poison core monitor config: %v", err)
	}
	testResp := performJSONRequest(t, server, http.MethodPost, "/v1/monitors/"+created.Data.Monitor.ID+"/test", nil, "")
	if testResp.Code != http.StatusBadRequest {
		t.Fatalf("invalid test-now status = %d, body = %s", testResp.Code, testResp.Body.String())
	}
}

func TestCoreMonitorManagementEnforcesTargetPolicy(t *testing.T) {
	server := setupTestServer(t)

	createResp := performJSONRequest(t, server, http.MethodPost, "/v1/monitors", map[string]interface{}{
		"name": "Metadata Target",
		"kind": "http",
		"config": map[string]interface{}{
			"url": "http://169.254.169.254/latest/meta-data?token=secret",
		},
	}, "")
	if createResp.Code != http.StatusBadRequest {
		t.Fatalf("blocked create status = %d, body = %s", createResp.Code, createResp.Body.String())
	}
	if strings.Contains(createResp.Body.String(), "token=secret") {
		t.Fatalf("blocked create leaked query secret: %s", createResp.Body.String())
	}

	validResp := performJSONRequest(t, server, http.MethodPost, "/v1/monitors", map[string]interface{}{
		"name": "Public Target",
		"kind": "http",
		"config": map[string]interface{}{
			"url": "https://example.com/health?token=secret",
		},
	}, "")
	if validResp.Code != http.StatusCreated {
		t.Fatalf("valid create status = %d, body = %s", validResp.Code, validResp.Body.String())
	}
	var created struct {
		Data struct {
			Monitor struct {
				ID string `json:"id"`
			} `json:"monitor"`
		} `json:"data"`
	}
	decodeResponse(t, validResp, &created)

	configResp := performJSONRequest(t, server, http.MethodGet, "/v1/monitors/"+created.Data.Monitor.ID+"/config", nil, "")
	if configResp.Code != http.StatusOK {
		t.Fatalf("config status = %d, body = %s", configResp.Code, configResp.Body.String())
	}
	if strings.Contains(configResp.Body.String(), "token=secret") {
		t.Fatalf("config response leaked query secret: %s", configResp.Body.String())
	}

	updateResp := performJSONRequest(t, server, http.MethodPatch, "/v1/monitors/"+created.Data.Monitor.ID, map[string]interface{}{
		"config": map[string]interface{}{
			"url": "http://127.0.0.1:8080/health",
		},
	}, "")
	if updateResp.Code != http.StatusBadRequest {
		t.Fatalf("blocked update status = %d, body = %s", updateResp.Code, updateResp.Body.String())
	}

	if err := server.db.Model(&db.CoreMonitorConfig{}).
		Where("monitor_id = ?", created.Data.Monitor.ID).
		Update("config_json", `{"url":"http://169.254.169.254/latest/meta-data"}`).Error; err != nil {
		t.Fatalf("poison core monitor config: %v", err)
	}
	testResp := performJSONRequest(t, server, http.MethodPost, "/v1/monitors/"+created.Data.Monitor.ID+"/test", nil, "")
	if testResp.Code != http.StatusBadRequest {
		t.Fatalf("blocked test-now status = %d, body = %s", testResp.Code, testResp.Body.String())
	}
}

func TestCoreMonitorManagementAllowsPrivateTargetsWhenConfigured(t *testing.T) {
	server := setupTestServerWithConfig(t, &config.Config{CoreMonitorAllowPrivateTargets: true})

	createResp := performJSONRequest(t, server, http.MethodPost, "/v1/monitors", map[string]interface{}{
		"name": "Private Target",
		"kind": "tcp",
		"config": map[string]interface{}{
			"host": "10.0.0.5",
			"port": 443,
		},
	}, "")
	if createResp.Code != http.StatusCreated {
		t.Fatalf("private create status = %d, body = %s", createResp.Code, createResp.Body.String())
	}
}

func TestCoreMonitorManagementValidatesCatalogConfigTypes(t *testing.T) {
	server := setupTestServer(t)

	cases := []struct {
		kind          string
		validConfig   map[string]interface{}
		invalidConfig map[string]interface{}
	}{
		{
			kind:          "heartbeat",
			validConfig:   map[string]interface{}{"grace_seconds": 30},
			invalidConfig: map[string]interface{}{"grace_seconds": -1},
		},
		{
			kind:          "http",
			validConfig:   map[string]interface{}{"url": "https://example.com/health", "method": "HEAD", "expected_status": 204},
			invalidConfig: map[string]interface{}{"url": "ftp://example.com/health"},
		},
		{
			kind:          "http_keyword",
			validConfig:   map[string]interface{}{"url": "https://example.com/health", "required_contains": []string{"ok"}},
			invalidConfig: map[string]interface{}{"url": "https://example.com/health", "method": "POST"},
		},
		{
			kind:          "expected_status",
			validConfig:   map[string]interface{}{"url": "https://example.com/health", "expected_statuses": []int{200, 204}},
			invalidConfig: map[string]interface{}{"url": "https://example.com/health", "expected_status": 99},
		},
		{
			kind:          "tcp",
			validConfig:   map[string]interface{}{"host": "example.com", "port": 443},
			invalidConfig: map[string]interface{}{"host": "example.com", "port": 70000},
		},
		{
			kind:          "dns",
			validConfig:   map[string]interface{}{"host": "example.com", "record_type": "AAAA"},
			invalidConfig: map[string]interface{}{"host": "example.com", "record_type": "SOA"},
		},
		{
			kind:          "tls",
			validConfig:   map[string]interface{}{"host": "example.com", "warning_days": 14},
			invalidConfig: map[string]interface{}{"host": "", "port": 443},
		},
		{
			kind:          "udp",
			validConfig:   map[string]interface{}{"host": "example.com", "port": 53, "payload": "ping", "expected_response": "pong"},
			invalidConfig: map[string]interface{}{"host": "example.com", "port": 53, "payload": "ping"},
		},
		{
			kind:          "api_request",
			validConfig:   map[string]interface{}{"url": "https://example.com/api", "method": "POST", "expected_status": 201},
			invalidConfig: map[string]interface{}{"url": "https://example.com/api", "method": "TRACE"},
		},
		{
			kind:          "domain_expiration",
			validConfig:   map[string]interface{}{"domain": "example.com", "warning_days": 30},
			invalidConfig: map[string]interface{}{"domain": "https://example.com"},
		},
		{
			kind:          "ping",
			validConfig:   map[string]interface{}{"host": "example.com", "method": "tcp", "port": 443},
			invalidConfig: map[string]interface{}{"host": "example.com", "method": "icmp", "port": 443},
		},
		{
			kind:          "smtp",
			validConfig:   map[string]interface{}{"protocol": "smtp", "host": "mail.example.com", "port": 25, "tls_mode": "starttls"},
			invalidConfig: map[string]interface{}{"protocol": "smtp", "host": "mail.example.com", "auth_enabled": true},
		},
		{
			kind:          "imap",
			validConfig:   map[string]interface{}{"protocol": "imap", "host": "mail.example.com", "port": 993, "tls_mode": "implicit"},
			invalidConfig: map[string]interface{}{"protocol": "imap", "host": "mail.example.com", "tls_mode": "bad"},
		},
		{
			kind:          "pop3",
			validConfig:   map[string]interface{}{"protocol": "pop", "host": "mail.example.com", "port": 995, "tls_mode": "implicit"},
			invalidConfig: map[string]interface{}{"protocol": "pop", "host": "", "port": 995},
		},
		{
			kind:          "synthetic",
			validConfig:   map[string]interface{}{"steps": []map[string]interface{}{{"type": "api", "url": "https://example.com/api", "method": "GET"}}},
			invalidConfig: map[string]interface{}{"steps": []map[string]interface{}{{"type": "api", "url": "https://example.com/api", "method": "TRACE"}}},
		},
		{
			kind:          "playwright",
			validConfig:   map[string]interface{}{"url": "https://example.com", "browser": "chromium", "steps": []map[string]interface{}{{"action": "goto", "url": "https://example.com"}}},
			invalidConfig: map[string]interface{}{"steps": []map[string]interface{}{{"action": "click"}}},
		},
	}

	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			createResp := performJSONRequest(t, server, http.MethodPost, "/v1/monitors", map[string]interface{}{
				"name":   "Valid " + tc.kind,
				"kind":   tc.kind,
				"type":   tc.kind,
				"config": tc.validConfig,
			}, "")
			if createResp.Code != http.StatusCreated {
				t.Fatalf("valid create %s status = %d, body = %s", tc.kind, createResp.Code, createResp.Body.String())
			}

			invalidResp := performJSONRequest(t, server, http.MethodPost, "/v1/monitors", map[string]interface{}{
				"name":   "Invalid " + tc.kind,
				"kind":   tc.kind,
				"type":   tc.kind,
				"config": tc.invalidConfig,
			}, "")
			if invalidResp.Code != http.StatusBadRequest {
				t.Fatalf("invalid create %s status = %d, body = %s", tc.kind, invalidResp.Code, invalidResp.Body.String())
			}
		})
	}
}

func TestHeartbeatMonitorTokenAndIngestRoutes(t *testing.T) {
	server := setupTestServer(t)

	httpResp := performJSONRequest(t, server, http.MethodPost, "/v1/monitors", map[string]interface{}{
		"name": "Convertible HTTP monitor",
		"kind": "http",
		"config": map[string]interface{}{
			"url": "https://example.com/health",
		},
	}, "")
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
	convertResp := performJSONRequest(t, server, http.MethodPatch, "/v1/monitors/"+convertible.Data.Monitor.ID, map[string]interface{}{
		"kind": "heartbeat",
		"type": "heartbeat",
		"config": map[string]interface{}{
			"grace_seconds": 30,
		},
	}, "")
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

	createResp := performJSONRequest(t, server, http.MethodPost, "/v1/monitors", map[string]interface{}{
		"name":             "Nightly backup heartbeat",
		"kind":             "heartbeat",
		"interval_seconds": 300,
		"config": map[string]interface{}{
			"grace_seconds": 60,
			"token":         "caller-supplied-token-must-not-be-used",
		},
	}, "")
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
	monitor := db.Monitor{
		ID:                       "monitor-core-severity-override",
		AgentID:                  "agent-core-severity-override",
		Name:                     "Checkout journey",
		Type:                     "synthetic",
		Lifecycle:                "active",
		Health:                   "unknown",
		ComputedHealth:           "unknown",
		ReportingIntervalSeconds: 60,
		CreatedAt:                time.Now(),
		UpdatedAt:                time.Now(),
	}
	if err := server.db.Create(&monitor).Error; err != nil {
		t.Fatalf("create core monitor: %v", err)
	}
	if err := server.db.Create(&db.CoreMonitorConfig{
		MonitorID:       monitor.ID,
		Kind:            "synthetic_multi_step",
		ConfigJSON:      `{"steps":[],"incident_severity":" Critical "}`,
		SecretRefJSON:   `{}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  10,
		NextRunAt:       time.Now().UTC(),
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}).Error; err != nil {
		t.Fatalf("create core monitor config: %v", err)
	}

	_, err := server.reportService.StoreMonitorReport(monitor.ID, service.MonitorReportPayload{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Health:    "down",
		Metrics: map[string]interface{}{
			"runner":       "core",
			"failure_step": "checkout",
		},
	})
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
	monitor := db.Monitor{
		ID:                       "monitor-core-severity-invalid",
		AgentID:                  "agent-core-severity-invalid",
		Name:                     "API health",
		Type:                     "http",
		Lifecycle:                "active",
		Health:                   "unknown",
		ComputedHealth:           "unknown",
		ReportingIntervalSeconds: 60,
		CreatedAt:                time.Now(),
		UpdatedAt:                time.Now(),
	}
	if err := server.db.Create(&monitor).Error; err != nil {
		t.Fatalf("create core monitor: %v", err)
	}
	if err := server.db.Create(&db.CoreMonitorConfig{
		MonitorID:       monitor.ID,
		Kind:            "http",
		ConfigJSON:      `{"url":"https://api.example.com/health","incident_severity":"urgent"}`,
		SecretRefJSON:   `{}`,
		IntervalSeconds: 60,
		TimeoutSeconds:  10,
		NextRunAt:       time.Now().UTC(),
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}).Error; err != nil {
		t.Fatalf("create core monitor config: %v", err)
	}

	_, err := server.reportService.StoreMonitorReport(monitor.ID, service.MonitorReportPayload{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Health:    "down",
		Metrics:   map[string]interface{}{"runner": "core", "status_code": 500},
	})
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

func TestListIncidentsReturnsPersistedActiveIncidents(t *testing.T) {
	server := setupTestServer(t)
	registered := registerTestAgent(t, server)
	registeredMonitor := registerTestMonitor(t, server, registered.Data.AgentID, registered.Data.Token)
	createIncidentImpactStatusPageMapping(t, server, registeredMonitor.Data.MonitorID, registered.Data.AgentID, "incident-list-component", "Public API", "monitor")

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

	incidentsResp := performJSONRequest(t, server, http.MethodGet, "/v1/incidents", nil, "")
	if incidentsResp.Code != http.StatusOK {
		t.Fatalf("incidents status = %d, body = %s", incidentsResp.Code, incidentsResp.Body.String())
	}

	var listed struct {
		Success bool `json:"success"`
		Data    struct {
			Incidents []struct {
				Status             string `json:"status"`
				AgentID            string `json:"agent_id"`
				AgentName          string `json:"agent_name"`
				MonitorName        string `json:"monitor_name"`
				ImpactedComponents []struct {
					ComponentID   string `json:"component_id"`
					ComponentName string `json:"component_name"`
					Status        string `json:"status"`
					Impact        string `json:"impact"`
				} `json:"impacted_components"`
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
	if len(listed.Data.Incidents[0].ImpactedComponents) != 1 ||
		listed.Data.Incidents[0].ImpactedComponents[0].ComponentID != "incident-list-component" ||
		listed.Data.Incidents[0].ImpactedComponents[0].ComponentName != "Public API" ||
		listed.Data.Incidents[0].ImpactedComponents[0].Status != "major_outage" ||
		listed.Data.Incidents[0].ImpactedComponents[0].Impact != "down" {
		t.Fatalf("incident component impact = %+v, want Public API down impact", listed.Data.Incidents[0].ImpactedComponents)
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

	ackResp := performJSONRequest(t, server, http.MethodPost, "/v1/incidents/"+incident.ID+"/acknowledge", nil, "")
	if ackResp.Code != http.StatusOK {
		t.Fatalf("acknowledge list incident status = %d, body = %s", ackResp.Code, ackResp.Body.String())
	}
	resolveResp := performJSONRequest(t, server, http.MethodPost, "/v1/incidents/"+incident.ID+"/resolve", nil, "")
	if resolveResp.Code != http.StatusOK {
		t.Fatalf("resolve list incident status = %d, body = %s", resolveResp.Code, resolveResp.Body.String())
	}

	secondReportResp := performJSONRequest(t, server, http.MethodPost, reportPath, map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"health":    "down",
		"metrics":   map[string]interface{}{"status_code": 503},
	}, registered.Data.Token)
	if secondReportResp.Code != http.StatusOK {
		t.Fatalf("second monitor report status = %d, body = %s", secondReportResp.Code, secondReportResp.Body.String())
	}
	var coveredIncident db.Incident
	if err := server.db.Where("monitor_id = ? AND status = ?", registeredMonitor.Data.MonitorID, "open").First(&coveredIncident).Error; err != nil {
		t.Fatalf("find second list incident: %v", err)
	}
	coverResp := performJSONRequest(t, server, http.MethodPost, "/v1/incidents/"+coveredIncident.ID+"/cover", map[string]interface{}{
		"note": "Known recurring outage",
	}, "")
	if coverResp.Code != http.StatusOK {
		t.Fatalf("cover list incident status = %d, body = %s", coverResp.Code, coverResp.Body.String())
	}

	manualFilterResp := performJSONRequest(t, server, http.MethodGet, "/v1/incidents?status=open,acknowledged,covered,resolved&resolution_kind=manual", nil, "")
	if manualFilterResp.Code != http.StatusOK {
		t.Fatalf("manual resolution filter status = %d, body = %s", manualFilterResp.Code, manualFilterResp.Body.String())
	}
	var manualFilter struct {
		Data struct {
			Count     int64 `json:"count"`
			Incidents []struct {
				ResolutionKind string `json:"resolution_kind"`
			} `json:"incidents"`
		} `json:"data"`
	}
	decodeResponse(t, manualFilterResp, &manualFilter)
	if manualFilter.Data.Count != 1 || manualFilter.Data.Incidents[0].ResolutionKind != "manual" {
		t.Fatalf("manual resolution filter = %+v, want one manual incident", manualFilter)
	}

	actorFilterResp := performJSONRequest(t, server, http.MethodGet, "/v1/incidents?status=open,acknowledged,covered,resolved&actor=manual", nil, "")
	if actorFilterResp.Code != http.StatusOK {
		t.Fatalf("manual actor filter status = %d, body = %s", actorFilterResp.Code, actorFilterResp.Body.String())
	}
	var actorFilter struct {
		Data struct {
			Count int64 `json:"count"`
		} `json:"data"`
	}
	decodeResponse(t, actorFilterResp, &actorFilter)
	if actorFilter.Data.Count != 2 {
		t.Fatalf("manual actor filter count = %d, want 2 acknowledged/covered incidents", actorFilter.Data.Count)
	}

	coveredFilterResp := performJSONRequest(t, server, http.MethodGet, "/v1/incidents?covered=true", nil, "")
	if coveredFilterResp.Code != http.StatusOK {
		t.Fatalf("covered filter status = %d, body = %s", coveredFilterResp.Code, coveredFilterResp.Body.String())
	}
	var coveredFilter struct {
		Data struct {
			Count     int64 `json:"count"`
			Incidents []struct {
				Status string `json:"status"`
			} `json:"incidents"`
		} `json:"data"`
	}
	decodeResponse(t, coveredFilterResp, &coveredFilter)
	if coveredFilter.Data.Count != 1 || coveredFilter.Data.Incidents[0].Status != "covered" {
		t.Fatalf("covered filter = %+v, want one covered incident", coveredFilter)
	}

	insightsResp := performJSONRequest(t, server, http.MethodGet, "/v1/incidents?status=open,acknowledged,covered,resolved", nil, "")
	if insightsResp.Code != http.StatusOK {
		t.Fatalf("incident insights status = %d, body = %s", insightsResp.Code, insightsResp.Body.String())
	}
	var insights struct {
		Data struct {
			Insights struct {
				RecurringFailures []struct {
					MonitorID     string `json:"monitor_id"`
					MonitorName   string `json:"monitor_name"`
					IncidentCount int64  `json:"incident_count"`
				} `json:"recurring_failures"`
				LifecycleTiming struct {
					AcknowledgedCount            int64 `json:"acknowledged_count"`
					ResolvedCount                int64 `json:"resolved_count"`
					MeanTimeToAcknowledgeSeconds int64 `json:"mean_time_to_acknowledge_seconds"`
					MeanTimeToResolveSeconds     int64 `json:"mean_time_to_resolve_seconds"`
				} `json:"lifecycle_timing"`
				NotificationReliability struct {
					TotalDeliveries      int64   `json:"total_deliveries"`
					SuppressedDeliveries int64   `json:"suppressed_deliveries"`
					SuccessRatePercent   float64 `json:"success_rate_percent"`
				} `json:"notification_reliability"`
			} `json:"insights"`
		} `json:"data"`
	}
	decodeResponse(t, insightsResp, &insights)
	if len(insights.Data.Insights.RecurringFailures) != 1 ||
		insights.Data.Insights.RecurringFailures[0].MonitorID != registeredMonitor.Data.MonitorID ||
		insights.Data.Insights.RecurringFailures[0].MonitorName != "homepage" ||
		insights.Data.Insights.RecurringFailures[0].IncidentCount != 2 {
		t.Fatalf("recurring failure insights = %+v, want homepage with two incidents", insights.Data.Insights.RecurringFailures)
	}
	if insights.Data.Insights.LifecycleTiming.AcknowledgedCount != 1 ||
		insights.Data.Insights.LifecycleTiming.ResolvedCount != 1 ||
		insights.Data.Insights.LifecycleTiming.MeanTimeToAcknowledgeSeconds < 0 ||
		insights.Data.Insights.LifecycleTiming.MeanTimeToResolveSeconds < 0 {
		t.Fatalf("lifecycle insights = %+v, want acknowledgement and resolution timing", insights.Data.Insights.LifecycleTiming)
	}
	if insights.Data.Insights.NotificationReliability.TotalDeliveries == 0 ||
		insights.Data.Insights.NotificationReliability.SuppressedDeliveries == 0 ||
		insights.Data.Insights.NotificationReliability.SuccessRatePercent != 0 {
		t.Fatalf("notification insights = %+v, want suppressed delivery reliability stats", insights.Data.Insights.NotificationReliability)
	}
}

func TestIncidentDetailAndTimelineEndpoints(t *testing.T) {
	server := setupTestServer(t)
	registered := registerTestAgent(t, server)
	registeredMonitor := registerTestMonitor(t, server, registered.Data.AgentID, registered.Data.Token)
	createIncidentImpactStatusPageMapping(t, server, registeredMonitor.Data.MonitorID, registered.Data.AgentID, "incident-detail-component", "Checkout", "agent")

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
	relatedIncident := db.Incident{
		ID:                 "incident-related-detail",
		Status:             "resolved",
		Severity:           "medium",
		Title:              "Prior homepage failure",
		AgentID:            registered.Data.AgentID,
		MonitorID:          registeredMonitor.Data.MonitorID,
		OpenedAt:           incident.OpenedAt.Add(-2 * time.Hour),
		ResolvedAt:         &incident.OpenedAt,
		LastEventAt:        incident.OpenedAt,
		LatestEvent:        "Prior incident resolved",
		NotificationStatus: "suppressed",
		ResolutionKind:     "recovered",
	}
	if err := server.db.Create(&relatedIncident).Error; err != nil {
		t.Fatalf("create related incident: %v", err)
	}

	latestResp := performJSONRequest(t, server, http.MethodPost, reportPath, map[string]interface{}{
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"health":    "degraded",
		"metrics":   map[string]interface{}{"status_code": 503, "message": "slow checkout"},
	}, registered.Data.Token)
	if latestResp.Code != http.StatusOK {
		t.Fatalf("latest report status = %d, body = %s", latestResp.Code, latestResp.Body.String())
	}

	detailResp := performJSONRequest(t, server, http.MethodGet, "/v1/incidents/"+incident.ID, nil, "")
	if detailResp.Code != http.StatusOK {
		t.Fatalf("incident detail status = %d, body = %s", detailResp.Code, detailResp.Body.String())
	}
	var detail struct {
		Success bool `json:"success"`
		Data    struct {
			Incident struct {
				ID                 string `json:"id"`
				AgentName          string `json:"agent_name"`
				MonitorName        string `json:"monitor_name"`
				ImpactedComponents []struct {
					ComponentID   string `json:"component_id"`
					ComponentName string `json:"component_name"`
					Status        string `json:"status"`
					Impact        string `json:"impact"`
				} `json:"impacted_components"`
			} `json:"incident"`
			Evidence struct {
				TriggeringReport *struct {
					ID      string `json:"id"`
					Health  string `json:"health"`
					Payload string `json:"payload"`
				} `json:"triggering_report"`
				LatestReport *struct {
					ID      string `json:"id"`
					Health  string `json:"health"`
					Payload string `json:"payload"`
				} `json:"latest_report"`
			} `json:"evidence"`
			RelatedIncidents []struct {
				ID             string `json:"id"`
				ResolutionKind string `json:"resolution_kind"`
			} `json:"related_incidents"`
			Timeline []struct {
				Type            string `json:"type"`
				Source          string `json:"source"`
				Evidence        string `json:"evidence"`
				MonitorReportID string `json:"monitor_report_id"`
				AlertDeliveryID string `json:"alert_delivery_id"`
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
	if len(detail.Data.Incident.ImpactedComponents) != 1 ||
		detail.Data.Incident.ImpactedComponents[0].ComponentID != "incident-detail-component" ||
		detail.Data.Incident.ImpactedComponents[0].ComponentName != "Checkout" ||
		detail.Data.Incident.ImpactedComponents[0].Status != "degraded" ||
		detail.Data.Incident.ImpactedComponents[0].Impact != "degraded" {
		t.Fatalf("incident detail component impact = %+v, want Checkout degraded impact", detail.Data.Incident.ImpactedComponents)
	}
	if len(detail.Data.Timeline) < 2 || len(detail.Data.AlertDeliveries) == 0 || len(detail.Data.MonitorReports) == 0 {
		t.Fatalf("incident detail linked data missing: %+v", detail.Data)
	}
	if detail.Data.Evidence.TriggeringReport == nil ||
		detail.Data.Evidence.TriggeringReport.Health != "down" ||
		!strings.Contains(detail.Data.Evidence.TriggeringReport.Payload, `"status_code":500`) {
		t.Fatalf("triggering evidence = %+v, want down report with safe payload", detail.Data.Evidence.TriggeringReport)
	}
	if detail.Data.Evidence.LatestReport == nil ||
		detail.Data.Evidence.LatestReport.Health != "degraded" ||
		!strings.Contains(detail.Data.Evidence.LatestReport.Payload, "slow checkout") {
		t.Fatalf("latest evidence = %+v, want latest degraded report", detail.Data.Evidence.LatestReport)
	}
	if len(detail.Data.RelatedIncidents) != 1 ||
		detail.Data.RelatedIncidents[0].ID != relatedIncident.ID ||
		detail.Data.RelatedIncidents[0].ResolutionKind != "recovered" {
		t.Fatalf("related incidents = %+v, want prior same-monitor incident", detail.Data.RelatedIncidents)
	}
	var reportLinked bool
	var deliveryLinked bool
	for _, item := range detail.Data.Timeline {
		if item.MonitorReportID != "" && item.Evidence != "" {
			reportLinked = true
		}
		if item.AlertDeliveryID != "" {
			deliveryLinked = true
		}
	}
	if !reportLinked || !deliveryLinked {
		t.Fatalf("timeline links = %+v, want report evidence and alert delivery links", detail.Data.Timeline)
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

func createIncidentImpactStatusPageMapping(t *testing.T, server *Server, monitorID string, agentID string, componentID string, componentName string, resourceType string) {
	t.Helper()

	now := time.Now().UTC()
	page := db.StatusPage{
		ID:                        componentID + "-page",
		Slug:                      componentID + "-page",
		Title:                     componentName + " Status",
		Visibility:                statusPageVisibilityPublic,
		ThemeSettings:             "{}",
		DefaultIncidentVisibility: statusPageIncidentVisibilityDraft,
		PublishedAt:               &now,
	}
	section := db.StatusPageSection{
		ID:           componentID + "-section",
		StatusPageID: page.ID,
		Name:         "Public services",
	}
	component := db.StatusPageComponent{
		ID:           componentID,
		StatusPageID: page.ID,
		SectionID:    section.ID,
		PublicName:   componentName,
		DisplayMode:  "single_resource",
		SortOrder:    1,
		Visible:      true,
	}
	resourceID := monitorID
	if resourceType == "agent" {
		resourceID = agentID
	}
	mapping := db.StatusPageComponentMapping{
		ID:                   componentID + "-mapping",
		ComponentID:          component.ID,
		ResourceType:         resourceType,
		ResourceID:           resourceID,
		HealthRollupStrategy: "worst",
		UptimeRollupStrategy: "worst",
	}
	if err := server.db.Create(&page).Error; err != nil {
		t.Fatalf("create status page for incident impact: %v", err)
	}
	if err := server.db.Create(&section).Error; err != nil {
		t.Fatalf("create status page section for incident impact: %v", err)
	}
	if err := server.db.Create(&component).Error; err != nil {
		t.Fatalf("create status page component for incident impact: %v", err)
	}
	if err := server.db.Create(&mapping).Error; err != nil {
		t.Fatalf("create status page component mapping for incident impact: %v", err)
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
	return setupTestServerWithConfig(t, &config.Config{AlertRecoveryNotifications: true, AlertTLSExpiryDays: 14})
}

func setupTestServerWithConfig(t *testing.T, cfg *config.Config) *Server {
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

	return NewServer(database, logging.NewLogger(), cfg)
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
		ID:                   "alert-channel-webhook",
		Name:                 "ops-webhook",
		Type:                 "webhook",
		Enabled:              true,
		WebhookURL:           "https://secret.example.com/hook",
		WebhookSigningSecret: "webhook-signing-secret",
	}).Error; err != nil {
		t.Fatalf("create webhook channel: %v", err)
	}
	if err := server.db.Create(&db.AlertChannel{
		ID:      "alert-channel-email",
		Name:    "ops-email",
		Type:    "email",
		Enabled: false,
	}).Error; err != nil {
		t.Fatalf("create email channel: %v", err)
	}

	delivery := db.AlertDelivery{
		ID:           "alert-delivery-test",
		IncidentID:   "incident-test",
		AlertGroupID: "alert-group-test",
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
	assertNotContains(t, channelsResp.Body.String(), "webhook-signing-secret")

	var channels struct {
		Success bool `json:"success"`
		Data    struct {
			Channels []struct {
				Name                       string `json:"name"`
				Type                       string `json:"type"`
				WebhookURL                 string `json:"webhook_url"`
				WebhookConfigured          bool   `json:"webhook_configured"`
				WebhookSignatureConfigured bool   `json:"webhook_signature_configured"`
				LastDeliveryStatus         string `json:"last_delivery_status"`
			} `json:"channels"`
			Count int `json:"count"`
		} `json:"data"`
	}
	decodeResponse(t, channelsResp, &channels)
	if !channels.Success || channels.Data.Count != 1 || len(channels.Data.Channels) != 1 {
		t.Fatalf("channels response = %+v, want one non-email channel", channels)
	}
	var webhookChannel struct {
		Name                       string `json:"name"`
		Type                       string `json:"type"`
		WebhookURL                 string `json:"webhook_url"`
		WebhookConfigured          bool   `json:"webhook_configured"`
		WebhookSignatureConfigured bool   `json:"webhook_signature_configured"`
		LastDeliveryStatus         string `json:"last_delivery_status"`
	}
	for _, channel := range channels.Data.Channels {
		if channel.Name == "ops-webhook" {
			webhookChannel = channel
			break
		}
	}
	if webhookChannel.WebhookURL != "https://secret.example.com/hook" || !webhookChannel.WebhookConfigured || !webhookChannel.WebhookSignatureConfigured || webhookChannel.LastDeliveryStatus != "failed" {
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
				IncidentID   string `json:"incident_id"`
				AlertGroupID string `json:"alert_group_id"`
				Status       string `json:"status"`
				Error        string `json:"error"`
				Attempts     []struct {
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
	if filteredDelivery.IncidentID != "incident-test" || filteredDelivery.AlertGroupID != "alert-group-test" || filteredDelivery.Status != "failed" || filteredDelivery.Error != "delivery failed; check Core logs" {
		t.Fatalf("filtered delivery = %+v, want sanitized failed incident-test delivery with alert_group_id", filteredDelivery)
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
	assertNotContains(t, rulesResp.Body.String(), "webhook-signing-secret")
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
		"name":                   "ops-webhook",
		"type":                   "webhook",
		"enabled":                true,
		"webhook_url":            "https://secret.example.com/hook",
		"webhook_signing_secret": "initial-signing-secret",
		"subscribed_events":      []string{db.AlertEventIncidentOpened},
	}, "")
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create channel status = %d, body = %s", createResp.Code, createResp.Body.String())
	}
	var created struct {
		Data struct {
			Channel struct {
				ID                         string   `json:"id"`
				Name                       string   `json:"name"`
				WebhookURL                 string   `json:"webhook_url"`
				WebhookConfigured          bool     `json:"webhook_configured"`
				WebhookSignatureConfigured bool     `json:"webhook_signature_configured"`
				SubscribedEvents           []string `json:"subscribed_events"`
			} `json:"channel"`
		} `json:"data"`
	}
	decodeResponse(t, createResp, &created)
	assertNotContains(t, createResp.Body.String(), "initial-signing-secret")
	if created.Data.Channel.ID == "" || created.Data.Channel.Name != "ops-webhook" || created.Data.Channel.WebhookURL != "https://secret.example.com/hook" || !created.Data.Channel.WebhookConfigured || !created.Data.Channel.WebhookSignatureConfigured {
		t.Fatalf("created channel = %+v, want webhook channel", created.Data.Channel)
	}
	if got := created.Data.Channel.SubscribedEvents; len(got) != 1 || got[0] != db.AlertEventIncidentOpened {
		t.Fatalf("created subscribed_events = %#v, want incident_opened", got)
	}

	updateResp := performJSONRequest(t, server, http.MethodPatch, "/v1/alerts/channels/"+created.Data.Channel.ID, gin.H{
		"name":                   "critical-webhook",
		"enabled":                false,
		"webhook_url":            "https://alerts.example.com/critical",
		"webhook_signing_secret": "rotated-signing-secret",
		"subscribed_events":      []string{db.AlertEventIncidentOpened, db.AlertEventIncidentResolved},
	}, "")
	if updateResp.Code != http.StatusOK {
		t.Fatalf("update channel status = %d, body = %s", updateResp.Code, updateResp.Body.String())
	}
	assertNotContains(t, updateResp.Body.String(), "rotated-signing-secret")

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
	if stored.WebhookSigningSecret != "rotated-signing-secret" {
		t.Fatalf("stored webhook signing secret = %q, want rotated signing secret", stored.WebhookSigningSecret)
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

func TestAlertChannelWriteEndpointsRejectNonWebhookTypes(t *testing.T) {
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
	for _, channelType := range []string{"slack", "discord", "email"} {
		createResp := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/channels", gin.H{
			"name":        "ops-" + channelType,
			"type":        channelType,
			"webhook_url": "https://alerts.example.com/" + channelType,
		}, "")
		if createResp.Code != http.StatusBadRequest || !strings.Contains(createResp.Body.String(), "unsupported alert channel type") {
			t.Fatalf("create %s channel status = %d, body = %s, want unsupported type rejection", channelType, createResp.Code, createResp.Body.String())
		}
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
		"name":                   "critical route",
		"priority":               10,
		"event_types":            []string{db.AlertEventIncidentOpened},
		"severities":             []string{"high"},
		"channel_ids":            []string{"channel-ops-webhook"},
		"grouping_policy":        db.AlertGroupingPolicyDelayedSummary,
		"grouping_delay_seconds": 120,
	}, "")
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create route status = %d, body = %s", createResp.Code, createResp.Body.String())
	}
	var created struct {
		Data struct {
			Route struct {
				ID                   string   `json:"id"`
				Name                 string   `json:"name"`
				EventTypes           []string `json:"event_types"`
				ChannelIDs           []string `json:"channel_ids"`
				GroupingPolicy       string   `json:"grouping_policy"`
				GroupingDelaySeconds int      `json:"grouping_delay_seconds"`
			} `json:"route"`
		} `json:"data"`
	}
	decodeResponse(t, createResp, &created)
	if created.Data.Route.ID == "" || created.Data.Route.Name != "critical route" || len(created.Data.Route.ChannelIDs) != 1 ||
		created.Data.Route.GroupingPolicy != db.AlertGroupingPolicyDelayedSummary || created.Data.Route.GroupingDelaySeconds != 120 {
		t.Fatalf("created route = %+v, want route with channel", created.Data.Route)
	}

	listResp := performJSONRequest(t, server, http.MethodGet, "/v1/alerts/routes", nil, "")
	if listResp.Code != http.StatusOK {
		t.Fatalf("list routes status = %d, body = %s", listResp.Code, listResp.Body.String())
	}
	var listed struct {
		Data struct {
			Routes []struct {
				ID                   string   `json:"id"`
				Name                 string   `json:"name"`
				Enabled              bool     `json:"enabled"`
				Priority             int      `json:"priority"`
				EventTypes           []string `json:"event_types"`
				Severities           []string `json:"severities"`
				ChannelIDs           []string `json:"channel_ids"`
				Suppress             bool     `json:"suppress"`
				GroupingPolicy       string   `json:"grouping_policy"`
				GroupingDelaySeconds int      `json:"grouping_delay_seconds"`
			} `json:"routes"`
			Count int `json:"count"`
		} `json:"data"`
	}
	decodeResponse(t, listResp, &listed)
	if listed.Data.Count != 1 || len(listed.Data.Routes) != 1 {
		t.Fatalf("listed routes count=%d len=%d, want one route", listed.Data.Count, len(listed.Data.Routes))
	}
	if listed.Data.Routes[0].ID != created.Data.Route.ID || listed.Data.Routes[0].Name != "critical route" || listed.Data.Routes[0].Priority != 10 ||
		listed.Data.Routes[0].GroupingPolicy != db.AlertGroupingPolicyDelayedSummary || listed.Data.Routes[0].GroupingDelaySeconds != 120 {
		t.Fatalf("listed route = %+v, want created critical route", listed.Data.Routes[0])
	}
	if len(listed.Data.Routes[0].EventTypes) != 1 || listed.Data.Routes[0].EventTypes[0] != db.AlertEventIncidentOpened || len(listed.Data.Routes[0].ChannelIDs) != 1 {
		t.Fatalf("listed route filters = %+v, want opened event and channel", listed.Data.Routes[0])
	}

	updateResp := performJSONRequest(t, server, http.MethodPatch, "/v1/alerts/routes/"+created.Data.Route.ID, gin.H{
		"name":                   "suppress recovery",
		"enabled":                false,
		"priority":               5,
		"event_types":            []string{db.AlertEventIncidentResolved},
		"severities":             []string{"medium"},
		"agent_ids":              []string{"agent-prod"},
		"monitor_ids":            []string{"monitor-api"},
		"monitor_types":          []string{"http"},
		"channel_ids":            []string{},
		"suppress":               true,
		"grouping_policy":        db.AlertGroupingPolicyNone,
		"grouping_delay_seconds": 30,
	}, "")
	if updateResp.Code != http.StatusOK {
		t.Fatalf("update route status = %d, body = %s", updateResp.Code, updateResp.Body.String())
	}
	var updated struct {
		Data struct {
			Route struct {
				ID                   string   `json:"id"`
				Name                 string   `json:"name"`
				Enabled              bool     `json:"enabled"`
				Priority             int      `json:"priority"`
				EventTypes           []string `json:"event_types"`
				Severities           []string `json:"severities"`
				AgentIDs             []string `json:"agent_ids"`
				MonitorIDs           []string `json:"monitor_ids"`
				MonitorTypes         []string `json:"monitor_types"`
				ChannelIDs           []string `json:"channel_ids"`
				Suppress             bool     `json:"suppress"`
				GroupingPolicy       string   `json:"grouping_policy"`
				GroupingDelaySeconds int      `json:"grouping_delay_seconds"`
			} `json:"route"`
		} `json:"data"`
	}
	decodeResponse(t, updateResp, &updated)
	if updated.Data.Route.ID != created.Data.Route.ID || updated.Data.Route.Name != "suppress recovery" || updated.Data.Route.Enabled || updated.Data.Route.Priority != 5 ||
		!updated.Data.Route.Suppress || updated.Data.Route.GroupingPolicy != db.AlertGroupingPolicyNone || updated.Data.Route.GroupingDelaySeconds != 30 {
		t.Fatalf("updated route = %+v, want disabled suppress recovery route", updated.Data.Route)
	}
	if len(updated.Data.Route.EventTypes) != 1 || updated.Data.Route.EventTypes[0] != db.AlertEventIncidentResolved {
		t.Fatalf("updated event_types = %#v, want resolved", updated.Data.Route.EventTypes)
	}
	if len(updated.Data.Route.Severities) != 1 || updated.Data.Route.Severities[0] != "medium" ||
		len(updated.Data.Route.AgentIDs) != 1 || updated.Data.Route.AgentIDs[0] != "agent-prod" ||
		len(updated.Data.Route.MonitorIDs) != 1 || updated.Data.Route.MonitorIDs[0] != "monitor-api" ||
		len(updated.Data.Route.MonitorTypes) != 1 || updated.Data.Route.MonitorTypes[0] != "http" ||
		len(updated.Data.Route.ChannelIDs) != 0 {
		t.Fatalf("updated route filters = %+v, want requested filters and no channels", updated.Data.Route)
	}

	deleteResp := performJSONRequest(t, server, http.MethodDelete, "/v1/alerts/routes/"+created.Data.Route.ID, nil, "")
	if deleteResp.Code != http.StatusOK {
		t.Fatalf("delete route status = %d, body = %s", deleteResp.Code, deleteResp.Body.String())
	}
	var routeCount int64
	if err := server.db.Model(&db.AlertRoute{}).Count(&routeCount).Error; err != nil {
		t.Fatalf("count alert routes: %v", err)
	}
	if routeCount != 0 {
		t.Fatalf("alert route count = %d, want 0", routeCount)
	}

	createResp = performJSONRequest(t, server, http.MethodPost, "/v1/alerts/routes", gin.H{
		"name":        "critical route",
		"priority":    10,
		"event_types": []string{db.AlertEventIncidentOpened},
		"severities":  []string{"high"},
		"channel_ids": []string{"channel-ops-webhook"},
	}, "")
	if createResp.Code != http.StatusCreated {
		t.Fatalf("recreate route status = %d, body = %s", createResp.Code, createResp.Body.String())
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

func TestAlertRuleWriteEnableDisableAndDryRunEndpoints(t *testing.T) {
	server := setupTestServer(t)
	if err := server.db.Create(&db.AlertChannel{
		ID:         "channel-ops-webhook",
		Name:       "ops-webhook",
		Type:       "webhook",
		Enabled:    true,
		WebhookURL: "https://alerts.example.com/hook",
	}).Error; err != nil {
		t.Fatalf("create webhook channel: %v", err)
	}
	if err := server.db.Create(&db.AlertChannel{
		ID:         "channel-ops-slack",
		Name:       "ops-slack",
		Type:       "slack",
		Enabled:    true,
		WebhookURL: "https://alerts.example.com/slack",
	}).Error; err != nil {
		t.Fatalf("create slack channel: %v", err)
	}

	rejectResp := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/rules", gin.H{
		"name":        "chat rule",
		"channel_ids": []string{"channel-ops-slack"},
	}, "")
	if rejectResp.Code != http.StatusBadRequest || !strings.Contains(rejectResp.Body.String(), "webhook alert channels") {
		t.Fatalf("chat rule status = %d body = %s, want webhook-only rejection", rejectResp.Code, rejectResp.Body.String())
	}

	createResp := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/rules", gin.H{
		"name":                   "critical webhook rule",
		"priority":               10,
		"event_types":            []string{db.AlertEventIncidentOpened},
		"severities":             []string{"high"},
		"agent_ids":              []string{"agent-prod"},
		"monitor_types":          []string{"http"},
		"channel_ids":            []string{"channel-ops-webhook"},
		"grouping_policy":        db.AlertGroupingPolicyDelayedSummary,
		"grouping_delay_seconds": 90,
	}, "")
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create rule status = %d, body = %s", createResp.Code, createResp.Body.String())
	}
	var created struct {
		Data struct {
			Rule struct {
				ID                   string   `json:"id"`
				Name                 string   `json:"name"`
				Enabled              bool     `json:"enabled"`
				Priority             int      `json:"priority"`
				EventTypes           []string `json:"event_types"`
				Severities           []string `json:"severities"`
				AgentIDs             []string `json:"agent_ids"`
				MonitorTypes         []string `json:"monitor_types"`
				ChannelIDs           []string `json:"channel_ids"`
				GroupingPolicy       string   `json:"grouping_policy"`
				GroupingDelaySeconds int      `json:"grouping_delay_seconds"`
			} `json:"rule"`
		} `json:"data"`
	}
	decodeResponse(t, createResp, &created)
	if created.Data.Rule.ID == "" || created.Data.Rule.Name != "critical webhook rule" || !created.Data.Rule.Enabled || created.Data.Rule.Priority != 10 ||
		created.Data.Rule.GroupingPolicy != db.AlertGroupingPolicyDelayedSummary || created.Data.Rule.GroupingDelaySeconds != 90 {
		t.Fatalf("created rule = %+v, want critical webhook rule", created.Data.Rule)
	}
	if len(created.Data.Rule.EventTypes) != 1 || created.Data.Rule.EventTypes[0] != db.AlertEventIncidentOpened ||
		len(created.Data.Rule.Severities) != 1 || created.Data.Rule.Severities[0] != "high" ||
		len(created.Data.Rule.AgentIDs) != 1 || created.Data.Rule.AgentIDs[0] != "agent-prod" ||
		len(created.Data.Rule.MonitorTypes) != 1 || created.Data.Rule.MonitorTypes[0] != "http" ||
		len(created.Data.Rule.ChannelIDs) != 1 || created.Data.Rule.ChannelIDs[0] != "channel-ops-webhook" {
		t.Fatalf("created rule filters = %+v, want requested filters and webhook channel", created.Data.Rule)
	}

	listResp := performJSONRequest(t, server, http.MethodGet, "/v1/alerts/rules", nil, "")
	if listResp.Code != http.StatusOK {
		t.Fatalf("list rules status = %d, body = %s", listResp.Code, listResp.Body.String())
	}
	var listed struct {
		Data struct {
			Rules []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"rules"`
			Count int `json:"count"`
		} `json:"data"`
	}
	decodeResponse(t, listResp, &listed)
	if listed.Data.Count != 1 || len(listed.Data.Rules) != 1 || listed.Data.Rules[0].ID != created.Data.Rule.ID {
		t.Fatalf("listed rules = %+v, want created rule", listed.Data)
	}

	invalidUpdateResp := performJSONRequest(t, server, http.MethodPatch, "/v1/alerts/rules/"+created.Data.Rule.ID, gin.H{
		"event_types": []string{"not-an-alert-event"},
	}, "")
	if invalidUpdateResp.Code != http.StatusBadRequest || !strings.Contains(invalidUpdateResp.Body.String(), "invalid event_types") {
		t.Fatalf("invalid update status = %d, body = %s, want invalid event rejection", invalidUpdateResp.Code, invalidUpdateResp.Body.String())
	}

	disableResp := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/rules/"+created.Data.Rule.ID+"/disable", nil, "")
	if disableResp.Code != http.StatusOK {
		t.Fatalf("disable rule status = %d, body = %s", disableResp.Code, disableResp.Body.String())
	}
	var disabled struct {
		Data struct {
			Rule struct {
				Enabled bool `json:"enabled"`
			} `json:"rule"`
		} `json:"data"`
	}
	decodeResponse(t, disableResp, &disabled)
	if disabled.Data.Rule.Enabled {
		t.Fatalf("disabled rule enabled = true, want false")
	}

	enableResp := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/rules/"+created.Data.Rule.ID+"/enable", nil, "")
	if enableResp.Code != http.StatusOK {
		t.Fatalf("enable rule status = %d, body = %s", enableResp.Code, enableResp.Body.String())
	}

	updateResp := performJSONRequest(t, server, http.MethodPatch, "/v1/alerts/rules/"+created.Data.Rule.ID, gin.H{
		"name":            "suppress webhook recovery",
		"event_types":     []string{db.AlertEventIncidentResolved},
		"severities":      []string{"medium"},
		"channel_ids":     []string{},
		"suppress":        true,
		"grouping_policy": db.AlertGroupingPolicyNone,
	}, "")
	if updateResp.Code != http.StatusOK {
		t.Fatalf("update rule status = %d, body = %s", updateResp.Code, updateResp.Body.String())
	}
	var updated struct {
		Data struct {
			Rule struct {
				Name           string   `json:"name"`
				Enabled        bool     `json:"enabled"`
				EventTypes     []string `json:"event_types"`
				ChannelIDs     []string `json:"channel_ids"`
				Suppress       bool     `json:"suppress"`
				GroupingPolicy string   `json:"grouping_policy"`
			} `json:"rule"`
		} `json:"data"`
	}
	decodeResponse(t, updateResp, &updated)
	if updated.Data.Rule.Name != "suppress webhook recovery" || !updated.Data.Rule.Enabled || !updated.Data.Rule.Suppress ||
		updated.Data.Rule.GroupingPolicy != db.AlertGroupingPolicyNone || len(updated.Data.Rule.ChannelIDs) != 0 ||
		len(updated.Data.Rule.EventTypes) != 1 || updated.Data.Rule.EventTypes[0] != db.AlertEventIncidentResolved {
		t.Fatalf("updated rule = %+v, want enabled suppress recovery rule", updated.Data.Rule)
	}

	dryRunResp := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/rules/dry-run", gin.H{
		"event_type":   db.AlertEventIncidentResolved,
		"severity":     "medium",
		"agent_id":     "agent-prod",
		"monitor_type": "http",
	}, "")
	if dryRunResp.Code != http.StatusOK {
		t.Fatalf("dry-run rule status = %d, body = %s", dryRunResp.Code, dryRunResp.Body.String())
	}
	var dryRun struct {
		Data struct {
			DryRun struct {
				Suppressed      bool `json:"suppressed"`
				RuleEvaluations []struct {
					Rule struct {
						ID string `json:"id"`
					} `json:"rule"`
					Matched    bool `json:"matched"`
					Suppressed bool `json:"suppressed"`
				} `json:"rule_evaluations"`
				DestinationDecisions []struct {
					RuleID   string `json:"rule_id"`
					RuleName string `json:"rule_name"`
					Status   string `json:"status"`
				} `json:"destination_decisions"`
			} `json:"dry_run"`
		} `json:"data"`
	}
	decodeResponse(t, dryRunResp, &dryRun)
	assertNotContains(t, dryRunResp.Body.String(), "route_id")
	assertNotContains(t, dryRunResp.Body.String(), "route_name")
	if !dryRun.Data.DryRun.Suppressed || len(dryRun.Data.DryRun.RuleEvaluations) != 1 ||
		dryRun.Data.DryRun.RuleEvaluations[0].Rule.ID != created.Data.Rule.ID ||
		!dryRun.Data.DryRun.RuleEvaluations[0].Matched || !dryRun.Data.DryRun.RuleEvaluations[0].Suppressed ||
		len(dryRun.Data.DryRun.DestinationDecisions) != 0 {
		t.Fatalf("dry-run rule response = %+v, want suppressing matched rule without destinations", dryRun.Data.DryRun)
	}
	var deliveryCount int64
	if err := server.db.Model(&db.AlertDelivery{}).Count(&deliveryCount).Error; err != nil {
		t.Fatalf("count deliveries: %v", err)
	}
	if deliveryCount != 0 {
		t.Fatalf("delivery count = %d, want rule dry-run to avoid side effects", deliveryCount)
	}

	deleteResp := performJSONRequest(t, server, http.MethodDelete, "/v1/alerts/rules/"+created.Data.Rule.ID, nil, "")
	if deleteResp.Code != http.StatusOK {
		t.Fatalf("delete rule status = %d, body = %s", deleteResp.Code, deleteResp.Body.String())
	}
	var ruleCount int64
	if err := server.db.Model(&db.AlertRoute{}).Count(&ruleCount).Error; err != nil {
		t.Fatalf("count alert rules: %v", err)
	}
	if ruleCount != 0 {
		t.Fatalf("alert rule count = %d, want 0", ruleCount)
	}
}

func TestRemovedAlertDestinationEndpointsAreUnavailable(t *testing.T) {
	server := setupTestServer(t)
	removedEndpoints := []struct {
		method string
		path   string
		body   interface{}
	}{
		{method: http.MethodGet, path: "/v1/alerts/smtp-services"},
		{method: http.MethodPost, path: "/v1/alerts/smtp-services", body: gin.H{"name": "SMTP"}},
		{method: http.MethodGet, path: "/v1/alerts/email-destinations"},
		{method: http.MethodPost, path: "/v1/alerts/email-destinations", body: gin.H{"name": "Ops Email"}},
	}
	for _, endpoint := range removedEndpoints {
		resp := performJSONRequest(t, server, endpoint.method, endpoint.path, endpoint.body, "")
		if resp.Code != http.StatusNotFound {
			t.Fatalf("%s %s status = %d, body = %s, want 404", endpoint.method, endpoint.path, resp.Code, resp.Body.String())
		}
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

func assertIncidentEvent(t *testing.T, server *Server, incidentID string, eventType string, wantMessage string) {
	t.Helper()

	var event db.IncidentEvent
	if err := server.db.Where("incident_id = ? AND type = ? AND message = ?", incidentID, eventType, wantMessage).First(&event).Error; err != nil {
		t.Fatalf("find incident event %s: %v", eventType, err)
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

func seedCoreConfirmationMonitor(t *testing.T, server *Server, monitorID string, confirmationPeriodSeconds int, confirmationCheckCount int) db.Monitor {
	t.Helper()

	return seedCoreNoiseMonitor(t, server, monitorID, confirmationPeriodSeconds, confirmationCheckCount, 0)
}

func seedCoreNoiseMonitor(t *testing.T, server *Server, monitorID string, confirmationPeriodSeconds int, confirmationCheckCount int, recoveryPeriodSeconds int) db.Monitor {
	t.Helper()

	now := time.Now().UTC()
	monitor := db.Monitor{
		ID:                       monitorID,
		AgentID:                  "agent-core-worker",
		Name:                     monitorID,
		Type:                     "http",
		Lifecycle:                "active",
		Health:                   "unknown",
		ComputedHealth:           "unknown",
		ReportingIntervalSeconds: 30,
		CreatedAt:                now,
		UpdatedAt:                now,
	}
	if err := server.db.Create(&monitor).Error; err != nil {
		t.Fatalf("create core confirmation monitor: %v", err)
	}
	config := db.CoreMonitorConfig{
		MonitorID:                 monitor.ID,
		Kind:                      "http",
		ConfigJSON:                "{}",
		SecretRefJSON:             "{}",
		IntervalSeconds:           30,
		TimeoutSeconds:            10,
		ConfirmationPeriodSeconds: confirmationPeriodSeconds,
		ConfirmationCheckCount:    confirmationCheckCount,
		RecoveryPeriodSeconds:     recoveryPeriodSeconds,
		NextRunAt:                 now,
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}
	if err := server.db.Create(&config).Error; err != nil {
		t.Fatalf("create core confirmation config: %v", err)
	}
	return monitor
}

func seedCoreMaintenanceMonitor(t *testing.T, server *Server, monitorID string, startAt time.Time, endAt time.Time) db.Monitor {
	t.Helper()

	monitor := seedCoreNoiseMonitor(t, server, monitorID, 0, 0, 0)
	configJSON := fmt.Sprintf(
		`{"url":"https://api.example.com/health","maintenance_windows":[{"start_at":%q,"end_at":%q}]}`,
		startAt.Format(time.RFC3339),
		endAt.Format(time.RFC3339),
	)
	if err := server.db.Model(&db.CoreMonitorConfig{}).Where("monitor_id = ?", monitor.ID).Update("config_json", configJSON).Error; err != nil {
		t.Fatalf("set core maintenance config: %v", err)
	}
	return monitor
}

func storeCoreConfirmationReport(t *testing.T, server *Server, monitorID string, health string, reportedAt time.Time) {
	t.Helper()

	statusCode := 500
	if health == "up" {
		statusCode = 200
	}
	reportID, err := server.reportService.StoreMonitorReport(monitorID, service.MonitorReportPayload{
		Timestamp: reportedAt.Format(time.RFC3339),
		Health:    health,
		Metrics: map[string]interface{}{
			"runner":      "core",
			"status_code": statusCode,
		},
	})
	if err != nil {
		t.Fatalf("store core confirmation %s report: %v", health, err)
	}
	if reportID == nil || *reportID == "" {
		t.Fatalf("core confirmation report id = %v, want generated id", reportID)
	}
}

func assertCoreIncidentCount(t *testing.T, server *Server, monitorID string, want int64) {
	t.Helper()

	var count int64
	if err := server.db.Model(&db.Incident{}).Where("monitor_id = ?", monitorID).Count(&count).Error; err != nil {
		t.Fatalf("count core confirmation incidents: %v", err)
	}
	if count != want {
		t.Fatalf("core confirmation incident count = %d, want %d", count, want)
	}
}

func assertIncidentEventCount(t *testing.T, server *Server, incidentID string, eventType string, want int64) {
	t.Helper()

	var count int64
	if err := server.db.Model(&db.IncidentEvent{}).Where("incident_id = ? AND type = ?", incidentID, eventType).Count(&count).Error; err != nil {
		t.Fatalf("count incident events: %v", err)
	}
	if count != want {
		t.Fatalf("incident event count for %s = %d, want %d", eventType, count, want)
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

func performRawRequest(t *testing.T, server *Server, method string, path string, body io.Reader, token string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, path, body)
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
