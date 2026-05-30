package api

import (
	"errors"
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

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
	if publicStatusPageRequestWantsHTML(c) {
		s.writePublicStatusPageHTML(c, preview)
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
			Slug:          detail.Page.Slug,
			Title:         detail.Page.Title,
			Description:   detail.Page.Description,
			Visibility:    detail.Page.Visibility,
			ThemeSettings: detail.Page.ThemeSettings,
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
