package api

import (
	"orion/core/internal/db"
	"time"
)

// AgentResponse represents an agent in API responses (without generics for OpenAPI compatibility)
type AgentResponse struct {
	ID                       string         `json:"id"`
	MachineId                string         `json:"machine_id"`
	Name                     string         `json:"name"`
	OS                       string         `json:"os"`
	Platform                 string         `json:"platform"`
	KernelVersion            string         `json:"kernel_version"`
	Arch                     string         `json:"arch"`
	Token                    string         `json:"token"`
	MaintenanceMode          bool           `json:"maintenance_mode"`
	ReportingIntervalSeconds int            `json:"reporting_interval_seconds"`
	CreatedAt                time.Time      `json:"created_at"`
	DeletedAt                time.Time      `json:"deleted_at"`
	LastSeen                 time.Time      `json:"last_seen"`
	Location                 db.GeoLocation `json:"location"`
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
