package api

import (
	"encoding/json"
	"fmt"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"net/http"
	"net/http/httptest"
	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"strings"
	"testing"
	"time"
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
	server := NewServer(database, utils.NewLogger(), &config.Config{AlertRecoveryNotifications: true, AlertTLSExpiryDays: 14, AlertCooldownSeconds: 300})
	if err := server.db.Create(&db.AlertChannel{ID: "alert-channel-webhook", Name: "ops-webhook", Type: "webhook", Enabled: true, WebhookURL: "https://secret.example.com/hook", WebhookSigningSecret: "webhook-signing-secret"}).Error; err != nil {
		t.Fatalf("create webhook channel: %v", err)
	}
	if err := server.db.Create(&db.AlertChannel{ID: "alert-channel-email", Name: "ops-email", Type: "email", Enabled: false}).Error; err != nil {
		t.Fatalf("create email channel: %v", err)
	}
	delivery := db.AlertDelivery{ID: "alert-delivery-test", IncidentID: "incident-test", AlertGroupID: "alert-group-test", EventType: "incident_opened", Channel: "ops-webhook", Type: "webhook", Status: "failed", Error: "post https://secret.example.com/hook: connection refused", AttemptCount: 1, MaxAttempts: 3}
	if err := server.db.Create(&delivery).Error; err != nil {
		t.Fatalf("create alert delivery: %v", err)
	}
	attemptTime := time.Now().UTC()
	if err := server.db.Create(&db.AlertDeliveryAttempt{ID: "alert-delivery-attempt-test", AlertDeliveryID: delivery.ID, AttemptNumber: 1, Status: "failed", Stage: "http_request", Error: "post https://secret.example.com/hook: connection refused", StartedAt: attemptTime, CompletedAt: &attemptTime}).Error; err != nil {
		t.Fatalf("create alert delivery attempt: %v", err)
	}
	secondDelivery := db.AlertDelivery{ID: "alert-delivery-sent", IncidentID: "incident-other", EventType: "incident_resolved", Channel: "ops-email", Type: "email", Status: "sent"}
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
	if !channels.Success || channels.Data.Count != 1 || len(channels.Data.Channels) != 1 {
		t.Fatalf("channels response = %+v, want one non-email channel", channels)
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
	createResp := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/channels", gin.H{"name": "ops-webhook", "type": "webhook", "enabled": false, "webhook_url": webhookServer.URL}, "")
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
	server := NewServer(database, utils.NewLogger(), &config.Config{})
	createResp := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/channels", gin.H{"name": "ops-webhook", "type": "webhook", "enabled": true, "webhook_url": "https://secret.example.com/hook", "webhook_signing_secret": "initial-signing-secret", "subscribed_events": []string{db.AlertEventIncidentOpened}}, "")
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
	updateResp := performJSONRequest(t, server, http.MethodPatch, "/v1/alerts/channels/"+created.Data.Channel.ID, gin.H{"name": "critical-webhook", "enabled": false, "webhook_url": "https://alerts.example.com/critical", "webhook_signing_secret": "rotated-signing-secret", "subscribed_events": []string{db.AlertEventIncidentOpened, db.AlertEventIncidentResolved}}, "")
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
func TestAlertChannelWriteEndpointsRejectNonWebhookTypes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	database, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.Migrate(database); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	server := NewServer(database, utils.NewLogger(), &config.Config{})
	for _, channelType := range []string{"slack", "discord", "email"} {
		createResp := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/channels", gin.H{"name": "ops-" + channelType, "type": channelType, "webhook_url": "https://alerts.example.com/" + channelType}, "")
		if createResp.Code != http.StatusBadRequest || !strings.Contains(createResp.Body.String(), "unsupported alert channel type") {
			t.Fatalf("create %s channel status = %d, body = %s, want unsupported type rejection", channelType, createResp.Code, createResp.Body.String())
		}
	}
}
