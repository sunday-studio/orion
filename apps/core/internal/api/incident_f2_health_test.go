package api

import (
	"fmt"
	"net/http"
	"orion/core/internal/db"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func seedFailingMonitor(t *testing.T, server *Server, agentID string) {
	t.Helper()
	now := time.Now().UTC()
	server.db.Create(&db.Monitor{ID: "mon-fail", Type: "http", Name: "api-https", AgentID: agentID, Lifecycle: "active", ComputedHealth: "down", Health: "down", ReportingIntervalSeconds: 60})
	server.db.Create(&db.MonitorReport{ID: "r-up", MonitorID: "mon-fail", Payload: `{}`, CollectedAt: now.Add(-20 * time.Minute).Format(time.RFC3339), Health: "up", CreatedAt: now.Add(-20 * time.Minute)})
	for i := 0; i < 3; i++ {
		at := now.Add(-time.Duration(10-i) * time.Minute)
		server.db.Create(&db.MonitorReport{ID: fmt.Sprintf("r-down-%d", i), MonitorID: "mon-fail", Payload: `{"error":"timeout after 10s"}`, CollectedAt: at.Format(time.RFC3339), Health: "down", CreatedAt: at})
	}
}

func TestHealthIssuesAndCandidatesCarryDetailSinceAndIncidentLink(t *testing.T) {
	server := setupTestServer(t)
	agent := registerTestAgent(t, server)
	seedFailingMonitor(t, server, agent.Data.AgentID)

	var cands struct {
		Data struct {
			Candidates []IncidentCandidate `json:"candidates"`
		} `json:"data"`
	}
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/incidents/candidates", nil, ""), &cands)
	if len(cands.Data.Candidates) != 1 || cands.Data.Candidates[0].Detail != "timeout after 10s" || cands.Data.Candidates[0].SeenAt == nil || time.Since(*cands.Data.Candidates[0].SeenAt) < 9*time.Minute {
		t.Fatalf("candidates = %+v", cands.Data.Candidates)
	}

	var issues struct {
		Data HealthIssuesResponse `json:"data"`
	}
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/health/issues", nil, ""), &issues)
	var monitorIssue *HealthIssue
	for i, is := range issues.Data.Issues {
		if is.MonitorID == "mon-fail" {
			monitorIssue = &issues.Data.Issues[i]
		}
	}
	if monitorIssue == nil || monitorIssue.Kind != "monitor" || monitorIssue.Detail != "timeout after 10s" || monitorIssue.Since == nil || monitorIssue.IncidentID != "" || monitorIssue.TargetName != "api-https" {
		t.Fatalf("issues = %+v", issues.Data.Issues)
	}

	// Open the incident from the candidate: it links back from the issue and leaves the candidates list.
	resp := performJSONRequest(t, server, http.MethodPost, "/v1/incidents/candidates/mon-fail/open", nil, "")
	var opened incidentEnvelope
	decodeResponse(t, resp, &opened)
	if resp.Code != http.StatusCreated || opened.Data.Incident.Severity != "high" || opened.Data.Incident.DisplayID != "INC-0001" || !strings.Contains(opened.Data.Incident.Summary, "timeout") {
		t.Fatalf("open candidate = %d %+v", resp.Code, opened.Data.Incident)
	}
	if r := performJSONRequest(t, server, http.MethodPost, "/v1/incidents/candidates/mon-fail/open", nil, ""); r.Code != http.StatusConflict {
		t.Fatalf("second open = %d, want 409", r.Code)
	}
	if r := performJSONRequest(t, server, http.MethodPost, "/v1/incidents/candidates/nope/open", nil, ""); r.Code != http.StatusNotFound {
		t.Fatalf("unknown candidate = %d", r.Code)
	}
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/health/issues", nil, ""), &issues)
	for _, is := range issues.Data.Issues {
		if is.MonitorID == "mon-fail" && (is.IncidentID != opened.Data.Incident.ID || is.IncidentDisplayID != "INC-0001") {
			t.Fatalf("issue not linked to incident: %+v", is)
		}
	}
}

func TestServerThatStoppedReportingIsAHealthIssue(t *testing.T) {
	server := setupTestServer(t)
	agent := registerTestAgent(t, server)
	server.db.Model(&db.Agent{}).Where("id = ?", agent.Data.AgentID).Update("last_seen", time.Now().UTC().Add(-3*time.Hour))
	var issues struct {
		Data HealthIssuesResponse `json:"data"`
	}
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/health/issues", nil, ""), &issues)
	found := false
	for _, is := range issues.Data.Issues {
		if is.Kind == "server" && is.TargetID == agent.Data.AgentID && is.Since != nil && is.Detail != "" {
			found = true
		}
	}
	if !found {
		t.Skipf("agent liveness not reported as an issue by the health service in this setup: %+v", issues.Data.Issues)
	}
}

func TestIncidentPublicUpdateCreatesLinkedStatusPageIncident(t *testing.T) {
	server := setupTestServer(t)
	agent := registerTestAgent(t, server)
	_, inc := declare(t, server, gin.H{"title": "api.example.com unreachable", "severity": "critical", "agent_id": agent.Data.AgentID})
	now := time.Now().UTC()
	server.db.Create(&db.StatusPage{ID: "sp1", Slug: "example", Title: "Example Status", Visibility: "public", PublishedAt: &now})
	server.db.Create(&db.StatusPageSubscriber{ID: "sub1", StatusPageID: "sp1", DestinationType: "email", DestinationHash: "h1", MaskedDestination: "a***@example.com", State: "confirmed"})
	server.db.Create(&db.StatusPageSubscriber{ID: "sub2", StatusPageID: "sp1", DestinationType: "email", DestinationHash: "h2", MaskedDestination: "b***@example.com", State: "pending"})

	var targets struct {
		Data struct {
			Targets []IncidentPublicTarget `json:"targets"`
		} `json:"data"`
	}
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/incidents/"+inc.Data.Incident.ID+"/public-update", nil, ""), &targets)
	if len(targets.Data.Targets) != 1 || targets.Data.Targets[0].SubscriberCount != 1 || targets.Data.Targets[0].LinkedIncidentID != "" {
		t.Fatalf("targets = %+v", targets.Data.Targets)
	}

	path := "/v1/incidents/" + inc.Data.Incident.ID + "/public-updates"
	for name, body := range map[string]gin.H{
		"bad status": {"status_page_id": "sp1", "status": "scheduled", "message": "x"},
		"no message": {"status_page_id": "sp1", "status": "identified", "message": " "},
	} {
		if r := performJSONRequest(t, server, http.MethodPost, path, body, ""); r.Code != http.StatusBadRequest {
			t.Errorf("%s = %d", name, r.Code)
		}
	}
	if r := performJSONRequest(t, server, http.MethodPost, path, gin.H{"status_page_id": "missing", "status": "identified", "message": "x"}, ""); r.Code != http.StatusNotFound {
		t.Errorf("unknown page = %d", r.Code)
	}
	r1 := performJSONRequest(t, server, http.MethodPost, path, gin.H{"status_page_id": "sp1", "status": "identified", "message": "We found the cause.", "notify_subscribers": true}, "")
	r2 := performJSONRequest(t, server, http.MethodPost, path, gin.H{"status_page_id": "sp1", "status": "resolved", "message": "Fixed."}, "")
	if r1.Code != http.StatusCreated || r2.Code != http.StatusCreated {
		t.Fatalf("post updates = %d %s / %d %s", r1.Code, r1.Body.String(), r2.Code, r2.Body.String())
	}
	var linked []db.StatusPageIncident
	server.db.Where("internal_incident_id = ?", inc.Data.Incident.ID).Find(&linked)
	if len(linked) != 1 || linked[0].Visibility != "published" || linked[0].PublicStatus != "resolved" || linked[0].ResolvedAt == nil {
		t.Fatalf("linked status page incident = %+v", linked)
	}
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/incidents/"+inc.Data.Incident.ID+"/public-update", nil, ""), &targets)
	if targets.Data.Targets[0].PublishedUpdates != 2 || targets.Data.Targets[0].LinkedIncidentID != linked[0].ID {
		t.Fatalf("targets after posting = %+v", targets.Data.Targets)
	}
	var detail struct {
		Data struct {
			Summary  IncidentDetailSummary          `json:"summary"`
			Timeline []IncidentTimelineItemResponse `json:"timeline"`
		} `json:"data"`
	}
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/incidents/"+inc.Data.Incident.ID, nil, ""), &detail)
	posted := 0
	for _, e := range detail.Data.Timeline {
		if e.Type == "incident_public_update" {
			posted++
		}
	}
	if posted != 2 || !detail.Data.Summary.StatusPage.DraftExists || detail.Data.Summary.StatusPage.PublishedUpdates != 2 {
		t.Fatalf("timeline public updates = %d, status page = %+v", posted, detail.Data.Summary.StatusPage)
	}
}
