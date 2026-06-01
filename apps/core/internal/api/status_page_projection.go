package api

import (
	"encoding/json"
	"orion/core/internal/db"
	"sort"
	"strings"
)

func (s *Server) validateStatusPageForPublish(pageID string) (StatusPagePublishValidationResponse, error) {
	detail, err := s.loadStatusPageDetail(pageID)
	if err != nil {
		return StatusPagePublishValidationResponse{}, err
	}

	validation := StatusPagePublishValidationResponse{
		Errors:   []string{},
		Warnings: []string{},
	}
	visibleComponents := 0
	for _, component := range detail.Components {
		if !component.Visible {
			continue
		}
		visibleComponents++
		if component.ManualStatus == "" && len(component.Mappings) == 0 {
			validation.Errors = append(validation.Errors, "visible component "+component.PublicName+" must have a mapped resource or manual status")
		}
		if looksLikePrivateStatusPageLabel(component.PublicName) {
			validation.Errors = append(validation.Errors, "visible component "+component.PublicName+" looks like an internal host, IP address, or private domain")
		}
	}
	if visibleComponents == 0 {
		validation.Errors = append(validation.Errors, "status page must have at least one visible component")
	}
	if looksLikePrivateStatusPageLabel(detail.Page.Title) {
		validation.Warnings = append(validation.Warnings, "status page title looks like an internal host, IP address, or private domain")
	}
	for _, incident := range detail.Incidents {
		if incident.Visibility != statusPageIncidentVisibilityPublished {
			continue
		}
		hasPublishedUpdate := false
		for _, update := range incident.Updates {
			if update.PublishedAt != nil && strings.TrimSpace(update.Message) != "" {
				hasPublishedUpdate = true
				break
			}
		}
		if !hasPublishedUpdate {
			validation.Errors = append(validation.Errors, "published incident "+incident.Title+" must have at least one published update message")
		}
	}
	return validation, nil
}

func (s *Server) statusPageComponentStatus(component StatusPageComponentResponse) string {
	if component.ManualStatus != "" {
		return component.ManualStatus
	}
	if len(component.Mappings) == 0 {
		return "unknown"
	}
	statuses := make([]string, 0, len(component.Mappings))
	for _, mapping := range component.Mappings {
		statuses = append(statuses, s.statusPageMappedResourceStatus(mapping))
	}
	sort.Slice(statuses, func(i, j int) bool {
		return statusPageStatusWeight(statuses[i]) > statusPageStatusWeight(statuses[j])
	})
	return statuses[0]
}

func (s *Server) statusPageMappedResourceStatus(mapping StatusPageComponentMappingResponse) string {
	switch mapping.ResourceType {
	case "monitor":
		var monitor db.Monitor
		if err := s.db.Where("id = ?", mapping.ResourceID).First(&monitor).Error; err != nil {
			return "unknown"
		}
		health := monitor.ComputedHealth
		if health == "" || health == "unknown" {
			health = monitor.Health
		}
		return publicStatusFromHealth(health)
	case "agent":
		return "unknown"
	default:
		return "unknown"
	}
}

func statusPageResponse(page db.StatusPage) StatusPageResponse {
	return StatusPageResponse{
		ID:                        page.ID,
		Slug:                      page.Slug,
		CustomDomain:              page.CustomDomain,
		Title:                     page.Title,
		Description:               page.Description,
		SEOTitle:                  page.SEOTitle,
		SEODescription:            page.SEODescription,
		OpenGraphImageURL:         page.OpenGraphImageURL,
		CanonicalURL:              page.CanonicalURL,
		Visibility:                page.Visibility,
		ThemeSettings:             decodeJSONObject(page.ThemeSettings),
		DefaultIncidentVisibility: page.DefaultIncidentVisibility,
		PublishedAt:               page.PublishedAt,
		CreatedAt:                 page.CreatedAt,
		UpdatedAt:                 page.UpdatedAt,
	}
}

func statusPageSectionResponses(sections []db.StatusPageSection) []StatusPageSectionResponse {
	responses := make([]StatusPageSectionResponse, 0, len(sections))
	for _, section := range sections {
		responses = append(responses, statusPageSectionResponse(section))
	}
	return responses
}

func statusPageSectionResponse(section db.StatusPageSection) StatusPageSectionResponse {
	return StatusPageSectionResponse{
		ID:                 section.ID,
		StatusPageID:       section.StatusPageID,
		Name:               section.Name,
		SortOrder:          section.SortOrder,
		CollapsedByDefault: section.CollapsedByDefault,
		CreatedAt:          section.CreatedAt,
		UpdatedAt:          section.UpdatedAt,
	}
}

func statusPageComponentResponse(component db.StatusPageComponent, mappings []StatusPageComponentMappingResponse) StatusPageComponentResponse {
	if mappings == nil {
		mappings = []StatusPageComponentMappingResponse{}
	}
	return StatusPageComponentResponse{
		ID:                 component.ID,
		StatusPageID:       component.StatusPageID,
		SectionID:          component.SectionID,
		PublicName:         component.PublicName,
		PublicDescription:  component.PublicDescription,
		DisplayMode:        component.DisplayMode,
		ManualStatus:       component.ManualStatus,
		ManualStatusReason: component.ManualStatusReason,
		SortOrder:          component.SortOrder,
		Visible:            component.Visible,
		Mappings:           mappings,
		CreatedAt:          component.CreatedAt,
		UpdatedAt:          component.UpdatedAt,
	}
}

func statusPageComponentMappingResponse(mapping db.StatusPageComponentMapping) StatusPageComponentMappingResponse {
	return StatusPageComponentMappingResponse{
		ID:                   mapping.ID,
		ComponentID:          mapping.ComponentID,
		ResourceType:         mapping.ResourceType,
		ResourceID:           mapping.ResourceID,
		HealthRollupStrategy: mapping.HealthRollupStrategy,
		UptimeRollupStrategy: mapping.UptimeRollupStrategy,
		CreatedAt:            mapping.CreatedAt,
		UpdatedAt:            mapping.UpdatedAt,
	}
}

func statusPageIncidentResponse(incident db.StatusPageIncident, updates []StatusPageIncidentUpdateResponse) StatusPageIncidentResponse {
	if updates == nil {
		updates = []StatusPageIncidentUpdateResponse{}
	}
	return StatusPageIncidentResponse{
		ID:                   incident.ID,
		StatusPageID:         incident.StatusPageID,
		InternalIncidentID:   incident.InternalIncidentID,
		Title:                incident.Title,
		PublicStatus:         incident.PublicStatus,
		Severity:             incident.Severity,
		ImpactSummary:        incident.ImpactSummary,
		Visibility:           incident.Visibility,
		AffectedComponentIDs: decodeResponseList(incident.AffectedComponentIDs, nil),
		PublishedAt:          incident.PublishedAt,
		ResolvedAt:           incident.ResolvedAt,
		ScheduledStartAt:     incident.ScheduledStartAt,
		ScheduledEndAt:       incident.ScheduledEndAt,
		Updates:              updates,
		CreatedAt:            incident.CreatedAt,
		UpdatedAt:            incident.UpdatedAt,
	}
}

func statusPageIncidentUpdateResponse(update db.StatusPageIncidentUpdate) StatusPageIncidentUpdateResponse {
	return StatusPageIncidentUpdateResponse{
		ID:          update.ID,
		IncidentID:  update.IncidentID,
		Status:      update.Status,
		Message:     update.Message,
		CreatedBy:   update.CreatedBy,
		PublishedAt: update.PublishedAt,
		CreatedAt:   update.CreatedAt,
	}
}

func decodeJSONObject(value string) map[string]interface{} {
	result := map[string]interface{}{}
	if strings.TrimSpace(value) == "" {
		return result
	}
	_ = json.Unmarshal([]byte(value), &result)
	return result
}

func publicStatusFromHealth(value string) string {
	switch value {
	case "up":
		return "operational"
	case "degraded":
		return "degraded"
	case "down", "stale":
		return "major_outage"
	default:
		return "unknown"
	}
}

func statusPageStatusWeight(value string) int {
	switch value {
	case "major_outage":
		return 5
	case "partial_outage":
		return 4
	case "degraded":
		return 3
	case "maintenance":
		return 2
	case "unknown":
		return 1
	case "operational":
		return 0
	default:
		return 1
	}
}
