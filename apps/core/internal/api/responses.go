package api

import (
	"orion/core/internal/db"
	"orion/core/internal/service"
	"time"
)

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

type MonitorReportResponse struct {
	ID          string    `json:"id"`
	MonitorID   string    `json:"monitor_id"`
	Payload     string    `json:"payload"`
	CollectedAt string    `json:"collected_at"`
	Health      string    `json:"health"`
	CreatedAt   time.Time `json:"created_at"`
}

type IncidentEvidenceResponse struct {
	TriggeringReport *MonitorReportResponse `json:"triggering_report,omitempty"`
	LatestReport     *MonitorReportResponse `json:"latest_report,omitempty"`
}

type IncidentNextActionResponse struct {
	ID           string `json:"id"`
	Label        string `json:"label"`
	Description  string `json:"description"`
	ActionType   string `json:"action_type"`
	Priority     int    `json:"priority"`
	TargetKind   string `json:"target_kind,omitempty"`
	TargetID     string `json:"target_id,omitempty"`
	TargetTab    string `json:"target_tab,omitempty"`
	FilterStatus string `json:"filter_status,omitempty"`
}

type IncidentRelatedIncidentResponse struct {
	ID                 string     `json:"id"`
	Status             string     `json:"status"`
	Severity           string     `json:"severity"`
	Title              string     `json:"title"`
	ResolutionKind     string     `json:"resolution_kind,omitempty"`
	OpenedAt           time.Time  `json:"opened_at"`
	ResolvedAt         *time.Time `json:"resolved_at,omitempty"`
	LastEventAt        time.Time  `json:"last_event_at"`
	LatestEvent        string     `json:"latest_event"`
	NotificationStatus string     `json:"notification_status"`
}

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

type AgentConfigSummaryResponse struct {
	ReportingInterval string         `json:"reporting_interval,omitempty"`
	MonitorCount      int            `json:"monitor_count,omitempty"`
	MonitorTypes      map[string]int `json:"monitor_types,omitempty"`
}

type IncidentResponse struct {
	ID                 string                            `json:"id"`
	Status             string                            `json:"status"`
	Severity           string                            `json:"severity"`
	Title              string                            `json:"title"`
	AgentID            string                            `json:"agent_id"`
	AgentName          string                            `json:"agent_name"`
	MonitorID          string                            `json:"monitor_id"`
	MonitorName        string                            `json:"monitor_name"`
	MonitorType        string                            `json:"monitor_type"`
	ImpactedComponents []IncidentComponentImpactResponse `json:"impacted_components"`
	CoveredAt          *time.Time                        `json:"covered_at,omitempty"`
	CoveredUntil       *time.Time                        `json:"covered_until,omitempty"`
	CoverageNote       string                            `json:"coverage_note,omitempty"`
	ResolutionKind     string                            `json:"resolution_kind,omitempty"`
	ReopenedAt         *time.Time                        `json:"reopened_at,omitempty"`
	ReopenCount        int                               `json:"reopen_count"`
	OpenedAt           time.Time                         `json:"opened_at"`
	ResolvedAt         *time.Time                        `json:"resolved_at"`
	LastEventAt        time.Time                         `json:"last_event_at"`
	LatestEvent        string                            `json:"latest_event"`
	NotificationStatus string                            `json:"notification_status"`
	AllowedActions     IncidentAllowedActionsResponse    `json:"allowed_actions"`
	CreatedAt          time.Time                         `json:"created_at"`
	UpdatedAt          time.Time                         `json:"updated_at"`
}

type IncidentAllowedActionsResponse struct {
	Acknowledge IncidentActionStateResponse `json:"acknowledge"`
	Cover       IncidentActionStateResponse `json:"cover"`
	Resolve     IncidentActionStateResponse `json:"resolve"`
	Reopen      IncidentActionStateResponse `json:"reopen"`
}

type IncidentActionStateResponse struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
}

type IncidentInsightsResponse struct {
	RecurringFailures       []IncidentRecurringFailureResponse   `json:"recurring_failures"`
	LifecycleTiming         IncidentLifecycleTimingResponse      `json:"lifecycle_timing"`
	NotificationReliability IncidentNotificationReliabilityStats `json:"notification_reliability"`
}

type IncidentRecurringFailureResponse struct {
	MonitorID      string    `json:"monitor_id"`
	MonitorName    string    `json:"monitor_name"`
	IncidentCount  int64     `json:"incident_count"`
	LastIncidentAt time.Time `json:"last_incident_at"`
}

type IncidentLifecycleTimingResponse struct {
	AcknowledgedCount            int64 `json:"acknowledged_count"`
	ResolvedCount                int64 `json:"resolved_count"`
	MeanTimeToAcknowledgeSeconds int64 `json:"mean_time_to_acknowledge_seconds"`
	MeanTimeToResolveSeconds     int64 `json:"mean_time_to_resolve_seconds"`
}

type IncidentNotificationReliabilityStats struct {
	TotalDeliveries      int64   `json:"total_deliveries"`
	SentDeliveries       int64   `json:"sent_deliveries"`
	FailedDeliveries     int64   `json:"failed_deliveries"`
	SuppressedDeliveries int64   `json:"suppressed_deliveries"`
	SuccessRatePercent   float64 `json:"success_rate_percent"`
}

type IncidentComponentImpactResponse struct {
	ComponentID   string `json:"component_id,omitempty"`
	ComponentName string `json:"component_name"`
	Status        string `json:"status,omitempty"`
	Impact        string `json:"impact,omitempty"`
}

type IncidentEventResponse struct {
	ID              string    `json:"id"`
	IncidentID      string    `json:"incident_id"`
	Type            string    `json:"type"`
	Message         string    `json:"message"`
	MonitorReportID string    `json:"monitor_report_id"`
	ActorType       string    `json:"actor_type"`
	ActorID         string    `json:"actor_id"`
	Note            string    `json:"note,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

type UptimeDayBucketResponse struct {
	Date          string  `json:"date"`
	Up            int     `json:"up"`
	Total         int     `json:"total"`
	UptimePercent float64 `json:"uptime_percent"`
}

type UptimeResponse struct {
	DailyBuckets  []UptimeDayBucketResponse `json:"daily_buckets"`
	UptimePercent float64                   `json:"uptime_percent"`
}

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

type AlertChannelResponse struct {
	ID                         string     `json:"id"`
	Name                       string     `json:"name"`
	Type                       string     `json:"type"`
	Enabled                    bool       `json:"enabled"`
	WebhookURL                 string     `json:"webhook_url,omitempty"`
	WebhookConfigured          bool       `json:"webhook_configured,omitempty"`
	WebhookSignatureConfigured bool       `json:"webhook_signature_configured,omitempty"`
	SubscribedEvents           []string   `json:"subscribed_events"`
	LastDeliveryStatus         string     `json:"last_delivery_status,omitempty"`
	LastDeliveryAt             *time.Time `json:"last_delivery_at,omitempty"`
	CreatedAt                  time.Time  `json:"created_at"`
	UpdatedAt                  time.Time  `json:"updated_at"`
}

type AlertRuleResponse struct {
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

type AlertRouteDryRunResponse struct {
	Event                service.AlertRouteContext          `json:"event"`
	LegacyFallback       bool                               `json:"legacy_fallback"`
	Suppressed           bool                               `json:"suppressed"`
	SuppressionReason    string                             `json:"suppression_reason,omitempty"`
	RouteEvaluations     []AlertRouteEvaluationResponse     `json:"route_evaluations"`
	DestinationDecisions []service.AlertDestinationDecision `json:"destination_decisions"`
}

type AlertRouteEvaluationResponse struct {
	Route      AlertRouteResponse `json:"route"`
	Matched    bool               `json:"matched"`
	Suppressed bool               `json:"suppressed"`
	Reasons    []string           `json:"reasons"`
}

type AlertRuleDryRunResponse struct {
	Event                AlertRuleDryRunContext         `json:"event"`
	LegacyFallback       bool                           `json:"legacy_fallback"`
	Suppressed           bool                           `json:"suppressed"`
	SuppressionReason    string                         `json:"suppression_reason,omitempty"`
	RuleEvaluations      []AlertRuleEvaluationResponse  `json:"rule_evaluations"`
	DestinationDecisions []AlertRuleDestinationDecision `json:"destination_decisions"`
}

type AlertRuleDryRunContext struct {
	IncidentID  string `json:"incident_id"`
	EventType   string `json:"event_type"`
	Severity    string `json:"severity"`
	AgentID     string `json:"agent_id"`
	MonitorID   string `json:"monitor_id"`
	MonitorType string `json:"monitor_type"`
}

type AlertRuleEvaluationResponse struct {
	Rule       AlertRuleResponse `json:"rule"`
	Matched    bool              `json:"matched"`
	Suppressed bool              `json:"suppressed"`
	Reasons    []string          `json:"reasons"`
}

type AlertRuleDestinationDecision struct {
	RuleID      string `json:"rule_id,omitempty"`
	RuleName    string `json:"rule_name,omitempty"`
	ChannelID   string `json:"channel_id,omitempty"`
	ChannelName string `json:"channel_name"`
	ChannelType string `json:"channel_type"`
	Status      string `json:"status"`
	Reason      string `json:"reason"`
}

type IncidentTimelineItemResponse struct {
	ID              string    `json:"id"`
	Type            string    `json:"type"`
	Source          string    `json:"source"`
	Message         string    `json:"message"`
	Evidence        string    `json:"evidence,omitempty"`
	MonitorReportID string    `json:"monitor_report_id,omitempty"`
	AlertDeliveryID string    `json:"alert_delivery_id,omitempty"`
	ActorType       string    `json:"actor_type,omitempty"`
	ActorID         string    `json:"actor_id,omitempty"`
	Note            string    `json:"note,omitempty"`
	Channel         string    `json:"channel,omitempty"`
	Status          string    `json:"status,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}
