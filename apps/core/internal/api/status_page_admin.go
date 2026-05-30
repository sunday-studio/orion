package api

import (
	"encoding/json"
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"orion/core/internal/utils"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

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
		settings, err := sanitizeStatusPageThemeSettings(request.ThemeSettings)
		if err != nil {
			return err
		}
		body, err := json.Marshal(settings)
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

func sanitizeStatusPageThemeSettings(settings map[string]interface{}) (map[string]interface{}, error) {
	sanitized := map[string]interface{}{}
	for key, value := range settings {
		if value == nil {
			continue
		}
		switch key {
		case "accent_color":
			text, err := statusPageThemeString(value, "theme_settings.accent_color", 32)
			if err != nil {
				return nil, err
			}
			if !validStatusPageThemeHexColor(text) {
				return nil, &requestValidationError{message: "theme_settings.accent_color must be a 6-digit hex color"}
			}
			sanitized[key] = strings.ToLower(text)
		case "logo_url":
			text, err := statusPageThemeString(value, "theme_settings.logo_url", 2048)
			if err != nil {
				return nil, err
			}
			if text == "" {
				continue
			}
			if err := validateOptionalURL(text, "theme_settings.logo_url"); err != nil {
				return nil, err
			}
			sanitized[key] = text
		case "logo_alt", "open_graph_title", "open_graph_description", "open_graph_site_name":
			text, err := statusPageThemeString(value, "theme_settings."+key, 280)
			if err != nil {
				return nil, err
			}
			if text != "" {
				sanitized[key] = text
			}
		case "header_style":
			text, err := statusPageThemeString(value, "theme_settings.header_style", 64)
			if err != nil {
				return nil, err
			}
			if text != "standard" && text != "compact" && text != "centered" {
				return nil, &requestValidationError{message: "theme_settings.header_style is unsupported"}
			}
			sanitized[key] = text
		case "component_density":
			text, err := statusPageThemeString(value, "theme_settings.component_density", 64)
			if err != nil {
				return nil, err
			}
			if text != "comfortable" && text != "compact" {
				return nil, &requestValidationError{message: "theme_settings.component_density is unsupported"}
			}
			sanitized[key] = text
		case "show_uptime_summary", "show_incident_history":
			boolean, ok := value.(bool)
			if !ok {
				return nil, &requestValidationError{message: "theme_settings." + key + " must be a boolean"}
			}
			sanitized[key] = boolean
		case "open_graph_type":
			text, err := statusPageThemeString(value, "theme_settings.open_graph_type", 64)
			if err != nil {
				return nil, err
			}
			if text != "website" {
				return nil, &requestValidationError{message: "theme_settings.open_graph_type is unsupported"}
			}
			sanitized[key] = text
		default:
			return nil, &requestValidationError{message: "theme_settings." + key + " is unsupported"}
		}
	}
	return sanitized, nil
}

func statusPageThemeString(value interface{}, field string, maxRunes int) (string, error) {
	text, ok := value.(string)
	if !ok {
		return "", &requestValidationError{message: field + " must be a string"}
	}
	text = strings.TrimSpace(text)
	if len([]rune(text)) > maxRunes {
		return "", &requestValidationError{message: field + " is too long"}
	}
	return text, nil
}

func validStatusPageThemeHexColor(value string) bool {
	if len(value) != 7 || value[0] != '#' {
		return false
	}
	for _, r := range value[1:] {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F') {
			continue
		}
		return false
	}
	return true
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
