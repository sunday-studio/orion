package api

import (
	"fmt"
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/datatypes"
)

func seedAgentReports(t *testing.T, server *Server, agentID string, n int, disk float64) {
	t.Helper()
	now := time.Now().UTC()
	for i := 0; i < n; i++ {
		at := now.Add(-time.Duration(n-1-i) * time.Minute) // oldest first in time
		server.db.Create(&db.AgentReport{ID: fmt.Sprintf("rep-%s-%02d", agentID, i), AgentID: agentID, CreatedAt: at, AgentVersion: "0.9.2", UptimeSeconds: 41 * 86400, Timestamp: at.Format(time.RFC3339),
			CPU: datatypes.NewJSONType(db.CPUStats{UsagePercent: float64(10 + i)}), Memory: datatypes.NewJSONType(db.MemoryStats{UsedPercent: 61}), Disk: datatypes.NewJSONType(db.DiskStats{UsedPercent: disk})})
	}
	server.db.Model(&db.Agent{}).Where("id = ?", agentID).Update("last_seen", now)
}

type agentEnvelope struct {
	Data struct {
		Agent AgentResponse `json:"agent"`
	} `json:"data"`
}

func TestServerListAndDetailCarryMetricsLabelsAndStatusText(t *testing.T) {
	server := setupTestServer(t)
	agent := registerTestAgent(t, server)
	id := agent.Data.AgentID
	seedAgentReports(t, server, id, 15, 91)

	var list struct {
		Data struct {
			Agents []AgentResponse `json:"agents"`
		} `json:"data"`
	}
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/agents", nil, ""), &list)
	if len(list.Data.Agents) != 1 {
		t.Fatalf("agents = %d", len(list.Data.Agents))
	}
	a := list.Data.Agents[0]
	if a.CPUPercent == nil || *a.CPUPercent != 24 || a.MemoryPercent == nil || *a.MemoryPercent != 61 || len(a.CPUSeries) != 12 || a.CPUSeries[0] != 13 || a.CPUSeries[11] != 24 ||
		a.AgentVersion != "0.9.2" || a.LastReportAt == nil || a.StatusDetail != "Disk 91%" || a.StatusText == "" || a.Labels == nil {
		t.Fatalf("list row = %+v", a)
	}

	// PATCH: valid, then invalid.
	resp := performJSONRequest(t, server, http.MethodPatch, "/v1/agents/"+id, gin.H{"name": "edge-01", "labels": []string{"env=prod", "role=edge", "env=prod"}, "notes": "Rack 4, owned by infra"}, "")
	var patched agentEnvelope
	decodeResponse(t, resp, &patched)
	if resp.Code != http.StatusOK || patched.Data.Agent.Name != "edge-01" || len(patched.Data.Agent.Labels) != 2 || patched.Data.Agent.Notes != "Rack 4, owned by infra" || patched.Data.Agent.CPUPercent == nil {
		t.Fatalf("patch = %d %+v", resp.Code, patched.Data.Agent)
	}
	for name, body := range map[string]gin.H{
		"empty name":   {"name": "  "},
		"bad label":    {"labels": []string{"no equals"}},
		"label inject": {"labels": []string{"a=b\nc=d"}},
		"too long":     {"notes": strings.Repeat("x", 2001)},
	} {
		if r := performJSONRequest(t, server, http.MethodPatch, "/v1/agents/"+id, body, ""); r.Code != http.StatusBadRequest {
			t.Errorf("%s = %d, want 400", name, r.Code)
		}
	}
	if r := performJSONRequest(t, server, http.MethodPatch, "/v1/agents/nope", gin.H{"name": "x"}, ""); r.Code != http.StatusNotFound {
		t.Errorf("unknown agent = %d", r.Code)
	}
	var detail agentEnvelope
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/agents/"+id, nil, ""), &detail)
	if detail.Data.Agent.Name != "edge-01" || len(detail.Data.Agent.Labels) != 2 || detail.Data.Agent.DiskPercent == nil || *detail.Data.Agent.DiskPercent != 91 || detail.Data.Agent.StatusDetail != "Disk 91%" {
		t.Fatalf("detail = %+v", detail.Data.Agent)
	}

	// Reports since/until.
	type reportsPage struct {
		Data struct {
			Reports []AgentReportResponse `json:"reports"`
			Count   int64                 `json:"count"`
		} `json:"data"`
	}
	var last5 reportsPage
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/agents/"+id+"/reports?since=300s&limit=2", nil, ""), &last5)
	if last5.Data.Count != 5 && last5.Data.Count != 6 || len(last5.Data.Reports) != 2 {
		t.Fatalf("since=300s count=%d len=%d", last5.Data.Count, len(last5.Data.Reports))
	}
	if r := performJSONRequest(t, server, http.MethodGet, "/v1/agents/"+id+"/reports?since=soon", nil, ""); r.Code != http.StatusBadRequest {
		t.Errorf("bad since = %d", r.Code)
	}
}

func TestMonitorStatusFilterHiddenCountAndStatusText(t *testing.T) {
	server := setupTestServer(t)
	agent := registerTestAgent(t, server)
	id := agent.Data.AgentID
	now := time.Now().UTC()
	seedFailingMonitor(t, server, id) // mon-fail: down, reports say "timeout after 10s"
	server.db.Model(&db.Monitor{}).Where("id = ?", "mon-fail").Updates(map[string]any{"agent_id": id, "reporting_interval_seconds": 3600})
	for i := 0; i < 3; i++ {
		server.db.Create(&db.Monitor{ID: fmt.Sprintf("mon-ok-%d", i), Type: "http", Name: fmt.Sprintf("ok-%d", i), AgentID: id, Lifecycle: "active", ComputedHealth: "up", Health: "up", ReportingIntervalSeconds: 3600, CreatedAt: now})
	}
	var out struct {
		Data struct {
			Monitors    []MonitorResponse `json:"monitors"`
			Count       int64             `json:"count"`
			Total       int64             `json:"total"`
			HiddenCount int64             `json:"hidden_count"`
		} `json:"data"`
	}
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/agents/"+id+"/monitors?status=down,degraded,stale", nil, ""), &out)
	if out.Data.Count != 1 || out.Data.Total != 4 || out.Data.HiddenCount != 3 || out.Data.Monitors[0].ID != "mon-fail" ||
		!strings.HasPrefix(out.Data.Monitors[0].StatusText, "Down ") || out.Data.Monitors[0].StatusDetail != "timeout after 10s" {
		t.Fatalf("status filter = %+v", out.Data)
	}
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/agents/"+id+"/monitors", nil, ""), &out)
	if out.Data.Count != 4 || out.Data.HiddenCount != 0 {
		t.Fatalf("unfiltered = %+v", out.Data)
	}
}

func TestDecommissionKeepsOrDeletesHistoryAndFreesTheMachine(t *testing.T) {
	for _, keep := range []bool{true, false} {
		t.Run(fmt.Sprintf("keep_history=%v", keep), func(t *testing.T) {
			server := setupTestServer(t)
			agent := registerTestAgent(t, server)
			id := agent.Data.AgentID
			seedAgentReports(t, server, id, 3, 40)
			server.db.Create(&db.Monitor{ID: "m1", Type: "http", Name: "api-https", AgentID: id, Lifecycle: "active", ComputedHealth: "down", Health: "down"})
			server.db.Create(&db.MonitorReport{ID: "mr1", MonitorID: "m1", Payload: "{}", CollectedAt: time.Now().Format(time.RFC3339), Health: "down", CreatedAt: time.Now().UTC()})
			server.db.Create(&db.ServiceLogEntry{ID: "sl1", AgentID: id, CreatedAt: time.Now().UTC()})
			server.db.Create(&db.AlertDelivery{ID: "d-pending", IncidentID: "inc1", EventType: "incident_opened", Channel: "c", Type: "webhook", Status: "pending"})
			server.db.Create(&db.Incident{ID: "inc1", Number: 1, Status: "open", Severity: "high", Title: "x", AgentID: id, MonitorID: "m1", OpenedAt: time.Now().UTC(), LastEventAt: time.Now().UTC(), Source: "monitor"})

			resp := performJSONRequest(t, server, http.MethodDelete, fmt.Sprintf("/v1/agents/%s?keep_history=%v", id, keep), nil, "")
			if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), `"monitors_removed":1`) || !strings.Contains(resp.Body.String(), `"incidents_closed":1`) {
				t.Fatalf("delete = %d %s", resp.Code, resp.Body.String())
			}
			if r := performJSONRequest(t, server, http.MethodGet, "/v1/agents/"+id, nil, ""); r.Code != http.StatusNotFound {
				t.Fatalf("removed server still readable: %d", r.Code)
			}
			if body := performJSONRequest(t, server, http.MethodGet, "/v1/agents", nil, "").Body.String(); strings.Contains(body, id) {
				t.Fatal("removed server still listed")
			}
			if r := performJSONRequest(t, server, http.MethodDelete, "/v1/agents/"+id, nil, ""); r.Code != http.StatusNotFound {
				t.Fatalf("second delete = %d", r.Code)
			}
			var ov struct{ Data OverviewResponse }
			decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/overview", nil, ""), &ov)
			if ov.Data.Stats.ServersTotal != 0 || ov.Data.FirstRun.HasServers || len(ov.Data.Services) != 0 {
				t.Fatalf("removed server still on the overview: %+v", ov.Data)
			}
			if body := performJSONRequest(t, server, http.MethodGet, "/v1/search?q=test-server", nil, "").Body.String(); strings.Contains(body, id) {
				t.Fatalf("removed server still searchable: %s", body)
			}
			var delivery db.AlertDelivery
			server.db.First(&delivery, "id = ?", "d-pending")
			var resolvedEvents int64
			server.db.Model(&db.IncidentEvent{}).Where("incident_id = ? AND type = ?", "inc1", "incident_resolved").Count(&resolvedEvents)
			if delivery.Status != "suppressed" || resolvedEvents != 1 {
				t.Fatalf("pending delivery %q, resolve events %d", delivery.Status, resolvedEvents)
			}
			var reports, monitorReports int64
			server.db.Model(&db.AgentReport{}).Where("agent_id = ?", id).Count(&reports)
			server.db.Model(&db.MonitorReport{}).Where("monitor_id = ?", "m1").Count(&monitorReports)
			var logs int64
			server.db.Model(&db.ServiceLogEntry{}).Where("agent_id = ?", id).Count(&logs)
			if keep && (reports != 3 || monitorReports != 1 || logs != 1) || !keep && (reports != 0 || monitorReports != 0 || logs != 0) {
				t.Fatalf("keep=%v reports=%d monitor_reports=%d logs=%d", keep, reports, monitorReports, logs)
			}
			var incident db.Incident
			server.db.First(&incident, "id = ?", "inc1")
			if incident.Status != "resolved" || incident.ResolutionKind != "decommissioned" {
				t.Fatalf("incident = %+v", incident)
			}
			// The old token no longer works, and the same host can be added again.
			if _, err := service.NewAgentService(server.db, server.logger).ValidateAgentToken(id, agent.Data.Token); err == nil {
				t.Fatal("token still valid after removal")
			}
			again := registerTestAgent(t, server)
			if again.Data.AgentID == id {
				t.Fatal("re-registration reused the removed server")
			}
			// Kept history is purged once it is 90 days old.
			if keep {
				server.db.Model(&db.Agent{}).Where("id = ?", id).Update("deleted_at", time.Now().UTC().AddDate(0, 0, -91))
				if n, err := service.PurgeDecommissionedAgents(server.db, server.logger, time.Now().UTC()); err != nil || n != 1 {
					t.Fatalf("purge = %d, %v", n, err)
				}
				server.db.Model(&db.AgentReport{}).Where("agent_id = ?", id).Count(&reports)
				if reports != 0 {
					t.Fatalf("reports after purge = %d", reports)
				}
			}
		})
	}
}

func TestTokenRotationKeepsTheOldTokenForTenMinutes(t *testing.T) {
	server := setupTestServer(t)
	agent := registerTestAgent(t, server)
	id := agent.Data.AgentID
	svc := service.NewAgentService(server.db, server.logger)

	rotated, err := svc.RotateAgentToken(id, service.AgentTokenActionInput{ActorID: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ValidateAgentToken(id, agent.Data.Token); err != nil {
		t.Fatalf("old token should still work during the grace window: %v", err)
	}
	if _, err := svc.ValidateAgentToken(id, rotated.Token); err != nil {
		t.Fatalf("new token: %v", err)
	}
	var status struct {
		Data AgentTokenStatusResponse `json:"data"`
	}
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/agents/"+id+"/token/status", nil, ""), &status)
	if status.Data.PreviousTokenValidUntil == nil || status.Data.LastUsedAt == nil || time.Until(*status.Data.PreviousTokenValidUntil) > 10*time.Minute {
		t.Fatalf("token status = %+v", status.Data)
	}
	server.db.Model(&db.Agent{}).Where("id = ?", id).Update("previous_token_expires_at", time.Now().UTC().Add(-time.Second))
	if _, err := svc.ValidateAgentToken(id, agent.Data.Token); err == nil {
		t.Fatal("old token must stop working after the grace window")
	}
	// A second rotation drops the first token immediately and starts a new grace for the second.
	third, _ := svc.RotateAgentToken(id, service.AgentTokenActionInput{ActorID: "admin"})
	if _, err := svc.ValidateAgentToken(id, rotated.Token); err != nil {
		t.Fatalf("previous token should be in grace: %v", err)
	}
	if _, err := svc.ValidateAgentToken(id, agent.Data.Token); err == nil {
		t.Fatal("the first token must not come back")
	}
	_ = third
	// Revoking kills both.
	if _, err := svc.RevokeAgentToken(id, service.AgentTokenActionInput{ActorID: "admin", Reason: "test"}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ValidateAgentToken(id, rotated.Token); err == nil {
		t.Fatal("revoke must also end the grace window")
	}
}
