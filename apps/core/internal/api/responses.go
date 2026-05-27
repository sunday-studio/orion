package api

import (
	"encoding/json"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"regexp"
	"strings"
	"time"
)

// AgentResponse represents an agent in API responses (without generics for OpenAPI compatibility)
type AgentResponse struct {
	ID                       string         `json:"id"`
	Name                     string         `json:"name"`
	OS                       string         `json:"os"`
	Platform                 string         `json:"platform"`
	KernelVersion            string         `json:"kernel_version"`
	Arch                     string         `json:"arch"`
	MaintenanceMode          bool           `json:"maintenance_mode"`
	Status                   string         `json:"status,omitempty"`
	AvailabilityHealth       string         `json:"availability_health,omitempty"`
	MonitorHealth            string         `json:"monitor_health,omitempty"`
	StatusReason             string         `json:"status_reason,omitempty"`
	ReportingIntervalSeconds int            `json:"reporting_interval_seconds"`
	CreatedAt                time.Time      `json:"created_at"`
	LastSeen                 time.Time      `json:"last_seen"`
	Location                 db.GeoLocation `json:"location"`
	MonitorCount             int64          `json:"monitor_count,omitempty"`
	IP                       *string        `json:"ip,omitempty"`
	UptimeSeconds            *uint64        `json:"uptime_seconds,omitempty"`
}

// AgentSummaryResponse represents aggregate agent counts for list summary cards.
type AgentSummaryResponse struct {
	Total        int64 `json:"total"`
	Up           int64 `json:"up"`
	Down         int64 `json:"down"`
	Degraded     int64 `json:"degraded"`
	Unknown      int64 `json:"unknown"`
	Maintenance  int64 `json:"maintenance"`
	Stale        int64 `json:"stale"`
	HasIncidents int64 `json:"has_incidents"`
}

// AgentHealthResponse represents split agent availability and monitor health.
type AgentHealthResponse struct {
	AgentID            string `json:"agent_id"`
	OverallHealth      string `json:"overall_health"`
	AvailabilityHealth string `json:"availability_health"`
	MonitorHealth      string `json:"monitor_health"`
	StatusReason       string `json:"status_reason"`
	UpCount            int    `json:"up_count"`
	DownCount          int    `json:"down_count"`
	DegradedCount      int    `json:"degraded_count"`
	StaleCount         int    `json:"stale_count"`
	UnknownCount       int    `json:"unknown_count"`
	TotalCount         int    `json:"total_count"`
}

// MonitorResponse represents a monitor in API responses
type MonitorResponse struct {
	ID                       string     `json:"id"`
	Description              *string    `json:"description"`
	Type                     string     `json:"type"`
	Name                     string     `json:"name"`
	AgentID                  string     `json:"agent_id"`
	AgentName                string     `json:"agent_name,omitempty"`
	OwnerKind                string     `json:"owner_kind"`
	OwnerID                  string     `json:"owner_id"`
	OwnerName                string     `json:"owner_name,omitempty"`
	Source                   string     `json:"source"`
	LastSuccessfulReportAt   *time.Time `json:"last_successful_report_at"`
	ReportingIntervalSeconds int        `json:"reporting_interval_seconds"`
	ComputedHealth           string     `json:"computed_health"`
	LastHealthComputation    *time.Time `json:"last_health_computation"`
	ActiveIncidentID         string     `json:"active_incident_id,omitempty"`
	IncidentState            string     `json:"incident_state,omitempty"`
	Lifecycle                string     `json:"lifecycle"`
	Health                   string     `json:"health"`
	CreatedAt                time.Time  `json:"created_at"`
	UpdatedAt                time.Time  `json:"updated_at"`
	DeletedAt                time.Time  `json:"deleted_at"`
}

// MonitorReportResponse represents a monitor report in API responses
type MonitorReportResponse struct {
	ID          string    `json:"id"`
	MonitorID   string    `json:"monitor_id"`
	Payload     string    `json:"payload"`
	CollectedAt string    `json:"collected_at"`
	Health      string    `json:"health"`
	CreatedAt   time.Time `json:"created_at"`
}

// AgentReportResponse represents a system report in frontend API responses.
type AgentReportResponse struct {
	ID            string                      `json:"id"`
	AgentID       string                      `json:"agent_id"`
	CreatedAt     time.Time                   `json:"created_at"`
	AgentVersion  string                      `json:"agent_version"`
	ConfigSummary *AgentConfigSummaryResponse `json:"config_summary,omitempty"`
	UptimeSeconds uint64                      `json:"uptime_seconds"`
	Timestamp     string                      `json:"timestamp"`
	CPU           db.CPUStats                 `json:"cpu"`
	Memory        db.MemoryStats              `json:"memory"`
	Disk          db.DiskStats                `json:"disk"`
	Location      db.GeoLocation              `json:"location"`
}

type ServiceLogEntryResponse struct {
	ID          string    `json:"id"`
	AgentID     string    `json:"agent_id"`
	AgentName   string    `json:"agent_name,omitempty"`
	MonitorID   string    `json:"monitor_id,omitempty"`
	Source      string    `json:"source"`
	Stream      string    `json:"stream"`
	Level       string    `json:"level"`
	Component   string    `json:"component,omitempty"`
	MonitorName string    `json:"monitor_name,omitempty"`
	Message     string    `json:"message"`
	Fields      string    `json:"fields,omitempty"`
	OccurredAt  time.Time `json:"occurred_at"`
	CollectedAt time.Time `json:"collected_at"`
	CreatedAt   time.Time `json:"created_at"`
}

// AgentConfigSummaryResponse is the frontend-safe subset of an agent's reported config summary.
type AgentConfigSummaryResponse struct {
	ReportingInterval string         `json:"reporting_interval,omitempty"`
	MonitorCount      int            `json:"monitor_count,omitempty"`
	MonitorTypes      map[string]int `json:"monitor_types,omitempty"`
}

// IncidentResponse represents a persisted incident in frontend API responses.
type IncidentResponse struct {
	ID                 string     `json:"id"`
	Status             string     `json:"status"`
	Severity           string     `json:"severity"`
	Title              string     `json:"title"`
	AgentID            string     `json:"agent_id"`
	AgentName          string     `json:"agent_name"`
	MonitorID          string     `json:"monitor_id"`
	MonitorName        string     `json:"monitor_name"`
	MonitorType        string     `json:"monitor_type"`
	OpenedAt           time.Time  `json:"opened_at"`
	ResolvedAt         *time.Time `json:"resolved_at"`
	LastEventAt        time.Time  `json:"last_event_at"`
	LatestEvent        string     `json:"latest_event"`
	NotificationStatus string     `json:"notification_status"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// IncidentEventResponse represents an incident event in frontend API responses.
type IncidentEventResponse struct {
	ID              string    `json:"id"`
	IncidentID      string    `json:"incident_id"`
	Type            string    `json:"type"`
	Message         string    `json:"message"`
	MonitorReportID string    `json:"monitor_report_id"`
	CreatedAt       time.Time `json:"created_at"`
}

// UptimeDayBucketResponse represents one daily uptime bucket.
type UptimeDayBucketResponse struct {
	Date          string  `json:"date"`
	Up            int     `json:"up"`
	Total         int     `json:"total"`
	UptimePercent float64 `json:"uptime_percent"`
}

// UptimeResponse represents uptime over a requested period.
type UptimeResponse struct {
	DailyBuckets  []UptimeDayBucketResponse `json:"daily_buckets"`
	UptimePercent float64                   `json:"uptime_percent"`
}

// AlertDeliveryResponse represents a frontend-safe alert delivery record.
type AlertDeliveryResponse struct {
	ID            string                         `json:"id"`
	IncidentID    string                         `json:"incident_id"`
	RouteID       string                         `json:"route_id,omitempty"`
	AlertGroupID  string                         `json:"alert_group_id,omitempty"`
	EventType     string                         `json:"event_type"`
	Channel       string                         `json:"channel"`
	Type          string                         `json:"type"`
	Status        string                         `json:"status"`
	Error         string                         `json:"error,omitempty"`
	AttemptCount  int                            `json:"attempt_count"`
	MaxAttempts   int                            `json:"max_attempts"`
	NextAttemptAt *time.Time                     `json:"next_attempt_at,omitempty"`
	LastAttemptAt *time.Time                     `json:"last_attempt_at,omitempty"`
	Attempts      []AlertDeliveryAttemptResponse `json:"attempts"`
	CreatedAt     time.Time                      `json:"created_at"`
	UpdatedAt     time.Time                      `json:"updated_at"`
}

// AlertDeliveryAttemptResponse represents one sanitized delivery try.
type AlertDeliveryAttemptResponse struct {
	ID              string     `json:"id"`
	AlertDeliveryID string     `json:"alert_delivery_id"`
	AttemptNumber   int        `json:"attempt_number"`
	Status          string     `json:"status"`
	Stage           string     `json:"stage"`
	Error           string     `json:"error,omitempty"`
	StartedAt       time.Time  `json:"started_at"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// AlertRouteResponse represents an explicit alert route.
type AlertRouteResponse struct {
	ID                   string    `json:"id"`
	Name                 string    `json:"name"`
	Enabled              bool      `json:"enabled"`
	Priority             int       `json:"priority"`
	EventTypes           []string  `json:"event_types"`
	Severities           []string  `json:"severities"`
	AgentIDs             []string  `json:"agent_ids"`
	MonitorIDs           []string  `json:"monitor_ids"`
	MonitorTypes         []string  `json:"monitor_types"`
	ChannelIDs           []string  `json:"channel_ids"`
	Suppress             bool      `json:"suppress"`
	GroupingPolicy       string    `json:"grouping_policy"`
	GroupingDelaySeconds int       `json:"grouping_delay_seconds"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// AlertChannelResponse represents a configured alert channel.
type AlertChannelResponse struct {
	ID                         string     `json:"id"`
	Name                       string     `json:"name"`
	Type                       string     `json:"type"`
	Enabled                    bool       `json:"enabled"`
	WebhookURL                 string     `json:"webhook_url,omitempty"`
	WebhookConfigured          bool       `json:"webhook_configured,omitempty"`
	WebhookSignatureConfigured bool       `json:"webhook_signature_configured,omitempty"`
	EmailToConfigured          bool       `json:"email_to_configured,omitempty"`
	EmailFromConfigured        bool       `json:"email_from_configured,omitempty"`
	SMTPHostConfigured         bool       `json:"smtp_host_configured,omitempty"`
	SMTPPortConfigured         bool       `json:"smtp_port_configured,omitempty"`
	SMTPUsernameConfigured     bool       `json:"smtp_username_configured,omitempty"`
	SubscribedEvents           []string   `json:"subscribed_events"`
	LastDeliveryStatus         string     `json:"last_delivery_status,omitempty"`
	LastDeliveryAt             *time.Time `json:"last_delivery_at,omitempty"`
	CreatedAt                  time.Time  `json:"created_at"`
	UpdatedAt                  time.Time  `json:"updated_at"`
}

// AlertSMTPServiceResponse represents a reusable SMTP service without secrets.
type AlertSMTPServiceResponse struct {
	ID                 string    `json:"id"`
	Name               string    `json:"name"`
	Enabled            bool      `json:"enabled"`
	Host               string    `json:"host"`
	Port               int       `json:"port"`
	FromEmail          string    `json:"from_email"`
	UsernameConfigured bool      `json:"username_configured,omitempty"`
	PasswordConfigured bool      `json:"password_configured,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// AlertEmailDestinationResponse represents a reusable email destination.
type AlertEmailDestinationResponse struct {
	ID                 string     `json:"id"`
	SMTPServiceID      string     `json:"smtp_service_id"`
	SMTPServiceName    string     `json:"smtp_service_name,omitempty"`
	Name               string     `json:"name"`
	Enabled            bool       `json:"enabled"`
	EmailTo            string     `json:"email_to"`
	SubscribedEvents   []string   `json:"subscribed_events"`
	LastDeliveryStatus string     `json:"last_delivery_status,omitempty"`
	LastDeliveryAt     *time.Time `json:"last_delivery_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// AlertRuleResponse represents an effective Core alert rule.
type AlertRuleResponse struct {
	Name                          string   `json:"name"`
	TriggerCondition              string   `json:"trigger_condition"`
	Severity                      string   `json:"severity"`
	Enabled                       bool     `json:"enabled"`
	CooldownSeconds               int      `json:"cooldown_seconds"`
	RecoveryNotificationEnabled   bool     `json:"recovery_notification_enabled"`
	MaintenanceSuppressionEnabled bool     `json:"maintenance_suppression_enabled"`
	TargetChannels                []string `json:"target_channels"`
}

// AlertRouteDryRunResponse explains route matching and destination decisions.
type AlertRouteDryRunResponse struct {
	Event                service.AlertRouteContext          `json:"event"`
	LegacyFallback       bool                               `json:"legacy_fallback"`
	Suppressed           bool                               `json:"suppressed"`
	SuppressionReason    string                             `json:"suppression_reason,omitempty"`
	RouteEvaluations     []AlertRouteEvaluationResponse     `json:"route_evaluations"`
	DestinationDecisions []service.AlertDestinationDecision `json:"destination_decisions"`
}

// AlertRouteEvaluationResponse explains one route's match result.
type AlertRouteEvaluationResponse struct {
	Route      AlertRouteResponse `json:"route"`
	Matched    bool               `json:"matched"`
	Suppressed bool               `json:"suppressed"`
	Reasons    []string           `json:"reasons"`
}

// IncidentTimelineItemResponse represents a normalized incident timeline item.
type IncidentTimelineItemResponse struct {
	ID              string    `json:"id"`
	Type            string    `json:"type"`
	Source          string    `json:"source"`
	Message         string    `json:"message"`
	Evidence        string    `json:"evidence,omitempty"`
	MonitorReportID string    `json:"monitor_report_id,omitempty"`
	AlertDeliveryID string    `json:"alert_delivery_id,omitempty"`
	Channel         string    `json:"channel,omitempty"`
	Status          string    `json:"status,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

// OrionEventResponse represents an operational Core event derived from stored records.
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
	return AgentResponse{
		ID:                       agent.ID,
		Name:                     agent.Name,
		OS:                       agent.OS,
		Platform:                 agent.Platform,
		KernelVersion:            agent.KernelVersion,
		Arch:                     agent.Arch,
		MaintenanceMode:          agent.MaintenanceMode,
		ReportingIntervalSeconds: agent.ReportingIntervalSeconds,
		CreatedAt:                agent.CreatedAt,
		LastSeen:                 agent.LastSeen,
		Location:                 agent.Location.Data(),
	}
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

func agentListResponses(rows []service.AgentListRow) []AgentResponse {
	responses := make([]AgentResponse, 0, len(rows))
	for _, row := range rows {
		responses = append(responses, agentListResponse(row))
	}
	return responses
}

func monitorResponse(monitor db.Monitor) MonitorResponse {
	return MonitorResponse{
		ID:                       monitor.ID,
		Description:              monitor.Description,
		Type:                     monitor.Type,
		Name:                     monitor.Name,
		AgentID:                  monitor.AgentID,
		OwnerKind:                "agent",
		OwnerID:                  monitor.AgentID,
		Source:                   "agent",
		LastSuccessfulReportAt:   monitor.LastSuccessfulReportAt,
		ReportingIntervalSeconds: monitor.ReportingIntervalSeconds,
		ComputedHealth:           monitor.ComputedHealth,
		LastHealthComputation:    monitor.LastHealthComputation,
		ActiveIncidentID:         monitor.ActiveIncidentID,
		IncidentState:            monitor.IncidentState,
		Lifecycle:                monitor.Lifecycle,
		Health:                   monitor.Health,
		CreatedAt:                monitor.CreatedAt,
		UpdatedAt:                monitor.UpdatedAt,
		DeletedAt:                monitor.DeletedAt,
	}
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
	return MonitorReportResponse{
		ID:          report.ID,
		MonitorID:   report.MonitorID,
		Payload:     safeMonitorReportPayload(report.Payload),
		CollectedAt: report.CollectedAt,
		Health:      report.Health,
		CreatedAt:   report.CreatedAt,
	}
}

type heartbeatSensitivePattern struct {
	pattern     *regexp.Regexp
	replacement string
}

var heartbeatSensitivePatterns = []heartbeatSensitivePattern{
	{
		pattern:     regexp.MustCompile(`(?i)(token|password|secret|api[_-]?key|authorization)(["']?\s*[:=]\s*["']?)[^"',\s}]+`),
		replacement: "${1}${2}[redacted]",
	},
	{
		pattern:     regexp.MustCompile(`(?i)(bearer\s+)[a-z0-9._~+/-]+`),
		replacement: "${1}[redacted]",
	},
}

func safeMonitorReportPayload(payload string) string {
	var fields map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &fields); err != nil {
		return payload
	}
	if !isHeartbeatReportPayload(fields) {
		return payload
	}
	redacted := redactHeartbeatPayloadValue(fields)
	body, err := json.Marshal(redacted)
	if err != nil {
		return payload
	}
	return string(body)
}

func isHeartbeatReportPayload(fields map[string]interface{}) bool {
	return stringFieldEquals(fields, "type", "heartbeat") || stringFieldEquals(fields, "runner", "heartbeat")
}

func stringFieldEquals(fields map[string]interface{}, key string, expected string) bool {
	value, ok := fields[key].(string)
	return ok && strings.EqualFold(strings.TrimSpace(value), expected)
}

func redactHeartbeatPayloadValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{}, len(typed))
		for key, field := range typed {
			if heartbeatSensitiveKey(key) {
				result[key] = "[redacted]"
				continue
			}
			result[key] = redactHeartbeatPayloadValue(field)
		}
		return result
	case []interface{}:
		result := make([]interface{}, 0, len(typed))
		for _, item := range typed {
			result = append(result, redactHeartbeatPayloadValue(item))
		}
		return result
	case string:
		return redactHeartbeatText(typed)
	default:
		return typed
	}
}

func heartbeatSensitiveKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	return strings.Contains(normalized, "token") ||
		strings.Contains(normalized, "password") ||
		strings.Contains(normalized, "secret") ||
		strings.Contains(normalized, "api_key") ||
		strings.Contains(normalized, "apikey") ||
		strings.Contains(normalized, "authorization")
}

func redactHeartbeatText(value string) string {
	redacted := value
	for _, pattern := range heartbeatSensitivePatterns {
		redacted = pattern.pattern.ReplaceAllString(redacted, pattern.replacement)
	}
	return redacted
}

func monitorReportResponses(reports []db.MonitorReport) []MonitorReportResponse {
	responses := make([]MonitorReportResponse, 0, len(reports))
	for _, report := range reports {
		responses = append(responses, monitorReportResponse(report))
	}
	return responses
}

func agentReportResponse(report db.AgentReport) AgentReportResponse {
	return AgentReportResponse{
		ID:            report.ID,
		AgentID:       report.AgentID,
		CreatedAt:     report.CreatedAt,
		AgentVersion:  report.AgentVersion,
		ConfigSummary: agentConfigSummaryResponse(report.ConfigSummary),
		UptimeSeconds: report.UptimeSeconds,
		Timestamp:     report.Timestamp,
		CPU:           report.CPU.Data(),
		Memory:        report.Memory.Data(),
		Disk:          report.Disk.Data(),
		Location:      report.Location.Data(),
	}
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
	response := ServiceLogEntryResponse{
		ID:          entry.ID,
		AgentID:     entry.AgentID,
		MonitorID:   entry.MonitorID,
		Source:      entry.Source,
		Stream:      entry.Stream,
		Level:       entry.Level,
		Component:   entry.Component,
		MonitorName: entry.MonitorName,
		Message:     entry.Message,
		Fields:      entry.FieldsJSON,
		OccurredAt:  entry.OccurredAt,
		CollectedAt: entry.CollectedAt,
		CreatedAt:   entry.CreatedAt,
	}
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
	return IncidentResponse{
		ID:                 incident.ID,
		Status:             incident.Status,
		Severity:           incident.Severity,
		Title:              incident.Title,
		AgentID:            incident.AgentID,
		AgentName:          agent.Name,
		MonitorID:          incident.MonitorID,
		MonitorName:        monitor.Name,
		MonitorType:        monitor.Type,
		OpenedAt:           incident.OpenedAt,
		ResolvedAt:         incident.ResolvedAt,
		LastEventAt:        incident.LastEventAt,
		LatestEvent:        incident.LatestEvent,
		NotificationStatus: incident.NotificationStatus,
		CreatedAt:          incident.CreatedAt,
		UpdatedAt:          incident.UpdatedAt,
	}
}

func incidentEventResponse(event db.IncidentEvent) IncidentEventResponse {
	return IncidentEventResponse{
		ID:              event.ID,
		IncidentID:      event.IncidentID,
		Type:            event.Type,
		Message:         event.Message,
		MonitorReportID: event.MonitorReportID,
		CreatedAt:       event.CreatedAt,
	}
}

func incidentEventResponses(events []db.IncidentEvent) []IncidentEventResponse {
	responses := make([]IncidentEventResponse, 0, len(events))
	for _, event := range events {
		responses = append(responses, incidentEventResponse(event))
	}
	return responses
}

func alertDeliveryResponse(delivery db.AlertDelivery) AlertDeliveryResponse {
	return AlertDeliveryResponse{
		ID:            delivery.ID,
		IncidentID:    delivery.IncidentID,
		RouteID:       delivery.RouteID,
		AlertGroupID:  delivery.AlertGroupID,
		EventType:     delivery.EventType,
		Channel:       delivery.Channel,
		Type:          delivery.Type,
		Status:        delivery.Status,
		Error:         safeAlertDeliveryError(delivery.Error),
		AttemptCount:  delivery.AttemptCount,
		MaxAttempts:   delivery.MaxAttempts,
		NextAttemptAt: delivery.NextAttemptAt,
		LastAttemptAt: delivery.LastAttemptAt,
		Attempts:      alertDeliveryAttemptResponses(delivery.Attempts),
		CreatedAt:     delivery.CreatedAt,
		UpdatedAt:     delivery.UpdatedAt,
	}
}

func alertRouteResponse(route db.AlertRoute) AlertRouteResponse {
	return AlertRouteResponse{
		ID:           route.ID,
		Name:         route.Name,
		Enabled:      route.Enabled,
		Priority:     route.Priority,
		EventTypes:   decodeResponseList(route.EventTypes, db.DefaultAlertEvents()),
		Severities:   decodeResponseList(route.Severities, nil),
		AgentIDs:     decodeResponseList(route.AgentIDs, nil),
		MonitorIDs:   decodeResponseList(route.MonitorIDs, nil),
		MonitorTypes: decodeResponseList(route.MonitorTypes, nil),
		ChannelIDs:   decodeResponseList(route.ChannelIDs, nil),
		Suppress:     route.Suppress,
		GroupingPolicy: normalizeAlertGroupingPolicy(
			route.GroupingPolicy,
		),
		GroupingDelaySeconds: normalizeAlertGroupingDelaySeconds(route.GroupingDelaySeconds),
		CreatedAt:            route.CreatedAt,
		UpdatedAt:            route.UpdatedAt,
	}
}

func alertRouteResponses(routes []db.AlertRoute) []AlertRouteResponse {
	responses := make([]AlertRouteResponse, 0, len(routes))
	for _, route := range routes {
		responses = append(responses, alertRouteResponse(route))
	}
	return responses
}

func alertRouteDryRunResponse(result *service.AlertRouteDryRunResult) AlertRouteDryRunResponse {
	evaluations := make([]AlertRouteEvaluationResponse, 0, len(result.RouteEvaluations))
	for _, evaluation := range result.RouteEvaluations {
		evaluations = append(evaluations, AlertRouteEvaluationResponse{
			Route:      alertRouteResponse(evaluation.Route),
			Matched:    evaluation.Matched,
			Suppressed: evaluation.Suppressed,
			Reasons:    evaluation.Reasons,
		})
	}
	return AlertRouteDryRunResponse{
		Event:                result.Event,
		LegacyFallback:       result.LegacyFallback,
		Suppressed:           result.Suppressed,
		SuppressionReason:    result.SuppressionReason,
		RouteEvaluations:     evaluations,
		DestinationDecisions: result.DestinationDecisions,
	}
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
	return AlertDeliveryAttemptResponse{
		ID:              attempt.ID,
		AlertDeliveryID: attempt.AlertDeliveryID,
		AttemptNumber:   attempt.AttemptNumber,
		Status:          attempt.Status,
		Stage:           attempt.Stage,
		Error:           safeAlertDeliveryError(attempt.Error),
		StartedAt:       attempt.StartedAt,
		CompletedAt:     attempt.CompletedAt,
		CreatedAt:       attempt.CreatedAt,
		UpdatedAt:       attempt.UpdatedAt,
	}
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
