package api

import (
	"net/url"
	"strings"
)

func statusPageHTMLSections(publicURL string, sections []StatusPagePublicSectionResponse, histories []StatusPagePublicComponentHistoryResponse) []statusPageHTMLSection {
	historyByComponent := make(map[string]StatusPagePublicComponentHistoryResponse, len(histories))
	for _, history := range histories {
		historyByComponent[history.Component.ID] = history
	}
	responses := make([]statusPageHTMLSection, 0, len(sections))
	base := strings.TrimRight(publicURL, "/")
	for _, section := range sections {
		response := statusPageHTMLSection{
			ID:            section.ID,
			Name:          section.Name,
			Status:        "operational",
			StatusDisplay: publicStatusDisplay("operational"),
			Components:    make([]statusPageHTMLComponent, 0, len(section.Components)),
		}
		if len(section.Components) == 0 {
			responses = append(responses, response)
			continue
		}
		sectionStatus := "operational"
		for _, component := range section.Components {
			if statusPageStatusWeight(component.Status) > statusPageStatusWeight(sectionStatus) {
				sectionStatus = component.Status
			}
			history := historyByComponent[component.ID]
			response.Components = append(response.Components, statusPageHTMLComponent{
				ID:               component.ID,
				Name:             component.Name,
				Description:      component.Description,
				Status:           component.Status,
				StatusDisplay:    component.StatusDisplay,
				StatusReason:     component.StatusReason,
				UptimeDisplay:    firstNonEmpty(history.Uptime.UptimeDisplay, statusPagePublicNoDataDisplay),
				HistoryURL:       base + "/components/" + url.PathEscape(component.ID) + "/history",
				BadgeURL:         base + "/components/" + url.PathEscape(component.ID) + "/badge.svg",
				WindowStartLabel: statusPageHTMLWindowStartLabel(statusPagePublicDefaultUptimeWindow),
				WindowEndLabel:   "today",
				Bars:             statusPageHTMLUptimeBars(history.History),
			})
		}
		response.Status = sectionStatus
		response.StatusDisplay = publicStatusDisplay(sectionStatus)
		responses = append(responses, response)
	}
	return responses
}

func statusPageHTMLHasComponents(sections []StatusPagePublicSectionResponse) bool {
	for _, section := range sections {
		if len(section.Components) > 0 {
			return true
		}
	}
	return false
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

func statusPageHTMLUptimeBars(history []StatusPagePublicUptimeBucketResponse) []statusPageHTMLUptimeBar {
	bars := make([]statusPageHTMLUptimeBar, 0, len(history))
	for _, bucket := range history {
		state := statusPageHTMLUptimeClass(bucket.UptimeRatio)
		label := bucket.Date + " · " + firstNonEmpty(bucket.UptimeDisplay, statusPagePublicNoDataDisplay)
		bars = append(bars, statusPageHTMLUptimeBar{Class: state, Label: label})
	}
	return bars
}

func statusPageHTMLUptimeClass(ratio *float64) string {
	if ratio == nil {
		return "no-data"
	}
	switch {
	case *ratio >= 0.999:
		return "operational"
	case *ratio >= 0.95:
		return "degraded"
	default:
		return "outage"
	}
}

func statusPageHTMLWindowStartLabel(window string) string {
	switch window {
	case "24h":
		return "24 hours ago"
	case "7d":
		return "7 days ago"
	case "30d":
		return "30 days ago"
	default:
		return "90 days ago"
	}
}

func statusPageHTMLIncidentUpdates(incidents []StatusPageIncidentResponse) map[string][]statusPageHTMLIncidentUpdate {
	responses := map[string][]statusPageHTMLIncidentUpdate{}
	for _, incident := range incidents {
		for _, update := range incident.Updates {
			if update.PublishedAt == nil {
				continue
			}
			responses[incident.ID] = append(responses[incident.ID], statusPageHTMLIncidentUpdate{
				Status:      publicIncidentStatusDisplay(update.Status),
				StatusClass: update.Status,
				Message:     update.Message,
				PublishedAt: publicStatusPageHTMLTimePtr(update.PublishedAt),
			})
		}
	}
	return responses
}

func statusPageHTMLIncidents(publicURL string, incidents []StatusPagePublicIncidentResponse, componentNames map[string]string, updates map[string][]statusPageHTMLIncidentUpdate) ([]statusPageHTMLIncident, []statusPageHTMLIncident) {
	active := make([]statusPageHTMLIncident, 0)
	recent := make([]statusPageHTMLIncident, 0, len(incidents))
	base := strings.TrimRight(publicURL, "/")
	for _, incident := range incidents {
		response := statusPageHTMLIncident{
			ID:            incident.ID,
			Title:         incident.Title,
			PublicStatus:  publicIncidentStatusDisplay(incident.PublicStatus),
			StatusClass:   incident.PublicStatus,
			Severity:      incident.Severity,
			ImpactSummary: incident.ImpactSummary,
			PublishedAt:   publicStatusPageHTMLTimePtr(incident.PublishedAt),
			ResolvedAt:    publicStatusPageHTMLTimePtr(incident.ResolvedAt),
			ScheduledAt:   publicStatusPageHTMLTimePtr(incident.ScheduledStartAt),
			DetailURL:     base + "/incidents/" + url.PathEscape(incident.ID),
			Components:    statusPageHTMLIncidentComponents(incident.AffectedComponentIDs, componentNames),
			Updates:       updates[incident.ID],
		}
		if incident.PublicStatus == "resolved" {
			recent = append(recent, response)
			continue
		}
		active = append(active, response)
	}
	return active, recent
}

func statusPageHTMLIncidentComponents(componentIDs []string, componentNames map[string]string) []string {
	components := make([]string, 0, len(componentIDs))
	for _, componentID := range componentIDs {
		if name := strings.TrimSpace(componentNames[componentID]); name != "" {
			components = append(components, name)
		}
	}
	return components
}
