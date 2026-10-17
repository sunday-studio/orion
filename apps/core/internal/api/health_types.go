package api

// HealthIssue is a monitor that is down, degraded or stale right now.
type HealthIssue struct {
	MonitorID   string `json:"monitor_id"`
	MonitorName string `json:"monitor_name"`
	MonitorType string `json:"monitor_type"`
	// Health is the computed health that makes this an issue.
	Health string `json:"health" enums:"down,degraded,stale"`
	// IssueType is "stale_data" for stale monitors and omitted otherwise.
	IssueType string `json:"issue_type,omitempty" enums:"stale_data"`
	AgentID   string `json:"agent_id"`
	AgentName string `json:"agent_name"`
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
	Severity  string `json:"severity" enums:"low,medium,high"`
	AgentID   string `json:"agent_id"`
	AgentName string `json:"agent_name"`
	// DownCount is how many of the last 5 reports were down.
	DownCount int `json:"down_count"`
}

type IncidentCandidatesResponse struct {
	Candidates []IncidentCandidate `json:"candidates"`
	Count      int                 `json:"count"`
}
