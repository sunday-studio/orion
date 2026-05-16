package api

import (
	"orion/core/internal/db"
	"orion/core/internal/service"
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

// MonitorResponse represents a monitor in API responses
type MonitorResponse struct {
	ID                       string     `json:"id"`
	Description              *string    `json:"description"`
	Type                     string     `json:"type"`
	Name                     string     `json:"name"`
	AgentID                  string     `json:"agent_id"`
	LastSuccessfulReportAt   *time.Time `json:"last_successful_report_at"`
	ReportingIntervalSeconds int        `json:"reporting_interval_seconds"`
	ComputedHealth           string     `json:"computed_health"`
	LastHealthComputation    *time.Time `json:"last_health_computation"`
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
	ID            string         `json:"id"`
	AgentID       string         `json:"agent_id"`
	CreatedAt     time.Time      `json:"created_at"`
	AgentVersion  string         `json:"agent_version"`
	ConfigSummary string         `json:"config_summary"`
	UptimeSeconds uint64         `json:"uptime_seconds"`
	Timestamp     string         `json:"timestamp"`
	CPU           db.CPUStats    `json:"cpu"`
	Memory        db.MemoryStats `json:"memory"`
	Disk          db.DiskStats   `json:"disk"`
	Location      db.GeoLocation `json:"location"`
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
	ID         string    `json:"id"`
	IncidentID string    `json:"incident_id"`
	EventType  string    `json:"event_type"`
	Channel    string    `json:"channel"`
	Type       string    `json:"type"`
	Status     string    `json:"status"`
	Error      string    `json:"error,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// AlertChannelResponse represents a redacted configured alert channel.
type AlertChannelResponse struct {
	Name                   string     `json:"name"`
	Type                   string     `json:"type"`
	Enabled                bool       `json:"enabled"`
	WebhookConfigured      bool       `json:"webhook_configured,omitempty"`
	EmailToConfigured      bool       `json:"email_to_configured,omitempty"`
	EmailFromConfigured    bool       `json:"email_from_configured,omitempty"`
	SMTPHostConfigured     bool       `json:"smtp_host_configured,omitempty"`
	SMTPPortConfigured     bool       `json:"smtp_port_configured,omitempty"`
	SMTPUsernameConfigured bool       `json:"smtp_username_configured,omitempty"`
	LastDeliveryStatus     string     `json:"last_delivery_status,omitempty"`
	LastDeliveryAt         *time.Time `json:"last_delivery_at,omitempty"`
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

// IncidentTimelineItemResponse represents a normalized incident timeline item.
type IncidentTimelineItemResponse struct {
	ID              string    `json:"id"`
	Type            string    `json:"type"`
	Source          string    `json:"source"`
	Message         string    `json:"message"`
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
		LastSuccessfulReportAt:   monitor.LastSuccessfulReportAt,
		ReportingIntervalSeconds: monitor.ReportingIntervalSeconds,
		ComputedHealth:           monitor.ComputedHealth,
		LastHealthComputation:    monitor.LastHealthComputation,
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

func monitorReportResponse(report db.MonitorReport) MonitorReportResponse {
	return MonitorReportResponse{
		ID:          report.ID,
		MonitorID:   report.MonitorID,
		Payload:     report.Payload,
		CollectedAt: report.CollectedAt,
		Health:      report.Health,
		CreatedAt:   report.CreatedAt,
	}
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
		ConfigSummary: report.ConfigSummary,
		UptimeSeconds: report.UptimeSeconds,
		Timestamp:     report.Timestamp,
		CPU:           report.CPU.Data(),
		Memory:        report.Memory.Data(),
		Disk:          report.Disk.Data(),
		Location:      report.Location.Data(),
	}
}

func agentReportResponses(reports []db.AgentReport) []AgentReportResponse {
	responses := make([]AgentReportResponse, 0, len(reports))
	for _, report := range reports {
		responses = append(responses, agentReportResponse(report))
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
		ID:         delivery.ID,
		IncidentID: delivery.IncidentID,
		EventType:  delivery.EventType,
		Channel:    delivery.Channel,
		Type:       delivery.Type,
		Status:     delivery.Status,
		Error:      safeAlertDeliveryError(delivery.Error),
		CreatedAt:  delivery.CreatedAt,
		UpdatedAt:  delivery.UpdatedAt,
	}
}

func alertDeliveryResponses(deliveries []db.AlertDelivery) []AlertDeliveryResponse {
	responses := make([]AlertDeliveryResponse, 0, len(deliveries))
	for _, delivery := range deliveries {
		responses = append(responses, alertDeliveryResponse(delivery))
	}
	return responses
}

func safeAlertDeliveryError(value string) string {
	switch value {
	case "", "alert channel disabled", "alert cooldown active", "no alert channels configured":
		return value
	default:
		return "delivery failed; check Core logs"
	}
}
