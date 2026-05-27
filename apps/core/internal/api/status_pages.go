package api

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"orion/core/internal/utils"
	"sort"
	"strings"
	"time"

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
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Visibility  string `json:"visibility"`
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
}

// listStatusPages retrieves status page drafts and publications.
// @Summary      List status pages
// @Description  Get status page publication configurations ordered by title
// @Tags         status-pages
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           listStatusPages
// @Success      200  {object}  utils.APIResponse{data=object{pages=[]StatusPageResponse,count=int}}
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/status-pages [get]
func (s *Server) listStatusPages(c *gin.Context) {
	var pages []db.StatusPage
	if err := s.db.Order("title ASC").Find(&pages).Error; err != nil {
		s.logger.Error("Failed to list status pages", "error", err)
		utils.InternalError(c, "Failed to list status pages", err)
		return
	}

	responses := make([]StatusPageResponse, 0, len(pages))
	for _, page := range pages {
		responses = append(responses, statusPageResponse(page))
	}
	utils.SuccessResponse(c, http.StatusOK, "Status pages retrieved successfully", gin.H{
		"pages": responses,
		"count": len(responses),
	})
}

// createStatusPage creates a status page draft.
// @Summary      Create status page
// @Description  Create a draft status page publication configuration
// @Tags         status-pages
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           createStatusPage
// @Param        request  body      statusPageRequest  true  "Status page payload"
// @Success      201      {object}  utils.APIResponse{data=object{page=StatusPageResponse}}
// @Failure      400      {object}  utils.APIResponse
// @Failure      409      {object}  utils.APIResponse
// @Failure      500      {object}  utils.APIResponse
// @Router       /v1/status-pages [post]
func (s *Server) createStatusPage(c *gin.Context) {
	var request statusPageRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.BadRequest(c, "Invalid status page payload")
		return
	}

	page := db.StatusPage{
		ID:                        utils.GenerateID("status_page"),
		Visibility:                statusPageVisibilityDraft,
		ThemeSettings:             "{}",
		DefaultIncidentVisibility: statusPageIncidentVisibilityDraft,
	}
	if err := s.applyStatusPageRequest(&page, request, true); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := s.db.Create(&page).Error; err != nil {
		writeStatusPageCreateError(c, err, "Status page slug already exists")
		return
	}

	utils.SuccessResponse(c, http.StatusCreated, "Status page created successfully", gin.H{
		"page": statusPageResponse(page),
	})
}

// getStatusPage retrieves a status page with nested admin configuration.
// @Summary      Get status page
// @Description  Get a status page with sections, components, mappings, incidents, and incident updates
// @Tags         status-pages
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           getStatusPage
// @Param        id   path      string  true  "Status page ID"
// @Success      200  {object}  utils.APIResponse{data=StatusPageDetailResponse}
// @Failure      404  {object}  utils.APIResponse
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/status-pages/{id} [get]
func (s *Server) getStatusPage(c *gin.Context) {
	detail, err := s.loadStatusPageDetail(c.Param("id"))
	if err != nil {
		writeStatusPageLoadError(c, err, "Failed to load status page")
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Status page retrieved successfully", detail)
}

// updateStatusPage updates page-level status page settings.
// @Summary      Update status page
// @Description  Update page-level status page publication settings
// @Tags         status-pages
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           updateStatusPage
// @Param        id       path      string             true  "Status page ID"
// @Param        request  body      statusPageRequest  true  "Status page payload"
// @Success      200      {object}  utils.APIResponse{data=object{page=StatusPageResponse}}
// @Failure      400      {object}  utils.APIResponse
// @Failure      404      {object}  utils.APIResponse
// @Failure      409      {object}  utils.APIResponse
// @Failure      500      {object}  utils.APIResponse
// @Router       /v1/status-pages/{id} [put]
func (s *Server) updateStatusPage(c *gin.Context) {
	var page db.StatusPage
	if err := s.db.Where("id = ?", c.Param("id")).First(&page).Error; err != nil {
		writeStatusPageLoadError(c, err, "Failed to load status page")
		return
	}
	var request statusPageRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.BadRequest(c, "Invalid status page payload")
		return
	}
	if err := s.applyStatusPageRequest(&page, request, false); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := s.db.Save(&page).Error; err != nil {
		writeStatusPageCreateError(c, err, "Status page slug already exists")
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Status page updated successfully", gin.H{
		"page": statusPageResponse(page),
	})
}

// publishStatusPage publishes a status page.
// @Summary      Publish status page
// @Description  Mark a status page as public and set published_at
// @Tags         status-pages
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           publishStatusPage
// @Param        id   path      string  true  "Status page ID"
// @Success      200  {object}  utils.APIResponse{data=object{page=StatusPageResponse}}
// @Failure      404  {object}  utils.APIResponse
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/status-pages/{id}/publish [post]
func (s *Server) publishStatusPage(c *gin.Context) {
	var page db.StatusPage
	if err := s.db.Where("id = ?", c.Param("id")).First(&page).Error; err != nil {
		writeStatusPageLoadError(c, err, "Failed to load status page")
		return
	}
	validation, err := s.validateStatusPageForPublish(page.ID)
	if err != nil {
		s.logger.Error("Failed to validate status page before publish", "status_page_id", page.ID, "error", err)
		utils.InternalError(c, "Failed to validate status page before publish", err)
		return
	}
	if len(validation.Errors) > 0 {
		c.JSON(http.StatusBadRequest, utils.APIResponse{
			Success: false,
			Message: "Status page is not ready to publish",
			Data: gin.H{
				"validation": validation,
			},
		})
		return
	}
	page.Visibility = statusPageVisibilityPublic
	now := time.Now().UTC()
	page.PublishedAt = &now
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&page).Error; err != nil {
			return err
		}
		return s.recordStatusPageAuditEvent(tx, c, service.StatusPageAuditEventInput{
			Action:             service.StatusPageAuditActionPublished,
			StatusPageID:       page.ID,
			AffectedObjectType: "status_page",
			AffectedObjectID:   page.ID,
		})
	}); err != nil {
		s.logger.Error("Failed to publish status page", "status_page_id", page.ID, "error", err)
		utils.InternalError(c, "Failed to publish status page", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Status page published successfully", gin.H{
		"page":       statusPageResponse(page),
		"validation": validation,
	})
}

// unpublishStatusPage unpublishes a status page.
// @Summary      Unpublish status page
// @Description  Return a status page to draft visibility while retaining published_at history
// @Tags         status-pages
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           unpublishStatusPage
// @Param        id   path      string  true  "Status page ID"
// @Success      200  {object}  utils.APIResponse{data=object{page=StatusPageResponse}}
// @Failure      404  {object}  utils.APIResponse
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/status-pages/{id}/unpublish [post]
func (s *Server) unpublishStatusPage(c *gin.Context) {
	s.setStatusPageVisibility(c, statusPageVisibilityDraft, false, "Status page unpublished successfully")
}

// previewStatusPage returns a public-safe draft preview.
// @Summary      Preview status page
// @Description  Preview the public-safe projection of a status page draft or publication
// @Tags         status-pages
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           previewStatusPage
// @Param        id   path      string  true  "Status page ID"
// @Success      200  {object}  utils.APIResponse{data=object{preview=StatusPagePreviewResponse}}
// @Failure      404  {object}  utils.APIResponse
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/status-pages/{id}/preview [get]
func (s *Server) previewStatusPage(c *gin.Context) {
	detail, err := s.loadStatusPageDetail(c.Param("id"))
	if err != nil {
		writeStatusPageLoadError(c, err, "Failed to load status page preview")
		return
	}
	preview := s.statusPagePreview(detail, true)
	utils.SuccessResponse(c, http.StatusOK, "Status page preview retrieved successfully", gin.H{
		"preview": preview,
	})
}

// getPublicStatusPage returns the published public status page projection.
// @Summary      Get public status page
// @Description  Get the public-safe projection of a published or unlisted status page
// @Tags         public-status
// @Accept       json
// @Produce      json
// @ID           getPublicStatusPage
// @Param        slug  path      string  true  "Status page slug"
// @Success      200   {object}  utils.APIResponse{data=object{status_page=StatusPagePreviewResponse}}
// @Failure      404   {object}  utils.APIResponse
// @Failure      500   {object}  utils.APIResponse
// @Router       /status/{slug} [get]
func (s *Server) getPublicStatusPage(c *gin.Context) {
	preview, ok := s.loadPublicStatusPageProjection(c, c.Param("slug"))
	if !ok {
		return
	}
	s.writePublicStatusPageJSON(c, http.StatusOK, "Status page retrieved successfully", gin.H{
		"status_page": preview,
	})
}

func (s *Server) getCustomDomainStatusPage(c *gin.Context) {
	if !s.requestHostHasCustomStatusPage(c) {
		s.serveConsole(c)
		return
	}
	s.getPublicStatusPage(c)
}

// listPublicStatusPageIncidents returns published public incidents for a status page.
// @Summary      List public status page incidents
// @Description  Get published public incidents for a status page
// @Tags         public-status
// @Accept       json
// @Produce      json
// @ID           listPublicStatusPageIncidents
// @Param        slug  path      string  true  "Status page slug"
// @Success      200   {object}  utils.APIResponse{data=object{incidents=[]StatusPagePublicIncidentResponse,count=int}}
// @Failure      404   {object}  utils.APIResponse
// @Failure      500   {object}  utils.APIResponse
// @Router       /status/{slug}/incidents [get]
func (s *Server) listPublicStatusPageIncidents(c *gin.Context) {
	preview, ok := s.loadPublicStatusPageProjection(c, c.Param("slug"))
	if !ok {
		return
	}
	s.writePublicStatusPageJSON(c, http.StatusOK, "Status page incidents retrieved successfully", gin.H{
		"incidents": preview.Incidents,
		"count":     len(preview.Incidents),
	})
}

// getPublicStatusPageIncident returns a published public incident.
// @Summary      Get public status page incident
// @Description  Get a published public incident for a status page
// @Tags         public-status
// @Accept       json
// @Produce      json
// @ID           getPublicStatusPageIncident
// @Param        slug         path      string  true  "Status page slug"
// @Param        incident_id  path      string  true  "Public incident ID"
// @Success      200          {object}  utils.APIResponse{data=object{incident=StatusPagePublicIncidentResponse}}
// @Failure      404          {object}  utils.APIResponse
// @Failure      500          {object}  utils.APIResponse
// @Router       /status/{slug}/incidents/{incident_id} [get]
func (s *Server) getPublicStatusPageIncident(c *gin.Context) {
	preview, ok := s.loadPublicStatusPageProjection(c, c.Param("slug"))
	if !ok {
		return
	}
	for _, incident := range preview.Incidents {
		if incident.ID == c.Param("incident_id") {
			s.writePublicStatusPageJSON(c, http.StatusOK, "Status page incident retrieved successfully", gin.H{
				"incident": incident,
			})
			return
		}
	}
	utils.NotFound(c, "Status page incident not found")
}

// listStatusPageSections lists sections for a status page.
// @Summary      List status page sections
// @Description  Get sections for a status page
// @Tags         status-pages
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           listStatusPageSections
// @Param        id   path      string  true  "Status page ID"
// @Success      200  {object}  utils.APIResponse{data=object{sections=[]StatusPageSectionResponse,count=int}}
// @Failure      404  {object}  utils.APIResponse
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/status-pages/{id}/sections [get]
func (s *Server) listStatusPageSections(c *gin.Context) {
	if !s.statusPageExists(c, c.Param("id")) {
		return
	}
	var sections []db.StatusPageSection
	if err := s.db.Where("status_page_id = ?", c.Param("id")).Order("sort_order ASC, name ASC").Find(&sections).Error; err != nil {
		s.logger.Error("Failed to list status page sections", "error", err)
		utils.InternalError(c, "Failed to list status page sections", err)
		return
	}
	responses := statusPageSectionResponses(sections)
	utils.SuccessResponse(c, http.StatusOK, "Status page sections retrieved successfully", gin.H{
		"sections": responses,
		"count":    len(responses),
	})
}

// createStatusPageSection creates a section.
// @Summary      Create status page section
// @Description  Create a section for grouping public components
// @Tags         status-pages
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           createStatusPageSection
// @Param        id       path      string                    true  "Status page ID"
// @Param        request  body      statusPageSectionRequest  true  "Section payload"
// @Success      201      {object}  utils.APIResponse{data=object{section=StatusPageSectionResponse}}
// @Failure      400      {object}  utils.APIResponse
// @Failure      404      {object}  utils.APIResponse
// @Failure      500      {object}  utils.APIResponse
// @Router       /v1/status-pages/{id}/sections [post]
func (s *Server) createStatusPageSection(c *gin.Context) {
	if !s.statusPageExists(c, c.Param("id")) {
		return
	}
	var request statusPageSectionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.BadRequest(c, "Invalid status page section payload")
		return
	}
	section := db.StatusPageSection{
		ID:           utils.GenerateID("status_page_section"),
		StatusPageID: c.Param("id"),
	}
	if err := applyStatusPageSectionRequest(&section, request, true); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := s.db.Create(&section).Error; err != nil {
		s.logger.Error("Failed to create status page section", "error", err)
		utils.InternalError(c, "Failed to create status page section", err)
		return
	}
	utils.SuccessResponse(c, http.StatusCreated, "Status page section created successfully", gin.H{
		"section": statusPageSectionResponse(section),
	})
}

// updateStatusPageSection updates a section.
// @Summary      Update status page section
// @Description  Update a status page section
// @Tags         status-pages
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           updateStatusPageSection
// @Param        id          path      string                    true  "Status page ID"
// @Param        section_id  path      string                    true  "Section ID"
// @Param        request     body      statusPageSectionRequest  true  "Section payload"
// @Success      200         {object}  utils.APIResponse{data=object{section=StatusPageSectionResponse}}
// @Failure      400         {object}  utils.APIResponse
// @Failure      404         {object}  utils.APIResponse
// @Failure      500         {object}  utils.APIResponse
// @Router       /v1/status-pages/{id}/sections/{section_id} [put]
func (s *Server) updateStatusPageSection(c *gin.Context) {
	var section db.StatusPageSection
	if err := s.db.Where("id = ? AND status_page_id = ?", c.Param("section_id"), c.Param("id")).First(&section).Error; err != nil {
		writeStatusPageLoadError(c, err, "Status page section not found")
		return
	}
	var request statusPageSectionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.BadRequest(c, "Invalid status page section payload")
		return
	}
	if err := applyStatusPageSectionRequest(&section, request, false); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := s.db.Save(&section).Error; err != nil {
		s.logger.Error("Failed to update status page section", "error", err)
		utils.InternalError(c, "Failed to update status page section", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Status page section updated successfully", gin.H{
		"section": statusPageSectionResponse(section),
	})
}

// listStatusPageComponents lists components for a status page.
// @Summary      List status page components
// @Description  Get components and mappings for a status page
// @Tags         status-pages
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           listStatusPageComponents
// @Param        id   path      string  true  "Status page ID"
// @Success      200  {object}  utils.APIResponse{data=object{components=[]StatusPageComponentResponse,count=int}}
// @Failure      404  {object}  utils.APIResponse
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/status-pages/{id}/components [get]
func (s *Server) listStatusPageComponents(c *gin.Context) {
	if !s.statusPageExists(c, c.Param("id")) {
		return
	}
	components, err := s.loadStatusPageComponents(c.Param("id"))
	if err != nil {
		s.logger.Error("Failed to list status page components", "error", err)
		utils.InternalError(c, "Failed to list status page components", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Status page components retrieved successfully", gin.H{
		"components": components,
		"count":      len(components),
	})
}

// createStatusPageComponent creates a component.
// @Summary      Create status page component
// @Description  Create a public component for a status page
// @Tags         status-pages
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           createStatusPageComponent
// @Param        id       path      string                      true  "Status page ID"
// @Param        request  body      statusPageComponentRequest  true  "Component payload"
// @Success      201      {object}  utils.APIResponse{data=object{component=StatusPageComponentResponse}}
// @Failure      400      {object}  utils.APIResponse
// @Failure      404      {object}  utils.APIResponse
// @Failure      500      {object}  utils.APIResponse
// @Router       /v1/status-pages/{id}/components [post]
func (s *Server) createStatusPageComponent(c *gin.Context) {
	if !s.statusPageExists(c, c.Param("id")) {
		return
	}
	var request statusPageComponentRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.BadRequest(c, "Invalid status page component payload")
		return
	}
	component := db.StatusPageComponent{
		ID:           utils.GenerateID("status_page_component"),
		StatusPageID: c.Param("id"),
		DisplayMode:  "single_resource",
		Visible:      true,
	}
	if err := s.applyStatusPageComponentRequest(&component, request, true); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := s.db.Create(&component).Error; err != nil {
		s.logger.Error("Failed to create status page component", "error", err)
		utils.InternalError(c, "Failed to create status page component", err)
		return
	}
	utils.SuccessResponse(c, http.StatusCreated, "Status page component created successfully", gin.H{
		"component": statusPageComponentResponse(component, nil),
	})
}

// updateStatusPageComponent updates a component.
// @Summary      Update status page component
// @Description  Update a public component for a status page
// @Tags         status-pages
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           updateStatusPageComponent
// @Param        id            path      string                      true  "Status page ID"
// @Param        component_id  path      string                      true  "Component ID"
// @Param        request       body      statusPageComponentRequest  true  "Component payload"
// @Success      200           {object}  utils.APIResponse{data=object{component=StatusPageComponentResponse}}
// @Failure      400           {object}  utils.APIResponse
// @Failure      404           {object}  utils.APIResponse
// @Failure      500           {object}  utils.APIResponse
// @Router       /v1/status-pages/{id}/components/{component_id} [put]
func (s *Server) updateStatusPageComponent(c *gin.Context) {
	component, ok := s.loadStatusPageComponentForRequest(c)
	if !ok {
		return
	}
	var request statusPageComponentRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.BadRequest(c, "Invalid status page component payload")
		return
	}
	if err := s.applyStatusPageComponentRequest(&component, request, false); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := s.db.Save(&component).Error; err != nil {
		s.logger.Error("Failed to update status page component", "error", err)
		utils.InternalError(c, "Failed to update status page component", err)
		return
	}
	mappings, err := s.statusPageComponentMappings(component.ID)
	if err != nil {
		s.logger.Error("Failed to load status page component mappings", "error", err)
		utils.InternalError(c, "Failed to update status page component", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Status page component updated successfully", gin.H{
		"component": statusPageComponentResponse(component, mappings),
	})
}

// listStatusPageComponentMappings lists component mappings.
// @Summary      List status page component mappings
// @Description  Get internal resource mappings for a public component
// @Tags         status-pages
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           listStatusPageComponentMappings
// @Param        id            path      string  true  "Status page ID"
// @Param        component_id  path      string  true  "Component ID"
// @Success      200           {object}  utils.APIResponse{data=object{mappings=[]StatusPageComponentMappingResponse,count=int}}
// @Failure      404           {object}  utils.APIResponse
// @Failure      500           {object}  utils.APIResponse
// @Router       /v1/status-pages/{id}/components/{component_id}/mappings [get]
func (s *Server) listStatusPageComponentMappings(c *gin.Context) {
	component, ok := s.loadStatusPageComponentForRequest(c)
	if !ok {
		return
	}
	mappings, err := s.statusPageComponentMappings(component.ID)
	if err != nil {
		s.logger.Error("Failed to list status page component mappings", "error", err)
		utils.InternalError(c, "Failed to list status page component mappings", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Status page component mappings retrieved successfully", gin.H{
		"mappings": mappings,
		"count":    len(mappings),
	})
}

// createStatusPageComponentMapping creates a component mapping.
// @Summary      Create status page component mapping
// @Description  Map a public component to an internal agent or monitor
// @Tags         status-pages
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           createStatusPageComponentMapping
// @Param        id            path      string                            true  "Status page ID"
// @Param        component_id  path      string                            true  "Component ID"
// @Param        request       body      statusPageComponentMappingRequest  true  "Mapping payload"
// @Success      201           {object}  utils.APIResponse{data=object{mapping=StatusPageComponentMappingResponse}}
// @Failure      400           {object}  utils.APIResponse
// @Failure      404           {object}  utils.APIResponse
// @Failure      409           {object}  utils.APIResponse
// @Failure      500           {object}  utils.APIResponse
// @Router       /v1/status-pages/{id}/components/{component_id}/mappings [post]
func (s *Server) createStatusPageComponentMapping(c *gin.Context) {
	component, ok := s.loadStatusPageComponentForRequest(c)
	if !ok {
		return
	}
	var request statusPageComponentMappingRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.BadRequest(c, "Invalid status page component mapping payload")
		return
	}
	mapping := db.StatusPageComponentMapping{
		ID:                   utils.GenerateID("status_page_mapping"),
		ComponentID:          component.ID,
		HealthRollupStrategy: "worst",
		UptimeRollupStrategy: "worst",
	}
	if err := s.applyStatusPageComponentMappingRequest(&mapping, request, true); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&mapping).Error; err != nil {
			return err
		}
		return s.recordStatusPageAuditEvent(tx, c, service.StatusPageAuditEventInput{
			Action:             service.StatusPageAuditActionComponentMappingCreated,
			StatusPageID:       c.Param("id"),
			AffectedObjectType: "component_mapping",
			AffectedObjectID:   mapping.ID,
		})
	}); err != nil {
		writeStatusPageCreateError(c, err, "Status page component mapping already exists")
		return
	}
	utils.SuccessResponse(c, http.StatusCreated, "Status page component mapping created successfully", gin.H{
		"mapping": statusPageComponentMappingResponse(mapping),
	})
}

// updateStatusPageComponentMapping updates a component mapping.
// @Summary      Update status page component mapping
// @Description  Update a public component internal resource mapping
// @Tags         status-pages
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           updateStatusPageComponentMapping
// @Param        id            path      string                            true  "Status page ID"
// @Param        component_id  path      string                            true  "Component ID"
// @Param        mapping_id    path      string                            true  "Mapping ID"
// @Param        request       body      statusPageComponentMappingRequest  true  "Mapping payload"
// @Success      200           {object}  utils.APIResponse{data=object{mapping=StatusPageComponentMappingResponse}}
// @Failure      400           {object}  utils.APIResponse
// @Failure      404           {object}  utils.APIResponse
// @Failure      409           {object}  utils.APIResponse
// @Failure      500           {object}  utils.APIResponse
// @Router       /v1/status-pages/{id}/components/{component_id}/mappings/{mapping_id} [put]
func (s *Server) updateStatusPageComponentMapping(c *gin.Context) {
	component, ok := s.loadStatusPageComponentForRequest(c)
	if !ok {
		return
	}
	var mapping db.StatusPageComponentMapping
	if err := s.db.Where("id = ? AND component_id = ?", c.Param("mapping_id"), component.ID).First(&mapping).Error; err != nil {
		writeStatusPageLoadError(c, err, "Status page component mapping not found")
		return
	}
	var request statusPageComponentMappingRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.BadRequest(c, "Invalid status page component mapping payload")
		return
	}
	if err := s.applyStatusPageComponentMappingRequest(&mapping, request, false); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&mapping).Error; err != nil {
			return err
		}
		return s.recordStatusPageAuditEvent(tx, c, service.StatusPageAuditEventInput{
			Action:             service.StatusPageAuditActionComponentMappingUpdated,
			StatusPageID:       c.Param("id"),
			AffectedObjectType: "component_mapping",
			AffectedObjectID:   mapping.ID,
		})
	}); err != nil {
		writeStatusPageCreateError(c, err, "Status page component mapping already exists")
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Status page component mapping updated successfully", gin.H{
		"mapping": statusPageComponentMappingResponse(mapping),
	})
}

// listStatusPageIncidents lists manual public incidents.
// @Summary      List status page incidents
// @Description  Get manual public incident records for a status page
// @Tags         status-pages
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           listStatusPageIncidents
// @Param        id   path      string  true  "Status page ID"
// @Success      200  {object}  utils.APIResponse{data=object{incidents=[]StatusPageIncidentResponse,count=int}}
// @Failure      404  {object}  utils.APIResponse
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/status-pages/{id}/incidents [get]
func (s *Server) listStatusPageIncidents(c *gin.Context) {
	if !s.statusPageExists(c, c.Param("id")) {
		return
	}
	incidents, err := s.loadStatusPageIncidents(c.Param("id"))
	if err != nil {
		s.logger.Error("Failed to list status page incidents", "error", err)
		utils.InternalError(c, "Failed to list status page incidents", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Status page incidents retrieved successfully", gin.H{
		"incidents": incidents,
		"count":     len(incidents),
	})
}

// suggestStatusPageIncidentComponents suggests public components affected by an internal incident.
// @Summary      Suggest status page incident components
// @Description  Suggest public status page components mapped to the internal incident monitor or agent without exposing internal incident details
// @Tags         status-pages
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           suggestStatusPageIncidentComponents
// @Param        id           path      string  true  "Status page ID"
// @Param        incident_id  query     string  true  "Internal incident ID"
// @Success      200          {object}  utils.APIResponse{data=object{suggestions=[]StatusPageIncidentComponentSuggestionResponse,count=int}}
// @Failure      400          {object}  utils.APIResponse
// @Failure      404          {object}  utils.APIResponse
// @Failure      500          {object}  utils.APIResponse
// @Router       /v1/status-pages/{id}/incidents/suggestions [get]
func (s *Server) suggestStatusPageIncidentComponents(c *gin.Context) {
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

	suggestions, err := s.statusPageIncidentComponentSuggestions(pageID, incident)
	if err != nil {
		s.logger.Error("Failed to suggest status page incident components", "status_page_id", pageID, "incident_id", incidentID, "error", err)
		utils.InternalError(c, "Failed to suggest status page incident components", err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Status page incident component suggestions retrieved successfully", gin.H{
		"suggestions": suggestions,
		"count":       len(suggestions),
	})
}

// createStatusPageIncident creates a manual public incident.
// @Summary      Create status page incident
// @Description  Create a manual public incident for a status page
// @Tags         status-pages
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           createStatusPageIncident
// @Param        id       path      string                     true  "Status page ID"
// @Param        request  body      statusPageIncidentRequest  true  "Incident payload"
// @Success      201      {object}  utils.APIResponse{data=object{incident=StatusPageIncidentResponse}}
// @Failure      400      {object}  utils.APIResponse
// @Failure      404      {object}  utils.APIResponse
// @Failure      500      {object}  utils.APIResponse
// @Router       /v1/status-pages/{id}/incidents [post]
func (s *Server) createStatusPageIncident(c *gin.Context) {
	if !s.statusPageExists(c, c.Param("id")) {
		return
	}
	var request statusPageIncidentRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.BadRequest(c, "Invalid status page incident payload")
		return
	}
	incident := db.StatusPageIncident{
		ID:                   utils.GenerateID("status_page_incident"),
		StatusPageID:         c.Param("id"),
		PublicStatus:         "investigating",
		Severity:             "medium",
		Visibility:           statusPageIncidentVisibilityDraft,
		AffectedComponentIDs: "[]",
	}
	if err := s.applyStatusPageIncidentRequest(&incident, request, true); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&incident).Error; err != nil {
			return err
		}
		return s.recordStatusPageAuditEvent(tx, c, service.StatusPageAuditEventInput{
			Action:             service.StatusPageAuditActionPublicIncidentCreated,
			StatusPageID:       incident.StatusPageID,
			AffectedObjectType: "public_incident",
			AffectedObjectID:   incident.ID,
		})
	}); err != nil {
		s.logger.Error("Failed to create status page incident", "status_page_id", incident.StatusPageID, "error", err)
		utils.InternalError(c, "Failed to create status page incident", err)
		return
	}
	utils.SuccessResponse(c, http.StatusCreated, "Status page incident created successfully", gin.H{
		"incident": statusPageIncidentResponse(incident, nil),
	})
}

// updateStatusPageIncident updates a manual public incident.
// @Summary      Update status page incident
// @Description  Update a manual public incident for a status page
// @Tags         status-pages
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           updateStatusPageIncident
// @Param        id           path      string                     true  "Status page ID"
// @Param        incident_id  path      string                     true  "Incident ID"
// @Param        request      body      statusPageIncidentRequest  true  "Incident payload"
// @Success      200          {object}  utils.APIResponse{data=object{incident=StatusPageIncidentResponse}}
// @Failure      400          {object}  utils.APIResponse
// @Failure      404          {object}  utils.APIResponse
// @Failure      500          {object}  utils.APIResponse
// @Router       /v1/status-pages/{id}/incidents/{incident_id} [put]
func (s *Server) updateStatusPageIncident(c *gin.Context) {
	var incident db.StatusPageIncident
	if err := s.db.Where("id = ? AND status_page_id = ?", c.Param("incident_id"), c.Param("id")).First(&incident).Error; err != nil {
		writeStatusPageLoadError(c, err, "Status page incident not found")
		return
	}
	var request statusPageIncidentRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.BadRequest(c, "Invalid status page incident payload")
		return
	}
	wasResolved := incident.PublicStatus == "resolved" || incident.ResolvedAt != nil
	if err := s.applyStatusPageIncidentRequest(&incident, request, false); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&incident).Error; err != nil {
			return err
		}
		if err := s.recordStatusPageAuditEvent(tx, c, service.StatusPageAuditEventInput{
			Action:             service.StatusPageAuditActionPublicIncidentUpdated,
			StatusPageID:       incident.StatusPageID,
			AffectedObjectType: "public_incident",
			AffectedObjectID:   incident.ID,
		}); err != nil {
			return err
		}
		if !wasResolved && (incident.PublicStatus == "resolved" || incident.ResolvedAt != nil) {
			return s.recordStatusPageAuditEvent(tx, c, service.StatusPageAuditEventInput{
				Action:             service.StatusPageAuditActionPublicIncidentResolved,
				StatusPageID:       incident.StatusPageID,
				AffectedObjectType: "public_incident",
				AffectedObjectID:   incident.ID,
			})
		}
		return nil
	}); err != nil {
		s.logger.Error("Failed to update status page incident", "status_page_id", incident.StatusPageID, "incident_id", incident.ID, "error", err)
		utils.InternalError(c, "Failed to update status page incident", err)
		return
	}
	updates, err := s.statusPageIncidentUpdates(incident.ID)
	if err != nil {
		s.logger.Error("Failed to load status page incident updates", "error", err)
		utils.InternalError(c, "Failed to update status page incident", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Status page incident updated successfully", gin.H{
		"incident": statusPageIncidentResponse(incident, updates),
	})
}

// createStatusPageIncidentUpdate creates a public incident update.
// @Summary      Create status page incident update
// @Description  Create a public timeline update for a manual status page incident
// @Tags         status-pages
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           createStatusPageIncidentUpdate
// @Param        id           path      string                           true  "Status page ID"
// @Param        incident_id  path      string                           true  "Incident ID"
// @Param        request      body      statusPageIncidentUpdateRequest  true  "Incident update payload"
// @Success      201          {object}  utils.APIResponse{data=object{update=StatusPageIncidentUpdateResponse,incident=StatusPageIncidentResponse}}
// @Failure      400          {object}  utils.APIResponse
// @Failure      404          {object}  utils.APIResponse
// @Failure      500          {object}  utils.APIResponse
// @Router       /v1/status-pages/{id}/incidents/{incident_id}/updates [post]
func (s *Server) createStatusPageIncidentUpdate(c *gin.Context) {
	var incident db.StatusPageIncident
	if err := s.db.Where("id = ? AND status_page_id = ?", c.Param("incident_id"), c.Param("id")).First(&incident).Error; err != nil {
		writeStatusPageLoadError(c, err, "Status page incident not found")
		return
	}
	var request statusPageIncidentUpdateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.BadRequest(c, "Invalid status page incident update payload")
		return
	}
	update := db.StatusPageIncidentUpdate{
		ID:         utils.GenerateID("status_page_update"),
		IncidentID: incident.ID,
	}
	if err := applyStatusPageIncidentUpdateRequest(&update, request, true); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	wasResolved := incident.PublicStatus == "resolved" || incident.ResolvedAt != nil
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&update).Error; err != nil {
			return err
		}
		incident.PublicStatus = update.Status
		if update.PublishedAt != nil && incident.Visibility == statusPageIncidentVisibilityDraft {
			incident.Visibility = statusPageIncidentVisibilityPublished
			incident.PublishedAt = update.PublishedAt
		}
		if update.Status == "resolved" && incident.ResolvedAt == nil {
			now := time.Now().UTC()
			incident.ResolvedAt = &now
		}
		if err := tx.Save(&incident).Error; err != nil {
			return err
		}
		if err := s.recordStatusPageAuditEvent(tx, c, service.StatusPageAuditEventInput{
			Action:             service.StatusPageAuditActionPublicIncidentUpdateCreated,
			StatusPageID:       incident.StatusPageID,
			AffectedObjectType: "public_incident_update",
			AffectedObjectID:   update.ID,
		}); err != nil {
			return err
		}
		if err := s.enqueueStatusPageSubscriberIncidentUpdateDeliveries(tx, incident, update); err != nil {
			return err
		}
		if !wasResolved && (incident.PublicStatus == "resolved" || incident.ResolvedAt != nil) {
			return s.recordStatusPageAuditEvent(tx, c, service.StatusPageAuditEventInput{
				Action:             service.StatusPageAuditActionPublicIncidentResolved,
				StatusPageID:       incident.StatusPageID,
				AffectedObjectType: "public_incident",
				AffectedObjectID:   incident.ID,
			})
		}
		return nil
	}); err != nil {
		s.logger.Error("Failed to create status page incident update", "error", err)
		utils.InternalError(c, "Failed to create status page incident update", err)
		return
	}
	utils.SuccessResponse(c, http.StatusCreated, "Status page incident update created successfully", gin.H{
		"update":   statusPageIncidentUpdateResponse(update),
		"incident": statusPageIncidentResponse(incident, []StatusPageIncidentUpdateResponse{statusPageIncidentUpdateResponse(update)}),
	})
}

func (s *Server) setStatusPageVisibility(c *gin.Context, visibility string, stampPublishedAt bool, message string) {
	var page db.StatusPage
	if err := s.db.Where("id = ?", c.Param("id")).First(&page).Error; err != nil {
		writeStatusPageLoadError(c, err, "Failed to load status page")
		return
	}
	page.Visibility = visibility
	if stampPublishedAt {
		now := time.Now().UTC()
		page.PublishedAt = &now
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&page).Error; err != nil {
			return err
		}
		action := service.StatusPageAuditActionUnpublished
		if visibility == statusPageVisibilityPublic || visibility == statusPageVisibilityUnlisted {
			action = service.StatusPageAuditActionPublished
		}
		return s.recordStatusPageAuditEvent(tx, c, service.StatusPageAuditEventInput{
			Action:             action,
			StatusPageID:       page.ID,
			AffectedObjectType: "status_page",
			AffectedObjectID:   page.ID,
		})
	}); err != nil {
		s.logger.Error("Failed to update status page visibility", "status_page_id", page.ID, "error", err)
		utils.InternalError(c, "Failed to update status page visibility", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, message, gin.H{"page": statusPageResponse(page)})
}

func (s *Server) loadStatusPageDetail(id string) (StatusPageDetailResponse, error) {
	var page db.StatusPage
	if err := s.db.Where("id = ?", id).First(&page).Error; err != nil {
		return StatusPageDetailResponse{}, err
	}

	var sections []db.StatusPageSection
	if err := s.db.Where("status_page_id = ?", id).Order("sort_order ASC, name ASC").Find(&sections).Error; err != nil {
		return StatusPageDetailResponse{}, err
	}
	components, err := s.loadStatusPageComponents(id)
	if err != nil {
		return StatusPageDetailResponse{}, err
	}
	incidents, err := s.loadStatusPageIncidents(id)
	if err != nil {
		return StatusPageDetailResponse{}, err
	}

	return StatusPageDetailResponse{
		Page:       statusPageResponse(page),
		Sections:   statusPageSectionResponses(sections),
		Components: components,
		Incidents:  incidents,
	}, nil
}

func (s *Server) loadStatusPageComponents(pageID string) ([]StatusPageComponentResponse, error) {
	var components []db.StatusPageComponent
	if err := s.db.Where("status_page_id = ?", pageID).Order("sort_order ASC, public_name ASC").Find(&components).Error; err != nil {
		return nil, err
	}
	componentIDs := make([]string, 0, len(components))
	for _, component := range components {
		componentIDs = append(componentIDs, component.ID)
	}
	mappingsByComponent := map[string][]StatusPageComponentMappingResponse{}
	if len(componentIDs) > 0 {
		var mappings []db.StatusPageComponentMapping
		if err := s.db.Where("component_id IN ?", componentIDs).Order("resource_type ASC, resource_id ASC").Find(&mappings).Error; err != nil {
			return nil, err
		}
		for _, mapping := range mappings {
			mappingsByComponent[mapping.ComponentID] = append(mappingsByComponent[mapping.ComponentID], statusPageComponentMappingResponse(mapping))
		}
	}
	responses := make([]StatusPageComponentResponse, 0, len(components))
	for _, component := range components {
		responses = append(responses, statusPageComponentResponse(component, mappingsByComponent[component.ID]))
	}
	return responses, nil
}

func (s *Server) loadStatusPageIncidents(pageID string) ([]StatusPageIncidentResponse, error) {
	var incidents []db.StatusPageIncident
	if err := s.db.Where("status_page_id = ?", pageID).Order("created_at DESC").Find(&incidents).Error; err != nil {
		return nil, err
	}
	incidentIDs := make([]string, 0, len(incidents))
	for _, incident := range incidents {
		incidentIDs = append(incidentIDs, incident.ID)
	}
	updatesByIncident := map[string][]StatusPageIncidentUpdateResponse{}
	if len(incidentIDs) > 0 {
		var updates []db.StatusPageIncidentUpdate
		if err := s.db.Where("incident_id IN ?", incidentIDs).Order("created_at ASC").Find(&updates).Error; err != nil {
			return nil, err
		}
		for _, update := range updates {
			updatesByIncident[update.IncidentID] = append(updatesByIncident[update.IncidentID], statusPageIncidentUpdateResponse(update))
		}
	}
	responses := make([]StatusPageIncidentResponse, 0, len(incidents))
	for _, incident := range incidents {
		responses = append(responses, statusPageIncidentResponse(incident, updatesByIncident[incident.ID]))
	}
	return responses, nil
}

func (s *Server) statusPageIncidentComponentSuggestions(pageID string, incident db.Incident) ([]StatusPageIncidentComponentSuggestionResponse, error) {
	type suggestionRow struct {
		ComponentID   string
		ComponentName string
		ResourceType  string
	}

	query := s.db.Table("status_page_components AS components").
		Select("components.id AS component_id, components.public_name AS component_name, mappings.resource_type AS resource_type").
		Joins("JOIN status_page_component_mappings AS mappings ON mappings.component_id = components.id").
		Where("components.status_page_id = ? AND components.visible = ?", pageID, true)

	matchClauses := make([]string, 0, 2)
	matchArgs := make([]interface{}, 0, 4)
	if incident.MonitorID != "" {
		matchClauses = append(matchClauses, "(mappings.resource_type = ? AND mappings.resource_id = ?)")
		matchArgs = append(matchArgs, "monitor", incident.MonitorID)
	}
	if incident.AgentID != "" {
		matchClauses = append(matchClauses, "(mappings.resource_type = ? AND mappings.resource_id = ?)")
		matchArgs = append(matchArgs, "agent", incident.AgentID)
	}
	if len(matchClauses) == 0 {
		return []StatusPageIncidentComponentSuggestionResponse{}, nil
	}

	var rows []suggestionRow
	if err := query.
		Where(strings.Join(matchClauses, " OR "), matchArgs...).
		Order("components.sort_order ASC, components.public_name ASC, mappings.resource_type ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	suggestionsByComponent := map[string]*StatusPageIncidentComponentSuggestionResponse{}
	order := make([]string, 0, len(rows))
	for _, row := range rows {
		if _, ok := suggestionsByComponent[row.ComponentID]; !ok {
			suggestionsByComponent[row.ComponentID] = &StatusPageIncidentComponentSuggestionResponse{
				ComponentID:   row.ComponentID,
				ComponentName: row.ComponentName,
				Matches:       []StatusPageIncidentComponentSuggestionMatchResponse{},
			}
			order = append(order, row.ComponentID)
		}
		suggestion := suggestionsByComponent[row.ComponentID]
		suggestion.Matches = append(suggestion.Matches, StatusPageIncidentComponentSuggestionMatchResponse{
			ResourceType: row.ResourceType,
			MatchReason:  statusPageIncidentSuggestionMatchReason(row.ResourceType),
		})
	}

	suggestions := make([]StatusPageIncidentComponentSuggestionResponse, 0, len(order))
	for _, componentID := range order {
		suggestions = append(suggestions, *suggestionsByComponent[componentID])
	}
	return suggestions, nil
}

func (s *Server) loadStatusPageComponentForRequest(c *gin.Context) (db.StatusPageComponent, bool) {
	var component db.StatusPageComponent
	if err := s.db.Where("id = ? AND status_page_id = ?", c.Param("component_id"), c.Param("id")).First(&component).Error; err != nil {
		writeStatusPageLoadError(c, err, "Status page component not found")
		return db.StatusPageComponent{}, false
	}
	return component, true
}

func (s *Server) statusPageExists(c *gin.Context, id string) bool {
	var count int64
	if err := s.db.Model(&db.StatusPage{}).Where("id = ?", id).Count(&count).Error; err != nil {
		s.logger.Error("Failed to load status page", "error", err)
		utils.InternalError(c, "Failed to load status page", err)
		return false
	}
	if count == 0 {
		utils.NotFound(c, "Status page not found")
		return false
	}
	return true
}

func (s *Server) statusPageComponentMappings(componentID string) ([]StatusPageComponentMappingResponse, error) {
	var mappings []db.StatusPageComponentMapping
	if err := s.db.Where("component_id = ?", componentID).Order("resource_type ASC, resource_id ASC").Find(&mappings).Error; err != nil {
		return nil, err
	}
	responses := make([]StatusPageComponentMappingResponse, 0, len(mappings))
	for _, mapping := range mappings {
		responses = append(responses, statusPageComponentMappingResponse(mapping))
	}
	return responses, nil
}

func (s *Server) statusPageIncidentUpdates(incidentID string) ([]StatusPageIncidentUpdateResponse, error) {
	var updates []db.StatusPageIncidentUpdate
	if err := s.db.Where("incident_id = ?", incidentID).Order("created_at ASC").Find(&updates).Error; err != nil {
		return nil, err
	}
	responses := make([]StatusPageIncidentUpdateResponse, 0, len(updates))
	for _, update := range updates {
		responses = append(responses, statusPageIncidentUpdateResponse(update))
	}
	return responses, nil
}

func (s *Server) loadPublicStatusPageProjection(c *gin.Context, slug string) (StatusPagePreviewResponse, bool) {
	detail, ok := s.loadPublicStatusPageDetail(c, slug)
	if !ok {
		return StatusPagePreviewResponse{}, false
	}
	return s.statusPagePreview(detail, false), true
}

func (s *Server) loadPublicStatusPageDetail(c *gin.Context, slug string) (StatusPageDetailResponse, bool) {
	page, err := s.loadPublicStatusPageForRequest(c, slug)
	if err != nil {
		writeStatusPageLoadError(c, err, "Status page not found")
		return StatusPageDetailResponse{}, false
	}
	detail, err := s.loadStatusPageDetail(page.ID)
	if err != nil {
		writeStatusPageLoadError(c, err, "Failed to load status page")
		return StatusPageDetailResponse{}, false
	}
	return detail, true
}

func (s *Server) loadPublicStatusPageForRequest(c *gin.Context, slug string) (db.StatusPage, error) {
	normalizedSlug := strings.TrimSpace(slug)
	if host, ok := publicStatusPageRequestHost(c); ok {
		var hostPage db.StatusPage
		err := s.db.
			Where("custom_domain = ? AND visibility IN ?", host, []string{statusPageVisibilityPublic, statusPageVisibilityUnlisted}).
			First(&hostPage).Error
		if err == nil {
			if normalizedSlug == "" || normalizedSlug == hostPage.Slug {
				return hostPage, nil
			}
			return db.StatusPage{}, gorm.ErrRecordNotFound
		}
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return db.StatusPage{}, err
		}
	}
	if normalizedSlug == "" {
		return db.StatusPage{}, gorm.ErrRecordNotFound
	}
	var page db.StatusPage
	if err := s.db.
		Where("slug = ? AND visibility IN ?", normalizedSlug, []string{statusPageVisibilityPublic, statusPageVisibilityUnlisted}).
		First(&page).Error; err != nil {
		return db.StatusPage{}, err
	}
	return page, nil
}

func (s *Server) statusPagePreview(detail StatusPageDetailResponse, includeDraftIncidents bool) StatusPagePreviewResponse {
	componentsBySection := map[string][]StatusPagePublicComponentResponse{}
	overallStatus := "operational"
	for _, component := range detail.Components {
		if !component.Visible {
			continue
		}
		componentStatus := s.statusPageComponentStatus(component)
		if statusPageStatusWeight(componentStatus) > statusPageStatusWeight(overallStatus) {
			overallStatus = componentStatus
		}
		componentsBySection[component.SectionID] = append(componentsBySection[component.SectionID], s.statusPagePublicComponentResponse(component, componentStatus))
	}

	sections := make([]StatusPagePublicSectionResponse, 0, len(detail.Sections))
	for _, section := range detail.Sections {
		sections = append(sections, StatusPagePublicSectionResponse{
			ID:                 section.ID,
			Name:               section.Name,
			CollapsedByDefault: section.CollapsedByDefault,
			Components:         componentsBySection[section.ID],
		})
	}

	incidents := make([]StatusPagePublicIncidentResponse, 0, len(detail.Incidents))
	for _, incident := range detail.Incidents {
		if incident.Visibility == statusPageIncidentVisibilityPrivate {
			continue
		}
		if !includeDraftIncidents && (incident.Visibility != statusPageIncidentVisibilityPublished || incident.PublishedAt == nil) {
			continue
		}
		incidents = append(incidents, StatusPagePublicIncidentResponse{
			ID:                   incident.ID,
			Title:                incident.Title,
			PublicStatus:         incident.PublicStatus,
			Severity:             incident.Severity,
			ImpactSummary:        incident.ImpactSummary,
			AffectedComponentIDs: incident.AffectedComponentIDs,
			PublishedAt:          publicMinutePtr(incident.PublishedAt),
			ResolvedAt:           publicMinutePtr(incident.ResolvedAt),
			ScheduledStartAt:     publicMinutePtr(incident.ScheduledStartAt),
			ScheduledEndAt:       publicMinutePtr(incident.ScheduledEndAt),
		})
	}

	return StatusPagePreviewResponse{
		Page: StatusPagePublicPageResponse{
			Slug:        detail.Page.Slug,
			Title:       detail.Page.Title,
			Description: detail.Page.Description,
			Visibility:  detail.Page.Visibility,
		},
		Metadata:             statusPagePublicMetadata(detail.Page),
		Sections:             sections,
		Incidents:            incidents,
		OverallStatus:        overallStatus,
		OverallStatusDisplay: publicStatusDisplay(overallStatus),
		LastUpdated:          publicMinute(time.Now()),
	}
}

func statusPagePublicMetadata(page StatusPageResponse) StatusPagePublicMetadataResponse {
	title := firstNonEmpty(page.SEOTitle, page.Title)
	description := firstNonEmpty(page.SEODescription, page.Description)
	canonicalURL := strings.TrimRight(strings.TrimSpace(page.CanonicalURL), "/")
	openGraphType := statusPageMetadataSetting(page.ThemeSettings, "open_graph_type")
	if openGraphType == "" {
		openGraphType = "website"
	}
	siteName := firstNonEmpty(statusPageMetadataSetting(page.ThemeSettings, "open_graph_site_name"), page.Title)

	return StatusPagePublicMetadataResponse{
		Title:        title,
		Description:  description,
		CanonicalURL: canonicalURL,
		OpenGraph: StatusPagePublicOpenGraphResponse{
			Title:       firstNonEmpty(statusPageMetadataSetting(page.ThemeSettings, "open_graph_title"), title),
			Description: firstNonEmpty(statusPageMetadataSetting(page.ThemeSettings, "open_graph_description"), description),
			URL:         canonicalURL,
			Type:        openGraphType,
			SiteName:    siteName,
			ImageURL:    strings.TrimSpace(page.OpenGraphImageURL),
		},
	}
}

func statusPageMetadataSetting(settings map[string]interface{}, key string) string {
	if settings == nil {
		return ""
	}
	value, ok := settings[key]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func (s *Server) statusPagePublicComponentResponse(component StatusPageComponentResponse, status string) StatusPagePublicComponentResponse {
	return StatusPagePublicComponentResponse{
		ID:            component.ID,
		Name:          component.PublicName,
		Description:   component.PublicDescription,
		Status:        status,
		StatusDisplay: publicStatusDisplay(status),
		StatusReason:  component.ManualStatusReason,
		DisplayMode:   component.DisplayMode,
	}
}

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

func (s *Server) applyStatusPageRequest(page *db.StatusPage, request statusPageRequest, create bool) error {
	if request.Slug != nil {
		page.Slug = normalizeSlug(*request.Slug)
	}
	if request.CustomDomain != nil {
		domain, err := normalizeStatusPageCustomDomain(*request.CustomDomain)
		if err != nil {
			return err
		}
		page.CustomDomain = domain
	}
	if request.Title != nil {
		page.Title = strings.TrimSpace(*request.Title)
	}
	if request.Description != nil {
		page.Description = strings.TrimSpace(*request.Description)
	}
	if request.SEOTitle != nil {
		page.SEOTitle = strings.TrimSpace(*request.SEOTitle)
	}
	if request.SEODescription != nil {
		page.SEODescription = strings.TrimSpace(*request.SEODescription)
	}
	if request.OpenGraphImageURL != nil {
		page.OpenGraphImageURL = strings.TrimSpace(*request.OpenGraphImageURL)
	}
	if request.CanonicalURL != nil {
		page.CanonicalURL = strings.TrimSpace(*request.CanonicalURL)
	}
	if request.Visibility != nil {
		page.Visibility = strings.TrimSpace(*request.Visibility)
	}
	if request.ThemeSettings != nil {
		body, err := json.Marshal(request.ThemeSettings)
		if err != nil {
			return &requestValidationError{message: "theme_settings must be valid JSON"}
		}
		page.ThemeSettings = string(body)
	}
	if request.DefaultIncidentVisibility != nil {
		page.DefaultIncidentVisibility = strings.TrimSpace(*request.DefaultIncidentVisibility)
	}

	if create && page.Visibility == "" {
		page.Visibility = statusPageVisibilityDraft
	}
	if create && page.DefaultIncidentVisibility == "" {
		page.DefaultIncidentVisibility = statusPageIncidentVisibilityDraft
	}
	if page.ThemeSettings == "" {
		page.ThemeSettings = "{}"
	}
	if page.Slug == "" {
		return &requestValidationError{message: "status page slug is required"}
	}
	if page.Title == "" {
		return &requestValidationError{message: "status page title is required"}
	}
	if !validSlug(page.Slug) {
		return &requestValidationError{message: "status page slug may contain only lowercase letters, numbers, and hyphens"}
	}
	if !validStatusPageVisibility(page.Visibility) {
		return &requestValidationError{message: "unsupported status page visibility"}
	}
	if !validStatusPageIncidentVisibility(page.DefaultIncidentVisibility) {
		return &requestValidationError{message: "unsupported default incident visibility"}
	}
	if page.CustomDomain != "" {
		if err := s.ensureStatusPageCustomDomainAvailable(page.ID, page.CustomDomain); err != nil {
			return err
		}
	}
	if err := validateOptionalURL(page.OpenGraphImageURL, "open_graph_image_url"); err != nil {
		return err
	}
	if err := validateOptionalURL(page.CanonicalURL, "canonical_url"); err != nil {
		return err
	}
	return nil
}

func (s *Server) ensureStatusPageCustomDomainAvailable(pageID string, domain string) error {
	var count int64
	query := s.db.Model(&db.StatusPage{}).Where("custom_domain = ?", domain)
	if strings.TrimSpace(pageID) != "" {
		query = query.Where("id <> ?", pageID)
	}
	if err := query.Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return &requestValidationError{message: "status page custom_domain is already in use"}
	}
	return nil
}

func applyStatusPageSectionRequest(section *db.StatusPageSection, request statusPageSectionRequest, create bool) error {
	if request.Name != nil {
		section.Name = strings.TrimSpace(*request.Name)
	}
	if request.SortOrder != nil {
		section.SortOrder = *request.SortOrder
	}
	if request.CollapsedByDefault != nil {
		section.CollapsedByDefault = *request.CollapsedByDefault
	}
	if create && section.Name == "" {
		return &requestValidationError{message: "status page section name is required"}
	}
	if strings.TrimSpace(section.Name) == "" {
		return &requestValidationError{message: "status page section name is required"}
	}
	return nil
}

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

func (s *Server) applyStatusPageIncidentRequest(incident *db.StatusPageIncident, request statusPageIncidentRequest, create bool) error {
	if request.InternalIncidentID != nil {
		incident.InternalIncidentID = strings.TrimSpace(*request.InternalIncidentID)
	}
	if request.Title != nil {
		incident.Title = strings.TrimSpace(*request.Title)
	}
	if request.PublicStatus != nil {
		incident.PublicStatus = strings.TrimSpace(*request.PublicStatus)
	}
	if request.Severity != nil {
		incident.Severity = strings.TrimSpace(*request.Severity)
	}
	if request.ImpactSummary != nil {
		incident.ImpactSummary = strings.TrimSpace(*request.ImpactSummary)
	}
	if request.Visibility != nil {
		incident.Visibility = strings.TrimSpace(*request.Visibility)
	}
	if request.AffectedComponentIDs != nil {
		componentIDs := normalizeStringList(request.AffectedComponentIDs)
		if err := s.ensureStatusPageComponentsExist(incident.StatusPageID, componentIDs); err != nil {
			return err
		}
		incident.AffectedComponentIDs = encodeStringList(componentIDs)
	}
	if request.PublishedAt != nil {
		incident.PublishedAt = request.PublishedAt
	}
	if request.ResolvedAt != nil {
		incident.ResolvedAt = request.ResolvedAt
	}
	if request.ScheduledStartAt != nil {
		incident.ScheduledStartAt = request.ScheduledStartAt
	}
	if request.ScheduledEndAt != nil {
		incident.ScheduledEndAt = request.ScheduledEndAt
	}
	if create && incident.PublicStatus == "" {
		incident.PublicStatus = "investigating"
	}
	if create && incident.Severity == "" {
		incident.Severity = "medium"
	}
	if create && incident.Visibility == "" {
		incident.Visibility = statusPageIncidentVisibilityDraft
	}
	if strings.TrimSpace(incident.Title) == "" {
		return &requestValidationError{message: "status page incident title is required"}
	}
	if !validStatusPageIncidentStatus(incident.PublicStatus) {
		return &requestValidationError{message: "unsupported status page incident public_status"}
	}
	if !validStatusPageSeverity(incident.Severity) {
		return &requestValidationError{message: "unsupported status page incident severity"}
	}
	if !validStatusPageIncidentVisibility(incident.Visibility) {
		return &requestValidationError{message: "unsupported status page incident visibility"}
	}
	if incident.InternalIncidentID != "" {
		var count int64
		if err := s.db.Model(&db.Incident{}).Where("id = ?", incident.InternalIncidentID).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return &requestValidationError{message: "status page incident internal_incident_id must reference an existing incident"}
		}
	}
	return nil
}

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

func (s *Server) recordStatusPageAuditEvent(tx *gorm.DB, c *gin.Context, input service.StatusPageAuditEventInput) error {
	input.ActorType, input.ActorID = statusPageAuditActor(c)
	_, err := service.NewAuditService(tx, s.logger).RecordStatusPageEvent(input)
	return err
}

func statusPageAuditActor(c *gin.Context) (string, string) {
	if actor, ok := c.Get("frontend_actor_id"); ok {
		if actorID, ok := actor.(string); ok && strings.TrimSpace(actorID) != "" {
			return "user", strings.TrimSpace(actorID)
		}
	}
	return "system", "console"
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

func decodeJSONObject(value string) map[string]interface{} {
	result := map[string]interface{}{}
	if strings.TrimSpace(value) == "" {
		return result
	}
	_ = json.Unmarshal([]byte(value), &result)
	return result
}

func normalizeSlug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, "_", "-")
	return value
}

func normalizeStatusPageCustomDomain(value string) (string, error) {
	domain := strings.ToLower(strings.TrimSpace(value))
	domain = strings.TrimSuffix(domain, ".")
	if domain == "" {
		return "", nil
	}
	if strings.Contains(domain, "://") || strings.ContainsAny(domain, "/?#") || strings.HasPrefix(domain, "*.") || strings.Contains(domain, "*") {
		return "", &requestValidationError{message: "status page custom_domain must be a hostname without scheme, path, query, fragment, or wildcard"}
	}
	if host, port, err := net.SplitHostPort(domain); err == nil && strings.TrimSpace(port) != "" {
		domain = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	} else if strings.Contains(errHostCandidate(domain), ":") {
		return "", &requestValidationError{message: "status page custom_domain must not be an IP address or include a path"}
	}
	if domain == "" || domain == "localhost" || strings.HasSuffix(domain, ".localhost") || strings.HasSuffix(domain, ".local") || strings.HasSuffix(domain, ".internal") || strings.HasSuffix(domain, ".lan") {
		return "", &requestValidationError{message: "status page custom_domain must be a public hostname"}
	}
	if ip := net.ParseIP(domain); ip != nil {
		return "", &requestValidationError{message: "status page custom_domain must not be an IP address"}
	}
	if !validStatusPageHostname(domain) {
		return "", &requestValidationError{message: "status page custom_domain must be a valid hostname"}
	}
	return domain, nil
}

func errHostCandidate(value string) string {
	if strings.HasPrefix(value, "[") && strings.Contains(value, "]") {
		return strings.Trim(value, "[]")
	}
	return value
}

func validStatusPageHostname(value string) bool {
	if len(value) > 253 || strings.HasPrefix(value, ".") || strings.HasSuffix(value, ".") || !strings.Contains(value, ".") {
		return false
	}
	labels := strings.Split(value, ".")
	for _, label := range labels {
		if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return false
		}
		for _, r := range label {
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
				continue
			}
			return false
		}
	}
	return true
}

func publicStatusPageRequestHost(c *gin.Context) (string, bool) {
	host, err := normalizePublicStatusPageRequestHost(c.Request.Host)
	if err != nil || host == "" {
		return "", false
	}
	return host, true
}

func normalizePublicStatusPageRequestHost(value string) (string, error) {
	host := strings.ToLower(strings.TrimSpace(value))
	host = strings.TrimSuffix(host, ".")
	if host == "" {
		return "", nil
	}
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(parsedHost)), ".")
	}
	domain, err := normalizeStatusPageCustomDomain(host)
	if err != nil {
		return "", err
	}
	return domain, nil
}

func (s *Server) requestHostHasCustomStatusPage(c *gin.Context) bool {
	host, ok := publicStatusPageRequestHost(c)
	if !ok {
		return false
	}
	var count int64
	if err := s.db.Model(&db.StatusPage{}).
		Where("custom_domain = ? AND visibility IN ?", host, []string{statusPageVisibilityPublic, statusPageVisibilityUnlisted}).
		Count(&count).Error; err != nil {
		s.logger.Error("Failed to check status page custom domain", "host", host, "error", err)
		return false
	}
	return count > 0
}

func validSlug(value string) bool {
	if value == "" || strings.HasPrefix(value, "-") || strings.HasSuffix(value, "-") {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			continue
		}
		return false
	}
	return true
}

func validateOptionalURL(value string, field string) error {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parsed, err := url.ParseRequestURI(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return &requestValidationError{message: field + " must be an absolute URL"}
	}
	return nil
}

func validStatusPageVisibility(value string) bool {
	switch value {
	case statusPageVisibilityDraft, statusPageVisibilityPublic, statusPageVisibilityUnlisted:
		return true
	default:
		return false
	}
}

func validStatusPageIncidentVisibility(value string) bool {
	switch value {
	case statusPageIncidentVisibilityDraft, statusPageIncidentVisibilityPublished, statusPageIncidentVisibilityPrivate:
		return true
	default:
		return false
	}
}

func validStatusPageDisplayMode(value string) bool {
	switch value {
	case "single_resource", "aggregate", "manual":
		return true
	default:
		return false
	}
}

func validStatusPageComponentStatus(value string) bool {
	switch value {
	case "operational", "degraded", "partial_outage", "major_outage", "maintenance", "unknown":
		return true
	default:
		return false
	}
}

func validStatusPageResourceType(value string) bool {
	switch value {
	case "agent", "monitor":
		return true
	default:
		return false
	}
}

func validStatusPageRollupStrategy(value string) bool {
	switch value {
	case "worst", "best", "average", "manual":
		return true
	default:
		return false
	}
}

func validStatusPageIncidentStatus(value string) bool {
	switch value {
	case "investigating", "identified", "monitoring", "resolved", "scheduled":
		return true
	default:
		return false
	}
}

func validStatusPageSeverity(value string) bool {
	switch value {
	case "low", "medium", "high", "critical":
		return true
	default:
		return false
	}
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

func looksLikePrivateStatusPageLabel(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return false
	}
	fields := strings.FieldsFunc(normalized, func(r rune) bool {
		return r == ' ' || r == '/' || r == ':' || r == '[' || r == ']' || r == '(' || r == ')' || r == ','
	})
	for _, field := range fields {
		field = strings.Trim(field, ".")
		if field == "" {
			continue
		}
		if field == "localhost" || strings.HasSuffix(field, ".local") || strings.HasSuffix(field, ".internal") || strings.HasSuffix(field, ".lan") {
			return true
		}
		if ip := net.ParseIP(field); ip != nil {
			return true
		}
	}
	return false
}

func writeStatusPageLoadError(c *gin.Context, err error, message string) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		utils.NotFound(c, strings.TrimPrefix(strings.TrimPrefix(message, "Failed to load "), "load "))
		return
	}
	utils.InternalError(c, message, err)
}

func writeStatusPageCreateError(c *gin.Context, err error, conflictMessage string) {
	errText := strings.ToLower(err.Error())
	if errors.Is(err, gorm.ErrDuplicatedKey) || strings.Contains(errText, "unique") || strings.Contains(errText, "duplicate") {
		utils.ErrorResponse(c, http.StatusConflict, conflictMessage, nil)
		return
	}
	utils.InternalError(c, conflictMessage, err)
}
