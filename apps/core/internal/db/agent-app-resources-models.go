package db

import "time"

// AgentAppResource is the latest measurement of one thing on an agent's host (a process group,
// systemd unit, container, path, volume or Docker's own disk use). One row per agent and key,
// replaced on every report; Core keeps no history of it. Fields the agent could not measure are nil.
type AgentAppResource struct {
	AgentID            string `gorm:"primaryKey"`
	Key                string `gorm:"primaryKey"`
	Kind               string `gorm:"not null"`
	Name               string `gorm:"not null"`
	CPUPercent         *float64
	MemoryBytes        *uint64
	IOReadBytesPerSec  *float64 `gorm:"column:io_read_bytes_per_sec"`
	IOWriteBytesPerSec *float64 `gorm:"column:io_write_bytes_per_sec"`
	NetRxBytesPerSec   *float64 `gorm:"column:net_rx_bytes_per_sec"`
	NetTxBytesPerSec   *float64 `gorm:"column:net_tx_bytes_per_sec"`
	Pids               *int
	StorageBytes       *uint64
	StorageCapped      bool      `gorm:"not null"`
	UpdatedAt          time.Time `gorm:"not null"`
}

// AgentAppResourceReport is the envelope of the latest app-resources report for one agent.
type AgentAppResourceReport struct {
	AgentID         string    `gorm:"primaryKey"`
	IntervalSeconds float64   `gorm:"not null"`
	Notes           string    `gorm:"type:text;not null;default:'[]'"`
	ReportedAt      time.Time `gorm:"not null"`
}
