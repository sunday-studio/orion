package db

import (
	"time"
)

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

type Report struct {
	ID        string    `json:"id" gorm:"primaryKey;type:varchar(255)"`
	AgentID   string    `json:"agent_id" gorm:"not null"`
	Payload   string    `json:"payload" gorm:"type:text;not null"`
	CreatedAt time.Time `json:"created_at"`
}
