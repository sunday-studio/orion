package transport

import (
	"time"

	"orion/node/internal/appres"
	"orion/node/internal/collector"
	"orion/node/internal/services"
)

type SystemReport struct {
	// ReportID is an idempotency key: replays of the same report reuse it so
	// Core can store it once.
	ReportID      string                 `json:"report_id,omitempty"`
	KernelVersion string                 `json:"kernel_version"`
	NodeVersion   string                 `json:"node_version,omitempty"`
	ConfigSummary map[string]interface{} `json:"config_summary,omitempty"`
	UptimeSeconds uint64                 `json:"uptime_seconds"`
	Timestamp     string                 `json:"timestamp"`
	CPU           *collector.CPUStats    `json:"cpu"`
	Memory        *collector.MemoryStats `json:"memory"`
	Disk          *collector.DiskStats   `json:"disk"`
	Location      *services.GeoLocation  `json:"location,omitempty"`
	// AppResources is passive per-process, per-unit and per-container usage.
	AppResources *appres.Report `json:"app_resources,omitempty"`
}

type MonitorReport struct {
	// ReportID is generated once per collected report and kept when the report is
	// spooled or retried, so Core can discard replays.
	ReportID  string                  `json:"report_id,omitempty"`
	Timestamp string                  `json:"timestamp" binding:"required"`
	Health    string                  `json:"health" binding:"required"` // up | down
	Metrics   interface{}             `json:"metrics,omitempty"`
	Error     *collector.MonitorError `json:"error,omitempty"`
}

type ServiceLogBatch struct {
	Entries []ServiceLogEntry `json:"entries"`
}

type ServiceLogEntry struct {
	Timestamp   string         `json:"timestamp"`
	Source      string         `json:"source,omitempty"`
	Stream      string         `json:"stream,omitempty"`
	Level       string         `json:"level,omitempty"`
	Component   string         `json:"component,omitempty"`
	MonitorName string         `json:"monitor_name,omitempty"`
	MonitorID   string         `json:"monitor_id,omitempty"`
	Message     string         `json:"message,omitempty"`
	Fields      map[string]any `json:"fields,omitempty"`
	Fingerprint string         `json:"fingerprint"`
}

type NodeRegistrationRequest struct {
	MachineId                string `json:"machine_id" binding:"required"`
	Name                     string `json:"name" binding:"required"`
	OS                       string `json:"os" binding:"required"`
	Arch                     string `json:"arch" binding:"required"`
	ReportingIntervalSeconds int    `json:"reporting_interval_seconds,omitempty"`
	Meta                     string `json:"meta,omitempty"`
}

type NodeRegistrationResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		NodeID string `json:"node_id"`
		Token  string `json:"token"`
	} `json:"data"`
}

type MonitorRegistrationRequest struct {
	NodeID                   string    `json:"node_id" binding:"required"`
	Name                     string    `json:"name" binding:"required"`
	Description              string    `json:"description" binding:"required"`
	Type                     string    `json:"type" binding:"required"`
	LastChecked              time.Time `json:"last_checked" binding:"required"`
	ReportingIntervalSeconds int       `json:"reporting_interval_seconds,omitempty"`
	Meta                     string    `json:"meta,omitempty"`
}

type UnRegisterMonitorRequest struct {
	NodeID    string `json:"node_id" binding:"required"`
	MonitorID string `json:"monitor_id" binding:"required"`
}

type MonitorRegistrationResponse struct {
	Success bool `json:"success"`
	Data    struct {
		MonitorID string `json:"monitor_id"`
	} `json:"data"`
	Message string `json:"message"`
}

type UnRegisterMonitorResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}
