package service

import (
	"encoding/json"
	"net/http"
	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/logging"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
)

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

func TestAlertServiceDelaysGroupedSummaryForRoutePolicy(t *testing.T) {
	database := setupAlertServiceDatabase(t)
	createTestIncident(t, database, "incident-1")
	createTestIncident(t, database, "incident-2")
	createTestAlertChannel(t, database, db.AlertChannel{Name: "ops-webhook", Type: "webhook", Enabled: true, WebhookURL: "https://alerts.example.com/hook"})
	createTestAlertRoute(t, database, db.AlertRoute{
		ID:                   "route-delayed-summary",
		Name:                 "delayed summary",
		Enabled:              true,
		Priority:             10,
		EventTypes:           db.EncodeAlertEvents([]string{db.AlertEventIncidentOpened}),
		ChannelIDs:           encodeTestStringList([]string{"channel-ops-webhook"}),
		GroupingPolicy:       db.AlertGroupingPolicyDelayedSummary,
		GroupingDelaySeconds: 60,
	})

	payloads := make([]AlertPayload, 0, 2)
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var payload AlertPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode webhook payload: %v", err)
		}
		payloads = append(payloads, payload)
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
		t.Fatalf("delivery count = %d, want immediate delivery and pending summary", len(deliveries))
	}
	if deliveries[0].Status != "sent" || deliveries[0].EventType != db.AlertEventIncidentOpened || deliveries[0].AlertGroupID == "" {
		t.Fatalf("first delivery = %+v, want sent opened delivery with group", deliveries[0])
	}
	if deliveries[1].Status != "pending" || deliveries[1].EventType != alertEventGroupSummary || deliveries[1].NextAttemptAt == nil || deliveries[1].AlertGroupID != deliveries[0].AlertGroupID {
		t.Fatalf("summary delivery = %+v, want pending grouped summary in same group", deliveries[1])
	}
	if len(payloads) != 1 || payloads[0].EventType != db.AlertEventIncidentOpened {
		t.Fatalf("payloads after queue = %+v, want only first opened payload", payloads)
	}

	past := time.Now().UTC().Add(-time.Minute)
	if err := database.Model(&db.AlertDelivery{}).Where("id = ?", deliveries[1].ID).Update("next_attempt_at", past).Error; err != nil {
		t.Fatalf("force summary due: %v", err)
	}
	processed, err := service.ProcessDueDeliveries(10)
	if err != nil {
		t.Fatalf("ProcessDueDeliveries() error = %v", err)
	}
	if processed != 1 {
		t.Fatalf("processed = %d, want 1 due summary", processed)
	}
	if len(payloads) != 2 || payloads[1].EventType != alertEventGroupSummary || !strings.Contains(payloads[1].Summary.Title, "2 high") {
		t.Fatalf("summary payload = %+v, want grouped summary for two incidents", payloads)
	}

	var summaryDelivery db.AlertDelivery
	if err := database.Where("id = ?", deliveries[1].ID).First(&summaryDelivery).Error; err != nil {
		t.Fatalf("find summary delivery: %v", err)
	}
	if summaryDelivery.Status != "sent" || summaryDelivery.AttemptCount != 1 || summaryDelivery.AlertGroupID == "" {
		t.Fatalf("summary delivery after processing = %+v, want sent with alert_group_id", summaryDelivery)
	}
}

func TestAlertServiceRouteCanDisableGrouping(t *testing.T) {
	database := setupAlertServiceDatabase(t)
	createTestIncident(t, database, "incident-1")
	createTestIncident(t, database, "incident-2")
	createTestAlertChannel(t, database, db.AlertChannel{Name: "ops-webhook", Type: "webhook", Enabled: true, WebhookURL: "https://alerts.example.com/hook"})
	createTestAlertRoute(t, database, db.AlertRoute{
		ID:                   "route-no-grouping",
		Name:                 "no grouping",
		Enabled:              true,
		Priority:             10,
		EventTypes:           db.EncodeAlertEvents([]string{db.AlertEventIncidentOpened}),
		ChannelIDs:           encodeTestStringList([]string{"channel-ops-webhook"}),
		GroupingPolicy:       db.AlertGroupingPolicyNone,
		GroupingDelaySeconds: db.DefaultAlertGroupingDelaySeconds,
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
		t.Fatalf("first QueueIncidentNotifications() error = %v", err)
	}
	if err := service.QueueIncidentNotifications("incident-2", db.AlertEventIncidentOpened); err != nil {
		t.Fatalf("second QueueIncidentNotifications() error = %v", err)
	}

	var deliveries []db.AlertDelivery
	if err := database.Order("created_at ASC").Find(&deliveries).Error; err != nil {
		t.Fatalf("find deliveries: %v", err)
	}
	if len(deliveries) != 2 || deliveries[0].Status != "sent" || deliveries[1].Status != "sent" || deliveries[0].AlertGroupID != "" || deliveries[1].AlertGroupID != "" {
		t.Fatalf("deliveries = %+v, want two sent ungrouped deliveries", deliveries)
	}
	if webhookRequests != 2 {
		t.Fatalf("webhook requests = %d, want 2", webhookRequests)
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
	createTestAlertChannel(t, database, db.AlertChannel{
		Name:    "ops-email",
		Type:    "email",
		Enabled: true,
	})
	service := NewAlertService(database, logging.NewLogger(), &config.Config{})

	if _, err := service.TestChannel("channel-ops-email"); err != gorm.ErrRecordNotFound {
		t.Fatalf("TestChannel() error = %v, want legacy email channel hidden", err)
	}
}
