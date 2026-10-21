package api

import (
	"encoding/json"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"strings"
	"time"
)

// OrionEventResponse represents an operational Core event derived from stored records.
type OrionEventResponse struct {
	ID         string    `json:"id"`
	Type       string    `json:"type"`
	Source     string    `json:"source"`
	Message    string    `json:"message"`
	NodeID     string    `json:"node_id,omitempty"`
	MonitorID  string    `json:"monitor_id,omitempty"`
	IncidentID string    `json:"incident_id,omitempty"`
	CreatedAt  time.Time `json:"created_at"`

	rowID string // source row id; pairs with CreatedAt as the keyset sort key
}

func nodeResponse(node db.Node) NodeResponse {
	return NodeResponse{Labels: service.DecodeNodeLabels(node.Labels), Notes: node.Notes, CPUSeries: []float64{}, ID: node.ID, System: node.System, Name: node.Name, OS: node.OS, Platform: node.Platform, KernelVersion: node.KernelVersion, Arch: node.Arch, MaintenanceMode: node.InMaintenance(time.Now().UTC()), MaintenanceUntil: node.MaintenanceUntil, ReportingIntervalSeconds: node.ReportingIntervalSeconds, CreatedAt: node.CreatedAt, LastSeen: node.LastSeen, Location: node.Location.Data()}
}
func nodeListResponse(row service.NodeListRow) NodeResponse {
	response := nodeResponse(row.Node)
	response.MonitorCount = row.MonitorCount
	response.IP = row.IP
	response.Status = row.Status
	response.AvailabilityHealth = row.AvailabilityHealth
	response.MonitorHealth = row.MonitorHealth
	response.StatusReason = row.StatusReason
	response.UptimeSeconds = row.UptimeSeconds
	applyNodeMetrics(&response, row.Metrics)
	response.StatusText, response.StatusDetail = row.StatusText, row.StatusDetail
	return response
}
func nodeListResponses(rows []service.NodeListRow) []NodeResponse {
	responses := make([]NodeResponse, 0, len(rows))
	for _, row := range rows {
		responses = append(responses, nodeListResponse(row))
	}
	return responses
}
func monitorResponse(monitor db.Monitor) MonitorResponse {
	return MonitorResponse{ID: monitor.ID, System: monitor.System, RunbookURL: monitor.RunbookURL, IntervalSeconds: monitor.ReportingIntervalSeconds, Description: monitor.Description, Type: monitor.Type, Name: monitor.Name, NodeID: monitor.NodeID, OwnerKind: "node", OwnerID: monitor.NodeID, Source: "node", LastSuccessfulReportAt: monitor.LastSuccessfulReportAt, ReportingIntervalSeconds: monitor.ReportingIntervalSeconds, ComputedHealth: monitor.ComputedHealth, LastHealthComputation: monitor.LastHealthComputation, ActiveIncidentID: monitor.ActiveIncidentID, IncidentState: monitor.IncidentState, Lifecycle: monitor.Lifecycle, Health: monitor.Health, CreatedAt: monitor.CreatedAt, UpdatedAt: monitor.UpdatedAt, DeletedAt: monitor.DeletedAt}
}
func monitorResponses(monitors []db.Monitor) []MonitorResponse {
	responses := make([]MonitorResponse, 0, len(monitors))
	for _, monitor := range monitors {
		responses = append(responses, monitorResponse(monitor))
	}
	return responses
}
func monitorResponsesWithNodes(monitors []db.Monitor, nodesByID map[string]db.Node, coreMonitorIDs map[string]struct{}) []MonitorResponse {
	responses := make([]MonitorResponse, 0, len(monitors))
	for _, monitor := range monitors {
		response := monitorResponse(monitor)
		if node, ok := nodesByID[monitor.NodeID]; ok {
			response.NodeName = node.Name
			response.OwnerName = node.Name
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
	response := MonitorReportResponse{ID: report.ID, MonitorID: report.MonitorID, Payload: service.SafeMonitorReportPayload(report.Payload), CollectedAt: report.CollectedAt, Health: report.Health, CreatedAt: report.CreatedAt, Passed: report.Health == "up"}
	response.Summary, response.DurationMs, response.ReportedBy = describeMonitorReport(report)
	return response
}
func monitorReportResponses(reports []db.MonitorReport) []MonitorReportResponse {
	responses := make([]MonitorReportResponse, 0, len(reports))
	for _, report := range reports {
		responses = append(responses, monitorReportResponse(report))
	}
	return responses
}
func nodeReportResponse(report db.NodeReport) NodeReportResponse {
	return NodeReportResponse{ID: report.ID, NodeID: report.NodeID, CreatedAt: report.CreatedAt, NodeVersion: report.NodeVersion, ConfigSummary: nodeConfigSummaryResponse(report.ConfigSummary), UptimeSeconds: report.UptimeSeconds, Timestamp: report.Timestamp, CPU: report.CPU.Data(), Memory: report.Memory.Data(), Disk: report.Disk.Data(), Location: report.Location.Data()}
}
func nodeConfigSummaryResponse(raw string) *NodeConfigSummaryResponse {
	if raw == "" {
		return nil
	}
	var summary NodeConfigSummaryResponse
	if err := json.Unmarshal([]byte(raw), &summary); err != nil {
		return nil
	}
	if summary.ReportingInterval == "" && summary.MonitorCount == 0 && len(summary.MonitorTypes) == 0 {
		return nil
	}
	return &summary
}
func nodeReportResponses(reports []db.NodeReport) []NodeReportResponse {
	responses := make([]NodeReportResponse, 0, len(reports))
	for _, report := range reports {
		responses = append(responses, nodeReportResponse(report))
	}
	return responses
}
func serviceLogEntryResponse(entry db.ServiceLogEntry, nodesByID map[string]db.Node) ServiceLogEntryResponse {
	response := ServiceLogEntryResponse{ID: entry.ID, NodeID: entry.NodeID, MonitorID: entry.MonitorID, Source: entry.Source, Stream: entry.Stream, Level: entry.Level, Component: entry.Component, MonitorName: entry.MonitorName, Message: entry.Message, Fields: entry.FieldsJSON, OccurredAt: entry.OccurredAt, CollectedAt: entry.CollectedAt, CreatedAt: entry.CreatedAt}
	if node, ok := nodesByID[entry.NodeID]; ok {
		response.NodeName = node.Name
	}
	return response
}
func serviceLogEntryResponses(entries []db.ServiceLogEntry, nodesByID map[string]db.Node) []ServiceLogEntryResponse {
	responses := make([]ServiceLogEntryResponse, 0, len(entries))
	for _, entry := range entries {
		responses = append(responses, serviceLogEntryResponse(entry, nodesByID))
	}
	return responses
}
func incidentResponse(incident db.Incident, node db.Node, monitor db.Monitor) IncidentResponse {
	return IncidentResponse{ID: incident.ID, Number: incident.Number, DisplayID: incidentDisplayID(incident.Number), Summary: incident.Summary, Source: incident.Source, Responder: incident.Responder, ResponderAt: incident.ResponderAt, CoveredByIncidentID: incident.CoveredByIncidentID, Status: incident.Status, Severity: incident.Severity, Title: incident.Title, NodeID: incident.NodeID, NodeName: node.Name, MonitorID: incident.MonitorID, MonitorName: monitor.Name, MonitorType: monitor.Type, ImpactedComponents: incidentComponentImpactResponses(incident.ImpactedComponents), CoveredAt: incident.CoveredAt, CoveredUntil: incident.CoveredUntil, CoverageNote: incident.CoverageNote, ResolutionKind: incident.ResolutionKind, ReopenedAt: incident.ReopenedAt, ReopenCount: incident.ReopenCount, OpenedAt: incident.OpenedAt, ResolvedAt: incident.ResolvedAt, LastEventAt: incident.LastEventAt, LatestEvent: incident.LatestEvent, NotificationStatus: incident.NotificationStatus, AllowedActions: incidentAllowedActionsResponse(incident), CreatedAt: incident.CreatedAt, UpdatedAt: incident.UpdatedAt}
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
	return AlertRouteResponse{ID: base.ID, Name: base.Name, Enabled: base.Enabled, Priority: base.Priority, EventTypes: base.EventTypes, Severities: base.Severities, NodeIDs: base.NodeIDs, MonitorIDs: base.MonitorIDs, MonitorTypes: base.MonitorTypes, ChannelIDs: base.ChannelIDs, Suppress: base.Suppress, GroupingPolicy: base.GroupingPolicy, GroupingDelaySeconds: base.GroupingDelaySeconds, CreatedAt: base.CreatedAt, UpdatedAt: base.UpdatedAt}
}

type alertRouteFieldSet struct {
	ID                   string
	Name                 string
	Enabled              bool
	Priority             int
	EventTypes           []string
	Severities           []string
	NodeIDs              []string
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
	return alertRouteFieldSet{ID: route.ID, Name: route.Name, Enabled: route.Enabled, Priority: route.Priority, EventTypes: decodeResponseList(route.EventTypes, db.DefaultAlertEvents()), Severities: decodeResponseList(route.Severities, nil), NodeIDs: decodeResponseList(route.NodeIDs, nil), MonitorIDs: decodeResponseList(route.MonitorIDs, nil), MonitorTypes: decodeResponseList(route.MonitorTypes, nil), ChannelIDs: decodeResponseList(route.ChannelIDs, nil), Suppress: route.Suppress, GroupingPolicy: normalizeAlertGroupingPolicy(route.GroupingPolicy), GroupingDelaySeconds: normalizeAlertGroupingDelaySeconds(route.GroupingDelaySeconds), CreatedAt: route.CreatedAt, UpdatedAt: route.UpdatedAt}
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
	return AlertRuleResponse{ID: base.ID, Name: base.Name, Enabled: base.Enabled, Priority: base.Priority, EventTypes: base.EventTypes, Severities: base.Severities, NodeIDs: base.NodeIDs, MonitorIDs: base.MonitorIDs, MonitorTypes: base.MonitorTypes, ChannelIDs: base.ChannelIDs, Suppress: base.Suppress, GroupingPolicy: base.GroupingPolicy, GroupingDelaySeconds: base.GroupingDelaySeconds, CreatedAt: base.CreatedAt, UpdatedAt: base.UpdatedAt}
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
