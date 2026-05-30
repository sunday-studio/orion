package service

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/logging"
)

func TestAlertServiceStopsRetryingAfterMaxAttempts(t *testing.T) {
	database := setupAlertServiceDatabase(t)
	createTestIncident(t, database, "incident-max-attempts")
	createTestAlertChannel(t, database, db.AlertChannel{Name: "ops-webhook", Type: "webhook", Enabled: true, WebhookURL: "https://alerts.example.com/hook"})

	service := NewAlertService(database, logging.NewLogger(), &config.Config{})
	service.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("connection refused")
	})

	if err := service.QueueIncidentNotifications("incident-max-attempts", db.AlertEventIncidentOpened); err != nil {
		t.Fatalf("QueueIncidentNotifications() error = %v", err)
	}

	for i := 0; i < defaultAlertDeliveryMaxAttempts-1; i++ {
		var delivery db.AlertDelivery
		if err := database.Where("incident_id = ?", "incident-max-attempts").First(&delivery).Error; err != nil {
			t.Fatalf("find delivery before retry %d: %v", i+1, err)
		}
		past := time.Now().UTC().Add(-time.Minute)
		if err := database.Model(&db.AlertDelivery{}).Where("id = ?", delivery.ID).Update("next_attempt_at", past).Error; err != nil {
			t.Fatalf("force retry %d due: %v", i+1, err)
		}
		processed, err := service.ProcessDueDeliveries(10)
		if err != nil {
			t.Fatalf("ProcessDueDeliveries() retry %d error = %v", i+1, err)
		}
		if processed != 1 {
			t.Fatalf("processed retry %d = %d, want 1", i+1, processed)
		}
	}

	var exhausted db.AlertDelivery
	if err := database.Where("incident_id = ?", "incident-max-attempts").First(&exhausted).Error; err != nil {
		t.Fatalf("find exhausted delivery: %v", err)
	}
	if exhausted.Status != "failed" || exhausted.AttemptCount != defaultAlertDeliveryMaxAttempts || exhausted.NextAttemptAt != nil {
		t.Fatalf("exhausted delivery = %+v, want failed without next retry", exhausted)
	}

	processed, err := service.ProcessDueDeliveries(10)
	if err != nil {
		t.Fatalf("ProcessDueDeliveries() after exhaustion error = %v", err)
	}
	if processed != 0 {
		t.Fatalf("processed after exhaustion = %d, want 0", processed)
	}

	var attempts []db.AlertDeliveryAttempt
	if err := database.Where("alert_delivery_id = ?", exhausted.ID).Order("attempt_number ASC").Find(&attempts).Error; err != nil {
		t.Fatalf("find attempts: %v", err)
	}
	if len(attempts) != defaultAlertDeliveryMaxAttempts {
		t.Fatalf("attempt count = %d, want %d", len(attempts), defaultAlertDeliveryMaxAttempts)
	}
	for i, attempt := range attempts {
		if attempt.AttemptNumber != i+1 || attempt.Status != "failed" || attempt.Stage != "http_request" || attempt.CompletedAt == nil {
			t.Fatalf("attempt[%d] = %+v, want failed completed http_request attempt", i, attempt)
		}
	}
}
