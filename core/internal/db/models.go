package db

import (
	"time"
)

// one agent to one machine/server
type Agent struct {
	ID        string    `json:"id" gorm:"primaryKey"`
	MachineId string    `json:"machine_id" gorm:"uniqueIndex;not null"` // this is from the agent; unique to the machine
	Name      string    `json:"name" gorm:"not null"`
	OS        string    `json:"os" gorm:"not null"`
	Arch      string    `json:"arch" gorm:"not null"`
	Token     string    `json:"token" gorm:"uniqueIndex;not null"`
	CreatedAt time.Time `json:"created_at"`
	LastSeen  time.Time `json:"last_seen"`
}

type AgentReport struct {
	ID        string    `json:"id" gorm:"primaryKey;type:varchar(255)"`
	AgentID   string    `json:"agent_id" gorm:"not null"`
	Payload   string    `json:"payload" gorm:"type:text;not null"`
	CreatedAt time.Time `json:"created_at"`
}

// subprocess are apps & services that are running on the agent's machine
type Monitor struct {
	ID          string    `json:"id" gorm:"primaryKey"`
	Description *string   `json:"description" gorm:"not null"`
	Type        string    `json:"type" gorm:"not null"`
	Name        string    `json:"name" gorm:"not null"`
	AgentID     string    `json:"agent_id" gorm:"not null"`
	Status      string    `json:"status" gorm:"not null"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type MonitorReport struct {
	ID        string    `json:"id" gorm:"primaryKey;type:varchar(255)"`
	MonitorID string    `json:"monitor_id" gorm:"not null"`
	Payload   string    `json:"payload" gorm:"type:text;not null"`
	CreatedAt time.Time `json:"created_at"`
}
