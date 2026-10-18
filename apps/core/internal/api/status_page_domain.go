package api

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	domainStatusPending  = "pending"
	domainStatusVerified = "verified"
	domainStatusFailed   = "failed"
	domainVerifyCooldown = 10 * time.Second
)

type StatusPageDomainRecord struct {
	Type  string `json:"type" example:"CNAME"`
	Host  string `json:"host" example:"status.example.com"`
	Value string `json:"value" example:"pages.orion.example.com"`
}

type StatusPageDomainResponse struct {
	Domain string `json:"domain"`
	// Status is pending, verified or failed.
	Status string `json:"status" enums:"pending,verified,failed"`
	// StatusText is the short line shown next to the status.
	StatusText     string                   `json:"status_text" example:"Record not found yet"`
	LastCheckedAt  *time.Time               `json:"last_checked_at"`
	VerifiedAt     *time.Time               `json:"verified_at"`
	Certificate    string                   `json:"certificate" enums:"none,pending,issued"`
	Records        []StatusPageDomainRecord `json:"records"`
	TargetKnown    bool                     `json:"target_known"`
	FallbackURL    string                   `json:"fallback_url"`
	VerifyAfterSec int                      `json:"verify_after_seconds"`
}

// statusPageCNAMETarget is where custom domains must point.
func (s *Server) statusPageCNAMETarget() string {
	if s.cfg == nil {
		return ""
	}
	if t := strings.TrimSpace(s.cfg.StatusPagesCNAMETarget); t != "" {
		return strings.ToLower(t)
	}
	for _, origin := range []string{s.cfg.PublicStatusURLOrigin, s.cfg.PublicURL} {
		if u, err := url.Parse(strings.TrimSpace(origin)); err == nil && u.Hostname() != "" {
			return strings.ToLower(u.Hostname())
		}
	}
	return ""
}

func (s *Server) domainResponse(page db.StatusPage) StatusPageDomainResponse {
	target := s.statusPageCNAMETarget()
	out := StatusPageDomainResponse{Domain: page.CustomDomain, Status: page.DomainStatus, LastCheckedAt: page.DomainCheckedAt, VerifiedAt: page.DomainVerifiedAt, Records: []StatusPageDomainRecord{},
		TargetKnown: target != "", FallbackURL: "/status/" + page.Slug, Certificate: "none"}
	if page.CustomDomain == "" {
		out.Status = ""
		return out
	}
	out.Records = append(out.Records, StatusPageDomainRecord{Type: "CNAME", Host: page.CustomDomain, Value: target})
	switch page.DomainStatus {
	case domainStatusVerified:
		out.StatusText, out.Certificate = "Verified", "issued"
	case domainStatusFailed:
		out.StatusText, out.Certificate = firstNonEmpty(page.DomainError, "Record points somewhere else"), "none"
	default:
		out.StatusText, out.Certificate = firstNonEmpty(page.DomainError, "Record not found yet"), "pending"
	}
	if page.DomainCheckedAt != nil {
		if wait := domainVerifyCooldown - time.Since(*page.DomainCheckedAt); wait > 0 {
			out.VerifyAfterSec = int(wait.Seconds()) + 1
		}
	}
	return out
}

func (s *Server) loadDomainPage(c *gin.Context) (db.StatusPage, bool) {
	var page db.StatusPage
	err := s.db.Where("id = ?", c.Param("id")).First(&page).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		utils.NotFound(c, "Status page not found")
		return page, false
	}
	if err != nil {
		utils.InternalError(c, "Failed to load status page", err)
		return page, false
	}
	return page, true
}

// getStatusPageDomain shows the custom domain state and the DNS record to create.
// @Summary      Status page custom domain
// @Tags         status-pages
// @Produce      json
// @Security     BearerAuth
// @ID           getStatusPageDomain
// @Param        id   path      string  true  "Status page ID"
// @Success      200  {object}  utils.APIResponse{data=StatusPageDomainResponse}
// @Failure      404  {object}  utils.APIResponse
// @Router       /v1/status-pages/{id}/domain [get]
func (s *Server) getStatusPageDomain(c *gin.Context) {
	page, ok := s.loadDomainPage(c)
	if !ok {
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Domain retrieved", s.domainResponse(page))
}

// verifyStatusPageDomain looks up the DNS record now.
// @Summary      Verify status page custom domain
// @Description  Resolves the domain's CNAME and compares it with the target. Checks closer than 10 seconds apart return the last result.
// @Tags         status-pages
// @Produce      json
// @Security     BearerAuth
// @ID           verifyStatusPageDomain
// @Param        id   path      string  true  "Status page ID"
// @Success      200  {object}  utils.APIResponse{data=StatusPageDomainResponse}
// @Failure      400  {object}  utils.APIResponse
// @Failure      404  {object}  utils.APIResponse
// @Router       /v1/status-pages/{id}/domain/verify [post]
func (s *Server) verifyStatusPageDomain(c *gin.Context) {
	page, ok := s.loadDomainPage(c)
	if !ok {
		return
	}
	if page.CustomDomain == "" {
		utils.BadRequest(c, "This page has no custom domain")
		return
	}
	if page.DomainCheckedAt != nil && time.Since(*page.DomainCheckedAt) < domainVerifyCooldown {
		utils.SuccessResponse(c, http.StatusOK, "Domain checked recently", s.domainResponse(page))
		return
	}
	now := time.Now().UTC()
	status, detail := s.checkDomainDNS(c.Request.Context(), page.CustomDomain)
	updates := map[string]any{"domain_status": status, "domain_checked_at": now, "domain_error": detail}
	if status == domainStatusVerified {
		updates["domain_verified_at"] = now
	} else {
		updates["domain_verified_at"] = nil
	}
	if err := s.db.Model(&db.StatusPage{}).Where("id = ?", page.ID).Updates(updates).Error; err != nil {
		utils.InternalError(c, "Failed to save domain check", err)
		return
	}
	s.db.Where("id = ?", page.ID).First(&page)
	utils.SuccessResponse(c, http.StatusOK, "Domain checked", s.domainResponse(page))
}

// checkDomainDNS returns the verification status and a short reason.
func (s *Server) checkDomainDNS(ctx context.Context, domain string) (string, string) {
	target := s.statusPageCNAMETarget()
	if target == "" {
		return domainStatusPending, "Orion has no CNAME target configured (ORION_STATUS_PAGES_CNAME_TARGET)"
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cname, err := s.lookupCNAME(ctx, domain)
	var dnsErr *net.DNSError
	switch {
	case errors.As(err, &dnsErr) && dnsErr.IsNotFound:
		return domainStatusPending, "Record not found yet"
	case err != nil:
		return domainStatusPending, "DNS lookup failed, try again in a minute"
	}
	got := strings.ToLower(strings.TrimSuffix(cname, "."))
	switch {
	case got == target:
		return domainStatusVerified, ""
	case got == "" || got == strings.ToLower(domain):
		return domainStatusPending, "Record not found yet"
	default:
		return domainStatusFailed, "Record points somewhere else"
	}
}

// removeStatusPageDomain clears the custom domain.
// @Summary      Remove status page custom domain
// @Tags         status-pages
// @Produce      json
// @Security     BearerAuth
// @ID           removeStatusPageDomain
// @Param        id   path      string  true  "Status page ID"
// @Success      200  {object}  utils.APIResponse{data=StatusPageDomainResponse}
// @Failure      404  {object}  utils.APIResponse
// @Router       /v1/status-pages/{id}/domain [delete]
func (s *Server) removeStatusPageDomain(c *gin.Context) {
	page, ok := s.loadDomainPage(c)
	if !ok {
		return
	}
	updates := map[string]any{"custom_domain": "", "domain_status": domainStatusPending, "domain_checked_at": nil, "domain_verified_at": nil, "domain_error": ""}
	if err := s.db.Model(&db.StatusPage{}).Where("id = ?", page.ID).Updates(updates).Error; err != nil {
		utils.InternalError(c, "Failed to remove domain", err)
		return
	}
	s.db.Where("id = ?", page.ID).First(&page)
	utils.SuccessResponse(c, http.StatusOK, "Domain removed", s.domainResponse(page))
}
