package api

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"strings"
)

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
		case "theme_mode":
			text, err := statusPageThemeString(value, "theme_settings.theme_mode", 64)
			if err != nil {
				return nil, err
			}
			if text != "light" && text != "dark" && text != "system" {
				return nil, &requestValidationError{message: "theme_settings.theme_mode is unsupported"}
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
