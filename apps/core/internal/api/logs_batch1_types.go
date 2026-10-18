package api

import "time"

// LogFacetCount is one value in the filter rail and how many entries have it.
type LogFacetCount struct {
	Value string `json:"value"`
	// Label is the display name (an app's name for the source facet).
	Label string `json:"label,omitempty"`
	Count int64  `json:"count"`
}

// LogFacetsResponse feeds the filter rail: severity, source, event, environment and field facets.
type LogFacetsResponse struct {
	Total    int64            `json:"total"`
	Severity map[string]int64 `json:"severity"`
	Sources  []LogFacetCount  `json:"sources"`
	Events   []LogFacetCount  `json:"events"`
	// EventsDistinct is how many different events exist, for "Show 41 more".
	EventsDistinct int             `json:"events_distinct"`
	Environments   []LogFacetCount `json:"environments"`
	Fields         []LogFieldStat  `json:"fields"`
	// Values lists the most common values when the field query parameter is set.
	Values []LogFacetCount `json:"values,omitempty"`
}

// LogPattern is a group of similar messages.
type LogPattern struct {
	Pattern   string      `json:"pattern" example:"Delivery failed: <n> returned <n>"`
	Severity  LogSeverity `json:"severity"`
	Count     int64       `json:"count"`
	Trend     []int64     `json:"trend"`
	FirstSeen time.Time   `json:"first_seen"`
	LastSeen  time.Time   `json:"last_seen"`
	IsNew     bool        `json:"is_new"`
	SampleID  string      `json:"sample_id,omitempty"`
}

type LogPatternsResponse struct {
	Patterns []LogPattern `json:"patterns"`
	From     time.Time    `json:"from"`
	To       time.Time    `json:"to"`
}

// LogRelatedResponse lists the entries that belong with one entry.
type LogRelatedResponse struct {
	// By is request_id, trace_id or nearby.
	By      string     `json:"by" enums:"request_id,trace_id,nearby"`
	Value   string     `json:"value,omitempty"`
	Total   int64      `json:"total"`
	Entries []LogEntry `json:"entries"`
}

// LogIngestStatus drives the "Ingest is behind" banner and the Apps header cards.
type LogIngestStatus struct {
	Behind bool `json:"behind"`
	// LagSeconds estimates how long newly accepted logs wait before they are searchable.
	LagSeconds  int   `json:"lag_seconds"`
	RateLimited bool  `json:"rate_limited"`
	StorageFull bool  `json:"storage_full"`
	QueueDepth  int64 `json:"queue_depth"`
	QueueMax    int64 `json:"queue_max"`
	LinesToday  int64 `json:"lines_today"`
	// Dropped counts entries dropped since Core started.
	Dropped      int64 `json:"dropped"`
	StorageBytes int64 `json:"storage_bytes"`
	// StorageLimitBytes is the sum of the apps' size caps; 0 when any app has no cap.
	StorageLimitBytes int64 `json:"storage_limit_bytes"`
}

// RedactionRule is one custom redaction rule.
type RedactionRule struct {
	Type        string `json:"type" enums:"key,regex"`
	Pattern     string `json:"pattern" example:"card_number"`
	ReplaceWith string `json:"replace_with,omitempty" example:"[redacted]"`
}

// RedactionTestRequest tries rules on a sample without saving anything.
type RedactionTestRequest struct {
	// Sample is a JSON value (object, array or string).
	Sample interface{} `json:"sample" swaggertype:"object"`
	// Redaction overrides the app's saved rules; omit to test what is saved.
	Redaction *RedactionSettings `json:"redaction,omitempty"`
}

type RedactionTestResponse struct {
	Redacted interface{} `json:"redacted" swaggertype:"object"`
	// Changed is true when at least one value differs from the sample.
	Changed bool `json:"changed"`
}

type RedactionDefaultsResponse struct {
	Keys []string `json:"keys"`
}

// SearchTimeoutBody is returned with HTTP 504 when a log query exceeds its time limit.
type SearchTimeoutBody struct {
	Code string `json:"code" example:"search_timeout"`
}
