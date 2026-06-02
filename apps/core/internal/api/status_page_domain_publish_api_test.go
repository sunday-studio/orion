package api

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"net/http"
	"net/http/httptest"
	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"strings"
	"testing"
	"time"
)

func TestPublicStatusPageMetadataDoesNotUseMappedInternalResources(t *testing.T) {
	server := setupTestServer(t)
	registered := registerTestAgent(t, server)
	registeredMonitor := registerTestMonitor(t, server, registered.Data.AgentID, registered.Data.Token)
	if err := server.db.Model(&db.Monitor{}).Where("id = ?", registeredMonitor.Data.MonitorID).Updates(map[string]any{"name": "internal-db-01.local", "computed_health": "up", "health": "up"}).Error; err != nil {
		t.Fatalf("update monitor: %v", err)
	}
	now := time.Now().UTC()
	page := db.StatusPage{ID: "status-page-metadata-redaction", Slug: "metadata-redaction", Title: "Customer Status", Description: "Customer-facing availability", Visibility: statusPageVisibilityPublic, ThemeSettings: "{}", DefaultIncidentVisibility: statusPageIncidentVisibilityDraft, PublishedAt: &now}
	section := db.StatusPageSection{ID: "status-page-metadata-redaction-section", StatusPageID: page.ID, Name: "Public services"}
	component := db.StatusPageComponent{ID: "status-page-metadata-redaction-component", StatusPageID: page.ID, SectionID: section.ID, PublicName: "Checkout API", DisplayMode: "single_resource", Visible: true}
	mapping := db.StatusPageComponentMapping{ID: "status-page-metadata-redaction-mapping", ComponentID: component.ID, ResourceType: "monitor", ResourceID: registeredMonitor.Data.MonitorID, HealthRollupStrategy: "worst", UptimeRollupStrategy: "worst"}
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
	createResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages", gin.H{"slug": "domain-status", "title": "Domain Status", "custom_domain": "Status.Example.COM:443"}, "")
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
	secondResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages", gin.H{"slug": "second-domain-status", "title": "Second Domain Status"}, "")
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
	conflictResp := performJSONRequest(t, server, http.MethodPut, "/v1/status-pages/"+second.Data.Page.ID, gin.H{"slug": "second-domain-status", "title": "Second Domain Status", "custom_domain": "status.example.com"}, "")
	if conflictResp.Code != http.StatusBadRequest {
		t.Fatalf("conflict status = %d, body = %s, want 400", conflictResp.Code, conflictResp.Body.String())
	}
	assertContains(t, conflictResp.Body.String(), "already in use")
	invalidDomains := []string{"localhost", "127.0.0.1", "*.example.com", "https://status.example.com/path", "status", "status.local"}
	for i, domain := range invalidDomains {
		resp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages", gin.H{"slug": fmt.Sprintf("invalid-domain-%d", i), "title": fmt.Sprintf("Invalid Domain %d", i), "custom_domain": domain}, "")
		if resp.Code != http.StatusBadRequest {
			t.Fatalf("domain %q status = %d, body = %s, want 400", domain, resp.Code, resp.Body.String())
		}
	}
}
func TestStatusPageCustomDomainHostRoutingAndIsolation(t *testing.T) {
	server := setupTestServer(t)
	now := time.Now().UTC()
	pages := []db.StatusPage{{ID: "status_page_custom_public", Slug: "custom-public", CustomDomain: "status.example.com", Title: "Custom Public", Visibility: statusPageVisibilityPublic, ThemeSettings: "{}", DefaultIncidentVisibility: statusPageIncidentVisibilityDraft, PublishedAt: &now}, {ID: "status_page_other_public", Slug: "other-public", CustomDomain: "other.example.com", Title: "Other Public", Visibility: statusPageVisibilityPublic, ThemeSettings: "{}", DefaultIncidentVisibility: statusPageIncidentVisibilityDraft, PublishedAt: &now}, {ID: "status_page_custom_draft", Slug: "custom-draft", CustomDomain: "draft.example.com", Title: "Custom Draft", Visibility: statusPageVisibilityDraft, ThemeSettings: "{}", DefaultIncidentVisibility: statusPageIncidentVisibilityDraft}}
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
	historyResp := performHostRequest(t, server, http.MethodGet, "/status/custom-public/history", "status.example.com")
	if historyResp.Code != http.StatusOK {
		t.Fatalf("custom host history status = %d, body = %s", historyResp.Code, historyResp.Body.String())
	}
	assertContains(t, historyResp.Body.String(), "Custom Public")
	assertNotContains(t, historyResp.Body.String(), "Other Public")
	otherHistoryOnCustomHostResp := performHostRequest(t, server, http.MethodGet, "/status/other-public/history", "status.example.com")
	if otherHistoryOnCustomHostResp.Code != http.StatusNotFound {
		t.Fatalf("other history on custom host status = %d, body = %s, want 404", otherHistoryOnCustomHostResp.Code, otherHistoryOnCustomHostResp.Body.String())
	}
	otherBadgeOnCustomHostResp := performHostRequest(t, server, http.MethodGet, "/status/other-public/badge.svg", "status.example.com")
	if otherBadgeOnCustomHostResp.Code != http.StatusNotFound {
		t.Fatalf("other badge on custom host status = %d, body = %s, want 404", otherBadgeOnCustomHostResp.Code, otherBadgeOnCustomHostResp.Body.String())
	}
	otherSubscriberOnCustomHostResp := performHostJSONRequest(t, server, http.MethodPost, "/status/other-public/subscribers", "status.example.com", gin.H{"destination": "other-status-subscriber@example.com"})
	if otherSubscriberOnCustomHostResp.Code != http.StatusNotFound {
		t.Fatalf("other subscriber on custom host status = %d, body = %s, want 404", otherSubscriberOnCustomHostResp.Code, otherSubscriberOnCustomHostResp.Body.String())
	}
	var subscriberCount int64
	if err := server.db.Model(&db.StatusPageSubscriber{}).Count(&subscriberCount).Error; err != nil {
		t.Fatalf("count subscribers: %v", err)
	}
	if subscriberCount != 0 {
		t.Fatalf("subscriber count = %d, want 0 after custom-domain mismatch", subscriberCount)
	}
}
func TestStatusPagePublishValidation(t *testing.T) {
	server := setupTestServer(t)
	createPageResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages", gin.H{"slug": "validation-status", "title": "Validation Status"}, "")
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
	duplicateSlugResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages", gin.H{"slug": "validation-status", "title": "Duplicate Validation Status"}, "")
	if duplicateSlugResp.Code != http.StatusConflict {
		t.Fatalf("duplicate slug status = %d, body = %s, want 409", duplicateSlugResp.Code, duplicateSlugResp.Body.String())
	}
	assertContains(t, duplicateSlugResp.Body.String(), "slug already exists")
	emptyPublishResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+createdPage.Data.Page.ID+"/publish", nil, "")
	if emptyPublishResp.Code != http.StatusBadRequest {
		t.Fatalf("empty publish status = %d, body = %s, want 400", emptyPublishResp.Code, emptyPublishResp.Body.String())
	}
	assertContains(t, emptyPublishResp.Body.String(), "at least one visible component")
	createSectionResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+createdPage.Data.Page.ID+"/sections", gin.H{"name": "Private"}, "")
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
	createComponentResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+createdPage.Data.Page.ID+"/components", gin.H{"section_id": createdSection.Data.Section.ID, "public_name": "localhost", "visible": true}, "")
	if createComponentResp.Code != http.StatusCreated {
		t.Fatalf("create component status = %d, body = %s", createComponentResp.Code, createComponentResp.Body.String())
	}
	unmappedPublishResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+createdPage.Data.Page.ID+"/publish", nil, "")
	if unmappedPublishResp.Code != http.StatusBadRequest {
		t.Fatalf("unmapped publish status = %d, body = %s, want 400", unmappedPublishResp.Code, unmappedPublishResp.Body.String())
	}
	assertContains(t, unmappedPublishResp.Body.String(), "mapped resource or manual status")
	assertContains(t, unmappedPublishResp.Body.String(), "looks like an internal host")
	ipPageResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages", gin.H{"slug": "ip-validation-status", "title": "IP Validation Status"}, "")
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
	ipSectionResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+ipPage.Data.Page.ID+"/sections", gin.H{"name": "IP Private"}, "")
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
	ipComponentResp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+ipPage.Data.Page.ID+"/components", gin.H{"section_id": ipSection.Data.Section.ID, "public_name": "192.168.1.10", "manual_status": "operational", "visible": true}, "")
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
func performHostJSONRequest(t *testing.T, server *Server, method string, path string, host string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}
	req := httptest.NewRequest(method, path, strings.NewReader(string(payload)))
	req.Host = host
	req.Header.Set("Content-Type", "application/json")
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
	return NewServer(database, utils.NewLogger(), &config.Config{FrontendAuthOn: true, AdminUsername: "admin", AdminPassword: "correct-password", JWTSecret: "test-secret", DataDir: t.TempDir()})
}
func loginStatusPageTestAdmin(t *testing.T, server *Server) string {
	t.Helper()
	loginResp := performJSONRequest(t, server, http.MethodPost, "/v1/auth/login", map[string]string{"username": "admin", "password": "correct-password"}, "")
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
