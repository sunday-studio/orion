package api

import (
	"orion/core/internal/db"
	"strings"
)

func (s *Server) applyStatusPageComponentRequest(component *db.StatusPageComponent, request statusPageComponentRequest, create bool) error {
	if request.SectionID != nil {
		component.SectionID = strings.TrimSpace(*request.SectionID)
	}
	if request.PublicName != nil {
		component.PublicName = strings.TrimSpace(*request.PublicName)
	}
	if request.PublicDescription != nil {
		component.PublicDescription = strings.TrimSpace(*request.PublicDescription)
	}
	if request.DisplayMode != nil {
		component.DisplayMode = strings.TrimSpace(*request.DisplayMode)
	}
	if request.ManualStatus != nil {
		component.ManualStatus = strings.TrimSpace(*request.ManualStatus)
	}
	if request.ManualStatusReason != nil {
		component.ManualStatusReason = strings.TrimSpace(*request.ManualStatusReason)
	}
	if request.SortOrder != nil {
		component.SortOrder = *request.SortOrder
	}
	if request.Visible != nil {
		component.Visible = *request.Visible
	}
	if create && component.DisplayMode == "" {
		component.DisplayMode = "single_resource"
	}
	if strings.TrimSpace(component.SectionID) == "" {
		return &requestValidationError{message: "status page component section_id is required"}
	}
	if strings.TrimSpace(component.PublicName) == "" {
		return &requestValidationError{message: "status page component public_name is required"}
	}
	if !validStatusPageDisplayMode(component.DisplayMode) {
		return &requestValidationError{message: "unsupported status page component display_mode"}
	}
	if component.ManualStatus != "" && !validStatusPageComponentStatus(component.ManualStatus) {
		return &requestValidationError{message: "unsupported status page component manual_status"}
	}
	var count int64
	if err := s.db.Model(&db.StatusPageSection{}).Where("id = ? AND status_page_id = ?", component.SectionID, component.StatusPageID).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return &requestValidationError{message: "status page component section_id must reference a section on this status page"}
	}
	return nil
}
func (s *Server) applyStatusPageComponentMappingRequest(mapping *db.StatusPageComponentMapping, request statusPageComponentMappingRequest, create bool) error {
	if request.ResourceType != nil {
		mapping.ResourceType = strings.TrimSpace(*request.ResourceType)
	}
	if request.ResourceID != nil {
		mapping.ResourceID = strings.TrimSpace(*request.ResourceID)
	}
	if request.HealthRollupStrategy != nil {
		mapping.HealthRollupStrategy = strings.TrimSpace(*request.HealthRollupStrategy)
	}
	if request.UptimeRollupStrategy != nil {
		mapping.UptimeRollupStrategy = strings.TrimSpace(*request.UptimeRollupStrategy)
	}
	if create && mapping.HealthRollupStrategy == "" {
		mapping.HealthRollupStrategy = "worst"
	}
	if create && mapping.UptimeRollupStrategy == "" {
		mapping.UptimeRollupStrategy = "worst"
	}
	if strings.TrimSpace(mapping.ResourceType) == "" || strings.TrimSpace(mapping.ResourceID) == "" {
		return &requestValidationError{message: "status page component mapping requires resource_type and resource_id"}
	}
	if !validStatusPageResourceType(mapping.ResourceType) {
		return &requestValidationError{message: "unsupported status page component mapping resource_type"}
	}
	if !validStatusPageRollupStrategy(mapping.HealthRollupStrategy) || !validStatusPageRollupStrategy(mapping.UptimeRollupStrategy) {
		return &requestValidationError{message: "unsupported status page component mapping rollup strategy"}
	}
	if err := s.ensureStatusPageMappingResourceExists(mapping.ResourceType, mapping.ResourceID); err != nil {
		return err
	}
	return nil
}
func (s *Server) ensureStatusPageMappingResourceExists(resourceType string, resourceID string) error {
	var count int64
	switch resourceType {
	case "agent":
		if err := s.db.Model(&db.Agent{}).Where("id = ?", resourceID).Count(&count).Error; err != nil {
			return err
		}
	case "monitor":
		if err := s.db.Model(&db.Monitor{}).Where("id = ?", resourceID).Count(&count).Error; err != nil {
			return err
		}
	}
	if count == 0 {
		return &requestValidationError{message: "status page component mapping resource_id must reference an existing resource"}
	}
	return nil
}
