package api

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"orion/core/internal/db"
)

func TestPublicStatusPageBadgeRendersCurrentStatusAndCacheHeaders(t *testing.T) {
	server, page, visibleComponent, monitorID := setupStatusPageBadgeTestData(t)

	resp := performJSONRequest(t, server, http.MethodGet, "/status/"+page.Slug+"/badge.svg", nil, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("page badge status = %d, body = %s", resp.Code, resp.Body.String())
	}
	if contentType := resp.Header().Get("Content-Type"); !strings.Contains(contentType, "image/svg+xml") {
		t.Fatalf("page badge content-type = %q, want image/svg+xml", contentType)
	}
	if cacheControl := resp.Header().Get("Cache-Control"); cacheControl != statusPageBadgeCacheControl {
		t.Fatalf("page badge cache-control = %q, want %q", cacheControl, statusPageBadgeCacheControl)
	}
	body := resp.Body.String()
	if !strings.Contains(body, "Customer Status") || !strings.Contains(body, "major outage") {
		t.Fatalf("page badge body = %s, want public title and current status", body)
	}
	assertNotContains(t, body, monitorID)
	assertNotContains(t, body, "agent-private-badge")
	assertNotContains(t, body, "mapping-private-badge")
	assertNotContains(t, body, visibleComponent.ManualStatusReason)
}

func TestPublicStatusPageComponentBadgeRendersVisibleComponent(t *testing.T) {
	server, page, visibleComponent, monitorID := setupStatusPageBadgeTestData(t)

	resp := performJSONRequest(t, server, http.MethodGet, "/status/"+page.Slug+"/components/"+visibleComponent.ID+"/badge.svg", nil, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("component badge status = %d, body = %s", resp.Code, resp.Body.String())
	}
	body := resp.Body.String()
	if !strings.Contains(body, "Public API") || !strings.Contains(body, "major outage") {
		t.Fatalf("component badge body = %s, want public component name and current status", body)
	}
	assertNotContains(t, body, visibleComponent.ID)
	assertNotContains(t, body, monitorID)
	assertNotContains(t, body, "agent-private-badge")
}

func TestPublicStatusPageComponentBadgeReturnsNotFoundForHiddenComponent(t *testing.T) {
	server, page, _, _ := setupStatusPageBadgeTestData(t)

	resp := performJSONRequest(t, server, http.MethodGet, "/status/"+page.Slug+"/components/component-hidden-badge/badge.svg", nil, "")
	if resp.Code != http.StatusNotFound {
		t.Fatalf("hidden component badge status = %d, body = %s, want 404", resp.Code, resp.Body.String())
	}
	assertNotContains(t, resp.Body.String(), "Hidden Database")
}

func TestPublicStatusPageBadgesRequirePublicOrUnlistedPage(t *testing.T) {
	server, _, visibleComponent, _ := setupStatusPageBadgeTestData(t)
	if err := server.db.Model(&db.StatusPage{}).Where("slug = ?", "customer-status").Update("visibility", "draft").Error; err != nil {
		t.Fatalf("mark page draft: %v", err)
	}

	pageResp := performJSONRequest(t, server, http.MethodGet, "/status/customer-status/badge.svg", nil, "")
	if pageResp.Code != http.StatusNotFound {
		t.Fatalf("draft page badge status = %d, body = %s, want 404", pageResp.Code, pageResp.Body.String())
	}

	componentResp := performJSONRequest(t, server, http.MethodGet, "/status/customer-status/components/"+visibleComponent.ID+"/badge.svg", nil, "")
	if componentResp.Code != http.StatusNotFound {
		t.Fatalf("draft component badge status = %d, body = %s, want 404", componentResp.Code, componentResp.Body.String())
	}
}

func setupStatusPageBadgeTestData(t *testing.T) (*Server, db.StatusPage, db.StatusPageComponent, string) {
	t.Helper()

	server := setupTestServer(t)
	now := time.Now().UTC()
	publishedAt := now.Add(-time.Hour)
	page := db.StatusPage{
		ID:                        "status-page-badge-test",
		Slug:                      "customer-status",
		Title:                     "Customer Status",
		Visibility:                statusPageVisibilityPublic,
		ThemeSettings:             "{}",
		DefaultIncidentVisibility: statusPageIncidentVisibilityDraft,
		PublishedAt:               &publishedAt,
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}
	section := db.StatusPageSection{
		ID:           "section-badge-test",
		StatusPageID: page.ID,
		Name:         "Core",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	visibleComponent := db.StatusPageComponent{
		ID:                 "component-visible-badge",
		StatusPageID:       page.ID,
		SectionID:          section.ID,
		PublicName:         "Public API",
		DisplayMode:        "single_resource",
		ManualStatusReason: "internal-only diagnostic context",
		Visible:            true,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	hiddenComponent := db.StatusPageComponent{
		ID:           "component-hidden-badge",
		StatusPageID: page.ID,
		SectionID:    section.ID,
		PublicName:   "Hidden Database",
		DisplayMode:  "single_resource",
		Visible:      false,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	agent := db.Agent{
		ID:        "agent-private-badge",
		MachineId: "machine-private-badge",
		Name:      "private host",
		OS:        "linux",
		Arch:      "arm64",
		Token:     "secret-agent-badge-token",
		LastSeen:  now,
		CreatedAt: now,
	}
	monitor := db.Monitor{
		ID:             "monitor-private-badge",
		AgentID:        agent.ID,
		Name:           "private monitor",
		Type:           "http",
		Lifecycle:      "active",
		Health:         "down",
		ComputedHealth: "down",
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	mapping := db.StatusPageComponentMapping{
		ID:                   "mapping-private-badge",
		ComponentID:          visibleComponent.ID,
		ResourceType:         "monitor",
		ResourceID:           monitor.ID,
		HealthRollupStrategy: "worst",
		UptimeRollupStrategy: "worst",
		CreatedAt:            now,
		UpdatedAt:            now,
	}

	if err := server.db.Create(&page).Error; err != nil {
		t.Fatalf("create status page: %v", err)
	}
	if err := server.db.Create(&section).Error; err != nil {
		t.Fatalf("create section: %v", err)
	}
	if err := server.db.Create(&[]db.StatusPageComponent{visibleComponent, hiddenComponent}).Error; err != nil {
		t.Fatalf("create components: %v", err)
	}
	if err := server.db.Model(&db.StatusPageComponent{}).Where("id = ?", hiddenComponent.ID).Update("visible", false).Error; err != nil {
		t.Fatalf("hide component: %v", err)
	}
	if err := server.db.Create(&agent).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	if err := server.db.Create(&monitor).Error; err != nil {
		t.Fatalf("create monitor: %v", err)
	}
	if err := server.db.Create(&mapping).Error; err != nil {
		t.Fatalf("create mapping: %v", err)
	}

	return server, page, visibleComponent, monitor.ID
}
