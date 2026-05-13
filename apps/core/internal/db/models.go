package db

import (
	"time"

	"gorm.io/datatypes"
)

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

// one agent to one machine/server
type Agent struct {
	ID                       string                          `json:"id" gorm:"primaryKey"`
	MachineId                string                          `json:"machine_id" gorm:"uniqueIndex;not null"`
	Name                     string                          `json:"name" gorm:"not null"`
	OS                       string                          `json:"os" gorm:"not null"`
	Platform                 string                          `json:"platform"`
	KernelVersion            string                          `json:"kernel_version"`
	Arch                     string                          `json:"arch" gorm:"not null"`
	Token                    string                          `json:"token" gorm:"uniqueIndex;not null"`
	MaintenanceMode          bool                            `json:"maintenance_mode" gorm:"default:false"`
	ReportingIntervalSeconds int                             `json:"reporting_interval_seconds" gorm:"default:60"` // System metrics reporting interval
	CreatedAt                time.Time                       `json:"created_at"`
	DeletedAt                time.Time                       `json:"deleted_at"`
	LastSeen                 time.Time                       `json:"last_seen"`
	Location                 datatypes.JSONType[GeoLocation] `json:"location" gorm:"type:json"`
	Meta                     string                          `json:"meta" gorm:"type:text"`
}

type AgentReport struct {
	ID            string                          `json:"id" gorm:"primaryKey;type:varchar(255)"`
	AgentID       string                          `json:"agent_id" gorm:"index:idx_agent_reports_agent_id;not null"`
	CreatedAt     time.Time                       `json:"created_at" gorm:"index:idx_agent_reports_created_at"`
	AgentVersion  string                          `json:"agent_version"`
	ConfigSummary string                          `json:"config_summary" gorm:"type:text"`
	UptimeSeconds uint64                          `json:"uptime_seconds"`
	Timestamp     string                          `json:"timestamp"`
	CPU           datatypes.JSONType[CPUStats]    `json:"cpu" gorm:"type:json"`
	Memory        datatypes.JSONType[MemoryStats] `json:"memory" gorm:"type:json"`
	Disk          datatypes.JSONType[DiskStats]   `json:"disk" gorm:"type:json"`
	Location      datatypes.JSONType[GeoLocation] `json:"location" gorm:"type:json"`
}

type Monitor struct {
	ID                       string     `json:"id" gorm:"primaryKey"`
	Description              *string    `json:"description"`
	Type                     string     `json:"type" gorm:"not null"`
	Name                     string     `json:"name" gorm:"not null"`
	AgentID                  string     `json:"agent_id" gorm:"index:idx_monitors_agent_id;not null"`
	LastSuccessfulReportAt   *time.Time `json:"last_successful_report_at"`
	ReportingIntervalSeconds int        `json:"reporting_interval_seconds" gorm:"default:60"` // Monitor check interval
	ComputedHealth           string     `json:"computed_health" gorm:"default:unknown"`       // Cached computed health (up | down | degraded | unknown)
	LastHealthComputation    *time.Time `json:"last_health_computation"`                      // When health was last computed

	Lifecycle string `json:"lifecycle" gorm:"not null;index:idx_monitors_lifecycle"` // active | disabled | deleted
	Health    string `json:"health" gorm:"not null;index:idx_monitors_health"`       // up | down | degraded | unknown
	Meta      string `json:"meta" gorm:"type:text"`

	CreatedAt time.Time `json:"created_at" gorm:"index:idx_monitors_created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	DeletedAt time.Time `json:"deleted_at"`
}

type MonitorReport struct {
	ID          string    `json:"id" gorm:"primaryKey;type:varchar(255)"`
	MonitorID   string    `json:"monitor_id" gorm:"index:idx_monitor_reports_monitor_id;not null"`
	Payload     string    `json:"payload" gorm:"type:text;not null"`
	CollectedAt string    `json:"collected_at" gorm:"not null"`
	Health      string    `json:"health" gorm:"not null"` // up | down
	CreatedAt   time.Time `json:"created_at" gorm:"index:idx_monitor_reports_created_at"`
}

type Incident struct {
	ID                 string     `json:"id" gorm:"primaryKey;type:varchar(255)"`
	Status             string     `json:"status" gorm:"not null;index:idx_incidents_status"` // open | acknowledged | resolved
	Severity           string     `json:"severity" gorm:"not null;index:idx_incidents_severity"`
	Title              string     `json:"title" gorm:"not null"`
	AgentID            string     `json:"agent_id" gorm:"index:idx_incidents_agent_id;not null"`
	MonitorID          string     `json:"monitor_id" gorm:"index:idx_incidents_monitor_id;not null"`
	OpenedAt           time.Time  `json:"opened_at" gorm:"not null;index:idx_incidents_opened_at"`
	ResolvedAt         *time.Time `json:"resolved_at"`
	LastEventAt        time.Time  `json:"last_event_at" gorm:"not null"`
	LatestEvent        string     `json:"latest_event" gorm:"type:text"`
	NotificationStatus string     `json:"notification_status" gorm:"not null;default:pending"` // pending | sent | failed | suppressed | cooldown
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type IncidentEvent struct {
	ID              string    `json:"id" gorm:"primaryKey;type:varchar(255)"`
	IncidentID      string    `json:"incident_id" gorm:"index:idx_incident_events_incident_id;not null"`
	Type            string    `json:"type" gorm:"not null"`
	Message         string    `json:"message" gorm:"type:text"`
	MonitorReportID string    `json:"monitor_report_id" gorm:"index:idx_incident_events_monitor_report_id"`
	CreatedAt       time.Time `json:"created_at" gorm:"index:idx_incident_events_created_at"`
}

type AlertDelivery struct {
	ID         string    `json:"id" gorm:"primaryKey;type:varchar(255)"`
	IncidentID string    `json:"incident_id" gorm:"index:idx_alert_deliveries_incident_id;not null"`
	EventType  string    `json:"event_type" gorm:"not null"` // incident_opened | incident_resolved
	Channel    string    `json:"channel" gorm:"not null"`
	Type       string    `json:"type" gorm:"not null"`   // webhook | email | none
	Status     string    `json:"status" gorm:"not null"` // pending | sent | failed | suppressed | cooldown
	Error      string    `json:"error" gorm:"type:text"`
	CreatedAt  time.Time `json:"created_at" gorm:"index:idx_alert_deliveries_created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
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
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}
