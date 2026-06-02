package api

import (
	"net/http"
	"strings"

	"orion/core/internal/utils"

	"github.com/gin-gonic/gin"
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
	preview, ok := s.loadPublicStatusPageProjection(c, c.Param("slug"))
	if !ok {
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
	preview, ok := s.loadPublicStatusPageProjection(c, c.Param("slug"))
	if !ok {
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
