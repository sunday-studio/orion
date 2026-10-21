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
	LockedBy      string                 `json:"-" gorm:"not null;default:''"`
	LockedUntil   *time.Time             `json:"-"`
	Attempts      []AlertDeliveryAttempt `json:"attempts,omitempty" gorm:"foreignKey:AlertDeliveryID"`
	CreatedAt     time.Time              `json:"created_at" gorm:"index:idx_alert_deliveries_created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
}

type AlertDeliveryAttempt struct {
	ID              string `json:"id" gorm:"primaryKey;type:varchar(255)"`
	AlertDeliveryID string `json:"alert_delivery_id" gorm:"index:idx_alert_delivery_attempts_delivery_id;not null"`
	AttemptNumber   int    `json:"attempt_number" gorm:"not null;index:idx_alert_delivery_attempts_delivery_number"`
	Status          string `json:"status" gorm:"not null"` // pending | sent | failed
	Stage           string `json:"stage" gorm:"not null"`  // load_incident | serialize | http_request | http_response | channel_lookup | transport
	Error           string `json:"error" gorm:"type:text"`
	// HTTPStatus is the webhook response code (0 when none was received); ResponseSnippet its first bytes.
	HTTPStatus      int        `json:"http_status" gorm:"not null;default:0"`
	ResponseSnippet string     `json:"-" gorm:"type:text;not null;default:''"`
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
	ID                   string `json:"id" gorm:"primaryKey;type:varchar(255)"`
	Name                 string `json:"name" gorm:"uniqueIndex;not null"`
	Enabled              bool   `json:"enabled" gorm:"not null;default:true;index:idx_alert_routes_enabled"`
	Priority             int    `json:"priority" gorm:"not null;default:100;index:idx_alert_routes_priority"`
	EventTypes           string `json:"event_types" gorm:"type:text"`
	Severities           string `json:"severities" gorm:"type:text"`
	NodeIDs              string `json:"node_ids" gorm:"type:text"`
	MonitorIDs           string `json:"monitor_ids" gorm:"type:text"`
	MonitorTypes         string `json:"monitor_types" gorm:"type:text"`
	ChannelIDs           string `json:"channel_ids" gorm:"type:text"`
	Suppress             bool   `json:"suppress" gorm:"not null;default:false;index:idx_alert_routes_suppress"`
	GroupingPolicy       string `json:"grouping_policy" gorm:"not null;default:suppress;index:idx_alert_routes_grouping_policy"`
	GroupingDelaySeconds int    `json:"grouping_delay_seconds" gorm:"not null;default:300"`
	// CooldownSeconds limits repeat notifications per incident and channel for this rule; 0 uses the global default.
	CooldownSeconds int       `json:"cooldown_seconds" gorm:"not null;default:0"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type AlertChannel struct {
	ID                   string `json:"id" gorm:"primaryKey;type:varchar(255)"`
	Name                 string `json:"name" gorm:"uniqueIndex;not null"`
	Type                 string `json:"type" gorm:"not null"` // webhook
	Enabled              bool   `json:"enabled" gorm:"not null"`
	WebhookURL           string `json:"webhook_url" gorm:"type:text"`
	WebhookSigningSecret string `json:"webhook_signing_secret" gorm:"type:text"`
	// HeadersJSON holds custom webhook request headers as a JSON object; values are never returned by the API.
	HeadersJSON string `json:"-" gorm:"type:text;not null;default:'{}'"`
	// EmailDeliveryMode is instant, or digest_15m (non-critical alerts wait for the next 15 minute mark).
	EmailDeliveryMode  string    `json:"email_delivery_mode" gorm:"not null;default:instant"`
	EmailSubjectPrefix string    `json:"email_subject_prefix" gorm:"not null;default:''"`
	SubscribedEvents   string    `json:"subscribed_events" gorm:"type:text"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// AlertChannelRecipient is one address on an email alert channel. Alerts go only to
// recipients whose status is "confirmed" (double opt-in).
type AlertChannelRecipient struct {
	ID                 string     `json:"id" gorm:"primaryKey;type:varchar(255)"`
	ChannelID          string     `json:"channel_id" gorm:"not null;uniqueIndex:idx_alert_channel_recipients_unique"`
	Email              string     `json:"email" gorm:"not null;uniqueIndex:idx_alert_channel_recipients_unique"`
	Status             string     `json:"status" gorm:"not null;default:pending"` // pending | confirmed | unsubscribed
	ConfirmedAt        *time.Time `json:"confirmed_at"`
	ConfirmationSentAt *time.Time `json:"confirmation_sent_at"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

// AlertDeliveryRecipient records which recipients already received a delivery so a
// retry after a partial SMTP failure never emails anyone twice.
type AlertDeliveryRecipient struct {
	DeliveryID  string    `gorm:"primaryKey"`
	RecipientID string    `gorm:"primaryKey"`
	SentAt      time.Time `gorm:"not null"`
}

// EnrollmentSession is a one-time, expiring node registration token created by the
// "Add server" wizard. Only the SHA-256 of the token is stored. The first machine to
// register with it binds MachineID; that machine may retry until expiry, nobody else can.
type EnrollmentSession struct {
	ID          string `gorm:"primaryKey"`
	TokenHash   string `gorm:"not null;uniqueIndex:idx_enrollment_sessions_token_hash"`
	Platform    string `gorm:"not null;default:''"`
	MachineID   string `gorm:"not null;default:''"`
	NodeID      string `gorm:"not null;default:''"`
	ClaimedAt   *time.Time
	CancelledAt *time.Time
	ExpiresAt   time.Time `gorm:"not null"`
	CreatedAt   time.Time `gorm:"not null"`
}

// NodeMonitorDefinition is a check the operator configured in the Console for the node to run
// ("Node monitor" setup). The node pulls its definitions and reports under the same name.
type NodeMonitorDefinition struct {
	ID              string    `gorm:"primaryKey"`
	NodeID          string    `gorm:"not null;uniqueIndex:idx_node_monitor_definitions_name"`
	Name            string    `gorm:"not null;uniqueIndex:idx_node_monitor_definitions_name"`
	CheckType       string    `gorm:"not null"` // disk | memory | process | port
	Params          string    `gorm:"type:text;not null;default:'{}'"`
	IntervalSeconds int       `gorm:"not null;default:30"`
	FailAfter       int       `gorm:"not null;default:3"`
	CreatedBy       string    `gorm:"not null;default:''"`
	CreatedAt       time.Time `gorm:"not null"`
	// DeliveredAt is when the node first pulled this definition; FailureReason is why the node
	// could not run it (verbatim from the node), empty when it can.
	DeliveredAt   *time.Time
	FailureReason string `gorm:"not null;default:''"`
}

// NodeDiscovery is the latest read-only scan a node made of its own host: what it found that could
// be monitored and how scanning each kind went. One row per node, replaced on every report.
type NodeDiscovery struct {
	NodeID    string `gorm:"primaryKey"`
	ScannedAt *time.Time
	Enabled   bool   `gorm:"not null"` // no gorm default: a first report of false must store false
	Kinds     string `gorm:"type:text;not null;default:'[]'"`
	Items     string `gorm:"type:text;not null;default:'[]'"`
	// Capabilities lists what this node can do for the Console ("requests", "log_key_fetch").
	Capabilities string    `gorm:"type:text;not null;default:'[]'"`
	ReceivedAt   time.Time `gorm:"not null"`
}

// NodeDiscoveryRequest is something the Console asked the node to do about discovery (rescan,
// dismiss, set a scan switch, ...). Core queues it; the node pulls it, applies it and reports back.
type NodeDiscoveryRequest struct {
	ID          string    `gorm:"primaryKey"`
	NodeID      string    `gorm:"not null;index:idx_node_discovery_requests_node"`
	Type        string    `gorm:"not null"`
	Payload     string    `gorm:"type:text;not null;default:'{}'"`
	Status      string    `gorm:"not null"`                      // queued | delivered | applied | failed | expired | unsupported | cancelled
	Result      string    `gorm:"type:text;not null;default:''"` // the node's words, verbatim
	CreatedBy   string    `gorm:"not null;default:''"`
	CreatedAt   time.Time `gorm:"not null;index:idx_node_discovery_requests_node"`
	DeliveredAt *time.Time
	FinishedAt  *time.Time
	ExpiresAt   time.Time `gorm:"not null"`
}

// AlertLabelRoute picks channels by label (the Routes tab): matchers are ANDed; routes are tried in
// position order and the first match wins unless ContinueMatching is set.
type AlertLabelRoute struct {
	ID               string    `json:"id" gorm:"primaryKey"`
	Position         int       `json:"position" gorm:"not null;default:0;index:idx_alert_label_routes_position"`
	Name             string    `json:"name" gorm:"not null;default:''"`
	Enabled          bool      `json:"enabled" gorm:"not null;default:true"`
	Matchers         string    `json:"matchers" gorm:"type:text;not null;default:'[]'"`
	ChannelIDs       string    `json:"channel_ids" gorm:"type:text;not null;default:'[]'"`
	ContinueMatching bool      `json:"continue_matching" gorm:"not null;default:false"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}
