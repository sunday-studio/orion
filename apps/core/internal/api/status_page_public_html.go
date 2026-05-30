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
	OverallStatusMessage string
	StatusClass          string
	LastUpdated          string
	AccentColor          string
	LogoURL              string
	LogoAlt              string
	HeaderClass          string
	DensityClass         string
	ThemeClass           string
	FeedURL              string
	BadgeURL             string
	HistoryURL           string
	SubscribeURL         string
	ShowUptimeSummary    bool
	ShowIncidentHistory  bool
	HasComponents        bool
	Sections             []statusPageHTMLSection
	ActiveIncidents      []statusPageHTMLIncident
	RecentIncidents      []statusPageHTMLIncident
}

type statusPageHTMLSection struct {
	ID         string
	Name       string
	Status     string
	StatusText string
	Components []statusPageHTMLComponent
}

type statusPageHTMLComponent struct {
	ID            string
	Name          string
	Description   string
	Status        string
	StatusDisplay string
	StatusReason  string
	StatusClass   string
	UptimeDisplay string
	Bars          []statusPageHTMLUptimeBar
	BarCount      int
	WindowStart   string
	WindowEnd     string
}

type statusPageHTMLUptimeBar struct {
	Date    string
	Label   string
	Class   string
	Display string
}

type statusPageHTMLIncident struct {
	ID                 string
	Title              string
	PublicStatus       string
	StatusClass        string
	Severity           string
	ImpactSummary      string
	PublishedAt        string
	ResolvedAt         string
	ScheduledStartAt   string
	ScheduledEndAt     string
	DetailURL          string
	AffectedComponents []string
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
    :root { color-scheme: light; --accent: {{ .AccentColor }}; --bg: #f7f8f5; --panel: #ffffff; --panel-soft: #f1f5f3; --text: #171717; --muted: #737373; --border: #deded8; --success: #58c56c; --degraded: #f2ad3e; --outage: #e86161; --maintenance: #5b8def; --unknown: #a3a3a3; }
    * { box-sizing: border-box; }
    body { margin: 0; background: var(--bg); color: var(--text); font-family: ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; line-height: 1.5; }
    body.theme-dark { color-scheme: dark; --bg: #080808; --panel: #171717; --panel-soft: #202020; --text: #ededed; --muted: #a3a3a3; --border: #2f2f2f; }
    @media (prefers-color-scheme: dark) { body.theme-system { color-scheme: dark; --bg: #080808; --panel: #171717; --panel-soft: #202020; --text: #ededed; --muted: #a3a3a3; --border: #2f2f2f; } }
    a { color: inherit; text-decoration-color: color-mix(in srgb, var(--accent) 65%, transparent); text-underline-offset: 4px; }
    main { max-width: 900px; margin: 0 auto; padding: 28px 20px 60px; }
    .topbar { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 18px; align-items: center; margin-bottom: 34px; }
    .brand { display: flex; align-items: center; gap: 12px; min-width: 0; }
    .brand img { max-width: 132px; max-height: 42px; object-fit: contain; }
    .brand-text { min-width: 0; }
    h1 { margin: 0; font-size: 1.45rem; line-height: 1.2; letter-spacing: 0; }
    .description { margin-top: 2px; color: var(--muted); font-size: .98rem; }
    .nav { display: flex; align-items: center; justify-content: flex-end; gap: 8px; flex-wrap: wrap; }
    .nav a, .button { border: 1px solid var(--border); background: var(--panel); border-radius: 8px; color: var(--text); display: inline-flex; font-weight: 650; gap: 8px; line-height: 1; padding: 10px 14px; text-decoration: none; }
    .nav a[aria-current="page"], .button.primary { background: var(--panel-soft); border-color: color-mix(in srgb, var(--accent) 40%, var(--border)); }
    .summary { border: 1px solid color-mix(in srgb, var(--status-color) 65%, var(--border)); background: color-mix(in srgb, var(--status-color) 14%, var(--panel)); border-radius: 8px; display: grid; grid-template-columns: auto minmax(0, 1fr) auto; gap: 16px; align-items: center; margin-bottom: 28px; padding: 16px 18px; }
    .status-icon { align-items: center; background: var(--status-color); border-radius: 999px; color: #071007; display: inline-flex; font-weight: 800; height: 34px; justify-content: center; width: 34px; }
    .summary strong { display: block; font-size: 1.35rem; letter-spacing: 0; line-height: 1.2; }
    .summary p, .summary time { color: var(--muted); font-size: .9rem; }
    .status-operational { --status-color: var(--success); }
    .status-degraded { --status-color: var(--degraded); }
    .status-partial_outage, .status-major_outage, .status-outage { --status-color: var(--outage); }
    .status-maintenance { --status-color: var(--maintenance); }
    .status-unknown, .status-no_data { --status-color: var(--unknown); }
    .section { margin-top: 26px; }
    .section-head, .component-head, .incident-head { align-items: center; display: flex; gap: 12px; justify-content: space-between; }
    h2 { font-size: 1.05rem; margin: 0 0 12px; letter-spacing: 0; }
    .section-status, .pill { align-items: center; border-radius: 999px; color: var(--muted); display: inline-flex; font-size: .84rem; gap: 6px; white-space: nowrap; }
    .component-list, .incident-list { display: grid; gap: 14px; }
    .component { border-bottom: 1px solid var(--border); padding: 0 0 18px; }
    .component:last-child { border-bottom: 0; }
    .compact-density .component-list { gap: 8px; }
    .compact-density .component { padding-bottom: 12px; }
    .name { font-weight: 700; min-width: 0; overflow-wrap: anywhere; }
    .muted { color: var(--muted); font-size: .9rem; }
    .uptime-meta { align-items: center; color: var(--muted); display: flex; font-size: .86rem; justify-content: space-between; gap: 10px; margin-top: 6px; }
    .uptime-value { color: var(--text); font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; white-space: nowrap; }
    .uptime-bars { display: grid; grid-template-columns: repeat(var(--bars), minmax(1px, 1fr)); gap: 2px; margin-top: 10px; min-width: 0; }
    .bar { background: var(--status-color); border: 0; border-radius: 999px; height: 34px; min-width: 1px; padding: 0; }
    .compact-density .bar { border-radius: 3px; height: 24px; }
    .bar:focus-visible { outline: 2px solid var(--accent); outline-offset: 2px; }
    .incident { background: var(--panel); border: 1px solid var(--border); border-radius: 8px; padding: 14px 16px; }
    .incident-title { font-weight: 750; overflow-wrap: anywhere; }
    .chips { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 10px; }
    .chip { background: var(--panel-soft); border-radius: 999px; color: var(--muted); font-size: .78rem; padding: 3px 8px; }
    .empty { border: 1px dashed var(--border); border-radius: 8px; color: var(--muted); padding: 14px 16px; }
    .subscribe { background: var(--panel); border: 1px solid var(--border); border-radius: 8px; display: grid; gap: 12px; margin-top: 32px; padding: 16px; }
    .subscribe form { display: grid; gap: 10px; }
    .subscribe-row { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 8px; }
    input[type="email"] { background: var(--bg); border: 1px solid var(--border); border-radius: 8px; color: var(--text); font: inherit; min-width: 0; padding: 10px 12px; }
    .component-prefs { display: flex; flex-wrap: wrap; gap: 8px 14px; }
    .component-prefs label { color: var(--muted); font-size: .86rem; }
    .form-message { color: var(--muted); font-size: .88rem; min-height: 1.3em; }
    footer { border-top: 1px solid var(--border); color: var(--muted); display: flex; flex-wrap: wrap; gap: 12px; justify-content: space-between; margin-top: 36px; padding-top: 18px; font-size: .88rem; }
    @media (max-width: 720px) {
      main { padding: 22px 14px 44px; }
      .topbar { grid-template-columns: 1fr; margin-bottom: 24px; }
      .nav { justify-content: flex-start; }
      .summary { grid-template-columns: auto minmax(0, 1fr); }
      .summary time { grid-column: 1 / -1; }
      .summary strong { font-size: 1.1rem; }
      .component-head, .incident-head { align-items: flex-start; flex-direction: column; gap: 4px; }
      .uptime-bars { gap: 1px; }
      .bar { height: 26px; }
      .subscribe-row { grid-template-columns: 1fr; }
    }
  </style>
</head>
<body class="{{ .ThemeClass }}">
  <main class="{{ .DensityClass }}">
    <header class="topbar {{ .HeaderClass }}">
      <div class="brand">
        {{- if .LogoURL }}
        <img src="{{ .LogoURL }}" alt="{{ .LogoAlt }}">
        {{- end }}
        <div class="brand-text">
          <h1>{{ .PageTitle }}</h1>
          {{- if .PageDescription }}
          <p class="description">{{ .PageDescription }}</p>
          {{- end }}
        </div>
      </div>
      <nav class="nav" aria-label="Status page navigation">
        <a aria-current="page" href="#status">Status</a>
        <a href="#events">Events</a>
        <a href="#components">Components</a>
        <a class="button primary" href="#updates">Get updates</a>
      </nav>
    </header>

    <section id="status" class="summary {{ .StatusClass }}" aria-label="Current status" aria-live="polite">
      <span class="status-icon" aria-hidden="true">✓</span>
      <div>
        <strong>{{ .OverallStatusMessage }}</strong>
        <p>{{ .OverallStatusDisplay }}</p>
      </div>
      <time datetime="{{ .LastUpdated }}">{{ .LastUpdated }}</time>
    </section>

    {{- if .ActiveIncidents }}
    <section class="section" aria-labelledby="active-events">
      <h2 id="active-events">Active events</h2>
      <div class="incident-list">
        {{- range .ActiveIncidents }}
        <article class="incident {{ .StatusClass }}">
          <div class="incident-head">
            <a class="incident-title" href="{{ .DetailURL }}">{{ .Title }}</a>
            <span class="pill"><span class="status-icon" aria-hidden="true" style="height:16px;width:16px;font-size:.7rem">!</span>{{ .PublicStatus }}</span>
          </div>
          {{- if .ImpactSummary }}<p class="muted">{{ .ImpactSummary }}</p>{{ end }}
          <p class="muted">{{ if .PublishedAt }}Published {{ .PublishedAt }}{{ end }}{{ if .ScheduledStartAt }} Scheduled {{ .ScheduledStartAt }}{{ end }}</p>
          {{- if .AffectedComponents }}
          <div class="chips" aria-label="Affected components">{{ range .AffectedComponents }}<span class="chip">{{ . }}</span>{{ end }}</div>
          {{- end }}
        </article>
        {{- end }}
      </div>
    </section>
    {{- end }}

    {{- if .ShowUptimeSummary }}
    <section id="components" class="section" aria-labelledby="components-title">
      <h2 id="components-title">Components</h2>
      {{- if .HasComponents }}
      {{- range .Sections }}
      {{- if .Components }}
      <section class="section" aria-labelledby="section-{{ .ID }}">
        <div class="section-head">
          <h2 id="section-{{ .ID }}">{{ .Name }}</h2>
          <span class="section-status {{ .Status }}">{{ .StatusText }}</span>
        </div>
        <div class="component-list">
          {{- range .Components }}
          <article class="component {{ .StatusClass }}">
            <div class="component-head">
              <div>
                <p class="name">{{ .Name }}</p>
                {{- if .Description }}<p class="muted">{{ .Description }}</p>{{ end }}
              </div>
              <span class="pill"><span class="status-icon" aria-hidden="true" style="height:16px;width:16px;font-size:.7rem">✓</span>{{ .StatusDisplay }} · <span class="uptime-value">{{ .UptimeDisplay }}</span></span>
            </div>
            {{- if .StatusReason }}<p class="muted">{{ .StatusReason }}</p>{{ end }}
            {{- if .Bars }}
            <div class="uptime-bars" style="--bars: {{ .BarCount }}" role="list" aria-label="{{ .Name }} uptime history">
              {{- range .Bars }}
              <button class="bar {{ .Class }}" type="button" role="listitem" aria-label="{{ .Label }}" title="{{ .Label }}"></button>
              {{- end }}
            </div>
            <div class="uptime-meta"><span>{{ .WindowStart }}</span><span>{{ .WindowEnd }}</span></div>
            {{- end }}
          </article>
          {{- end }}
        </div>
      </section>
      {{- end }}
      {{- end }}
      {{- else }}
      <p class="empty">No public components are configured.</p>
      {{- end }}
    </section>
    {{- end }}

    {{- if .ShowIncidentHistory }}
    <section id="events" class="section" aria-labelledby="events-title">
      <h2 id="events-title">Recent events</h2>
      {{- if .RecentIncidents }}
      <div class="incident-list">
        {{- range .RecentIncidents }}
        <article class="incident {{ .StatusClass }}">
          <div class="incident-head">
            <a class="incident-title" href="{{ .DetailURL }}">{{ .Title }}</a>
            <span class="pill">{{ .PublicStatus }}</span>
          </div>
          {{- if .ImpactSummary }}<p class="muted">{{ .ImpactSummary }}</p>{{ end }}
          <p class="muted">{{ if .PublishedAt }}Published {{ .PublishedAt }}{{ end }}{{ if .ResolvedAt }} · Resolved {{ .ResolvedAt }}{{ end }}</p>
          {{- if .AffectedComponents }}
          <div class="chips" aria-label="Affected components">{{ range .AffectedComponents }}<span class="chip">{{ . }}</span>{{ end }}</div>
          {{- end }}
        </article>
        {{- end }}
      </div>
      {{- else }}
      <p class="empty">No recent incidents.</p>
      {{- end }}
    </section>
    {{- end }}

    <section id="updates" class="subscribe" aria-labelledby="updates-title">
      <div>
        <h2 id="updates-title">Get updates</h2>
        <p class="muted">Subscribe to public incident updates for this page.</p>
      </div>
      <form data-subscribe-form data-endpoint="{{ .SubscribeURL }}">
        <div class="subscribe-row">
          <input name="destination" type="email" autocomplete="email" required placeholder="you@example.com" aria-label="Email address">
          <button class="button primary" type="submit">Subscribe</button>
        </div>
        {{- if .HasComponents }}
        <div class="component-prefs" aria-label="Component preferences">
          {{- range .Sections }}{{- range .Components }}
          <label><input type="checkbox" name="component_ids" value="{{ .ID }}"> {{ .Name }}</label>
          {{- end }}{{- end }}
        </div>
        {{- end }}
        <p class="form-message" role="status" aria-live="polite"></p>
      </form>
    </section>

    <footer>
      <span>Powered by Orion public status.</span>
      <span>{{ if .FeedURL }}<a href="{{ .FeedURL }}">Atom feed</a>{{ end }}{{ if .BadgeURL }} · <a href="{{ .BadgeURL }}">Status badge</a>{{ end }}{{ if .HistoryURL }} · <a href="{{ .HistoryURL }}">History</a>{{ end }}</span>
    </footer>
  </main>
  <script>
    for (const form of document.querySelectorAll("[data-subscribe-form]")) {
      form.addEventListener("submit", async (event) => {
        event.preventDefault();
        const data = new FormData(form);
        const message = form.querySelector(".form-message");
        const componentIds = data.getAll("component_ids").map(String);
        message.textContent = "Submitting...";
        try {
          const response = await fetch(form.dataset.endpoint, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ destination_type: "email", destination: String(data.get("destination") || ""), component_ids: componentIds }),
          });
          message.textContent = response.ok ? "Check your email to confirm updates." : "Subscription could not be started.";
          if (response.ok) form.reset();
        } catch {
          message.textContent = "Subscription could not be started.";
        }
      });
    }
  </script>
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
	canonicalURL := firstNonEmpty(strings.TrimSpace(preview.Metadata.CanonicalURL), publicURL)
	openGraphURL := firstNonEmpty(strings.TrimSpace(preview.Metadata.OpenGraph.URL), canonicalURL)
	sections := statusPageHTMLSections(preview.Sections)

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
		OverallStatusMessage: publicStatusPageHTMLSummary(preview.OverallStatus),
		StatusClass:          statusPageHTMLStatusClass(preview.OverallStatus),
		LastUpdated:          publicStatusPageHTMLTime(preview.LastUpdated),
		AccentColor:          theme.AccentColor,
		LogoURL:              theme.LogoURL,
		LogoAlt:              theme.LogoAlt,
		HeaderClass:          theme.HeaderClass,
		DensityClass:         theme.DensityClass,
		ThemeClass:           theme.ThemeClass,
		FeedURL:              strings.TrimRight(publicURL, "/") + "/feed.atom",
		BadgeURL:             strings.TrimRight(publicURL, "/") + "/badge.svg",
		HistoryURL:           strings.TrimRight(publicURL, "/") + "/history",
		SubscribeURL:         "/status/" + url.PathEscape(preview.Page.Slug) + "/subscribers",
		ShowUptimeSummary:    theme.ShowUptimeSummary,
		ShowIncidentHistory:  theme.ShowIncidentHistory,
		HasComponents:        statusPageHTMLHasComponents(sections),
		Sections:             sections,
		ActiveIncidents:      statusPageHTMLIncidents(publicURL, preview.Incidents, sections, true),
		RecentIncidents:      statusPageHTMLIncidents(publicURL, preview.Incidents, sections, false),
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
	ThemeClass          string
	ShowUptimeSummary   bool
	ShowIncidentHistory bool
}

func statusPagePublicHTMLTheme(settings map[string]interface{}) statusPagePublicHTMLThemeConfig {
	theme := statusPagePublicHTMLThemeConfig{
		AccentColor:         "#2563eb",
		HeaderClass:         "standard",
		ThemeClass:          "theme-light",
		ShowUptimeSummary:   true,
		ShowIncidentHistory: true,
	}
	settings = safeStatusPagePublicThemeSettings(settings)
	if value, ok := settings["accent_color"].(string); ok && validStatusPageThemeHexColor(value) {
		theme.AccentColor = strings.ToLower(value)
	}
	if value, ok := settings["logo_url"].(string); ok {
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
	if value, ok := settings["theme_mode"].(string); ok {
		switch value {
		case "dark":
			theme.ThemeClass = "theme-dark"
		case "system":
			theme.ThemeClass = "theme-system"
		}
	}
	if value, ok := settings["show_uptime_summary"].(bool); ok {
		theme.ShowUptimeSummary = value
	}
	if value, ok := settings["show_incident_history"].(bool); ok {
		theme.ShowIncidentHistory = value
	}
	return theme
}

func statusPageHTMLSections(sections []StatusPagePublicSectionResponse) []statusPageHTMLSection {
	responses := make([]statusPageHTMLSection, 0, len(sections))
	for _, section := range sections {
		components := make([]statusPageHTMLComponent, 0, len(section.Components))
		sectionStatus := "operational"
		for _, component := range section.Components {
			if statusPageStatusWeight(component.Status) > statusPageStatusWeight(sectionStatus) {
				sectionStatus = component.Status
			}
			components = append(components, statusPageHTMLComponent{
				ID:            component.ID,
				Name:          component.Name,
				Description:   component.Description,
				Status:        component.Status,
				StatusDisplay: component.StatusDisplay,
				StatusReason:  component.StatusReason,
				StatusClass:   statusPageHTMLStatusClass(component.Status),
				UptimeDisplay: statusPageHTMLUptimeDisplay(component.Uptime),
				Bars:          statusPageHTMLUptimeBars(component.UptimeHistory),
				BarCount:      len(component.UptimeHistory),
				WindowStart:   statusPageHTMLWindowStart(component.UptimeHistory),
				WindowEnd:     "today",
			})
		}
		responses = append(responses, statusPageHTMLSection{
			ID:         section.ID,
			Name:       section.Name,
			Status:     statusPageHTMLStatusClass(sectionStatus),
			StatusText: publicStatusDisplay(sectionStatus),
			Components: components,
		})
	}
	return responses
}

func statusPageHTMLUptimeBars(history []StatusPagePublicUptimeBucketResponse) []statusPageHTMLUptimeBar {
	bars := make([]statusPageHTMLUptimeBar, 0, len(history))
	for _, bucket := range history {
		status := bucket.Status
		if status == "" {
			status = publicUptimeStatus(bucket.UptimeRatio)
		}
		bars = append(bars, statusPageHTMLUptimeBar{
			Date:    bucket.Date,
			Label:   bucket.Date + ": " + publicStatusPageHTMLBucketLabel(status, bucket.UptimeDisplay),
			Class:   statusPageHTMLStatusClass(status),
			Display: bucket.UptimeDisplay,
		})
	}
	return bars
}

func statusPageHTMLIncidents(publicURL string, incidents []StatusPagePublicIncidentResponse, sections []statusPageHTMLSection, active bool) []statusPageHTMLIncident {
	names := statusPageHTMLComponentNames(sections)
	responses := []statusPageHTMLIncident{}
	base := strings.TrimRight(publicURL, "/")
	for _, incident := range incidents {
		isActive := incident.PublicStatus != "resolved"
		if isActive != active {
			continue
		}
		responses = append(responses, statusPageHTMLIncident{
			ID:                 incident.ID,
			Title:              incident.Title,
			PublicStatus:       publicIncidentStatusDisplay(incident.PublicStatus),
			StatusClass:        statusPageHTMLStatusClass(publicStatusFromIncident(incident.PublicStatus)),
			Severity:           incident.Severity,
			ImpactSummary:      incident.ImpactSummary,
			PublishedAt:        publicStatusPageHTMLTimePtr(incident.PublishedAt),
			ResolvedAt:         publicStatusPageHTMLTimePtr(incident.ResolvedAt),
			ScheduledStartAt:   publicStatusPageHTMLTimePtr(incident.ScheduledStartAt),
			ScheduledEndAt:     publicStatusPageHTMLTimePtr(incident.ScheduledEndAt),
			DetailURL:          base + "/incidents/" + url.PathEscape(incident.ID),
			AffectedComponents: statusPageHTMLAffectedComponents(incident.AffectedComponentIDs, names),
		})
	}
	return responses
}

func statusPageHTMLComponentNames(sections []statusPageHTMLSection) map[string]string {
	names := map[string]string{}
	for _, section := range sections {
		for _, component := range section.Components {
			names[component.ID] = component.Name
		}
	}
	return names
}

func statusPageHTMLAffectedComponents(ids []string, names map[string]string) []string {
	affected := []string{}
	for _, id := range ids {
		if name := names[id]; name != "" {
			affected = append(affected, name)
		}
	}
	return affected
}

func statusPageHTMLHasComponents(sections []statusPageHTMLSection) bool {
	for _, section := range sections {
		if len(section.Components) > 0 {
			return true
		}
	}
	return false
}

func statusPageHTMLStatusClass(status string) string {
	if status == "" {
		status = "unknown"
	}
	return "status-" + status
}

func statusPageHTMLUptimeDisplay(uptime *StatusPagePublicUptimeResponse) string {
	if uptime == nil || uptime.UptimeDisplay == "" {
		return statusPagePublicNoDataDisplay
	}
	return uptime.UptimeDisplay
}

func statusPageHTMLWindowStart(history []StatusPagePublicUptimeBucketResponse) string {
	if len(history) == 0 {
		return ""
	}
	return history[0].Date
}

func publicStatusPageHTMLBucketLabel(status string, display string) string {
	label := publicStatusDisplay(status)
	if status == "no_data" {
		label = statusPagePublicNoDataDisplay
	}
	if display == "" {
		return label
	}
	return label + ", " + display
}

func publicStatusFromIncident(status string) string {
	switch status {
	case "resolved":
		return "operational"
	case "scheduled", "monitoring":
		return "maintenance"
	case "identified":
		return "degraded"
	case "investigating":
		return "partial_outage"
	default:
		return "unknown"
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
