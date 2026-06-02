package db

import (
	"time"
)

type AlertDelivery struct {
	ID            string                 `json:"id" gorm:"primaryKey;type:varchar(255)"`
	IncidentID    string                 `json:"incident_id" gorm:"index:idx_alert_deliveries_incident_id;not null"`
	RouteID       string                 `json:"route_id" gorm:"not null;default:'';index:idx_alert_deliveries_route_id"`
	AlertGroupID  string                 `json:"alert_group_id" gorm:"not null;default:'';index:idx_alert_deliveries_alert_group_id"`
	EventType     string                 `json:"event_type" gorm:"not null"` // incident_opened | incident_resolved | test
	Channel       string                 `json:"channel" gorm:"not null"`
	Type          string                 `json:"type" gorm:"not null"`   // webhook | none | route | group
	Status        string                 `json:"status" gorm:"not null"` // pending | sent | failed | suppressed | cooldown
	Error         string                 `json:"error" gorm:"type:text"`
	AttemptCount  int                    `json:"attempt_count" gorm:"not null;default:0"`
	MaxAttempts   int                    `json:"max_attempts" gorm:"not null;default:3"`
	NextAttemptAt *time.Time             `json:"next_attempt_at" gorm:"index:idx_alert_deliveries_next_attempt_at"`
	LastAttemptAt *time.Time             `json:"last_attempt_at"`
	Attempts      []AlertDeliveryAttempt `json:"attempts,omitempty" gorm:"foreignKey:AlertDeliveryID"`
	CreatedAt     time.Time              `json:"created_at" gorm:"index:idx_alert_deliveries_created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
}

type AlertDeliveryAttempt struct {
	ID              string     `json:"id" gorm:"primaryKey;type:varchar(255)"`
	AlertDeliveryID string     `json:"alert_delivery_id" gorm:"index:idx_alert_delivery_attempts_delivery_id;not null"`
	AttemptNumber   int        `json:"attempt_number" gorm:"not null;index:idx_alert_delivery_attempts_delivery_number"`
	Status          string     `json:"status" gorm:"not null"` // pending | sent | failed
	Stage           string     `json:"stage" gorm:"not null"`  // load_incident | serialize | http_request | http_response | channel_lookup | transport
	Error           string     `json:"error" gorm:"type:text"`
	StartedAt       time.Time  `json:"started_at" gorm:"not null;index:idx_alert_delivery_attempts_started_at"`
	CompletedAt     *time.Time `json:"completed_at"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type AlertGroup struct {
	ID              string     `json:"id" gorm:"primaryKey;type:varchar(255)"`
	GroupKey        string     `json:"group_key" gorm:"not null;index:idx_alert_groups_group_key_status"`
	Status          string     `json:"status" gorm:"not null;index:idx_alert_groups_status"` // open | resolved
	EventType       string     `json:"event_type" gorm:"not null"`
	Severity        string     `json:"severity" gorm:"not null;index:idx_alert_groups_severity"`
	Summary         string     `json:"summary" gorm:"type:text"`
	FirstIncidentID string     `json:"first_incident_id" gorm:"not null"`
	LastIncidentID  string     `json:"last_incident_id" gorm:"not null"`
	IncidentCount   int        `json:"incident_count" gorm:"not null;default:0"`
	FirstEventAt    time.Time  `json:"first_event_at" gorm:"not null"`
	LastEventAt     time.Time  `json:"last_event_at" gorm:"not null;index:idx_alert_groups_last_event_at"`
	ResolvedAt      *time.Time `json:"resolved_at"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type AlertGroupMember struct {
	ID           string    `json:"id" gorm:"primaryKey;type:varchar(255)"`
	AlertGroupID string    `json:"alert_group_id" gorm:"not null;index:idx_alert_group_members_group_incident,unique"`
	IncidentID   string    `json:"incident_id" gorm:"not null;index:idx_alert_group_members_group_incident,unique;index:idx_alert_group_members_incident_id"`
	CreatedAt    time.Time `json:"created_at"`
}

type AlertRoute struct {
	ID                   string    `json:"id" gorm:"primaryKey;type:varchar(255)"`
	Name                 string    `json:"name" gorm:"uniqueIndex;not null"`
	Enabled              bool      `json:"enabled" gorm:"not null;default:true;index:idx_alert_routes_enabled"`
	Priority             int       `json:"priority" gorm:"not null;default:100;index:idx_alert_routes_priority"`
	EventTypes           string    `json:"event_types" gorm:"type:text"`
	Severities           string    `json:"severities" gorm:"type:text"`
	AgentIDs             string    `json:"agent_ids" gorm:"type:text"`
	MonitorIDs           string    `json:"monitor_ids" gorm:"type:text"`
	MonitorTypes         string    `json:"monitor_types" gorm:"type:text"`
	ChannelIDs           string    `json:"channel_ids" gorm:"type:text"`
	Suppress             bool      `json:"suppress" gorm:"not null;default:false;index:idx_alert_routes_suppress"`
	GroupingPolicy       string    `json:"grouping_policy" gorm:"not null;default:suppress;index:idx_alert_routes_grouping_policy"`
	GroupingDelaySeconds int       `json:"grouping_delay_seconds" gorm:"not null;default:300"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}

type AlertChannel struct {
	ID                   string    `json:"id" gorm:"primaryKey;type:varchar(255)"`
	Name                 string    `json:"name" gorm:"uniqueIndex;not null"`
	Type                 string    `json:"type" gorm:"not null"` // webhook
	Enabled              bool      `json:"enabled" gorm:"not null"`
	WebhookURL           string    `json:"webhook_url" gorm:"type:text"`
	WebhookSigningSecret string    `json:"webhook_signing_secret" gorm:"type:text"`
	SubscribedEvents     string    `json:"subscribed_events" gorm:"type:text"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`
}
