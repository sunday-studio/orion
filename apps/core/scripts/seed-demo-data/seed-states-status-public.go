package main

import (
	"fmt"
	"time"

	"orion/core/internal/db"

	"gorm.io/gorm"
)

// publicPageSpec is one small public status page whose overall state is fixed by its manual components,
// with an incident history spread over about three months. They are worker-proof: nothing here is
// computed from monitors except the uptime bars, which come from a healthy seeded monitor.
type publicPageSpec struct {
	key, title, description string
	components              []publicComponentSpec
	open                    []publicIncidentSpec // incidents still open today
	pastDays                []int                // resolved incidents this many days ago
	maintenance             string               // "", "active" or "upcoming"
}

type publicComponentSpec struct{ name, status, reason string }

func publicPageSpecs() []publicPageSpec {
	return []publicPageSpec{
		{key: "operational", title: "Northwind Status", description: "Everything at Northwind, live.", pastDays: []int{9, 41, 77}, components: []publicComponentSpec{{"Website", "operational", ""}, {"API", "operational", ""}, {"Dashboard", "operational", ""}}},
		{key: "degraded", title: "Contoso Status", description: "Availability of Contoso services.", pastDays: []int{6, 33, 58, 84}, components: []publicComponentSpec{{"API", "degraded", "Responses are slower than usual."}, {"Web app", "operational", ""}, {"Notifications", "operational", ""}},
			open: []publicIncidentSpec{{key: "latency", title: "Elevated API latency", status: "identified", severity: "medium", summary: "Some API calls take several seconds.", affected: []string{"api"}, updates: []publicUpdateSpec{{"investigating", "We are looking into slow API responses.", 55}, {"identified", "A saturated database replica is the cause; traffic is being moved.", 25}}}}},
		{key: "outage", title: "Fabrikam Status", description: "Service health for Fabrikam.", pastDays: []int{4, 19, 38, 61}, components: []publicComponentSpec{{"Checkout", "major_outage", "Customers cannot pay."}, {"Catalog", "partial_outage", "Some searches fail."}, {"Account", "operational", ""}},
			open: []publicIncidentSpec{{key: "checkout", title: "Checkout is down", status: "investigating", severity: "critical", summary: "Payments cannot be completed.", affected: []string{"checkout", "catalog"}, updates: []publicUpdateSpec{{"investigating", "Checkout requests are failing for everyone.", 42}, {"investigating", "We are rolling back the last release.", 18}}}}},
		{key: "maintenance", title: "Tailspin Status", description: "Planned work and availability for Tailspin.", pastDays: []int{14, 45, 72}, maintenance: "active", components: []publicComponentSpec{{"Booking", "operational", ""}, {"Search", "operational", ""}, {"Payments", "operational", ""}}},
	}
}

func seedShowcasePublicPages(database *gorm.DB, now time.Time) error {
	for _, sp := range publicPageSpecs() {
		if err := seedPublicPage(database, sp, now); err != nil {
			return fmt.Errorf("public page %s: %w", sp.key, err)
		}
	}
	return nil
}

func seedPublicPage(database *gorm.DB, sp publicPageSpec, now time.Time) error {
	pageID, sectionID := "seed-public-page-"+sp.key, "seed-public-section-"+sp.key
	published := now.AddDate(0, 0, -100)
	page := db.StatusPage{ID: pageID, Slug: "seed-public-" + sp.key, Title: sp.title, Description: sp.description, Visibility: "public", DefaultIncidentVisibility: "published", PublishedAt: &published, DomainStatus: "pending",
		ThemeSettings: mustJSON(map[string]any{"accent_color": "#14b8a6", "theme_mode": "system", "show_uptime_summary": true, "show_incident_history": true}), CreatedAt: published, UpdatedAt: now}
	if err := database.Create(&page).Error; err != nil {
		return err
	}
	if err := database.Create(&db.StatusPageSection{ID: sectionID, StatusPageID: pageID, Name: "Services", SortOrder: 1, CreatedAt: published, UpdatedAt: now}).Error; err != nil {
		return err
	}
	ids := map[string]string{}
	for i, c := range sp.components {
		id := fmt.Sprintf("seed-public-component-%s-%d", sp.key, i)
		ids[lower(c.name)] = id
		comp := db.StatusPageComponent{ID: id, StatusPageID: pageID, SectionID: sectionID, PublicName: c.name, DisplayMode: "manual", ManualStatus: c.status, ManualStatusReason: c.reason, SortOrder: i, Visible: true, CreatedAt: published, UpdatedAt: now}
		if err := database.Create(&comp).Error; err != nil {
			return err
		}
		mapping := db.StatusPageComponentMapping{ID: fmt.Sprintf("seed-public-mapping-%s-%d", sp.key, i), ComponentID: id, ResourceType: "monitor", ResourceID: coreMonitorID("http-up"), HealthRollupStrategy: "worst", UptimeRollupStrategy: "worst", CreatedAt: published, UpdatedAt: now}
		if err := database.Create(&mapping).Error; err != nil {
			return err
		}
	}
	componentIDs := func(names []string) string {
		var out []string
		for _, n := range names {
			out = append(out, ids[n])
		}
		return mustJSON(out)
	}
	for _, o := range sp.open {
		inc := db.StatusPageIncident{ID: fmt.Sprintf("seed-public-inc-%s-%s", sp.key, o.key), StatusPageID: pageID, Title: o.title, PublicStatus: o.status, Severity: o.severity, ImpactSummary: o.summary, Visibility: "published", AffectedComponentIDs: componentIDs(o.affected),
			PublishedAt: ptrTime(now.Add(-time.Duration(o.updates[0].agoMin) * time.Minute)), CreatedAt: now.Add(-time.Duration(o.updates[0].agoMin) * time.Minute), UpdatedAt: now}
		if err := database.Create(&inc).Error; err != nil {
			return err
		}
		for i, u := range o.updates {
			at := now.Add(-time.Duration(u.agoMin) * time.Minute)
			if err := database.Create(&db.StatusPageIncidentUpdate{ID: fmt.Sprintf("seed-public-upd-%s-%s-%d", sp.key, o.key, i), IncidentID: inc.ID, Status: u.status, Message: u.message, CreatedBy: "admin", PublishedAt: &at, CreatedAt: at}).Error; err != nil {
				return err
			}
		}
	}
	titles := []string{"Elevated error rate", "Slow page loads", "Login failures", "Delayed emails"}
	first := sp.components[0].name
	for i, days := range sp.pastDays {
		start := now.AddDate(0, 0, -days).Add(-time.Duration(i*37) * time.Minute)
		duration := time.Duration(25+i*35) * time.Minute
		end := start.Add(duration)
		inc := db.StatusPageIncident{ID: fmt.Sprintf("seed-public-inc-%s-past-%d", sp.key, i), StatusPageID: pageID, Title: titles[i%len(titles)], PublicStatus: "resolved", Severity: "medium", ImpactSummary: "Resolved.", Visibility: "published",
			AffectedComponentIDs: componentIDs([]string{lower(first)}), PublishedAt: &start, ResolvedAt: &end, CreatedAt: start, UpdatedAt: end}
		if err := database.Create(&inc).Error; err != nil {
			return err
		}
		steps := []publicUpdateSpec{{"investigating", "We are investigating reports of problems.", 0}, {"identified", "The cause was found and a fix is being applied.", 10}, {"resolved", "This incident has been resolved.", int(duration.Minutes())}}
		for j, st := range steps {
			at := start.Add(time.Duration(st.agoMin) * time.Minute)
			if err := database.Create(&db.StatusPageIncidentUpdate{ID: fmt.Sprintf("seed-public-upd-%s-past-%d-%d", sp.key, i, j), IncidentID: inc.ID, Status: st.status, Message: st.message, CreatedBy: "admin", PublishedAt: &at, CreatedAt: at}).Error; err != nil {
				return err
			}
		}
	}
	past := now.AddDate(0, 0, -22)
	windows := []db.StatusPageMaintenance{{ID: "seed-public-maint-" + sp.key + "-done", StatusPageID: pageID, Title: "Scheduled infrastructure work", Description: "Completed without impact.", ComponentIDs: componentIDs([]string{lower(first)}), StartsAt: past, EndsAt: past.Add(time.Hour), CreatedAt: past.AddDate(0, 0, -3), UpdatedAt: past}}
	switch sp.maintenance {
	case "active":
		windows = append(windows, db.StatusPageMaintenance{ID: "seed-public-maint-" + sp.key + "-active", StatusPageID: pageID, Title: "Payments provider migration", Description: "Payments are unavailable while we migrate providers.", ComponentIDs: componentIDs([]string{"payments"}), StartsAt: now.Add(-30 * time.Minute), EndsAt: now.Add(90 * time.Minute), CreatedAt: now.AddDate(0, 0, -2), UpdatedAt: now.AddDate(0, 0, -2)},
			db.StatusPageMaintenance{ID: "seed-public-maint-" + sp.key + "-soon", StatusPageID: pageID, Title: "Search index rebuild", ComponentIDs: componentIDs([]string{"search"}), StartsAt: now.Add(52 * time.Hour), EndsAt: now.Add(54 * time.Hour), CreatedAt: now.AddDate(0, 0, -1), UpdatedAt: now.AddDate(0, 0, -1)})
	}
	return database.Create(&windows).Error
}

func lower(s string) string {
	out := []byte(s)
	for i, c := range out {
		if c >= 'A' && c <= 'Z' {
			out[i] = c + 32
		}
	}
	return string(out)
}

// publicPageLinks lists the URLs worth opening after a seed: one page per state and a link for every
// subscriber token state. Tokens in the seed are readable on purpose (the database stores only their hashes).
func publicPageLinks() []string {
	acme := "/status/seed-acme-cloud"
	links := []string{
		"/status/seed-public-operational", "/status/seed-public-degraded", "/status/seed-public-outage", "/status/seed-public-maintenance", acme,
		acme + "/history", acme + "/incidents/seed-state-pubinc-payments", acme + "/subscribe",
		acme + "/subscribers/confirm/confirm-pending (confirms on open)", acme + "/subscribers/confirm/confirm-expired (expired)",
		acme + "/subscribers/manage/manage-ada", acme + "/subscribers/manage/manage-cto (selected components)",
		acme + "/subscribers/unsubscribe/unsub-ada", acme + "/subscribers/unsubscribe/unsub-unsub (already unsubscribed)", acme + "/subscribers/unsubscribe/not-a-token (invalid)",
	}
	return links
}
