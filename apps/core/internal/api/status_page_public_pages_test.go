package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"strings"
	"testing"
	"time"
)

func htmlGet(server *Server, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Accept", "text/html")
	resp := httptest.NewRecorder()
	server.router.ServeHTTP(resp, req)
	return resp
}

func htmlPost(server *Server, path string, form url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
	req.Header.Set("Accept", "text/html")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp := httptest.NewRecorder()
	server.router.ServeHTTP(resp, req)
	return resp
}

func setComponentStatus(t *testing.T, server *Server, id string, status string) {
	t.Helper()
	if err := server.db.Model(&db.StatusPageComponent{}).Where("id = ?", id).Update("manual_status", status).Error; err != nil {
		t.Fatalf("set component status: %v", err)
	}
}

func TestPublicStatusPageHTMLShowsEachOverallState(t *testing.T) {
	server := setupTestServer(t)
	page, component, _ := createPublishedStatusPageForSubscriberTest(t, server, "state-status")
	for status, want := range map[string]string{"operational": "All systems operational", "degraded": "Some systems degraded", "major_outage": "Major outage", "maintenance": "Maintenance in progress"} {
		setComponentStatus(t, server, component.ID, status)
		resp := htmlGet(server, "/status/"+page.Slug)
		if resp.Code != http.StatusOK {
			t.Fatalf("%s: status = %d", status, resp.Code)
		}
		body := resp.Body.String()
		assertContains(t, body, want)
		assertContains(t, body, "Visible API")
		assertContains(t, body, `class="tkr"`)
		assertContains(t, body, "Powered by Orion")
		assertContains(t, body, `href="/status/state-status/subscribe"`)
		assertNotContains(t, body, "Hidden Database")
		assertNotContains(t, body, "<script")
	}
	setComponentStatus(t, server, component.ID, "degraded")
	body := htmlGet(server, "/status/"+page.Slug).Body.String()
	assertContains(t, body, `Affected</span><span class="v">1 <small>of 1</small>`)
}

func TestPublicStatusPageHTMLListsActiveIncidentsAndMaintenance(t *testing.T) {
	server := setupTestServer(t)
	page, component, _ := createPublishedStatusPageForSubscriberTest(t, server, "active-status")
	now := time.Now().UTC()
	incident := db.StatusPageIncident{ID: "pub-inc-1", StatusPageID: page.ID, Title: "Checkout failing", PublicStatus: "investigating", Severity: "major", Visibility: statusPageIncidentVisibilityPublished, AffectedComponentIDs: `["` + component.ID + `"]`, PublishedAt: &now}
	draft := db.StatusPageIncident{ID: "pub-inc-draft", StatusPageID: page.ID, Title: "Secret draft", PublicStatus: "investigating", Severity: "major", Visibility: statusPageIncidentVisibilityDraft}
	for _, row := range []*db.StatusPageIncident{&incident, &draft} {
		if err := server.db.Create(row).Error; err != nil {
			t.Fatalf("create incident: %v", err)
		}
	}
	msg := now.Add(-5 * time.Minute)
	if err := server.db.Create(&db.StatusPageIncidentUpdate{ID: "pub-upd-1", IncidentID: incident.ID, Status: "investigating", Message: "We are looking into elevated errors.", PublishedAt: &msg}).Error; err != nil {
		t.Fatalf("create update: %v", err)
	}
	window := db.StatusPageMaintenance{ID: "pub-mnt-1", StatusPageID: page.ID, Title: "Database upgrade", ComponentIDs: `["` + component.ID + `"]`, StartsAt: now.Add(48 * time.Hour), EndsAt: now.Add(50 * time.Hour)}
	if err := server.db.Create(&window).Error; err != nil {
		t.Fatalf("create maintenance: %v", err)
	}
	body := htmlGet(server, "/status/"+page.Slug).Body.String()
	assertContains(t, body, "Active incidents")
	assertContains(t, body, "Checkout failing")
	assertContains(t, body, "We are looking into elevated errors.")
	assertContains(t, body, "Investigating")
	assertContains(t, body, `Open incidents</span><span class="v">1`)
	assertContains(t, body, "Scheduled maintenance")
	assertContains(t, body, "Database upgrade")
	assertNotContains(t, body, "Secret draft")
}

func TestPublicStatusPageHistoryHTMLGroupsByMonthAndHidesDrafts(t *testing.T) {
	server := setupTestServer(t)
	page, _, _ := createPublishedStatusPageForSubscriberTest(t, server, "history-status")
	now := time.Now().UTC()
	old := now.AddDate(0, -5, 0)
	rows := []db.StatusPageIncident{
		{ID: "h1", StatusPageID: page.ID, Title: "Recent outage", PublicStatus: "resolved", Severity: "major", Visibility: statusPageIncidentVisibilityPublished, PublishedAt: &now, ResolvedAt: ptrTime(now.Add(2 * time.Hour))},
		{ID: "h2", StatusPageID: page.ID, Title: "Ancient outage", PublicStatus: "resolved", Severity: "major", Visibility: statusPageIncidentVisibilityPublished, PublishedAt: &old, ResolvedAt: ptrTime(old.Add(time.Hour))},
		{ID: "h3", StatusPageID: page.ID, Title: "Hidden draft", PublicStatus: "resolved", Severity: "major", Visibility: statusPageIncidentVisibilityDraft, PublishedAt: &now},
	}
	for i := range rows {
		if err := server.db.Create(&rows[i]).Error; err != nil {
			t.Fatalf("create incident: %v", err)
		}
	}
	past := now.Add(-72 * time.Hour)
	if err := server.db.Create(&db.StatusPageMaintenance{ID: "hm1", StatusPageID: page.ID, Title: "Planned patching", ComponentIDs: "[]", StartsAt: past, EndsAt: past.Add(time.Hour)}).Error; err != nil {
		t.Fatalf("create maintenance: %v", err)
	}
	resp := htmlGet(server, "/status/"+page.Slug+"/history")
	if resp.Code != http.StatusOK {
		t.Fatalf("history status = %d, body = %s", resp.Code, resp.Body.String())
	}
	body := resp.Body.String()
	assertContains(t, body, "Incident history")
	assertContains(t, body, "Recent outage")
	assertContains(t, body, "2h")
	assertContains(t, body, "Resolved")
	assertContains(t, body, "Planned patching")
	assertContains(t, body, now.Format("January 2006"))
	assertNotContains(t, body, "Ancient outage")
	assertNotContains(t, body, "Hidden draft")
	assertContains(t, body, `href="/status/history-status/history?before=`+now.AddDate(0, -3, 0).Format("2006-01"))
	older := htmlGet(server, "/status/"+page.Slug+"/history?before="+now.AddDate(0, -3, 0).Format("2006-01")).Body.String()
	assertContains(t, older, "Ancient outage")
	assertNotContains(t, older, "Recent outage")
	jsonResp := performJSONRequest(t, server, http.MethodGet, "/status/"+page.Slug+"/history", nil, "")
	assertContains(t, jsonResp.Header().Get("Content-Type"), "application/json")
}

func TestPublicStatusPageIncidentHTMLShowsUpdatesNewestFirst(t *testing.T) {
	server := setupTestServer(t)
	page, _, _ := createPublishedStatusPageForSubscriberTest(t, server, "incident-status")
	now := time.Now().UTC().Truncate(time.Minute)
	incident := db.StatusPageIncident{ID: "pi1", StatusPageID: page.ID, Title: "API latency", PublicStatus: "monitoring", Severity: "minor", ImpactSummary: "Requests are slow.", Visibility: statusPageIncidentVisibilityPublished, PublishedAt: &now}
	private := db.StatusPageIncident{ID: "pi2", StatusPageID: page.ID, Title: "Private thing", PublicStatus: "monitoring", Severity: "minor", Visibility: statusPageIncidentVisibilityPrivate}
	for _, row := range []*db.StatusPageIncident{&incident, &private} {
		if err := server.db.Create(row).Error; err != nil {
			t.Fatalf("create incident: %v", err)
		}
	}
	first, second := now.Add(-time.Hour), now
	for _, u := range []db.StatusPageIncidentUpdate{{ID: "u1", IncidentID: incident.ID, Status: "investigating", Message: "First look.", PublishedAt: &first}, {ID: "u2", IncidentID: incident.ID, Status: "monitoring", Message: "Fix deployed.", PublishedAt: &second}} {
		if err := server.db.Create(&u).Error; err != nil {
			t.Fatalf("create update: %v", err)
		}
	}
	resp := htmlGet(server, "/status/"+page.Slug+"/incidents/pi1")
	if resp.Code != http.StatusOK {
		t.Fatalf("incident status = %d", resp.Code)
	}
	body := resp.Body.String()
	assertContains(t, body, "API latency")
	assertContains(t, body, "Requests are slow.")
	if strings.Index(body, "Fix deployed.") > strings.Index(body, "First look.") {
		t.Fatalf("updates are not newest first")
	}
	assertContains(t, body, "Get updates for this incident by email")
	if resp := htmlGet(server, "/status/"+page.Slug+"/incidents/pi2"); resp.Code != http.StatusNotFound {
		t.Fatalf("private incident status = %d, want 404", resp.Code)
	}
}

func TestPublicStatusPageRSSFeedListsPublishedIncidents(t *testing.T) {
	server := setupTestServer(t)
	page, _, _ := createPublishedStatusPageForSubscriberTest(t, server, "rss-status")
	now := time.Now().UTC()
	rows := []db.StatusPageIncident{{ID: "r1", StatusPageID: page.ID, Title: "Feed <me>", PublicStatus: "resolved", Severity: "minor", Visibility: statusPageIncidentVisibilityPublished, PublishedAt: &now}, {ID: "r2", StatusPageID: page.ID, Title: "Unpublished", PublicStatus: "resolved", Severity: "minor", Visibility: statusPageIncidentVisibilityDraft}}
	for i := range rows {
		if err := server.db.Create(&rows[i]).Error; err != nil {
			t.Fatalf("create incident: %v", err)
		}
	}
	resp := performJSONRequest(t, server, http.MethodGet, "/status/"+page.Slug+"/feed.rss", nil, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("rss status = %d", resp.Code)
	}
	assertContains(t, resp.Header().Get("Content-Type"), "application/rss+xml")
	assertContains(t, resp.Body.String(), `<rss version="2.0">`)
	assertContains(t, resp.Body.String(), "Feed &lt;me&gt;")
	assertNotContains(t, resp.Body.String(), "Unpublished")
}

func TestPublicSubscribeFormSendsConfirmationAndRejectsBadInput(t *testing.T) {
	server := setupTestServer(t)
	configurePublicStatusMailForTest(server)
	sent := 0
	server.publicStatusMailSend = func(publicStatusMailMessage) error { sent++; return nil }
	page, component, _ := createPublishedStatusPageForSubscriberTest(t, server, "form-status")
	form := htmlGet(server, "/status/"+page.Slug+"/subscribe")
	assertContains(t, form.Body.String(), "Get status updates")
	assertContains(t, form.Body.String(), `name="email"`)
	assertContains(t, form.Body.String(), `value="`+component.ID+`"`)
	assertContains(t, form.Header().Get("Cache-Control"), "no-store")
	bad := htmlPost(server, "/status/"+page.Slug+"/subscribe", url.Values{"email": {"nope"}})
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("bad email status = %d", bad.Code)
	}
	assertContains(t, bad.Body.String(), `class="flash err"`)
	ok := htmlPost(server, "/status/"+page.Slug+"/subscribe", url.Values{"email": {"reader@example.com"}, "scope": {"selected"}, "component_ids": {component.ID}})
	if ok.Code != http.StatusOK {
		t.Fatalf("subscribe status = %d, body = %s", ok.Code, ok.Body.String())
	}
	assertContains(t, ok.Body.String(), "Check your inbox. We sent a link to confirm your email.")
	assertNotContains(t, ok.Body.String(), "reader@example.com")
	if sent != 1 {
		t.Fatalf("confirmation mails = %d, want 1", sent)
	}
	var prefs int64
	server.db.Model(&db.StatusPageSubscriberComponent{}).Where("component_id = ?", component.ID).Count(&prefs)
	if prefs != 1 {
		t.Fatalf("component preferences = %d, want 1", prefs)
	}
}

func TestPublicSubscriberConfirmManageUnsubscribePages(t *testing.T) {
	server := setupTestServer(t)
	page, visible, _ := createPublishedStatusPageForSubscriberTest(t, server, "token-status")
	other := db.StatusPageComponent{ID: utils.GenerateID("status_page_component"), StatusPageID: page.ID, SectionID: visible.SectionID, PublicName: "Second API", DisplayMode: "manual", ManualStatus: "operational", Visible: true}
	if err := server.db.Create(&other).Error; err != nil {
		t.Fatalf("create component: %v", err)
	}
	sub := seedStatusPageSubscriberForTest(t, server, page.ID, "Pat.Reader@example.com", statusPageSubscriberStatePending, "c-token", "m-token", "u-token", []string{})
	base := "/status/" + page.Slug + "/subscribers/"
	confirmed := htmlGet(server, base+"confirm/c-token")
	if confirmed.Code != http.StatusOK {
		t.Fatalf("confirm status = %d, body = %s", confirmed.Code, confirmed.Body.String())
	}
	assertContains(t, confirmed.Body.String(), "You are subscribed")
	assertContains(t, confirmed.Body.String(), "p***@example.com")
	assertContains(t, confirmed.Header().Get("Referrer-Policy"), "no-referrer")
	assertContains(t, confirmed.Header().Get("X-Robots-Tag"), "noindex")
	assertContains(t, confirmed.Body.String(), `<meta name="robots" content="noindex">`)
	assertNotContains(t, confirmed.Body.String(), "c-token")
	reused := htmlGet(server, base+"confirm/c-token")
	if reused.Code != http.StatusNotFound {
		t.Fatalf("reused confirm status = %d", reused.Code)
	}
	assertContains(t, reused.Body.String(), "This link has expired")
	manage := htmlGet(server, base+"manage/m-token")
	assertContains(t, manage.Body.String(), "Save preferences")
	assertContains(t, manage.Body.String(), "Visible API")
	assertContains(t, manage.Body.String(), "Second API")
	saved := htmlPost(server, base+"manage/m-token", url.Values{"component_ids": {visible.ID}})
	if saved.Code != http.StatusOK {
		t.Fatalf("save status = %d, body = %s", saved.Code, saved.Body.String())
	}
	assertContains(t, saved.Body.String(), "Preferences saved.")
	assertContains(t, saved.Body.String(), "1 of 2 components")
	var stored db.StatusPageSubscriber
	if err := server.db.First(&stored, "id = ?", sub.ID).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if stored.ManageTokenHash != hashStatusPageSubscriberToken("m-token") || stored.UnsubscribeTokenHash != hashStatusPageSubscriberToken("u-token") {
		t.Fatalf("the form must not rotate the links in the subscriber's inbox")
	}
	if none := htmlPost(server, base+"manage/m-token", url.Values{}); !strings.Contains(none.Body.String(), "Choose at least one component") {
		t.Fatalf("an empty selection must not silently mean everything")
	}
	if all := htmlPost(server, base+"manage/m-token", url.Values{"component_ids": {visible.ID, other.ID}}); !strings.Contains(all.Body.String(), "All components") {
		t.Fatalf("every component selected should read as all components")
	}
	if invalid := htmlGet(server, base+"manage/nope"); invalid.Code != http.StatusNotFound {
		t.Fatalf("invalid manage token status = %d", invalid.Code)
	}
	ask := htmlGet(server, base+"unsubscribe/u-token")
	assertContains(t, ask.Body.String(), "Leave this list?")
	if err := server.db.First(&stored, "id = ?", sub.ID).Error; err != nil || stored.State != statusPageSubscriberStateConfirmed {
		t.Fatalf("GET unsubscribe changed state to %q", stored.State)
	}
	done := htmlPost(server, base+"unsubscribe/u-token", url.Values{})
	assertContains(t, done.Body.String(), "You are unsubscribed")
	if err := server.db.First(&stored, "id = ?", sub.ID).Error; err != nil || stored.State != statusPageSubscriberStateUnsubscribed {
		t.Fatalf("subscriber state = %q, want unsubscribed", stored.State)
	}
	again := htmlGet(server, base+"unsubscribe/u-token")
	assertContains(t, again.Body.String(), "You are unsubscribed")
	bad := htmlGet(server, base+"unsubscribe/not-a-token")
	if bad.Code != http.StatusNotFound {
		t.Fatalf("invalid unsubscribe status = %d", bad.Code)
	}
	assertContains(t, bad.Body.String(), "This link no longer works")
}

func TestPublicPagesRespectThemeAndHiddenHistory(t *testing.T) {
	server := setupTestServer(t)
	page, _, _ := createPublishedStatusPageForSubscriberTest(t, server, "theme-status")
	if err := server.db.Model(&db.StatusPage{}).Where("id = ?", page.ID).Update("theme_settings", `{"theme_mode":"light","accent_color":"#10b981","show_incident_history":false,"show_uptime_summary":false}`).Error; err != nil {
		t.Fatalf("set theme: %v", err)
	}
	body := htmlGet(server, "/status/"+page.Slug).Body.String()
	assertContains(t, body, `<body class="theme-light">`)
	assertContains(t, body, `<link rel="icon" href="data:image/svg+xml,%3Csvg`)
	assertContains(t, body, "--accent:#10b981")
	assertNotContains(t, body, "Incident history")
	assertNotContains(t, body, `class="stats"`)
	if resp := htmlGet(server, "/status/"+page.Slug+"/history"); resp.Code != http.StatusNotFound {
		t.Fatalf("hidden history status = %d, want 404", resp.Code)
	}
}

func TestAlertEmailConfirmPageIsStyled(t *testing.T) {
	server := setupTestServer(t)
	resp := htmlGet(server, "/alerts/email/confirm/not-a-token")
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", resp.Code)
	}
	assertContains(t, resp.Body.String(), "Link expired")
	assertContains(t, resp.Body.String(), `class="plain-page`)
	assertContains(t, resp.Body.String(), "Powered by Orion")
}

func TestPublicBarTipNamesDayStatusAndUptime(t *testing.T) {
	ratio := 0.9
	if got := publicBarTip(StatusPagePublicUptimeBucketResponse{Date: "2026-10-03", Status: "degraded", UptimeRatio: &ratio, UptimeDisplay: "90.0%"}); got != "Oct 3 · Degraded · 90.0%" {
		t.Fatalf("tip = %q", got)
	}
	if got := publicBarTip(StatusPagePublicUptimeBucketResponse{Date: "2026-10-04", Status: "no_data"}); !strings.HasPrefix(got, "Oct 4 · ") {
		t.Fatalf("no-data tip = %q", got)
	}
}

func TestPublicPagesRenderCompactFromThemeOrQuery(t *testing.T) {
	server := setupTestServer(t)
	page, _, _ := createPublishedStatusPageForSubscriberTest(t, server, "compact-status")
	standard := htmlGet(server, "/status/"+page.Slug).Body.String()
	assertContains(t, standard, `class="stats"`)
	assertNotContains(t, standard, `hero slim`)
	forced := htmlGet(server, "/status/"+page.Slug+"?compact=1").Body.String()
	assertContains(t, forced, `hero slim`)
	assertContains(t, forced, ` compact">`)
	assertNotContains(t, forced, `class="stats"`)
	assertNotContains(t, forced, `class="tkr"`)
	if err := server.db.Model(&db.StatusPage{}).Where("id = ?", page.ID).Update("theme_settings", `{"density":"compact"}`).Error; err != nil {
		t.Fatalf("set density: %v", err)
	}
	assertContains(t, htmlGet(server, "/status/"+page.Slug).Body.String(), `hero slim`)
	assertContains(t, htmlGet(server, "/status/"+page.Slug+"/history").Body.String(), `hero slim`)
	assertNotContains(t, htmlGet(server, "/status/"+page.Slug+"?compact=0").Body.String(), `hero slim`)
}

func TestThemeSettingsAcceptDensityStandardAndCompactOnly(t *testing.T) {
	for _, ok := range []string{"standard", "compact"} {
		if got, err := sanitizeStatusPageThemeSettings(map[string]interface{}{"density": ok}); err != nil || got["density"] != ok {
			t.Fatalf("density %q: %v %v", ok, got, err)
		}
	}
	if _, err := sanitizeStatusPageThemeSettings(map[string]interface{}{"density": "tiny"}); err == nil {
		t.Fatalf("unknown density must be rejected")
	}
}
