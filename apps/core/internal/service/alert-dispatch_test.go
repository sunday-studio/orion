package service

import (
	"net/http"
	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/testkit"
	"orion/core/internal/utils"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestQueueIncidentNotificationsDoesNotSendInline(t *testing.T) {
	database := setupAlertServiceDatabase(t)
	createTestIncident(t, database, "incident-1")
	createTestAlertChannel(t, database, db.AlertChannel{Name: "ops-webhook", Type: "webhook", Enabled: true, WebhookURL: "https://alerts.example.com/hook"})
	var requests atomic.Int32
	svc := NewAlertService(database, utils.NewLogger(), &config.Config{})
	svc.httpClient.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		requests.Add(1)
		return &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody, Header: make(http.Header)}, nil
	})

	if err := svc.QueueIncidentNotifications("incident-1", "incident_opened"); err != nil {
		t.Fatalf("QueueIncidentNotifications() error = %v", err)
	}
	if requests.Load() != 0 {
		t.Fatalf("webhook requests during queue = %d, want 0", requests.Load())
	}
	var delivery db.AlertDelivery
	if err := database.First(&delivery).Error; err != nil {
		t.Fatalf("load delivery: %v", err)
	}
	if delivery.Status != "pending" || delivery.AttemptCount != 0 {
		t.Fatalf("queued delivery = %+v, want pending with no attempts", delivery)
	}

	if _, err := svc.ProcessDueDeliveries(10); err != nil {
		t.Fatalf("ProcessDueDeliveries() error = %v", err)
	}
	if requests.Load() != 1 {
		t.Fatalf("webhook requests after dispatch = %d, want 1", requests.Load())
	}
}

func TestProcessDueDeliveriesRetriesFailedWebhookUntilSent(t *testing.T) {
	database := setupAlertServiceDatabase(t)
	createTestIncident(t, database, "incident-1")
	createTestAlertChannel(t, database, db.AlertChannel{Name: "ops-webhook", Type: "webhook", Enabled: true, WebhookURL: "https://alerts.example.com/hook"})
	var requests atomic.Int32
	svc := NewAlertService(database, utils.NewLogger(), &config.Config{})
	svc.httpClient.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		status := http.StatusNoContent
		if requests.Add(1) == 1 {
			status = http.StatusBadGateway
		}
		return &http.Response{StatusCode: status, Body: http.NoBody, Header: make(http.Header)}, nil
	})

	if err := queueAndDispatch(svc, "incident-1", "incident_opened"); err != nil {
		t.Fatalf("queueAndDispatch() error = %v", err)
	}
	var delivery db.AlertDelivery
	database.First(&delivery)
	if delivery.Status != "failed" || delivery.NextAttemptAt == nil {
		t.Fatalf("after first attempt = %+v, want failed with retry scheduled", delivery)
	}

	// Not due yet: a dispatcher tick must not resend early.
	if n, _ := svc.ProcessDueDeliveries(10); n != 0 {
		t.Fatalf("processed before retry is due = %d, want 0", n)
	}
	past := time.Now().UTC().Add(-time.Minute)
	database.Model(&db.AlertDelivery{}).Where("id = ?", delivery.ID).Update("next_attempt_at", past)

	if n, err := svc.ProcessDueDeliveries(10); err != nil || n != 1 {
		t.Fatalf("retry processed = %d, err = %v, want 1", n, err)
	}
	database.First(&delivery, "id = ?", delivery.ID)
	if delivery.Status != "sent" || delivery.AttemptCount != 2 {
		t.Fatalf("after retry = %+v, want sent after two attempts", delivery)
	}
}

func TestConcurrentDispatchersNeverDoubleSend(t *testing.T) {
	database := testkit.NewDB(t)
	createTestIncident(t, database, "incident-1")
	createTestAlertChannel(t, database, db.AlertChannel{Name: "ops-webhook", Type: "webhook", Enabled: true, WebhookURL: "https://alerts.example.com/hook"})
	if err := NewAlertService(database, utils.NewLogger(), &config.Config{}).QueueIncidentNotifications("incident-1", "incident_opened"); err != nil {
		t.Fatalf("QueueIncidentNotifications() error = %v", err)
	}

	var requests atomic.Int32
	transport := roundTripFunc(func(*http.Request) (*http.Response, error) {
		requests.Add(1)
		time.Sleep(50 * time.Millisecond)
		return &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody, Header: make(http.Header)}, nil
	})

	var wg sync.WaitGroup
	for range 4 {
		svc := NewAlertService(database, utils.NewLogger(), &config.Config{})
		svc.httpClient.Transport = transport
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = svc.ProcessDueDeliveries(10)
		}()
	}
	wg.Wait()

	if requests.Load() != 1 {
		t.Fatalf("webhook requests = %d, want exactly 1 across 4 dispatchers", requests.Load())
	}
	var delivery db.AlertDelivery
	database.First(&delivery)
	if delivery.Status != "sent" || delivery.AttemptCount != 1 {
		t.Fatalf("delivery = %+v, want sent once", delivery)
	}
}

func TestExpiredDeliveryLeaseIsReclaimed(t *testing.T) {
	database := setupAlertServiceDatabase(t)
	createTestIncident(t, database, "incident-1")
	createTestAlertChannel(t, database, db.AlertChannel{Name: "ops-webhook", Type: "webhook", Enabled: true, WebhookURL: "https://alerts.example.com/hook"})
	svc := NewAlertService(database, utils.NewLogger(), &config.Config{})
	svc.httpClient.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody, Header: make(http.Header)}, nil
	})
	if err := svc.QueueIncidentNotifications("incident-1", "incident_opened"); err != nil {
		t.Fatal(err)
	}

	live := time.Now().UTC().Add(time.Minute)
	database.Model(&db.AlertDelivery{}).Where("1 = 1").Updates(map[string]any{"locked_by": "other", "locked_until": live})
	if n, _ := svc.ProcessDueDeliveries(10); n != 0 {
		t.Fatalf("processed while another dispatcher holds the lease = %d, want 0", n)
	}
	expired := time.Now().UTC().Add(-time.Minute)
	database.Model(&db.AlertDelivery{}).Where("1 = 1").Update("locked_until", expired)
	if n, _ := svc.ProcessDueDeliveries(10); n != 1 {
		t.Fatalf("processed after lease expiry = %d, want 1", n)
	}
}
