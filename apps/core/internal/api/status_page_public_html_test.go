package api

import (
	"net/http"
	"net/http/httptest"
	"orion/core/internal/db"
	"strings"
	"testing"
	"time"
)

func TestPublicStatusPageHTMLRendersIAWithActiveIncident(t *testing.T) {
	server := setupTestServer(t)
	page, component, hiddenComponent := createPublishedStatusPageForSubscriberTest(t, server, "ia-active-status")
	now := time.Date(2026, 5, 29, 17, 14, 0, 0, time.UTC)
	if err := server.db.Model(&db.StatusPageComponent{}).Where("id = ?", component.ID).Updates(map[string]interface{}{
		"manual_status":        "degraded",
		"manual_status_reason": "Elevated latency for public traffic",
	}).Error; err != nil {
		t.Fatalf("update component: %v", err)
	}
	activeIncident := db.StatusPageIncident{
		ID:                   "status-page-ia-active-incident",
		StatusPageID:         page.ID,
		InternalIncidentID:   "internal-incident-secret",
		Title:                "Checkout latency",
		PublicStatus:         "investigating",
		Severity:             "high",
		ImpactSummary:        "Checkout requests are slower than usual.",
		Visibility:           statusPageIncidentVisibilityPublished,
		AffectedComponentIDs: `["` + component.ID + `","` + hiddenComponent.ID + `"]`,
		PublishedAt:          &now,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	resolvedAt := now.Add(-2 * time.Hour)
	resolvedIncident := db.StatusPageIncident{
		ID:                   "status-page-ia-resolved-incident",
		StatusPageID:         page.ID,
		Title:                "Earlier API issue",
		PublicStatus:         "resolved",
		Severity:             "medium",
		ImpactSummary:        "A resolved public incident.",
		Visibility:           statusPageIncidentVisibilityPublished,
		AffectedComponentIDs: `["` + component.ID + `"]`,
		PublishedAt:          &resolvedAt,
		ResolvedAt:           &now,
		CreatedAt:            resolvedAt,
		UpdatedAt:            now,
	}
	privateIncident := db.StatusPageIncident{
		ID:           "status-page-ia-private-incident",
		StatusPageID: page.ID,
		Title:        "Private operator note",
		PublicStatus: "investigating",
		Severity:     "critical",
		Visibility:   statusPageIncidentVisibilityPrivate,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := server.db.Create(&[]db.StatusPageIncident{activeIncident, resolvedIncident, privateIncident}).Error; err != nil {
		t.Fatalf("create incidents: %v", err)
	}
	updates := []db.StatusPageIncidentUpdate{
		{
			ID:          "status-page-ia-published-update",
			IncidentID:  activeIncident.ID,
			Status:      "identified",
			Message:     "We identified the public database pool saturation.",
			CreatedBy:   "operator@example.com",
			PublishedAt: &now,
			CreatedAt:   now,
		},
		{
			ID:         "status-page-ia-draft-update",
			IncidentID: activeIncident.ID,
			Status:     "monitoring",
			Message:    "Draft update should not render.",
			CreatedBy:  "operator@example.com",
			CreatedAt:  now,
		},
	}
	if err := server.db.Create(&updates).Error; err != nil {
		t.Fatalf("create incident updates: %v", err)
	}

	body := requestPublicStatusPageHTML(t, server, "/status/"+page.Slug)
	assertContains(t, body, "Public status navigation")
	assertContains(t, body, "Get updates")
	assertContains(t, body, "Active incidents and maintenance")
	assertContains(t, body, "Checkout latency")
	assertContains(t, body, "Checkout requests are slower than usual.")
	assertContains(t, body, "We identified the public database pool saturation.")
	assertContains(t, body, "Component health")
	assertContains(t, body, "Visible API")
	assertContains(t, body, "Elevated latency for public traffic")
	assertContains(t, body, "Recent public events")
	assertContains(t, body, "Earlier API issue")
	assertContains(t, body, "Atom feed")
	assertContains(t, body, "Status badge")
	assertNotContains(t, body, "Hidden Database")
	assertNotContains(t, body, "Private operator note")
	assertNotContains(t, body, "internal-incident-secret")
	assertNotContains(t, body, "operator@example.com")
	assertNotContains(t, body, "Draft update should not render.")

	assertHTMLOrder(t, body, "Public status navigation", "Active incidents and maintenance", "Component health", "Recent public events", "Updates and utilities")
}

func TestPublicStatusPageHTMLRendersIANoRecentIncidents(t *testing.T) {
	server := setupTestServer(t)
	page, component, hiddenComponent := createPublishedStatusPageForSubscriberTest(t, server, "ia-empty-status")

	body := requestPublicStatusPageHTML(t, server, "/status/"+page.Slug)
	assertContains(t, body, "Subscriber Test Status")
	assertContains(t, body, "Get updates")
	assertContains(t, body, "Component health")
	assertContains(t, body, "Visible API")
	assertContains(t, body, "No recent incidents.")
	assertContains(t, body, "/status/"+page.Slug+"/history")
	assertContains(t, body, "/status/"+page.Slug+"/feed.atom")
	assertContains(t, body, "/status/"+page.Slug+"/badge.svg")
	assertContains(t, body, "no-data")
	assertNotContains(t, body, "Active incidents and maintenance")
	assertNotContains(t, body, hiddenComponent.PublicName)
	assertNotContains(t, body, component.ID)
}

func requestPublicStatusPageHTML(t *testing.T, server *Server, path string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Accept", "text/html")
	resp := httptest.NewRecorder()
	server.router.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("HTML status = %d, body = %s", resp.Code, resp.Body.String())
	}
	assertContains(t, resp.Header().Get("Content-Type"), "text/html")
	return resp.Body.String()
}

func assertHTMLOrder(t *testing.T, body string, values ...string) {
	t.Helper()
	last := -1
	for _, value := range values {
		index := strings.Index(body, value)
		if index < 0 {
			t.Fatalf("body missing %q", value)
		}
		if index <= last {
			t.Fatalf("%q appears out of order in HTML body", value)
		}
		last = index
	}
}
