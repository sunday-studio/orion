package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"orion/core/internal/db"
	"orion/core/internal/service"
	"orion/core/internal/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	statusPageVisibilityDraft    = "draft"
	statusPageVisibilityPublic   = "public"
	statusPageVisibilityUnlisted = "unlisted"

	statusPageIncidentVisibilityDraft     = "draft"
	statusPageIncidentVisibilityPublished = "published"
	statusPageIncidentVisibilityPrivate   = "private"
)

type statusPageRequest struct {
	Slug                      *string                `json:"slug"`
	CustomDomain              *string                `json:"custom_domain"`
	Title                     *string                `json:"title"`
	Description               *string                `json:"description"`
	SEOTitle                  *string                `json:"seo_title"`
	SEODescription            *string                `json:"seo_description"`
	OpenGraphImageURL         *string                `json:"open_graph_image_url"`
	CanonicalURL              *string                `json:"canonical_url"`
	Visibility                *string                `json:"visibility"`
	ThemeSettings             map[string]interface{} `json:"theme_settings"`
	DefaultIncidentVisibility *string                `json:"default_incident_visibility"`
}

type statusPageSectionRequest struct {
	Name               *string `json:"name"`
	SortOrder          *int    `json:"sort_order"`
	CollapsedByDefault *bool   `json:"collapsed_by_default"`
}

type statusPageComponentRequest struct {
	SectionID          *string `json:"section_id"`
	PublicName         *string `json:"public_name"`
	PublicDescription  *string `json:"public_description"`
	DisplayMode        *string `json:"display_mode"`
	ManualStatus       *string `json:"manual_status"`
	ManualStatusReason *string `json:"manual_status_reason"`
	SortOrder          *int    `json:"sort_order"`
	Visible            *bool   `json:"visible"`
}

type statusPageComponentMappingRequest struct {
	ResourceType         *string `json:"resource_type"`
	ResourceID           *string `json:"resource_id"`
	HealthRollupStrategy *string `json:"health_rollup_strategy"`
	UptimeRollupStrategy *string `json:"uptime_rollup_strategy"`
}

type statusPageIncidentRequest struct {
	InternalIncidentID   *string    `json:"internal_incident_id"`
	Title                *string    `json:"title"`
	PublicStatus         *string    `json:"public_status"`
	Severity             *string    `json:"severity"`
	ImpactSummary        *string    `json:"impact_summary"`
	Visibility           *string    `json:"visibility"`
	AffectedComponentIDs []string   `json:"affected_component_ids"`
	PublishedAt          *time.Time `json:"published_at"`
	ResolvedAt           *time.Time `json:"resolved_at"`
	ScheduledStartAt     *time.Time `json:"scheduled_start_at"`
	ScheduledEndAt       *time.Time `json:"scheduled_end_at"`
}

type statusPageIncidentUpdateRequest struct {
	Status      *string    `json:"status"`
	Message     *string    `json:"message"`
	CreatedBy   *string    `json:"created_by"`
	PublishedAt *time.Time `json:"published_at"`
}

type StatusPageResponse struct {
	ID                        string                 `json:"id"`
	Slug                      string                 `json:"slug"`
	CustomDomain              string                 `json:"custom_domain,omitempty"`
	Title                     string                 `json:"title"`
	Description               string                 `json:"description,omitempty"`
	SEOTitle                  string                 `json:"seo_title,omitempty"`
	SEODescription            string                 `json:"seo_description,omitempty"`
	OpenGraphImageURL         string                 `json:"open_graph_image_url,omitempty"`
	CanonicalURL              string                 `json:"canonical_url,omitempty"`
	Visibility                string                 `json:"visibility"`
	ThemeSettings             map[string]interface{} `json:"theme_settings"`
	DefaultIncidentVisibility string                 `json:"default_incident_visibility"`
	PublishedAt               *time.Time             `json:"published_at,omitempty"`
	CreatedAt                 time.Time              `json:"created_at"`
	UpdatedAt                 time.Time              `json:"updated_at"`
}

type StatusPageSectionResponse struct {
	ID                 string    `json:"id"`
	StatusPageID       string    `json:"status_page_id"`
	Name               string    `json:"name"`
	SortOrder          int       `json:"sort_order"`
	CollapsedByDefault bool      `json:"collapsed_by_default"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type StatusPageComponentResponse struct {
	ID                 string                               `json:"id"`
	StatusPageID       string                               `json:"status_page_id"`
	SectionID          string                               `json:"section_id"`
	PublicName         string                               `json:"public_name"`
	PublicDescription  string                               `json:"public_description,omitempty"`
	DisplayMode        string                               `json:"display_mode"`
	ManualStatus       string                               `json:"manual_status,omitempty"`
	ManualStatusReason string                               `json:"manual_status_reason,omitempty"`
	SortOrder          int                                  `json:"sort_order"`
	Visible            bool                                 `json:"visible"`
	Mappings           []StatusPageComponentMappingResponse `json:"mappings,omitempty"`
	CreatedAt          time.Time                            `json:"created_at"`
	UpdatedAt          time.Time                            `json:"updated_at"`
}

type StatusPageComponentMappingResponse struct {
	ID                   string    `json:"id"`
	ComponentID          string    `json:"component_id"`
	ResourceType         string    `json:"resource_type"`
	ResourceID           string    `json:"resource_id"`
	HealthRollupStrategy string    `json:"health_rollup_strategy"`
	UptimeRollupStrategy string    `json:"uptime_rollup_strategy"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

type StatusPageIncidentResponse struct {
	ID                   string                             `json:"id"`
	StatusPageID         string                             `json:"status_page_id"`
	InternalIncidentID   string                             `json:"internal_incident_id,omitempty"`
	Title                string                             `json:"title"`
	PublicStatus         string                             `json:"public_status"`
	Severity             string                             `json:"severity"`
	ImpactSummary        string                             `json:"impact_summary,omitempty"`
	Visibility           string                             `json:"visibility"`
	AffectedComponentIDs []string                           `json:"affected_component_ids"`
	PublishedAt          *time.Time                         `json:"published_at,omitempty"`
	ResolvedAt           *time.Time                         `json:"resolved_at,omitempty"`
	ScheduledStartAt     *time.Time                         `json:"scheduled_start_at,omitempty"`
	ScheduledEndAt       *time.Time                         `json:"scheduled_end_at,omitempty"`
	Updates              []StatusPageIncidentUpdateResponse `json:"updates,omitempty"`
	CreatedAt            time.Time                          `json:"created_at"`
	UpdatedAt            time.Time                          `json:"updated_at"`
}

type StatusPageIncidentUpdateResponse struct {
	ID          string     `json:"id"`
	IncidentID  string     `json:"incident_id"`
	Status      string     `json:"status"`
	Message     string     `json:"message"`
	CreatedBy   string     `json:"created_by,omitempty"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

type StatusPageDetailResponse struct {
	Page       StatusPageResponse            `json:"page"`
	Sections   []StatusPageSectionResponse   `json:"sections"`
	Components []StatusPageComponentResponse `json:"components"`
	Incidents  []StatusPageIncidentResponse  `json:"incidents"`
}

type StatusPagePreviewResponse struct {
	Page                 StatusPagePublicPageResponse       `json:"page"`
	Metadata             StatusPagePublicMetadataResponse   `json:"metadata"`
	Sections             []StatusPagePublicSectionResponse  `json:"sections"`
	Incidents            []StatusPagePublicIncidentResponse `json:"incidents"`
	OverallStatus        string                             `json:"overall_status"`
	OverallStatusDisplay string                             `json:"overall_status_display"`
	LastUpdated          time.Time                          `json:"last_updated"`
}

type StatusPagePublicMetadataResponse struct {
	Title        string                            `json:"title"`
	Description  string                            `json:"description,omitempty"`
	CanonicalURL string                            `json:"canonical_url,omitempty"`
	OpenGraph    StatusPagePublicOpenGraphResponse `json:"open_graph"`
}

type StatusPagePublicOpenGraphResponse struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	URL         string `json:"url,omitempty"`
	Type        string `json:"type"`
	SiteName    string `json:"site_name"`
	ImageURL    string `json:"image_url,omitempty"`
}

type StatusPagePublicPageResponse struct {
	Slug          string                 `json:"slug"`
	Title         string                 `json:"title"`
	Description   string                 `json:"description,omitempty"`
	Visibility    string                 `json:"visibility"`
	ThemeSettings map[string]interface{} `json:"theme_settings"`
}

type StatusPagePublicSectionResponse struct {
	ID                 string                              `json:"id"`
	Name               string                              `json:"name"`
	CollapsedByDefault bool                                `json:"collapsed_by_default"`
	Components         []StatusPagePublicComponentResponse `json:"components"`
}

type StatusPagePublicComponentResponse struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description,omitempty"`
	Status        string `json:"status"`
	StatusDisplay string `json:"status_display"`
	StatusReason  string `json:"status_reason,omitempty"`
	DisplayMode   string `json:"display_mode"`
}

type StatusPagePublicIncidentResponse struct {
	ID                   string     `json:"id"`
	Title                string     `json:"title"`
	PublicStatus         string     `json:"public_status"`
	Severity             string     `json:"severity"`
	ImpactSummary        string     `json:"impact_summary,omitempty"`
	AffectedComponentIDs []string   `json:"affected_component_ids"`
	PublishedAt          *time.Time `json:"published_at,omitempty"`
	ResolvedAt           *time.Time `json:"resolved_at,omitempty"`
	ScheduledStartAt     *time.Time `json:"scheduled_start_at,omitempty"`
	ScheduledEndAt       *time.Time `json:"scheduled_end_at,omitempty"`
}

type StatusPagePublishValidationResponse struct {
	Errors   []string `json:"errors"`
	Warnings []string `json:"warnings"`
}

type StatusPageIncidentComponentSuggestionMatchResponse struct {
	ResourceType string `json:"resource_type"`
	MatchReason  string `json:"match_reason"`
}

type StatusPageIncidentComponentSuggestionResponse struct {
	ComponentID   string                                               `json:"component_id"`
	ComponentName string                                               `json:"component_name"`
	Matches       []StatusPageIncidentComponentSuggestionMatchResponse `json:"matches"`
}

func (s *Server) registerStatusPageAdminRoutes(frontend *gin.RouterGroup) {
	frontend.GET("/status-pages", s.listStatusPages)
	frontend.POST("/status-pages", s.createStatusPage)
	frontend.GET("/status-pages/:id", s.getStatusPage)
	frontend.PUT("/status-pages/:id", s.updateStatusPage)
	frontend.POST("/status-pages/:id/publish", s.publishStatusPage)
	frontend.POST("/status-pages/:id/unpublish", s.unpublishStatusPage)
	frontend.GET("/status-pages/:id/preview", s.previewStatusPage)
	frontend.GET("/status-pages/:id/sections", s.listStatusPageSections)
	frontend.POST("/status-pages/:id/sections", s.createStatusPageSection)
	frontend.PUT("/status-pages/:id/sections/:section_id", s.updateStatusPageSection)
	frontend.GET("/status-pages/:id/components", s.listStatusPageComponents)
	frontend.POST("/status-pages/:id/components", s.createStatusPageComponent)
	frontend.PUT("/status-pages/:id/components/:component_id", s.updateStatusPageComponent)
	frontend.GET("/status-pages/:id/components/:component_id/mappings", s.listStatusPageComponentMappings)
	frontend.POST("/status-pages/:id/components/:component_id/mappings", s.createStatusPageComponentMapping)
	frontend.PUT("/status-pages/:id/components/:component_id/mappings/:mapping_id", s.updateStatusPageComponentMapping)
	frontend.GET("/status-pages/:id/incidents", s.listStatusPageIncidents)
	frontend.GET("/status-pages/:id/incidents/suggestions", s.suggestStatusPageIncidentComponents)
	frontend.POST("/status-pages/:id/incidents", s.createStatusPageIncident)
	frontend.PUT("/status-pages/:id/incidents/:incident_id", s.updateStatusPageIncident)
	frontend.POST("/status-pages/:id/incidents/:incident_id/updates", s.createStatusPageIncidentUpdate)
	frontend.GET("/status-pages/:id/incidents/draft", s.previewStatusPageIncidentDraft)
	frontend.POST("/status-pages/:id/incidents/draft", s.createStatusPageIncidentDraft)
}

type statusPageIncidentDraftRequest struct {
	InternalIncidentID   string   `json:"internal_incident_id"`
	AffectedComponentIDs []string `json:"affected_component_ids"`
}

type StatusPageIncidentDraftResponse struct {
	Title                string                                          `json:"title"`
	PublicStatus         string                                          `json:"public_status"`
	Severity             string                                          `json:"severity"`
	ImpactSummary        string                                          `json:"impact_summary"`
	InitialUpdateMessage string                                          `json:"initial_update_message"`
	AffectedComponentIDs []string                                        `json:"affected_component_ids"`
	Suggestions          []StatusPageIncidentComponentSuggestionResponse `json:"suggestions"`
}

// previewStatusPageIncidentDraft previews a safe public incident draft from an internal incident.
// @Summary      Preview public incident draft
// @Description  Generate safe public incident draft copy from an internal incident using visible public component names and approved templates only
// @Tags         status-pages
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           previewStatusPageIncidentDraft
// @Param        id           path      string    true   "Status page ID"
// @Param        incident_id  query     string    true   "Internal incident ID"
// @Param        component_id query     []string  false  "Visible public component IDs to include"
// @Success      200          {object}  utils.APIResponse{data=object{draft=StatusPageIncidentDraftResponse}}
// @Failure      400          {object}  utils.APIResponse
// @Failure      404          {object}  utils.APIResponse
// @Failure      500          {object}  utils.APIResponse
// @Router       /v1/status-pages/{id}/incidents/draft [get]
func (s *Server) previewStatusPageIncidentDraft(c *gin.Context) {
	pageID := c.Param("id")
	if !s.statusPageExists(c, pageID) {
		return
	}

	incidentID := strings.TrimSpace(c.Query("incident_id"))
	if incidentID == "" {
		utils.BadRequest(c, "incident_id is required")
		return
	}

	var incident db.Incident
	if err := s.db.Where("id = ?", incidentID).First(&incident).Error; err != nil {
		writeStatusPageLoadError(c, err, "Incident not found")
		return
	}

	draft, err := s.statusPageIncidentDraft(pageID, incident, c.QueryArray("component_id"))
	if err != nil {
		writeStatusPageDraftError(c, err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Status page incident draft generated successfully", gin.H{
		"draft": draft,
	})
}

// createStatusPageIncidentDraft creates a safe public incident draft from an internal incident.
// @Summary      Create public incident draft
// @Description  Create a draft public incident and draft initial update from an internal incident without copying internal details
// @Tags         status-pages
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           createStatusPageIncidentDraft
// @Param        id       path      string                           true  "Status page ID"
// @Param        request  body      statusPageIncidentDraftRequest  true  "Draft source payload"
// @Success      201      {object}  utils.APIResponse{data=object{draft=StatusPageIncidentDraftResponse,incident=StatusPageIncidentResponse,update=StatusPageIncidentUpdateResponse}}
// @Failure      400      {object}  utils.APIResponse
// @Failure      404      {object}  utils.APIResponse
// @Failure      500      {object}  utils.APIResponse
// @Router       /v1/status-pages/{id}/incidents/draft [post]
func (s *Server) createStatusPageIncidentDraft(c *gin.Context) {
	pageID := c.Param("id")
	if !s.statusPageExists(c, pageID) {
		return
	}

	var request statusPageIncidentDraftRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.BadRequest(c, "Invalid status page incident draft payload")
		return
	}
	incidentID := strings.TrimSpace(request.InternalIncidentID)
	if incidentID == "" {
		utils.BadRequest(c, "internal_incident_id is required")
		return
	}

	var internalIncident db.Incident
	if err := s.db.Where("id = ?", incidentID).First(&internalIncident).Error; err != nil {
		writeStatusPageLoadError(c, err, "Incident not found")
		return
	}

	draft, err := s.statusPageIncidentDraft(pageID, internalIncident, request.AffectedComponentIDs)
	if err != nil {
		writeStatusPageDraftError(c, err)
		return
	}

	publicIncident := db.StatusPageIncident{
		ID:                   utils.GenerateID("status_page_incident"),
		StatusPageID:         pageID,
		InternalIncidentID:   internalIncident.ID,
		Title:                draft.Title,
		PublicStatus:         draft.PublicStatus,
		Severity:             draft.Severity,
		ImpactSummary:        draft.ImpactSummary,
		Visibility:           statusPageIncidentVisibilityDraft,
		AffectedComponentIDs: encodeStringList(draft.AffectedComponentIDs),
	}
	update := db.StatusPageIncidentUpdate{
		ID:         utils.GenerateID("status_page_incident_update"),
		IncidentID: publicIncident.ID,
		Status:     draft.PublicStatus,
		Message:    draft.InitialUpdateMessage,
		CreatedBy:  "orion",
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&publicIncident).Error; err != nil {
			return err
		}
		if err := tx.Create(&update).Error; err != nil {
			return err
		}
		if err := s.recordStatusPageAuditEvent(tx, c, service.StatusPageAuditEventInput{
			Action:             service.StatusPageAuditActionPublicIncidentCreated,
			StatusPageID:       publicIncident.StatusPageID,
			AffectedObjectType: "public_incident",
			AffectedObjectID:   publicIncident.ID,
		}); err != nil {
			return err
		}
		return s.recordStatusPageAuditEvent(tx, c, service.StatusPageAuditEventInput{
			Action:             service.StatusPageAuditActionPublicIncidentUpdateCreated,
			StatusPageID:       publicIncident.StatusPageID,
			AffectedObjectType: "public_incident_update",
			AffectedObjectID:   update.ID,
		})
	}); err != nil {
		s.logger.Error("Failed to create status page incident draft", "status_page_id", pageID, "incident_id", internalIncident.ID, "error", err)
		utils.InternalError(c, "Failed to create status page incident draft", err)
		return
	}

	utils.SuccessResponse(c, http.StatusCreated, "Status page incident draft created successfully", gin.H{
		"draft":    draft,
		"incident": statusPageIncidentResponse(publicIncident, []StatusPageIncidentUpdateResponse{statusPageIncidentUpdateResponse(update)}),
		"update":   statusPageIncidentUpdateResponse(update),
	})
}

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

	return StatusPageIncidentDraftResponse{
		Title:                publicIncidentDraftTitle(componentLabel),
		PublicStatus:         publicStatus,
		Severity:             severity,
		ImpactSummary:        publicIncidentDraftImpactSummary(componentLabel),
		InitialUpdateMessage: publicIncidentDraftUpdateMessage(componentLabel),
		AffectedComponentIDs: componentIDs,
		Suggestions:          suggestions,
	}, nil
}

func (s *Server) visibleStatusPageComponentNames(pageID string, componentIDs []string) ([]string, error) {
	if len(componentIDs) == 0 {
		return []string{}, nil
	}

	var components []db.StatusPageComponent
	if err := s.db.
		Where("status_page_id = ? AND visible = ? AND id IN ?", pageID, true, componentIDs).
		Find(&components).Error; err != nil {
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

