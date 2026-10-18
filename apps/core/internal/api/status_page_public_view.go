package api

import (
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"orion/core/internal/db"
	"orion/core/internal/publicpage"
	"orion/core/internal/utils"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// publicStatusPageRequestWantsHTML is true for browsers; API clients and ?format=json get JSON.
func publicStatusPageRequestWantsHTML(c *gin.Context) bool {
	if strings.EqualFold(strings.TrimSpace(c.Query("format")), "json") {
		return false
	}
	accept := strings.ToLower(c.GetHeader("Accept"))
	return strings.Contains(accept, "text/html") && !strings.Contains(accept, "application/json")
}

// publicPageBase is the address links on a public page hang from. It works on custom domains too, because
// /status/<slug> routes resolve against the verified host as well.
func publicPageBase(slug string) string { return "/status/" + url.PathEscape(slug) }

type publicPageTheme struct {
	accent, logoURL, logoAlt, class string
	showUptime, showHistory         bool
}

func publicPageThemeFrom(settings map[string]any) publicPageTheme {
	settings = safeStatusPagePublicThemeSettings(settings)
	theme := publicPageTheme{showUptime: true, showHistory: true}
	if value, ok := settings["accent_color"].(string); ok && validStatusPageThemeHexColor(value) {
		theme.accent = strings.ToLower(value)
	}
	if value, ok := settings["logo_url"].(string); ok {
		theme.logoURL = strings.TrimSpace(value)
	}
	if value, ok := settings["logo_alt"].(string); ok {
		theme.logoAlt = strings.TrimSpace(value)
	}
	if theme.logoURL != "" && theme.logoAlt == "" {
		theme.logoAlt = "Status page logo"
	}
	switch settings["theme_mode"] {
	case "dark":
		theme.class = "theme-dark"
	case "light":
		theme.class = "theme-light"
	}
	if value, ok := settings["show_uptime_summary"].(bool); ok {
		theme.showUptime = value
	}
	if value, ok := settings["show_incident_history"].(bool); ok {
		theme.showHistory = value
	}
	return theme
}

// publicPageShell builds the header, metadata and theme shared by every themed public page.
// Pages other than the status page set their own title and are not given the canonical address.
func publicPageShell(c *gin.Context, preview StatusPagePreviewResponse, title string, main bool) (publicpage.Shell, publicPageTheme) {
	theme := publicPageThemeFrom(preview.Page.ThemeSettings)
	base := publicPageBase(preview.Page.Slug)
	meta := preview.Metadata
	shell := publicpage.Shell{
		SiteName: preview.Page.Title, LogoURL: theme.logoURL, LogoAlt: theme.logoAlt, AccentColor: theme.accent, ThemeClass: theme.class,
		Title: firstNonEmpty(title, meta.Title, preview.Page.Title), Description: firstNonEmpty(meta.Description, preview.Page.Description),
		OGType: firstNonEmpty(meta.OpenGraph.Type, "website"), OGImage: meta.OpenGraph.ImageURL,
		BasePath: base, HomeURL: base, HistoryURL: base + "/history", SubscribeURL: base + "/subscribe", FeedURL: base + "/feed.atom", RSSURL: base + "/feed.rss",
		ShowHistory: theme.showHistory,
	}
	if main {
		shell.Canonical = firstNonEmpty(meta.CanonicalURL, publicStatusPageHTMLURL(c, preview.Page.Slug))
		shell.OGTitle = firstNonEmpty(meta.OpenGraph.Title, shell.Title)
		shell.OGDesc = firstNonEmpty(meta.OpenGraph.Description, shell.Description)
		shell.OGURL = firstNonEmpty(meta.OpenGraph.URL, shell.Canonical)
	}
	return shell, theme
}

func publicStatusPageHTMLURL(c *gin.Context, slug string) string {
	scheme := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto"))
	if scheme == "" {
		scheme = "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
	}
	if c.Request.URL.Path == "/" {
		return scheme + "://" + c.Request.Host
	}
	return scheme + "://" + c.Request.Host + publicPageBase(slug)
}

// publicStatusClass maps a component or overall status to a pill and dot colour class.
func publicStatusClass(status string) string {
	switch status {
	case "operational":
		return "up"
	case "degraded":
		return "deg"
	case "partial_outage", "major_outage":
		return "down"
	case "maintenance":
		return "mnt"
	default:
		return "unk"
	}
}

func publicBarClass(bucket StatusPagePublicUptimeBucketResponse) string {
	switch bucket.Status {
	case "operational":
		return "up"
	case "degraded":
		return "deg"
	case "partial_outage", "major_outage":
		return "down"
	case "maintenance":
		return "mnt"
	default:
		return ""
	}
}

// publicIncidentPill is how a published incident status reads and looks.
func publicIncidentPill(status string) (string, string) {
	switch status {
	case "investigating":
		return "Investigating", "down"
	case "identified":
		return "Identified", "deg"
	case "monitoring":
		return "Monitoring", "deg"
	case "resolved":
		return "Resolved", "res"
	case "scheduled":
		return "Scheduled", "mnt"
	default:
		return "Unknown", "unk"
	}
}

func publicStatusPageHTMLSummary(status string) string {
	switch status {
	case "operational":
		return "All systems operational"
	case "degraded":
		return "Some systems degraded"
	case "partial_outage":
		return "Partial outage"
	case "major_outage":
		return "Major outage"
	case "maintenance":
		return "Maintenance in progress"
	default:
		return "Status unavailable"
	}
}

func publicTickerFor(status string) (string, publicpage.Tone) {
	switch status {
	case "operational":
		return "Operational", publicpage.ToneGood
	case "degraded":
		return "Degraded", publicpage.ToneWarn
	case "partial_outage":
		return "Partial outage", publicpage.ToneBad
	case "major_outage":
		return "Major outage", publicpage.ToneBad
	case "maintenance":
		return "Maintenance", publicpage.ToneMaint
	default:
		return "Unknown", publicpage.ToneWarn
	}
}

func publicIncidentTone(status string) publicpage.Tone {
	switch status {
	case "resolved":
		return publicpage.ToneGood
	case "scheduled":
		return publicpage.ToneMaint
	case "investigating":
		return publicpage.ToneBad
	default:
		return publicpage.ToneWarn
	}
}

func publicDuration(from time.Time, to *time.Time) string {
	if to == nil {
		return "Ongoing"
	}
	d := to.Sub(from).Round(time.Minute)
	switch {
	case d < time.Minute:
		return "<1m"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 48*time.Hour:
		if m := int(d.Minutes()) % 60; m != 0 {
			return fmt.Sprintf("%dh %dm", int(d.Hours()), m)
		}
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

func publicIncidentStart(i StatusPagePublicIncidentResponse) time.Time {
	switch {
	case i.PublishedAt != nil:
		return *i.PublishedAt
	case i.ScheduledStartAt != nil:
		return *i.ScheduledStartAt
	}
	return time.Time{}
}

// writePublicPage renders a public HTML page. Pages that carry a secret in the address are never cached,
// referred onward or indexed.
func (s *Server) writePublicPage(c *gin.Context, status int, name string, data any, private bool) {
	payload, err := publicpage.Render(name, data)
	if err != nil {
		s.logger.Error("Failed to render public page", "page", name, "error", err)
		utils.InternalError(c, "Failed to render status page", err)
		return
	}
	if private {
		c.Header("Cache-Control", "no-store")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("X-Robots-Tag", "noindex")
		c.Data(status, "text/html; charset=utf-8", payload)
		return
	}
	writePublicStatusPagePayload(c, status, "text/html; charset=utf-8", payload)
}

func (s *Server) publicPreviewFor(page db.StatusPage) (StatusPagePreviewResponse, error) {
	detail, err := s.loadStatusPageDetail(page.ID)
	if err != nil {
		return StatusPagePreviewResponse{}, err
	}
	return s.statusPagePreviewProjection(detail, false, "", false), nil
}

func ticker(text string, tone publicpage.Tone) template.HTML { return publicpage.Ticker(text, tone) }

var _ = http.StatusOK
