package api

import (
	"encoding/json"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"strings"
)

func alertRuleDryRunResponse(result *service.AlertRouteDryRunResult) AlertRuleDryRunResponse {
	evaluations := make([]AlertRuleEvaluationResponse, 0, len(result.RouteEvaluations))
	for _, evaluation := range result.RouteEvaluations {
		evaluations = append(evaluations, AlertRuleEvaluationResponse{Rule: alertRuleResponse(evaluation.Route), Matched: evaluation.Matched, Suppressed: evaluation.Suppressed, Reasons: evaluation.Reasons})
	}
	return AlertRuleDryRunResponse{Event: alertRuleDryRunContext(result.Event), LegacyFallback: result.LegacyFallback, Suppressed: result.Suppressed, SuppressionReason: result.SuppressionReason, RuleEvaluations: evaluations, DestinationDecisions: alertRuleDestinationDecisions(result.DestinationDecisions)}
}
func alertRuleDryRunContext(event service.AlertRouteContext) AlertRuleDryRunContext {
	return AlertRuleDryRunContext{IncidentID: event.IncidentID, EventType: event.EventType, Severity: event.Severity, AgentID: event.AgentID, MonitorID: event.MonitorID, MonitorType: event.MonitorType}
}
func alertRuleDestinationDecisions(decisions []service.AlertDestinationDecision) []AlertRuleDestinationDecision {
	responses := make([]AlertRuleDestinationDecision, 0, len(decisions))
	for _, decision := range decisions {
		responses = append(responses, AlertRuleDestinationDecision{RuleID: decision.RouteID, RuleName: decision.RouteName, ChannelID: decision.ChannelID, ChannelName: decision.ChannelName, ChannelType: decision.ChannelType, Status: decision.Status, Reason: decision.Reason})
	}
	return responses
}
func decodeResponseList(value string, fallback []string) []string {
	if value == "" {
		return fallback
	}
	var values []string
	if err := json.Unmarshal([]byte(value), &values); err != nil || len(values) == 0 {
		return fallback
	}
	return values
}
func alertDeliveryResponses(deliveries []db.AlertDelivery) []AlertDeliveryResponse {
	responses := make([]AlertDeliveryResponse, 0, len(deliveries))
	for _, delivery := range deliveries {
		responses = append(responses, alertDeliveryResponse(delivery))
	}
	return responses
}
func alertDeliveryAttemptResponse(attempt db.AlertDeliveryAttempt) AlertDeliveryAttemptResponse {
	return AlertDeliveryAttemptResponse{ID: attempt.ID, AlertDeliveryID: attempt.AlertDeliveryID, AttemptNumber: attempt.AttemptNumber, Status: attempt.Status, Stage: attempt.Stage, Error: safeAlertDeliveryError(attempt.Error), StartedAt: attempt.StartedAt, CompletedAt: attempt.CompletedAt, CreatedAt: attempt.CreatedAt, UpdatedAt: attempt.UpdatedAt}
}
func alertDeliveryAttemptResponses(attempts []db.AlertDeliveryAttempt) []AlertDeliveryAttemptResponse {
	responses := make([]AlertDeliveryAttemptResponse, 0, len(attempts))
	for _, attempt := range attempts {
		responses = append(responses, alertDeliveryAttemptResponse(attempt))
	}
	return responses
}
func safeAlertDeliveryError(value string) string {
	switch value {
	case "", "alert channel disabled", "alert cooldown active", "no alert channels configured", "no alert routes matched", "alert route destination missing", "alert grouped into active alert group", "alert grouped; sibling incidents still active", "alert grouped summary pending":
		return value
	default:
		if strings.HasPrefix(value, "alert route suppressed event") {
			return value
		}
		return "delivery failed; check Core logs"
	}
}
