package main

import (
	"orion/core/internal/db"
	"time"

	"gorm.io/gorm"
)

// seedStatusPageDraft adds an unpublished page so the list shows both states.
func seedStatusPageDraft(database *gorm.DB, now time.Time) (int, error) {
	page := db.StatusPage{
		ID: "seed-status-page-internal", Slug: "seed-internal-tools", Title: "Internal tools", Visibility: "draft",
		ThemeSettings: `{"timezone":"Europe/London","accent_color":"#3366ff"}`, DefaultIncidentVisibility: "draft",
		DomainStatus: "pending", CreatedAt: now.AddDate(0, 0, -6), UpdatedAt: now.Add(-3 * time.Hour),
	}
	if err := database.Create(&page).Error; err != nil {
		return 0, err
	}
	section := db.StatusPageSection{ID: "seed-status-section-internal", StatusPageID: page.ID, Name: "Internal", CreatedAt: now, UpdatedAt: now}
	if err := database.Create(&section).Error; err != nil {
		return 0, err
	}
	for i, name := range []string{"Build runner", "Wiki"} {
		component := db.StatusPageComponent{ID: "seed-status-component-internal-" + string(rune('a'+i)), StatusPageID: page.ID, SectionID: section.ID, PublicName: name,
			DisplayMode: "single_resource", ManualStatus: "operational", SortOrder: i, Visible: true, CreatedAt: now, UpdatedAt: now}
		if err := database.Create(&component).Error; err != nil {
			return 0, err
		}
	}
	return 1, nil
}

// seedStatusPageMaintenance adds one upcoming window (holding alerts) and one finished window to the main page.
func seedStatusPageMaintenance(database *gorm.DB, now time.Time) (int, error) {
	windows := []db.StatusPageMaintenance{
		{ID: "seed-status-maintenance-db-upgrade", StatusPageID: "seed-status-page-main", Title: "Planned database upgrade", Description: "Writes are paused for a few minutes while we upgrade the primary.",
			ComponentIDs: `["seed-status-page-component-public-api","seed-status-page-component-database"]`, HoldAlerts: true, NotifySubscribers: true,
			StartsAt: now.Add(48 * time.Hour), EndsAt: now.Add(50 * time.Hour), CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour)},
		{ID: "seed-status-maintenance-past", StatusPageID: "seed-status-page-main", Title: "Network switch replacement", ComponentIDs: `["seed-status-page-component-worker"]`,
			StartsAt: now.AddDate(0, 0, -9), EndsAt: now.AddDate(0, 0, -9).Add(time.Hour), CreatedAt: now.AddDate(0, 0, -12), UpdatedAt: now.AddDate(0, 0, -9)},
	}
	if err := database.Create(&windows).Error; err != nil {
		return 0, err
	}
	return len(windows), nil
}
