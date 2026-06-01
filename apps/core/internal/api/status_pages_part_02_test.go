package api

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"orion/core/internal/db"
	"strings"
	"testing"
	"time"
)

func TestStatusPageIncidentDraftFromInternalIncidentUsesSafePublicCopy(t *testing.T) {
	server := setupTestServer(t)
	now := time.Now().UTC()
	fixtures := createStatusPageSuggestionFixtures(t, server, now)
	incident := db.Incident{ID: "incident-public-draft-source", Status: "open", Severity: "critical", Title: "private checkout host db.internal.example failed", AgentID: "unmapped-agent-for-public-draft", MonitorID: fixtures.Monitor.ID, OpenedAt: now, LastEventAt: now, LatestEvent: "raw report payload token=super-secret host=db.internal.example monitor-private-suggestions", NotificationStatus: "pending", CreatedAt: now, UpdatedAt: now}
	if err := server.db.Create(&incident).Error; err != nil {
		t.Fatalf("create incident: %v", err)
	}
	if err := server.db.Create(&db.MonitorReport{ID: "report-public-draft-secret", MonitorID: fixtures.Monitor.ID, Payload: `{"hostname":"db.internal.example","token":"super-secret"}`, CollectedAt: now.Format(time.RFC3339), Health: "down", CreatedAt: now}).Error; err != nil {
		t.Fatalf("create report: %v", err)
	}
	previewResp := performJSONRequest(t, server, http.MethodGet, "/v1/status-pages/"+fixtures.Page.ID+"/incidents/draft?incident_id="+incident.ID, nil, "")
	if previewResp.Code != http.StatusOK {
		t.Fatalf("draft preview status = %d, body = %s", previewResp.Code, previewResp.Body.String())
	}
	var preview struct {
		Data struct {
			Draft StatusPageIncidentDraftResponse `json:"draft"`
		} `json:"data"`
	}
	decodeResponse(t, previewResp, &preview)
	assertSafePublicDraftCopy(t, preview.Data.Draft, []string{incident.Title, incident.LatestEvent, fixtures.Agent.Name, fixtures.Agent.Token, fixtures.Monitor.Name, fixtures.Monitor.ID, fixtures.Agent.ID, "db.internal.example", "super-secret", "report-public-draft-secret", fixtures.HiddenComponent.PublicName})
	if preview.Data.Draft.PublicStatus != "investigating" || preview.Data.Draft.Severity != "critical" {
		t.Fatalf("draft status/severity = %+v, want investigating critical", preview.Data.Draft)
	}
	if len(preview.Data.Draft.AffectedComponentIDs) != 1 || preview.Data.Draft.AffectedComponentIDs[0] != fixtures.MonitorComponent.ID {
		t.Fatalf("draft affected components = %+v, want monitor component", preview.Data.Draft.AffectedComponentIDs)
	}
	createResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+fixtures.Page.ID+"/incidents/draft", gin.H{"internal_incident_id": incident.ID, "affected_component_ids": []string{fixtures.MonitorComponent.ID}}, "")
	if createResp.Code != http.StatusCreated {
		t.Fatalf("draft create status = %d, body = %s", createResp.Code, createResp.Body.String())
	}
	var created struct {
		Data struct {
			Draft    StatusPageIncidentDraftResponse  `json:"draft"`
			Incident StatusPageIncidentResponse       `json:"incident"`
			Update   StatusPageIncidentUpdateResponse `json:"update"`
		} `json:"data"`
	}
	decodeResponse(t, createResp, &created)
	assertSafePublicDraftCopy(t, created.Data.Draft, []string{incident.Title, incident.LatestEvent, fixtures.Agent.Name, fixtures.Agent.Token, fixtures.Monitor.Name, fixtures.Monitor.ID, "db.internal.example", "super-secret"})
	if created.Data.Incident.InternalIncidentID != incident.ID || created.Data.Incident.Visibility != statusPageIncidentVisibilityDraft {
		t.Fatalf("created incident = %+v, want linked draft", created.Data.Incident)
	}
	if created.Data.Update.PublishedAt != nil || created.Data.Update.Message != created.Data.Draft.InitialUpdateMessage {
		t.Fatalf("created update = %+v, want unpublished generated update", created.Data.Update)
	}
	if err := server.db.Model(&db.StatusPage{}).Where("id = ?", fixtures.Page.ID).Updates(map[string]interface{}{"visibility": statusPageVisibilityPublic, "published_at": &now}).Error; err != nil {
		t.Fatalf("publish status page fixture: %v", err)
	}
	publicDetailResp := performJSONRequest(t, server, http.MethodGet, "/status/"+fixtures.Page.Slug+"/incidents/"+created.Data.Incident.ID, nil, "")
	if publicDetailResp.Code != http.StatusNotFound {
		t.Fatalf("public draft incident status = %d, body = %s, want 404", publicDetailResp.Code, publicDetailResp.Body.String())
	}
	publicListResp := performJSONRequest(t, server, http.MethodGet, "/status/"+fixtures.Page.Slug+"/incidents", nil, "")
	if publicListResp.Code != http.StatusOK {
		t.Fatalf("public incident list status = %d, body = %s", publicListResp.Code, publicListResp.Body.String())
	}
	if strings.Contains(publicListResp.Body.String(), created.Data.Incident.Title) {
		t.Fatalf("public incident list exposed draft incident: %s", publicListResp.Body.String())
	}
}
func TestStatusPageThemeSettingsValidationAndPublicProjection(t *testing.T) {
	server := setupTestServer(t)
	createPageResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages", gin.H{"slug": "branded-status", "title": "Branded Status", "description": "Customer-facing availability", "visibility": statusPageVisibilityPublic, "theme_settings": gin.H{"accent_color": "#2AB3C4", "component_density": "compact", "header_style": "centered", "logo_alt": "  Acme status logo  ", "logo_url": "https://cdn.acme.test/logo.svg", "open_graph_site_name": "Acme Trust", "open_graph_type": "website", "show_incident_history": false, "show_uptime_summary": true, "theme_mode": "dark"}}, "")
	if createPageResp.Code != http.StatusCreated {
		t.Fatalf("create themed page status = %d, body = %s", createPageResp.Code, createPageResp.Body.String())
	}
	var createdPage struct {
		Data struct {
			Page struct {
				ThemeSettings map[string]any `json:"theme_settings"`
			} `json:"page"`
		} `json:"data"`
	}
	decodeResponse(t, createPageResp, &createdPage)
	if createdPage.Data.Page.ThemeSettings["accent_color"] != "#2ab3c4" || createdPage.Data.Page.ThemeSettings["logo_alt"] != "Acme status logo" || createdPage.Data.Page.ThemeSettings["header_style"] != "centered" || createdPage.Data.Page.ThemeSettings["component_density"] != "compact" || createdPage.Data.Page.ThemeSettings["show_incident_history"] != false || createdPage.Data.Page.ThemeSettings["theme_mode"] != "dark" {
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
					ThemeSettings map[string]any `json:"theme_settings"`
				} `json:"page"`
			} `json:"status_page"`
		} `json:"data"`
	}
	decodeResponse(t, publicResp, &publicPayload)
	if publicPayload.Data.StatusPage.Page.ThemeSettings["accent_color"] != "#2ab3c4" || publicPayload.Data.StatusPage.Page.ThemeSettings["logo_url"] != "https://cdn.acme.test/logo.svg" || publicPayload.Data.StatusPage.Page.ThemeSettings["open_graph_site_name"] != "Acme Trust" {
		t.Fatalf("public theme settings = %+v, want public sanitized theme values", publicPayload.Data.StatusPage.Page.ThemeSettings)
	}
	invalidSettings := []gin.H{{"accent_color": "green"}, {"logo_url": "javascript://status.example.test/logo.svg"}, {"header_style": "hero"}, {"component_density": "dense"}, {"theme_mode": "sepia"}, {"show_uptime_summary": "true"}, {"open_graph_type": "article"}, {"accent": "green"}}
	for index, themeSettings := range invalidSettings {
		resp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages", gin.H{"slug": fmt.Sprintf("invalid-theme-%d", index), "title": fmt.Sprintf("Invalid Theme %d", index), "theme_settings": themeSettings}, "")
		if resp.Code != http.StatusBadRequest {
			t.Fatalf("invalid theme %d status = %d, body = %s, want 400", index, resp.Code, resp.Body.String())
		}
	}
}
func TestPublicStatusPageProjectionSanitizesLegacyThemeSettings(t *testing.T) {
	server := setupTestServer(t)
	now := time.Date(2026, 5, 29, 17, 15, 0, 0, time.UTC)
	page := db.StatusPage{ID: "status-page-legacy-theme", Slug: "legacy-theme-status", Title: "Legacy Theme Status", Description: "Customer-facing availability", Visibility: statusPageVisibilityPublic, ThemeSettings: `{"accent_color":"#FFAA00","component_density":"compact","header_style":"centered","logo_alt":" Legacy logo ","logo_url":"javascript://do-not-render","mode":"private-dark-mode","internal_note":"do-not-leak","open_graph_description_extra":"bad-og-description","open_graph_site_name":"Legacy Trust","open_graph_title":"Legacy Public Title","open_graph_type":"article","show_incident_history":"false","show_uptime_summary":true}`, DefaultIncidentVisibility: statusPageIncidentVisibilityDraft, PublishedAt: &now, CreatedAt: now, UpdatedAt: now}
	section := db.StatusPageSection{ID: "legacy-theme-section", StatusPageID: page.ID, Name: "Public services", CreatedAt: now, UpdatedAt: now}
	component := db.StatusPageComponent{ID: "legacy-theme-component", StatusPageID: page.ID, SectionID: section.ID, PublicName: "Public API", DisplayMode: "manual", ManualStatus: "operational", Visible: true, CreatedAt: now, UpdatedAt: now}
	incident := db.StatusPageIncident{ID: "legacy-theme-incident", StatusPageID: page.ID, Title: "Public maintenance notice", PublicStatus: "resolved", Severity: "low", ImpactSummary: "Maintenance completed.", Visibility: statusPageIncidentVisibilityPublished, PublishedAt: &now, ResolvedAt: &now, CreatedAt: now, UpdatedAt: now}
	update := db.StatusPageIncidentUpdate{ID: "legacy-theme-update", IncidentID: incident.ID, Status: "resolved", Message: "Maintenance is complete.", PublishedAt: &now, CreatedAt: now}
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
	if err := server.db.Create(&update).Error; err != nil {
		t.Fatalf("create incident update: %v", err)
	}
	publicResp := performJSONRequest(t, server, http.MethodGet, "/status/legacy-theme-status", nil, "")
	if publicResp.Code != http.StatusOK {
		t.Fatalf("public status = %d, body = %s", publicResp.Code, publicResp.Body.String())
	}
	var publicPayload struct {
		Data struct {
			StatusPage StatusPagePreviewResponse `json:"status_page"`
		} `json:"data"`
	}
	decodeResponse(t, publicResp, &publicPayload)
	assertLegacyThemePublicProjection(t, publicPayload.Data.StatusPage.Page.ThemeSettings)
	if publicPayload.Data.StatusPage.Metadata.OpenGraph.Title != "Legacy Public Title" || publicPayload.Data.StatusPage.Metadata.OpenGraph.SiteName != "Legacy Trust" || publicPayload.Data.StatusPage.Metadata.OpenGraph.Type != "website" {
		t.Fatalf("metadata = %+v, want sanitized public metadata", publicPayload.Data.StatusPage.Metadata.OpenGraph)
	}
	assertPublicStatusPageBodyDoesNotContainLegacyThemeLeaks(t, publicResp.Body.String())
	previewResp := performJSONRequest(t, server, http.MethodGet, "/v1/status-pages/"+page.ID+"/preview", nil, "")
	if previewResp.Code != http.StatusOK {
		t.Fatalf("preview status = %d, body = %s", previewResp.Code, previewResp.Body.String())
	}
	var previewPayload struct {
		Data struct {
			Preview StatusPagePreviewResponse `json:"preview"`
		} `json:"data"`
	}
	decodeResponse(t, previewResp, &previewPayload)
	assertLegacyThemePublicProjection(t, previewPayload.Data.Preview.Page.ThemeSettings)
	assertPublicStatusPageBodyDoesNotContainLegacyThemeLeaks(t, previewResp.Body.String())
	historyResp := performJSONRequest(t, server, http.MethodGet, "/status/legacy-theme-status/history?window=7d", nil, "")
	if historyResp.Code != http.StatusOK {
		t.Fatalf("history status = %d, body = %s", historyResp.Code, historyResp.Body.String())
	}
	assertPublicStatusPageBodyDoesNotContainLegacyThemeLeaks(t, historyResp.Body.String())
	htmlReq := httptest.NewRequest(http.MethodGet, "/status/legacy-theme-status", nil)
	htmlReq.Header.Set("Accept", "text/html")
	htmlResp := httptest.NewRecorder()
	server.router.ServeHTTP(htmlResp, htmlReq)
	if htmlResp.Code != http.StatusOK {
		t.Fatalf("HTML status = %d, body = %s", htmlResp.Code, htmlResp.Body.String())
	}
	assertContains(t, htmlResp.Body.String(), `<meta property="og:title" content="Legacy Public Title">`)
	assertContains(t, htmlResp.Body.String(), `--accent: #ffaa00`)
	assertPublicStatusPageBodyDoesNotContainLegacyThemeLeaks(t, htmlResp.Body.String())
	feedResp := performJSONRequest(t, server, http.MethodGet, "/status/legacy-theme-status/feed.atom", nil, "")
	if feedResp.Code != http.StatusOK {
		t.Fatalf("feed status = %d, body = %s", feedResp.Code, feedResp.Body.String())
	}
	assertPublicStatusPageBodyDoesNotContainLegacyThemeLeaks(t, feedResp.Body.String())
	badgeResp := performJSONRequest(t, server, http.MethodGet, "/status/legacy-theme-status/badge.svg", nil, "")
	if badgeResp.Code != http.StatusOK {
		t.Fatalf("badge status = %d, body = %s", badgeResp.Code, badgeResp.Body.String())
	}
	assertPublicStatusPageBodyDoesNotContainLegacyThemeLeaks(t, badgeResp.Body.String())
	subscriberResp := performJSONRequest(t, server, http.MethodPost, "/status/legacy-theme-status/subscribers", gin.H{"destination": "legacy-subscriber@example.com", "component_ids": []string{component.ID}}, "")
	if subscriberResp.Code != http.StatusAccepted {
		t.Fatalf("subscriber status = %d, body = %s", subscriberResp.Code, subscriberResp.Body.String())
	}
	assertPublicStatusPageBodyDoesNotContainLegacyThemeLeaks(t, subscriberResp.Body.String())
}
func assertLegacyThemePublicProjection(t *testing.T, settings map[string]interface{}) {
	t.Helper()
	if settings["accent_color"] != "#ffaa00" || settings["component_density"] != "compact" || settings["header_style"] != "centered" || settings["logo_alt"] != "Legacy logo" || settings["open_graph_site_name"] != "Legacy Trust" || settings["open_graph_title"] != "Legacy Public Title" || settings["show_uptime_summary"] != true {
		t.Fatalf("public theme settings = %+v, want only sanitized supported values", settings)
	}
	for _, key := range []string{"internal_note", "logo_url", "mode", "open_graph_description_extra", "open_graph_type", "show_incident_history"} {
		if _, ok := settings[key]; ok {
			t.Fatalf("public theme settings = %+v, want %q omitted", settings, key)
		}
	}
}
func assertPublicStatusPageBodyDoesNotContainLegacyThemeLeaks(t *testing.T, body string) {
	t.Helper()
	for _, value := range []string{"bad-og-description", "do-not-leak", "do-not-render", "javascript:", "private-dark-mode"} {
		assertNotContains(t, body, value)
	}
}
