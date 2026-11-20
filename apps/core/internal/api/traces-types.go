package api

import "time"

// TraceStatus is ok when no span of the trace failed, error when at least one did.
type TraceStatus string

const (
	TraceStatusOK    TraceStatus = "ok"
	TraceStatusError TraceStatus = "error"
)

// TraceListItem is the summary of the spans one app received for a trace id: one row of the trace list.
// A trace id seen by two apps is two rows; GET /v1/traces/{trace_id} joins them.
type TraceListItem struct {
	TraceID     string `json:"trace_id" example:"4bf92f3577b34da6a3ce929d0e0e4736"`
	AppID       string `json:"app_id"`
	AppName     string `json:"app_name"`
	ServiceName string `json:"service_name" example:"checkout-api"`
	// RootName is the name of the root span, or of the earliest span while the root has not arrived (has_root false).
	RootName  string    `json:"root_name" example:"POST /checkout"`
	StartTime time.Time `json:"start_time"`
	// DurationNs is the root span's duration, or the extent of all spans while has_root is false.
	DurationNs int64       `json:"duration_ns" example:"182000000"`
	SpanCount  int64       `json:"span_count"`
	ErrorCount int64       `json:"error_count"`
	HasRoot    bool        `json:"has_root"`
	Status     TraceStatus `json:"status" enums:"ok,error"`
}

// TraceListResponse is one page of traces, newest first.
type TraceListResponse struct {
	Traces     []TraceListItem `json:"traces"`
	NextCursor string          `json:"next_cursor"`
}

// TraceSpanEvent is a point-in-time annotation on a span, for example an exception.
type TraceSpanEvent struct {
	Time time.Time `json:"time"`
	// OffsetNs is the time since the trace's first span started.
	OffsetNs   int64                  `json:"offset_ns"`
	Name       string                 `json:"name"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

// TraceSpanLink points at a span of another trace.
type TraceSpanLink struct {
	TraceID    string                 `json:"trace_id"`
	SpanID     string                 `json:"span_id"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

// TraceSpan is one stored span.
type TraceSpan struct {
	SpanID       string    `json:"span_id" example:"00f067aa0ba902b7"`
	ParentSpanID string    `json:"parent_span_id" example:"b9c7c989f97918e1"`
	AppID        string    `json:"app_id"`
	AppName      string    `json:"app_name"`
	Name         string    `json:"name" example:"SELECT orders"`
	Kind         string    `json:"kind" enums:"unspecified,internal,server,client,producer,consumer"`
	ServiceName  string    `json:"service_name"`
	StartTime    time.Time `json:"start_time"`
	// StartOffsetNs is the time since the trace's first span started: draw the waterfall from it.
	StartOffsetNs int64                  `json:"start_offset_ns"`
	DurationNs    int64                  `json:"duration_ns"`
	Status        string                 `json:"status" enums:"unset,ok,error"`
	StatusMessage string                 `json:"status_message,omitempty"`
	Attributes    map[string]interface{} `json:"attributes,omitempty"`
	Resource      map[string]interface{} `json:"resource,omitempty"`
	Events        []TraceSpanEvent       `json:"events"`
	Links         []TraceSpanLink        `json:"links"`
}

// TraceDetailResponse is every stored span of one trace id, across apps, with the logs that carry the id counted.
type TraceDetailResponse struct {
	TraceID     string    `json:"trace_id"`
	RootName    string    `json:"root_name"`
	ServiceName string    `json:"service_name"`
	StartTime   time.Time `json:"start_time"`
	// DurationNs is the root span's duration, or the extent of the spans while no root arrived.
	DurationNs int64 `json:"duration_ns"`
	HasRoot    bool  `json:"has_root"`
	// SpanCount is every stored span; Spans holds at most 5000 of them (truncated is then true).
	SpanCount  int64       `json:"span_count"`
	ErrorCount int64       `json:"error_count"`
	Status     TraceStatus `json:"status" enums:"ok,error"`
	Truncated  bool        `json:"truncated"`
	// LogCount is how many product log entries carry this trace id; search them with GET /v1/logs?trace_id=.
	LogCount int64       `json:"log_count"`
	Spans    []TraceSpan `json:"spans"`
}

// TraceFacetCount is one choice of the filter bar with how many traces have it.
type TraceFacetCount struct {
	Value string `json:"value"`
	Count int64  `json:"count"`
}

// TraceFacetsResponse lists the filter choices for the traces matching the query: at most 50 services and 50 root span names.
// Total counts the traces matching every filter, the status one included.
type TraceFacetsResponse struct {
	Total    int64             `json:"total"`
	Services []TraceFacetCount `json:"services"`
	Names    []TraceFacetCount `json:"names"`
	// Status has the counts of ok and error traces, whatever status filter is set.
	Status []TraceFacetCount `json:"status"`
}

// ServiceTraceSummaryResponse is the trace volume, error rate and root-span latency of the apps linked to a service.
type ServiceTraceSummaryResponse struct {
	ServiceID  string    `json:"service_id"`
	From       time.Time `json:"from"`
	To         time.Time `json:"to"`
	AppIDs     []string  `json:"app_ids"`
	TraceCount int64     `json:"trace_count"`
	ErrorCount int64     `json:"error_count"`
	// ErrorRate is error_count / trace_count, 0 to 1 (0 when there are no traces).
	ErrorRate float64 `json:"error_rate"`
	// RootedCount is the traces whose root span arrived; only they have a latency.
	RootedCount int64 `json:"rooted_count"`
	// P50Ns, P95Ns and P99Ns are root-span durations (nearest rank); null when no trace has its root.
	P50Ns *int64 `json:"p50_ns"`
	P95Ns *int64 `json:"p95_ns"`
	P99Ns *int64 `json:"p99_ns"`
	// Sampled is true when more than 50,000 traces matched and the percentiles use the newest 50,000.
	Sampled bool `json:"sampled"`
}
