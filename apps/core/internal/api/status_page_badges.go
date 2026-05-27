package api

import (
	"fmt"
	"html"
	"math"
	"net/http"
	"strings"
	"unicode/utf8"

	"orion/core/internal/utils"

	"github.com/gin-gonic/gin"
)

const statusPageBadgeCacheControl = "public, max-age=60, stale-while-revalidate=120"

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

	svg := renderStatusPageBadgeSVG(label, strings.ToLower(statusDisplay), statusPageBadgeColor(badge.Status))
	c.Header("Cache-Control", statusPageBadgeCacheControl)
	c.Header("X-Content-Type-Options", "nosniff")
	c.Data(http.StatusOK, "image/svg+xml; charset=utf-8", []byte(svg))
}

func renderStatusPageBadgeSVG(label string, message string, color string) string {
	label = truncateStatusPageBadgeText(label, 48)
	message = truncateStatusPageBadgeText(message, 32)

	labelWidth := statusPageBadgeTextWidth(label, 72, 240)
	messageWidth := statusPageBadgeTextWidth(message, 92, 220)
	totalWidth := labelWidth + messageWidth
	labelTextX := labelWidth / 2
	messageTextX := labelWidth + messageWidth/2

	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="20" role="img" aria-label="%s: %s"><title>%s: %s</title><linearGradient id="s" x2="0" y2="100%%"><stop offset="0" stop-color="#fff" stop-opacity=".08"/><stop offset="1" stop-opacity=".08"/></linearGradient><clipPath id="r"><rect width="%d" height="20" rx="3" fill="#fff"/></clipPath><g clip-path="url(#r)"><rect width="%d" height="20" fill="#555"/><rect x="%d" width="%d" height="20" fill="%s"/><rect width="%d" height="20" fill="url(#s)"/></g><g fill="#fff" text-anchor="middle" font-family="Verdana,Geneva,DejaVu Sans,sans-serif" font-size="11"><text x="%d" y="15" fill="#010101" fill-opacity=".3">%s</text><text x="%d" y="14">%s</text><text x="%d" y="15" fill="#010101" fill-opacity=".3">%s</text><text x="%d" y="14">%s</text></g></svg>`,
		totalWidth,
		html.EscapeString(label),
		html.EscapeString(message),
		html.EscapeString(label),
		html.EscapeString(message),
		totalWidth,
		labelWidth,
		labelWidth,
		messageWidth,
		color,
		totalWidth,
		labelTextX,
		html.EscapeString(label),
		labelTextX,
		html.EscapeString(label),
		messageTextX,
		html.EscapeString(message),
		messageTextX,
		html.EscapeString(message),
	)
}

func statusPageBadgeTextWidth(value string, minWidth int, maxWidth int) int {
	width := utf8.RuneCountInString(value)*7 + 18
	return int(math.Max(float64(minWidth), math.Min(float64(maxWidth), float64(width))))
}

func truncateStatusPageBadgeText(value string, limit int) string {
	value = strings.Join(strings.Fields(value), " ")
	if utf8.RuneCountInString(value) <= limit {
		return value
	}

	runes := []rune(value)
	if limit <= 1 {
		return string(runes[:limit])
	}
	return string(runes[:limit-1]) + "..."
}

func statusPageBadgeColor(status string) string {
	switch status {
	case "operational":
		return "#15803d"
	case "degraded":
		return "#ca8a04"
	case "partial_outage":
		return "#ea580c"
	case "major_outage":
		return "#dc2626"
	case "maintenance":
		return "#2563eb"
	default:
		return "#64748b"
	}
}
