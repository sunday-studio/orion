package api

import (
	"errors"
	"net/http"
	"strings"

	"orion/core/internal/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type statusPageBadge struct {
	Label         string
	Status        string
	StatusDisplay string
}

// getPublicStatusPageBadge returns a public SVG badge for a status page.
// @Summary      Get public status page badge
// @Description  Get an embeddable SVG badge for the current public-safe status page state
// @Tags         public-status
// @Produce      image/svg+xml
// @ID           getPublicStatusPageBadge
// @Param        slug  path  string  true  "Status page slug"
// @Success      200   {string}  string  "SVG badge"
// @Failure      404   {object}  utils.APIResponse
// @Failure      500   {object}  utils.APIResponse
// @Router       /status/{slug}/badge.svg [get]
func (s *Server) getPublicStatusPageBadge(c *gin.Context) {
	preview, found := s.publicBadgeProjection(c)
	if !found {
		return
	}

	s.writeStatusPageBadge(c, statusPageBadge{
		Label:         preview.Page.Title,
		Status:        preview.OverallStatus,
		StatusDisplay: preview.OverallStatusDisplay,
	})
}

// getPublicStatusPageComponentBadge returns a public SVG badge for a component.
// @Summary      Get public status page component badge
// @Description  Get an embeddable SVG badge for a visible public component state
// @Tags         public-status
// @Produce      image/svg+xml
// @ID           getPublicStatusPageComponentBadge
// @Param        slug          path  string  true  "Status page slug"
// @Param        component_id  path  string  true  "Public component ID"
// @Success      200           {string}  string  "SVG badge"
// @Failure      404           {object}  utils.APIResponse
// @Failure      500           {object}  utils.APIResponse
// @Router       /status/{slug}/components/{component_id}/badge.svg [get]
func (s *Server) getPublicStatusPageComponentBadge(c *gin.Context) {
	preview, found := s.publicBadgeProjection(c)
	if !found {
		return
	}

	componentID := strings.TrimSpace(c.Param("component_id"))
	for _, section := range preview.Sections {
		for _, component := range section.Components {
			if component.ID == componentID {
				s.writeStatusPageBadge(c, statusPageBadge{
					Label:         component.Name,
					Status:        component.Status,
					StatusDisplay: component.StatusDisplay,
				})
				return
			}
		}
	}

	utils.NotFound(c, "Status page component not found")
}

func (s *Server) writeStatusPageBadge(c *gin.Context, badge statusPageBadge) {
	label := strings.TrimSpace(badge.Label)
	if label == "" {
		label = "Status"
	}
	statusDisplay := strings.TrimSpace(badge.StatusDisplay)
	if statusDisplay == "" {
		statusDisplay = publicStatusDisplay(badge.Status)
	}

	svg := utils.StatusBadgeSVG(label, strings.ToLower(statusDisplay), utils.StatusBadgeColor(badge.Status))
	c.Header("Cache-Control", utils.StatusBadgeCacheControl)
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, "image/svg+xml; charset=utf-8", []byte(svg))
}

// publicBadgeProjection loads the public page for a badge. Unknown, draft and private pages all
// answer with the same grey "unknown" badge, so a badge never reveals whether a page exists.
func (s *Server) publicBadgeProjection(c *gin.Context) (StatusPagePreviewResponse, bool) {
	page, err := s.loadPublicStatusPageForRequest(c, c.Param("slug"))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			s.writeStatusPageBadge(c, statusPageBadge{Label: "Status", Status: "unknown", StatusDisplay: "unknown"})
		} else {
			utils.InternalError(c, "Failed to load status page", err)
		}
		return StatusPagePreviewResponse{}, false
	}
	detail, err := s.loadStatusPageDetail(page.ID)
	if err != nil {
		utils.InternalError(c, "Failed to load status page", err)
		return StatusPagePreviewResponse{}, false
	}
	return s.statusPagePreviewWithUptime(detail, false, statusPagePublicDefaultUptimeWindow), true
}

// getPublicStatusPageBadgeJSON returns a shields.io endpoint-compatible status badge.
// @Summary      Get public status page badge as JSON
// @Description  Shields.io endpoint format: schemaVersion, label, message, color.
// @Tags         public-status
// @Produce      json
// @ID           getPublicStatusPageBadgeJSON
// @Param        slug  path  string  true  "Status page slug"
// @Success      200   {object}  map[string]interface{}
// @Router       /status/{slug}/badge.json [get]
func (s *Server) getPublicStatusPageBadgeJSON(c *gin.Context) {
	label, status, display := "Status", "unknown", "unknown"
	if preview, found := s.publicBadgeProjectionQuiet(c); found {
		label, status, display = preview.Page.Title, preview.OverallStatus, strings.ToLower(preview.OverallStatusDisplay)
	}
	c.Header("Cache-Control", utils.StatusBadgeCacheControl)
	c.JSON(http.StatusOK, gin.H{"schemaVersion": 1, "label": label, "message": display, "color": strings.TrimPrefix(utils.StatusBadgeColor(status), "#")})
}

func (s *Server) publicBadgeProjectionQuiet(c *gin.Context) (StatusPagePreviewResponse, bool) {
	page, err := s.loadPublicStatusPageForRequest(c, c.Param("slug"))
	if err != nil {
		return StatusPagePreviewResponse{}, false
	}
	detail, err := s.loadStatusPageDetail(page.ID)
	if err != nil {
		return StatusPagePreviewResponse{}, false
	}
	return s.statusPagePreviewWithUptime(detail, false, statusPagePublicDefaultUptimeWindow), true
}
