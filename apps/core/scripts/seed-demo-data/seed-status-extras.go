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
