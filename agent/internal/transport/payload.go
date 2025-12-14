package transport

import (
	"time"

	"orion/agent/internal/collector"
	"orion/agent/internal/services"
)

type SystemReport struct {
	KernelVersion string                 `json:"kernel_version"`
	UptimeSeconds uint64                 `json:"uptime_seconds"`
	Timestamp     string                 `json:"timestamp"`
	CPU           *collector.CPUStats    `json:"cpu"`
	Memory        *collector.MemoryStats `json:"memory"`
	Disk          *collector.DiskStats   `json:"disk"`
	Location      *services.GeoLocation  `json:"location,omitempty"`
}

type AgentRegistrationRequest struct {
	MachineId string `json:"machine_id" binding:"required"`
	Name      string `json:"name" binding:"required"`
	OS        string `json:"os" binding:"required"`
	Arch      string `json:"arch" binding:"required"`
}

type AgentRegistrationResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		AgentID string `json:"agent_id"`
		Token   string `json:"token"`
	} `json:"data"`
}

type MonitorRegistrationRequest struct {
	AgentID     string    `json:"agent_id" binding:"required"`
	Name        string    `json:"name" binding:"required"`
	Description string    `json:"description" binding:"required"`
	Type        string    `json:"type" binding:"required"`
	LastChecked time.Time `json:"last_checked" binding:"required"`
}

type UnRegisterMonitorRequest struct {
	AgentID   string `json:"agent_id" binding:"required"`
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
