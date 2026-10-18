package api

import (
	"fmt"
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// StatusPageListRow is one row of the status pages table.
type StatusPageListRow struct {
	StatusPageResponse
	// PublicURL is the custom domain when set, else the /status/<slug> path.
	PublicURL string `json:"public_url" example:"status.example.com"`
	// State is published, draft or maintenance.
	State           string    `json:"state" enums:"published,draft,maintenance"`
	OverallStatus   string    `json:"overall_status" example:"operational"`
	ComponentCount  int       `json:"component_count"`
	SubscriberCount int       `json:"subscriber_count"`
	LastUpdated     time.Time `json:"last_updated"`
}

func statusPageDisplayAddress(page db.StatusPage) string {
	if page.CustomDomain != "" {
		return page.CustomDomain
	}
	return "/status/" + page.Slug
}

func (s *Server) statusPageListRow(page db.StatusPage) StatusPageListRow {
	row := StatusPageListRow{StatusPageResponse: statusPageResponse(page), PublicURL: statusPageDisplayAddress(page), State: "draft", OverallStatus: "unknown", LastUpdated: page.UpdatedAt}
	var components, subscribers int64
	s.db.Model(&db.StatusPageComponent{}).Where("status_page_id = ? AND visible = ?", page.ID, true).Count(&components)
	s.db.Model(&db.StatusPageSubscriber{}).Where("status_page_id = ? AND state = ?", page.ID, "confirmed").Count(&subscribers)
	row.ComponentCount, row.SubscriberCount = int(components), int(subscribers)
	if detail, err := s.loadStatusPageDetail(page.ID); err == nil {
		row.OverallStatus = s.statusPagePreview(detail, false).OverallStatus
	}
	if page.Visibility == statusPageVisibilityPublic || page.Visibility == statusPageVisibilityUnlisted {
		row.State = "published"
		if row.OverallStatus == "maintenance" {
			row.State = "maintenance"
		}
	}
	return row
}

type StatusPageReadinessCheck struct {
	Key      string `json:"key" example:"components_mapped"`
	Label    string `json:"label" example:"Every component is mapped to a resource"`
	OK       bool   `json:"ok"`
	Blocking bool   `json:"blocking"`
	Detail   string `json:"detail,omitempty"`
}

type StatusPageReadinessResponse struct {
	Ready  bool                       `json:"ready"`
	Passed int                        `json:"passed"`
	Total  int                        `json:"total"`
	Checks []StatusPageReadinessCheck `json:"checks"`
	// Errors and Warnings repeat what publishing would report.
	Validation StatusPagePublishValidationResponse `json:"validation"`
}

func (s *Server) statusPageReadiness(pageID string) (StatusPageReadinessResponse, error) {
	detail, err := s.loadStatusPageDetail(pageID)
	if err != nil {
		return StatusPageReadinessResponse{}, err
	}
	validation, err := s.validateStatusPageForPublish(pageID)
	if err != nil {
		return StatusPageReadinessResponse{}, err
	}
	visible, unmapped := 0, 0
	for _, component := range detail.Components {
		if !component.Visible {
			continue
		}
		visible++
		if component.ManualStatus == "" && len(component.Mappings) == 0 {
			unmapped++
		}
	}
	checks := []StatusPageReadinessCheck{
		{Key: "has_components", Label: "At least one component is shown", OK: visible > 0, Blocking: true},
		{Key: "components_mapped", Label: "Every component is mapped to a resource", OK: visible > 0 && unmapped == 0, Blocking: true},
		{Key: "description_set", Label: "Public description is set", OK: strings.TrimSpace(detail.Page.Description) != "", Blocking: false},
	}
	if unmapped > 0 {
		checks[1].Detail = fmt.Sprintf("%d not mapped", unmapped)
	}
	checks = append(checks, s.statusPageDomainCheck(detail.Page)...)
	out := StatusPageReadinessResponse{Checks: checks, Total: len(checks), Validation: validation, Ready: len(validation.Errors) == 0}
	for _, check := range checks {
		if check.OK {
			out.Passed++
		}
	}
	return out, nil
}

// getStatusPageReadiness lists what the publish step checks.
// @Summary      Status page publish checklist
// @Description  Structured publish checks: blocking ones stop publishing, others are advice.
// @Tags         status-pages
// @Produce      json
// @Security     BearerAuth
// @ID           getStatusPageReadiness
// @Param        id   path      string  true  "Status page ID"
// @Success      200  {object}  utils.APIResponse{data=StatusPageReadinessResponse}
// @Failure      404  {object}  utils.APIResponse
// @Router       /v1/status-pages/{id}/readiness [get]
func (s *Server) getStatusPageReadiness(c *gin.Context) {
	out, err := s.statusPageReadiness(c.Param("id"))
	if err != nil {
		writeStatusPageLoadError(c, err, "Failed to check status page")
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Readiness retrieved", out)
}

type StatusPageSlugCheckResponse struct {
	Slug      string `json:"slug"`
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

// checkStatusPageSlug tells whether a slug can be used.
// @Summary      Check status page slug
// @Tags         status-pages
// @Produce      json
// @Security     BearerAuth
// @ID           checkStatusPageSlug
// @Param        slug        query     string  true   "Slug to check"
// @Param        exclude_id  query     string  false  "Page being edited, ignored in the check"
// @Success      200         {object}  utils.APIResponse{data=StatusPageSlugCheckResponse}
// @Router       /v1/status-pages/slug-check [get]
func (s *Server) checkStatusPageSlug(c *gin.Context) {
	slug := normalizeSlug(c.Query("slug"))
	out := StatusPageSlugCheckResponse{Slug: slug}
	switch {
	case slug == "":
		out.Reason = "Enter an address."
	case !validSlug(slug):
		out.Reason = "Use lowercase letters, numbers and hyphens."
	default:
		var n int64
		s.db.Model(&db.StatusPage{}).Where("slug = ? AND id <> ?", slug, c.Query("exclude_id")).Count(&n)
		if n > 0 {
			out.Reason = "That address is taken."
		} else {
			out.Available = true
		}
	}
	utils.SuccessResponse(c, http.StatusOK, "Slug checked", out)
}

type StatusPageBadgeLinks struct {
	SVGURL   string `json:"svg_url"`
	JSONURL  string `json:"json_url,omitempty"`
	Markdown string `json:"markdown"`
	HTML     string `json:"html"`
}

type StatusPageComponentBadgeLinks struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	StatusPageBadgeLinks
}

type StatusPageBadgesResponse struct {
	Page       StatusPageBadgeLinks            `json:"page"`
	Components []StatusPageComponentBadgeLinks `json:"components"`
	// Note: unpublished pages serve an unknown badge.
	Note string `json:"note"`
}

func (s *Server) statusBadgeOrigin() string {
	if s.cfg == nil {
		return ""
	}
	if o := strings.TrimRight(strings.TrimSpace(s.cfg.PublicStatusURLOrigin), "/"); o != "" {
		return o
	}
	return strings.TrimRight(strings.TrimSpace(s.cfg.PublicURL), "/")
}

func badgeLinks(label, svg, jsonURL string) StatusPageBadgeLinks {
	return StatusPageBadgeLinks{SVGURL: svg, JSONURL: jsonURL, Markdown: fmt.Sprintf("![%s](%s)", label, svg), HTML: fmt.Sprintf(`<img src="%s" alt="%s">`, svg, label)}
}

// getStatusPageBadges returns embeddable badge URLs and snippets.
// @Summary      Status page badges
// @Tags         status-pages
// @Produce      json
// @Security     BearerAuth
// @ID           getStatusPageBadges
// @Param        id   path      string  true  "Status page ID"
// @Success      200  {object}  utils.APIResponse{data=StatusPageBadgesResponse}
// @Failure      404  {object}  utils.APIResponse
// @Router       /v1/status-pages/{id}/badges [get]
func (s *Server) getStatusPageBadges(c *gin.Context) {
	detail, err := s.loadStatusPageDetail(c.Param("id"))
	if err != nil {
		writeStatusPageLoadError(c, err, "Failed to load status page")
		return
	}
	base := s.statusBadgeOrigin() + "/status/" + detail.Page.Slug
	out := StatusPageBadgesResponse{Page: badgeLinks("Status", base+"/badge.svg", base+"/badge.json"), Components: []StatusPageComponentBadgeLinks{},
		Note: "Served by Core with short cache. Unpublished pages return an unknown badge."}
	for _, component := range detail.Components {
		if !component.Visible {
			continue
		}
		url := base + "/components/" + component.ID + "/badge.svg"
		out.Components = append(out.Components, StatusPageComponentBadgeLinks{ID: component.ID, Name: component.PublicName, StatusPageBadgeLinks: badgeLinks(component.PublicName, url, "")})
	}
	utils.SuccessResponse(c, http.StatusOK, "Badges retrieved", out)
}

// statusPageDomainCheck is the optional custom-domain row of the checklist.
func (s *Server) statusPageDomainCheck(page StatusPageResponse) []StatusPageReadinessCheck {
	if page.CustomDomain == "" {
		return nil
	}
	ok := page.DomainStatus == "verified"
	check := StatusPageReadinessCheck{Key: "domain_verified", Label: "Custom domain DNS is verified", OK: ok}
	if !ok {
		check.Label = "Custom domain DNS is not verified yet"
		check.Detail = "The page will use /status/" + page.Slug + " until it is"
	}
	return []StatusPageReadinessCheck{check}
}
