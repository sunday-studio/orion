package api

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"net/http"
	"net/http/httptest"
	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/logging"
	"strings"
	"testing"
	"time"
)

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
	agentReportResp := performJSONRequest(t, server, http.MethodPost, "/v1/agents/"+registered.Data.AgentID+"/report", map[string]interface{}{"uptime_seconds": 60, "timestamp": time.Now().UTC().Format(time.RFC3339), "cpu": map[string]interface{}{"usage_percent": 10}, "memory": map[string]interface{}{"used_percent": 20}, "disk": map[string]interface{}{"used_percent": 30}}, registered.Data.Token)
	if agentReportResp.Code != http.StatusOK {
		t.Fatalf("agent report status = %d, body = %s", agentReportResp.Code, agentReportResp.Body.String())
	}
	monitorReportResp := performJSONRequest(t, server, http.MethodPost, "/v1/agents/"+registered.Data.AgentID+"/"+registeredMonitor.Data.MonitorID+"/report", map[string]interface{}{"timestamp": time.Now().UTC().Format(time.RFC3339), "health": "down", "metrics": map[string]interface{}{"failure_reason": "diagnostics test failure"}}, registered.Data.Token)
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
	loginResp := performJSONRequest(t, server, http.MethodPost, "/v1/auth/login", map[string]string{"username": "admin", "password": "password"}, "")
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
	server := NewServer(database, logging.NewLogger(), &config.Config{FrontendAuthOn: true, AdminUsername: "admin", AdminPassword: "correct-password", JWTSecret: "test-secret"})
	badResp := performJSONRequest(t, server, http.MethodPost, "/v1/auth/login", map[string]string{"username": "admin", "password": "wrong-password"}, "")
	if badResp.Code != http.StatusUnauthorized {
		t.Fatalf("bad login status = %d, body = %s, want 401", badResp.Code, badResp.Body.String())
	}
	assertNotContains(t, badResp.Body.String(), "correct-password")
	goodResp := performJSONRequest(t, server, http.MethodPost, "/v1/auth/login", map[string]string{"username": "admin", "password": "correct-password"}, "")
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
