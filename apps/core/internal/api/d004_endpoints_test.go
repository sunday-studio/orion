package api

import (
	"encoding/json"
	"net/http"
	"orion/core/internal/db"
	"testing"
	"time"
)

func createDelivery(t *testing.T, server *Server, id, status string, attempts int) {
	t.Helper()
	if err := server.db.Create(&db.AlertDelivery{ID: id, IncidentID: "inc-1", EventType: "incident_opened", Channel: "ops", Type: "webhook", Status: status, Error: "boom", AttemptCount: attempts, MaxAttempts: 3}).Error; err != nil {
		t.Fatal(err)
	}
}

func TestRetryAlertDeliveryRequeuesOnlyFailedDeliveries(t *testing.T) {
	server := setupTestServer(t)
	createDelivery(t, server, "d-failed", "failed", 3) // exhausted
	createDelivery(t, server, "d-sent", "sent", 1)
	createDelivery(t, server, "d-pending", "pending", 0)

	resp := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/deliveries/d-failed/retry", nil, "")
	if resp.Code != http.StatusAccepted {
		t.Fatalf("retry failed delivery status = %d, body = %s, want 202", resp.Code, resp.Body.String())
	}
	var got db.AlertDelivery
	server.db.First(&got, "id = ?", "d-failed")
	if got.Status != "pending" || got.MaxAttempts != 4 || got.NextAttemptAt != nil {
		t.Fatalf("after retry = status %q max %d next %v, want pending with one extra attempt available", got.Status, got.MaxAttempts, got.NextAttemptAt)
	}

	for _, id := range []string{"d-sent", "d-pending"} {
		if r := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/deliveries/"+id+"/retry", nil, ""); r.Code != http.StatusConflict {
			t.Fatalf("retry %s status = %d, want 409", id, r.Code)
		}
	}
	if r := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/deliveries/nope/retry", nil, ""); r.Code != http.StatusNotFound {
		t.Fatalf("retry unknown status = %d, want 404", r.Code)
	}
	// A second retry of the same (now pending) delivery is rejected, not double-queued.
	if r := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/deliveries/d-failed/retry", nil, ""); r.Code != http.StatusConflict {
		t.Fatalf("second retry status = %d, want 409", r.Code)
	}
}

func candidateIDs(t *testing.T, server *Server) map[string]bool {
	t.Helper()
	resp := performJSONRequest(t, server, http.MethodGet, "/v1/incidents/candidates", nil, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("candidates status = %d, body = %s", resp.Code, resp.Body.String())
	}
	var body struct {
		Data struct {
			Candidates []struct {
				MonitorID string `json:"monitor_id"`
			} `json:"candidates"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, c := range body.Data.Candidates {
		ids[c.MonitorID] = true
	}
	return ids
}

func TestDismissIncidentCandidateHidesUntilMonitorRecovers(t *testing.T) {
	server := setupTestServer(t)
	agent := registerTestAgent(t, server)
	now := time.Now().UTC()
	monitor := db.Monitor{ID: "monitor-cand", AgentID: agent.Data.AgentID, Name: "api", Type: "http", Lifecycle: "active", Health: "down", ComputedHealth: "", IncidentState: "unknown", ReportingIntervalSeconds: 60}
	if err := server.db.Create(&monitor).Error; err != nil {
		t.Fatal(err)
	}
	server.db.Create(&db.MonitorReport{ID: "r1", MonitorID: monitor.ID, Payload: "{}", CollectedAt: now.Format(time.RFC3339), Health: "down", CreatedAt: now})

	if !candidateIDs(t, server)[monitor.ID] {
		t.Fatal("failing monitor should be a candidate before dismissal")
	}
	if r := performJSONRequest(t, server, http.MethodPost, "/v1/incidents/candidates/"+monitor.ID+"/dismiss", map[string]any{"reason": "known maintenance"}, ""); r.Code != http.StatusOK {
		t.Fatalf("dismiss status = %d, body = %s", r.Code, r.Body.String())
	}
	if candidateIDs(t, server)[monitor.ID] {
		t.Fatal("dismissed candidate still listed")
	}

	// Monitor recovers, then fails again: the candidate reappears.
	later := time.Now().UTC().Add(time.Second)
	server.db.Create(&db.MonitorReport{ID: "r2", MonitorID: monitor.ID, Payload: "{}", CollectedAt: later.Format(time.RFC3339), Health: "up", CreatedAt: later})
	later2 := later.Add(time.Second)
	server.db.Create(&db.MonitorReport{ID: "r3", MonitorID: monitor.ID, Payload: "{}", CollectedAt: later2.Format(time.RFC3339), Health: "down", CreatedAt: later2})
	server.db.Model(&db.Monitor{}).Where("id = ?", monitor.ID).Updates(map[string]any{"computed_health": "", "last_health_computation": nil})
	if !candidateIDs(t, server)[monitor.ID] {
		t.Fatal("candidate should reappear after the monitor recovered and failed again")
	}

	if r := performJSONRequest(t, server, http.MethodPost, "/v1/incidents/candidates/missing/dismiss", nil, ""); r.Code != http.StatusNotFound {
		t.Fatalf("dismiss unknown status = %d, want 404", r.Code)
	}
}
