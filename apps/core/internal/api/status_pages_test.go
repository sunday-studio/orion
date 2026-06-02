package api

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"strings"
	"testing"
	"time"
)

func TestStatusPageAdminAPIRequiresFrontendJWTWhenConfigured(t *testing.T) {
	server := setupStatusPageAuthTestServer(t)
	unauthorized := performJSONRequest(t, server, http.MethodGet, "/v1/status-pages", nil, "")
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d, body = %s, want 401", unauthorized.Code, unauthorized.Body.String())
	}
	token := loginStatusPageTestAdmin(t, server)
	authorized := performJSONRequest(t, server, http.MethodGet, "/v1/status-pages", nil, token)
	if authorized.Code != http.StatusOK {
		t.Fatalf("authorized status = %d, body = %s", authorized.Code, authorized.Body.String())
	}
}
func TestPublicStatusRoutesStayUnauthenticatedWhenFrontendAuthConfigured(t *testing.T) {
	server := setupStatusPageAuthTestServer(t)
	publishedAt := time.Now().UTC()
	page := db.StatusPage{ID: "status-page-auth-boundary", Slug: "auth-boundary", Title: "Auth Boundary", Visibility: "public", ThemeSettings: "{}", DefaultIncidentVisibility: "draft", PublishedAt: &publishedAt}
	section := db.StatusPageSection{ID: "status-page-auth-boundary-section", StatusPageID: page.ID, Name: "Public"}
	component := db.StatusPageComponent{ID: "status-page-auth-boundary-component", StatusPageID: page.ID, SectionID: section.ID, PublicName: "Public API", DisplayMode: "manual", ManualStatus: "operational", Visible: true}
	if err := server.db.Create(&page).Error; err != nil {
		t.Fatalf("create status page: %v", err)
	}
	if err := server.db.Create(&section).Error; err != nil {
		t.Fatalf("create status page section: %v", err)
	}
	if err := server.db.Create(&component).Error; err != nil {
		t.Fatalf("create status page component: %v", err)
	}
	publicResp := performJSONRequest(t, server, http.MethodGet, "/status/auth-boundary", nil, "")
	if publicResp.Code != http.StatusOK {
		t.Fatalf("public status route status = %d, body = %s", publicResp.Code, publicResp.Body.String())
	}
	adminResp := performJSONRequest(t, server, http.MethodGet, "/v1/status-pages", nil, "")
	if adminResp.Code != http.StatusUnauthorized {
		t.Fatalf("admin status page route status = %d, body = %s, want 401", adminResp.Code, adminResp.Body.String())
	}
}
func TestStatusPageAuditEventsRecordActorAndMinimalFields(t *testing.T) {
	server := setupStatusPageAuthTestServer(t)
	token := loginStatusPageTestAdmin(t, server)
	registered := registerTestAgent(t, server)
	registeredMonitor := registerTestMonitor(t, server, registered.Data.AgentID, registered.Data.Token)
	createPageResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages", gin.H{"slug": "audit-status", "title": "Audit Status"}, token)
	if createPageResp.Code != http.StatusCreated {
		t.Fatalf("create status page status = %d, body = %s", createPageResp.Code, createPageResp.Body.String())
	}
	var createdPage struct {
		Data struct {
			Page struct {
				ID string `json:"id"`
			} `json:"page"`
		} `json:"data"`
	}
	decodeResponse(t, createPageResp, &createdPage)
	createSectionResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+createdPage.Data.Page.ID+"/sections", gin.H{"name": "API"}, token)
	if createSectionResp.Code != http.StatusCreated {
		t.Fatalf("create section status = %d, body = %s", createSectionResp.Code, createSectionResp.Body.String())
	}
	var createdSection struct {
		Data struct {
			Section struct {
				ID string `json:"id"`
			} `json:"section"`
		} `json:"data"`
	}
	decodeResponse(t, createSectionResp, &createdSection)
	createComponentResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+createdPage.Data.Page.ID+"/components", gin.H{"section_id": createdSection.Data.Section.ID, "public_name": "REST API", "visible": true}, token)
	if createComponentResp.Code != http.StatusCreated {
		t.Fatalf("create component status = %d, body = %s", createComponentResp.Code, createComponentResp.Body.String())
	}
	var createdComponent struct {
		Data struct {
			Component struct {
				ID string `json:"id"`
			} `json:"component"`
		} `json:"data"`
	}
	decodeResponse(t, createComponentResp, &createdComponent)
	createMappingResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+createdPage.Data.Page.ID+"/components/"+createdComponent.Data.Component.ID+"/mappings", gin.H{"resource_type": "monitor", "resource_id": registeredMonitor.Data.MonitorID, "health_rollup_strategy": "worst", "uptime_rollup_strategy": "worst"}, token)
	if createMappingResp.Code != http.StatusCreated {
		t.Fatalf("create mapping status = %d, body = %s", createMappingResp.Code, createMappingResp.Body.String())
	}
	var createdMapping struct {
		Data struct {
			Mapping struct {
				ID string `json:"id"`
			} `json:"mapping"`
		} `json:"data"`
	}
	decodeResponse(t, createMappingResp, &createdMapping)
	updateMappingResp := performJSONRequest(t, server, http.MethodPut, "/v1/status-pages/"+createdPage.Data.Page.ID+"/components/"+createdComponent.Data.Component.ID+"/mappings/"+createdMapping.Data.Mapping.ID, gin.H{"health_rollup_strategy": "average"}, token)
	if updateMappingResp.Code != http.StatusOK {
		t.Fatalf("update mapping status = %d, body = %s", updateMappingResp.Code, updateMappingResp.Body.String())
	}
	publishResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+createdPage.Data.Page.ID+"/publish", nil, token)
	if publishResp.Code != http.StatusOK {
		t.Fatalf("publish status = %d, body = %s", publishResp.Code, publishResp.Body.String())
	}
	now := time.Now().UTC()
	createIncidentResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+createdPage.Data.Page.ID+"/incidents", gin.H{"title": "Elevated API errors", "public_status": "investigating", "severity": "high", "impact_summary": "Customer-safe summary only", "visibility": "published", "affected_component_ids": []string{createdComponent.Data.Component.ID}, "published_at": now}, token)
	if createIncidentResp.Code != http.StatusCreated {
		t.Fatalf("create incident status = %d, body = %s", createIncidentResp.Code, createIncidentResp.Body.String())
	}
	var createdIncident struct {
		Data struct {
			Incident struct {
				ID string `json:"id"`
			} `json:"incident"`
		} `json:"data"`
	}
	decodeResponse(t, createIncidentResp, &createdIncident)
	updateIncidentResp := performJSONRequest(t, server, http.MethodPut, "/v1/status-pages/"+createdPage.Data.Page.ID+"/incidents/"+createdIncident.Data.Incident.ID, gin.H{"public_status": "identified"}, token)
	if updateIncidentResp.Code != http.StatusOK {
		t.Fatalf("update incident status = %d, body = %s", updateIncidentResp.Code, updateIncidentResp.Body.String())
	}
	rawTimelineMessage := "raw incident payload with internal secret token"
	createUpdateResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+createdPage.Data.Page.ID+"/incidents/"+createdIncident.Data.Incident.ID+"/updates", gin.H{"status": "resolved", "message": rawTimelineMessage, "created_by": "ops", "published_at": now}, token)
	if createUpdateResp.Code != http.StatusCreated {
		t.Fatalf("create incident update status = %d, body = %s", createUpdateResp.Code, createUpdateResp.Body.String())
	}
	unpublishResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+createdPage.Data.Page.ID+"/unpublish", nil, token)
	if unpublishResp.Code != http.StatusOK {
		t.Fatalf("unpublish status = %d, body = %s", unpublishResp.Code, unpublishResp.Body.String())
	}
	var events []db.AuditEvent
	if err := server.db.Order("created_at ASC").Find(&events).Error; err != nil {
		t.Fatalf("load audit events: %v", err)
	}
	actions := map[string]bool{}
	for _, event := range events {
		if event.StatusPageID != createdPage.Data.Page.ID {
			t.Fatalf("audit event status_page_id = %q, want %q", event.StatusPageID, createdPage.Data.Page.ID)
		}
		if event.ActorType != "user" || event.ActorID != "admin" {
			t.Fatalf("audit event actor = %s/%s, want user/admin", event.ActorType, event.ActorID)
		}
		if event.AffectedObjectType == "" || event.AffectedObjectID == "" || event.CreatedAt.IsZero() {
			t.Fatalf("audit event missing required field: %+v", event)
		}
		serializedFields := strings.Join([]string{event.Action, event.StatusPageID, event.AffectedObjectType, event.AffectedObjectID, event.ActorType, event.ActorID}, " ")
		if strings.Contains(serializedFields, rawTimelineMessage) {
			t.Fatalf("audit event stored raw incident update message: %+v", event)
		}
		actions[event.Action] = true
	}
	for _, action := range []string{service.StatusPageAuditActionComponentMappingCreated, service.StatusPageAuditActionComponentMappingUpdated, service.StatusPageAuditActionPublished, service.StatusPageAuditActionPublicIncidentCreated, service.StatusPageAuditActionPublicIncidentUpdated, service.StatusPageAuditActionPublicIncidentUpdateCreated, service.StatusPageAuditActionPublicIncidentResolved, service.StatusPageAuditActionUnpublished} {
		if !actions[action] {
			t.Fatalf("missing audit action %q in events: %+v", action, events)
		}
	}
}
func TestStatusPageIncidentComponentSuggestionsMatchMonitorAndRedactInternals(t *testing.T) {
	server := setupTestServer(t)
	now := time.Now().UTC()
	fixtures := createStatusPageSuggestionFixtures(t, server, now)
	incident := db.Incident{ID: "incident-monitor-secret", Status: "open", Severity: "critical", Title: "internal private monitor incident", AgentID: "unmapped-agent-for-monitor-test", MonitorID: fixtures.Monitor.ID, OpenedAt: now, LastEventAt: now, LatestEvent: "raw report payload should not leak", NotificationStatus: "pending", CreatedAt: now, UpdatedAt: now}
	if err := server.db.Create(&incident).Error; err != nil {
		t.Fatalf("create incident: %v", err)
	}
	if err := server.db.Create(&db.MonitorReport{ID: "report-monitor-secret", MonitorID: fixtures.Monitor.ID, Payload: `{"secret":"monitor-report-private-token"}`, CollectedAt: now.Format(time.RFC3339), Health: "down", CreatedAt: now}).Error; err != nil {
		t.Fatalf("create report: %v", err)
	}
	resp := performJSONRequest(t, server, http.MethodGet, "/v1/status-pages/"+fixtures.Page.ID+"/incidents/suggestions?incident_id="+incident.ID, nil, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("suggestions status = %d, body = %s", resp.Code, resp.Body.String())
	}
	body := resp.Body.String()
	for _, privateValue := range []string{incident.Title, incident.LatestEvent, fixtures.Agent.Name, fixtures.Agent.Token, fixtures.Monitor.Name, fixtures.Monitor.ID, fixtures.Agent.ID, "monitor-report-private-token", fixtures.HiddenComponent.PublicName} {
		if strings.Contains(body, privateValue) {
			t.Fatalf("suggestions leaked %q in %s", privateValue, body)
		}
	}
	var parsed struct {
		Data struct {
			Suggestions []StatusPageIncidentComponentSuggestionResponse `json:"suggestions"`
			Count       int                                             `json:"count"`
		} `json:"data"`
	}
	decodeResponse(t, resp, &parsed)
	if parsed.Data.Count != 1 || len(parsed.Data.Suggestions) != 1 {
		t.Fatalf("suggestions = %+v, want one public monitor component", parsed.Data.Suggestions)
	}
	suggestion := parsed.Data.Suggestions[0]
	if suggestion.ComponentID != fixtures.MonitorComponent.ID || suggestion.ComponentName != fixtures.MonitorComponent.PublicName {
		t.Fatalf("suggestion = %+v, want monitor component", suggestion)
	}
	if len(suggestion.Matches) != 1 || suggestion.Matches[0].ResourceType != "monitor" || !strings.Contains(suggestion.Matches[0].MatchReason, "monitor") {
		t.Fatalf("suggestion matches = %+v, want monitor match reason", suggestion.Matches)
	}
}
func TestStatusPageIncidentComponentSuggestionsMatchAgentAndRedactInternals(t *testing.T) {
	server := setupTestServer(t)
	now := time.Now().UTC()
	fixtures := createStatusPageSuggestionFixtures(t, server, now)
	incident := db.Incident{ID: "incident-agent-secret", Status: "open", Severity: "high", Title: "internal private agent incident", AgentID: fixtures.Agent.ID, MonitorID: "unmapped-monitor-for-agent-test", OpenedAt: now, LastEventAt: now, LatestEvent: "agent raw event should not leak", NotificationStatus: "pending", CreatedAt: now, UpdatedAt: now}
	if err := server.db.Create(&incident).Error; err != nil {
		t.Fatalf("create incident: %v", err)
	}
	resp := performJSONRequest(t, server, http.MethodGet, "/v1/status-pages/"+fixtures.Page.ID+"/incidents/suggestions?incident_id="+incident.ID, nil, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("suggestions status = %d, body = %s", resp.Code, resp.Body.String())
	}
	body := resp.Body.String()
	for _, privateValue := range []string{incident.Title, incident.LatestEvent, fixtures.Agent.Name, fixtures.Agent.Token, fixtures.Agent.ID, fixtures.Monitor.Name, fixtures.Monitor.ID, fixtures.HiddenComponent.PublicName} {
		if strings.Contains(body, privateValue) {
			t.Fatalf("suggestions leaked %q in %s", privateValue, body)
		}
	}
	var parsed struct {
		Data struct {
			Suggestions []StatusPageIncidentComponentSuggestionResponse `json:"suggestions"`
			Count       int                                             `json:"count"`
		} `json:"data"`
	}
	decodeResponse(t, resp, &parsed)
	if parsed.Data.Count != 1 || len(parsed.Data.Suggestions) != 1 {
		t.Fatalf("suggestions = %+v, want one public agent component", parsed.Data.Suggestions)
	}
	suggestion := parsed.Data.Suggestions[0]
	if suggestion.ComponentID != fixtures.AgentComponent.ID || suggestion.ComponentName != fixtures.AgentComponent.PublicName {
		t.Fatalf("suggestion = %+v, want agent component", suggestion)
	}
	if len(suggestion.Matches) != 1 || suggestion.Matches[0].ResourceType != "agent" || !strings.Contains(suggestion.Matches[0].MatchReason, "agent") {
		t.Fatalf("suggestion matches = %+v, want agent match reason", suggestion.Matches)
	}
}
