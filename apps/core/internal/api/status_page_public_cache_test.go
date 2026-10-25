package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"orion/core/internal/db"
)

func TestPublicStatusPageReadRoutesUseETagsAndCacheHeaders(t *testing.T) {
	server, fixture := setupPublicStatusPageCacheFixture(t)

	routes := []string{
		"/status/" + fixture.slug,
		"/status/" + fixture.slug + "/history",
		"/status/" + fixture.slug + "/incidents",
		"/status/" + fixture.slug + "/incidents/" + fixture.incidentID,
		"/status/" + fixture.slug + "/incidents/" + fixture.incidentID + "/history",
		"/status/" + fixture.slug + "/components/" + fixture.componentID + "/uptime",
		"/status/" + fixture.slug + "/components/" + fixture.componentID + "/history",
		"/status/" + fixture.slug + "/feed.atom",
	}

	for _, path := range routes {
		t.Run(path, func(t *testing.T) {
			first := performRequestWithHeaders(t, server, http.MethodGet, path, nil)
			if first.Code != http.StatusOK {
				t.Fatalf("first response status = %d, body = %s", first.Code, first.Body.String())
			}
			assertPublicCacheHeaders(t, first)
			for _, privateValue := range []string{
				"Private database maintenance",
				"Draft operator note",
				"private-host.internal",
				"secret-token",
				"internal-operator",
			} {
				assertNotContains(t, first.Body.String(), privateValue)
			}

			second := performRequestWithHeaders(t, server, http.MethodGet, path, map[string]string{
				"If-None-Match": first.Header().Get("ETag"),
			})
			if second.Code != http.StatusNotModified {
				t.Fatalf("second response status = %d, body = %s, want 304", second.Code, second.Body.String())
			}
			assertPublicCacheHeaders(t, second)
			if second.Body.Len() != 0 {
				t.Fatalf("304 response body = %q, want empty", second.Body.String())
			}
		})
	}

	preview := performJSONRequest(t, server, http.MethodGet, "/v1/status-pages/"+fixture.pageID+"/preview", nil, "")
	if preview.Code != http.StatusOK {
		t.Fatalf("admin preview status = %d, body = %s", preview.Code, preview.Body.String())
	}
	if preview.Header().Get("ETag") != "" || preview.Header().Get("Cache-Control") != "" {
		t.Fatalf("admin preview cache headers = ETag %q Cache-Control %q, want none", preview.Header().Get("ETag"), preview.Header().Get("Cache-Control"))
	}
}

func TestPublicStatusPageETagChangesAfterPublishedIncidentUpdate(t *testing.T) {
	server, fixture := setupPublicStatusPageCacheFixture(t)
	historyPath := "/status/" + fixture.slug + "/incidents/" + fixture.incidentID + "/history"
	feedPath := "/status/" + fixture.slug + "/feed.atom"

	initialHistory := performRequestWithHeaders(t, server, http.MethodGet, historyPath, nil)
	if initialHistory.Code != http.StatusOK {
		t.Fatalf("initial history status = %d, body = %s", initialHistory.Code, initialHistory.Body.String())
	}
	initialFeed := performRequestWithHeaders(t, server, http.MethodGet, feedPath, nil)
	if initialFeed.Code != http.StatusOK {
		t.Fatalf("initial feed status = %d, body = %s", initialFeed.Code, initialFeed.Body.String())
	}

	now := time.Now().UTC()
	update := db.StatusPageIncidentUpdate{
		ID:          "cache-public-update-fresh",
		IncidentID:  fixture.incidentID,
		Status:      "monitoring",
		Message:     "Customer-visible mitigation update",
		CreatedBy:   "internal-operator",
		PublishedAt: &now,
		CreatedAt:   now,
	}
	if err := server.db.Create(&update).Error; err != nil {
		t.Fatalf("create published update: %v", err)
	}

	updatedHistory := performRequestWithHeaders(t, server, http.MethodGet, historyPath, map[string]string{
		"If-None-Match": initialHistory.Header().Get("ETag"),
	})
	if updatedHistory.Code != http.StatusOK {
		t.Fatalf("updated history status = %d, body = %s, want 200", updatedHistory.Code, updatedHistory.Body.String())
	}
	if updatedHistory.Header().Get("ETag") == initialHistory.Header().Get("ETag") {
		t.Fatalf("history ETag did not change after published update: %q", updatedHistory.Header().Get("ETag"))
	}
	assertContains(t, updatedHistory.Body.String(), "Customer-visible mitigation update")
	assertNotContains(t, updatedHistory.Body.String(), "internal-operator")

	updatedFeed := performRequestWithHeaders(t, server, http.MethodGet, feedPath, map[string]string{
		"If-None-Match": initialFeed.Header().Get("ETag"),
	})
	if updatedFeed.Code != http.StatusOK {
		t.Fatalf("updated feed status = %d, body = %s, want 200", updatedFeed.Code, updatedFeed.Body.String())
	}
	if updatedFeed.Header().Get("ETag") == initialFeed.Header().Get("ETag") {
		t.Fatalf("feed ETag did not change after published update: %q", updatedFeed.Header().Get("ETag"))
	}
	assertContains(t, updatedFeed.Body.String(), "Customer-visible mitigation update")
	assertNotContains(t, updatedFeed.Body.String(), "internal-operator")
}

type publicStatusPageCacheFixture struct {
	pageID      string
	slug        string
	componentID string
	incidentID  string
}

func setupPublicStatusPageCacheFixture(t *testing.T) (*Server, publicStatusPageCacheFixture) {
	t.Helper()

	server := setupTestServer(t)
	now := time.Date(2026, 5, 27, 15, 30, 0, 0, time.UTC)
	publishedAt := now.Add(-10 * time.Minute)
	page := db.StatusPage{
		ID:                        "cache-public-page",
		Slug:                      "cache-status",
		Title:                     "Cache Status",
		Description:               "Customer-facing status.",
		Visibility:                "public",
		ThemeSettings:             "{}",
		DefaultIncidentVisibility: "draft",
		PublishedAt:               &publishedAt,
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}
	section := db.StatusPageSection{
		ID:           "cache-public-section",
		StatusPageID: page.ID,
		Name:         "Core",
		SortOrder:    1,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	component := db.StatusPageComponent{
		ID:                 "cache-public-component",
		StatusPageID:       page.ID,
		SectionID:          section.ID,
		PublicName:         "API",
		PublicDescription:  "Public API",
		DisplayMode:        "single_resource",
		ManualStatus:       "degraded",
		ManualStatusReason: "Some requests are slower than usual.",
		SortOrder:          1,
		Visible:            true,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	hiddenComponent := db.StatusPageComponent{
		ID:           "cache-hidden-component",
		StatusPageID: page.ID,
		SectionID:    section.ID,
		PublicName:   "private-host.internal",
		DisplayMode:  "single_resource",
		Visible:      false,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	publicIncident := db.StatusPageIncident{
		ID:                   "cache-public-incident",
		StatusPageID:         page.ID,
		Title:                "Elevated API latency",
		PublicStatus:         "identified",
		Severity:             "minor",
		ImpactSummary:        "Some customer API requests are slower than usual.",
		Visibility:           "published",
		AffectedComponentIDs: `["cache-public-component"]`,
		PublishedAt:          &publishedAt,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	privateIncident := db.StatusPageIncident{
		ID:            "cache-private-incident",
		StatusPageID:  page.ID,
		Title:         "Private database maintenance",
		PublicStatus:  "investigating",
		Severity:      "major",
		ImpactSummary: "secret-token",
		Visibility:    "private",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	draftIncident := db.StatusPageIncident{
		ID:            "cache-draft-incident",
		StatusPageID:  page.ID,
		Title:         "Draft operator note",
		PublicStatus:  "investigating",
		Severity:      "minor",
		ImpactSummary: "Not ready for customers.",
		Visibility:    "draft",
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	updates := []db.StatusPageIncidentUpdate{
		{
			ID:          "cache-public-update-initial",
			IncidentID:  publicIncident.ID,
			Status:      "identified",
			Message:     "We identified the latency source.",
			CreatedBy:   "ops",
			PublishedAt: &publishedAt,
			CreatedAt:   now,
		},
		{
			ID:         "cache-private-update",
			IncidentID: publicIncident.ID,
			Status:     "investigating",
			Message:    "secret-token internal runbook",
			CreatedBy:  "internal-operator",
			CreatedAt:  now,
		},
	}

	if err := server.db.Create(&page).Error; err != nil {
		t.Fatalf("create status page: %v", err)
	}
	if err := server.db.Create(&section).Error; err != nil {
		t.Fatalf("create section: %v", err)
	}
	if err := server.db.Create(&[]db.StatusPageComponent{component, hiddenComponent}).Error; err != nil {
		t.Fatalf("create components: %v", err)
	}
	if err := server.db.Model(&db.StatusPageComponent{}).Where("id = ?", hiddenComponent.ID).Update("visible", false).Error; err != nil {
		t.Fatalf("hide component: %v", err)
	}
	if err := server.db.Create(&[]db.StatusPageIncident{publicIncident, privateIncident, draftIncident}).Error; err != nil {
		t.Fatalf("create incidents: %v", err)
	}
	if err := server.db.Create(&updates).Error; err != nil {
		t.Fatalf("create updates: %v", err)
	}

	return server, publicStatusPageCacheFixture{
		pageID:      page.ID,
		slug:        page.Slug,
		componentID: component.ID,
		incidentID:  publicIncident.ID,
	}
}

func performRequestWithHeaders(t *testing.T, server *Server, method string, path string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, path, nil)
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	recorder := httptest.NewRecorder()
	server.router.ServeHTTP(recorder, req)
	return recorder
}

func assertPublicCacheHeaders(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()

	if response.Header().Get("Cache-Control") != statusPagePublicCacheControl {
		t.Fatalf("Cache-Control = %q, want %q", response.Header().Get("Cache-Control"), statusPagePublicCacheControl)
	}
	etag := response.Header().Get("ETag")
	if !strings.HasPrefix(etag, `"`) || !strings.HasSuffix(etag, `"`) || len(etag) < 4 {
		t.Fatalf("ETag = %q, want quoted deterministic hash", etag)
	}
}
