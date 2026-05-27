package service

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/logging"
	"strconv"
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
	createTestAlertChannel(t, database, db.AlertChannel{Name: "ops-email", Type: "email", Enabled: false, EmailTo: "ops@example.com"})
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

	service := NewAlertService(database, logging.NewLogger(), &config.Config{})
	service.httpClient.Transport = transport

	if err := service.QueueIncidentNotifications("incident-1", "incident_opened"); err != nil {
		t.Fatalf("QueueIncidentNotifications() error = %v", err)
	}

	var deliveries []db.AlertDelivery
	if err := database.Preload("Attempts").Order("channel ASC").Find(&deliveries).Error; err != nil {
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
	if deliveries[1].AttemptCount != 1 || len(deliveries[1].Attempts) != 1 || deliveries[1].Attempts[0].Status != "sent" {
		t.Fatalf("enabled delivery attempts = %+v, want one sent attempt", deliveries[1])
	}
	if webhookRequests != 1 {
		t.Fatalf("webhook requests = %d, want 1", webhookRequests)
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

	service := NewAlertService(database, logging.NewLogger(), &config.Config{})
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

	service := NewAlertService(database, logging.NewLogger(), &config.Config{})
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

	service := NewAlertService(database, logging.NewLogger(), &config.Config{
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

func TestAlertServiceSkipsUnsubscribedEvents(t *testing.T) {
	database := setupAlertServiceDatabase(t)
	createTestIncident(t, database, "incident-1")
	createTestAlertChannel(t, database, db.AlertChannel{
		Name:             "opened-only",
		Type:             "webhook",
		Enabled:          true,
		WebhookURL:       "https://alerts.example.com/hook",
		SubscribedEvents: db.EncodeAlertEvents([]string{db.AlertEventIncidentOpened}),
	})
	var webhookRequests int
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		webhookRequests++
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Body:       http.NoBody,
			Header:     make(http.Header),
		}, nil
	})
	service := NewAlertService(database, logging.NewLogger(), &config.Config{})
	service.httpClient.Transport = transport

	if err := service.QueueIncidentNotifications("incident-1", db.AlertEventIncidentResolved); err != nil {
		t.Fatalf("QueueIncidentNotifications() error = %v", err)
	}

	var count int64
	if err := database.Model(&db.AlertDelivery{}).Count(&count).Error; err != nil {
		t.Fatalf("count deliveries: %v", err)
	}
	if count != 0 || webhookRequests != 0 {
		t.Fatalf("deliveries = %d requests = %d, want no delivery for unsubscribed event", count, webhookRequests)
	}
}

func TestAlertServiceUsesMatchingAlertRoute(t *testing.T) {
	database := setupAlertServiceDatabase(t)
	createTestIncident(t, database, "incident-1")
	createTestAlertChannel(t, database, db.AlertChannel{Name: "ops-webhook", Type: "webhook", Enabled: true, WebhookURL: "https://alerts.example.com/hook"})
	createTestAlertRoute(t, database, db.AlertRoute{
		ID:         "route-critical",
		Name:       "critical route",
		Enabled:    true,
		Priority:   10,
		EventTypes: db.EncodeAlertEvents([]string{db.AlertEventIncidentOpened}),
		Severities: encodeTestStringList([]string{"high"}),
		ChannelIDs: encodeTestStringList([]string{"channel-ops-webhook"}),
	})
	var webhookRequests int
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		webhookRequests++
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Body:       http.NoBody,
			Header:     make(http.Header),
		}, nil
	})
	service := NewAlertService(database, logging.NewLogger(), &config.Config{})
	service.httpClient.Transport = transport

	if err := service.QueueIncidentNotifications("incident-1", db.AlertEventIncidentOpened); err != nil {
		t.Fatalf("QueueIncidentNotifications() error = %v", err)
	}

	var delivery db.AlertDelivery
	if err := database.First(&delivery).Error; err != nil {
		t.Fatalf("find delivery: %v", err)
	}
	if delivery.RouteID != "route-critical" || delivery.Channel != "ops-webhook" || delivery.Status != "sent" {
		t.Fatalf("delivery = %+v, want sent delivery through route-critical", delivery)
	}
	if webhookRequests != 1 {
		t.Fatalf("webhook requests = %d, want 1", webhookRequests)
	}
}

func TestAlertServiceDryRunExplainsRouteSuppression(t *testing.T) {
	database := setupAlertServiceDatabase(t)
	createTestIncident(t, database, "incident-1")
	createTestAlertChannel(t, database, db.AlertChannel{Name: "ops-webhook", Type: "webhook", Enabled: true, WebhookURL: "https://alerts.example.com/hook"})
	createTestAlertRoute(t, database, db.AlertRoute{
		ID:         "route-suppress",
		Name:       "suppress high",
		Enabled:    true,
		Priority:   1,
		EventTypes: db.EncodeAlertEvents([]string{db.AlertEventIncidentOpened}),
		Severities: encodeTestStringList([]string{"high"}),
		Suppress:   true,
	})
	createTestAlertRoute(t, database, db.AlertRoute{
		ID:         "route-send",
		Name:       "send high",
		Enabled:    true,
		Priority:   10,
		EventTypes: db.EncodeAlertEvents([]string{db.AlertEventIncidentOpened}),
		Severities: encodeTestStringList([]string{"high"}),
		ChannelIDs: encodeTestStringList([]string{"channel-ops-webhook"}),
	})
	service := NewAlertService(database, logging.NewLogger(), &config.Config{})

	event, err := service.LoadAlertRouteContext("incident-1", db.AlertEventIncidentOpened)
	if err != nil {
		t.Fatalf("LoadAlertRouteContext() error = %v", err)
	}
	result, err := service.DryRunRoutes(*event)
	if err != nil {
		t.Fatalf("DryRunRoutes() error = %v", err)
	}

	if !result.Suppressed || result.SuppressionReason != "alert route suppressed event: suppress high" {
		t.Fatalf("dry-run suppression = %v %q, want suppress high", result.Suppressed, result.SuppressionReason)
	}
	if len(result.RouteEvaluations) != 2 || !result.RouteEvaluations[0].Suppressed || !result.RouteEvaluations[1].Matched {
		t.Fatalf("route evaluations = %+v, want suppressing route and matched send route", result.RouteEvaluations)
	}
	if len(result.DestinationDecisions) != 1 || result.DestinationDecisions[0].Status != "suppressed" {
		t.Fatalf("destination decisions = %+v, want suppressed destination", result.DestinationDecisions)
	}
}

func TestAlertServiceGroupsSiblingIncidentNotifications(t *testing.T) {
	database := setupAlertServiceDatabase(t)
	createTestIncident(t, database, "incident-1")
	createTestIncident(t, database, "incident-2")
	createTestAlertChannel(t, database, db.AlertChannel{Name: "ops-webhook", Type: "webhook", Enabled: true, WebhookURL: "https://alerts.example.com/hook"})
	var webhookRequests int
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		webhookRequests++
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Body:       http.NoBody,
			Header:     make(http.Header),
		}, nil
	})
	service := NewAlertService(database, logging.NewLogger(), &config.Config{})
	service.httpClient.Transport = transport

	if err := service.QueueIncidentNotifications("incident-1", db.AlertEventIncidentOpened); err != nil {
		t.Fatalf("first QueueIncidentNotifications() error = %v", err)
	}
	if err := service.QueueIncidentNotifications("incident-2", db.AlertEventIncidentOpened); err != nil {
		t.Fatalf("second QueueIncidentNotifications() error = %v", err)
	}

	var deliveries []db.AlertDelivery
	if err := database.Order("created_at ASC").Find(&deliveries).Error; err != nil {
		t.Fatalf("find deliveries: %v", err)
	}
	if len(deliveries) != 2 {
		t.Fatalf("delivery count = %d, want 2", len(deliveries))
	}
	if deliveries[0].Status != "sent" || deliveries[0].AlertGroupID == "" {
		t.Fatalf("first delivery = %+v, want sent with alert group", deliveries[0])
	}
	if deliveries[1].Status != "suppressed" || deliveries[1].Error != "alert grouped into active alert group" || deliveries[1].AlertGroupID != deliveries[0].AlertGroupID {
		t.Fatalf("second delivery = %+v, want grouped suppression in same group", deliveries[1])
	}
	if webhookRequests != 1 {
		t.Fatalf("webhook requests = %d, want 1", webhookRequests)
	}

	var group db.AlertGroup
	if err := database.Where("id = ?", deliveries[0].AlertGroupID).First(&group).Error; err != nil {
		t.Fatalf("find alert group: %v", err)
	}
	if group.Status != "open" || group.IncidentCount != 2 || !strings.Contains(group.Summary, "2 high") {
		t.Fatalf("alert group = %+v, want open summary for two high incidents", group)
	}
}

func TestAlertServiceSuppressesRecoveryUntilGroupedSiblingsResolve(t *testing.T) {
	database := setupAlertServiceDatabase(t)
	createTestIncident(t, database, "incident-1")
	createTestIncident(t, database, "incident-2")
	createTestAlertChannel(t, database, db.AlertChannel{Name: "ops-webhook", Type: "webhook", Enabled: true, WebhookURL: "https://alerts.example.com/hook"})
	var webhookRequests int
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		webhookRequests++
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Body:       http.NoBody,
			Header:     make(http.Header),
		}, nil
	})
	service := NewAlertService(database, logging.NewLogger(), &config.Config{})
	service.httpClient.Transport = transport

	if err := service.QueueIncidentNotifications("incident-1", db.AlertEventIncidentOpened); err != nil {
		t.Fatalf("first opened QueueIncidentNotifications() error = %v", err)
	}
	if err := service.QueueIncidentNotifications("incident-2", db.AlertEventIncidentOpened); err != nil {
		t.Fatalf("second opened QueueIncidentNotifications() error = %v", err)
	}
	if err := database.Model(&db.Incident{}).Where("id = ?", "incident-1").Update("status", "resolved").Error; err != nil {
		t.Fatalf("resolve incident-1: %v", err)
	}
	if err := service.QueueIncidentNotifications("incident-1", db.AlertEventIncidentResolved); err != nil {
		t.Fatalf("first resolved QueueIncidentNotifications() error = %v", err)
	}
	if err := database.Model(&db.Incident{}).Where("id = ?", "incident-2").Update("status", "resolved").Error; err != nil {
		t.Fatalf("resolve incident-2: %v", err)
	}
	if err := service.QueueIncidentNotifications("incident-2", db.AlertEventIncidentResolved); err != nil {
		t.Fatalf("second resolved QueueIncidentNotifications() error = %v", err)
	}

	var statuses []string
	if err := database.Model(&db.AlertDelivery{}).Order("created_at ASC").Pluck("status", &statuses).Error; err != nil {
		t.Fatalf("pluck delivery statuses: %v", err)
	}
	wantStatuses := []string{"sent", "suppressed", "suppressed", "sent"}
	if strings.Join(statuses, ",") != strings.Join(wantStatuses, ",") {
		t.Fatalf("statuses = %#v, want %#v", statuses, wantStatuses)
	}
	if webhookRequests != 2 {
		t.Fatalf("webhook requests = %d, want opened summary and final recovery", webhookRequests)
	}

	var group db.AlertGroup
	if err := database.First(&group).Error; err != nil {
		t.Fatalf("find alert group: %v", err)
	}
	if group.Status != "resolved" || group.ResolvedAt == nil {
		t.Fatalf("alert group = %+v, want resolved group", group)
	}
}

func TestAlertServiceTestsConfiguredWebhookChannel(t *testing.T) {
	database := setupAlertServiceDatabase(t)
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
	service := NewAlertService(database, logging.NewLogger(), &config.Config{})
	service.httpClient.Transport = transport

	delivery, err := service.TestChannel("channel-ops-webhook")
	if err != nil {
		t.Fatalf("TestChannel() error = %v", err)
	}

	if delivery.EventType != "test" || delivery.Channel != "ops-webhook" || delivery.Type != "webhook" || delivery.Status != "sent" {
		t.Fatalf("test delivery = %+v, want sent webhook test delivery", delivery)
	}
	if webhookRequests != 1 {
		t.Fatalf("webhook requests = %d, want 1", webhookRequests)
	}
}

func TestAlertServiceTestsConfiguredEmailChannel(t *testing.T) {
	database := setupAlertServiceDatabase(t)
	smtpAddress, messages := startTestSMTPServer(t)
	host, portValue, err := net.SplitHostPort(smtpAddress)
	if err != nil {
		t.Fatalf("split smtp address: %v", err)
	}
	port, err := strconv.Atoi(portValue)
	if err != nil {
		t.Fatalf("parse smtp port: %v", err)
	}
	createTestAlertChannel(t, database, db.AlertChannel{
		Name:      "ops-email",
		Type:      "email",
		Enabled:   true,
		EmailTo:   "ops@example.com",
		EmailFrom: "orion@example.com",
		SMTPHost:  host,
		SMTPPort:  port,
	})
	service := NewAlertService(database, logging.NewLogger(), &config.Config{})

	delivery, err := service.TestChannel("channel-ops-email")
	if err != nil {
		t.Fatalf("TestChannel() error = %v", err)
	}

	if delivery.EventType != "test" || delivery.Channel != "ops-email" || delivery.Type != "email" || delivery.Status != "sent" {
		t.Fatalf("test delivery = %+v, want sent email test delivery", delivery)
	}
	select {
	case message := <-messages:
		if !strings.Contains(message, "Subject: Orion alert: Alert channel test") || !strings.Contains(message, "Event: test") {
			t.Fatalf("email message = %q, want alert channel test content", message)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for test email")
	}
}

func TestAlertServiceQueuesReusableEmailDestinations(t *testing.T) {
	database := setupAlertServiceDatabase(t)
	createTestIncident(t, database, "incident-1")
	smtpAddress, messages := startTestSMTPServer(t)
	host, portValue, err := net.SplitHostPort(smtpAddress)
	if err != nil {
		t.Fatalf("split smtp address: %v", err)
	}
	port, err := strconv.Atoi(portValue)
	if err != nil {
		t.Fatalf("parse smtp port: %v", err)
	}
	if err := database.Create(&db.AlertSMTPService{
		ID:        "smtp-primary",
		Name:      "Primary SMTP",
		Enabled:   true,
		Host:      host,
		Port:      port,
		FromEmail: "orion@example.com",
	}).Error; err != nil {
		t.Fatalf("create smtp service: %v", err)
	}
	if err := database.Create(&db.AlertEmailDestination{
		ID:               "destination-ops",
		SMTPServiceID:    "smtp-primary",
		Name:             "ops-email",
		Enabled:          true,
		EmailTo:          "ops@example.com",
		SubscribedEvents: db.EncodeAlertEvents([]string{db.AlertEventIncidentOpened}),
	}).Error; err != nil {
		t.Fatalf("create email destination: %v", err)
	}

	service := NewAlertService(database, logging.NewLogger(), &config.Config{})
	if err := service.QueueIncidentNotifications("incident-1", db.AlertEventIncidentOpened); err != nil {
		t.Fatalf("QueueIncidentNotifications() error = %v", err)
	}

	var delivery db.AlertDelivery
	if err := database.Where("channel = ?", "ops-email").First(&delivery).Error; err != nil {
		t.Fatalf("find destination delivery: %v", err)
	}
	if delivery.Type != "email" || delivery.Status != "sent" {
		t.Fatalf("delivery = %+v, want sent email delivery", delivery)
	}
	select {
	case message := <-messages:
		if !strings.Contains(message, "To: ops@example.com") || !strings.Contains(message, "From: orion@example.com") {
			t.Fatalf("email message = %q, want destination and smtp sender", message)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for reusable destination email")
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

func startTestSMTPServer(t *testing.T) (string, <-chan string) {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen smtp: %v", err)
	}
	t.Cleanup(func() {
		_ = listener.Close()
	})

	messages := make(chan string, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		reader := bufio.NewReader(conn)
		_, _ = fmt.Fprint(conn, "220 orion-test-smtp\r\n")
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			command := strings.TrimRight(line, "\r\n")
			upperCommand := strings.ToUpper(command)
			switch {
			case strings.HasPrefix(upperCommand, "EHLO"), strings.HasPrefix(upperCommand, "HELO"):
				_, _ = fmt.Fprint(conn, "250 orion-test-smtp\r\n")
			case strings.HasPrefix(upperCommand, "MAIL FROM:"), strings.HasPrefix(upperCommand, "RCPT TO:"):
				_, _ = fmt.Fprint(conn, "250 ok\r\n")
			case upperCommand == "DATA":
				_, _ = fmt.Fprint(conn, "354 send message\r\n")
				var message strings.Builder
				for {
					dataLine, err := reader.ReadString('\n')
					if err != nil {
						return
					}
					if strings.TrimRight(dataLine, "\r\n") == "." {
						break
					}
					message.WriteString(dataLine)
				}
				messages <- message.String()
				_, _ = fmt.Fprint(conn, "250 queued\r\n")
			case upperCommand == "QUIT":
				_, _ = fmt.Fprint(conn, "221 bye\r\n")
				return
			default:
				_, _ = fmt.Fprint(conn, "250 ok\r\n")
			}
		}
	}()

	return listener.Addr().String(), messages
}
