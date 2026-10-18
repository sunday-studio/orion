package api

import (
	"context"
	"net"
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestStatusPageDomainVerification(t *testing.T) {
	server := setupTestServer(t)
	server.cfg.StatusPagesCNAMETarget = "pages.orion.example.com"
	page, _, _ := createPublishedStatusPageForSubscriberTest(t, server, "dom-status")
	answers := []struct {
		cname string
		err   error
	}{{"", &net.DNSError{Err: "no such host", IsNotFound: true}}, {"somewhere.else.net.", nil}, {"Pages.Orion.Example.com.", nil}}
	calls := 0
	server.lookupCNAME = func(ctx context.Context, host string) (string, error) {
		a := answers[calls]
		calls++
		return a.cname, a.err
	}
	if r := performJSONRequest(t, server, http.MethodPut, "/v1/status-pages/"+page.ID, gin.H{"custom_domain": "status.acme.test"}, ""); r.Code != http.StatusOK {
		t.Fatalf("set domain = %d %s", r.Code, r.Body.String())
	}
	get := func() StatusPageDomainResponse {
		var env struct {
			Data StatusPageDomainResponse `json:"data"`
		}
		decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/status-pages/"+page.ID+"/domain", nil, ""), &env)
		return env.Data
	}
	d := get()
	if d.Status != "pending" || d.Certificate != "pending" || len(d.Records) != 1 || d.Records[0].Type != "CNAME" || d.Records[0].Host != "status.acme.test" || d.Records[0].Value != "pages.orion.example.com" {
		t.Fatalf("initial = %+v", d)
	}
	// Unverified domain does not route yet.
	if r := performHostRequest(t, server, http.MethodGet, "/", "status.acme.test"); r.Code == http.StatusOK && strings.Contains(r.Body.String(), "Subscriber Test Status") {
		t.Fatalf("unverified domain served the page")
	}
	verify := func() StatusPageDomainResponse {
		var env struct {
			Data StatusPageDomainResponse `json:"data"`
		}
		r := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+page.ID+"/domain/verify", nil, "")
		decodeResponse(t, r, &env)
		if r.Code != http.StatusOK {
			t.Fatalf("verify = %d %s", r.Code, r.Body.String())
		}
		return env.Data
	}
	d = verify()
	if d.Status != "pending" || d.StatusText != "Record not found yet" || d.LastCheckedAt == nil {
		t.Fatalf("not found = %+v", d)
	}
	// Within the cooldown the last result is returned without another lookup.
	if again := verify(); calls != 1 || again.Status != "pending" {
		t.Fatalf("cooldown calls=%d %+v", calls, again)
	}
	server.db.Model(&db.StatusPage{}).Where("id = ?", page.ID).Update("domain_checked_at", time.Now().Add(-time.Minute))
	if d = verify(); d.Status != "failed" || d.StatusText != "Record points somewhere else" {
		t.Fatalf("wrong target = %+v", d)
	}
	server.db.Model(&db.StatusPage{}).Where("id = ?", page.ID).Update("domain_checked_at", time.Now().Add(-time.Minute))
	if d = verify(); d.Status != "verified" || d.Certificate != "issued" || d.VerifiedAt == nil {
		t.Fatalf("verified = %+v", d)
	}
	if r := performHostRequest(t, server, http.MethodGet, "/", "status.acme.test"); r.Code != http.StatusOK {
		t.Fatalf("verified domain status = %d", r.Code)
	}
	var ready struct {
		Data StatusPageReadinessResponse `json:"data"`
	}
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/status-pages/"+page.ID+"/readiness", nil, ""), &ready)
	if last := ready.Data.Checks[len(ready.Data.Checks)-1]; last.Key != "domain_verified" || !last.OK {
		t.Fatalf("readiness domain check = %+v", last)
	}
	// Changing the domain resets verification; removing clears it.
	performJSONRequest(t, server, http.MethodPut, "/v1/status-pages/"+page.ID, gin.H{"custom_domain": "other.acme.test"}, "")
	if d = get(); d.Status != "pending" || d.VerifiedAt != nil {
		t.Fatalf("after change = %+v", d)
	}
	if r := performJSONRequest(t, server, http.MethodDelete, "/v1/status-pages/"+page.ID+"/domain", nil, ""); r.Code != http.StatusOK {
		t.Fatalf("remove = %d", r.Code)
	}
	if r := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+page.ID+"/domain/verify", nil, ""); r.Code != http.StatusBadRequest {
		t.Errorf("verify without domain = %d", r.Code)
	}
}

func TestStatusPageMaintenanceWindows(t *testing.T) {
	server := setupTestServer(t)
	page, visible, hidden := createPublishedStatusPageForSubscriberTest(t, server, "maint-status")
	now := time.Now().UTC()
	body := func(start, end time.Time, ids ...string) gin.H {
		return gin.H{"title": "Planned database upgrade", "description": "Short read-only period", "starts_at": start, "ends_at": end, "component_ids": ids, "hold_alerts": true, "notify_subscribers": true}
	}
	for name, b := range map[string]gin.H{
		"no components": body(now.Add(time.Hour), now.Add(2*time.Hour)),
		"end before":    body(now.Add(2*time.Hour), now.Add(time.Hour), visible.ID),
		"foreign comp":  body(now.Add(time.Hour), now.Add(2*time.Hour), "nope"),
		"too long":      body(now.Add(time.Hour), now.Add(40*24*time.Hour), visible.ID),
		"already ended": body(now.Add(-2*time.Hour), now.Add(-time.Hour), visible.ID),
		"missing title": {"starts_at": now.Add(time.Hour), "ends_at": now.Add(2 * time.Hour), "component_ids": []string{visible.ID}},
	} {
		if r := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+page.ID+"/maintenance", b, ""); r.Code != http.StatusBadRequest {
			t.Errorf("%s = %d %s", name, r.Code, r.Body.String())
		}
	}
	var created struct {
		Data struct {
			Window StatusPageMaintenanceResponse `json:"window"`
		} `json:"data"`
	}
	r := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+page.ID+"/maintenance", body(now.Add(10*time.Hour), now.Add(12*time.Hour), visible.ID, hidden.ID), "")
	decodeResponse(t, r, &created)
	w := created.Data.Window
	if r.Code != http.StatusCreated || w.Status != "scheduled" || len(w.ComponentNames) != 2 || !w.HoldAlerts || w.NoticeSent {
		t.Fatalf("created = %d %+v", r.Code, w)
	}

	// Public page lists it (visible components only) without changing status yet.
	detail, _ := server.loadStatusPageDetail(page.ID)
	windows, active := server.publicMaintenance(detail, now)
	if len(windows) != 1 || len(windows[0].ComponentIDs) != 1 || windows[0].ComponentIDs[0] != visible.ID || len(active) != 0 {
		t.Fatalf("public windows = %+v active=%v", windows, active)
	}
	// Move it to now: the component shows maintenance and the page says so.
	var upd struct {
		Data struct {
			Window StatusPageMaintenanceResponse `json:"window"`
		} `json:"data"`
	}
	r = performJSONRequest(t, server, http.MethodPatch, "/v1/status-pages/"+page.ID+"/maintenance/"+w.ID, gin.H{"starts_at": now.Add(-time.Minute), "ends_at": now.Add(time.Hour)}, "")
	decodeResponse(t, r, &upd)
	if r.Code != http.StatusOK || upd.Data.Window.Status != "in_progress" {
		t.Fatalf("update = %d %+v", r.Code, upd.Data.Window)
	}
	preview := server.statusPagePreview(detail, false)
	if preview.OverallStatus != "maintenance" || len(preview.Maintenance) != 1 || preview.Maintenance[0].Status != "in_progress" {
		t.Fatalf("preview = %s %+v", preview.OverallStatus, preview.Maintenance)
	}
	html := performRequestWithHeaders(t, server, http.MethodGet, "/status/maint-status", map[string]string{"Accept": "text/html"})
	assertContains(t, html.Body.String(), "Scheduled maintenance")
	assertContains(t, html.Body.String(), "Planned database upgrade")
	assertNotContains(t, html.Body.String(), "Hidden Database")

	var list struct {
		Data struct {
			Maintenance []StatusPageMaintenanceResponse `json:"maintenance"`
		} `json:"data"`
	}
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/status-pages/"+page.ID+"/maintenance", nil, ""), &list)
	if len(list.Data.Maintenance) != 1 {
		t.Fatalf("list = %+v", list.Data)
	}
	if r := performJSONRequest(t, server, http.MethodDelete, "/v1/status-pages/"+page.ID+"/maintenance/"+w.ID, nil, ""); r.Code != http.StatusOK {
		t.Errorf("delete = %d", r.Code)
	}
	if r := performJSONRequest(t, server, http.MethodDelete, "/v1/status-pages/"+page.ID+"/maintenance/"+w.ID, nil, ""); r.Code != http.StatusNotFound {
		t.Errorf("delete again = %d", r.Code)
	}
}

func TestStatusPageMaintenanceHoldsAlertsForMappedResources(t *testing.T) {
	server := setupTestServer(t)
	page, visible, _ := createPublishedStatusPageForSubscriberTest(t, server, "hold-status")
	now := time.Now().UTC()
	server.db.Create(&db.StatusPageComponentMapping{ID: "map1", ComponentID: visible.ID, ResourceType: "monitor", ResourceID: "mon-held", HealthRollupStrategy: "worst", UptimeRollupStrategy: "worst"})
	server.db.Create(&db.StatusPageComponentMapping{ID: "map2", ComponentID: visible.ID, ResourceType: "agent", ResourceID: "agent-held", HealthRollupStrategy: "worst", UptimeRollupStrategy: "worst"})
	win := db.StatusPageMaintenance{ID: "w1", StatusPageID: page.ID, Title: "Upgrade", ComponentIDs: `["` + visible.ID + `"]`, HoldAlerts: true, StartsAt: now.Add(-time.Minute), EndsAt: now.Add(time.Hour)}
	server.db.Create(&win)
	if !service.StatusPageMaintenanceHolds(server.db, "mon-held", "x", now) || !service.StatusPageMaintenanceHolds(server.db, "x", "agent-held", now) {
		t.Fatal("mapped resources should be held")
	}
	if service.StatusPageMaintenanceHolds(server.db, "other", "other", now) || service.StatusPageMaintenanceHolds(server.db, "mon-held", "x", now.Add(2*time.Hour)) {
		t.Fatal("unmapped resources and times after the window must not be held")
	}
	server.db.Model(&db.StatusPageMaintenance{}).Where("id = ?", "w1").Update("hold_alerts", false)
	if service.StatusPageMaintenanceHolds(server.db, "mon-held", "x", now) {
		t.Fatal("hold_alerts=false must not hold")
	}
}

func TestStatusPageMaintenanceNoticeAndSubscriberImport(t *testing.T) {
	server := setupTestServer(t)
	configurePublicStatusMailForTest(server)
	page, visible, _ := createPublishedStatusPageForSubscriberTest(t, server, "notice-status")
	var mails []publicStatusMailMessage
	server.publicStatusMailSend = func(m publicStatusMailMessage) error { mails = append(mails, m); return nil }
	seedStatusPageSubscriberForTest(t, server, page.ID, "ada@acme.test", statusPageSubscriberStateConfirmed, "c1", "m1", "u1", nil)
	seedStatusPageSubscriberForTest(t, server, page.ID, "pending@acme.test", statusPageSubscriberStatePending, "c2", "m2", "u2", nil)
	now := time.Now().UTC()
	server.db.Create(&db.StatusPageMaintenance{ID: "soon", StatusPageID: page.ID, Title: "Upgrade soon", ComponentIDs: `["` + visible.ID + `"]`, NotifySubscribers: true, StartsAt: now.Add(3 * time.Hour), EndsAt: now.Add(4 * time.Hour)})
	server.db.Create(&db.StatusPageMaintenance{ID: "later", StatusPageID: page.ID, Title: "Too far", ComponentIDs: `["` + visible.ID + `"]`, NotifySubscribers: true, StartsAt: now.Add(72 * time.Hour), EndsAt: now.Add(73 * time.Hour)})
	server.db.Create(&db.StatusPageMaintenance{ID: "quiet", StatusPageID: page.ID, Title: "No notice", ComponentIDs: `["` + visible.ID + `"]`, StartsAt: now.Add(2 * time.Hour), EndsAt: now.Add(3 * time.Hour)})
	if n := server.SendDueMaintenanceNotices(now); n != 1 || len(mails) != 1 || mails[0].To != "ada@acme.test" || !strings.Contains(mails[0].Subject, "Upgrade soon") {
		t.Fatalf("notices sent=%d mails=%+v", n, mails)
	}
	if n := server.SendDueMaintenanceNotices(now); n != 0 {
		t.Fatalf("second pass resent %d", n)
	}

	mails = nil
	var imp struct {
		Data StatusPageSubscriberImportResponse `json:"data"`
	}
	csv := "email\nnew1@acme.test\nada@acme.test\nnot-an-email\n=evil@acme.test\n"
	r := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+page.ID+"/subscribers/import", gin.H{"csv": csv}, "")
	decodeResponse(t, r, &imp)
	if r.Code != http.StatusOK || imp.Data.Imported != 2 || imp.Data.Skipped != 1 || imp.Data.InvalidCount != 1 || imp.Data.ConfirmMailed != 2 || len(mails) != 2 || !strings.Contains(mails[0].Text, "Confirm") {
		t.Fatalf("import = %d %+v mails=%d", r.Code, imp.Data, len(mails))
	}
	var sub db.StatusPageSubscriber
	server.db.Where("status_page_id = ? AND source = ?", page.ID, "import").First(&sub)
	if sub.State != statusPageSubscriberStatePending {
		t.Fatalf("imported state = %q, want pending", sub.State)
	}
	tooMany := "email\n" + strings.Repeat("x@acme.test\n", maxSubscriberImportRows+1)
	if r := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+page.ID+"/subscribers/import", gin.H{"csv": tooMany}, ""); r.Code != http.StatusBadRequest {
		t.Errorf("oversized import = %d", r.Code)
	}
	server.db.Model(&db.StatusPage{}).Where("id = ?", page.ID).Update("visibility", "draft")
	if r := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+page.ID+"/subscribers/import", gin.H{"csv": "a@b.test"}, ""); r.Code != http.StatusBadRequest {
		t.Errorf("draft import = %d", r.Code)
	}
}
