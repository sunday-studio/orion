package api

import (
	"encoding/xml"
	"net/http"
	"strings"
	"testing"
	"time"

	"orion/core/internal/db"
)

type testAtomFeed struct {
	XMLName xml.Name        `xml:"feed"`
	Title   string          `xml:"title"`
	Entries []testAtomEntry `xml:"entry"`
}

type testAtomEntry struct {
	Title   string `xml:"title"`
	Summary string `xml:"summary"`
	Content string `xml:"content"`
}

func TestStatusPageAtomFeedIncludesOnlyPublishedPublicIncidents(t *testing.T) {
	server := setupTestServer(t)
	now := time.Date(2026, 5, 27, 12, 0, 0, 0, time.UTC)
	publishedAt := now.Add(5 * time.Minute)
	updatePublishedAt := now.Add(10 * time.Minute)

	page := db.StatusPage{
		ID:                        "status-page-feed-test",
		Slug:                      "public-status",
		Title:                     "Public Status",
		Description:               "External status page",
		Visibility:                "public",
		ThemeSettings:             "{}",
		DefaultIncidentVisibility: "draft",
		PublishedAt:               &publishedAt,
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}
	if err := server.db.Create(&page).Error; err != nil {
		t.Fatalf("create status page: %v", err)
	}

	internalIncident := db.Incident{
		ID:                 "internal-secret-incident",
		Status:             "open",
		Severity:           "critical",
		Title:              "Internal database password leaked",
		AgentID:            "agent-secret",
		MonitorID:          "monitor-secret",
		OpenedAt:           now,
		LastEventAt:        now,
		LatestEvent:        "Internal diagnostic contains secret-token",
		NotificationStatus: "pending",
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := server.db.Create(&internalIncident).Error; err != nil {
		t.Fatalf("create internal incident: %v", err)
	}

	publishedIncident := db.StatusPageIncident{
		ID:                 "public-incident-api",
		StatusPageID:       page.ID,
		InternalIncidentID: internalIncident.ID,
		Title:              "API availability issue",
		PublicStatus:       "investigating",
		Severity:           "critical",
		ImpactSummary:      "Some API requests are failing.",
		Visibility:         "published",
		PublishedAt:        &publishedAt,
		CreatedAt:          now,
		UpdatedAt:          publishedAt,
	}
	draftIncident := db.StatusPageIncident{
		ID:            "draft-public-incident",
		StatusPageID:  page.ID,
		Title:         "Draft incident should not appear",
		PublicStatus:  "identified",
		Severity:      "minor",
		ImpactSummary: "Draft-only public copy",
		Visibility:    "draft",
		PublishedAt:   &publishedAt,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	privateIncident := db.StatusPageIncident{
		ID:            "private-public-incident",
		StatusPageID:  page.ID,
		Title:         "Private incident should not appear",
		PublicStatus:  "monitoring",
		Severity:      "major",
		ImpactSummary: "Private public copy",
		Visibility:    "private",
		PublishedAt:   &publishedAt,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := server.db.Create(&[]db.StatusPageIncident{publishedIncident, draftIncident, privateIncident}).Error; err != nil {
		t.Fatalf("create public incidents: %v", err)
	}

	if err := server.db.Create(&[]db.StatusPageIncidentUpdate{
		{
			ID:          "public-update",
			IncidentID:  publishedIncident.ID,
			Status:      "investigating",
			Message:     "We are investigating API errors.",
			PublishedAt: &updatePublishedAt,
			CreatedAt:   now,
		},
		{
			ID:         "draft-update",
			IncidentID: publishedIncident.ID,
			Status:     "identified",
			Message:    "Unpublished operator-only update.",
			CreatedAt:  now,
		},
	}).Error; err != nil {
		t.Fatalf("create public incident updates: %v", err)
	}

	resp := performJSONRequest(t, server, http.MethodGet, "/status/public-status/feed.atom", nil, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("feed status = %d, body = %s", resp.Code, resp.Body.String())
	}
	if contentType := resp.Header().Get("Content-Type"); !strings.Contains(contentType, "application/atom+xml") {
		t.Fatalf("content type = %q, want application/atom+xml", contentType)
	}

	var feed testAtomFeed
	if err := xml.Unmarshal(resp.Body.Bytes(), &feed); err != nil {
		t.Fatalf("parse feed XML: %v\n%s", err, resp.Body.String())
	}
	if feed.XMLName.Space != "http://www.w3.org/2005/Atom" || feed.XMLName.Local != "feed" {
		t.Fatalf("feed XML name = %#v, want Atom feed", feed.XMLName)
	}
	if feed.Title != "Public Status" {
		t.Fatalf("feed title = %q, want Public Status", feed.Title)
	}
	if len(feed.Entries) != 1 {
		t.Fatalf("feed entries = %d, want 1: %s", len(feed.Entries), resp.Body.String())
	}
	if feed.Entries[0].Title != "API availability issue" {
		t.Fatalf("entry title = %q, want public incident title", feed.Entries[0].Title)
	}

	body := resp.Body.String()
	assertContains(t, body, "Some API requests are failing.")
	assertContains(t, body, "We are investigating API errors.")
	assertNotContains(t, body, "Internal database password leaked")
	assertNotContains(t, body, "secret-token")
	assertNotContains(t, body, "Draft incident should not appear")
	assertNotContains(t, body, "Private incident should not appear")
	assertNotContains(t, body, "Unpublished operator-only update")
}

func TestStatusPageAtomFeedOmitsUnpublishedIncidents(t *testing.T) {
	server := setupTestServer(t)
	now := time.Date(2026, 5, 27, 12, 0, 0, 0, time.UTC)

	page := db.StatusPage{
		ID:                        "status-page-feed-unpublished-test",
		Slug:                      "empty-status",
		Title:                     "Empty Status",
		Visibility:                "unlisted",
		ThemeSettings:             "{}",
		DefaultIncidentVisibility: "draft",
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}
	if err := server.db.Create(&page).Error; err != nil {
		t.Fatalf("create status page: %v", err)
	}

	if err := server.db.Create(&db.StatusPageIncident{
		ID:            "published-without-published-at",
		StatusPageID:  page.ID,
		Title:         "No publish timestamp should not appear",
		PublicStatus:  "investigating",
		Severity:      "minor",
		ImpactSummary: "No publish timestamp",
		Visibility:    "published",
		CreatedAt:     now,
		UpdatedAt:     now,
	}).Error; err != nil {
		t.Fatalf("create public incident: %v", err)
	}

	resp := performJSONRequest(t, server, http.MethodGet, "/status/empty-status/feed.atom", nil, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("feed status = %d, body = %s", resp.Code, resp.Body.String())
	}

	var feed testAtomFeed
	if err := xml.Unmarshal(resp.Body.Bytes(), &feed); err != nil {
		t.Fatalf("parse feed XML: %v\n%s", err, resp.Body.String())
	}
	if len(feed.Entries) != 0 {
		t.Fatalf("feed entries = %d, want 0: %s", len(feed.Entries), resp.Body.String())
	}
	assertNotContains(t, resp.Body.String(), "No publish timestamp should not appear")
}

func TestStatusPageAtomFeedDoesNotExposeDraftPages(t *testing.T) {
	server := setupTestServer(t)
	now := time.Date(2026, 5, 27, 12, 0, 0, 0, time.UTC)

	page := db.StatusPage{
		ID:                        "status-page-feed-draft-test",
		Slug:                      "draft-status",
		Title:                     "Draft Status",
		Visibility:                "draft",
		ThemeSettings:             "{}",
		DefaultIncidentVisibility: "draft",
		CreatedAt:                 now,
		UpdatedAt:                 now,
	}
	if err := server.db.Create(&page).Error; err != nil {
		t.Fatalf("create status page: %v", err)
	}

	resp := performJSONRequest(t, server, http.MethodGet, "/status/draft-status/feed.atom", nil, "")
	if resp.Code != http.StatusNotFound {
		t.Fatalf("feed status = %d, body = %s, want 404", resp.Code, resp.Body.String())
	}
}

func assertContains(t *testing.T, body string, value string) {
	t.Helper()

	if !strings.Contains(body, value) {
		t.Fatalf("response missing %q: %s", value, body)
	}
}
