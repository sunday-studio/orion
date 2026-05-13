package service

import (
	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/logging"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAlertServiceQueuesConfiguredChannels(t *testing.T) {
	database := setupAlertServiceDatabase(t)
	service := NewAlertService(database, logging.NewLogger(), &config.Config{
		AlertChannels: []config.AlertChannelConfig{
			{Name: "ops-webhook", Type: "webhook", Enabled: true, WebhookURL: "https://example.com/hook"},
			{Name: "ops-email", Type: "email", Enabled: false, EmailTo: "ops@example.com"},
		},
	})

	if err := service.QueueIncidentNotifications("incident-1", "incident_opened"); err != nil {
		t.Fatalf("QueueIncidentNotifications() error = %v", err)
	}

	var deliveries []db.AlertDelivery
	if err := database.Order("channel ASC").Find(&deliveries).Error; err != nil {
		t.Fatalf("find deliveries: %v", err)
	}
	if len(deliveries) != 2 {
		t.Fatalf("delivery count = %d, want 2", len(deliveries))
	}
	if deliveries[0].Channel != "ops-email" || deliveries[0].Status != "suppressed" {
		t.Fatalf("disabled delivery = %+v, want suppressed ops-email", deliveries[0])
	}
	if deliveries[1].Channel != "ops-webhook" || deliveries[1].Status != "pending" {
		t.Fatalf("enabled delivery = %+v, want pending ops-webhook", deliveries[1])
	}
}

func setupAlertServiceDatabase(t *testing.T) *gorm.DB {
	t.Helper()

	database, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.Migrate(database); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	return database
}
