package api

import (
	"encoding/json"
	"fmt"
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
	page := db.StatusPage{
		ID:                        "status-page-auth-boundary",
		Slug:                      "auth-boundary",
		Title:                     "Auth Boundary",
		Visibility:                "public",
		ThemeSettings:             "{}",
		DefaultIncidentVisibility: "draft",
		PublishedAt:               &publishedAt,
	}
	section := db.StatusPageSection{
		ID:           "status-page-auth-boundary-section",
		StatusPageID: page.ID,
		Name:         "Public",
	}
	component := db.StatusPageComponent{
		ID:           "status-page-auth-boundary-component",
		StatusPageID: page.ID,
		SectionID:    section.ID,
		PublicName:   "Public API",
		DisplayMode:  "manual",
		ManualStatus: "operational",
		Visible:      true,
	}
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

	createPageResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages", gin.H{
		"slug":  "audit-status",
		"title": "Audit Status",
	}, token)
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

	createSectionResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+createdPage.Data.Page.ID+"/sections", gin.H{
		"name": "API",
	}, token)
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

	createComponentResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+createdPage.Data.Page.ID+"/components", gin.H{
		"section_id":  createdSection.Data.Section.ID,
		"public_name": "REST API",
		"visible":     true,
	}, token)
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

	createMappingResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+createdPage.Data.Page.ID+"/components/"+createdComponent.Data.Component.ID+"/mappings", gin.H{
		"resource_type":          "monitor",
		"resource_id":            registeredMonitor.Data.MonitorID,
		"health_rollup_strategy": "worst",
		"uptime_rollup_strategy": "worst",
	}, token)
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

	updateMappingResp := performJSONRequest(t, server, http.MethodPut, "/v1/status-pages/"+createdPage.Data.Page.ID+"/components/"+createdComponent.Data.Component.ID+"/mappings/"+createdMapping.Data.Mapping.ID, gin.H{
		"health_rollup_strategy": "average",
	}, token)
	if updateMappingResp.Code != http.StatusOK {
		t.Fatalf("update mapping status = %d, body = %s", updateMappingResp.Code, updateMappingResp.Body.String())
	}

	publishResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+createdPage.Data.Page.ID+"/publish", nil, token)
	if publishResp.Code != http.StatusOK {
		t.Fatalf("publish status = %d, body = %s", publishResp.Code, publishResp.Body.String())
	}

	now := time.Now().UTC()
	createIncidentResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+createdPage.Data.Page.ID+"/incidents", gin.H{
		"title":                  "Elevated API errors",
		"public_status":          "investigating",
		"severity":               "high",
		"impact_summary":         "Customer-safe summary only",
		"visibility":             "published",
		"affected_component_ids": []string{createdComponent.Data.Component.ID},
		"published_at":           now,
	}, token)
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

	updateIncidentResp := performJSONRequest(t, server, http.MethodPut, "/v1/status-pages/"+createdPage.Data.Page.ID+"/incidents/"+createdIncident.Data.Incident.ID, gin.H{
		"public_status": "identified",
	}, token)
	if updateIncidentResp.Code != http.StatusOK {
		t.Fatalf("update incident status = %d, body = %s", updateIncidentResp.Code, updateIncidentResp.Body.String())
	}

	rawTimelineMessage := "raw incident payload with internal secret token"
	createUpdateResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+createdPage.Data.Page.ID+"/incidents/"+createdIncident.Data.Incident.ID+"/updates", gin.H{
		"status":       "resolved",
		"message":      rawTimelineMessage,
		"created_by":   "ops",
		"published_at": now,
	}, token)
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

	for _, action := range []string{
		service.StatusPageAuditActionComponentMappingCreated,
		service.StatusPageAuditActionComponentMappingUpdated,
		service.StatusPageAuditActionPublished,
		service.StatusPageAuditActionPublicIncidentCreated,
		service.StatusPageAuditActionPublicIncidentUpdated,
		service.StatusPageAuditActionPublicIncidentUpdateCreated,
		service.StatusPageAuditActionPublicIncidentResolved,
		service.StatusPageAuditActionUnpublished,
	} {
		if !actions[action] {
			t.Fatalf("missing audit action %q in events: %+v", action, events)
		}
	}
}

func TestStatusPageIncidentComponentSuggestionsMatchMonitorAndRedactInternals(t *testing.T) {
	server := setupTestServer(t)
	now := time.Now().UTC()

	fixtures := createStatusPageSuggestionFixtures(t, server, now)
	incident := db.Incident{
		ID:                 "incident-monitor-secret",
		Status:             "open",
		Severity:           "critical",
		Title:              "internal private monitor incident",
		AgentID:            "unmapped-agent-for-monitor-test",
		MonitorID:          fixtures.Monitor.ID,
		OpenedAt:           now,
		LastEventAt:        now,
		LatestEvent:        "raw report payload should not leak",
		NotificationStatus: "pending",
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := server.db.Create(&incident).Error; err != nil {
		t.Fatalf("create incident: %v", err)
	}
	if err := server.db.Create(&db.MonitorReport{
		ID:          "report-monitor-secret",
		MonitorID:   fixtures.Monitor.ID,
		Payload:     `{"secret":"monitor-report-private-token"}`,
		CollectedAt: now.Format(time.RFC3339),
		Health:      "down",
		CreatedAt:   now,
	}).Error; err != nil {
		t.Fatalf("create report: %v", err)
	}

	resp := performJSONRequest(t, server, http.MethodGet, "/v1/status-pages/"+fixtures.Page.ID+"/incidents/suggestions?incident_id="+incident.ID, nil, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("suggestions status = %d, body = %s", resp.Code, resp.Body.String())
	}

	body := resp.Body.String()
	for _, privateValue := range []string{
		incident.Title,
		incident.LatestEvent,
		fixtures.Agent.Name,
		fixtures.Agent.Token,
		fixtures.Monitor.Name,
		fixtures.Monitor.ID,
		fixtures.Agent.ID,
		"monitor-report-private-token",
		fixtures.HiddenComponent.PublicName,
	} {
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
	if len(suggestion.Matches) != 1 || suggestion.Matches[0].ResourceType != "monitor" ||
		!strings.Contains(suggestion.Matches[0].MatchReason, "monitor") {
		t.Fatalf("suggestion matches = %+v, want monitor match reason", suggestion.Matches)
	}
}

func TestStatusPageIncidentComponentSuggestionsMatchAgentAndRedactInternals(t *testing.T) {
	server := setupTestServer(t)
	now := time.Now().UTC()

	fixtures := createStatusPageSuggestionFixtures(t, server, now)
	incident := db.Incident{
		ID:                 "incident-agent-secret",
		Status:             "open",
		Severity:           "high",
		Title:              "internal private agent incident",
		AgentID:            fixtures.Agent.ID,
		MonitorID:          "unmapped-monitor-for-agent-test",
		OpenedAt:           now,
		LastEventAt:        now,
		LatestEvent:        "agent raw event should not leak",
		NotificationStatus: "pending",
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := server.db.Create(&incident).Error; err != nil {
		t.Fatalf("create incident: %v", err)
	}

	resp := performJSONRequest(t, server, http.MethodGet, "/v1/status-pages/"+fixtures.Page.ID+"/incidents/suggestions?incident_id="+incident.ID, nil, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("suggestions status = %d, body = %s", resp.Code, resp.Body.String())
	}

	body := resp.Body.String()
	for _, privateValue := range []string{
		incident.Title,
		incident.LatestEvent,
		fixtures.Agent.Name,
		fixtures.Agent.Token,
		fixtures.Agent.ID,
		fixtures.Monitor.Name,
		fixtures.Monitor.ID,
		fixtures.HiddenComponent.PublicName,
	} {
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
	if len(suggestion.Matches) != 1 || suggestion.Matches[0].ResourceType != "agent" ||
		!strings.Contains(suggestion.Matches[0].MatchReason, "agent") {
		t.Fatalf("suggestion matches = %+v, want agent match reason", suggestion.Matches)
	}
}

func TestStatusPageThemeSettingsValidationAndPublicProjection(t *testing.T) {
	server := setupTestServer(t)
	createPageResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages", gin.H{
		"slug":        "branded-status",
		"title":       "Branded Status",
		"description": "Customer-facing availability",
		"visibility":  statusPageVisibilityPublic,
		"theme_settings": gin.H{
			"accent_color":          "#2AB3C4",
			"component_density":     "compact",
			"header_style":          "centered",
			"logo_alt":              "  Acme status logo  ",
			"logo_url":              "https://cdn.acme.test/logo.svg",
			"open_graph_site_name":  "Acme Trust",
			"open_graph_type":       "website",
			"show_incident_history": false,
			"show_uptime_summary":   true,
		},
	}, "")
	if createPageResp.Code != http.StatusCreated {
		t.Fatalf("create themed page status = %d, body = %s", createPageResp.Code, createPageResp.Body.String())
	}
	var createdPage struct {
		Data struct {
			Page struct {
				ThemeSettings map[string]interface{} `json:"theme_settings"`
			} `json:"page"`
		} `json:"data"`
	}
	decodeResponse(t, createPageResp, &createdPage)
	if createdPage.Data.Page.ThemeSettings["accent_color"] != "#2ab3c4" ||
		createdPage.Data.Page.ThemeSettings["logo_alt"] != "Acme status logo" ||
		createdPage.Data.Page.ThemeSettings["header_style"] != "centered" ||
		createdPage.Data.Page.ThemeSettings["component_density"] != "compact" ||
		createdPage.Data.Page.ThemeSettings["show_incident_history"] != false {
		t.Fatalf("admin theme settings = %+v, want sanitized supported values", createdPage.Data.Page.ThemeSettings)
	}

	publicResp := performJSONRequest(t, server, http.MethodGet, "/status/branded-status", nil, "")
	if publicResp.Code != http.StatusOK {
		t.Fatalf("public themed page status = %d, body = %s", publicResp.Code, publicResp.Body.String())
	}
	var publicPayload struct {
		Data struct {
			StatusPage struct {
				Page struct {
					ThemeSettings map[string]interface{} `json:"theme_settings"`
				} `json:"page"`
			} `json:"status_page"`
		} `json:"data"`
	}
	decodeResponse(t, publicResp, &publicPayload)
	if publicPayload.Data.StatusPage.Page.ThemeSettings["accent_color"] != "#2ab3c4" ||
		publicPayload.Data.StatusPage.Page.ThemeSettings["logo_url"] != "https://cdn.acme.test/logo.svg" ||
		publicPayload.Data.StatusPage.Page.ThemeSettings["open_graph_site_name"] != "Acme Trust" {
		t.Fatalf("public theme settings = %+v, want public sanitized theme values", publicPayload.Data.StatusPage.Page.ThemeSettings)
	}

	invalidSettings := []gin.H{
		{"accent_color": "green"},
		{"logo_url": "javascript://status.example.test/logo.svg"},
		{"header_style": "hero"},
		{"component_density": "dense"},
		{"show_uptime_summary": "true"},
		{"open_graph_type": "article"},
		{"accent": "green"},
	}
	for index, themeSettings := range invalidSettings {
		resp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages", gin.H{
			"slug":           fmt.Sprintf("invalid-theme-%d", index),
			"title":          fmt.Sprintf("Invalid Theme %d", index),
			"theme_settings": themeSettings,
		}, "")
		if resp.Code != http.StatusBadRequest {
			t.Fatalf("invalid theme %d status = %d, body = %s, want 400", index, resp.Code, resp.Body.String())
		}
	}
}

func TestStatusPageAdminAPIFlow(t *testing.T) {
	server := setupTestServer(t)
	registered := registerTestAgent(t, server)
	registeredMonitor := registerTestMonitor(t, server, registered.Data.AgentID, registered.Data.Token)
	if err := server.db.Model(&db.Monitor{}).Where("id = ?", registeredMonitor.Data.MonitorID).Updates(map[string]interface{}{
		"health":          "down",
		"computed_health": "down",
	}).Error; err != nil {
		t.Fatalf("update monitor health: %v", err)
	}

	createPageResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages", gin.H{
		"slug":           "main-status",
		"title":          "Main Status",
		"description":    "Customer-facing availability",
		"theme_settings": gin.H{"accent_color": "#10b981"},
	}, "")
	if createPageResp.Code != http.StatusCreated {
		t.Fatalf("create status page status = %d, body = %s", createPageResp.Code, createPageResp.Body.String())
	}
	var createdPage struct {
		Data struct {
			Page struct {
				ID            string                 `json:"id"`
				Slug          string                 `json:"slug"`
				Title         string                 `json:"title"`
				Visibility    string                 `json:"visibility"`
				ThemeSettings map[string]interface{} `json:"theme_settings"`
			} `json:"page"`
		} `json:"data"`
	}
	decodeResponse(t, createPageResp, &createdPage)
	if createdPage.Data.Page.ID == "" || createdPage.Data.Page.Slug != "main-status" || createdPage.Data.Page.Visibility != "draft" {
		t.Fatalf("created page = %+v, want draft main-status page", createdPage.Data.Page)
	}
	if createdPage.Data.Page.ThemeSettings["accent_color"] != "#10b981" {
		t.Fatalf("theme settings = %+v, want accent color", createdPage.Data.Page.ThemeSettings)
	}

	createSectionResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+createdPage.Data.Page.ID+"/sections", gin.H{
		"name":                 "API",
		"sort_order":           10,
		"collapsed_by_default": false,
	}, "")
	if createSectionResp.Code != http.StatusCreated {
		t.Fatalf("create section status = %d, body = %s", createSectionResp.Code, createSectionResp.Body.String())
	}
	var createdSection struct {
		Data struct {
			Section struct {
				ID        string `json:"id"`
				Name      string `json:"name"`
				SortOrder int    `json:"sort_order"`
			} `json:"section"`
		} `json:"data"`
	}
	decodeResponse(t, createSectionResp, &createdSection)
	if createdSection.Data.Section.ID == "" || createdSection.Data.Section.Name != "API" || createdSection.Data.Section.SortOrder != 10 {
		t.Fatalf("created section = %+v, want API section", createdSection.Data.Section)
	}

	updateSectionResp := performJSONRequest(t, server, http.MethodPut, "/v1/status-pages/"+createdPage.Data.Page.ID+"/sections/"+createdSection.Data.Section.ID, gin.H{
		"name":                 "Core API",
		"collapsed_by_default": true,
	}, "")
	if updateSectionResp.Code != http.StatusOK {
		t.Fatalf("update section status = %d, body = %s", updateSectionResp.Code, updateSectionResp.Body.String())
	}

	createComponentResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+createdPage.Data.Page.ID+"/components", gin.H{
		"section_id":         createdSection.Data.Section.ID,
		"public_name":        "REST API",
		"public_description": "Primary API",
		"display_mode":       "single_resource",
		"sort_order":         5,
		"visible":            true,
	}, "")
	if createComponentResp.Code != http.StatusCreated {
		t.Fatalf("create component status = %d, body = %s", createComponentResp.Code, createComponentResp.Body.String())
	}
	var createdComponent struct {
		Data struct {
			Component struct {
				ID         string `json:"id"`
				SectionID  string `json:"section_id"`
				PublicName string `json:"public_name"`
				Visible    bool   `json:"visible"`
			} `json:"component"`
		} `json:"data"`
	}
	decodeResponse(t, createComponentResp, &createdComponent)
	if createdComponent.Data.Component.ID == "" || createdComponent.Data.Component.SectionID != createdSection.Data.Section.ID ||
		createdComponent.Data.Component.PublicName != "REST API" || !createdComponent.Data.Component.Visible {
		t.Fatalf("created component = %+v, want visible REST API component", createdComponent.Data.Component)
	}

	createMappingResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+createdPage.Data.Page.ID+"/components/"+createdComponent.Data.Component.ID+"/mappings", gin.H{
		"resource_type":          "monitor",
		"resource_id":            registeredMonitor.Data.MonitorID,
		"health_rollup_strategy": "worst",
		"uptime_rollup_strategy": "worst",
	}, "")
	if createMappingResp.Code != http.StatusCreated {
		t.Fatalf("create mapping status = %d, body = %s", createMappingResp.Code, createMappingResp.Body.String())
	}
	var createdMapping struct {
		Data struct {
			Mapping struct {
				ID                   string `json:"id"`
				ResourceType         string `json:"resource_type"`
				ResourceID           string `json:"resource_id"`
				HealthRollupStrategy string `json:"health_rollup_strategy"`
			} `json:"mapping"`
		} `json:"data"`
	}
	decodeResponse(t, createMappingResp, &createdMapping)
	if createdMapping.Data.Mapping.ID == "" || createdMapping.Data.Mapping.ResourceType != "monitor" ||
		createdMapping.Data.Mapping.ResourceID != registeredMonitor.Data.MonitorID {
		t.Fatalf("created mapping = %+v, want monitor mapping", createdMapping.Data.Mapping)
	}

	updateMappingResp := performJSONRequest(t, server, http.MethodPut, "/v1/status-pages/"+createdPage.Data.Page.ID+"/components/"+createdComponent.Data.Component.ID+"/mappings/"+createdMapping.Data.Mapping.ID, gin.H{
		"health_rollup_strategy": "average",
	}, "")
	if updateMappingResp.Code != http.StatusOK {
		t.Fatalf("update mapping status = %d, body = %s", updateMappingResp.Code, updateMappingResp.Body.String())
	}

	now := time.Now().UTC()
	scheduledStart := now.Add(2 * time.Hour).Truncate(time.Second)
	scheduledEnd := scheduledStart.Add(30 * time.Minute)
	createIncidentResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+createdPage.Data.Page.ID+"/incidents", gin.H{
		"title":                  "Elevated API errors",
		"public_status":          "scheduled",
		"severity":               "high",
		"impact_summary":         "Some requests are failing.",
		"visibility":             "draft",
		"affected_component_ids": []string{createdComponent.Data.Component.ID},
		"scheduled_start_at":     scheduledStart,
		"scheduled_end_at":       scheduledEnd,
	}, "")
	if createIncidentResp.Code != http.StatusCreated {
		t.Fatalf("create public incident status = %d, body = %s", createIncidentResp.Code, createIncidentResp.Body.String())
	}
	var createdIncident struct {
		Data struct {
			Incident struct {
				ID                   string   `json:"id"`
				Title                string   `json:"title"`
				PublicStatus         string   `json:"public_status"`
				AffectedComponentIDs []string `json:"affected_component_ids"`
				ScheduledStartAt     string   `json:"scheduled_start_at"`
				ScheduledEndAt       string   `json:"scheduled_end_at"`
			} `json:"incident"`
		} `json:"data"`
	}
	decodeResponse(t, createIncidentResp, &createdIncident)
	if createdIncident.Data.Incident.ID == "" || createdIncident.Data.Incident.Title != "Elevated API errors" ||
		createdIncident.Data.Incident.PublicStatus != "scheduled" ||
		createdIncident.Data.Incident.ScheduledStartAt == "" ||
		createdIncident.Data.Incident.ScheduledEndAt == "" ||
		len(createdIncident.Data.Incident.AffectedComponentIDs) != 1 ||
		createdIncident.Data.Incident.AffectedComponentIDs[0] != createdComponent.Data.Component.ID {
		t.Fatalf("created incident = %+v, want API incident", createdIncident.Data.Incident)
	}

	updateIncidentResp := performJSONRequest(t, server, http.MethodPut, "/v1/status-pages/"+createdPage.Data.Page.ID+"/incidents/"+createdIncident.Data.Incident.ID, gin.H{
		"public_status":      "scheduled",
		"visibility":         "published",
		"published_at":       now,
		"scheduled_start_at": scheduledStart,
		"scheduled_end_at":   scheduledEnd,
	}, "")
	if updateIncidentResp.Code != http.StatusOK {
		t.Fatalf("update public incident status = %d, body = %s", updateIncidentResp.Code, updateIncidentResp.Body.String())
	}

	createUpdateResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+createdPage.Data.Page.ID+"/incidents/"+createdIncident.Data.Incident.ID+"/updates", gin.H{
		"status":       "monitoring",
		"message":      "A mitigation is in place.",
		"created_by":   "ops",
		"published_at": now,
	}, "")
	if createUpdateResp.Code != http.StatusCreated {
		t.Fatalf("create public incident update status = %d, body = %s", createUpdateResp.Code, createUpdateResp.Body.String())
	}

	detailResp := performJSONRequest(t, server, http.MethodGet, "/v1/status-pages/"+createdPage.Data.Page.ID, nil, "")
	if detailResp.Code != http.StatusOK {
		t.Fatalf("detail status = %d, body = %s", detailResp.Code, detailResp.Body.String())
	}
	var detail struct {
		Data struct {
			Page struct {
				ID string `json:"id"`
			} `json:"page"`
			Sections []struct {
				ID                 string `json:"id"`
				Name               string `json:"name"`
				CollapsedByDefault bool   `json:"collapsed_by_default"`
			} `json:"sections"`
			Components []struct {
				ID       string `json:"id"`
				Mappings []struct {
					ID                   string `json:"id"`
					HealthRollupStrategy string `json:"health_rollup_strategy"`
				} `json:"mappings"`
			} `json:"components"`
			Incidents []struct {
				ID           string `json:"id"`
				PublicStatus string `json:"public_status"`
				Updates      []struct {
					Message string `json:"message"`
				} `json:"updates"`
			} `json:"incidents"`
		} `json:"data"`
	}
	decodeResponse(t, detailResp, &detail)
	if detail.Data.Page.ID != createdPage.Data.Page.ID || len(detail.Data.Sections) != 1 || len(detail.Data.Components) != 1 || len(detail.Data.Incidents) != 1 {
		t.Fatalf("detail = %+v, want one section/component/incident", detail.Data)
	}
	if detail.Data.Sections[0].Name != "Core API" || !detail.Data.Sections[0].CollapsedByDefault {
		t.Fatalf("detail section = %+v, want updated Core API section", detail.Data.Sections[0])
	}
	if len(detail.Data.Components[0].Mappings) != 1 || detail.Data.Components[0].Mappings[0].HealthRollupStrategy != "average" {
		t.Fatalf("detail mappings = %+v, want updated average mapping", detail.Data.Components[0].Mappings)
	}
	if detail.Data.Incidents[0].PublicStatus != "monitoring" || len(detail.Data.Incidents[0].Updates) != 1 ||
		detail.Data.Incidents[0].Updates[0].Message != "A mitigation is in place." {
		t.Fatalf("detail incident = %+v, want monitoring incident with update", detail.Data.Incidents[0])
	}

	previewResp := performJSONRequest(t, server, http.MethodGet, "/v1/status-pages/"+createdPage.Data.Page.ID+"/preview", nil, "")
	if previewResp.Code != http.StatusOK {
		t.Fatalf("preview status = %d, body = %s", previewResp.Code, previewResp.Body.String())
	}
	assertNotContains(t, previewResp.Body.String(), registeredMonitor.Data.MonitorID)
	var preview struct {
		Data struct {
			Preview struct {
				Page struct {
					Slug string `json:"slug"`
				} `json:"page"`
				Sections []struct {
					Components []struct {
						Name   string `json:"name"`
						Status string `json:"status"`
					} `json:"components"`
				} `json:"sections"`
			} `json:"preview"`
		} `json:"data"`
	}
	decodeResponse(t, previewResp, &preview)
	if preview.Data.Preview.Page.Slug != "main-status" || len(preview.Data.Preview.Sections) != 1 ||
		len(preview.Data.Preview.Sections[0].Components) != 1 ||
		preview.Data.Preview.Sections[0].Components[0].Status != "major_outage" {
		t.Fatalf("preview = %+v, want public-safe major outage preview", preview.Data.Preview)
	}

	publishResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+createdPage.Data.Page.ID+"/publish", nil, "")
	if publishResp.Code != http.StatusOK {
		t.Fatalf("publish status = %d, body = %s", publishResp.Code, publishResp.Body.String())
	}
	publicResp := performJSONRequest(t, server, http.MethodGet, "/status/main-status", nil, "")
	if publicResp.Code != http.StatusOK {
		t.Fatalf("public status page status = %d, body = %s", publicResp.Code, publicResp.Body.String())
	}
	assertNotContains(t, publicResp.Body.String(), registeredMonitor.Data.MonitorID)
	var publicPage struct {
		Data struct {
			StatusPage struct {
				Page struct {
					Slug string `json:"slug"`
				} `json:"page"`
				OverallStatus string `json:"overall_status"`
				Sections      []struct {
					Components []struct {
						ID     string `json:"id"`
						Name   string `json:"name"`
						Status string `json:"status"`
					} `json:"components"`
				} `json:"sections"`
				Incidents []struct {
					ID               string `json:"id"`
					Title            string `json:"title"`
					PublicStatus     string `json:"public_status"`
					ScheduledStartAt string `json:"scheduled_start_at"`
					ScheduledEndAt   string `json:"scheduled_end_at"`
				} `json:"incidents"`
			} `json:"status_page"`
		} `json:"data"`
	}
	decodeResponse(t, publicResp, &publicPage)
	if publicPage.Data.StatusPage.Page.Slug != "main-status" ||
		publicPage.Data.StatusPage.OverallStatus != "major_outage" ||
		len(publicPage.Data.StatusPage.Sections) != 1 ||
		len(publicPage.Data.StatusPage.Sections[0].Components) != 1 ||
		len(publicPage.Data.StatusPage.Incidents) != 1 {
		t.Fatalf("public page = %+v, want public-safe status projection", publicPage.Data.StatusPage)
	}
	if publicPage.Data.StatusPage.Incidents[0].ScheduledStartAt == "" ||
		publicPage.Data.StatusPage.Incidents[0].ScheduledEndAt == "" {
		t.Fatalf("public incident = %+v, want scheduled window timestamps", publicPage.Data.StatusPage.Incidents[0])
	}

	publicIncidentsResp := performJSONRequest(t, server, http.MethodGet, "/status/main-status/incidents", nil, "")
	if publicIncidentsResp.Code != http.StatusOK {
		t.Fatalf("public incidents status = %d, body = %s", publicIncidentsResp.Code, publicIncidentsResp.Body.String())
	}
	publicIncidentResp := performJSONRequest(t, server, http.MethodGet, "/status/main-status/incidents/"+createdIncident.Data.Incident.ID, nil, "")
	if publicIncidentResp.Code != http.StatusOK {
		t.Fatalf("public incident detail status = %d, body = %s", publicIncidentResp.Code, publicIncidentResp.Body.String())
	}

	unpublishResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+createdPage.Data.Page.ID+"/unpublish", nil, "")
	if unpublishResp.Code != http.StatusOK {
		t.Fatalf("unpublish status = %d, body = %s", unpublishResp.Code, unpublishResp.Body.String())
	}
	notFoundPublicResp := performJSONRequest(t, server, http.MethodGet, "/status/main-status", nil, "")
	if notFoundPublicResp.Code != http.StatusNotFound {
		t.Fatalf("unpublished public status = %d, body = %s, want 404", notFoundPublicResp.Code, notFoundPublicResp.Body.String())
	}

	for _, path := range []string{
		"/v1/status-pages",
		"/v1/status-pages/" + createdPage.Data.Page.ID + "/sections",
		"/v1/status-pages/" + createdPage.Data.Page.ID + "/components",
		"/v1/status-pages/" + createdPage.Data.Page.ID + "/components/" + createdComponent.Data.Component.ID + "/mappings",
		"/v1/status-pages/" + createdPage.Data.Page.ID + "/incidents",
	} {
		resp := performJSONRequest(t, server, http.MethodGet, path, nil, "")
		if resp.Code != http.StatusOK {
			t.Fatalf("list %s status = %d, body = %s", path, resp.Code, resp.Body.String())
		}
	}
}

func TestPublicStatusPageMetadataProjectionUsesSafeDefaultsAndConfiguredFields(t *testing.T) {
	server := setupTestServer(t)
	now := time.Now().UTC()
	pages := []db.StatusPage{
		{
			ID:                        "status-page-metadata-default",
			Slug:                      "metadata-default",
			Title:                     "Acme Status",
			Description:               "Service availability",
			Visibility:                statusPageVisibilityPublic,
			ThemeSettings:             "{}",
			DefaultIncidentVisibility: statusPageIncidentVisibilityDraft,
			PublishedAt:               &now,
		},
		{
			ID:                "status-page-metadata-configured",
			Slug:              "metadata-configured",
			Title:             "Acme Internal Page Title",
			Description:       "Fallback public description",
			SEOTitle:          "Acme availability",
			SEODescription:    "Live platform state",
			OpenGraphImageURL: "https://cdn.acme.test/status.png",
			CanonicalURL:      "https://status.acme.test/",
			Visibility:        statusPageVisibilityUnlisted,
			ThemeSettings: `{
				"open_graph_description": "Realtime availability for Acme",
				"open_graph_site_name": "Acme Trust",
				"open_graph_title": "Acme Status Updates",
				"open_graph_type": "website"
			}`,
			DefaultIncidentVisibility: statusPageIncidentVisibilityDraft,
			PublishedAt:               &now,
		},
		{
			ID:                        "status-page-metadata-draft",
			Slug:                      "metadata-draft",
			Title:                     "Draft Status",
			Visibility:                statusPageVisibilityDraft,
			ThemeSettings:             "{}",
			DefaultIncidentVisibility: statusPageIncidentVisibilityDraft,
		},
	}
	if err := server.db.Create(&pages).Error; err != nil {
		t.Fatalf("create status pages: %v", err)
	}

	defaultResp := performJSONRequest(t, server, http.MethodGet, "/status/metadata-default", nil, "")
	if defaultResp.Code != http.StatusOK {
		t.Fatalf("default metadata status = %d, body = %s", defaultResp.Code, defaultResp.Body.String())
	}
	var defaultPayload struct {
		Data struct {
			StatusPage struct {
				Metadata StatusPagePublicMetadataResponse `json:"metadata"`
			} `json:"status_page"`
		} `json:"data"`
	}
	decodeResponse(t, defaultResp, &defaultPayload)
	defaultMetadata := defaultPayload.Data.StatusPage.Metadata
	if defaultMetadata.Title != "Acme Status" ||
		defaultMetadata.Description != "Service availability" ||
		defaultMetadata.CanonicalURL != "" ||
		defaultMetadata.OpenGraph.Title != "Acme Status" ||
		defaultMetadata.OpenGraph.Description != "Service availability" ||
		defaultMetadata.OpenGraph.URL != "" ||
		defaultMetadata.OpenGraph.Type != "website" ||
		defaultMetadata.OpenGraph.SiteName != "Acme Status" ||
		defaultMetadata.OpenGraph.ImageURL != "" {
		t.Fatalf("default metadata = %+v, want page-owned safe defaults", defaultMetadata)
	}

	configuredResp := performJSONRequest(t, server, http.MethodGet, "/status/metadata-configured", nil, "")
	if configuredResp.Code != http.StatusOK {
		t.Fatalf("configured metadata status = %d, body = %s", configuredResp.Code, configuredResp.Body.String())
	}
	var configuredPayload struct {
		Data struct {
			StatusPage struct {
				Metadata StatusPagePublicMetadataResponse `json:"metadata"`
			} `json:"status_page"`
		} `json:"data"`
	}
	decodeResponse(t, configuredResp, &configuredPayload)
	configuredMetadata := configuredPayload.Data.StatusPage.Metadata
	if configuredMetadata.Title != "Acme availability" ||
		configuredMetadata.Description != "Live platform state" ||
		configuredMetadata.CanonicalURL != "https://status.acme.test" ||
		configuredMetadata.OpenGraph.Title != "Acme Status Updates" ||
		configuredMetadata.OpenGraph.Description != "Realtime availability for Acme" ||
		configuredMetadata.OpenGraph.URL != "https://status.acme.test" ||
		configuredMetadata.OpenGraph.Type != "website" ||
		configuredMetadata.OpenGraph.SiteName != "Acme Trust" ||
		configuredMetadata.OpenGraph.ImageURL != "https://cdn.acme.test/status.png" {
		t.Fatalf("configured metadata = %+v, want configured public SEO and Open Graph fields", configuredMetadata)
	}

	draftResp := performJSONRequest(t, server, http.MethodGet, "/status/metadata-draft", nil, "")
	if draftResp.Code != http.StatusNotFound {
		t.Fatalf("draft public metadata status = %d, body = %s, want 404", draftResp.Code, draftResp.Body.String())
	}
	draftPreviewResp := performJSONRequest(t, server, http.MethodGet, "/v1/status-pages/status-page-metadata-draft/preview", nil, "")
	if draftPreviewResp.Code != http.StatusOK {
		t.Fatalf("draft preview metadata status = %d, body = %s", draftPreviewResp.Code, draftPreviewResp.Body.String())
	}
	var draftPreviewPayload struct {
		Data struct {
			Preview struct {
				Metadata StatusPagePublicMetadataResponse `json:"metadata"`
			} `json:"preview"`
		} `json:"data"`
	}
	decodeResponse(t, draftPreviewResp, &draftPreviewPayload)
	if draftPreviewPayload.Data.Preview.Metadata.Title != "Draft Status" {
		t.Fatalf("draft preview metadata = %+v, want draft metadata in admin preview", draftPreviewPayload.Data.Preview.Metadata)
	}
}

func TestPublicStatusPageHTMLRendersSafeMetadataAndTheme(t *testing.T) {
	server := setupTestServer(t)
	now := time.Date(2026, 5, 28, 3, 30, 0, 0, time.UTC)
	page := db.StatusPage{
		ID:                        "status-page-html",
		Slug:                      "html-status",
		CustomDomain:              "status.acme.test",
		Title:                     "Acme Public Status",
		Description:               "Customer-facing availability",
		SEOTitle:                  "Acme Status",
		SEODescription:            "Public availability for Acme",
		OpenGraphImageURL:         "https://cdn.acme.test/status.png",
		CanonicalURL:              "https://status.acme.test/",
		Visibility:                statusPageVisibilityPublic,
		ThemeSettings:             `{"accent_color":"#10b981","component_density":"compact","header_style":"centered","logo_alt":"Acme logo","logo_url":"https://cdn.acme.test/logo.svg","open_graph_site_name":"Acme Trust","open_graph_title":"Acme Status Updates","open_graph_description":"Realtime public availability","open_graph_type":"website"}`,
		DefaultIncidentVisibility: statusPageIncidentVisibilityDraft,
		PublishedAt:               &now,
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}
	section := db.StatusPageSection{
		ID:           "status-page-html-section",
		StatusPageID: page.ID,
		Name:         "Public services",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	component := db.StatusPageComponent{
		ID:                 "status-page-html-component",
		StatusPageID:       page.ID,
		SectionID:          section.ID,
		PublicName:         "Checkout API",
		PublicDescription:  "Customer checkout traffic",
		DisplayMode:        "manual",
		ManualStatus:       "degraded",
		ManualStatusReason: "Elevated latency",
		Visible:            true,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	incident := db.StatusPageIncident{
		ID:                   "status-page-html-incident",
		StatusPageID:         page.ID,
		Title:                "Checkout latency",
		PublicStatus:         "identified",
		Severity:             "medium",
		ImpactSummary:        "Some checkouts are slower than usual.",
		Visibility:           statusPageIncidentVisibilityPublished,
		AffectedComponentIDs: `["status-page-html-component"]`,
		PublishedAt:          &now,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if err := server.db.Create(&page).Error; err != nil {
		t.Fatalf("create status page: %v", err)
	}
	if err := server.db.Create(&section).Error; err != nil {
		t.Fatalf("create section: %v", err)
	}
	if err := server.db.Create(&component).Error; err != nil {
		t.Fatalf("create component: %v", err)
	}
	if err := server.db.Create(&incident).Error; err != nil {
		t.Fatalf("create incident: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/status/html-status", nil)
	req.Header.Set("Accept", "text/html")
	resp := httptest.NewRecorder()
	server.router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("HTML status = %d, body = %s", resp.Code, resp.Body.String())
	}
	body := resp.Body.String()
	assertContains(t, resp.Header().Get("Content-Type"), "text/html")
	assertContains(t, body, "<title>Acme Status</title>")
	assertContains(t, body, `<meta name="description" content="Public availability for Acme">`)
	assertContains(t, body, `<link rel="canonical" href="https://status.acme.test">`)
	assertContains(t, body, `<meta property="og:title" content="Acme Status Updates">`)
	assertContains(t, body, `<meta property="og:description" content="Realtime public availability">`)
	assertContains(t, body, `<meta property="og:image" content="https://cdn.acme.test/status.png">`)
	assertContains(t, body, `<img src="https://cdn.acme.test/logo.svg" alt="Acme logo">`)
	assertContains(t, body, "Checkout API")
	assertContains(t, body, "Elevated latency")
	assertContains(t, body, "Checkout latency")
	assertContains(t, body, "Identified")
	assertNotContains(t, body, "Private monitor name must not leak")
	assertNotContains(t, body, "10.0.0.7")

	jsonReq := httptest.NewRequest(http.MethodGet, "/status/html-status", nil)
	jsonReq.Header.Set("Accept", "application/json")
	jsonResp := httptest.NewRecorder()
	server.router.ServeHTTP(jsonResp, jsonReq)
	if jsonResp.Code != http.StatusOK {
		t.Fatalf("JSON status = %d, body = %s", jsonResp.Code, jsonResp.Body.String())
	}
	assertContains(t, jsonResp.Header().Get("Content-Type"), "application/json")
	assertContains(t, jsonResp.Body.String(), `"status_page"`)

	customDomainReq := httptest.NewRequest(http.MethodGet, "/", nil)
	customDomainReq.Host = "status.acme.test"
	customDomainReq.Header.Set("Accept", "text/html")
	customDomainResp := httptest.NewRecorder()
	server.router.ServeHTTP(customDomainResp, customDomainReq)
	if customDomainResp.Code != http.StatusOK {
		t.Fatalf("custom domain HTML status = %d, body = %s", customDomainResp.Code, customDomainResp.Body.String())
	}
	assertContains(t, customDomainResp.Body.String(), `<a href="http://status.acme.test/feed.atom">Atom feed</a>`)
}

func TestPublicStatusPageMetadataDoesNotUseMappedInternalResources(t *testing.T) {
	server := setupTestServer(t)
	registered := registerTestAgent(t, server)
	registeredMonitor := registerTestMonitor(t, server, registered.Data.AgentID, registered.Data.Token)
	if err := server.db.Model(&db.Monitor{}).Where("id = ?", registeredMonitor.Data.MonitorID).Updates(map[string]interface{}{
		"name":            "internal-db-01.local",
		"computed_health": "up",
		"health":          "up",
	}).Error; err != nil {
		t.Fatalf("update monitor: %v", err)
	}

	now := time.Now().UTC()
	page := db.StatusPage{
		ID:                        "status-page-metadata-redaction",
		Slug:                      "metadata-redaction",
		Title:                     "Customer Status",
		Description:               "Customer-facing availability",
		Visibility:                statusPageVisibilityPublic,
		ThemeSettings:             "{}",
		DefaultIncidentVisibility: statusPageIncidentVisibilityDraft,
		PublishedAt:               &now,
	}
	section := db.StatusPageSection{
		ID:           "status-page-metadata-redaction-section",
		StatusPageID: page.ID,
		Name:         "Public services",
	}
	component := db.StatusPageComponent{
		ID:           "status-page-metadata-redaction-component",
		StatusPageID: page.ID,
		SectionID:    section.ID,
		PublicName:   "Checkout API",
		DisplayMode:  "single_resource",
		Visible:      true,
	}
	mapping := db.StatusPageComponentMapping{
		ID:                   "status-page-metadata-redaction-mapping",
		ComponentID:          component.ID,
		ResourceType:         "monitor",
		ResourceID:           registeredMonitor.Data.MonitorID,
		HealthRollupStrategy: "worst",
		UptimeRollupStrategy: "worst",
	}
	if err := server.db.Create(&page).Error; err != nil {
		t.Fatalf("create status page: %v", err)
	}
	if err := server.db.Create(&section).Error; err != nil {
		t.Fatalf("create section: %v", err)
	}
	if err := server.db.Create(&component).Error; err != nil {
		t.Fatalf("create component: %v", err)
	}
	if err := server.db.Create(&mapping).Error; err != nil {
		t.Fatalf("create mapping: %v", err)
	}

	resp := performJSONRequest(t, server, http.MethodGet, "/status/metadata-redaction", nil, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("metadata redaction status = %d, body = %s", resp.Code, resp.Body.String())
	}
	var payload struct {
		Data struct {
			StatusPage struct {
				Metadata StatusPagePublicMetadataResponse `json:"metadata"`
			} `json:"status_page"`
		} `json:"data"`
	}
	decodeResponse(t, resp, &payload)
	metadataJSON, err := json.Marshal(payload.Data.StatusPage.Metadata)
	if err != nil {
		t.Fatalf("marshal metadata: %v", err)
	}
	metadata := string(metadataJSON)
	for _, internalValue := range []string{registeredMonitor.Data.MonitorID, "internal-db-01.local", registered.Data.AgentID} {
		if strings.Contains(metadata, internalValue) {
			t.Fatalf("metadata %s leaked internal resource value %q", metadata, internalValue)
		}
	}
}

func TestStatusPageCustomDomainValidationAndConflict(t *testing.T) {
	server := setupTestServer(t)

	createResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages", gin.H{
		"slug":          "domain-status",
		"title":         "Domain Status",
		"custom_domain": "Status.Example.COM:443",
	}, "")
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create status page status = %d, body = %s", createResp.Code, createResp.Body.String())
	}
	var created struct {
		Data struct {
			Page struct {
				ID           string `json:"id"`
				CustomDomain string `json:"custom_domain"`
			} `json:"page"`
		} `json:"data"`
	}
	decodeResponse(t, createResp, &created)
	if created.Data.Page.CustomDomain != "status.example.com" {
		t.Fatalf("custom_domain = %q, want status.example.com", created.Data.Page.CustomDomain)
	}

	secondResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages", gin.H{
		"slug":  "second-domain-status",
		"title": "Second Domain Status",
	}, "")
	if secondResp.Code != http.StatusCreated {
		t.Fatalf("create second page status = %d, body = %s", secondResp.Code, secondResp.Body.String())
	}
	var second struct {
		Data struct {
			Page struct {
				ID string `json:"id"`
			} `json:"page"`
		} `json:"data"`
	}
	decodeResponse(t, secondResp, &second)

	conflictResp := performJSONRequest(t, server, http.MethodPut, "/v1/status-pages/"+second.Data.Page.ID, gin.H{
		"slug":          "second-domain-status",
		"title":         "Second Domain Status",
		"custom_domain": "status.example.com",
	}, "")
	if conflictResp.Code != http.StatusBadRequest {
		t.Fatalf("conflict status = %d, body = %s, want 400", conflictResp.Code, conflictResp.Body.String())
	}
	assertContains(t, conflictResp.Body.String(), "already in use")

	invalidDomains := []string{
		"localhost",
		"127.0.0.1",
		"*.example.com",
		"https://status.example.com/path",
		"status",
		"status.local",
	}
	for i, domain := range invalidDomains {
		resp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages", gin.H{
			"slug":          fmt.Sprintf("invalid-domain-%d", i),
			"title":         fmt.Sprintf("Invalid Domain %d", i),
			"custom_domain": domain,
		}, "")
		if resp.Code != http.StatusBadRequest {
			t.Fatalf("domain %q status = %d, body = %s, want 400", domain, resp.Code, resp.Body.String())
		}
	}
}

func TestStatusPageCustomDomainHostRoutingAndIsolation(t *testing.T) {
	server := setupTestServer(t)
	now := time.Now().UTC()
	pages := []db.StatusPage{
		{
			ID:                        "status_page_custom_public",
			Slug:                      "custom-public",
			CustomDomain:              "status.example.com",
			Title:                     "Custom Public",
			Visibility:                statusPageVisibilityPublic,
			ThemeSettings:             "{}",
			DefaultIncidentVisibility: statusPageIncidentVisibilityDraft,
			PublishedAt:               &now,
		},
		{
			ID:                        "status_page_other_public",
			Slug:                      "other-public",
			CustomDomain:              "other.example.com",
			Title:                     "Other Public",
			Visibility:                statusPageVisibilityPublic,
			ThemeSettings:             "{}",
			DefaultIncidentVisibility: statusPageIncidentVisibilityDraft,
			PublishedAt:               &now,
		},
		{
			ID:                        "status_page_custom_draft",
			Slug:                      "custom-draft",
			CustomDomain:              "draft.example.com",
			Title:                     "Custom Draft",
			Visibility:                statusPageVisibilityDraft,
			ThemeSettings:             "{}",
			DefaultIncidentVisibility: statusPageIncidentVisibilityDraft,
		},
	}
	if err := server.db.Create(&pages).Error; err != nil {
		t.Fatalf("seed status pages: %v", err)
	}

	publicResp := performHostRequest(t, server, http.MethodGet, "/", "STATUS.EXAMPLE.COM:443")
	if publicResp.Code != http.StatusOK {
		t.Fatalf("custom host status = %d, body = %s", publicResp.Code, publicResp.Body.String())
	}
	assertContains(t, publicResp.Body.String(), "Custom Public")
	assertNotContains(t, publicResp.Body.String(), "Other Public")
	assertNotContains(t, publicResp.Body.String(), "Custom Draft")

	otherSlugOnCustomHostResp := performHostRequest(t, server, http.MethodGet, "/status/other-public", "status.example.com")
	if otherSlugOnCustomHostResp.Code != http.StatusNotFound {
		t.Fatalf("other slug on custom host status = %d, body = %s, want 404", otherSlugOnCustomHostResp.Code, otherSlugOnCustomHostResp.Body.String())
	}

	draftResp := performHostRequest(t, server, http.MethodGet, "/", "draft.example.com")
	if draftResp.Code != http.StatusNotFound {
		t.Fatalf("draft custom host status = %d, body = %s, want 404", draftResp.Code, draftResp.Body.String())
	}

	feedResp := performHostRequest(t, server, http.MethodGet, "/feed.atom", "status.example.com")
	if feedResp.Code != http.StatusOK {
		t.Fatalf("custom feed status = %d, body = %s", feedResp.Code, feedResp.Body.String())
	}
	assertContains(t, feedResp.Body.String(), "http://status.example.com")
	assertNotContains(t, feedResp.Body.String(), "/status/custom-public")
}

func TestStatusPagePublishValidation(t *testing.T) {
	server := setupTestServer(t)

	createPageResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages", gin.H{
		"slug":  "validation-status",
		"title": "Validation Status",
	}, "")
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

	duplicateSlugResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages", gin.H{
		"slug":  "validation-status",
		"title": "Duplicate Validation Status",
	}, "")
	if duplicateSlugResp.Code != http.StatusConflict {
		t.Fatalf("duplicate slug status = %d, body = %s, want 409", duplicateSlugResp.Code, duplicateSlugResp.Body.String())
	}
	assertContains(t, duplicateSlugResp.Body.String(), "slug already exists")

	emptyPublishResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+createdPage.Data.Page.ID+"/publish", nil, "")
	if emptyPublishResp.Code != http.StatusBadRequest {
		t.Fatalf("empty publish status = %d, body = %s, want 400", emptyPublishResp.Code, emptyPublishResp.Body.String())
	}
	assertContains(t, emptyPublishResp.Body.String(), "at least one visible component")

	createSectionResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+createdPage.Data.Page.ID+"/sections", gin.H{
		"name": "Private",
	}, "")
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

	createComponentResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+createdPage.Data.Page.ID+"/components", gin.H{
		"section_id":  createdSection.Data.Section.ID,
		"public_name": "localhost",
		"visible":     true,
	}, "")
	if createComponentResp.Code != http.StatusCreated {
		t.Fatalf("create component status = %d, body = %s", createComponentResp.Code, createComponentResp.Body.String())
	}
	unmappedPublishResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+createdPage.Data.Page.ID+"/publish", nil, "")
	if unmappedPublishResp.Code != http.StatusBadRequest {
		t.Fatalf("unmapped publish status = %d, body = %s, want 400", unmappedPublishResp.Code, unmappedPublishResp.Body.String())
	}
	assertContains(t, unmappedPublishResp.Body.String(), "mapped resource or manual status")
	assertContains(t, unmappedPublishResp.Body.String(), "looks like an internal host")

	ipPageResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages", gin.H{
		"slug":  "ip-validation-status",
		"title": "IP Validation Status",
	}, "")
	if ipPageResp.Code != http.StatusCreated {
		t.Fatalf("create IP validation page status = %d, body = %s", ipPageResp.Code, ipPageResp.Body.String())
	}
	var ipPage struct {
		Data struct {
			Page struct {
				ID string `json:"id"`
			} `json:"page"`
		} `json:"data"`
	}
	decodeResponse(t, ipPageResp, &ipPage)
	ipSectionResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+ipPage.Data.Page.ID+"/sections", gin.H{
		"name": "IP Private",
	}, "")
	if ipSectionResp.Code != http.StatusCreated {
		t.Fatalf("create IP validation section status = %d, body = %s", ipSectionResp.Code, ipSectionResp.Body.String())
	}
	var ipSection struct {
		Data struct {
			Section struct {
				ID string `json:"id"`
			} `json:"section"`
		} `json:"data"`
	}
	decodeResponse(t, ipSectionResp, &ipSection)
	ipComponentResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+ipPage.Data.Page.ID+"/components", gin.H{
		"section_id":    ipSection.Data.Section.ID,
		"public_name":   "192.168.1.10",
		"manual_status": "operational",
		"visible":       true,
	}, "")
	if ipComponentResp.Code != http.StatusCreated {
		t.Fatalf("create IP validation component status = %d, body = %s", ipComponentResp.Code, ipComponentResp.Body.String())
	}
	ipPublishResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+ipPage.Data.Page.ID+"/publish", nil, "")
	if ipPublishResp.Code != http.StatusBadRequest {
		t.Fatalf("IP label publish status = %d, body = %s, want 400", ipPublishResp.Code, ipPublishResp.Body.String())
	}
	assertContains(t, ipPublishResp.Body.String(), "looks like an internal host")
}

func performHostRequest(t *testing.T, server *Server, method string, path string, host string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, path, nil)
	req.Host = host
	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, req)
	return recorder
}

func setupStatusPageAuthTestServer(t *testing.T) *Server {
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

	return NewServer(database, logging.NewLogger(), &config.Config{
		FrontendAuthOn: true,
		AdminUsername:  "admin",
		AdminPassword:  "correct-password",
		JWTSecret:      "test-secret",
	})
}

func loginStatusPageTestAdmin(t *testing.T, server *Server) string {
	t.Helper()

	loginResp := performJSONRequest(t, server, http.MethodPost, "/v1/auth/login", map[string]string{
		"username": "admin",
		"password": "correct-password",
	}, "")
	if loginResp.Code != http.StatusOK {
		t.Fatalf("login status = %d, body = %s", loginResp.Code, loginResp.Body.String())
	}
	var login struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	decodeResponse(t, loginResp, &login)
	if login.Data.Token == "" {
		t.Fatalf("login response missing token: %+v", login)
	}
	return login.Data.Token
}

type statusPageSuggestionFixtures struct {
	Page             db.StatusPage
	Agent            db.Agent
	Monitor          db.Monitor
	MonitorComponent db.StatusPageComponent
	AgentComponent   db.StatusPageComponent
	HiddenComponent  db.StatusPageComponent
}

func createStatusPageSuggestionFixtures(t *testing.T, server *Server, now time.Time) statusPageSuggestionFixtures {
	t.Helper()

	page := db.StatusPage{
		ID:                        "status-page-suggestions",
		Slug:                      "suggestions",
		Title:                     "Suggestions",
		Visibility:                "draft",
		ThemeSettings:             "{}",
		DefaultIncidentVisibility: "draft",
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}
	section := db.StatusPageSection{
		ID:           "status-page-suggestions-section",
		StatusPageID: page.ID,
		Name:         "Customer components",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	agent := db.Agent{
		ID:        "agent-private-suggestions",
		MachineId: "machine-private-suggestions",
		Name:      "private-prod-agent-name",
		OS:        "linux",
		Arch:      "arm64",
		Token:     "private-agent-token-value",
		LastSeen:  now,
		CreatedAt: now,
	}
	monitor := db.Monitor{
		ID:             "monitor-private-suggestions",
		AgentID:        agent.ID,
		Name:           "private-checkout-monitor",
		Type:           "http",
		Lifecycle:      "active",
		Health:         "down",
		ComputedHealth: "down",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	monitorComponent := db.StatusPageComponent{
		ID:           "status-page-public-monitor-component",
		StatusPageID: page.ID,
		SectionID:    section.ID,
		PublicName:   "Checkout API",
		DisplayMode:  "single_resource",
		SortOrder:    1,
		Visible:      true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	agentComponent := db.StatusPageComponent{
		ID:           "status-page-public-agent-component",
		StatusPageID: page.ID,
		SectionID:    section.ID,
		PublicName:   "Core Platform",
		DisplayMode:  "single_resource",
		SortOrder:    2,
		Visible:      true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	hiddenComponent := db.StatusPageComponent{
		ID:           "status-page-hidden-private-component",
		StatusPageID: page.ID,
		SectionID:    section.ID,
		PublicName:   "Hidden private database",
		DisplayMode:  "single_resource",
		SortOrder:    3,
		Visible:      false,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	mappings := []db.StatusPageComponentMapping{
		{
			ID:                   "status-page-monitor-suggestion-mapping",
			ComponentID:          monitorComponent.ID,
			ResourceType:         "monitor",
			ResourceID:           monitor.ID,
			HealthRollupStrategy: "worst",
			UptimeRollupStrategy: "worst",
			CreatedAt:            now,
			UpdatedAt:            now,
		},
		{
			ID:                   "status-page-agent-suggestion-mapping",
			ComponentID:          agentComponent.ID,
			ResourceType:         "agent",
			ResourceID:           agent.ID,
			HealthRollupStrategy: "worst",
			UptimeRollupStrategy: "worst",
			CreatedAt:            now,
			UpdatedAt:            now,
		},
		{
			ID:                   "status-page-hidden-suggestion-mapping",
			ComponentID:          hiddenComponent.ID,
			ResourceType:         "monitor",
			ResourceID:           monitor.ID,
			HealthRollupStrategy: "worst",
			UptimeRollupStrategy: "worst",
			CreatedAt:            now,
			UpdatedAt:            now,
		},
	}

	if err := server.db.Create(&page).Error; err != nil {
		t.Fatalf("create status page: %v", err)
	}
	if err := server.db.Create(&section).Error; err != nil {
		t.Fatalf("create status page section: %v", err)
	}
	if err := server.db.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	if err := server.db.Create(&monitor).Error; err != nil {
		t.Fatalf("create monitor: %v", err)
	}
	if err := server.db.Create(&[]db.StatusPageComponent{monitorComponent, agentComponent, hiddenComponent}).Error; err != nil {
		t.Fatalf("create status page components: %v", err)
	}
	if err := server.db.Model(&db.StatusPageComponent{}).Where("id = ?", hiddenComponent.ID).Update("visible", false).Error; err != nil {
		t.Fatalf("hide status page component: %v", err)
	}
	if err := server.db.Create(&mappings).Error; err != nil {
		t.Fatalf("create status page component mappings: %v", err)
	}

	return statusPageSuggestionFixtures{
		Page:             page,
		Agent:            agent,
		Monitor:          monitor,
		MonitorComponent: monitorComponent,
		AgentComponent:   agentComponent,
		HiddenComponent:  hiddenComponent,
	}
}
