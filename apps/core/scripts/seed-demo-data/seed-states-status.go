package main

import (
	"time"

	"orion/core/internal/db"

	"gorm.io/gorm"
)

const (
	acmePageID  = "seed-state-page-acme"
	betaPageID  = "seed-state-page-beta"
	emptyPageID = "seed-state-page-empty"
)

type componentSpec struct {
	key, name, section, mode, manual, reason string
	hidden                                   bool
	mappings                                 [][2]string // {resource type, resource id}
}

// acmeComponents covers every component state: operational, degraded, partial/major outage,
// maintenance, unknown (manual), computed from mapped monitors and agents, aggregate of two, and hidden.
func acmeComponents() []componentSpec {
	return []componentSpec{
		{key: "api", name: "API", section: "core", mode: "aggregate", mappings: [][2]string{{"monitor", coreMonitorID("http-up")}, {"monitor", coreMonitorID("http-down")}}},
		{key: "dashboard", name: "Dashboard", section: "core", mode: "single_resource", mappings: [][2]string{{"monitor", coreMonitorID("http-up")}}},
		{key: "webhooks", name: "Webhooks", section: "core", mode: "manual", manual: "degraded", reason: "Delivery delays of up to 5 minutes."},
		{key: "database", name: "Database", section: "infra", mode: "single_resource", mappings: [][2]string{{"monitor", agentMonitorID("db-down", "pg")}}},
		{key: "storage", name: "File storage", section: "infra", mode: "manual", manual: "partial_outage", reason: "Uploads fail for some regions."},
		{key: "jobs", name: "Background jobs", section: "infra", mode: "manual", manual: "maintenance", reason: "Scheduled maintenance in progress."},
		{key: "payments", name: "Payments provider", section: "third", mode: "manual", manual: "major_outage", reason: "Provider outage, see incident."},
		{key: "legacy", name: "Legacy API", section: "third", mode: "manual", manual: "unknown"},
		{key: "email", name: "Email delivery", section: "third", mode: "manual", manual: "operational"},
		{key: "internal", name: "Internal admin", section: "third", mode: "manual", manual: "operational", hidden: true},
	}
}

func seedShowcaseStatusPages(database *gorm.DB, now time.Time) error {
	published := now.AddDate(0, 0, -45)
	checked := now.Add(-2 * time.Minute)
	pages := []db.StatusPage{
		{ID: acmePageID, Slug: "seed-acme-cloud", CustomDomain: "status.acme-cloud.test", Title: "Acme Cloud Status", Description: "Live status for Acme Cloud services.", SEOTitle: "Acme Cloud status", SEODescription: "Current availability of Acme Cloud.",
			Visibility: "public", ThemeSettings: mustJSON(map[string]any{"accent_color": "#2563eb", "timezone": "Europe/London", "theme_mode": "system", "show_uptime_summary": true, "show_incident_history": true}),
			DefaultIncidentVisibility: "published", PublishedAt: &published, DomainStatus: "pending", DomainCheckedAt: &checked, DomainError: "Record not found yet", CreatedAt: published, UpdatedAt: now.Add(-20 * time.Minute)},
		{ID: betaPageID, Slug: "seed-beta-platform", CustomDomain: "status.beta-platform.test", Title: "Beta Platform", Description: "", Visibility: "unlisted", ThemeSettings: `{"timezone":"America/New_York"}`, DefaultIncidentVisibility: "draft",
			PublishedAt: ptrTime(now.AddDate(0, 0, -3)), DomainStatus: "failed", DomainCheckedAt: &checked, DomainError: "Record points somewhere else", CreatedAt: now.AddDate(0, 0, -4), UpdatedAt: now.AddDate(0, 0, -1)},
		{ID: emptyPageID, Slug: "seed-empty-draft", Title: "Empty draft", Visibility: "draft", ThemeSettings: "{}", DefaultIncidentVisibility: "draft", DomainStatus: "pending", CreatedAt: now.Add(-30 * time.Minute), UpdatedAt: now.Add(-30 * time.Minute)},
	}
	if err := database.Create(&pages).Error; err != nil {
		return err
	}
	sections := []db.StatusPageSection{
		{ID: "seed-state-section-core", StatusPageID: acmePageID, Name: "Core platform", SortOrder: 1, CreatedAt: published, UpdatedAt: now},
		{ID: "seed-state-section-infra", StatusPageID: acmePageID, Name: "Infrastructure", SortOrder: 2, CreatedAt: published, UpdatedAt: now},
		{ID: "seed-state-section-third", StatusPageID: acmePageID, Name: "Third parties", SortOrder: 3, CollapsedByDefault: true, CreatedAt: published, UpdatedAt: now},
		{ID: "seed-state-section-beta", StatusPageID: betaPageID, Name: "Platform", SortOrder: 1, CreatedAt: now.AddDate(0, 0, -4), UpdatedAt: now},
	}
	if err := database.Create(&sections).Error; err != nil {
		return err
	}
	for i, c := range acmeComponents() {
		comp := db.StatusPageComponent{ID: "seed-state-component-" + c.key, StatusPageID: acmePageID, SectionID: "seed-state-section-" + c.section, PublicName: c.name, DisplayMode: c.mode,
			ManualStatus: c.manual, ManualStatusReason: c.reason, SortOrder: i, Visible: true, CreatedAt: published, UpdatedAt: now}
		if err := database.Create(&comp).Error; err != nil {
			return err
		}
		if c.hidden { // Visible defaults to true on create, so hide explicitly.
			if err := database.Model(&db.StatusPageComponent{}).Where("id = ?", comp.ID).Update("visible", false).Error; err != nil {
				return err
			}
		}
		for j, m := range c.mappings {
			mp := db.StatusPageComponentMapping{ID: "seed-state-mapping-" + c.key + "-" + string(rune('a'+j)), ComponentID: comp.ID, ResourceType: m[0], ResourceID: m[1], HealthRollupStrategy: "worst", UptimeRollupStrategy: "worst", CreatedAt: published, UpdatedAt: now}
			if err := database.Create(&mp).Error; err != nil {
				return err
			}
		}
	}
	for i, name := range []string{"Web app", "Public API"} {
		comp := db.StatusPageComponent{ID: "seed-state-component-beta-" + string(rune('a'+i)), StatusPageID: betaPageID, SectionID: "seed-state-section-beta", PublicName: name, DisplayMode: "manual", ManualStatus: "operational", SortOrder: i, Visible: true, CreatedAt: now, UpdatedAt: now}
		if err := database.Create(&comp).Error; err != nil {
			return err
		}
	}
	if err := seedShowcasePublicIncidents(database, now); err != nil {
		return err
	}
	if err := seedShowcaseStatusMaintenance(database, now); err != nil {
		return err
	}
	return seedShowcaseStatusSubscribers(database, now)
}

func seedShowcaseStatusMaintenance(database *gorm.DB, now time.Time) error {
	windows := []db.StatusPageMaintenance{
		{ID: "seed-state-maint-active", StatusPageID: acmePageID, Title: "Job queue migration", Description: "Background jobs pause while we move to the new queue cluster.", ComponentIDs: `["seed-state-component-jobs"]`, HoldAlerts: true, NotifySubscribers: true, NoticeSentAt: ptrTime(now.Add(-24 * time.Hour)),
			StartsAt: now.Add(-40 * time.Minute), EndsAt: now.Add(80 * time.Minute), CreatedAt: now.AddDate(0, 0, -2), UpdatedAt: now.AddDate(0, 0, -2)},
		{ID: "seed-state-maint-soon", StatusPageID: acmePageID, Title: "Database minor version upgrade", Description: "Brief read-only period expected.", ComponentIDs: `["seed-state-component-database","seed-state-component-api"]`, HoldAlerts: true, NotifySubscribers: true,
			StartsAt: now.Add(30 * time.Hour), EndsAt: now.Add(32 * time.Hour), CreatedAt: now.AddDate(0, 0, -1), UpdatedAt: now.AddDate(0, 0, -1)},
		{ID: "seed-state-maint-later", StatusPageID: acmePageID, Title: "Certificate rotation", ComponentIDs: `["seed-state-component-dashboard"]`, StartsAt: now.AddDate(0, 0, 9), EndsAt: now.AddDate(0, 0, 9).Add(time.Hour), CreatedAt: now.Add(-3 * time.Hour), UpdatedAt: now.Add(-3 * time.Hour)},
		{ID: "seed-state-maint-done", StatusPageID: acmePageID, Title: "Storage hardware swap", Description: "Completed without customer impact.", ComponentIDs: `["seed-state-component-storage"]`, HoldAlerts: true,
			StartsAt: now.AddDate(0, 0, -6), EndsAt: now.AddDate(0, 0, -6).Add(90 * time.Minute), CreatedAt: now.AddDate(0, 0, -10), UpdatedAt: now.AddDate(0, 0, -6)},
	}
	return database.Create(&windows).Error
}
