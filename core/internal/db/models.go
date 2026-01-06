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
	ID            string                          `json:"id" gorm:"primaryKey"`
	MachineId     string                          `json:"machine_id" gorm:"uniqueIndex;not null"`
	Name          string                          `json:"name" gorm:"not null"`
	OS            string                          `json:"os" gorm:"not null"`
	Platform      string                          `json:"platform"`
	KernelVersion string                          `json:"kernel_version"`
	Arch          string                          `json:"arch" gorm:"not null"`
	Token         string                          `json:"token" gorm:"uniqueIndex;not null"`
	CreatedAt     time.Time                       `json:"created_at"`
	DeletedAt     time.Time                       `json:"deleted_at"`
	LastSeen      time.Time                       `json:"last_seen"`
	Location      datatypes.JSONType[GeoLocation] `json:"location" gorm:"type:json"`
}

type AgentReport struct {
	ID            string                          `json:"id" gorm:"primaryKey;type:varchar(255)"`
	AgentID       string                          `json:"agent_id" gorm:"index;not null"`
	CreatedAt     time.Time                       `json:"created_at"`
	UptimeSeconds uint64                          `json:"uptime_seconds"`
	Timestamp     string                          `json:"timestamp"`
	CPU           datatypes.JSONType[CPUStats]    `json:"cpu" gorm:"type:json"`
	Memory        datatypes.JSONType[MemoryStats] `json:"memory" gorm:"type:json"`
	Disk          datatypes.JSONType[DiskStats]   `json:"disk" gorm:"type:json"`
	Location      datatypes.JSONType[GeoLocation] `json:"location" gorm:"type:json"`
}

type Monitor struct {
	ID          string  `json:"id" gorm:"primaryKey"`
	Description *string `json:"description"`
	Type        string  `json:"type" gorm:"not null"`
	Name        string  `json:"name" gorm:"not null"`
	AgentID     string  `json:"agent_id" gorm:"index;not null"`

	Lifecycle string `json:"lifecycle" gorm:"not null"` // active | disabled | deleted
	Health    string `json:"health" gorm:"not null"`    // up | down | degraded | unknown

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	DeletedAt time.Time `json:"deleted_at"`
}

type MonitorReport struct {
	ID          string    `json:"id" gorm:"primaryKey;type:varchar(255)"`
	MonitorID   string    `json:"monitor_id" gorm:"index;not null"`
	Payload     string    `json:"payload" gorm:"type:text;not null"`
	CollectedAt string    `json:"collected_at" gorm:"not null"`
	Health      string    `json:"health" gorm:"not null"` // up | down
	CreatedAt   time.Time `json:"created_at"`
}
