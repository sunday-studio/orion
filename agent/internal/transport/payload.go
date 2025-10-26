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

type RegistrationRequest struct {
	MachineId string `json:"machine_id"`
	Name      string `json:"name"`
	OS        string `json:"os"`
	Arch      string `json:"arch"`
}

type RegistrationResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		AgentID string `json:"agent_id"`
		Token   string `json:"token"`
	} `json:"data"`
}
