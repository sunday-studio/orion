package db

import (
	"time"
)

// Agent represents a registered agent in the system
type Agent struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	UUID      string    `json:"uuid" gorm:"uniqueIndex;not null"`
	Name      string    `json:"name" gorm:"not null"`
	OS        string    `json:"os" gorm:"not null"`
	Arch      string    `json:"arch" gorm:"not null"`
	Token     string    `json:"token" gorm:"uniqueIndex;not null"`
	CreatedAt time.Time `json:"created_at"`
	LastSeen  time.Time `json:"last_seen"`
}

// Report represents a telemetry report from an agent
type Report struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	AgentID   uint      `json:"agent_id" gorm:"not null"`
	Agent     Agent     `json:"agent" gorm:"foreignKey:AgentID"`
	Payload   string    `json:"payload" gorm:"type:text;not null"`
	CreatedAt time.Time `json:"created_at"`
}
