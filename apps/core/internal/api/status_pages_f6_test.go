package api

import (
	"net/http"
	"orion/core/internal/db"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestStatusPageListRowsReadinessSlugAndBadges(t *testing.T) {
	server, page, visible, _ := setupStatusPageBadgeTestData(t)
	server.cfg.PublicStatusURLOrigin = "https://core.example.com"
	server.db.Create(&db.StatusPage{ID: "draft-page", Slug: "internal-tools", Title: "Internal tools", Visibility: statusPageVisibilityDraft, ThemeSettings: "{}", DefaultIncidentVisibility: statusPageIncidentVisibilityDraft})
	server.db.Create(&db.StatusPageSubscriber{ID: "sub-c", StatusPageID: page.ID, DestinationType: "email", DestinationHash: "h1", MaskedDestination: "a***@x.test", State: "confirmed"})
	server.db.Create(&db.StatusPageSubscriber{ID: "sub-p", StatusPageID: page.ID, DestinationType: "email", DestinationHash: "h2", MaskedDestination: "b***@x.test", State: "pending"})

	var list struct {
		Data struct {
			Pages []StatusPageListRow `json:"pages"`
		} `json:"data"`
	}
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/status-pages", nil, ""), &list)
	rows := map[string]StatusPageListRow{}
	for _, r := range list.Data.Pages {
		rows[r.Slug] = r
	}
	pub, draft := rows["customer-status"], rows["internal-tools"]
	if pub.State != "published" || pub.PublicURL != "/status/customer-status" || pub.SubscriberCount != 1 || pub.ComponentCount != 1 || pub.OverallStatus == "" || draft.State != "draft" || draft.ComponentCount != 0 {
		t.Fatalf("rows = %+v / %+v", pub, draft)
	}

	var ready struct {
		Data StatusPageReadinessResponse `json:"data"`
	}
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/status-pages/draft-page/readiness", nil, ""), &ready)
	if ready.Data.Ready || ready.Data.Passed != 0 || ready.Data.Total != 3 || ready.Data.Checks[0].Key != "has_components" || ready.Data.Checks[0].OK {
		t.Fatalf("draft readiness = %+v", ready.Data)
	}
	server.db.Model(&db.StatusPage{}).Where("id = ?", page.ID).Update("custom_domain", "status.example.com")
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/status-pages/"+page.ID+"/readiness", nil, ""), &ready)
	last := ready.Data.Checks[len(ready.Data.Checks)-1]
	if last.Key != "domain_verified" || last.OK || last.Blocking || !strings.Contains(last.Detail, "/status/customer-status") {
		t.Fatalf("domain check = %+v", last)
	}

	var slug struct {
		Data StatusPageSlugCheckResponse `json:"data"`
	}
	for q, want := range map[string]bool{"customer-status": false, "free-slug": true, "Bad Slug!": false, "": false} {
		decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/status-pages/slug-check?slug="+strings.ReplaceAll(q, " ", "%20"), nil, ""), &slug)
		if slug.Data.Available != want {
			t.Errorf("slug %q = %+v, want available=%v", q, slug.Data, want)
		}
	}
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/status-pages/slug-check?slug=customer-status&exclude_id="+page.ID, nil, ""), &slug)
	if !slug.Data.Available {
		t.Errorf("own slug should be available when excluded: %+v", slug.Data)
	}

	var badges struct {
		Data StatusPageBadgesResponse `json:"data"`
	}
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/status-pages/"+page.ID+"/badges", nil, ""), &badges)
	if badges.Data.Page.SVGURL != "https://core.example.com/status/customer-status/badge.svg" || !strings.HasPrefix(badges.Data.Page.Markdown, "![Status](") || len(badges.Data.Components) != 1 || badges.Data.Components[0].ID != visible.ID {
		t.Fatalf("badges = %+v", badges.Data)
	}
	js := performJSONRequest(t, server, http.MethodGet, "/status/customer-status/badge.json", nil, "")
	assertContains(t, js.Body.String(), `"schemaVersion":1`)
	if r := performJSONRequest(t, server, http.MethodGet, "/status/nope/badge.json", nil, ""); r.Code != http.StatusOK || !strings.Contains(r.Body.String(), `"message":"unknown"`) {
		t.Errorf("unknown json badge = %d %s", r.Code, r.Body.String())
	}
}

func TestStatusPageCreateFromMonitorsAndTimezone(t *testing.T) {
	server := setupTestServer(t)
	now := time.Now().UTC()
	server.db.Create(&db.Monitor{ID: "mon-1", Name: "API", Type: "http", AgentID: "core", CreatedAt: now})
	server.db.Create(&db.Monitor{ID: "mon-sys", Name: "Core worker", Type: "http", AgentID: "core", System: true, CreatedAt: now})
	r := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages", gin.H{"title": "Example", "slug": "example", "start_from": "monitors", "theme_settings": gin.H{"timezone": "Europe/London", "accent_color": "#3366ff"}}, "")
	var created struct {
		Data struct {
			Page StatusPageResponse `json:"page"`
		} `json:"data"`
	}
	decodeResponse(t, r, &created)
	if r.Code != http.StatusCreated || created.Data.Page.ThemeSettings["timezone"] != "Europe/London" {
		t.Fatalf("create = %d %s", r.Code, r.Body.String())
	}
	var comps []db.StatusPageComponent
	server.db.Where("status_page_id = ?", created.Data.Page.ID).Find(&comps)
	var maps int64
	server.db.Model(&db.StatusPageComponentMapping{}).Where("resource_id = ?", "mon-1").Count(&maps)
	if len(comps) != 1 || comps[0].PublicName != "API" || maps != 1 {
		t.Fatalf("components = %+v maps=%d", comps, maps)
	}
	for name, body := range map[string]gin.H{
		"bad start_from": {"title": "X", "slug": "x1", "start_from": "everything"},
		"bad timezone":   {"title": "X", "slug": "x2", "theme_settings": gin.H{"timezone": "Mars/Base"}},
	} {
		if r := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages", body, ""); r.Code != http.StatusBadRequest {
			t.Errorf("%s = %d", name, r.Code)
		}
	}
}

func TestStatusPageSubscribersCountsSearchAndExport(t *testing.T) {
	server := setupTestServer(t)
	server.cfg.PublicStatusSubscriberSecret = "test-subscriber-secret"
	page, comp, _ := createPublishedStatusPageForSubscriberTest(t, server, "f6-subs")
	seedStatusPageSubscriberForTest(t, server, page.ID, "ada@acme.test", statusPageSubscriberStateConfirmed, "c1", "m1", "u1", nil)
	seedStatusPageSubscriberForTest(t, server, page.ID, "ops@acme.test", statusPageSubscriberStatePending, "c2", "m2", "u2", []string{comp.ID})
	seedStatusPageSubscriberForTest(t, server, page.ID, "=cmd@evil.test", statusPageSubscriberStateConfirmed, "c3", "m3", "u3", nil)

	var list struct {
		Data struct {
			Subscribers []StatusPageSubscriberAdminResponse `json:"subscribers"`
			Counts      StatusPageSubscriberCounts          `json:"counts"`
		} `json:"data"`
	}
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/status-pages/"+page.ID+"/subscribers", nil, ""), &list)
	if list.Data.Counts.Confirmed != 2 || list.Data.Counts.Total != 3 || list.Data.Counts.Pending != 1 {
		t.Fatalf("counts = %+v", list.Data.Counts)
	}
	byEmail := map[string]StatusPageSubscriberAdminResponse{}
	for _, s := range list.Data.Subscribers {
		byEmail[s.Email] = s
	}
	if byEmail["ada@acme.test"].Scope != "All components" || !strings.HasSuffix(byEmail["ops@acme.test"].Scope, " only") || byEmail["ada@acme.test"].MaskedDestination == "" {
		t.Fatalf("rows = %+v", byEmail)
	}
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/status-pages/"+page.ID+"/subscribers?q=ops&state=pending", nil, ""), &list)
	if len(list.Data.Subscribers) != 1 || list.Data.Subscribers[0].Email != "ops@acme.test" {
		t.Fatalf("filtered = %+v", list.Data.Subscribers)
	}

	csv := performJSONRequest(t, server, http.MethodGet, "/v1/status-pages/"+page.ID+"/subscribers/export.csv", nil, "")
	body := csv.Body.String()
	if csv.Code != http.StatusOK || !strings.HasPrefix(csv.Header().Get("Content-Type"), "text/csv") || !strings.HasPrefix(body, "email,scope,status,since") ||
		!strings.Contains(body, "ada@acme.test,All components,confirmed,") || !strings.Contains(body, "'=cmd@evil.test") || strings.Contains(body, "\n=cmd") {
		t.Fatalf("csv = %d %s", csv.Code, body)
	}
}
