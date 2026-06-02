package service

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAlertServiceQueuesConfiguredChannels(t *testing.T) {
	database := setupAlertServiceDatabase(t)
	createTestIncident(t, database, "incident-1")
	createTestAlertChannel(t, database, db.AlertChannel{Name: "ops-webhook", Type: "webhook", Enabled: true, WebhookURL: "https://alerts.example.com/hook"})
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

	service := NewAlertService(database, utils.NewLogger(), &config.Config{})
	service.httpClient.Transport = transport

	if err := service.QueueIncidentNotifications("incident-1", "incident_opened"); err != nil {
		t.Fatalf("QueueIncidentNotifications() error = %v", err)
	}

	var deliveries []db.AlertDelivery
	if err := database.Preload("Attempts").Order("channel ASC").Find(&deliveries).Error; err != nil {
		t.Fatalf("find deliveries: %v", err)
	}
	if len(deliveries) != 1 {
		t.Fatalf("delivery count = %d, want 1", len(deliveries))
	}
	if deliveries[0].Channel != "ops-webhook" || deliveries[0].Status != "sent" {
		t.Fatalf("enabled delivery = %+v, want sent ops-webhook", deliveries[0])
	}
	if deliveries[0].AttemptCount != 1 || len(deliveries[0].Attempts) != 1 || deliveries[0].Attempts[0].Status != "sent" {
		t.Fatalf("enabled delivery attempts = %+v, want one sent attempt", deliveries[0])
	}
	if webhookRequests != 1 {
		t.Fatalf("webhook requests = %d, want 1", webhookRequests)
	}
}

func TestAlertServiceIgnoresLegacyEmailChannels(t *testing.T) {
	database := setupAlertServiceDatabase(t)
	createTestIncident(t, database, "incident-1")
	createTestAlertChannel(t, database, db.AlertChannel{Name: "ops-email", Type: "email", Enabled: true})

	service := NewAlertService(database, utils.NewLogger(), &config.Config{})
	if err := service.QueueIncidentNotifications("incident-1", "incident_opened"); err != nil {
		t.Fatalf("QueueIncidentNotifications() error = %v", err)
	}

	var delivery db.AlertDelivery
	if err := database.First(&delivery).Error; err != nil {
		t.Fatalf("find delivery: %v", err)
	}
	if delivery.Channel != "none" || delivery.Type != "none" || delivery.Status != "suppressed" {
		t.Fatalf("delivery = %+v, want no alert channels configured suppression", delivery)
	}
}

func TestAlertServiceRecordsFailedAttemptAndRetriesDueDelivery(t *testing.T) {
	database := setupAlertServiceDatabase(t)
	createTestIncident(t, database, "incident-1")
	createTestAlertChannel(t, database, db.AlertChannel{Name: "ops-webhook", Type: "webhook", Enabled: true, WebhookURL: "https://alerts.example.com/hook"})

	var webhookRequests int
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		webhookRequests++
		if webhookRequests == 1 {
			return nil, fmt.Errorf("connection refused")
		}
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Body:       http.NoBody,
			Header:     make(http.Header),
		}, nil
	})

	service := NewAlertService(database, utils.NewLogger(), &config.Config{})
	service.httpClient.Transport = transport

	if err := service.QueueIncidentNotifications("incident-1", "incident_opened"); err != nil {
		t.Fatalf("QueueIncidentNotifications() error = %v", err)
	}

	var delivery db.AlertDelivery
	if err := database.Preload("Attempts").Where("channel = ?", "ops-webhook").First(&delivery).Error; err != nil {
		t.Fatalf("find failed delivery: %v", err)
	}
	if delivery.Status != "failed" || delivery.AttemptCount != 1 || delivery.NextAttemptAt == nil {
		t.Fatalf("delivery = %+v, want failed queued retry after one attempt", delivery)
	}
	if len(delivery.Attempts) != 1 || delivery.Attempts[0].Status != "failed" || delivery.Attempts[0].Stage != "http_request" {
		t.Fatalf("attempts = %+v, want one failed http_request attempt", delivery.Attempts)
	}

	past := time.Now().UTC().Add(-time.Minute)
	if err := database.Model(&db.AlertDelivery{}).Where("id = ?", delivery.ID).Update("next_attempt_at", past).Error; err != nil {
		t.Fatalf("force retry due: %v", err)
	}
	processed, err := service.ProcessDueDeliveries(10)
	if err != nil {
		t.Fatalf("ProcessDueDeliveries() error = %v", err)
	}
	if processed != 1 {
		t.Fatalf("processed = %d, want 1", processed)
	}

	var retried db.AlertDelivery
	if err := database.
		Preload("Attempts", func(tx *gorm.DB) *gorm.DB {
			return tx.Order("attempt_number ASC")
		}).
		Where("id = ?", delivery.ID).
		First(&retried).Error; err != nil {
		t.Fatalf("find retried delivery: %v", err)
	}
	if retried.Status != "sent" || retried.AttemptCount != 2 || retried.NextAttemptAt != nil {
		t.Fatalf("retried delivery = %+v, want sent with no next attempt", retried)
	}
	if len(retried.Attempts) != 2 || retried.Attempts[1].Status != "sent" {
		t.Fatalf("retry attempts = %+v, want failed then sent attempts", retried.Attempts)
	}
}

func TestAlertServiceSuppressesRetiredDueDeliveries(t *testing.T) {
	database := setupAlertServiceDatabase(t)
	createTestIncident(t, database, "incident-1")
	dueAt := time.Now().UTC().Add(-time.Minute)
	if err := database.Create(&db.AlertDelivery{
		ID:            "delivery-retired-email",
		IncidentID:    "incident-1",
		EventType:     db.AlertEventIncidentOpened,
		Channel:       "ops-email",
		Type:          "email",
		Status:        "failed",
		AttemptCount:  1,
		MaxAttempts:   3,
		NextAttemptAt: &dueAt,
	}).Error; err != nil {
		t.Fatalf("create retired delivery: %v", err)
	}

	service := NewAlertService(database, utils.NewLogger(), &config.Config{})
	processed, err := service.ProcessDueDeliveries(10)
	if err != nil {
		t.Fatalf("ProcessDueDeliveries() error = %v", err)
	}
	if processed != 1 {
		t.Fatalf("processed = %d, want 1", processed)
	}

	var delivery db.AlertDelivery
	if err := database.Where("id = ?", "delivery-retired-email").First(&delivery).Error; err != nil {
		t.Fatalf("find retired delivery: %v", err)
	}
	if delivery.Status != "suppressed" || delivery.Error != retiredAlertDeliveryMessage || delivery.AttemptCount != 1 {
		t.Fatalf("retired delivery = %+v, want suppressed without retry attempt", delivery)
	}
}

func TestAlertServiceWebhookUsesPayloadV1(t *testing.T) {
	database := setupAlertServiceDatabase(t)
	createTestIncident(t, database, "incident-1")
	if err := database.Create(&db.Agent{
		ID:        "agent-1",
		MachineId: "machine-1",
		Name:      "Edge Server",
		OS:        "linux",
		Arch:      "amd64",
		Token:     "token-1",
	}).Error; err != nil {
		t.Fatalf("create agent: %v", err)
	}
	if err := database.Create(&db.Monitor{
		ID:             "monitor-1",
		Type:           "http",
		Name:           "Homepage",
		AgentID:        "agent-1",
		Lifecycle:      "active",
		Health:         "down",
		ComputedHealth: "down",
	}).Error; err != nil {
		t.Fatalf("create monitor: %v", err)
	}
	createTestAlertChannel(t, database, db.AlertChannel{Name: "ops-webhook", Type: "webhook", Enabled: true, WebhookURL: "https://alerts.example.com/hook"})

	var payload AlertPayload
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if got := r.Header.Get("X-Orion-Payload-Version"); got != AlertPayloadVersion {
			t.Fatalf("payload version header = %q, want %q", got, AlertPayloadVersion)
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode webhook body: %v", err)
		}
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Body:       http.NoBody,
			Header:     make(http.Header),
		}, nil
	})

	service := NewAlertService(database, utils.NewLogger(), &config.Config{})
	service.httpClient.Transport = transport

	if err := service.QueueIncidentNotifications("incident-1", db.AlertEventIncidentOpened); err != nil {
		t.Fatalf("QueueIncidentNotifications() error = %v", err)
	}

	if payload.Version != AlertPayloadVersion || payload.EventType != db.AlertEventIncidentOpened {
		t.Fatalf("payload identity = %+v, want v1 incident_opened", payload)
	}
	if payload.Incident.ID != "incident-1" || payload.Incident.Title != "homepage is down" {
		t.Fatalf("payload incident = %+v, want incident context", payload.Incident)
	}
	if payload.Agent == nil || payload.Agent.Name != "Edge Server" {
		t.Fatalf("payload agent = %+v, want Edge Server", payload.Agent)
	}
	if payload.Monitor == nil || payload.Monitor.Name != "Homepage" || payload.Monitor.Type != "http" {
		t.Fatalf("payload monitor = %+v, want Homepage http", payload.Monitor)
	}
}

func TestAlertServiceWebhookAppliesConfiguredSignature(t *testing.T) {
	database := setupAlertServiceDatabase(t)
	createTestIncident(t, database, "incident-1")
	createTestAlertChannel(t, database, db.AlertChannel{
		Name:                 "ops-webhook",
		Type:                 "webhook",
		Enabled:              true,
		WebhookURL:           "https://alerts.example.com/hook",
		WebhookSigningSecret: "signing-secret",
	})

	var receivedBody []byte
	var signatureHeader string
	var timestampHeader string
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var err error
		receivedBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read webhook body: %v", err)
		}
		signatureHeader = r.Header.Get("X-Orion-Signature")
		timestampHeader = r.Header.Get("X-Orion-Timestamp")
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Body:       http.NoBody,
			Header:     make(http.Header),
		}, nil
	})

	service := NewAlertService(database, utils.NewLogger(), &config.Config{})
	service.httpClient.Transport = transport

	if err := service.QueueIncidentNotifications("incident-1", db.AlertEventIncidentOpened); err != nil {
		t.Fatalf("QueueIncidentNotifications() error = %v", err)
	}

	var payload AlertPayload
	if err := json.Unmarshal(receivedBody, &payload); err != nil {
		t.Fatalf("decode webhook body: %v", err)
	}
	want := SignAlertWebhookPayload("signing-secret", payload.DeliveredAt, receivedBody)
	if signatureHeader != want.Value {
		t.Fatalf("signature header = %q, want %q", signatureHeader, want.Value)
	}
	if timestampHeader != want.Timestamp {
		t.Fatalf("timestamp header = %q, want %q", timestampHeader, want.Timestamp)
	}
}

func TestSignAlertWebhookPayloadIsDeterministic(t *testing.T) {
	timestamp := time.Date(2026, 5, 27, 11, 30, 0, 0, time.UTC)
	first := SignAlertWebhookPayload("secret", timestamp, []byte(`{"event_type":"incident_opened"}`))
	second := SignAlertWebhookPayload("secret", timestamp, []byte(`{"event_type":"incident_opened"}`))
	changed := SignAlertWebhookPayload("secret", timestamp, []byte(`{"event_type":"incident_resolved"}`))

	if first.Header != "X-Orion-Signature" || first.Timestamp != "2026-05-27T11:30:00Z" {
		t.Fatalf("signature metadata = %+v, want Orion signature header and timestamp", first)
	}
	if first.Value != second.Value {
		t.Fatalf("signature is not deterministic: %q != %q", first.Value, second.Value)
	}
	if first.Value == changed.Value || !strings.HasPrefix(first.Value, "t=2026-05-27T11:30:00Z,v1=") {
		t.Fatalf("signature value = %q, want v1 digest that changes with body", first.Value)
	}
}

func TestAlertServiceCooldownSuppressesRecentDuplicate(t *testing.T) {
	database := setupAlertServiceDatabase(t)
	createTestIncident(t, database, "incident-1")
	createTestAlertChannel(t, database, db.AlertChannel{Name: "ops-webhook", Type: "webhook", Enabled: true, WebhookURL: "https://alerts.example.com/hook"})
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Body:       http.NoBody,
			Header:     make(http.Header),
		}, nil
	})

	service := NewAlertService(database, utils.NewLogger(), &config.Config{
		AlertCooldownSeconds: 300,
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

func createTestAlertChannel(t *testing.T, database *gorm.DB, channel db.AlertChannel) {
	t.Helper()

	channel.ID = "channel-" + channel.Name
	if err := database.Create(&channel).Error; err != nil {
		t.Fatalf("create alert channel: %v", err)
	}
}

func createTestAlertRoute(t *testing.T, database *gorm.DB, route db.AlertRoute) {
	t.Helper()

	if err := database.Create(&route).Error; err != nil {
		t.Fatalf("create alert route: %v", err)
	}
}

func encodeTestStringList(values []string) string {
	body, _ := json.Marshal(values)
	return string(body)
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
