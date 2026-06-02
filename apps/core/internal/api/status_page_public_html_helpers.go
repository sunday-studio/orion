package api

import (
	"github.com/gin-gonic/gin"
	"net/url"
	"strings"
	"time"
)

func statusPagePublicHTMLTheme(settings map[string]any) statusPagePublicHTMLThemeConfig {
	theme := statusPagePublicHTMLThemeConfig{AccentColor: "#2563eb", HeaderClass: "standard", ThemeClass: "theme-light", ShowUptimeSummary: true, ShowIncidentHistory: true}
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
			components = append(components, statusPageHTMLComponent{ID: component.ID, Name: component.Name, Description: component.Description, Status: component.Status, StatusDisplay: component.StatusDisplay, StatusReason: component.StatusReason, StatusClass: statusPageHTMLStatusClass(component.Status), UptimeDisplay: statusPageHTMLUptimeDisplay(component.Uptime), Bars: statusPageHTMLUptimeBars(component.UptimeHistory), BarCount: len(component.UptimeHistory), WindowStart: statusPageHTMLWindowStart(component.UptimeHistory), WindowEnd: "today"})
		}
		responses = append(responses, statusPageHTMLSection{ID: section.ID, Name: section.Name, Status: statusPageHTMLStatusClass(sectionStatus), StatusText: publicStatusDisplay(sectionStatus), Components: components})
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
		bars = append(bars, statusPageHTMLUptimeBar{Date: bucket.Date, Label: bucket.Date + ": " + publicStatusPageHTMLBucketLabel(status, bucket.UptimeDisplay), Class: statusPageHTMLStatusClass(status), Display: bucket.UptimeDisplay})
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
		responses = append(responses, statusPageHTMLIncident{ID: incident.ID, Title: incident.Title, PublicStatus: publicIncidentStatusDisplay(incident.PublicStatus), StatusClass: statusPageHTMLStatusClass(publicStatusFromIncident(incident.PublicStatus)), Severity: incident.Severity, ImpactSummary: incident.ImpactSummary, PublishedAt: publicStatusPageHTMLTimePtr(incident.PublishedAt), ResolvedAt: publicStatusPageHTMLTimePtr(incident.ResolvedAt), ScheduledStartAt: publicStatusPageHTMLTimePtr(incident.ScheduledStartAt), ScheduledEndAt: publicStatusPageHTMLTimePtr(incident.ScheduledEndAt), DetailURL: base + "/incidents/" + url.PathEscape(incident.ID), AffectedComponents: statusPageHTMLAffectedComponents(incident.AffectedComponentIDs, names)})
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
