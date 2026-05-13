package service

import (
	"net/http"
	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/logging"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAlertServiceQueuesConfiguredChannels(t *testing.T) {
	database := setupAlertServiceDatabase(t)
	createTestIncident(t, database, "incident-1")
	var webhookRequests int
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		webhookRequests++
		if r.Method != http.MethodPost {
			t.Fatalf("webhook method = %s, want POST", r.Method)
		}
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Body:       http.NoBody,
			Header:     make(http.Header),
		}, nil
	})

	service := NewAlertService(database, logging.NewLogger(), &config.Config{
		AlertChannels: []config.AlertChannelConfig{
			{Name: "ops-webhook", Type: "webhook", Enabled: true, WebhookURL: "https://alerts.example.com/hook"},
			{Name: "ops-email", Type: "email", Enabled: false, EmailTo: "ops@example.com"},
		},
	})
	service.httpClient.Transport = transport

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
	if deliveries[1].Channel != "ops-webhook" || deliveries[1].Status != "sent" {
		t.Fatalf("enabled delivery = %+v, want sent ops-webhook", deliveries[1])
	}
	if webhookRequests != 1 {
		t.Fatalf("webhook requests = %d, want 1", webhookRequests)
	}
}

func TestAlertServiceCooldownSuppressesRecentDuplicate(t *testing.T) {
	database := setupAlertServiceDatabase(t)
	createTestIncident(t, database, "incident-1")
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Body:       http.NoBody,
			Header:     make(http.Header),
		}, nil
	})

	service := NewAlertService(database, logging.NewLogger(), &config.Config{
		AlertCooldownSeconds: 300,
		AlertChannels: []config.AlertChannelConfig{
			{Name: "ops-webhook", Type: "webhook", Enabled: true, WebhookURL: "https://alerts.example.com/hook"},
		},
	})
	service.httpClient.Transport = transport

	if err := service.QueueIncidentNotifications("incident-1", "incident_opened"); err != nil {
		t.Fatalf("first QueueIncidentNotifications() error = %v", err)
	}
	if err := service.QueueIncidentNotifications("incident-1", "incident_opened"); err != nil {
		t.Fatalf("second QueueIncidentNotifications() error = %v", err)
	}

	var statuses []string
	if err := database.Model(&db.AlertDelivery{}).Order("created_at ASC").Pluck("status", &statuses).Error; err != nil {
		t.Fatalf("pluck statuses: %v", err)
	}
	if len(statuses) != 2 || statuses[0] != "sent" || statuses[1] != "cooldown" {
		t.Fatalf("statuses = %#v, want [sent cooldown]", statuses)
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

func createTestIncident(t *testing.T, database *gorm.DB, incidentID string) {
	t.Helper()

	now := time.Now().UTC()
	incident := db.Incident{
		ID:                 incidentID,
		Status:             "open",
		Severity:           "high",
		Title:              "homepage is down",
		AgentID:            "agent-1",
		MonitorID:          "monitor-1",
		OpenedAt:           now,
		LastEventAt:        now,
		LatestEvent:        "Monitor homepage reported down",
		NotificationStatus: "pending",
	}
	if err := database.Create(&incident).Error; err != nil {
		t.Fatalf("create incident: %v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
