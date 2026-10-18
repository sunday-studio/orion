package api

import (
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"time"
)

// seedStatusPageFromMonitors adds a Services section with one mapped component per user monitor.
func (s *Server) seedStatusPageFromMonitors(pageID string) error {
	var monitors []db.Monitor
	if err := s.db.Where("system = ? AND (deleted_at IS NULL OR deleted_at = ?)", false, time.Time{}).Order("name ASC").Limit(20).Find(&monitors).Error; err != nil {
		return err
	}
	if len(monitors) == 0 {
		return nil
	}
	now := time.Now().UTC()
	section := db.StatusPageSection{ID: utils.GenerateID("status_page_section"), StatusPageID: pageID, Name: "Services", CreatedAt: now, UpdatedAt: now}
	if err := s.db.Create(&section).Error; err != nil {
		return err
	}
	for i, m := range monitors {
		component := db.StatusPageComponent{ID: utils.GenerateID("status_page_component"), StatusPageID: pageID, SectionID: section.ID, PublicName: m.Name, DisplayMode: "single_resource", SortOrder: i, Visible: true, CreatedAt: now, UpdatedAt: now}
		if err := s.db.Create(&component).Error; err != nil {
			return err
		}
		mapping := db.StatusPageComponentMapping{ID: utils.GenerateID("status_page_mapping"), ComponentID: component.ID, ResourceType: "monitor", ResourceID: m.ID, HealthRollupStrategy: "worst", UptimeRollupStrategy: "worst", CreatedAt: now, UpdatedAt: now}
		if err := s.db.Create(&mapping).Error; err != nil {
			return err
		}
	}
	return nil
}
