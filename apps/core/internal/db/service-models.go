package db

import "time"

// Service is one named thing (for example "checkout, production") that ties monitors, an app's
// logs and measured resources together. Apps, monitors and node app resources point at it through
// a nullable service_id; a Service owns none of them, so deleting it only clears those links.
type Service struct {
	ID          string `gorm:"primaryKey"`
	Name        string `gorm:"not null"`
	Slug        string `gorm:"not null;uniqueIndex:idx_services_slug"`
	Environment string `gorm:"not null;default:''"`
	// NodeID is the optional owner node (the server the service mainly runs on); empty when unset.
	NodeID     string `gorm:"not null;default:''"`
	RunbookURL string `gorm:"not null;default:''"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
