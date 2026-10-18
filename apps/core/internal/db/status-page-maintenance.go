package db

import "time"

// StatusPageMaintenance is a scheduled maintenance window shown on a status page.
type StatusPageMaintenance struct {
	ID           string `json:"id" gorm:"primaryKey;type:varchar(255)"`
	StatusPageID string `json:"status_page_id" gorm:"not null;index:idx_status_page_maintenance_page"`
	Title        string `json:"title" gorm:"not null"`
	Description  string `json:"description" gorm:"type:text;not null;default:''"`
	// ComponentIDs is a JSON array of status page component ids.
	ComponentIDs string `json:"component_ids" gorm:"type:text;not null;default:'[]'"`
	// HoldAlerts suppresses incidents and alerts for resources mapped to the components during the window.
	HoldAlerts        bool       `json:"hold_alerts" gorm:"not null;default:false"`
	NotifySubscribers bool       `json:"notify_subscribers" gorm:"not null;default:false"`
	NoticeSentAt      *time.Time `json:"notice_sent_at"`
	StartsAt          time.Time  `json:"starts_at" gorm:"not null;index:idx_status_page_maintenance_window"`
	EndsAt            time.Time  `json:"ends_at" gorm:"not null;index:idx_status_page_maintenance_window"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

func (StatusPageMaintenance) TableName() string { return "status_page_maintenance" }
