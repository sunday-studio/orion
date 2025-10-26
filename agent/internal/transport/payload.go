package transport

import "time"

type SystemReport struct {
	Hostname   string            `json:"hostname"`
	IPAddress  string            `json:"ip_address"`
	OS         string            `json:"os"`
	Uptime     string            `json:"uptime"`
	Load       string            `json:"load"`
	CPUUsage   float64           `json:"cpu_usage"`
	MemoryUsed uint64            `json:"memory_used"`
	MemoryFree uint64            `json:"memory_free"`
	DiskUsed   uint64            `json:"disk_used"`
	DiskFree   uint64            `json:"disk_free"`
	Timestamp  time.Time         `json:"timestamp"`
	Tags       map[string]string `json:"tags,omitempty"`
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

type ApplicationRegistrationRequest struct {
	AgentID     string    `json:"agent_id" binding:"required"`
	Name        string    `json:"name" binding:"required"`
	Description string    `json:"description" binding:"required"`
	Type        string    `json:"type" binding:"required"`
	LastChecked time.Time `json:"last_checked" binding:"required"`
}

type ApplicationRegistrationResponse struct {
	Success bool `json:"success"`
	Data    struct {
		ApplicationID string `json:"application_id"`
	} `json:"data"`
	Message string `json:"message"`
}
