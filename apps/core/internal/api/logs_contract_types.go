package api

import "time"

// This file is the wire contract for product logs (docs/product-logs.md,
// docs/plans/product-logs-plan.md). Handlers return 501 until phase L1 implements
// them; the shapes below are stable so Console and app authors can build against them.

// LogSeverity is the canonical severity set. Ingest also accepts OpenTelemetry
// severity numbers (1-24) and common aliases (warning, err, critical, ...).
type LogSeverity string

const (
	LogSeverityTrace LogSeverity = "trace"
	LogSeverityDebug LogSeverity = "debug"
	LogSeverityInfo  LogSeverity = "info"
	LogSeverityWarn  LogSeverity = "warn"
	LogSeverityError LogSeverity = "error"
	LogSeverityFatal LogSeverity = "fatal"
)

// LogEntryInput is one log record sent by an app. Every field is optional except
// that at least one of body or event must be present. Unknown top-level fields are
// kept as attributes, so a plain JSON log line is a valid entry.
type LogEntryInput struct {
	// ID is an optional client-chosen idempotency key (unique per app). Re-sending the same id is a no-op.
	ID string `json:"id,omitempty" example:"01JBXK3Q8Z"`
	// Timestamp is RFC 3339 text, or a number of Unix seconds, milliseconds or nanoseconds. Defaults to the receive time.
	Timestamp interface{} `json:"timestamp,omitempty" swaggertype:"string" example:"2026-10-17T10:15:30.123Z"`
	// Severity is trace|debug|info|warn|error|fatal, an OpenTelemetry number (1-24), or a common alias. Unknown values become info.
	Severity interface{} `json:"severity,omitempty" swaggertype:"string" example:"error"`
	// Body is the human message: a string, or any JSON value (kept as structured data and searchable as text).
	Body interface{} `json:"body,omitempty" swaggertype:"string" example:"payment failed for order 8841"`
	// Event is a short stable name for product events, for example payment.failed.
	Event string `json:"event,omitempty" example:"payment.failed"`
	// Attributes is free-form JSON of any shape. Limits: depth 8, 128 keys per entry.
	Attributes map[string]interface{} `json:"attributes,omitempty"`
	// Resource describes the emitter: service.name, service.version, deployment.environment, host.name, ...
	Resource map[string]interface{} `json:"resource,omitempty"`
	TraceID  string                 `json:"trace_id,omitempty" example:"4bf92f3577b34da6a3ce929d0e0e4736"`
	SpanID   string                 `json:"span_id,omitempty" example:"00f067aa0ba902b7"`
	// RequestID correlates entries of one request or job.
	RequestID string `json:"request_id,omitempty" example:"req_7f3a"`
}

// IngestError reports why one entry of a batch was rejected. The rest of the batch is still accepted.
type IngestError struct {
	// Index is the zero-based position of the entry in the request (or the NDJSON line number minus one).
	Index   int    `json:"index" example:"3"`
	Code    string `json:"code" enums:"invalid_json,empty_entry,entry_too_large,invalid_timestamp,too_many_attributes,attributes_too_deep" example:"entry_too_large"`
	Message string `json:"message"`
}

// IngestResponse is returned with 202. Accepted entries are queued, not yet searchable (typically under a second).
type IngestResponse struct {
	Accepted int           `json:"accepted" example:"98"`
	Rejected int           `json:"rejected" example:"2"`
	Errors   []IngestError `json:"errors,omitempty"`
}

// LogEntry is a stored log record as returned by the read API.
type LogEntry struct {
	ID string `json:"id"`
	// Source is "app" for ingested product logs and "agent" for agent service logs.
	Source        string                 `json:"source" enums:"app,agent"`
	AppID         string                 `json:"app_id,omitempty"`
	AppName       string                 `json:"app_name,omitempty"`
	AgentID       string                 `json:"agent_id,omitempty"`
	Timestamp     time.Time              `json:"timestamp"`
	IngestedAt    time.Time              `json:"ingested_at"`
	Severity      LogSeverity            `json:"severity"`
	SeverityValue int                    `json:"severity_number" example:"17"`
	Event         string                 `json:"event,omitempty"`
	Body          string                 `json:"body"`
	BodyJSON      interface{}            `json:"body_json,omitempty" swaggertype:"object"`
	Attributes    map[string]interface{} `json:"attributes,omitempty"`
	Resource      map[string]interface{} `json:"resource,omitempty"`
	TraceID       string                 `json:"trace_id,omitempty"`
	SpanID        string                 `json:"span_id,omitempty"`
	RequestID     string                 `json:"request_id,omitempty"`
}

// LogListResponse is one page of results; pass NextCursor back as `cursor` for the next page.
type LogListResponse struct {
	Entries    []LogEntry `json:"entries"`
	NextCursor string     `json:"next_cursor,omitempty"`
	// Truncated is true when the time range matched more rows than the server will scan.
	Truncated bool `json:"truncated"`
}

// LogHistogramBucket is the count of entries per severity in one time bucket.
type LogHistogramBucket struct {
	Start    time.Time      `json:"start"`
	Total    int64          `json:"total"`
	Severity map[string]int `json:"severity"`
}

type LogHistogramResponse struct {
	BucketSeconds int                  `json:"bucket_seconds"`
	Buckets       []LogHistogramBucket `json:"buckets"`
}

// LogFieldStat describes one attribute key seen in an app's logs (the field catalog behind facets and columns).
type LogFieldStat struct {
	Key string `json:"key" example:"http.status_code"`
	// Type is the dominant JSON type observed.
	Type string `json:"type" enums:"string,number,boolean,object,array,mixed"`
	// Cardinality is an approximate distinct-value count, capped at 1000.
	Cardinality  int       `json:"cardinality"`
	SampleValues []string  `json:"sample_values,omitempty"`
	LastSeen     time.Time `json:"last_seen"`
}

type LogFieldCatalogResponse struct {
	Fields []LogFieldStat `json:"fields"`
}

// LogSavedView stores a filter set so it can be shared by URL.
type LogSavedView struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Filters   map[string]string `json:"filters"`
	CreatedAt time.Time         `json:"created_at"`
}

type LogSavedViewRequest struct {
	Name    string            `json:"name" binding:"required"`
	Filters map[string]string `json:"filters" binding:"required"`
}

// DisplayProfile lets an app choose how its logs render. Everything has an automatic default.
type DisplayProfile struct {
	// TitleField is the attribute shown as the row title instead of the body.
	TitleField string `json:"title_field,omitempty" example:"event"`
	// Columns are attribute keys shown as extra columns in the list.
	Columns []string `json:"columns,omitempty"`
	// Hidden are attribute keys removed from the detail view (they are still stored).
	Hidden []string `json:"hidden,omitempty"`
}

// RedactionSettings extends the default redaction (password, authorization, token, secret, api_key, cookie, ...).
type RedactionSettings struct {
	// ExtraKeys are additional attribute key names (case-insensitive, any depth) whose values are replaced by [redacted] before storage.
	ExtraKeys []string `json:"extra_keys,omitempty"`
	// DisableDefaults turns off the built-in key list. Not recommended.
	DisableDefaults bool `json:"disable_defaults,omitempty"`
}

// App is a product that sends logs to Orion.
type App struct {
	ID          string `json:"id"`
	Name        string `json:"name" example:"checkout-api"`
	Slug        string `json:"slug" example:"checkout-api"`
	Environment string `json:"environment,omitempty" example:"production"`
	Description string `json:"description,omitempty"`
	// RetentionDays defaults to 14.
	RetentionDays int `json:"retention_days" example:"14"`
	// MaxStorageMB caps stored log size for this app; the oldest entries are pruned first. 0 means the server default.
	MaxStorageMB int `json:"max_storage_mb"`
	// DebugSampleRate keeps this fraction (0-1) of trace and debug entries. Default 1.
	DebugSampleRate float64           `json:"debug_sample_rate" example:"1"`
	Display         DisplayProfile    `json:"display_profile"`
	Redaction       RedactionSettings `json:"redaction"`
	CreatedAt       time.Time         `json:"created_at"`
	LastIngestAt    *time.Time        `json:"last_ingest_at"`
	KeyCount        int               `json:"key_count"`
}

type AppRequest struct {
	Name            string             `json:"name" binding:"required"`
	Environment     string             `json:"environment,omitempty"`
	Description     string             `json:"description,omitempty"`
	RetentionDays   *int               `json:"retention_days,omitempty"`
	MaxStorageMB    *int               `json:"max_storage_mb,omitempty"`
	DebugSampleRate *float64           `json:"debug_sample_rate,omitempty"`
	Display         *DisplayProfile    `json:"display_profile,omitempty"`
	Redaction       *RedactionSettings `json:"redaction,omitempty"`
}

// AppUpdateRequest changes an app; every field is optional and omitted fields are unchanged.
type AppUpdateRequest struct {
	Name            string             `json:"name,omitempty"`
	Environment     string             `json:"environment,omitempty"`
	Description     string             `json:"description,omitempty"`
	RetentionDays   *int               `json:"retention_days,omitempty"`
	MaxStorageMB    *int               `json:"max_storage_mb,omitempty"`
	DebugSampleRate *float64           `json:"debug_sample_rate,omitempty"`
	Display         *DisplayProfile    `json:"display_profile,omitempty"`
	Redaction       *RedactionSettings `json:"redaction,omitempty"`
}

// IngestKey is a write-only credential for one app. Only a hash is stored.
type IngestKey struct {
	ID         string     `json:"id"`
	AppID      string     `json:"app_id"`
	Name       string     `json:"name" example:"production"`
	Prefix     string     `json:"prefix" example:"orion_ik_a1b2c3d4"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	RevokedAt  *time.Time `json:"revoked_at"`
}

type IngestKeyRequest struct {
	Name string `json:"name" binding:"required"`
}

// IngestKeyCreated carries the full key exactly once, at creation.
type IngestKeyCreated struct {
	IngestKey
	// Key is the secret to send as "Authorization: Bearer <key>". It cannot be retrieved again.
	Key string `json:"key" example:"orion_ik_a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0"`
}

type TestLogResponse struct {
	Accepted bool   `json:"accepted"`
	EntryID  string `json:"entry_id,omitempty"`
}
