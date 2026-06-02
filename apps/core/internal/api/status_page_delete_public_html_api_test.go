package api

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"testing"
	"time"
)

func TestStatusPageAdminDeleteRoutesRemoveNestedRecords(t *testing.T) {
	server := setupTestServer(t)
	registered := registerTestAgent(t, server)
	registeredMonitor := registerTestMonitor(t, server, registered.Data.AgentID, registered.Data.Token)
	createPageResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages", gin.H{"slug": "delete-status", "title": "Delete Status"}, "")
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
	createSection := func(name string) string {
		resp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+createdPage.Data.Page.ID+"/sections", gin.H{"name": name}, "")
		if resp.Code != http.StatusCreated {
			t.Fatalf("create section status = %d, body = %s", resp.Code, resp.Body.String())
		}
		var payload struct {
			Data struct {
				Section struct {
					ID string `json:"id"`
				} `json:"section"`
			} `json:"data"`
		}
		decodeResponse(t, resp, &payload)
		return payload.Data.Section.ID
	}
	createComponent := func(sectionID string, name string) string {
		resp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+createdPage.Data.Page.ID+"/components", gin.H{"section_id": sectionID, "public_name": name, "display_mode": "single_resource", "visible": true}, "")
		if resp.Code != http.StatusCreated {
			t.Fatalf("create component status = %d, body = %s", resp.Code, resp.Body.String())
		}
		var payload struct {
			Data struct {
				Component struct {
					ID string `json:"id"`
				} `json:"component"`
			} `json:"data"`
		}
		decodeResponse(t, resp, &payload)
		return payload.Data.Component.ID
	}
	createMapping := func(componentID string) string {
		resp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+createdPage.Data.Page.ID+"/components/"+componentID+"/mappings", gin.H{"resource_type": "monitor", "resource_id": registeredMonitor.Data.MonitorID}, "")
		if resp.Code != http.StatusCreated {
			t.Fatalf("create mapping status = %d, body = %s", resp.Code, resp.Body.String())
		}
		var payload struct {
			Data struct {
				Mapping struct {
					ID string `json:"id"`
				} `json:"mapping"`
			} `json:"data"`
		}
		decodeResponse(t, resp, &payload)
		return payload.Data.Mapping.ID
	}
	createIncident := func(componentID string, title string) string {
		resp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+createdPage.Data.Page.ID+"/incidents", gin.H{"title": title, "affected_component_ids": []string{componentID}}, "")
		if resp.Code != http.StatusCreated {
			t.Fatalf("create public incident status = %d, body = %s", resp.Code, resp.Body.String())
		}
		var payload struct {
			Data struct {
				Incident struct {
					ID string `json:"id"`
				} `json:"incident"`
			} `json:"data"`
		}
		decodeResponse(t, resp, &payload)
		return payload.Data.Incident.ID
	}
	createDelivery := func(id string, subscriberID string, incidentID string) {
		delivery := db.StatusPageSubscriberDelivery{ID: id, SubscriberID: subscriberID, StatusPageID: createdPage.Data.Page.ID, PublicIncidentID: incidentID, DeliveryType: statusPageSubscriberDeliveryTypeEmail, DeliveryState: statusPageSubscriberDeliveryStateSent}
		if err := server.db.Create(&delivery).Error; err != nil {
			t.Fatalf("create subscriber delivery: %v", err)
		}
	}
	countRows := func(model interface{}, query string, args ...interface{}) int64 {
		var count int64
		if err := server.db.Model(model).Where(query, args...).Count(&count).Error; err != nil {
			t.Fatalf("count rows for %T: %v", model, err)
		}
		return count
	}
	sectionID := createSection("API")
	componentID := createComponent(sectionID, "REST API")
	mappingID := createMapping(componentID)
	deleteMappingResp := performJSONRequest(t, server, http.MethodDelete, "/v1/status-pages/"+createdPage.Data.Page.ID+"/components/"+componentID+"/mappings/"+mappingID, nil, "")
	if deleteMappingResp.Code != http.StatusOK {
		t.Fatalf("delete mapping status = %d, body = %s", deleteMappingResp.Code, deleteMappingResp.Body.String())
	}
	if countRows(&db.StatusPageComponentMapping{}, "id = ?", mappingID) != 0 {
		t.Fatalf("mapping %q still exists after delete", mappingID)
	}
	mappingID = createMapping(componentID)
	now := time.Now().UTC()
	createdIncidentID := createIncident(componentID, "Customer API issue")
	createUpdateResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+createdPage.Data.Page.ID+"/incidents/"+createdIncidentID+"/updates", gin.H{"status": "investigating", "message": "We are investigating.", "published_at": now}, "")
	if createUpdateResp.Code != http.StatusCreated {
		t.Fatalf("create public incident update status = %d, body = %s", createUpdateResp.Code, createUpdateResp.Body.String())
	}
	incidentSubscriber := seedStatusPageSubscriberForTest(t, server, createdPage.Data.Page.ID, "incident-delete@example.com", statusPageSubscriberStateConfirmed, "incident-delete-confirm", "incident-delete-manage", "incident-delete-unsubscribe", []string{componentID})
	createDelivery("incident-delete-delivery", incidentSubscriber.ID, createdIncidentID)
	deleteIncidentResp := performJSONRequest(t, server, http.MethodDelete, "/v1/status-pages/"+createdPage.Data.Page.ID+"/incidents/"+createdIncidentID, nil, "")
	if deleteIncidentResp.Code != http.StatusOK {
		t.Fatalf("delete incident status = %d, body = %s", deleteIncidentResp.Code, deleteIncidentResp.Body.String())
	}
	if countRows(&db.StatusPageIncident{}, "id = ?", createdIncidentID) != 0 || countRows(&db.StatusPageIncidentUpdate{}, "incident_id = ?", createdIncidentID) != 0 || countRows(&db.StatusPageSubscriberDelivery{}, "public_incident_id = ?", createdIncidentID) != 0 {
		t.Fatalf("incident %q, its updates, or its deliveries still exist after delete", createdIncidentID)
	}
	componentPruneIncidentID := createIncident(componentID, "Component prune issue")
	componentSubscriber := seedStatusPageSubscriberForTest(t, server, createdPage.Data.Page.ID, "component-delete@example.com", statusPageSubscriberStateConfirmed, "component-delete-confirm", "component-delete-manage", "component-delete-unsubscribe", []string{componentID})
	createDelivery("component-delete-delivery", componentSubscriber.ID, componentPruneIncidentID)
	deleteComponentResp := performJSONRequest(t, server, http.MethodDelete, "/v1/status-pages/"+createdPage.Data.Page.ID+"/components/"+componentID, nil, "")
	if deleteComponentResp.Code != http.StatusOK {
		t.Fatalf("delete component status = %d, body = %s", deleteComponentResp.Code, deleteComponentResp.Body.String())
	}
	if countRows(&db.StatusPageComponent{}, "id = ?", componentID) != 0 || countRows(&db.StatusPageComponentMapping{}, "id = ?", mappingID) != 0 {
		t.Fatalf("component %q or mapping %q still exists after delete", componentID, mappingID)
	}
	if countRows(&db.StatusPageSubscriberComponent{}, "subscriber_id = ?", componentSubscriber.ID) != 0 {
		t.Fatalf("subscriber component preferences still exist after component delete")
	}
	var componentPruneIncident db.StatusPageIncident
	if err := server.db.Where("id = ?", componentPruneIncidentID).First(&componentPruneIncident).Error; err != nil {
		t.Fatalf("load component prune incident: %v", err)
	}
	if got := decodeResponseList(componentPruneIncident.AffectedComponentIDs, nil); len(got) != 0 {
		t.Fatalf("incident affected components after component delete = %+v, want empty", got)
	}
	componentID = createComponent(sectionID, "GraphQL API")
	sectionPruneIncidentID := createIncident(componentID, "Section prune issue")
	sectionSubscriber := seedStatusPageSubscriberForTest(t, server, createdPage.Data.Page.ID, "section-delete@example.com", statusPageSubscriberStateConfirmed, "section-delete-confirm", "section-delete-manage", "section-delete-unsubscribe", []string{componentID})
	deleteSectionResp := performJSONRequest(t, server, http.MethodDelete, "/v1/status-pages/"+createdPage.Data.Page.ID+"/sections/"+sectionID, nil, "")
	if deleteSectionResp.Code != http.StatusOK {
		t.Fatalf("delete section status = %d, body = %s", deleteSectionResp.Code, deleteSectionResp.Body.String())
	}
	if countRows(&db.StatusPageSection{}, "id = ?", sectionID) != 0 || countRows(&db.StatusPageComponent{}, "id = ?", componentID) != 0 {
		t.Fatalf("section %q or component %q still exists after delete", sectionID, componentID)
	}
	if countRows(&db.StatusPageSubscriberComponent{}, "subscriber_id = ?", sectionSubscriber.ID) != 0 {
		t.Fatalf("subscriber component preferences still exist after section delete")
	}
	var sectionPruneIncident db.StatusPageIncident
	if err := server.db.Where("id = ?", sectionPruneIncidentID).First(&sectionPruneIncident).Error; err != nil {
		t.Fatalf("load section prune incident: %v", err)
	}
	if got := decodeResponseList(sectionPruneIncident.AffectedComponentIDs, nil); len(got) != 0 {
		t.Fatalf("incident affected components after section delete = %+v, want empty", got)
	}
	sectionID = createSection("Web")
	componentID = createComponent(sectionID, "Web App")
	_ = createMapping(componentID)
	pageIncidentID := createIncident(componentID, "Page delete issue")
	pageSubscriber := seedStatusPageSubscriberForTest(t, server, createdPage.Data.Page.ID, "page-delete@example.com", statusPageSubscriberStateConfirmed, "page-delete-confirm", "page-delete-manage", "page-delete-unsubscribe", []string{componentID})
	createDelivery("page-delete-delivery", pageSubscriber.ID, pageIncidentID)
	deletePageResp := performJSONRequest(t, server, http.MethodDelete, "/v1/status-pages/"+createdPage.Data.Page.ID, nil, "")
	if deletePageResp.Code != http.StatusOK {
		t.Fatalf("delete page status = %d, body = %s", deletePageResp.Code, deletePageResp.Body.String())
	}
	if countRows(&db.StatusPage{}, "id = ?", createdPage.Data.Page.ID) != 0 || countRows(&db.StatusPageSection{}, "status_page_id = ?", createdPage.Data.Page.ID) != 0 || countRows(&db.StatusPageComponent{}, "status_page_id = ?", createdPage.Data.Page.ID) != 0 || countRows(&db.StatusPageComponentMapping{}, "component_id = ?", componentID) != 0 || countRows(&db.StatusPageIncident{}, "status_page_id = ?", createdPage.Data.Page.ID) != 0 || countRows(&db.StatusPageIncidentUpdate{}, "incident_id = ?", pageIncidentID) != 0 || countRows(&db.StatusPageSubscriber{}, "status_page_id = ?", createdPage.Data.Page.ID) != 0 || countRows(&db.StatusPageSubscriberComponent{}, "subscriber_id = ?", pageSubscriber.ID) != 0 || countRows(&db.StatusPageSubscriberDelivery{}, "status_page_id = ?", createdPage.Data.Page.ID) != 0 {
		t.Fatalf("status page %q or nested rows still exist after delete", createdPage.Data.Page.ID)
	}
	for _, action := range []string{service.StatusPageAuditActionDeleted, service.StatusPageAuditActionSectionDeleted, service.StatusPageAuditActionComponentDeleted, service.StatusPageAuditActionComponentMappingDeleted, service.StatusPageAuditActionPublicIncidentDeleted} {
		if countRows(&db.AuditEvent{}, "action = ?", action) == 0 {
			t.Fatalf("missing audit event for %s", action)
		}
	}
}
func TestPublicStatusPageMetadataProjectionUsesSafeDefaultsAndConfiguredFields(t *testing.T) {
	server := setupTestServer(t)
	now := time.Now().UTC()
	pages := []db.StatusPage{{ID: "status-page-metadata-default", Slug: "metadata-default", Title: "Acme Status", Description: "Service availability", Visibility: statusPageVisibilityPublic, ThemeSettings: "{}", DefaultIncidentVisibility: statusPageIncidentVisibilityDraft, PublishedAt: &now}, {ID: "status-page-metadata-configured", Slug: "metadata-configured", Title: "Acme Internal Page Title", Description: "Fallback public description", SEOTitle: "Acme availability", SEODescription: "Live platform state", OpenGraphImageURL: "https://cdn.acme.test/status.png", CanonicalURL: "https://status.acme.test/", Visibility: statusPageVisibilityUnlisted, ThemeSettings: `{
				"open_graph_description": "Realtime availability for Acme",
				"open_graph_site_name": "Acme Trust",
				"open_graph_title": "Acme Status Updates",
				"open_graph_type": "website"
			}`, DefaultIncidentVisibility: statusPageIncidentVisibilityDraft, PublishedAt: &now}, {ID: "status-page-metadata-draft", Slug: "metadata-draft", Title: "Draft Status", Visibility: statusPageVisibilityDraft, ThemeSettings: "{}", DefaultIncidentVisibility: statusPageIncidentVisibilityDraft}}
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
	if defaultMetadata.Title != "Acme Status" || defaultMetadata.Description != "Service availability" || defaultMetadata.CanonicalURL != "" || defaultMetadata.OpenGraph.Title != "Acme Status" || defaultMetadata.OpenGraph.Description != "Service availability" || defaultMetadata.OpenGraph.URL != "" || defaultMetadata.OpenGraph.Type != "website" || defaultMetadata.OpenGraph.SiteName != "Acme Status" || defaultMetadata.OpenGraph.ImageURL != "" {
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
	if configuredMetadata.Title != "Acme availability" || configuredMetadata.Description != "Live platform state" || configuredMetadata.CanonicalURL != "https://status.acme.test" || configuredMetadata.OpenGraph.Title != "Acme Status Updates" || configuredMetadata.OpenGraph.Description != "Realtime availability for Acme" || configuredMetadata.OpenGraph.URL != "https://status.acme.test" || configuredMetadata.OpenGraph.Type != "website" || configuredMetadata.OpenGraph.SiteName != "Acme Trust" || configuredMetadata.OpenGraph.ImageURL != "https://cdn.acme.test/status.png" {
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
	page := db.StatusPage{ID: "status-page-html", Slug: "html-status", CustomDomain: "status.acme.test", Title: "Acme Public Status", Description: "Customer-facing availability", SEOTitle: "Acme Status", SEODescription: "Public availability for Acme", OpenGraphImageURL: "https://cdn.acme.test/status.png", CanonicalURL: "https://status.acme.test/", Visibility: statusPageVisibilityPublic, ThemeSettings: `{"accent_color":"#10b981","component_density":"compact","header_style":"centered","logo_alt":"Acme logo","logo_url":"https://cdn.acme.test/logo.svg","open_graph_site_name":"Acme Trust","open_graph_title":"Acme Status Updates","open_graph_description":"Realtime public availability","open_graph_type":"website","theme_mode":"dark"}`, DefaultIncidentVisibility: statusPageIncidentVisibilityDraft, PublishedAt: &now, CreatedAt: now, UpdatedAt: now}
	section := db.StatusPageSection{ID: "status-page-html-section", StatusPageID: page.ID, Name: "Public services", CreatedAt: now, UpdatedAt: now}
	component := db.StatusPageComponent{ID: "status-page-html-component", StatusPageID: page.ID, SectionID: section.ID, PublicName: "Checkout API", PublicDescription: "Customer checkout traffic", DisplayMode: "manual", ManualStatus: "degraded", ManualStatusReason: "Elevated latency", Visible: true, CreatedAt: now, UpdatedAt: now}
	incident := db.StatusPageIncident{ID: "status-page-html-incident", StatusPageID: page.ID, Title: "Checkout latency", PublicStatus: "identified", Severity: "medium", ImpactSummary: "Some checkouts are slower than usual.", Visibility: statusPageIncidentVisibilityPublished, AffectedComponentIDs: `["status-page-html-component"]`, PublishedAt: &now, CreatedAt: now, UpdatedAt: now}
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
	assertContains(t, body, `<body class="theme-dark">`)
	assertContains(t, body, `data-subscribe-form`)
	assertContains(t, body, `class="uptime-bars"`)
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
