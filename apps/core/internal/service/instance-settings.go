package service

import (
	"orion/core/internal/db"

	"gorm.io/gorm"
)

// SavedPublicURL is the Public Core URL saved in Settings ("" when none), read by the Worker so email links match the Console.
func SavedPublicURL(database *gorm.DB) string {
	var row db.InstanceSettings
	if database.Select("public_url").Where("id = 1").Limit(1).Find(&row).RowsAffected == 0 {
		return ""
	}
	return row.PublicURL
}
