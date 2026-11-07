package db

import "time"

type CoreMonitorConfig struct {
	MonitorID                 string     `json:"monitor_id" gorm:"primaryKey"`
	Kind                      string     `json:"kind" gorm:"not null;index:idx_core_monitor_configs_kind"`
	ConfigJSON                string     `json:"config_json" gorm:"type:text;not null;default:'{}'"`
	SecretRefJSON             string     `json:"secret_ref_json" gorm:"type:text;not null;default:'{}'"`
	HeartbeatTokenHash        string     `json:"heartbeat_token_hash" gorm:"not null;default:'';index:idx_core_monitor_configs_heartbeat_token_hash"`
	IntervalSeconds           int        `json:"interval_seconds" gorm:"not null;default:60"`
	TimeoutSeconds            int        `json:"timeout_seconds" gorm:"not null;default:10"`
	ConfirmationPeriodSeconds int        `json:"confirmation_period_seconds" gorm:"not null;default:0"`
	ConfirmationCheckCount    int        `json:"confirmation_check_count" gorm:"not null;default:0"`
	RecoveryPeriodSeconds     int        `json:"recovery_period_seconds" gorm:"not null;default:0"`
	Paused                    bool       `json:"paused" gorm:"not null;default:false;index:idx_core_monitor_configs_due"`
	PausedAt                  *time.Time `json:"paused_at"`
	PausedBy                  string     `json:"paused_by" gorm:"not null;default:''"`
	// HeartbeatTokenPrefix is the first characters of a heartbeat token, so the setup screen can show a masked URL.
	HeartbeatTokenPrefix string     `json:"-" gorm:"not null;default:''"`
	NextRunAt            time.Time  `json:"next_run_at" gorm:"not null;index:idx_core_monitor_configs_due"`
	LastRunAt            *time.Time `json:"last_run_at"`
	LastSignalAt         *time.Time `json:"last_signal_at"`
	LastSuccessAt        *time.Time `json:"last_success_at"`
	LastFailureAt        *time.Time `json:"last_failure_at"`
	LeaseOwner           string     `json:"lease_owner" gorm:"not null;default:'';index:idx_core_monitor_configs_lease_owner"`
	LeaseExpiresAt       *time.Time `json:"lease_expires_at" gorm:"index:idx_core_monitor_configs_due;index:idx_core_monitor_configs_lease_expires_at"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
	Monitor              Monitor    `json:"monitor,omitempty" gorm:"foreignKey:MonitorID;references:ID"`
}

type CoreWorkerStatus struct {
	WorkerID        string    `json:"worker_id" gorm:"primaryKey;type:varchar(255)"`
	ProcessKind     string    `json:"process_kind" gorm:"not null;default:core-monitor-worker"`
	Hostname        string    `json:"hostname" gorm:"not null;default:''"`
	Status          string    `json:"status" gorm:"not null;default:unknown"`
	Version         string    `json:"version" gorm:"not null;default:''"`
	StartedAt       time.Time `json:"started_at" gorm:"not null"`
	LastHeartbeatAt time.Time `json:"last_heartbeat_at" gorm:"not null;index:idx_core_worker_statuses_last_heartbeat_at"`
	LastError       string    `json:"last_error" gorm:"not null;default:'';type:text"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}
