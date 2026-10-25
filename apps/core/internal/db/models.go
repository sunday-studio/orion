package db

import (
	"encoding/json"
	"time"

	"gorm.io/datatypes"
)

const (
	AlertEventIncidentOpened   = "incident_opened"
	AlertEventIncidentResolved = "incident_resolved"

	AlertGroupingPolicySuppress       = "suppress"
	AlertGroupingPolicyDelayedSummary = "delayed_summary"
	AlertGroupingPolicyNone           = "none"
	DefaultAlertGroupingDelaySeconds  = 300
)

func SupportedAlertEvents() []string {
	return []string{AlertEventIncidentOpened, AlertEventIncidentResolved}
}

func DefaultAlertEvents() []string {
	return SupportedAlertEvents()
}

func ValidAlertEvent(event string) bool {
	for _, supported := range SupportedAlertEvents() {
		if event == supported {
			return true
		}
	}
	return false
}

func EncodeAlertEvents(events []string) string {
	if len(events) == 0 {
		events = DefaultAlertEvents()
	}
	body, err := json.Marshal(events)
	if err != nil {
		return `["incident_opened","incident_resolved"]`
	}
	return string(body)
}

func DecodeAlertEvents(value string) []string {
	if value == "" {
		return DefaultAlertEvents()
	}
	var events []string
	if err := json.Unmarshal([]byte(value), &events); err != nil || len(events) == 0 {
		return DefaultAlertEvents()
	}
	filtered := make([]string, 0, len(events))
	seen := map[string]bool{}
	for _, event := range events {
		if !ValidAlertEvent(event) || seen[event] {
			continue
		}
		seen[event] = true
		filtered = append(filtered, event)
	}
	if len(filtered) == 0 {
		return DefaultAlertEvents()
	}
	return filtered
}

type GeoLocation struct {
	IP       string `json:"ip"`
	Hostname string `json:"hostname"`
	City     string `json:"city"`
	Region   string `json:"region"`
	Country  string `json:"country"`
	Loc      string `json:"loc"`
	Org      string `json:"org"`
	Postal   string `json:"postal"`
	Timezone string `json:"timezone"`
}

type CPUStats struct {
	Cores        int     `json:"cores"`
	UsagePercent float64 `json:"usage_percent"`
	Load1        float64 `json:"load_1"`
	Load5        float64 `json:"load_5"`
	Load15       float64 `json:"load_15"`
}

type MemoryStats struct {
	TotalBytes     uint64  `json:"total_bytes"`
	UsedBytes      uint64  `json:"used_bytes"`
	FreeBytes      uint64  `json:"free_bytes"`
	AvailableBytes uint64  `json:"available_bytes"`
	UsedPercent    float64 `json:"used_percent"`
}

type DiskStats struct {
	TotalBytes  uint64  `json:"total_bytes"`
	UsedBytes   uint64  `json:"used_bytes"`
	FreeBytes   uint64  `json:"free_bytes"`
	UsedPercent float64 `json:"used_percent"`
}

// one node to one machine/server
type Node struct {
	ID string `json:"id" gorm:"primaryKey"`
	// System marks Core's own owner node; it is not a user server and is hidden from counts and default lists.
	System                bool       `json:"system" gorm:"not null;default:false"`
	MachineId             string     `json:"machine_id" gorm:"uniqueIndex;not null"`
	Name                  string     `json:"name" gorm:"not null"`
	OS                    string     `json:"os" gorm:"not null"`
	Platform              string     `json:"platform"`
	KernelVersion         string     `json:"kernel_version"`
	Arch                  string     `json:"arch" gorm:"not null"`
	Token                 string     `json:"token" gorm:"uniqueIndex;not null"`
	TokenHash             string     `json:"token_hash" gorm:"not null;default:'';index:idx_nodes_token_hash"`
	TokenVersion          int        `json:"token_version" gorm:"not null;default:1"`
	TokenRotatedAt        *time.Time `json:"token_rotated_at"`
	TokenRevokedAt        *time.Time `json:"token_revoked_at" gorm:"index:idx_nodes_token_revoked_at"`
	TokenRevocationReason string     `json:"token_revocation_reason" gorm:"type:text;not null;default:''"`
	// PreviousTokenHash keeps the replaced token valid until PreviousTokenExpiresAt after a rotation,
	// so the node can be updated without a gap.
	PreviousTokenHash      string     `json:"-" gorm:"not null;default:''"`
	PreviousTokenExpiresAt *time.Time `json:"-"`
	// Labels is a JSON array of "key=value" strings; Notes is free text for operators.
	Labels                   string                          `json:"labels" gorm:"type:text;not null;default:'[]'"`
	Notes                    string                          `json:"notes" gorm:"type:text;not null;default:''"`
	MaintenanceMode          bool                            `json:"maintenance_mode" gorm:"default:false"`
	MaintenanceUntil         *time.Time                      `json:"maintenance_until"`
	ReportingIntervalSeconds int                             `json:"reporting_interval_seconds" gorm:"default:60"` // System metrics reporting interval
	CreatedAt                time.Time                       `json:"created_at"`
	DeletedAt                time.Time                       `json:"deleted_at"`
	LastSeen                 time.Time                       `json:"last_seen"`
	Location                 datatypes.JSONType[GeoLocation] `json:"location" gorm:"type:json"`
	Meta                     string                          `json:"meta" gorm:"type:text"`
}

type NodeReport struct {
	ID            string                          `json:"id" gorm:"primaryKey;type:varchar(255)"`
	NodeID        string                          `json:"node_id" gorm:"index:idx_node_reports_node_id;not null"`
	CreatedAt     time.Time                       `json:"created_at" gorm:"index:idx_node_reports_created_at"`
	NodeVersion   string                          `json:"node_version"`
	ConfigSummary string                          `json:"config_summary" gorm:"type:text"`
	UptimeSeconds uint64                          `json:"uptime_seconds"`
	Timestamp     string                          `json:"timestamp"`
	CPU           datatypes.JSONType[CPUStats]    `json:"cpu" gorm:"type:json"`
	Memory        datatypes.JSONType[MemoryStats] `json:"memory" gorm:"type:json"`
	Disk          datatypes.JSONType[DiskStats]   `json:"disk" gorm:"type:json"`
	Location      datatypes.JSONType[GeoLocation] `json:"location" gorm:"type:json"`
}

type ServiceLogEntry struct {
	ID          string    `json:"id" gorm:"primaryKey;type:varchar(255)"`
	NodeID      string    `json:"node_id" gorm:"not null;index:idx_service_log_entries_node_time;uniqueIndex:idx_service_log_entries_node_fingerprint"`
	MonitorID   string    `json:"monitor_id" gorm:"not null;default:'';index:idx_service_log_entries_monitor_time"`
	Source      string    `json:"source" gorm:"not null;default:'node';index:idx_service_log_entries_source_time"`
	Stream      string    `json:"stream" gorm:"not null;default:'jsonl'"`
	Level       string    `json:"level" gorm:"not null;default:'INFO';index:idx_service_log_entries_level_time"`
	Component   string    `json:"component" gorm:"not null;default:'';index:idx_service_log_entries_component_time"`
	MonitorName string    `json:"monitor_name" gorm:"not null;default:''"`
	Message     string    `json:"message" gorm:"type:text;not null;default:''"`
	FieldsJSON  string    `json:"fields_json" gorm:"type:text;not null;default:'{}'"`
	Raw         string    `json:"raw" gorm:"type:text;not null;default:''"`
	Fingerprint string    `json:"fingerprint" gorm:"not null;uniqueIndex:idx_service_log_entries_node_fingerprint"`
	OccurredAt  time.Time `json:"occurred_at" gorm:"not null;index:idx_service_log_entries_node_time;index:idx_service_log_entries_monitor_time;index:idx_service_log_entries_source_time;index:idx_service_log_entries_level_time;index:idx_service_log_entries_component_time"`
	CollectedAt time.Time `json:"collected_at" gorm:"not null;index:idx_service_log_entries_collected_at"`
	CreatedAt   time.Time `json:"created_at" gorm:"index:idx_service_log_entries_created_at"`
}

type Monitor struct {
	ID string `json:"id" gorm:"primaryKey"`
	// System marks Core's self-monitoring (worker liveness, node connectivity); hidden from counts and default lists.
	System                   bool       `json:"system" gorm:"not null;default:false"`
	Description              *string    `json:"description"`
	Type                     string     `json:"type" gorm:"not null"`
	Name                     string     `json:"name" gorm:"not null"`
	NodeID                   string     `json:"node_id" gorm:"index:idx_monitors_node_id;not null"`
	LastSuccessfulReportAt   *time.Time `json:"last_successful_report_at"`
	ReportingIntervalSeconds int        `json:"reporting_interval_seconds" gorm:"default:60"` // Monitor check interval
	ComputedHealth           string     `json:"computed_health" gorm:"default:unknown"`       // Cached computed health (up | down | degraded | unknown)
	LastHealthComputation    *time.Time `json:"last_health_computation"`                      // When health was last computed
	ActiveIncidentID         string     `json:"active_incident_id" gorm:"not null;default:'';index:idx_monitors_active_incident_id"`
	IncidentState            string     `json:"incident_state" gorm:"not null;default:unknown;index:idx_monitors_incident_state"`

	Lifecycle string `json:"lifecycle" gorm:"not null;index:idx_monitors_lifecycle"` // active | disabled | deleted
	// RunbookURL is shown on incidents for this monitor.
	RunbookURL string `json:"runbook_url" gorm:"not null;default:''"`
	// Maintenance window: checks keep running but incidents and alerts are held between the two times.
	MaintenanceStartsAt *time.Time `json:"maintenance_starts_at"`
	MaintenanceEndsAt   *time.Time `json:"maintenance_ends_at"`
	MaintenanceReason   string     `json:"maintenance_reason" gorm:"not null;default:''"`
	Health              string     `json:"health" gorm:"not null;index:idx_monitors_health"` // up | down | degraded | unknown
	Meta                string     `json:"meta" gorm:"type:text"`

	CreatedAt time.Time `json:"created_at" gorm:"index:idx_monitors_created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	DeletedAt time.Time `json:"deleted_at"`
}

type MonitorReport struct {
	ID          string `json:"id" gorm:"primaryKey;type:varchar(255)"`
	MonitorID   string `json:"monitor_id" gorm:"index:idx_monitor_reports_monitor_id;not null"`
	Payload     string `json:"payload" gorm:"type:text;not null"`
	CollectedAt string `json:"collected_at" gorm:"not null"`
	Health      string `json:"health" gorm:"not null"` // up | down
	// ClientReportID is the node-generated idempotency key; empty for reports without one.
	ClientReportID string    `json:"-" gorm:"not null;default:''"`
	CreatedAt      time.Time `json:"created_at" gorm:"index:idx_monitor_reports_created_at"`
}

type MonitorUptimeRollup struct {
	ID            int     `json:"id" gorm:"primaryKey"`
	MonitorID     string  `json:"monitor_id" gorm:"not null;uniqueIndex:idx_monitor_uptime_rollups_monitor_date"`
	Date          string  `json:"date" gorm:"not null;uniqueIndex:idx_monitor_uptime_rollups_monitor_date;index:idx_monitor_uptime_rollups_date"`
	UpCount       int     `json:"up_count" gorm:"not null;default:0"`
	DownCount     int     `json:"down_count" gorm:"not null;default:0"`
	DegradedCount int     `json:"degraded_count" gorm:"not null;default:0"`
	UnknownCount  int     `json:"unknown_count" gorm:"not null;default:0"`
	TotalCount    int     `json:"total_count" gorm:"not null;default:0"`
	UptimePercent float64 `json:"uptime_percent" gorm:"not null;default:0"`
	// MissedCount is checks that should have run but did not during silences longer than the stale
	// window, minus paused and maintenance time. They count as downtime.
	MissedCount int `json:"missed_count" gorm:"not null;default:0"`
	// IntervalSeconds is the monitor's check interval that day; it weights the monitor by time when
	// uptimes are combined. 0 on rows written before it existed.
	IntervalSeconds int       `json:"interval_seconds" gorm:"not null;default:0"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// UptimeExclusion is a period when a monitor was not expected to report (paused or in maintenance).
// Rows are written by database triggers; see migration 000056.
type UptimeExclusion struct {
	ID          int64  `gorm:"primaryKey"`
	SubjectType string // monitor_pause | monitor_maintenance | node_maintenance
	SubjectID   string
	StartsAt    time.Time
	EndsAt      *time.Time
}

type CoreMonitorConfig struct {
	MonitorID                 string     `json:"monitor_id" gorm:"primaryKey"`
	Kind                      string     `json:"kind" gorm:"not null;index:idx_core_monitor_configs_kind"`
	ConfigJSON                string     `json:"config_json" gorm:"type:text;not null;default:'{}'"`
	SecretRefJSON             string     `json:"secret_ref_json" gorm:"type:text;not null;default:'{}'"`
	HeartbeatTokenHash        string     `json:"heartbeat_token_hash" gorm:"not null;default:'';index:idx_core_monitor_configs_heartbeat_token_hash"`
	IntervalSeconds           int        `json:"interval_seconds" gorm:"not null;default:60"`
	TimeoutSeconds            int        `json:"timeout_seconds" gorm:"not null;default:10"`
	ConfirmationPeriodSeconds int        `json:"confirmation_period_seconds" gorm:"not null;default:0"`
	ConfirmationCheckCount    int        `json:"confirmation_check_count" gorm:"not null;default:0"`
	RecoveryPeriodSeconds     int        `json:"recovery_period_seconds" gorm:"not null;default:0"`
	Paused                    bool       `json:"paused" gorm:"not null;default:false;index:idx_core_monitor_configs_due"`
	PausedAt                  *time.Time `json:"paused_at"`
	PausedBy                  string     `json:"paused_by" gorm:"not null;default:''"`
	// HeartbeatTokenPrefix is the first characters of a heartbeat token, so the setup screen can show a masked URL.
	HeartbeatTokenPrefix string     `json:"-" gorm:"not null;default:''"`
	NextRunAt            time.Time  `json:"next_run_at" gorm:"not null;index:idx_core_monitor_configs_due"`
	LastRunAt            *time.Time `json:"last_run_at"`
	LastSignalAt         *time.Time `json:"last_signal_at"`
	LastSuccessAt        *time.Time `json:"last_success_at"`
	LastFailureAt        *time.Time `json:"last_failure_at"`
	LeaseOwner           string     `json:"lease_owner" gorm:"not null;default:'';index:idx_core_monitor_configs_lease_owner"`
	LeaseExpiresAt       *time.Time `json:"lease_expires_at" gorm:"index:idx_core_monitor_configs_due;index:idx_core_monitor_configs_lease_expires_at"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
	Monitor              Monitor    `json:"monitor,omitempty" gorm:"foreignKey:MonitorID;references:ID"`
}

type CoreWorkerStatus struct {
	WorkerID        string    `json:"worker_id" gorm:"primaryKey;type:varchar(255)"`
	ProcessKind     string    `json:"process_kind" gorm:"not null;default:core-monitor-worker"`
	Hostname        string    `json:"hostname" gorm:"not null;default:''"`
	Status          string    `json:"status" gorm:"not null;default:unknown"`
	Version         string    `json:"version" gorm:"not null;default:''"`
	StartedAt       time.Time `json:"started_at" gorm:"not null"`
	LastHeartbeatAt time.Time `json:"last_heartbeat_at" gorm:"not null;index:idx_core_worker_statuses_last_heartbeat_at"`
	LastError       string    `json:"last_error" gorm:"not null;default:'';type:text"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type IncidentComponentImpact struct {
	ComponentID   string `json:"component_id,omitempty"`
	ComponentName string `json:"component_name"`
	Status        string `json:"status,omitempty"`
	Impact        string `json:"impact,omitempty"`
}

type Incident struct {
	ID                 string     `json:"id" gorm:"primaryKey;type:varchar(255)"`
	Status             string     `json:"status" gorm:"not null;index:idx_incidents_status"` // open | acknowledged | covered | resolved
	Severity           string     `json:"severity" gorm:"not null;index:idx_incidents_severity"`
	Title              string     `json:"title" gorm:"not null"`
	NodeID             string     `json:"node_id" gorm:"index:idx_incidents_node_id;not null"`
	MonitorID          string     `json:"monitor_id" gorm:"index:idx_incidents_monitor_id;not null"`
	ImpactedComponents string     `json:"impacted_components" gorm:"type:text;not null;default:'[]'"`
	CoveredAt          *time.Time `json:"covered_at"`
	CoveredUntil       *time.Time `json:"covered_until" gorm:"index:idx_incidents_covered_until"`
	CoverageNote       string     `json:"coverage_note" gorm:"type:text"`
	ResolutionKind     string     `json:"resolution_kind" gorm:"not null;default:''"`
	// Number is the human id shown as INC-0231; assigned on creation, never reused.
	Number              int        `json:"number" gorm:"not null;default:0"`
	Responder           string     `json:"responder" gorm:"not null;default:''"`
	ResponderAt         *time.Time `json:"responder_at"`
	Summary             string     `json:"summary" gorm:"type:text;not null;default:''"`
	Source              string     `json:"source" gorm:"not null;default:monitor"` // monitor | manual
	CoveredByIncidentID string     `json:"covered_by_incident_id" gorm:"not null;default:''"`
	ReopenedAt          *time.Time `json:"reopened_at"`
	ReopenCount         int        `json:"reopen_count" gorm:"not null;default:0"`
	OpenedAt            time.Time  `json:"opened_at" gorm:"not null;index:idx_incidents_opened_at"`
	ResolvedAt          *time.Time `json:"resolved_at"`
	LastEventAt         time.Time  `json:"last_event_at" gorm:"not null"`
	LatestEvent         string     `json:"latest_event" gorm:"type:text"`
	NotificationStatus  string     `json:"notification_status" gorm:"not null;default:pending"` // pending | sent | failed | suppressed | cooldown
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

type IncidentEvent struct {
	ID              string    `json:"id" gorm:"primaryKey;type:varchar(255)"`
	IncidentID      string    `json:"incident_id" gorm:"index:idx_incident_events_incident_id;not null"`
	Type            string    `json:"type" gorm:"not null"`
	Message         string    `json:"message" gorm:"type:text"`
	MonitorReportID string    `json:"monitor_report_id" gorm:"index:idx_incident_events_monitor_report_id"`
	ActorType       string    `json:"actor_type" gorm:"not null;default:system;index:idx_incident_events_actor"`
	ActorID         string    `json:"actor_id" gorm:"not null;default:core;index:idx_incident_events_actor"`
	Note            string    `json:"note" gorm:"type:text;not null;default:''"`
	CreatedAt       time.Time `json:"created_at" gorm:"index:idx_incident_events_created_at"`
}

type AuditEvent struct {
	ID                 string    `json:"id" gorm:"primaryKey;type:varchar(255)"`
	Action             string    `json:"action" gorm:"not null;index:idx_audit_events_action"`
	StatusPageID       string    `json:"status_page_id" gorm:"not null;index:idx_audit_events_status_page_id"`
	AffectedObjectType string    `json:"affected_object_type" gorm:"not null;index:idx_audit_events_affected_object"`
	AffectedObjectID   string    `json:"affected_object_id" gorm:"not null;index:idx_audit_events_affected_object"`
	ActorType          string    `json:"actor_type" gorm:"not null"`
	ActorID            string    `json:"actor_id" gorm:"not null"`
	MetadataJSON       string    `json:"metadata_json" gorm:"type:text;not null;default:'{}'"`
	CreatedAt          time.Time `json:"created_at" gorm:"not null;index:idx_audit_events_created_at"`
}

type StatusPage struct {
	ID                        string     `json:"id" gorm:"primaryKey;type:varchar(255)"`
	Slug                      string     `json:"slug" gorm:"uniqueIndex;not null"`
	CustomDomain              string     `json:"custom_domain" gorm:"not null;default:'';index:idx_status_pages_custom_domain,unique,where:custom_domain <> ''"`
	Title                     string     `json:"title" gorm:"not null"`
	Description               string     `json:"description" gorm:"type:text"`
	SEOTitle                  string     `json:"seo_title"`
	SEODescription            string     `json:"seo_description" gorm:"type:text"`
	OpenGraphImageURL         string     `json:"open_graph_image_url" gorm:"type:text"`
	CanonicalURL              string     `json:"canonical_url" gorm:"type:text"`
	Visibility                string     `json:"visibility" gorm:"not null;default:draft;index:idx_status_pages_visibility"`
	ThemeSettings             string     `json:"theme_settings" gorm:"type:text;not null;default:'{}'"`
	DefaultIncidentVisibility string     `json:"default_incident_visibility" gorm:"not null;default:draft"`
	PublishedAt               *time.Time `json:"published_at" gorm:"index:idx_status_pages_published_at"`
	// Custom domain DNS verification: pending, verified or failed.
	DomainStatus     string     `json:"domain_status" gorm:"not null;default:pending"`
	DomainCheckedAt  *time.Time `json:"domain_checked_at"`
	DomainVerifiedAt *time.Time `json:"domain_verified_at"`
	DomainError      string     `json:"domain_error" gorm:"not null;default:''"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type StatusPageSection struct {
	ID                 string    `json:"id" gorm:"primaryKey;type:varchar(255)"`
	StatusPageID       string    `json:"status_page_id" gorm:"not null;index:idx_status_page_sections_page_sort"`
	Name               string    `json:"name" gorm:"not null"`
	SortOrder          int       `json:"sort_order" gorm:"not null;default:0;index:idx_status_page_sections_page_sort"`
	CollapsedByDefault bool      `json:"collapsed_by_default" gorm:"not null;default:false"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type StatusPageComponent struct {
	ID                 string    `json:"id" gorm:"primaryKey;type:varchar(255)"`
	StatusPageID       string    `json:"status_page_id" gorm:"not null;index:idx_status_page_components_page_visible_sort"`
	SectionID          string    `json:"section_id" gorm:"not null;index:idx_status_page_components_section_sort"`
	PublicName         string    `json:"public_name" gorm:"not null"`
	PublicDescription  string    `json:"public_description" gorm:"type:text"`
	DisplayMode        string    `json:"display_mode" gorm:"not null;default:single_resource"`
	ManualStatus       string    `json:"manual_status"`
	ManualStatusReason string    `json:"manual_status_reason" gorm:"type:text"`
	SortOrder          int       `json:"sort_order" gorm:"not null;default:0;index:idx_status_page_components_page_visible_sort;index:idx_status_page_components_section_sort"`
	Visible            bool      `json:"visible" gorm:"not null;default:true;index:idx_status_page_components_page_visible_sort"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

type StatusPageComponentMapping struct {
	ID                   string    `json:"id" gorm:"primaryKey;type:varchar(255)"`
	ComponentID          string    `json:"component_id" gorm:"not null;uniqueIndex:idx_status_page_component_mapping_unique;index:idx_status_page_component_mappings_component_id"`
	ResourceType         string    `json:"resource_type" gorm:"not null;uniqueIndex:idx_status_page_component_mapping_unique;index:idx_status_page_component_mappings_resource"`
	ResourceID           string    `json:"resource_id" gorm:"not null;uniqueIndex:idx_status_page_component_mapping_unique;index:idx_status_page_component_mappings_resource"`
	HealthRollupStrategy string    `json:"health_rollup_strategy" gorm:"not null;default:worst"`
	UptimeRollupStrategy string    `json:"uptime_rollup_strategy" gorm:"not null;default:worst"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

type StatusPageIncident struct {
	ID                   string     `json:"id" gorm:"primaryKey;type:varchar(255)"`
	StatusPageID         string     `json:"status_page_id" gorm:"not null;index:idx_status_page_incidents_page_visibility"`
	InternalIncidentID   string     `json:"internal_incident_id" gorm:"index:idx_status_page_incidents_internal_incident_id"`
	Title                string     `json:"title" gorm:"not null"`
	PublicStatus         string     `json:"public_status" gorm:"not null;index:idx_status_page_incidents_public_status"`
	Severity             string     `json:"severity" gorm:"not null"`
	ImpactSummary        string     `json:"impact_summary" gorm:"type:text"`
	Visibility           string     `json:"visibility" gorm:"not null;default:draft;index:idx_status_page_incidents_page_visibility"`
	AffectedComponentIDs string     `json:"affected_component_ids" gorm:"type:text;not null;default:'[]'"`
	PublishedAt          *time.Time `json:"published_at" gorm:"index:idx_status_page_incidents_published_at"`
	ResolvedAt           *time.Time `json:"resolved_at"`
	ScheduledStartAt     *time.Time `json:"scheduled_start_at"`
	ScheduledEndAt       *time.Time `json:"scheduled_end_at"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

type StatusPageIncidentUpdate struct {
	ID          string     `json:"id" gorm:"primaryKey;type:varchar(255)"`
	IncidentID  string     `json:"incident_id" gorm:"not null;index:idx_status_page_incident_updates_incident_published"`
	Status      string     `json:"status" gorm:"not null"`
	Message     string     `json:"message" gorm:"type:text;not null"`
	CreatedBy   string     `json:"created_by"`
	PublishedAt *time.Time `json:"published_at" gorm:"index:idx_status_page_incident_updates_incident_published"`
	CreatedAt   time.Time  `json:"created_at"`
}

type StatusPageSubscriber struct {
	ID                         string     `json:"id" gorm:"primaryKey;type:varchar(255)"`
	StatusPageID               string     `json:"status_page_id" gorm:"not null;uniqueIndex:idx_status_page_subscribers_destination;index:idx_status_page_subscribers_page_state"`
	DestinationType            string     `json:"destination_type" gorm:"not null;uniqueIndex:idx_status_page_subscribers_destination"`
	DestinationHash            string     `json:"destination_hash" gorm:"not null;uniqueIndex:idx_status_page_subscribers_destination"`
	DestinationValueCiphertext string     `json:"destination_value_ciphertext" gorm:"not null;default:''"`
	MaskedDestination          string     `json:"masked_destination" gorm:"not null"`
	State                      string     `json:"state" gorm:"not null;default:pending;index:idx_status_page_subscribers_page_state"`
	ConfirmationTokenHash      string     `json:"confirmation_token_hash" gorm:"not null;default:'';index:idx_status_page_subscribers_confirmation_token"`
	ConfirmationTokenExpiresAt *time.Time `json:"confirmation_token_expires_at"`
	ManageTokenHash            string     `json:"manage_token_hash" gorm:"not null;default:'';index:idx_status_page_subscribers_manage_token"`
	ManageTokenVersion         int        `json:"manage_token_version" gorm:"not null;default:1"`
	UnsubscribeTokenHash       string     `json:"unsubscribe_token_hash" gorm:"not null;default:'';index:idx_status_page_subscribers_unsubscribe_token"`
	UnsubscribeTokenVersion    int        `json:"unsubscribe_token_version" gorm:"not null;default:1"`
	BounceCount                int        `json:"bounce_count" gorm:"not null;default:0"`
	LastDeliveryStatus         string     `json:"last_delivery_status" gorm:"not null;default:''"`
	LastDeliveryAt             *time.Time `json:"last_delivery_at"`
	Source                     string     `json:"source" gorm:"not null;default:public_page"`
	ConfirmedAt                *time.Time `json:"confirmed_at"`
	UnsubscribedAt             *time.Time `json:"unsubscribed_at"`
	DisabledAt                 *time.Time `json:"disabled_at"`
	CreatedAt                  time.Time  `json:"created_at"`
	UpdatedAt                  time.Time  `json:"updated_at"`
}

type StatusPageSubscriberComponent struct {
	ID           string    `json:"id" gorm:"primaryKey;type:varchar(255)"`
	SubscriberID string    `json:"subscriber_id" gorm:"not null;uniqueIndex:idx_status_page_subscriber_components_unique;index:idx_status_page_subscriber_components_subscriber"`
	ComponentID  string    `json:"component_id" gorm:"not null;uniqueIndex:idx_status_page_subscriber_components_unique;index:idx_status_page_subscriber_components_component"`
	EventScope   string    `json:"event_scope" gorm:"not null;default:all_updates;uniqueIndex:idx_status_page_subscriber_components_unique"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type StatusPageSubscriberDelivery struct {
	ID                     string     `json:"id" gorm:"primaryKey;type:varchar(255)"`
	SubscriberID           string     `json:"subscriber_id" gorm:"not null;index:idx_status_page_subscriber_deliveries_subscriber"`
	StatusPageID           string     `json:"status_page_id" gorm:"not null;index:idx_status_page_subscriber_deliveries_page_state"`
	PublicIncidentID       string     `json:"public_incident_id" gorm:"not null;default:''"`
	PublicIncidentUpdateID string     `json:"public_incident_update_id" gorm:"not null;default:''"`
	DeliveryType           string     `json:"delivery_type" gorm:"not null"`
	DeliveryState          string     `json:"delivery_state" gorm:"not null;default:queued;index:idx_status_page_subscriber_deliveries_page_state"`
	ProviderMessageID      string     `json:"provider_message_id" gorm:"not null;default:''"`
	ErrorCode              string     `json:"error_code" gorm:"not null;default:''"`
	SafeErrorSummary       string     `json:"safe_error_summary" gorm:"not null;default:''"`
	AttemptCount           int        `json:"attempt_count" gorm:"not null;default:0"`
	QueuedAt               *time.Time `json:"queued_at"`
	SentAt                 *time.Time `json:"sent_at"`
	FailedAt               *time.Time `json:"failed_at"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
}

type DataLifecycleSettings struct {
	ID                  int        `json:"id" gorm:"primaryKey"`
	RawReportHotDays    int        `json:"raw_report_hot_days" gorm:"not null"`
	ArchiveRawReports   bool       `json:"archive_raw_reports" gorm:"not null"`
	ArchiveDir          string     `json:"archive_dir" gorm:"not null"`
	RollupsEnabled      bool       `json:"rollups_enabled" gorm:"not null"`
	RollupRetentionDays *int       `json:"rollup_retention_days"`
	ArchiveSchedule     string     `json:"archive_schedule" gorm:"not null"`
	LastRollupRunAt     *time.Time `json:"last_rollup_run_at"`
	LastArchiveRunAt    *time.Time `json:"last_archive_run_at"`
	LastArchiveStatus   string     `json:"last_archive_status"`
	LastArchiveError    string     `json:"last_archive_error" gorm:"type:text"`
	// Retention windows in days for tables that otherwise grow without bound.
	ServiceLogRetentionDays    int       `json:"service_log_retention_days" gorm:"not null;default:14"`
	AlertDeliveryRetentionDays int       `json:"alert_delivery_retention_days" gorm:"not null;default:90"`
	IncidentEventRetentionDays int       `json:"incident_event_retention_days" gorm:"not null;default:365"`
	AuditEventRetentionDays    int       `json:"audit_event_retention_days" gorm:"not null;default:365"`
	WorkerStatusRetentionDays  int       `json:"worker_status_retention_days" gorm:"not null;default:7"`
	CreatedAt                  time.Time `json:"created_at"`
	UpdatedAt                  time.Time `json:"updated_at"`
}

// IncidentCandidateDismissal hides an incident candidate (a failing monitor with no
// incident) from the review list until the monitor reports healthy again.
type IncidentCandidateDismissal struct {
	MonitorID   string    `json:"monitor_id" gorm:"primaryKey"`
	DismissedBy string    `json:"dismissed_by"`
	Reason      string    `json:"reason"`
	DismissedAt time.Time `json:"dismissed_at"`
}
