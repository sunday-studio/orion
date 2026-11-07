package db

import "time"

// NodeAppResource is the latest measurement of one thing on a node's host (a process group,
// systemd unit, container, path, volume or Docker's own disk use). One row per node and key,
// replaced on every report; Core keeps no history of it. Fields the node could not measure are nil.
type NodeAppResource struct {
	NodeID             string `gorm:"primaryKey"`
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
	// ServiceID links the thing to a Service (nil = unlinked). The row is replaced on every
	// report, so the node service carries this value over for keys that are still reported.
	ServiceID *string `gorm:"index:idx_node_app_resources_service_id"`
}

// NodeAppResourceReport is the envelope of the latest app-resources report for one node.
type NodeAppResourceReport struct {
	NodeID          string    `gorm:"primaryKey"`
	IntervalSeconds float64   `gorm:"not null"`
	Notes           string    `gorm:"type:text;not null;default:'[]'"`
	ReportedAt      time.Time `gorm:"not null"`
}
