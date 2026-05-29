package api

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/logging"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAlertReadEndpointsShowWebhookURLAndRedactSecrets(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	database, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.Migrate(database); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	server := NewServer(database, logging.NewLogger(), &config.Config{
		AlertRecoveryNotifications: true,
		AlertTLSExpiryDays:         14,
		AlertCooldownSeconds:       300,
	})
	if err := server.db.Create(&db.AlertChannel{
		ID:                   "alert-channel-webhook",
		Name:                 "ops-webhook",
		Type:                 "webhook",
		Enabled:              true,
		WebhookURL:           "https://secret.example.com/hook",
		WebhookSigningSecret: "webhook-signing-secret",
	}).Error; err != nil {
		t.Fatalf("create webhook channel: %v", err)
	}
	if err := server.db.Create(&db.AlertChannel{
		ID:           "alert-channel-email",
		Name:         "ops-email",
		Type:         "email",
		Enabled:      false,
		EmailTo:      "ops@example.com",
		EmailFrom:    "orion@example.com",
		SMTPHost:     "smtp.example.com",
		SMTPPort:     587,
		SMTPUsername: "mailer",
		SMTPPassword: "secret-password",
	}).Error; err != nil {
		t.Fatalf("create email channel: %v", err)
	}

	delivery := db.AlertDelivery{
		ID:           "alert-delivery-test",
		IncidentID:   "incident-test",
		AlertGroupID: "alert-group-test",
		EventType:    "incident_opened",
		Channel:      "ops-webhook",
		Type:         "webhook",
		Status:       "failed",
		Error:        "post https://secret.example.com/hook: connection refused",
		AttemptCount: 1,
		MaxAttempts:  3,
	}
	if err := server.db.Create(&delivery).Error; err != nil {
		t.Fatalf("create alert delivery: %v", err)
	}
	attemptTime := time.Now().UTC()
	if err := server.db.Create(&db.AlertDeliveryAttempt{
		ID:              "alert-delivery-attempt-test",
		AlertDeliveryID: delivery.ID,
		AttemptNumber:   1,
		Status:          "failed",
		Stage:           "http_request",
		Error:           "post https://secret.example.com/hook: connection refused",
		StartedAt:       attemptTime,
		CompletedAt:     &attemptTime,
	}).Error; err != nil {
		t.Fatalf("create alert delivery attempt: %v", err)
	}
	secondDelivery := db.AlertDelivery{
		ID:         "alert-delivery-sent",
		IncidentID: "incident-other",
		EventType:  "incident_resolved",
		Channel:    "ops-email",
		Type:       "email",
		Status:     "sent",
	}
	if err := server.db.Create(&secondDelivery).Error; err != nil {
		t.Fatalf("create second alert delivery: %v", err)
	}

	channelsResp := performJSONRequest(t, server, http.MethodGet, "/v1/alerts/channels", nil, "")
	if channelsResp.Code != http.StatusOK {
		t.Fatalf("channels status = %d, body = %s", channelsResp.Code, channelsResp.Body.String())
	}
	assertNotContains(t, channelsResp.Body.String(), "secret-password")
	assertNotContains(t, channelsResp.Body.String(), "webhook-signing-secret")

	var channels struct {
		Success bool `json:"success"`
		Data    struct {
			Channels []struct {
				Name                       string `json:"name"`
				Type                       string `json:"type"`
				WebhookURL                 string `json:"webhook_url"`
				WebhookConfigured          bool   `json:"webhook_configured"`
				WebhookSignatureConfigured bool   `json:"webhook_signature_configured"`
				LastDeliveryStatus         string `json:"last_delivery_status"`
			} `json:"channels"`
			Count int `json:"count"`
		} `json:"data"`
	}
	decodeResponse(t, channelsResp, &channels)
	if !channels.Success || channels.Data.Count != 2 || len(channels.Data.Channels) != 2 {
		t.Fatalf("channels response = %+v, want two channels", channels)
	}
	var webhookChannel struct {
		Name                       string `json:"name"`
		Type                       string `json:"type"`
		WebhookURL                 string `json:"webhook_url"`
		WebhookConfigured          bool   `json:"webhook_configured"`
		WebhookSignatureConfigured bool   `json:"webhook_signature_configured"`
		LastDeliveryStatus         string `json:"last_delivery_status"`
	}
	for _, channel := range channels.Data.Channels {
		if channel.Name == "ops-webhook" {
			webhookChannel = channel
			break
		}
	}
	if webhookChannel.WebhookURL != "https://secret.example.com/hook" || !webhookChannel.WebhookConfigured || !webhookChannel.WebhookSignatureConfigured || webhookChannel.LastDeliveryStatus != "failed" {
		t.Fatalf("webhook channel response = %+v, want webhook URL with last failed status", webhookChannel)
	}

	deliveriesResp := performJSONRequest(t, server, http.MethodGet, "/v1/alerts/deliveries?limit=10", nil, "")
	if deliveriesResp.Code != http.StatusOK {
		t.Fatalf("deliveries status = %d, body = %s", deliveriesResp.Code, deliveriesResp.Body.String())
	}
	assertNotContains(t, deliveriesResp.Body.String(), "secret.example.com")
	if !strings.Contains(deliveriesResp.Body.String(), "delivery failed; check Core logs") {
		t.Fatalf("delivery error was not sanitized: %s", deliveriesResp.Body.String())
	}

	filteredDeliveriesResp := performJSONRequest(t, server, http.MethodGet, "/v1/alerts/deliveries?status=failed&incident_id=incident-test", nil, "")
	if filteredDeliveriesResp.Code != http.StatusOK {
		t.Fatalf("filtered deliveries status = %d, body = %s", filteredDeliveriesResp.Code, filteredDeliveriesResp.Body.String())
	}
	var filteredDeliveries struct {
		Success bool `json:"success"`
		Data    struct {
			Deliveries []struct {
				IncidentID   string `json:"incident_id"`
				AlertGroupID string `json:"alert_group_id"`
				Status       string `json:"status"`
				Error        string `json:"error"`
				Attempts     []struct {
					Status string `json:"status"`
					Stage  string `json:"stage"`
					Error  string `json:"error"`
				} `json:"attempts"`
			} `json:"deliveries"`
			Count int64 `json:"count"`
		} `json:"data"`
	}
	decodeResponse(t, filteredDeliveriesResp, &filteredDeliveries)
	if !filteredDeliveries.Success || filteredDeliveries.Data.Count != 1 || len(filteredDeliveries.Data.Deliveries) != 1 {
		t.Fatalf("filtered deliveries response = %+v, want one delivery", filteredDeliveries)
	}
	filteredDelivery := filteredDeliveries.Data.Deliveries[0]
	if filteredDelivery.IncidentID != "incident-test" || filteredDelivery.AlertGroupID != "alert-group-test" || filteredDelivery.Status != "failed" || filteredDelivery.Error != "delivery failed; check Core logs" {
		t.Fatalf("filtered delivery = %+v, want sanitized failed incident-test delivery with alert_group_id", filteredDelivery)
	}
	if len(filteredDelivery.Attempts) != 1 || filteredDelivery.Attempts[0].Status != "failed" || filteredDelivery.Attempts[0].Stage != "http_request" || filteredDelivery.Attempts[0].Error != "delivery failed; check Core logs" {
		t.Fatalf("filtered delivery attempts = %+v, want sanitized http_request failure", filteredDelivery.Attempts)
	}

	destinationFilteredResp := performJSONRequest(t, server, http.MethodGet, "/v1/alerts/deliveries?type=email&channel=ops-email&event_type=incident_resolved", nil, "")
	if destinationFilteredResp.Code != http.StatusOK {
		t.Fatalf("destination filtered deliveries status = %d, body = %s", destinationFilteredResp.Code, destinationFilteredResp.Body.String())
	}
	var destinationFiltered struct {
		Success bool `json:"success"`
		Data    struct {
			Deliveries []struct {
				Channel   string `json:"channel"`
				Type      string `json:"type"`
				EventType string `json:"event_type"`
				Status    string `json:"status"`
			} `json:"deliveries"`
			Count int64 `json:"count"`
		} `json:"data"`
	}
	decodeResponse(t, destinationFilteredResp, &destinationFiltered)
	if !destinationFiltered.Success || destinationFiltered.Data.Count != 1 || len(destinationFiltered.Data.Deliveries) != 1 {
		t.Fatalf("destination filtered deliveries response = %+v, want one email delivery", destinationFiltered)
	}
	destinationDelivery := destinationFiltered.Data.Deliveries[0]
	if destinationDelivery.Channel != "ops-email" || destinationDelivery.Type != "email" || destinationDelivery.EventType != "incident_resolved" || destinationDelivery.Status != "sent" {
		t.Fatalf("destination filtered delivery = %+v, want sent ops-email incident_resolved delivery", destinationDelivery)
	}

	rulesResp := performJSONRequest(t, server, http.MethodGet, "/v1/alerts/rules", nil, "")
	if rulesResp.Code != http.StatusOK {
		t.Fatalf("rules status = %d, body = %s", rulesResp.Code, rulesResp.Body.String())
	}
	assertNotContains(t, rulesResp.Body.String(), "secret.example.com")
	assertNotContains(t, rulesResp.Body.String(), "secret-password")
	assertNotContains(t, rulesResp.Body.String(), "webhook-signing-secret")
}

func TestAlertChannelTestEndpointSendsConfiguredWebhook(t *testing.T) {
	server := setupTestServer(t)
	webhookPayloads := make(chan map[string]interface{}, 1)
	webhookServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("webhook method = %s, want POST", r.Method)
		}
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode webhook payload: %v", err)
		}
		webhookPayloads <- payload
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(webhookServer.Close)

	createResp := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/channels", gin.H{
		"name":        "ops-webhook",
		"type":        "webhook",
		"enabled":     false,
		"webhook_url": webhookServer.URL,
	}, "")
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create channel status = %d, body = %s", createResp.Code, createResp.Body.String())
	}
	var created struct {
		Data struct {
			Channel struct {
				ID string `json:"id"`
			} `json:"channel"`
		} `json:"data"`
	}
	decodeResponse(t, createResp, &created)

	testResp := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/channels/"+created.Data.Channel.ID+"/test", nil, "")
	if testResp.Code != http.StatusOK {
		t.Fatalf("test channel status = %d, body = %s", testResp.Code, testResp.Body.String())
	}
	var tested struct {
		Success bool `json:"success"`
		Data    struct {
			Delivery struct {
				IncidentID string `json:"incident_id"`
				EventType  string `json:"event_type"`
				Channel    string `json:"channel"`
				Type       string `json:"type"`
				Status     string `json:"status"`
				Error      string `json:"error"`
			} `json:"delivery"`
		} `json:"data"`
	}
	decodeResponse(t, testResp, &tested)
	if !tested.Success || tested.Data.Delivery.IncidentID != "alert-channel-test" || tested.Data.Delivery.EventType != "test" || tested.Data.Delivery.Channel != "ops-webhook" || tested.Data.Delivery.Type != "webhook" || tested.Data.Delivery.Status != "sent" || tested.Data.Delivery.Error != "" {
		t.Fatalf("test delivery response = %+v, want sent webhook test delivery", tested)
	}

	select {
	case payload := <-webhookPayloads:
		if payload["event_type"] != "test" {
			t.Fatalf("webhook payload = %+v, want test event", payload)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for webhook test payload")
	}

	var stored db.AlertDelivery
	if err := server.db.Where("incident_id = ? AND event_type = ? AND channel = ?", "alert-channel-test", "test", "ops-webhook").First(&stored).Error; err != nil {
		t.Fatalf("find stored test delivery: %v", err)
	}
	if stored.Status != "sent" {
		t.Fatalf("stored test delivery status = %q, want sent", stored.Status)
	}
}

func TestAlertChannelWriteEndpointsPersistWebhookConfiguration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	database, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.Migrate(database); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	server := NewServer(database, logging.NewLogger(), &config.Config{})
	createResp := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/channels", gin.H{
		"name":                   "ops-webhook",
		"type":                   "webhook",
		"enabled":                true,
		"webhook_url":            "https://secret.example.com/hook",
		"webhook_signing_secret": "initial-signing-secret",
		"subscribed_events":      []string{db.AlertEventIncidentOpened},
	}, "")
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create channel status = %d, body = %s", createResp.Code, createResp.Body.String())
	}
	var created struct {
		Data struct {
			Channel struct {
				ID                         string   `json:"id"`
				Name                       string   `json:"name"`
				WebhookURL                 string   `json:"webhook_url"`
				WebhookConfigured          bool     `json:"webhook_configured"`
				WebhookSignatureConfigured bool     `json:"webhook_signature_configured"`
				SubscribedEvents           []string `json:"subscribed_events"`
			} `json:"channel"`
		} `json:"data"`
	}
	decodeResponse(t, createResp, &created)
	assertNotContains(t, createResp.Body.String(), "initial-signing-secret")
	if created.Data.Channel.ID == "" || created.Data.Channel.Name != "ops-webhook" || created.Data.Channel.WebhookURL != "https://secret.example.com/hook" || !created.Data.Channel.WebhookConfigured || !created.Data.Channel.WebhookSignatureConfigured {
		t.Fatalf("created channel = %+v, want webhook channel", created.Data.Channel)
	}
	if got := created.Data.Channel.SubscribedEvents; len(got) != 1 || got[0] != db.AlertEventIncidentOpened {
		t.Fatalf("created subscribed_events = %#v, want incident_opened", got)
	}

	updateResp := performJSONRequest(t, server, http.MethodPatch, "/v1/alerts/channels/"+created.Data.Channel.ID, gin.H{
		"name":                   "critical-webhook",
		"enabled":                false,
		"webhook_url":            "https://alerts.example.com/critical",
		"webhook_signing_secret": "rotated-signing-secret",
		"subscribed_events":      []string{db.AlertEventIncidentOpened, db.AlertEventIncidentResolved},
	}, "")
	if updateResp.Code != http.StatusOK {
		t.Fatalf("update channel status = %d, body = %s", updateResp.Code, updateResp.Body.String())
	}
	assertNotContains(t, updateResp.Body.String(), "rotated-signing-secret")

	var stored db.AlertChannel
	if err := server.db.Where("id = ?", created.Data.Channel.ID).First(&stored).Error; err != nil {
		t.Fatalf("find updated channel: %v", err)
	}
	if stored.Name != "critical-webhook" || stored.Enabled {
		t.Fatalf("stored channel = %+v, want renamed disabled channel", stored)
	}
	if stored.WebhookURL != "https://alerts.example.com/critical" {
		t.Fatalf("stored webhook url = %q, want updated webhook url", stored.WebhookURL)
	}
	if stored.WebhookSigningSecret != "rotated-signing-secret" {
		t.Fatalf("stored webhook signing secret = %q, want rotated signing secret", stored.WebhookSigningSecret)
	}
	if got := db.DecodeAlertEvents(stored.SubscribedEvents); len(got) != 2 || got[0] != db.AlertEventIncidentOpened || got[1] != db.AlertEventIncidentResolved {
		t.Fatalf("stored subscribed_events = %#v, want opened and resolved", got)
	}

	deleteResp := performJSONRequest(t, server, http.MethodDelete, "/v1/alerts/channels/"+created.Data.Channel.ID, nil, "")
	if deleteResp.Code != http.StatusOK {
		t.Fatalf("delete channel status = %d, body = %s", deleteResp.Code, deleteResp.Body.String())
	}
	var count int64
	if err := server.db.Model(&db.AlertChannel{}).Count(&count).Error; err != nil {
		t.Fatalf("count alert channels: %v", err)
	}
	if count != 0 {
		t.Fatalf("alert channel count = %d, want 0", count)
	}
}

func TestAlertChannelWriteEndpointsPersistChatConfiguration(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	database, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.Migrate(database); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	server := NewServer(database, logging.NewLogger(), &config.Config{})
	for _, channelType := range []string{"slack", "discord"} {
		missingWebhookResp := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/channels", gin.H{
			"name": "missing-" + channelType,
			"type": channelType,
		}, "")
		if missingWebhookResp.Code != http.StatusBadRequest {
			t.Fatalf("missing %s webhook status = %d, body = %s", channelType, missingWebhookResp.Code, missingWebhookResp.Body.String())
		}
	}

	createResp := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/channels", gin.H{
		"name":              "ops-slack",
		"type":              "slack",
		"webhook_url":       "https://hooks.slack.example.com/services/T000/B000/secret",
		"subscribed_events": []string{db.AlertEventIncidentOpened},
	}, "")
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create slack channel status = %d, body = %s", createResp.Code, createResp.Body.String())
	}
	var created struct {
		Data struct {
			Channel struct {
				ID                 string   `json:"id"`
				Name               string   `json:"name"`
				Type               string   `json:"type"`
				WebhookURL         string   `json:"webhook_url"`
				WebhookConfigured  bool     `json:"webhook_configured"`
				SubscribedEvents   []string `json:"subscribed_events"`
				LastDeliveryStatus string   `json:"last_delivery_status"`
			} `json:"channel"`
		} `json:"data"`
	}
	decodeResponse(t, createResp, &created)
	if created.Data.Channel.ID == "" || created.Data.Channel.Name != "ops-slack" || created.Data.Channel.Type != "slack" || created.Data.Channel.WebhookURL == "" || !created.Data.Channel.WebhookConfigured {
		t.Fatalf("created chat channel = %+v, want configured slack channel", created.Data.Channel)
	}
	if got := created.Data.Channel.SubscribedEvents; len(got) != 1 || got[0] != db.AlertEventIncidentOpened {
		t.Fatalf("created subscribed_events = %#v, want incident_opened", got)
	}
	if created.Data.Channel.LastDeliveryStatus != "" {
		t.Fatalf("created last_delivery_status = %q, want empty", created.Data.Channel.LastDeliveryStatus)
	}

	updateResp := performJSONRequest(t, server, http.MethodPatch, "/v1/alerts/channels/"+created.Data.Channel.ID, gin.H{
		"name":              "ops-discord",
		"type":              "discord",
		"webhook_url":       "https://discord.example.com/api/webhooks/1/secret",
		"subscribed_events": []string{db.AlertEventIncidentResolved},
	}, "")
	if updateResp.Code != http.StatusOK {
		t.Fatalf("update chat channel status = %d, body = %s", updateResp.Code, updateResp.Body.String())
	}

	var stored db.AlertChannel
	if err := server.db.Where("id = ?", created.Data.Channel.ID).First(&stored).Error; err != nil {
		t.Fatalf("find updated chat channel: %v", err)
	}
	if stored.Name != "ops-discord" || stored.Type != "discord" || stored.WebhookURL != "https://discord.example.com/api/webhooks/1/secret" {
		t.Fatalf("stored chat channel = %+v, want updated discord channel", stored)
	}
	if got := db.DecodeAlertEvents(stored.SubscribedEvents); len(got) != 1 || got[0] != db.AlertEventIncidentResolved {
		t.Fatalf("stored subscribed_events = %#v, want incident_resolved", got)
	}

	deliveredAt := time.Now().UTC().Add(-time.Minute)
	if err := server.db.Create(&db.AlertDelivery{
		ID:           "delivery-chat-last",
		IncidentID:   "incident-chat",
		EventType:    db.AlertEventIncidentResolved,
		Channel:      stored.Name,
		Type:         stored.Type,
		Status:       "sent",
		AttemptCount: 1,
		MaxAttempts:  3,
		CreatedAt:    deliveredAt,
		UpdatedAt:    deliveredAt,
	}).Error; err != nil {
		t.Fatalf("create chat delivery: %v", err)
	}

	listResp := performJSONRequest(t, server, http.MethodGet, "/v1/alerts/channels", nil, "")
	if listResp.Code != http.StatusOK {
		t.Fatalf("list chat channel status = %d, body = %s", listResp.Code, listResp.Body.String())
	}
	var listed struct {
		Data struct {
			Channels []struct {
				ID                 string     `json:"id"`
				Name               string     `json:"name"`
				Type               string     `json:"type"`
				WebhookConfigured  bool       `json:"webhook_configured"`
				SubscribedEvents   []string   `json:"subscribed_events"`
				LastDeliveryStatus string     `json:"last_delivery_status"`
				LastDeliveryAt     *time.Time `json:"last_delivery_at"`
			} `json:"channels"`
		} `json:"data"`
	}
	decodeResponse(t, listResp, &listed)
	if len(listed.Data.Channels) != 1 {
		t.Fatalf("listed channel count = %d, want 1", len(listed.Data.Channels))
	}
	channel := listed.Data.Channels[0]
	if channel.ID != stored.ID || channel.Name != "ops-discord" || channel.Type != "discord" || !channel.WebhookConfigured {
		t.Fatalf("listed chat channel = %+v, want discord channel with webhook", channel)
	}
	if got := channel.SubscribedEvents; len(got) != 1 || got[0] != db.AlertEventIncidentResolved {
		t.Fatalf("listed subscribed_events = %#v, want incident_resolved", got)
	}
	if channel.LastDeliveryStatus != "sent" || channel.LastDeliveryAt == nil {
		t.Fatalf("listed delivery metadata status=%q at=%v, want sent with timestamp", channel.LastDeliveryStatus, channel.LastDeliveryAt)
	}
}

func TestAlertRouteWriteAndDryRunEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	database, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.Migrate(database); err != nil {
		t.Fatalf("migrate database: %v", err)
	}

	server := NewServer(database, logging.NewLogger(), &config.Config{})
	if err := server.db.Create(&db.AlertChannel{
		ID:         "channel-ops-webhook",
		Name:       "ops-webhook",
		Type:       "webhook",
		Enabled:    true,
		WebhookURL: "https://alerts.example.com/hook",
	}).Error; err != nil {
		t.Fatalf("create alert channel: %v", err)
	}

	createResp := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/routes", gin.H{
		"name":                   "critical route",
		"priority":               10,
		"event_types":            []string{db.AlertEventIncidentOpened},
		"severities":             []string{"high"},
		"channel_ids":            []string{"channel-ops-webhook"},
		"grouping_policy":        db.AlertGroupingPolicyDelayedSummary,
		"grouping_delay_seconds": 120,
	}, "")
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create route status = %d, body = %s", createResp.Code, createResp.Body.String())
	}
	var created struct {
		Data struct {
			Route struct {
				ID                   string   `json:"id"`
				Name                 string   `json:"name"`
				EventTypes           []string `json:"event_types"`
				ChannelIDs           []string `json:"channel_ids"`
				GroupingPolicy       string   `json:"grouping_policy"`
				GroupingDelaySeconds int      `json:"grouping_delay_seconds"`
			} `json:"route"`
		} `json:"data"`
	}
	decodeResponse(t, createResp, &created)
	if created.Data.Route.ID == "" || created.Data.Route.Name != "critical route" || len(created.Data.Route.ChannelIDs) != 1 ||
		created.Data.Route.GroupingPolicy != db.AlertGroupingPolicyDelayedSummary || created.Data.Route.GroupingDelaySeconds != 120 {
		t.Fatalf("created route = %+v, want route with channel", created.Data.Route)
	}

	listResp := performJSONRequest(t, server, http.MethodGet, "/v1/alerts/routes", nil, "")
	if listResp.Code != http.StatusOK {
		t.Fatalf("list routes status = %d, body = %s", listResp.Code, listResp.Body.String())
	}
	var listed struct {
		Data struct {
			Routes []struct {
				ID                   string   `json:"id"`
				Name                 string   `json:"name"`
				Enabled              bool     `json:"enabled"`
				Priority             int      `json:"priority"`
				EventTypes           []string `json:"event_types"`
				Severities           []string `json:"severities"`
				ChannelIDs           []string `json:"channel_ids"`
				Suppress             bool     `json:"suppress"`
				GroupingPolicy       string   `json:"grouping_policy"`
				GroupingDelaySeconds int      `json:"grouping_delay_seconds"`
			} `json:"routes"`
			Count int `json:"count"`
		} `json:"data"`
	}
	decodeResponse(t, listResp, &listed)
	if listed.Data.Count != 1 || len(listed.Data.Routes) != 1 {
		t.Fatalf("listed routes count=%d len=%d, want one route", listed.Data.Count, len(listed.Data.Routes))
	}
	if listed.Data.Routes[0].ID != created.Data.Route.ID || listed.Data.Routes[0].Name != "critical route" || listed.Data.Routes[0].Priority != 10 ||
		listed.Data.Routes[0].GroupingPolicy != db.AlertGroupingPolicyDelayedSummary || listed.Data.Routes[0].GroupingDelaySeconds != 120 {
		t.Fatalf("listed route = %+v, want created critical route", listed.Data.Routes[0])
	}
	if len(listed.Data.Routes[0].EventTypes) != 1 || listed.Data.Routes[0].EventTypes[0] != db.AlertEventIncidentOpened || len(listed.Data.Routes[0].ChannelIDs) != 1 {
		t.Fatalf("listed route filters = %+v, want opened event and channel", listed.Data.Routes[0])
	}

	updateResp := performJSONRequest(t, server, http.MethodPatch, "/v1/alerts/routes/"+created.Data.Route.ID, gin.H{
		"name":                   "suppress recovery",
		"enabled":                false,
		"priority":               5,
		"event_types":            []string{db.AlertEventIncidentResolved},
		"severities":             []string{"medium"},
		"agent_ids":              []string{"agent-prod"},
		"monitor_ids":            []string{"monitor-api"},
		"monitor_types":          []string{"http"},
		"channel_ids":            []string{},
		"suppress":               true,
		"grouping_policy":        db.AlertGroupingPolicyNone,
		"grouping_delay_seconds": 30,
	}, "")
	if updateResp.Code != http.StatusOK {
		t.Fatalf("update route status = %d, body = %s", updateResp.Code, updateResp.Body.String())
	}
	var updated struct {
		Data struct {
			Route struct {
				ID                   string   `json:"id"`
				Name                 string   `json:"name"`
				Enabled              bool     `json:"enabled"`
				Priority             int      `json:"priority"`
				EventTypes           []string `json:"event_types"`
				Severities           []string `json:"severities"`
				AgentIDs             []string `json:"agent_ids"`
				MonitorIDs           []string `json:"monitor_ids"`
				MonitorTypes         []string `json:"monitor_types"`
				ChannelIDs           []string `json:"channel_ids"`
				Suppress             bool     `json:"suppress"`
				GroupingPolicy       string   `json:"grouping_policy"`
				GroupingDelaySeconds int      `json:"grouping_delay_seconds"`
			} `json:"route"`
		} `json:"data"`
	}
	decodeResponse(t, updateResp, &updated)
	if updated.Data.Route.ID != created.Data.Route.ID || updated.Data.Route.Name != "suppress recovery" || updated.Data.Route.Enabled || updated.Data.Route.Priority != 5 ||
		!updated.Data.Route.Suppress || updated.Data.Route.GroupingPolicy != db.AlertGroupingPolicyNone || updated.Data.Route.GroupingDelaySeconds != 30 {
		t.Fatalf("updated route = %+v, want disabled suppress recovery route", updated.Data.Route)
	}
	if len(updated.Data.Route.EventTypes) != 1 || updated.Data.Route.EventTypes[0] != db.AlertEventIncidentResolved {
		t.Fatalf("updated event_types = %#v, want resolved", updated.Data.Route.EventTypes)
	}
	if len(updated.Data.Route.Severities) != 1 || updated.Data.Route.Severities[0] != "medium" ||
		len(updated.Data.Route.AgentIDs) != 1 || updated.Data.Route.AgentIDs[0] != "agent-prod" ||
		len(updated.Data.Route.MonitorIDs) != 1 || updated.Data.Route.MonitorIDs[0] != "monitor-api" ||
		len(updated.Data.Route.MonitorTypes) != 1 || updated.Data.Route.MonitorTypes[0] != "http" ||
		len(updated.Data.Route.ChannelIDs) != 0 {
		t.Fatalf("updated route filters = %+v, want requested filters and no channels", updated.Data.Route)
	}

	deleteResp := performJSONRequest(t, server, http.MethodDelete, "/v1/alerts/routes/"+created.Data.Route.ID, nil, "")
	if deleteResp.Code != http.StatusOK {
		t.Fatalf("delete route status = %d, body = %s", deleteResp.Code, deleteResp.Body.String())
	}
	var routeCount int64
	if err := server.db.Model(&db.AlertRoute{}).Count(&routeCount).Error; err != nil {
		t.Fatalf("count alert routes: %v", err)
	}
	if routeCount != 0 {
		t.Fatalf("alert route count = %d, want 0", routeCount)
	}

	createResp = performJSONRequest(t, server, http.MethodPost, "/v1/alerts/routes", gin.H{
		"name":        "critical route",
		"priority":    10,
		"event_types": []string{db.AlertEventIncidentOpened},
		"severities":  []string{"high"},
		"channel_ids": []string{"channel-ops-webhook"},
	}, "")
	if createResp.Code != http.StatusCreated {
		t.Fatalf("recreate route status = %d, body = %s", createResp.Code, createResp.Body.String())
	}

	dryRunResp := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/routes/dry-run", gin.H{
		"event_type": db.AlertEventIncidentOpened,
		"severity":   "high",
	}, "")
	if dryRunResp.Code != http.StatusOK {
		t.Fatalf("dry-run status = %d, body = %s", dryRunResp.Code, dryRunResp.Body.String())
	}
	var dryRun struct {
		Data struct {
			DryRun struct {
				Suppressed       bool `json:"suppressed"`
				RouteEvaluations []struct {
					Matched bool `json:"matched"`
				} `json:"route_evaluations"`
				DestinationDecisions []struct {
					ChannelName string `json:"channel_name"`
					Status      string `json:"status"`
				} `json:"destination_decisions"`
			} `json:"dry_run"`
		} `json:"data"`
	}
	decodeResponse(t, dryRunResp, &dryRun)
	if dryRun.Data.DryRun.Suppressed || len(dryRun.Data.DryRun.RouteEvaluations) != 1 || !dryRun.Data.DryRun.RouteEvaluations[0].Matched {
		t.Fatalf("dry-run route evaluations = %+v, want one matched non-suppressed route", dryRun.Data.DryRun)
	}
	if len(dryRun.Data.DryRun.DestinationDecisions) != 1 || dryRun.Data.DryRun.DestinationDecisions[0].ChannelName != "ops-webhook" || dryRun.Data.DryRun.DestinationDecisions[0].Status != "pending" {
		t.Fatalf("dry-run destinations = %+v, want pending ops-webhook", dryRun.Data.DryRun.DestinationDecisions)
	}

	var deliveryCount int64
	if err := server.db.Model(&db.AlertDelivery{}).Count(&deliveryCount).Error; err != nil {
		t.Fatalf("count deliveries: %v", err)
	}
	if deliveryCount != 0 {
		t.Fatalf("delivery count = %d, want dry-run to avoid side effects", deliveryCount)
	}
}

func TestAlertSMTPServiceAndEmailDestinationEndpoints(t *testing.T) {
	server := setupTestServer(t)

	createServiceResp := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/smtp-services", gin.H{
		"name":       "Primary SMTP",
		"enabled":    true,
		"host":       "smtp.example.com",
		"port":       587,
		"username":   "mailer",
		"password":   "secret-password",
		"from_email": "orion@example.com",
	}, "")
	if createServiceResp.Code != http.StatusCreated {
		t.Fatalf("create smtp service status = %d, body = %s", createServiceResp.Code, createServiceResp.Body.String())
	}
	assertNotContains(t, createServiceResp.Body.String(), "secret-password")

	var createdService struct {
		Data struct {
			SMTPService struct {
				ID                 string `json:"id"`
				Name               string `json:"name"`
				Host               string `json:"host"`
				Port               int    `json:"port"`
				UsernameConfigured bool   `json:"username_configured"`
				PasswordConfigured bool   `json:"password_configured"`
			} `json:"smtp_service"`
		} `json:"data"`
	}
	decodeResponse(t, createServiceResp, &createdService)
	if createdService.Data.SMTPService.ID == "" || createdService.Data.SMTPService.Name != "Primary SMTP" ||
		createdService.Data.SMTPService.Host != "smtp.example.com" || createdService.Data.SMTPService.Port != 587 ||
		!createdService.Data.SMTPService.UsernameConfigured || !createdService.Data.SMTPService.PasswordConfigured {
		t.Fatalf("created smtp service = %+v, want redacted configured service", createdService.Data.SMTPService)
	}

	createDestinationResp := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/email-destinations", gin.H{
		"smtp_service_id":   createdService.Data.SMTPService.ID,
		"name":              "Ops Email",
		"enabled":           true,
		"email_to":          "ops@example.com",
		"subscribed_events": []string{db.AlertEventIncidentOpened},
	}, "")
	if createDestinationResp.Code != http.StatusCreated {
		t.Fatalf("create email destination status = %d, body = %s", createDestinationResp.Code, createDestinationResp.Body.String())
	}

	var createdDestination struct {
		Data struct {
			EmailDestination struct {
				ID               string   `json:"id"`
				SMTPServiceID    string   `json:"smtp_service_id"`
				SMTPServiceName  string   `json:"smtp_service_name"`
				Name             string   `json:"name"`
				EmailTo          string   `json:"email_to"`
				SubscribedEvents []string `json:"subscribed_events"`
			} `json:"email_destination"`
		} `json:"data"`
	}
	decodeResponse(t, createDestinationResp, &createdDestination)
	if createdDestination.Data.EmailDestination.SMTPServiceID != createdService.Data.SMTPService.ID ||
		createdDestination.Data.EmailDestination.SMTPServiceName != "Primary SMTP" ||
		createdDestination.Data.EmailDestination.Name != "Ops Email" ||
		createdDestination.Data.EmailDestination.EmailTo != "ops@example.com" ||
		len(createdDestination.Data.EmailDestination.SubscribedEvents) != 1 ||
		createdDestination.Data.EmailDestination.SubscribedEvents[0] != db.AlertEventIncidentOpened {
		t.Fatalf("created email destination = %+v, want linked opened-only destination", createdDestination.Data.EmailDestination)
	}

	listServicesResp := performJSONRequest(t, server, http.MethodGet, "/v1/alerts/smtp-services", nil, "")
	if listServicesResp.Code != http.StatusOK {
		t.Fatalf("list smtp services status = %d, body = %s", listServicesResp.Code, listServicesResp.Body.String())
	}
	assertNotContains(t, listServicesResp.Body.String(), "secret-password")

	deleteServiceResp := performJSONRequest(t, server, http.MethodDelete, "/v1/alerts/smtp-services/"+createdService.Data.SMTPService.ID, nil, "")
	if deleteServiceResp.Code != http.StatusConflict {
		t.Fatalf("delete referenced smtp service status = %d, body = %s, want 409", deleteServiceResp.Code, deleteServiceResp.Body.String())
	}

	deleteDestinationResp := performJSONRequest(t, server, http.MethodDelete, "/v1/alerts/email-destinations/"+createdDestination.Data.EmailDestination.ID, nil, "")
	if deleteDestinationResp.Code != http.StatusOK {
		t.Fatalf("delete email destination status = %d, body = %s", deleteDestinationResp.Code, deleteDestinationResp.Body.String())
	}
	deleteServiceResp = performJSONRequest(t, server, http.MethodDelete, "/v1/alerts/smtp-services/"+createdService.Data.SMTPService.ID, nil, "")
	if deleteServiceResp.Code != http.StatusOK {
		t.Fatalf("delete smtp service status = %d, body = %s", deleteServiceResp.Code, deleteServiceResp.Body.String())
	}
}

func TestAlertSMTPServiceAndEmailDestinationTestEndpoints(t *testing.T) {
	server := setupTestServer(t)
	smtpAddress, messages := startAPITestSMTPServer(t)
	host, portValue, err := net.SplitHostPort(smtpAddress)
	if err != nil {
		t.Fatalf("split smtp address: %v", err)
	}
	port, err := strconv.Atoi(portValue)
	if err != nil {
		t.Fatalf("parse smtp port: %v", err)
	}

	createServiceResp := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/smtp-services", gin.H{
		"name":       "Local SMTP",
		"enabled":    true,
		"host":       host,
		"port":       port,
		"from_email": "orion@example.com",
	}, "")
	if createServiceResp.Code != http.StatusCreated {
		t.Fatalf("create smtp service status = %d, body = %s", createServiceResp.Code, createServiceResp.Body.String())
	}
	var createdService struct {
		Data struct {
			SMTPService struct {
				ID string `json:"id"`
			} `json:"smtp_service"`
		} `json:"data"`
	}
	decodeResponse(t, createServiceResp, &createdService)

	testServiceResp := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/smtp-services/"+createdService.Data.SMTPService.ID+"/test", nil, "")
	if testServiceResp.Code != http.StatusOK {
		t.Fatalf("test smtp service status = %d, body = %s", testServiceResp.Code, testServiceResp.Body.String())
	}
	var testedService struct {
		Data struct {
			Test struct {
				SMTPServiceID string `json:"smtp_service_id"`
				Status        string `json:"status"`
				Stage         string `json:"stage"`
				Error         string `json:"error"`
			} `json:"test"`
		} `json:"data"`
	}
	decodeResponse(t, testServiceResp, &testedService)
	if testedService.Data.Test.SMTPServiceID != createdService.Data.SMTPService.ID || testedService.Data.Test.Status != "ok" || testedService.Data.Test.Stage != "connected" || testedService.Data.Test.Error != "" {
		t.Fatalf("smtp service test = %+v, want successful connectivity result", testedService.Data.Test)
	}

	createDestinationResp := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/email-destinations", gin.H{
		"smtp_service_id": createdService.Data.SMTPService.ID,
		"name":            "Ops Email",
		"enabled":         true,
		"email_to":        "ops@example.com",
	}, "")
	if createDestinationResp.Code != http.StatusCreated {
		t.Fatalf("create email destination status = %d, body = %s", createDestinationResp.Code, createDestinationResp.Body.String())
	}
	var createdDestination struct {
		Data struct {
			EmailDestination struct {
				ID string `json:"id"`
			} `json:"email_destination"`
		} `json:"data"`
	}
	decodeResponse(t, createDestinationResp, &createdDestination)

	testDestinationResp := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/email-destinations/"+createdDestination.Data.EmailDestination.ID+"/test", nil, "")
	if testDestinationResp.Code != http.StatusOK {
		t.Fatalf("test email destination status = %d, body = %s", testDestinationResp.Code, testDestinationResp.Body.String())
	}
	var testedDestination struct {
		Data struct {
			Delivery struct {
				IncidentID string `json:"incident_id"`
				EventType  string `json:"event_type"`
				Channel    string `json:"channel"`
				Type       string `json:"type"`
				Status     string `json:"status"`
				Error      string `json:"error"`
			} `json:"delivery"`
		} `json:"data"`
	}
	decodeResponse(t, testDestinationResp, &testedDestination)
	if testedDestination.Data.Delivery.IncidentID != "alert-email-destination-test" ||
		testedDestination.Data.Delivery.EventType != "test" ||
		testedDestination.Data.Delivery.Channel != "Ops Email" ||
		testedDestination.Data.Delivery.Type != "email" ||
		testedDestination.Data.Delivery.Status != "sent" ||
		testedDestination.Data.Delivery.Error != "" {
		t.Fatalf("email destination test delivery = %+v, want sent sanitized test delivery", testedDestination.Data.Delivery)
	}
	select {
	case message := <-messages:
		if !strings.Contains(message, "To: ops@example.com") || !strings.Contains(message, "Subject: Orion alert: Alert channel test") {
			t.Fatalf("email message = %q, want destination test email content", message)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for email destination test")
	}
}

func TestAlertSMTPServiceTestEndpointSanitizesFailures(t *testing.T) {
	server := setupTestServer(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen closed smtp address: %v", err)
	}
	address := listener.Addr().String()
	_ = listener.Close()
	host, portValue, err := net.SplitHostPort(address)
	if err != nil {
		t.Fatalf("split smtp address: %v", err)
	}
	port, err := strconv.Atoi(portValue)
	if err != nil {
		t.Fatalf("parse smtp port: %v", err)
	}

	createServiceResp := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/smtp-services", gin.H{
		"name":       "Failing SMTP",
		"enabled":    true,
		"host":       host,
		"port":       port,
		"username":   "mailer",
		"password":   "secret-password",
		"from_email": "orion@example.com",
	}, "")
	if createServiceResp.Code != http.StatusCreated {
		t.Fatalf("create smtp service status = %d, body = %s", createServiceResp.Code, createServiceResp.Body.String())
	}
	var createdService struct {
		Data struct {
			SMTPService struct {
				ID string `json:"id"`
			} `json:"smtp_service"`
		} `json:"data"`
	}
	decodeResponse(t, createServiceResp, &createdService)

	testServiceResp := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/smtp-services/"+createdService.Data.SMTPService.ID+"/test", nil, "")
	if testServiceResp.Code != http.StatusOK {
		t.Fatalf("test smtp service status = %d, body = %s", testServiceResp.Code, testServiceResp.Body.String())
	}
	assertNotContains(t, testServiceResp.Body.String(), "secret-password")
	assertNotContains(t, testServiceResp.Body.String(), "mailer")

	var testedService struct {
		Data struct {
			Test struct {
				Status string `json:"status"`
				Stage  string `json:"stage"`
				Error  string `json:"error"`
			} `json:"test"`
		} `json:"data"`
	}
	decodeResponse(t, testServiceResp, &testedService)
	if testedService.Data.Test.Status != "failed" || testedService.Data.Test.Stage != "smtp_connect" || testedService.Data.Test.Error != "smtp connectivity failed; check Core logs" {
		t.Fatalf("smtp service test = %+v, want sanitized failed result", testedService.Data.Test)
	}
}
