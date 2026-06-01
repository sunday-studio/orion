package api

import (
	"bytes"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type statusPageHTMLView struct {
	Title                string
	Description          string
	CanonicalURL         string
	OpenGraphTitle       string
	OpenGraphDescription string
	OpenGraphURL         string
	OpenGraphType        string
	OpenGraphSiteName    string
	OpenGraphImageURL    string
	PageTitle            string
	PageDescription      string
	OverallStatus        string
	OverallStatusDisplay string
	LastUpdated          string
	AccentColor          string
	LogoURL              string
	LogoAlt              string
	HeaderClass          string
	DensityClass         string
	FeedURL              string
	BadgeURL             string
	ShowUptimeSummary    bool
	ShowIncidentHistory  bool
	Sections             []StatusPagePublicSectionResponse
	Incidents            []statusPageHTMLIncident
}

type statusPageHTMLIncident struct {
	ID            string
	Title         string
	PublicStatus  string
	Severity      string
	ImpactSummary string
	PublishedAt   string
	ResolvedAt    string
	DetailURL     string
}

var statusPageHTMLTemplate = template.Must(template.New("status-page").Parse(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{{ .Title }}</title>
  {{- if .Description }}
  <meta name="description" content="{{ .Description }}">
  {{- end }}
  {{- if .CanonicalURL }}
  <link rel="canonical" href="{{ .CanonicalURL }}">
  {{- end }}
  <meta property="og:title" content="{{ .OpenGraphTitle }}">
  {{- if .OpenGraphDescription }}
  <meta property="og:description" content="{{ .OpenGraphDescription }}">
  {{- end }}
  {{- if .OpenGraphURL }}
  <meta property="og:url" content="{{ .OpenGraphURL }}">
  {{- end }}
  <meta property="og:type" content="{{ .OpenGraphType }}">
  <meta property="og:site_name" content="{{ .OpenGraphSiteName }}">
  {{- if .OpenGraphImageURL }}
  <meta property="og:image" content="{{ .OpenGraphImageURL }}">
  {{- end }}
  {{- if .FeedURL }}
  <link rel="alternate" type="application/atom+xml" href="{{ .FeedURL }}" title="{{ .PageTitle }} feed">
  {{- end }}
  <style>
    :root { color-scheme: light; --accent: {{ .AccentColor }}; --border: #d7dde5; --muted: #5f6b7a; --text: #182230; --bg: #f6f8fb; --panel: #ffffff; }
    * { box-sizing: border-box; }
    body { margin: 0; font-family: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; background: var(--bg); color: var(--text); line-height: 1.5; }
    main { max-width: 960px; margin: 0 auto; padding: 40px 20px 56px; }
    header { display: flex; align-items: center; justify-content: space-between; gap: 24px; margin-bottom: 32px; }
    header.centered { flex-direction: column; align-items: flex-start; }
    header.compact { margin-bottom: 20px; }
    .brand { display: flex; align-items: center; gap: 14px; min-width: 0; }
    .brand img { max-width: 160px; max-height: 56px; object-fit: contain; }
    h1 { margin: 0; font-size: 2rem; line-height: 1.15; letter-spacing: 0; }
    h2 { margin: 0 0 14px; font-size: 1.1rem; letter-spacing: 0; }
    p { margin: 0; }
    a { color: var(--accent); text-decoration-thickness: 1px; text-underline-offset: 3px; }
    .description { margin-top: 8px; color: var(--muted); max-width: 64ch; }
    .summary { border-left: 5px solid var(--accent); background: var(--panel); border-radius: 8px; padding: 18px 20px; margin-bottom: 24px; box-shadow: 0 1px 2px rgba(16, 24, 40, .06); }
    .summary strong { display: block; font-size: 1.35rem; line-height: 1.25; }
    .summary span { color: var(--muted); font-size: .95rem; }
    .links { display: flex; gap: 14px; flex-wrap: wrap; color: var(--muted); font-size: .95rem; }
    .section { margin-top: 22px; }
    .component-list, .incident-list { display: grid; gap: 10px; }
    .compact-density .component-list, .compact-density .incident-list { gap: 6px; }
    .component, .incident { background: var(--panel); border: 1px solid var(--border); border-radius: 8px; padding: 14px 16px; display: grid; gap: 6px; }
    .compact-density .component, .compact-density .incident { padding: 10px 12px; }
    .row { display: flex; align-items: center; justify-content: space-between; gap: 16px; }
    .name { font-weight: 700; }
    .muted { color: var(--muted); font-size: .94rem; }
    .status { display: inline-flex; align-items: center; border-radius: 999px; padding: 4px 10px; background: color-mix(in srgb, var(--accent) 12%, white); color: #101828; font-size: .86rem; white-space: nowrap; }
    .empty { color: var(--muted); background: var(--panel); border: 1px dashed var(--border); border-radius: 8px; padding: 16px; }
    footer { margin-top: 36px; color: var(--muted); font-size: .9rem; }
    @media (max-width: 640px) { main { padding: 28px 16px 44px; } header, .row { align-items: flex-start; flex-direction: column; } h1 { font-size: 1.55rem; } }
  </style>
</head>
<body>
  <main class="{{ .DensityClass }}">
    <header class="{{ .HeaderClass }}">
      <div>
        <div class="brand">
          {{- if .LogoURL }}
          <img src="{{ .LogoURL }}" alt="{{ .LogoAlt }}">
          {{- end }}
          <h1>{{ .PageTitle }}</h1>
        </div>
        {{- if .PageDescription }}
        <p class="description">{{ .PageDescription }}</p>
        {{- end }}
      </div>
      <nav class="links" aria-label="Public status links">
        {{- if .FeedURL }}<a href="{{ .FeedURL }}">Atom feed</a>{{ end }}
        {{- if .BadgeURL }}<a href="{{ .BadgeURL }}">Status badge</a>{{ end }}
      </nav>
    </header>
    <section class="summary" aria-label="Current status">
      <strong>{{ .OverallStatusDisplay }}</strong>
      <span>Updated {{ .LastUpdated }}</span>
    </section>
    {{- if .ShowUptimeSummary }}
    {{- range .Sections }}
    <section class="section" aria-labelledby="section-{{ .ID }}">
      <h2 id="section-{{ .ID }}">{{ .Name }}</h2>
      {{- if .Components }}
      <div class="component-list">
        {{- range .Components }}
        <article class="component">
          <div class="row"><p class="name">{{ .Name }}</p><span class="status">{{ .StatusDisplay }}</span></div>
          {{- if .Description }}<p class="muted">{{ .Description }}</p>{{ end }}
          {{- if .StatusReason }}<p class="muted">{{ .StatusReason }}</p>{{ end }}
        </article>
        {{- end }}
      </div>
      {{- else }}
      <p class="empty">No public components are configured.</p>
      {{- end }}
    </section>
    {{- end }}
    {{- end }}
    {{- if .ShowIncidentHistory }}
    <section class="section" aria-labelledby="incidents">
      <h2 id="incidents">Recent incidents</h2>
      {{- if .Incidents }}
      <div class="incident-list">
        {{- range .Incidents }}
        <article class="incident">
          <div class="row"><p class="name"><a href="{{ .DetailURL }}">{{ .Title }}</a></p><span class="status">{{ .PublicStatus }}</span></div>
          {{- if .ImpactSummary }}<p class="muted">{{ .ImpactSummary }}</p>{{ end }}
          <p class="muted">Severity {{ .Severity }}{{ if .PublishedAt }} · Published {{ .PublishedAt }}{{ end }}{{ if .ResolvedAt }} · Resolved {{ .ResolvedAt }}{{ end }}</p>
        </article>
        {{- end }}
      </div>
      {{- else }}
      <p class="empty">No published incidents.</p>
      {{- end }}
    </section>
    {{- end }}
    <footer>Powered by Orion public status.</footer>
  </main>
</body>
</html>`))

func publicStatusPageRequestWantsHTML(c *gin.Context) bool {
	if strings.EqualFold(strings.TrimSpace(c.Query("format")), "json") {
		return false
	}
	accept := strings.ToLower(c.GetHeader("Accept"))
	return strings.Contains(accept, "text/html") && !strings.Contains(accept, "application/json")
}

func (s *Server) writePublicStatusPageHTML(c *gin.Context, preview StatusPagePreviewResponse) {
	payload, err := renderPublicStatusPageHTML(c, preview)
	if err != nil {
		s.logger.Error("Failed to render public status page HTML", "slug", preview.Page.Slug, "error", err)
		c.Status(http.StatusInternalServerError)
		return
	}
	writePublicStatusPagePayload(c, http.StatusOK, "text/html; charset=utf-8", payload)
}

func renderPublicStatusPageHTML(c *gin.Context, preview StatusPagePreviewResponse) ([]byte, error) {
	theme := statusPagePublicHTMLTheme(preview.Page.ThemeSettings)
	publicURL := publicStatusPageHTMLURL(c, preview.Page.Slug)
	canonicalURL := strings.TrimSpace(preview.Metadata.CanonicalURL)
	if canonicalURL == "" {
		canonicalURL = publicURL
	}
	openGraphURL := strings.TrimSpace(preview.Metadata.OpenGraph.URL)
	if openGraphURL == "" {
		openGraphURL = canonicalURL
	}

	view := statusPageHTMLView{
		Title:                firstNonEmpty(preview.Metadata.Title, preview.Page.Title),
		Description:          firstNonEmpty(preview.Metadata.Description, preview.Page.Description),
		CanonicalURL:         canonicalURL,
		OpenGraphTitle:       firstNonEmpty(preview.Metadata.OpenGraph.Title, preview.Metadata.Title, preview.Page.Title),
		OpenGraphDescription: firstNonEmpty(preview.Metadata.OpenGraph.Description, preview.Metadata.Description, preview.Page.Description),
		OpenGraphURL:         openGraphURL,
		OpenGraphType:        firstNonEmpty(preview.Metadata.OpenGraph.Type, "website"),
		OpenGraphSiteName:    firstNonEmpty(preview.Metadata.OpenGraph.SiteName, preview.Page.Title),
		OpenGraphImageURL:    preview.Metadata.OpenGraph.ImageURL,
		PageTitle:            preview.Page.Title,
		PageDescription:      preview.Page.Description,
		OverallStatus:        preview.OverallStatus,
		OverallStatusDisplay: preview.OverallStatusDisplay,
		LastUpdated:          publicStatusPageHTMLTime(preview.LastUpdated),
		AccentColor:          theme.AccentColor,
		LogoURL:              theme.LogoURL,
		LogoAlt:              theme.LogoAlt,
		HeaderClass:          theme.HeaderClass,
		DensityClass:         theme.DensityClass,
		FeedURL:              strings.TrimRight(publicURL, "/") + "/feed.atom",
		BadgeURL:             strings.TrimRight(publicURL, "/") + "/badge.svg",
		ShowUptimeSummary:    theme.ShowUptimeSummary,
		ShowIncidentHistory:  theme.ShowIncidentHistory,
		Sections:             preview.Sections,
		Incidents:            statusPageHTMLIncidents(publicURL, preview.Incidents),
	}

	var body bytes.Buffer
	if err := statusPageHTMLTemplate.Execute(&body, view); err != nil {
		return nil, err
	}
	return body.Bytes(), nil
}

type statusPagePublicHTMLThemeConfig struct {
	AccentColor         string
	LogoURL             string
	LogoAlt             string
	HeaderClass         string
	DensityClass        string
	ShowUptimeSummary   bool
	ShowIncidentHistory bool
}

func statusPagePublicHTMLTheme(settings map[string]any) statusPagePublicHTMLThemeConfig {
	theme := statusPagePublicHTMLThemeConfig{
		AccentColor:         "#2563eb",
		HeaderClass:         "standard",
		DensityClass:        "",
		ShowUptimeSummary:   true,
		ShowIncidentHistory: true,
	}
	if value, ok := settings["accent_color"].(string); ok && validStatusPageThemeHexColor(value) {
		theme.AccentColor = strings.ToLower(value)
	}
	if value, ok := settings["logo_url"].(string); ok && validateOptionalURL(value, "theme_settings.logo_url") == nil {
		theme.LogoURL = strings.TrimSpace(value)
	}
	if value, ok := settings["logo_alt"].(string); ok {
		theme.LogoAlt = strings.TrimSpace(value)
	}
	if theme.LogoURL != "" && theme.LogoAlt == "" {
		theme.LogoAlt = "Status page logo"
	}
	if value, ok := settings["header_style"].(string); ok {
		switch value {
		case "compact", "centered":
			theme.HeaderClass = value
		}
	}
	if value, ok := settings["component_density"].(string); ok && value == "compact" {
		theme.DensityClass = "compact-density"
	}
	if value, ok := settings["show_uptime_summary"].(bool); ok {
		theme.ShowUptimeSummary = value
	}
	if value, ok := settings["show_incident_history"].(bool); ok {
		theme.ShowIncidentHistory = value
	}
	return theme
}

func statusPageHTMLIncidents(publicURL string, incidents []StatusPagePublicIncidentResponse) []statusPageHTMLIncident {
	responses := make([]statusPageHTMLIncident, 0, len(incidents))
	base := strings.TrimRight(publicURL, "/")
	for _, incident := range incidents {
		responses = append(responses, statusPageHTMLIncident{
			ID:            incident.ID,
			Title:         incident.Title,
			PublicStatus:  publicIncidentStatusDisplay(incident.PublicStatus),
			Severity:      incident.Severity,
			ImpactSummary: incident.ImpactSummary,
			PublishedAt:   publicStatusPageHTMLTimePtr(incident.PublishedAt),
			ResolvedAt:    publicStatusPageHTMLTimePtr(incident.ResolvedAt),
			DetailURL:     base + "/incidents/" + url.PathEscape(incident.ID),
		})
	}
	return responses
}

func publicIncidentStatusDisplay(value string) string {
	switch value {
	case "investigating":
		return "Investigating"
	case "identified":
		return "Identified"
	case "monitoring":
		return "Monitoring"
	case "resolved":
		return "Resolved"
	case "scheduled":
		return "Scheduled"
	default:
		return "Unknown"
	}
}

func publicStatusPageHTMLURL(c *gin.Context, slug string) string {
	scheme := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto"))
	if scheme == "" {
		scheme = "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
	}
	host := c.Request.Host
	path := c.Request.URL.Path
	if path == "/" {
		return scheme + "://" + host
	}
	return scheme + "://" + host + "/status/" + url.PathEscape(slug)
}

func publicStatusPageHTMLTimePtr(value *time.Time) string {
	if value == nil {
		return ""
	}
	return publicStatusPageHTMLTime(*value)
}

func publicStatusPageHTMLTime(value time.Time) string {
	if value.IsZero() {
		return "unknown"
	}
	return publicMinute(value).Format("2006-01-02 15:04 MST")
}
