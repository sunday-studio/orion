package api

import (
	"errors"
	"net"
	"net/http"
	"net/url"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

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
		return &requestValidationError{message: field + " must be an absolute http or https URL"}
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return &requestValidationError{message: field + " must be an absolute http or https URL"}
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
