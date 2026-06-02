package api

import (
	"errors"
	"github.com/gin-gonic/gin"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"strings"
)

func (s *Server) statusPageIncidentDraft(pageID string, incident db.Incident, requestedComponentIDs []string) (StatusPageIncidentDraftResponse, error) {
	suggestions, err := s.statusPageIncidentComponentSuggestions(pageID, incident)
	if err != nil {
		return StatusPageIncidentDraftResponse{}, err
	}
	componentIDs := normalizeStringList(requestedComponentIDs)
	if len(componentIDs) == 0 {
		componentIDs = make([]string, 0, len(suggestions))
		for _, suggestion := range suggestions {
			componentIDs = append(componentIDs, suggestion.ComponentID)
		}
	}
	componentNames, err := s.visibleStatusPageComponentNames(pageID, componentIDs)
	if err != nil {
		return StatusPageIncidentDraftResponse{}, err
	}
	componentLabel := publicIncidentComponentLabel(componentNames)
	publicStatus := publicIncidentDraftStatus(incident)
	severity := publicIncidentDraftSeverity(incident.Severity)
	return StatusPageIncidentDraftResponse{Title: publicIncidentDraftTitle(componentLabel), PublicStatus: publicStatus, Severity: severity, ImpactSummary: publicIncidentDraftImpactSummary(componentLabel), InitialUpdateMessage: publicIncidentDraftUpdateMessage(componentLabel), AffectedComponentIDs: componentIDs, Suggestions: suggestions}, nil
}
func (s *Server) visibleStatusPageComponentNames(pageID string, componentIDs []string) ([]string, error) {
	if len(componentIDs) == 0 {
		return []string{}, nil
	}
	var components []db.StatusPageComponent
	if err := s.db.Where("status_page_id = ? AND visible = ? AND id IN ?", pageID, true, componentIDs).Find(&components).Error; err != nil {
		return nil, err
	}
	namesByID := make(map[string]string, len(components))
	for _, component := range components {
		namesByID[component.ID] = component.PublicName
	}
	if len(namesByID) != len(componentIDs) {
		return nil, &requestValidationError{message: "affected_component_ids must reference visible components on this status page"}
	}
	names := make([]string, 0, len(componentIDs))
	for _, componentID := range componentIDs {
		names = append(names, namesByID[componentID])
	}
	return names, nil
}
func publicIncidentComponentLabel(componentNames []string) string {
	if len(componentNames) == 0 {
		return "one or more services"
	}
	if len(componentNames) == 1 {
		return componentNames[0]
	}
	if len(componentNames) == 2 {
		return componentNames[0] + " and " + componentNames[1]
	}
	return strings.Join(componentNames[:2], ", ") + ", and other services"
}
func publicIncidentDraftTitle(componentLabel string) string {
	if componentLabel == "one or more services" {
		return "Investigating a service issue"
	}
	return "Investigating an issue affecting " + componentLabel
}
func publicIncidentDraftImpactSummary(componentLabel string) string {
	return "We are investigating a disruption affecting " + componentLabel + ". We will share updates as more information is available."
}
func publicIncidentDraftUpdateMessage(componentLabel string) string {
	return "We are investigating reports of impact to " + componentLabel + ". The team is confirming scope and working toward recovery."
}
func publicIncidentDraftStatus(incident db.Incident) string {
	if incident.Status == "resolved" {
		return "resolved"
	}
	return "investigating"
}
func publicIncidentDraftSeverity(severity string) string {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "low", "medium", "high", "critical":
		return strings.ToLower(strings.TrimSpace(severity))
	case "error":
		return "high"
	default:
		return "medium"
	}
}
func writeStatusPageDraftError(c *gin.Context, err error) {
	var validationErr *requestValidationError
	if errors.As(err, &validationErr) {
		utils.BadRequest(c, validationErr.Error())
		return
	}
	utils.InternalError(c, "Failed to generate status page incident draft", err)
}
