package api

import "time"

// HealthIssue is a monitor that is down, degraded or stale right now.
type HealthIssue struct {
	MonitorID   string `json:"monitor_id"`
	MonitorName string `json:"monitor_name"`
	MonitorType string `json:"monitor_type"`
	// Health is the computed health that makes this an issue.
	Health string `json:"health" enums:"down,degraded,stale"`
	// IssueType is "stale_data" for stale monitors and omitted otherwise.
	IssueType string `json:"issue_type,omitempty" enums:"stale_data"`
	NodeID    string `json:"node_id"`
	NodeName  string `json:"node_name"`
	// Kind is monitor or server; TargetID/TargetName identify the row to open.
	Kind       string     `json:"kind" enums:"monitor,server"`
	TargetID   string     `json:"target_id"`
	TargetName string     `json:"target_name"`
	Detail     string     `json:"detail,omitempty" example:"timeout after 10s"`
	Since      *time.Time `json:"since,omitempty"`
	// IncidentID and IncidentDisplayID are set when an active incident already covers it.
	IncidentID        string `json:"incident_id,omitempty"`
	IncidentDisplayID string `json:"incident_display_id,omitempty"`
}

type HealthIssuesResponse struct {
	Issues []HealthIssue `json:"issues"`
	Count  int           `json:"count"`
}

// IncidentCandidate is a failing monitor that has no incident yet.
type IncidentCandidate struct {
	MonitorID   string `json:"monitor_id"`
	MonitorName string `json:"monitor_name"`
	MonitorType string `json:"monitor_type"`
	Health      string `json:"health" enums:"down,degraded,stale"`
	IssueType   string `json:"issue_type,omitempty" enums:"stale_data"`
	// Severity is derived from recent failures: high (3+ of the last 5 reports down), medium, low; stale monitors are high.
	Severity string `json:"severity" enums:"low,medium,high"`
	NodeID   string `json:"node_id"`
	NodeName string `json:"node_name"`
	// DownCount is how many of the last 5 reports were down.
	DownCount int `json:"down_count"`
	// Detail is a one-line reason such as "timeout after 10s"; SeenAt is when the signal started.
	Detail string     `json:"detail,omitempty"`
	SeenAt *time.Time `json:"seen_at,omitempty"`
}

type IncidentCandidatesResponse struct {
	Candidates []IncidentCandidate `json:"candidates"`
	Count      int                 `json:"count"`
}
