package api

import (
	"net/http"
	"orion/core/internal/db"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

type incidentEnvelope struct {
	Data struct {
		Incident IncidentResponse `json:"incident"`
	} `json:"data"`
}

func declare(t *testing.T, server *Server, body gin.H) (int, incidentEnvelope) {
	t.Helper()
	resp := performJSONRequest(t, server, http.MethodPost, "/v1/incidents", body, "")
	var out incidentEnvelope
	if resp.Code == http.StatusCreated {
		decodeResponse(t, resp, &out)
	}
	return resp.Code, out
}

func TestDeclareIncidentAssignsNumbersAndValidates(t *testing.T) {
	server := setupTestServer(t)
	agent := registerTestAgent(t, server)
	server.db.Create(&db.Monitor{ID: "mon-1", Type: "http", Name: "checkout-https", AgentID: agent.Data.AgentID, Lifecycle: "active", ComputedHealth: "up", Health: "up"})

	code, first := declare(t, server, gin.H{"title": "Checkout is slow for EU customers", "severity": "high", "monitor_id": "mon-1", "summary": "p95 up", "notify": false})
	if code != http.StatusCreated || first.Data.Incident.DisplayID != "INC-0001" || first.Data.Incident.Source != "manual" || first.Data.Incident.Status != "open" || first.Data.Incident.MonitorName != "checkout-https" {
		t.Fatalf("declare = %d %+v", code, first.Data.Incident)
	}
	if code, _ := declare(t, server, gin.H{"title": "again", "severity": "low", "monitor_id": "mon-1"}); code != http.StatusConflict {
		t.Fatalf("second incident on the same monitor = %d, want 409", code)
	}
	code, second := declare(t, server, gin.H{"title": "Whole server flaky", "severity": "critical", "agent_id": agent.Data.AgentID})
	if code != http.StatusCreated || second.Data.Incident.DisplayID != "INC-0002" || second.Data.Incident.MonitorID != "" {
		t.Fatalf("agent-level declare = %d %+v", code, second.Data.Incident)
	}
	for name, body := range map[string]gin.H{
		"no title":     {"severity": "low", "agent_id": agent.Data.AgentID},
		"bad severity": {"title": "x", "severity": "urgent", "agent_id": agent.Data.AgentID},
		"no target":    {"title": "x", "severity": "low"},
	} {
		if code, _ := declare(t, server, body); code != http.StatusBadRequest {
			t.Errorf("%s = %d, want 400", name, code)
		}
	}
	if code, _ := declare(t, server, gin.H{"title": "x", "severity": "low", "monitor_id": "nope"}); code != http.StatusNotFound {
		t.Errorf("unknown monitor = %d, want 404", code)
	}
	// notify:true queues an alert delivery.
	server.db.Create(&db.AlertChannel{ID: "c1", Name: "hook", Type: "webhook", Enabled: true, WebhookURL: "https://example.com/h", SubscribedEvents: db.EncodeAlertEvents(db.DefaultAlertEvents())})
	if code, third := declare(t, server, gin.H{"title": "loud", "severity": "high", "agent_id": agent.Data.AgentID, "notify": true}); code != http.StatusCreated {
		t.Fatalf("notify declare = %d", code)
	} else {
		var n int64
		server.db.Model(&db.AlertDelivery{}).Where("incident_id = ?", third.Data.Incident.ID).Count(&n)
		if n == 0 {
			t.Fatal("notify=true queued no delivery")
		}
	}
}

func TestIncidentNotesResponderCoverByAndBulk(t *testing.T) {
	server := setupTestServer(t)
	agent := registerTestAgent(t, server)
	_, a := declare(t, server, gin.H{"title": "A", "severity": "high", "agent_id": agent.Data.AgentID})
	_, b := declare(t, server, gin.H{"title": "B", "severity": "low", "agent_id": agent.Data.AgentID})
	_, cinc := declare(t, server, gin.H{"title": "C", "severity": "medium", "agent_id": agent.Data.AgentID})

	if r := performJSONRequest(t, server, http.MethodPost, "/v1/incidents/"+a.Data.Incident.ID+"/notes", gin.H{"text": "Restarting the load balancer"}, ""); r.Code != http.StatusCreated {
		t.Fatalf("note = %d %s", r.Code, r.Body.String())
	}
	if r := performJSONRequest(t, server, http.MethodPost, "/v1/incidents/"+a.Data.Incident.ID+"/notes", gin.H{"text": "  "}, ""); r.Code != http.StatusBadRequest {
		t.Fatalf("blank note = %d", r.Code)
	}
	if r := performJSONRequest(t, server, http.MethodPut, "/v1/incidents/"+a.Data.Incident.ID+"/responder", gin.H{"name": "Dana"}, ""); r.Code != http.StatusOK || !strings.Contains(r.Body.String(), `"responder":"Dana"`) {
		t.Fatalf("responder = %d %s", r.Code, r.Body.String())
	}
	var detail struct {
		Data struct {
			Summary  IncidentDetailSummary          `json:"summary"`
			Timeline []IncidentTimelineItemResponse `json:"timeline"`
		} `json:"data"`
	}
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/incidents/"+a.Data.Incident.ID, nil, ""), &detail)
	foundNote := false
	for _, e := range detail.Data.Timeline {
		if e.Type == "incident_note" && e.Note == "Restarting the load balancer" && e.ActorType == "user" {
			foundNote = true
		}
	}
	if !foundNote || detail.Data.Summary.DetectedAt.IsZero() || detail.Data.Summary.TimeToAcknowledgeSeconds != nil || detail.Data.Summary.Source.ServerName != "test-server" {
		t.Fatalf("detail = %+v", detail.Data)
	}

	// Cover B by INC number; self-cover and unknown refs are rejected.
	if r := performJSONRequest(t, server, http.MethodPost, "/v1/incidents/"+b.Data.Incident.ID+"/cover", gin.H{"covered_by": "INC-0001"}, ""); r.Code != http.StatusOK || !strings.Contains(r.Body.String(), a.Data.Incident.ID) {
		t.Fatalf("cover by = %d %s", r.Code, r.Body.String())
	}
	if r := performJSONRequest(t, server, http.MethodPost, "/v1/incidents/"+cinc.Data.Incident.ID+"/cover", gin.H{"covered_by": cinc.Data.Incident.ID}, ""); r.Code != http.StatusBadRequest {
		t.Fatalf("self cover = %d", r.Code)
	}
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/incidents/"+b.Data.Incident.ID, nil, ""), &detail)
	if detail.Data.Summary.CoveredByDisplayID != "INC-0001" {
		t.Fatalf("covered_by_display_id = %q", detail.Data.Summary.CoveredByDisplayID)
	}

	// Bulk: acknowledge a + c, plus an unknown id that fails alone.
	r := performJSONRequest(t, server, http.MethodPost, "/v1/incidents/bulk", gin.H{"ids": []string{a.Data.Incident.ID, cinc.Data.Incident.ID, "missing"}, "action": "acknowledge"}, "")
	var bulk struct {
		Data struct {
			Results   []BulkIncidentResult `json:"results"`
			Succeeded int                  `json:"succeeded"`
			Failed    int                  `json:"failed"`
		} `json:"data"`
	}
	decodeResponse(t, r, &bulk)
	if r.Code != http.StatusOK || bulk.Data.Succeeded != 2 || bulk.Data.Failed != 1 || bulk.Data.Results[2].OK || bulk.Data.Results[0].Incident.Status != "acknowledged" {
		t.Fatalf("bulk ack = %d %+v", r.Code, bulk.Data)
	}
	decodeResponse(t, performJSONRequest(t, server, http.MethodPost, "/v1/incidents/bulk", gin.H{"ids": []string{a.Data.Incident.ID, cinc.Data.Incident.ID}, "action": "resolve", "note": "fixed"}, ""), &bulk)
	if bulk.Data.Succeeded != 2 {
		t.Fatalf("bulk resolve = %+v", bulk.Data)
	}
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/incidents/"+a.Data.Incident.ID, nil, ""), &detail)
	if detail.Data.Summary.TimeToAcknowledgeSeconds == nil || detail.Data.Summary.TimeToResolveSeconds == nil || detail.Data.Summary.ResolvedAt == nil {
		t.Fatalf("resolved summary = %+v", detail.Data.Summary)
	}
	for name, body := range map[string]gin.H{"empty ids": {"ids": []string{}, "action": "resolve"}, "bad action": {"ids": []string{"x"}, "action": "delete"}} {
		if r := performJSONRequest(t, server, http.MethodPost, "/v1/incidents/bulk", body, ""); r.Code != http.StatusBadRequest {
			t.Errorf("%s = %d", name, r.Code)
		}
	}
}

func TestIncidentListFiltersHiddenCountAndExport(t *testing.T) {
	server := setupTestServer(t)
	agent := registerTestAgent(t, server)
	now := time.Now().UTC()
	mk := func(id string, n int, status, sev, title string, opened time.Time) {
		server.db.Create(&db.Incident{ID: id, Number: n, Status: status, Severity: sev, Title: title, AgentID: agent.Data.AgentID, MonitorID: "", OpenedAt: opened, LastEventAt: opened, Source: "monitor"})
	}
	mk("i1", 1, "open", "critical", "api.example.com unreachable", now.Add(-30*time.Minute))
	mk("i2", 2, "acknowledged", "high", "db-02 disk above 90%", now.Add(-16*time.Hour))
	mk("i3", 3, "resolved", "low", "old blip", now.Add(-10*24*time.Hour))
	mk("i4", 4, "resolved", "high", "=SUM(A1) attack", now.Add(-2*time.Hour))

	type list struct {
		Data struct {
			Incidents []IncidentResponse `json:"incidents"`
			Hidden    int64              `json:"hidden_resolved_count"`
			Count     int64              `json:"count"`
		} `json:"data"`
	}
	get := func(q string) list {
		var l list
		resp := performJSONRequest(t, server, http.MethodGet, "/v1/incidents"+q, nil, "")
		if resp.Code != http.StatusOK {
			t.Fatalf("%s = %d %s", q, resp.Code, resp.Body.String())
		}
		decodeResponse(t, resp, &l)
		return l
	}
	if l := get(""); l.Data.Count != 2 || l.Data.Hidden != 2 || l.Data.Incidents[0].DisplayID == "" {
		t.Fatalf("default list = %+v", l.Data)
	}
	if l := get("?severity=critical"); l.Data.Count != 1 || l.Data.Incidents[0].ID != "i1" {
		t.Fatalf("severity filter = %+v", l.Data)
	}
	if l := get("?q=disk"); l.Data.Count != 1 || l.Data.Incidents[0].ID != "i2" {
		t.Fatalf("q title = %+v", l.Data)
	}
	if l := get("?q=INC-0001"); l.Data.Count != 1 || l.Data.Incidents[0].ID != "i1" {
		t.Fatalf("q number = %+v", l.Data)
	}
	if l := get("?q=_"); l.Data.Count != 0 {
		t.Fatalf("wildcard must be literal: %+v", l.Data)
	}
	if l := get("?opened_since=1h"); l.Data.Count != 1 {
		t.Fatalf("opened_since=1h = %+v", l.Data)
	}
	if l := get("?status=resolved&opened_since=7d"); l.Data.Count != 1 || l.Data.Incidents[0].ID != "i4" || l.Data.Hidden != 0 {
		t.Fatalf("resolved last 7d = %+v", l.Data)
	}
	for _, bad := range []string{"?severity=urgent", "?opened_since=soon", "?opened_until=x"} {
		if r := performJSONRequest(t, server, http.MethodGet, "/v1/incidents"+bad, nil, ""); r.Code != http.StatusBadRequest {
			t.Errorf("%s = %d, want 400", bad, r.Code)
		}
	}

	resp := performRawRequest(t, server, http.MethodGet, "/v1/incidents/export?status=open,acknowledged,resolved&severity=high,critical", nil, "")
	body := resp.Body.String()
	if resp.Code != http.StatusOK || !strings.HasPrefix(resp.Header().Get("Content-Type"), "text/csv") || !strings.HasPrefix(body, "id,title,status,severity") ||
		!strings.Contains(body, "INC-0001,api.example.com unreachable") || !strings.Contains(body, "'=SUM(A1) attack") || strings.Contains(body, "old blip") {
		t.Fatalf("export = %d %s", resp.Code, body)
	}
}
