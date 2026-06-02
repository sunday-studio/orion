package service

import (
	"encoding/json"
	"orion/core/internal/db"
	"strings"
)

func (s *AlertService) LoadAlertRouteContext(incidentID string, eventType string) (*AlertRouteContext, error) {
	var incident db.Incident
	if err := s.db.Where("id = ?", incidentID).First(&incident).Error; err != nil {
		return nil, err
	}

	var monitor db.Monitor
	if err := s.db.Where("id = ?", incident.MonitorID).First(&monitor).Error; err != nil {
		monitor = db.Monitor{ID: incident.MonitorID, AgentID: incident.AgentID}
	}

	return &AlertRouteContext{
		IncidentID:  incident.ID,
		EventType:   eventType,
		Severity:    incident.Severity,
		AgentID:     incident.AgentID,
		MonitorID:   incident.MonitorID,
		MonitorType: monitor.Type,
	}, nil
}

func (s *AlertService) DryRunRoutes(event AlertRouteContext) (*AlertRouteDryRunResult, error) {
	routes, err := s.alertRoutes()
	if err != nil {
		return nil, err
	}
	if len(routes) == 0 {
		return s.evaluateLegacyFallback(event)
	}
	return s.evaluateRoutes(event, routes)
}

func (s *AlertService) evaluateLegacyFallback(event AlertRouteContext) (*AlertRouteDryRunResult, error) {
	channels, err := s.deliveryChannels()
	if err != nil {
		return nil, err
	}
	result := &AlertRouteDryRunResult{
		Event:          event,
		LegacyFallback: true,
	}
	if len(channels) == 0 {
		result.Suppressed = true
		result.SuppressionReason = "no alert channels configured"
		return result, nil
	}
	for _, channel := range channels {
		result.DestinationDecisions = append(result.DestinationDecisions, s.channelDecision(event, db.AlertRoute{}, channel, false, "legacy fallback: no alert routes configured"))
	}
	return result, nil
}

func (s *AlertService) evaluateRoutes(event AlertRouteContext, routes []db.AlertRoute) (*AlertRouteDryRunResult, error) {
	channels, err := s.deliveryChannels()
	if err != nil {
		return nil, err
	}
	channelsByID := map[string]db.AlertChannel{}
	for _, channel := range channels {
		channelsByID[channel.ID] = channel
	}

	result := &AlertRouteDryRunResult{Event: event}
	var suppressingRoute *db.AlertRoute
	for _, route := range routes {
		matched, reasons := routeMatchesEvent(route, event)
		evaluation := AlertRouteEvaluation{
			Route:      route,
			Matched:    matched,
			Suppressed: matched && route.Enabled && route.Suppress,
			Reasons:    reasons,
		}
		if evaluation.Suppressed && suppressingRoute == nil {
			copyRoute := route
			suppressingRoute = &copyRoute
			result.Suppressed = true
			result.SuppressionReason = "alert route suppressed event: " + route.Name
		}
		result.RouteEvaluations = append(result.RouteEvaluations, evaluation)
	}

	for _, evaluation := range result.RouteEvaluations {
		if !evaluation.Matched || !evaluation.Route.Enabled || evaluation.Route.Suppress {
			continue
		}
		for _, channelID := range decodeStringList(evaluation.Route.ChannelIDs) {
			channel, ok := channelsByID[channelID]
			if !ok {
				result.DestinationDecisions = append(result.DestinationDecisions, AlertDestinationDecision{
					RouteID:     evaluation.Route.ID,
					RouteName:   evaluation.Route.Name,
					ChannelID:   channelID,
					ChannelName: channelID,
					ChannelType: "unknown",
					Status:      "suppressed",
					Reason:      "alert route destination missing",
				})
				continue
			}
			suppressedByRoute := suppressingRoute != nil
			reason := "matched alert route: " + evaluation.Route.Name
			if suppressedByRoute {
				reason = "suppressed by alert route: " + suppressingRoute.Name
			}
			result.DestinationDecisions = append(result.DestinationDecisions, s.channelDecision(event, evaluation.Route, channel, suppressedByRoute, reason))
		}
	}

	return result, nil
}

func (s *AlertService) channelDecision(event AlertRouteContext, route db.AlertRoute, channel db.AlertChannel, suppressedByRoute bool, reason string) AlertDestinationDecision {
	decision := AlertDestinationDecision{
		RouteID:     route.ID,
		RouteName:   route.Name,
		ChannelID:   channel.ID,
		ChannelName: channel.Name,
		ChannelType: channel.Type,
		Status:      "pending",
		Reason:      reason,
	}
	switch {
	case suppressedByRoute:
		decision.Status = "suppressed"
	case !channel.Enabled:
		decision.Status = "suppressed"
		decision.Reason = "alert channel disabled"
	case !subscribesToAlertEvent(channel, event.EventType):
		decision.Status = "suppressed"
		decision.Reason = "alert channel is not subscribed to event"
	case s.inCooldown(event.IncidentID, channel.Name, event.EventType, ""):
		decision.Status = "cooldown"
		decision.Reason = "alert cooldown active"
	}
	return decision
}

func routeMatchesEvent(route db.AlertRoute, event AlertRouteContext) (bool, []string) {
	reasons := []string{}
	if !route.Enabled {
		return false, []string{"route disabled"}
	}

	matched := true
	if listContains(decodeAlertRouteEvents(route.EventTypes), event.EventType) {
		reasons = append(reasons, "event matched")
	} else {
		reasons = append(reasons, "event did not match")
		matched = false
	}
	if !filterMatches(route.Severities, event.Severity) {
		reasons = append(reasons, "severity did not match")
		matched = false
	} else if len(decodeStringList(route.Severities)) > 0 {
		reasons = append(reasons, "severity matched")
	}
	if !filterMatches(route.AgentIDs, event.AgentID) {
		reasons = append(reasons, "agent did not match")
		matched = false
	} else if len(decodeStringList(route.AgentIDs)) > 0 {
		reasons = append(reasons, "agent matched")
	}
	if !filterMatches(route.MonitorIDs, event.MonitorID) {
		reasons = append(reasons, "monitor did not match")
		matched = false
	} else if len(decodeStringList(route.MonitorIDs)) > 0 {
		reasons = append(reasons, "monitor matched")
	}
	if !filterMatches(route.MonitorTypes, event.MonitorType) {
		reasons = append(reasons, "monitor type did not match")
		matched = false
	} else if len(decodeStringList(route.MonitorTypes)) > 0 {
		reasons = append(reasons, "monitor type matched")
	}
	return matched, reasons
}

func filterMatches(encoded string, value string) bool {
	values := decodeStringList(encoded)
	if len(values) == 0 {
		return true
	}
	return listContains(values, value)
}

func decodeAlertRouteEvents(encoded string) []string {
	values := decodeStringList(encoded)
	if len(values) == 0 {
		return db.DefaultAlertEvents()
	}
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		if db.ValidAlertEvent(value) {
			filtered = append(filtered, value)
		}
	}
	if len(filtered) == 0 {
		return db.DefaultAlertEvents()
	}
	return filtered
}

func decodeStringList(encoded string) []string {
	if strings.TrimSpace(encoded) == "" {
		return nil
	}
	var values []string
	if err := json.Unmarshal([]byte(encoded), &values); err != nil {
		return nil
	}
	normalized := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		normalized = append(normalized, value)
	}
	return normalized
}

func listContains(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

func suppressingRouteID(plan *AlertRouteDryRunResult) string {
	for _, evaluation := range plan.RouteEvaluations {
		if evaluation.Suppressed {
			return evaluation.Route.ID
		}
	}
	return ""
}
