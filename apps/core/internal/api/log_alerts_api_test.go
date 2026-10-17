package api

import (
	"encoding/json"
	"net/http"
	"orion/core/internal/db"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestLogAlertRuleCRUDAndValidation(t *testing.T) {
	server := setupLogsServer(t, nil)
	appID, _ := createAppWithKey(t, server, "alerts-app")

	create := performJSONRequest(t, server, http.MethodPost, "/v1/logs/alerts", gin.H{"name": "errors", "app_id": appID, "severity": ">=error", "threshold": 20, "window_seconds": 300}, "")
	if create.Code != http.StatusCreated {
		t.Fatalf("create status = %d body = %s", create.Code, create.Body.String())
	}
	var created struct {
		Data struct {
			Rule LogAlertRule `json:"rule"`
		} `json:"data"`
	}
	json.Unmarshal(create.Body.Bytes(), &created)
	rule := created.Data.Rule
	if rule.Mode != "threshold" || !rule.Enabled || rule.State != "ok" || rule.Threshold != 20 {
		t.Fatalf("rule = %+v", rule)
	}

	for name, body := range map[string]gin.H{
		"no name":        {"name": " ", "app_id": appID},
		"unknown app":    {"name": "x", "app_id": "nope"},
		"bad mode":       {"name": "x", "app_id": appID, "mode": "vibes"},
		"tiny window":    {"name": "x", "app_id": appID, "window_seconds": 5},
		"huge threshold": {"name": "x", "app_id": appID, "threshold": 99999999},
		"bad severity":   {"name": "x", "app_id": appID, "severity": "catastrophic-ish"},
	} {
		if got := performJSONRequest(t, server, http.MethodPost, "/v1/logs/alerts", body, "").Code; got != http.StatusBadRequest {
			t.Errorf("%s: status = %d, want 400", name, got)
		}
	}

	patch := performJSONRequest(t, server, http.MethodPatch, "/v1/logs/alerts/"+rule.ID, gin.H{"threshold": 50, "event": "payment.failed"}, "")
	if patch.Code != http.StatusOK || !strings.Contains(patch.Body.String(), `"threshold":50`) || !strings.Contains(patch.Body.String(), "payment.failed") {
		t.Fatalf("patch = %d %s", patch.Code, patch.Body.String())
	}
	if got := performJSONRequest(t, server, http.MethodPatch, "/v1/logs/alerts/"+rule.ID, gin.H{"window_seconds": 1}, "").Code; got != http.StatusBadRequest {
		t.Errorf("invalid patch status = %d, want 400", got)
	}
	if got := performJSONRequest(t, server, http.MethodPatch, "/v1/logs/alerts/missing", gin.H{"threshold": 1}, "").Code; got != http.StatusNotFound {
		t.Errorf("patch missing status = %d, want 404", got)
	}
	list := performJSONRequest(t, server, http.MethodGet, "/v1/logs/alerts", nil, "")
	if !strings.Contains(list.Body.String(), rule.ID) {
		t.Fatalf("list = %s", list.Body.String())
	}
	if got := performJSONRequest(t, server, http.MethodDelete, "/v1/logs/alerts/"+rule.ID, nil, "").Code; got != http.StatusOK {
		t.Errorf("delete status = %d", got)
	}
	if got := performJSONRequest(t, server, http.MethodDelete, "/v1/logs/alerts/"+rule.ID, nil, "").Code; got != http.StatusNotFound {
		t.Errorf("second delete status = %d, want 404", got)
	}
}

func TestDisablingARuleResolvesItsIncident(t *testing.T) {
	server := setupLogsServer(t, nil)
	appID, _ := createAppWithKey(t, server, "disable-app")
	now := time.Now().UTC()
	server.db.Create(&db.LogAlertRule{ID: "r1", Name: "burst", AppID: appID, Mode: "threshold", WindowSeconds: 300, Enabled: true, State: "firing", MonitorID: "monitor-log-alert-r1", CreatedAt: now, UpdatedAt: now})
	owner := db.Agent{ID: "agent-core", MachineId: "core", Name: "Orion Core", OS: "core", Arch: "internal", Token: "t", LastSeen: now}
	server.db.Create(&owner)
	server.db.Create(&db.Monitor{ID: "monitor-log-alert-r1", AgentID: owner.ID, Name: "Log alert: burst", Type: "log-alert", Lifecycle: "active", Health: "down", ActiveIncidentID: "inc-1", IncidentState: "down", ReportingIntervalSeconds: 600, CreatedAt: now})
	server.db.Create(&db.Incident{ID: "inc-1", Status: "open", Severity: "high", Title: "t", AgentID: owner.ID, MonitorID: "monitor-log-alert-r1", OpenedAt: now, LastEventAt: now, NotificationStatus: "pending"})

	resp := performJSONRequest(t, server, http.MethodPatch, "/v1/logs/alerts/r1", gin.H{"enabled": false}, "")
	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), `"state":"ok"`) {
		t.Fatalf("disable = %d %s", resp.Code, resp.Body.String())
	}
	var incident db.Incident
	server.db.First(&incident, "id = ?", "inc-1")
	if incident.Status != "resolved" {
		t.Fatalf("incident status = %q, want resolved when its rule is disabled", incident.Status)
	}
}

func TestIncidentLogsReturnsTheWindowAroundAnIncident(t *testing.T) {
	server := setupLogsServer(t, nil)
	appID, key := createAppWithKey(t, server, "linked")
	agent := registerTestAgent(t, server)
	opened := time.Now().UTC().Add(-10 * time.Minute)
	resolved := opened.Add(5 * time.Minute)
	server.db.Create(&db.Incident{ID: "inc-x", Status: "resolved", Severity: "high", Title: "api down", AgentID: agent.Data.AgentID, MonitorID: "m-x", OpenedAt: opened, ResolvedAt: &resolved, LastEventAt: resolved, NotificationStatus: "sent"})

	ts := func(d time.Duration) string { return opened.Add(d).Format(time.RFC3339Nano) }
	ingest(server, key, "application/x-ndjson", strings.Join([]string{
		`{"body":"too early","timestamp":"` + ts(-40*time.Minute) + `"}`,
		`{"body":"before the incident","timestamp":"` + ts(-5*time.Minute) + `"}`,
		`{"body":"during","severity":"error","timestamp":"` + ts(2*time.Minute) + `"}`,
		`{"body":"just after","timestamp":"` + ts(8*time.Minute) + `"}`,
		`{"body":"way after","timestamp":"` + ts(30*time.Minute) + `"}`,
	}, "\n"))
	server.logs.store.Flush(3 * time.Second)
	server.db.Create(&db.ServiceLogEntry{ID: "sl-1", AgentID: agent.Data.AgentID, Level: "ERROR", Message: "agent saw trouble", Fingerprint: "f1", OccurredAt: opened.Add(time.Minute), CollectedAt: opened})

	resp := performJSONRequest(t, server, http.MethodGet, "/v1/incidents/inc-x/logs?app_id="+appID, nil, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", resp.Code, resp.Body.String())
	}
	body := resp.Body.String()
	for _, want := range []string{"before the incident", "during", "just after", "agent saw trouble"} {
		if !strings.Contains(body, want) {
			t.Errorf("incident logs missing %q", want)
		}
	}
	for _, unwanted := range []string{"too early", "way after"} {
		if strings.Contains(body, unwanted) {
			t.Errorf("incident logs include %q outside the window", unwanted)
		}
	}
	if strings.Index(body, "just after") > strings.Index(body, "before the incident") {
		t.Error("entries should be newest first")
	}
	if got := performJSONRequest(t, server, http.MethodGet, "/v1/incidents/nope/logs", nil, "").Code; got != http.StatusNotFound {
		t.Errorf("unknown incident status = %d, want 404", got)
	}
}
