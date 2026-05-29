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
	PublicURL            string
	FeedURL              string
	BadgeURL             string
	HistoryURL           string
	SubscriberURL        string
	ShowGetUpdates       bool
	ShowUptimeBars       bool
	ShowIncidentHistory  bool
	Sections             []statusPageHTMLSection
	ActiveIncidents      []statusPageHTMLIncident
	Incidents            []statusPageHTMLIncident
}

type statusPageHTMLSection struct {
	ID            string
	Name          string
	Status        string
	StatusDisplay string
	Components    []statusPageHTMLComponent
}

type statusPageHTMLComponent struct {
	ID               string
	Name             string
	Description      string
	Status           string
	StatusDisplay    string
	StatusReason     string
	UptimeDisplay    string
	HistoryURL       string
	BadgeURL         string
	WindowStartLabel string
	WindowEndLabel   string
	Bars             []statusPageHTMLUptimeBar
}

type statusPageHTMLUptimeBar struct {
	Class string
	Label string
}

type statusPageHTMLIncident struct {
	ID            string
	Title         string
	PublicStatus  string
	StatusClass   string
	Severity      string
	ImpactSummary string
	PublishedAt   string
	ResolvedAt    string
	ScheduledAt   string
	DetailURL     string
	Components    []string
	Updates       []statusPageHTMLIncidentUpdate
}

type statusPageHTMLIncidentUpdate struct {
	Status      string
	StatusClass string
	Message     string
	PublishedAt string
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
    :root { color-scheme: light; --accent: {{ .AccentColor }}; --border: #d7dde5; --muted: #5f6b7a; --text: #182230; --bg: #f6f8fb; --panel: #ffffff; --success: #16a34a; --warning: #d97706; --danger: #dc2626; --maintenance: #2563eb; --unknown: #98a2b3; }
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
    .top-actions { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }
    .nav { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
    .nav a, .button { border: 1px solid var(--border); border-radius: 8px; color: var(--text); display: inline-flex; font-weight: 650; line-height: 1; padding: 10px 13px; text-decoration: none; }
    .nav a[aria-current="page"], .button.primary { background: var(--text); border-color: var(--text); color: var(--panel); }
    .summary { border: 1px solid color-mix(in srgb, var(--accent) 60%, var(--border)); background: color-mix(in srgb, var(--accent) 10%, var(--panel)); border-radius: 8px; padding: 18px 20px; margin-bottom: 24px; box-shadow: 0 1px 2px rgba(16, 24, 40, .06); }
    .summary.degraded, .summary.maintenance { background: #fffbeb; border-color: #f59e0b; }
    .summary.partial_outage, .summary.major_outage { background: #fef2f2; border-color: var(--danger); }
    .summary.unknown { background: #f8fafc; border-color: var(--unknown); }
    .summary-row { display: flex; align-items: center; justify-content: space-between; gap: 20px; }
    .summary-icon { align-items: center; background: var(--accent); border-radius: 999px; color: white; display: inline-flex; font-weight: 800; height: 34px; justify-content: center; width: 34px; }
    .summary-copy { align-items: center; display: flex; gap: 14px; min-width: 0; }
    .summary strong { display: block; font-size: 1.35rem; line-height: 1.25; }
    .summary span { color: var(--muted); font-size: .95rem; }
    .links { display: flex; gap: 14px; flex-wrap: wrap; color: var(--muted); font-size: .95rem; }
    .section { margin-top: 28px; scroll-margin-top: 20px; }
    .section-header { align-items: center; display: flex; justify-content: space-between; gap: 16px; margin-bottom: 12px; }
    .component-list, .incident-list, .utility-list { display: grid; gap: 10px; }
    .compact-density .component-list, .compact-density .incident-list { gap: 6px; }
    .component, .incident { background: var(--panel); border: 1px solid var(--border); border-radius: 8px; padding: 14px 16px; display: grid; gap: 6px; }
    .compact-density .component, .compact-density .incident { padding: 10px 12px; }
    .row { display: flex; align-items: center; justify-content: space-between; gap: 16px; }
    .name { font-weight: 700; }
    .muted { color: var(--muted); font-size: .94rem; }
    .status { display: inline-flex; align-items: center; border-radius: 999px; padding: 4px 10px; background: color-mix(in srgb, var(--accent) 12%, white); color: #101828; font-size: .86rem; white-space: nowrap; }
    .status::before { border-radius: 999px; content: ""; height: 8px; margin-right: 7px; width: 8px; }
    .status.operational::before, .status.resolved::before { background: var(--success); }
    .status.degraded::before, .status.identified::before, .status.monitoring::before { background: var(--warning); }
    .status.partial_outage::before, .status.major_outage::before, .status.investigating::before { background: var(--danger); }
    .status.maintenance::before, .status.scheduled::before { background: var(--maintenance); }
    .status.unknown::before { background: var(--unknown); }
    .component-meta { display: flex; justify-content: space-between; gap: 16px; }
    .uptime-strip { display: flex; gap: 3px; min-height: 28px; overflow: hidden; padding-top: 4px; }
    .uptime-bar { background: var(--unknown); border-radius: 999px; display: block; flex: 1 1 7px; min-width: 3px; }
    .uptime-bar.operational { background: #22c55e; }
    .uptime-bar.degraded { background: #f59e0b; }
    .uptime-bar.outage { background: #ef4444; }
    .uptime-bar.no-data { background: #d0d5dd; }
    .chips { display: flex; flex-wrap: wrap; gap: 6px; }
    .chip { background: #f2f4f7; border: 1px solid var(--border); border-radius: 999px; color: var(--muted); font-size: .82rem; padding: 3px 8px; }
    .timeline { border-left: 2px solid var(--border); display: grid; gap: 10px; margin-top: 6px; padding-left: 14px; }
    .timeline-item { display: grid; gap: 3px; position: relative; }
    .timeline-item::before { background: var(--accent); border: 2px solid var(--panel); border-radius: 999px; content: ""; height: 9px; left: -20px; position: absolute; top: 7px; width: 9px; }
    .empty { color: var(--muted); background: var(--panel); border: 1px dashed var(--border); border-radius: 8px; padding: 16px; }
    .utility { align-items: center; background: var(--panel); border: 1px solid var(--border); border-radius: 8px; display: flex; justify-content: space-between; gap: 12px; padding: 13px 16px; }
    footer { margin-top: 36px; color: var(--muted); font-size: .9rem; }
    @media (max-width: 640px) { main { padding: 28px 16px 44px; } header, .row, .summary-row, .component-meta, .utility { align-items: flex-start; flex-direction: column; } h1 { font-size: 1.55rem; } .top-actions { width: 100%; } .button.primary { justify-content: center; width: 100%; } }
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
      <div class="top-actions">
        <nav class="nav" aria-label="Public status navigation">
          <a href="#status" aria-current="page">Status</a>
          <a href="#events">Events</a>
          <a href="#components">Components</a>
        </nav>
        {{- if .ShowGetUpdates }}<a class="button primary" href="#updates">Get updates</a>{{ end }}
      </div>
    </header>
    <section id="status" class="summary {{ .OverallStatus }}" aria-label="Current status">
      <div class="summary-row">
        <div class="summary-copy"><span class="summary-icon" aria-hidden="true">✓</span><strong>{{ .OverallStatusDisplay }}</strong></div>
        <span>Updated {{ .LastUpdated }}</span>
      </div>
    </section>
    {{- if .ActiveIncidents }}
    <section class="section" aria-labelledby="active-events">
      <div class="section-header"><h2 id="active-events">Active incidents and maintenance</h2></div>
      <div class="incident-list">
        {{- range .ActiveIncidents }}
        <article class="incident">
          <div class="row"><p class="name"><a href="{{ .DetailURL }}">{{ .Title }}</a></p><span class="status {{ .StatusClass }}">{{ .PublicStatus }}</span></div>
          {{- if .Components }}<div class="chips" aria-label="Affected components">{{ range .Components }}<span class="chip">{{ . }}</span>{{ end }}</div>{{ end }}
          {{- if .ImpactSummary }}<p class="muted">{{ .ImpactSummary }}</p>{{ end }}
          <p class="muted">Severity {{ .Severity }}{{ if .PublishedAt }} · Published {{ .PublishedAt }}{{ end }}{{ if .ScheduledAt }} · Scheduled {{ .ScheduledAt }}{{ end }}</p>
          {{- if .Updates }}
          <div class="timeline">
            {{- range .Updates }}
            <div class="timeline-item"><p><strong>{{ .Status }}</strong>{{ if .PublishedAt }} · <span class="muted">{{ .PublishedAt }}</span>{{ end }}</p><p class="muted">{{ .Message }}</p></div>
            {{- end }}
          </div>
          {{- end }}
        </article>
        {{- end }}
      </div>
    </section>
    {{- end }}
    <section id="components" class="section" aria-labelledby="components-heading">
      <div class="section-header"><h2 id="components-heading">Component health</h2></div>
    {{- range .Sections }}
    <section class="section" aria-labelledby="section-{{ .ID }}">
      <div class="section-header"><h2 id="section-{{ .ID }}">{{ .Name }}</h2><span class="status {{ .Status }}">{{ .StatusDisplay }}</span></div>
      {{- if .Components }}
      <div class="component-list">
        {{- range .Components }}
        <article class="component">
          <div class="row"><p class="name">{{ .Name }}</p><span class="status {{ .Status }}">{{ .StatusDisplay }}</span></div>
          {{- if .Description }}<p class="muted">{{ .Description }}</p>{{ end }}
          {{- if .StatusReason }}<p class="muted">{{ .StatusReason }}</p>{{ end }}
          {{- if $.ShowUptimeBars }}
          <div class="uptime-strip" aria-label="{{ .Name }} {{ .UptimeDisplay }} uptime over the current window">
            {{- range .Bars }}<span class="uptime-bar {{ .Class }}" title="{{ .Label }}"></span>{{ end }}
          </div>
          <div class="component-meta"><span class="muted">{{ .WindowStartLabel }}</span><span class="muted">{{ .UptimeDisplay }} uptime · {{ .WindowEndLabel }}</span></div>
          {{- end }}
        </article>
        {{- end }}
      </div>
      {{- else }}
      <p class="empty">No public components are configured.</p>
      {{- end }}
    </section>
    {{- end }}
    </section>
    {{- if .ShowIncidentHistory }}
    <section id="events" class="section" aria-labelledby="incidents">
      <div class="section-header"><h2 id="incidents">Recent public events</h2><a href="{{ .HistoryURL }}">View history</a></div>
      {{- if .Incidents }}
      <div class="incident-list">
        {{- range .Incidents }}
        <article class="incident">
          <div class="row"><p class="name"><a href="{{ .DetailURL }}">{{ .Title }}</a></p><span class="status {{ .StatusClass }}">{{ .PublicStatus }}</span></div>
          {{- if .Components }}<div class="chips" aria-label="Affected components">{{ range .Components }}<span class="chip">{{ . }}</span>{{ end }}</div>{{ end }}
          {{- if .ImpactSummary }}<p class="muted">{{ .ImpactSummary }}</p>{{ end }}
          <p class="muted">Severity {{ .Severity }}{{ if .PublishedAt }} · Published {{ .PublishedAt }}{{ end }}{{ if .ResolvedAt }} · Resolved {{ .ResolvedAt }}{{ end }}</p>
          {{- if .Updates }}
          <div class="timeline">
            {{- range .Updates }}
            <div class="timeline-item"><p><strong>{{ .Status }}</strong>{{ if .PublishedAt }} · <span class="muted">{{ .PublishedAt }}</span>{{ end }}</p><p class="muted">{{ .Message }}</p></div>
            {{- end }}
          </div>
          {{- end }}
        </article>
        {{- end }}
      </div>
      {{- else }}
      <p class="empty">No recent incidents.</p>
      {{- end }}
    </section>
    {{- end }}
    <section id="updates" class="section" aria-labelledby="updates-heading">
      <div class="section-header"><h2 id="updates-heading">Updates and utilities</h2></div>
      <div class="utility-list">
        {{- if .ShowGetUpdates }}<div class="utility"><span>Get notified about public incidents and maintenance.</span><code>{{ .SubscriberURL }}</code></div>{{ end }}
        <div class="utility"><span>{{ .FeedURL }}</span><a href="{{ .FeedURL }}">Atom feed</a></div>
        <div class="utility"><span>{{ .BadgeURL }}</span><a href="{{ .BadgeURL }}">Status badge</a></div>
      </div>
    </section>
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

func (s *Server) writePublicStatusPageHTML(c *gin.Context, detail StatusPageDetailResponse, preview StatusPagePreviewResponse) {
	payload, err := s.renderPublicStatusPageHTML(c, detail, preview)
	if err != nil {
		s.logger.Error("Failed to render public status page HTML", "slug", preview.Page.Slug, "error", err)
		c.Status(http.StatusInternalServerError)
		return
	}
	writePublicStatusPagePayload(c, http.StatusOK, "text/html; charset=utf-8", payload)
}

func (s *Server) renderPublicStatusPageHTML(c *gin.Context, detail StatusPageDetailResponse, preview StatusPagePreviewResponse) ([]byte, error) {
	theme := statusPagePublicHTMLTheme(preview.Page.ThemeSettings)
	publicURL := publicStatusPageHTMLURL(c, preview.Page.Slug)
	routeBaseURL := publicStatusPageHTMLRouteBaseURL(c, preview.Page.Slug)
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
		PublicURL:            publicURL,
		FeedURL:              strings.TrimRight(publicURL, "/") + "/feed.atom",
		BadgeURL:             strings.TrimRight(routeBaseURL, "/") + "/badge.svg",
		HistoryURL:           strings.TrimRight(routeBaseURL, "/") + "/history",
		SubscriberURL:        strings.TrimRight(routeBaseURL, "/") + "/subscribers",
		ShowGetUpdates:       statusPageHTMLHasComponents(preview.Sections),
		ShowUptimeBars:       theme.ShowUptimeSummary,
		ShowIncidentHistory:  theme.ShowIncidentHistory,
	}
	histories := s.publicStatusPageComponentHistories(detail, statusPagePublicDefaultUptimeWindow, true)
	view.Sections = statusPageHTMLSections(routeBaseURL, preview.Sections, histories)
	componentNames := statusPageHTMLComponentNames(view.Sections)
	updates := statusPageHTMLIncidentUpdates(detail.Incidents)
	view.ActiveIncidents, view.Incidents = statusPageHTMLIncidents(routeBaseURL, preview.Incidents, componentNames, updates)

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

func statusPagePublicHTMLTheme(settings map[string]interface{}) statusPagePublicHTMLThemeConfig {
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

func publicStatusPageHTMLRouteBaseURL(c *gin.Context, slug string) string {
	scheme := strings.TrimSpace(c.GetHeader("X-Forwarded-Proto"))
	if scheme == "" {
		scheme = "http"
		if c.Request.TLS != nil {
			scheme = "https"
		}
	}
	return scheme + "://" + c.Request.Host + "/status/" + url.PathEscape(slug)
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
