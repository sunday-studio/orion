package api

import (
	"orion/core/internal/db"
	"strings"
)

func applyStatusPageIncidentUpdateRequest(update *db.StatusPageIncidentUpdate, request statusPageIncidentUpdateRequest, create bool) error {
	if request.Status != nil {
		update.Status = strings.TrimSpace(*request.Status)
	}
	if request.Message != nil {
		update.Message = strings.TrimSpace(*request.Message)
	}
	if request.CreatedBy != nil {
		update.CreatedBy = strings.TrimSpace(*request.CreatedBy)
	}
	if request.PublishedAt != nil {
		update.PublishedAt = request.PublishedAt
	}
	if create && update.Status == "" {
		update.Status = "investigating"
	}
	if strings.TrimSpace(update.Message) == "" {
		return &requestValidationError{message: "status page incident update message is required"}
	}
	if !validStatusPageIncidentStatus(update.Status) {
		return &requestValidationError{message: "unsupported status page incident update status"}
	}
	return nil
}
func (s *Server) ensureStatusPageComponentsExist(pageID string, componentIDs []string) error {
	if len(componentIDs) == 0 {
		return nil
	}
	var count int64
	if err := s.db.Model(&db.StatusPageComponent{}).Where("status_page_id = ? AND id IN ?", pageID, componentIDs).Count(&count).Error; err != nil {
		return err
	}
	if int(count) != len(componentIDs) {
		return &requestValidationError{message: "affected_component_ids must reference components on this status page"}
	}
	return nil
}
func statusPageIncidentSuggestionMatchReason(resourceType string) string {
	switch resourceType {
	case "monitor":
		return "internal incident monitor matched a public component mapping"
	case "agent":
		return "internal incident agent matched a public component mapping"
	default:
		return "internal incident resource matched a public component mapping"
	}
}
