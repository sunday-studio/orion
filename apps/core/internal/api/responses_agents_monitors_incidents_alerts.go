package api

import (
	"encoding/json"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"strings"
	"time"
)

type OrionEventResponse struct {
	ID         string    `json:"id"`
	Type       string    `json:"type"`
	Source     string    `json:"source"`
	Message    string    `json:"message"`
	AgentID    string    `json:"agent_id,omitempty"`
	MonitorID  string    `json:"monitor_id,omitempty"`
	IncidentID string    `json:"incident_id,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

func agentResponse(agent db.Agent) AgentResponse {
	return AgentResponse{ID: agent.ID, Name: agent.Name, OS: agent.OS, Platform: agent.Platform, KernelVersion: agent.KernelVersion, Arch: agent.Arch, MaintenanceMode: agent.MaintenanceMode, ReportingIntervalSeconds: agent.ReportingIntervalSeconds, CreatedAt: agent.CreatedAt, LastSeen: agent.LastSeen, Location: agent.Location.Data()}
}
func agentListResponse(row service.AgentListRow) AgentResponse {
	response := agentResponse(row.Agent)
	response.MonitorCount = row.MonitorCount
	response.IP = row.IP
	response.Status = row.Status
	response.AvailabilityHealth = row.AvailabilityHealth
	response.MonitorHealth = row.MonitorHealth
	response.StatusReason = row.StatusReason
	response.UptimeSeconds = row.UptimeSeconds
	return response
}
func agentListResponses(rows []service.// OrionEventResponse represents an operational Core event derived from stored records.
AgentListRow) []AgentResponse {
	responses := make([]AgentResponse, 0, len(rows))
	for _, row := range rows {
		responses = append(responses, agentListResponse(row))
	}
	return responses
}
func monitorResponse(monitor db.Monitor) MonitorResponse {
	return MonitorResponse{ID: monitor.ID, Description: monitor.Description, Type: monitor.Type, Name: monitor.Name, AgentID: monitor.AgentID, OwnerKind: "agent", OwnerID: monitor.AgentID, Source: "agent", LastSuccessfulReportAt: monitor.LastSuccessfulReportAt, ReportingIntervalSeconds: monitor.ReportingIntervalSeconds, ComputedHealth: monitor.ComputedHealth, LastHealthComputation: monitor.LastHealthComputation, ActiveIncidentID: monitor.ActiveIncidentID, IncidentState: monitor.IncidentState, Lifecycle: monitor.Lifecycle, Health: monitor.Health, CreatedAt: monitor.CreatedAt, UpdatedAt: monitor.UpdatedAt, DeletedAt: monitor.DeletedAt}
}
func monitorResponses(monitors []db.Monitor) []MonitorResponse {
	responses := make([]MonitorResponse, 0, len(monitors))
	for _, monitor := range monitors {
		responses = append(responses, monitorResponse(monitor))
	}
	return responses
}
func monitorResponsesWithAgents(monitors []db.Monitor, agentsByID map[string]db.Agent, coreMonitorIDs map[string]struct{}) []MonitorResponse {
	responses := make([]MonitorResponse, 0, len(monitors))
	for _, monitor := range monitors {
		response := monitorResponse(monitor)
		if agent, ok := agentsByID[monitor.AgentID]; ok {
			response.AgentName = agent.Name
			response.OwnerName = agent.Name
		}
		if _, ok := coreMonitorIDs[monitor.ID]; ok {
			response.OwnerKind = "core"
			response.Source = "core"
		}
		responses = append(responses, response)
	}
	return responses
}
func monitorReportResponse(report db.MonitorReport) MonitorReportResponse {
	return MonitorReportResponse{ID: report.ID, MonitorID: report.MonitorID, Payload: service.SafeMonitorReportPayload(report.Payload), CollectedAt: report.CollectedAt, Health: report.Health, CreatedAt: report.CreatedAt}
}
func monitorReportResponses(reports []db.MonitorReport) []MonitorReportResponse {
	responses := make([]MonitorReportResponse, 0, len(reports))
	for _, report := range reports {
		responses = append(responses, monitorReportResponse(report))
	}
	return responses
}
func agentReportResponse(report db.AgentReport) AgentReportResponse {
	return AgentReportResponse{ID: report.ID, AgentID: report.AgentID, CreatedAt: report.CreatedAt, AgentVersion: report.AgentVersion, ConfigSummary: agentConfigSummaryResponse(report.ConfigSummary), UptimeSeconds: report.UptimeSeconds, Timestamp: report.Timestamp, CPU: report.CPU.Data(), Memory: report.Memory.Data(), Disk: report.Disk.Data(), Location: report.Location.Data()}
}
func agentConfigSummaryResponse(raw string) *AgentConfigSummaryResponse {
	if raw == "" {
		return nil
	}
	var summary AgentConfigSummaryResponse
	if err := json.Unmarshal([]byte(raw), &summary); err != nil {
		return nil
	}
	if summary.ReportingInterval == "" && summary.MonitorCount == 0 && len(summary.MonitorTypes) == 0 {
		return nil
	}
	return &summary
}
func agentReportResponses(reports []db.AgentReport) []AgentReportResponse {
	responses := make([]AgentReportResponse, 0, len(reports))
	for _, report := range reports {
		responses = append(responses, agentReportResponse(report))
	}
	return responses
}
func serviceLogEntryResponse(entry db.ServiceLogEntry, agentsByID map[string]db.Agent) ServiceLogEntryResponse {
	response := ServiceLogEntryResponse{ID: entry.ID, AgentID: entry.AgentID, MonitorID: entry.MonitorID, Source: entry.Source, Stream: entry.Stream, Level: entry.Level, Component: entry.Component, MonitorName: entry.MonitorName, Message: entry.Message, Fields: entry.FieldsJSON, OccurredAt: entry.OccurredAt, CollectedAt: entry.CollectedAt, CreatedAt: entry.CreatedAt}
	if agent, ok := agentsByID[entry.AgentID]; ok {
		response.AgentName = agent.Name
	}
	return response
}
func serviceLogEntryResponses(entries []db.ServiceLogEntry, agentsByID map[string]db.Agent) []ServiceLogEntryResponse {
	responses := make([]ServiceLogEntryResponse, 0, len(entries))
	for _, entry := range entries {
		responses = append(responses, serviceLogEntryResponse(entry, agentsByID))
	}
	return responses
}
func incidentResponse(incident db.Incident, agent db.Agent, monitor db.Monitor) IncidentResponse {
	return IncidentResponse{ID: incident.ID, Status: incident.Status, Severity: incident.Severity, Title: incident.Title, AgentID: incident.AgentID, AgentName: agent.Name, MonitorID: incident.MonitorID, MonitorName: monitor.Name, MonitorType: monitor.Type, ImpactedComponents: incidentComponentImpactResponses(incident.ImpactedComponents), CoveredAt: incident.CoveredAt, CoveredUntil: incident.CoveredUntil, CoverageNote: incident.CoverageNote, ResolutionKind: incident.ResolutionKind, ReopenedAt: incident.ReopenedAt, ReopenCount: incident.ReopenCount, OpenedAt: incident.OpenedAt, ResolvedAt: incident.ResolvedAt, LastEventAt: incident.LastEventAt, LatestEvent: incident.LatestEvent, NotificationStatus: incident.NotificationStatus, AllowedActions: incidentAllowedActionsResponse(incident), CreatedAt: incident.CreatedAt, UpdatedAt: incident.UpdatedAt}
}
func incidentAllowedActionsResponse(incident db.Incident) IncidentAllowedActionsResponse {
	return IncidentAllowedActionsResponse{Acknowledge: incidentActionState(incident.Status == "open" || incident.Status == "covered", "incident must be open or covered"), Cover: incidentActionState(incident.Status == "open" || incident.Status == "acknowledged" || incident.Status == "covered", "incident must be open, acknowledged, or covered"), Resolve: incidentActionState(incident.Status != "resolved", "incident is already resolved"), Reopen: incidentActionState(incident.Status == "resolved" || incident.Status == "covered", "incident must be resolved or covered")}
}
func incidentActionState(allowed bool, blockedReason string) IncidentActionStateResponse {
	if allowed {
		return IncidentActionStateResponse{Allowed: true}
	}
	return IncidentActionStateResponse{Reason: blockedReason}
}
func incidentComponentImpactResponses(raw string) []IncidentComponentImpactResponse {
	if strings.TrimSpace(raw) == "" {
		return []IncidentComponentImpactResponse{}
	}
	var impacts []db.IncidentComponentImpact
	if err := json.Unmarshal([]byte(raw), &impacts); err != nil {
		return []IncidentComponentImpactResponse{}
	}
	responses := make([]IncidentComponentImpactResponse, 0, len(impacts))
	for _, impact := range impacts {
		response := IncidentComponentImpactResponse{ComponentID: strings.TrimSpace(impact.ComponentID), ComponentName: strings.TrimSpace(impact.ComponentName), Status: strings.TrimSpace(impact.Status), Impact: strings.TrimSpace(impact.Impact)}
		if response.ComponentID == "" && response.ComponentName == "" {
			continue
		}
		responses = append(responses, response)
	}
	return responses
}
func incidentEventResponse(event db.IncidentEvent) IncidentEventResponse {
	return IncidentEventResponse{ID: event.ID, IncidentID: event.IncidentID, Type: event.Type, Message: event.Message, MonitorReportID: event.MonitorReportID, ActorType: event.ActorType, ActorID: event.ActorID, Note: event.Note, CreatedAt: event.CreatedAt}
}
func incidentEventResponses(events []db.IncidentEvent) []IncidentEventResponse {
	responses := make([]IncidentEventResponse, 0, len(events))
	for _, event := range events {
		responses = append(responses, incidentEventResponse(event))
	}
	return responses
}
func alertDeliveryResponse(delivery db.AlertDelivery) AlertDeliveryResponse {
	return AlertDeliveryResponse{ID: delivery.ID, IncidentID: delivery.IncidentID, RouteID: delivery.RouteID, AlertGroupID: delivery.AlertGroupID, EventType: delivery.EventType, Channel: delivery.Channel, Type: delivery.Type, Status: delivery.Status, Error: safeAlertDeliveryError(delivery.Error), AttemptCount: delivery.AttemptCount, MaxAttempts: delivery.MaxAttempts, NextAttemptAt: delivery.NextAttemptAt, LastAttemptAt: delivery.LastAttemptAt, Attempts: alertDeliveryAttemptResponses(delivery.Attempts), CreatedAt: delivery.CreatedAt, UpdatedAt: delivery.UpdatedAt}
}
func alertRouteResponse(route db.AlertRoute) AlertRouteResponse {
	base := alertRouteFields(route)
	return AlertRouteResponse{ID: base.ID, Name: base.Name, Enabled: base.Enabled, Priority: base.Priority, EventTypes: base.EventTypes, Severities: base.Severities, AgentIDs: base.AgentIDs, MonitorIDs: base.MonitorIDs, MonitorTypes: base.MonitorTypes, ChannelIDs: base.ChannelIDs, Suppress: base.Suppress, GroupingPolicy: base.GroupingPolicy, GroupingDelaySeconds: base.GroupingDelaySeconds, CreatedAt: base.CreatedAt, UpdatedAt: base.UpdatedAt}
}

type alertRouteFieldSet struct {
	ID                   string
	Name                 string
	Enabled              bool
	Priority             int
	EventTypes           []string
	Severities           []string
	AgentIDs             []string
	MonitorIDs           []string
	MonitorTypes         []string
	ChannelIDs           []string
	Suppress             bool
	GroupingPolicy       string
	GroupingDelaySeconds int
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

func alertRouteFields(route db.AlertRoute) alertRouteFieldSet {
	return alertRouteFieldSet{ID: route.ID, Name: route.Name, Enabled: route.Enabled, Priority: route.Priority, EventTypes: decodeResponseList(route.EventTypes, db.DefaultAlertEvents()), Severities: decodeResponseList(route.Severities, nil), AgentIDs: decodeResponseList(route.AgentIDs, nil), MonitorIDs: decodeResponseList(route.MonitorIDs, nil), MonitorTypes: decodeResponseList(route.MonitorTypes, nil), ChannelIDs: decodeResponseList(route.ChannelIDs, nil), Suppress: route.Suppress, GroupingPolicy: normalizeAlertGroupingPolicy(route.GroupingPolicy), GroupingDelaySeconds: normalizeAlertGroupingDelaySeconds(route.GroupingDelaySeconds), CreatedAt: route.CreatedAt, UpdatedAt: route.UpdatedAt}
}
func alertRouteResponses(routes []db.AlertRoute) []AlertRouteResponse {
	responses := make([]AlertRouteResponse, 0, len(routes))
	for _, route := range routes {
		responses = append(responses, alertRouteResponse(route))
	}
	return responses
}
func alertRuleResponse(rule db.AlertRoute) AlertRuleResponse {
	base := alertRouteFields(rule)
	return AlertRuleResponse{ID: base.ID, Name: base.Name, Enabled: base.Enabled, Priority: base.Priority, EventTypes: base.EventTypes, Severities: base.Severities, AgentIDs: base.AgentIDs, MonitorIDs: base.MonitorIDs, MonitorTypes: base.MonitorTypes, ChannelIDs: base.ChannelIDs, Suppress: base.Suppress, GroupingPolicy: base.GroupingPolicy, GroupingDelaySeconds: base.GroupingDelaySeconds, CreatedAt: base.CreatedAt, UpdatedAt: base.UpdatedAt}
}
func alertRuleResponses(rules []db.AlertRoute) []AlertRuleResponse {
	responses := make([]AlertRuleResponse, 0, len(rules))
	for _, rule := range rules {
		responses = append(responses, alertRuleResponse(rule))
	}
	return responses
}
func alertRouteDryRunResponse(result *service.AlertRouteDryRunResult) AlertRouteDryRunResponse {
	evaluations := make([]AlertRouteEvaluationResponse, 0, len(result.RouteEvaluations))
	for _, evaluation := range result.RouteEvaluations {
		evaluations = append(evaluations, AlertRouteEvaluationResponse{Route: alertRouteResponse(evaluation.Route), Matched: evaluation.Matched, Suppressed: evaluation.Suppressed, Reasons: evaluation.Reasons})
	}
	return AlertRouteDryRunResponse{Event: result.Event, LegacyFallback: result.LegacyFallback, Suppressed: result.Suppressed, SuppressionReason: result.SuppressionReason, RouteEvaluations: evaluations, DestinationDecisions: result.DestinationDecisions}
}
