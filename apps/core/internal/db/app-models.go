package db

import "time"

// App is a product that sends logs to Orion (control-plane data; the entries live in logs.db).
type App struct {
	ID              string  `gorm:"primaryKey"`
	Name            string  `gorm:"not null"`
	Slug            string  `gorm:"not null;uniqueIndex:idx_apps_slug"`
	Environment     string  `gorm:"not null;default:''"`
	Description     string  `gorm:"not null;default:''"`
	RetentionDays   int     `gorm:"not null;default:14"`
	MaxStorageMB    int     `gorm:"not null;default:0"`
	DebugSampleRate float64 `gorm:"not null;default:1"`
	DisplayJSON     string  `gorm:"column:display_json;not null;default:'{}'"`
	RedactionJSON   string  `gorm:"column:redaction_json;not null;default:'{}'"`
	LastIngestAt    *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// AppIngestKey is a write-only credential for one App; only its SHA-256 is stored.
type AppIngestKey struct {
	ID         string `gorm:"primaryKey"`
	AppID      string `gorm:"not null;index:idx_app_ingest_keys_app_id"`
	Name       string `gorm:"not null"`
	Prefix     string `gorm:"not null"`
	KeyHash    string `gorm:"not null;uniqueIndex:idx_app_ingest_keys_hash"`
	LastUsedAt *time.Time
	RevokedAt  *time.Time
	CreatedAt  time.Time
}

// LogSavedView stores a named set of log filters so it can be shared by URL.
type LogSavedView struct {
	ID          string `gorm:"primaryKey"`
	Name        string `gorm:"not null"`
	FiltersJSON string `gorm:"column:filters_json;not null;default:'{}'"`
	CreatedBy   string `gorm:"not null;default:''"`
	CreatedAt   time.Time
}

// LogAlertRule raises an incident when an app's logs match a condition: more than
// Threshold matching entries inside the window ("threshold") or none at all ("absence").
type LogAlertRule struct {
	ID              string `gorm:"primaryKey"`
	Name            string `gorm:"not null"`
	AppID           string `gorm:"not null;index:idx_log_alert_rules_app_id"`
	Mode            string `gorm:"not null;default:threshold"`
	Severity        string `gorm:"not null;default:''"`
	Event           string `gorm:"not null;default:''"`
	Text            string `gorm:"not null;default:''"`
	Threshold       int    `gorm:"not null;default:0"`
	WindowSeconds   int    `gorm:"not null;default:300"`
	Enabled         bool   `gorm:"not null;default:true"`
	MonitorID       string `gorm:"not null;default:''"`
	State           string `gorm:"not null;default:ok"`
	LastCount       int    `gorm:"not null;default:0"`
	LastEvaluatedAt *time.Time
	LastReportedAt  *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
