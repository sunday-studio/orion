package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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
		ID:      "alert-channel-email",
		Name:    "ops-email",
		Type:    "email",
		Enabled: false,
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

	server := NewServer(database, logging.NewLogger(), &config.Config{})
	for _, channelType := range []string{"slack", "discord", "email"} {
		createResp := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/channels", gin.H{
			"name":        "ops-" + channelType,
			"type":        channelType,
			"webhook_url": "https://alerts.example.com/" + channelType,
		}, "")
		if createResp.Code != http.StatusBadRequest || !strings.Contains(createResp.Body.String(), "unsupported alert channel type") {
			t.Fatalf("create %s channel status = %d, body = %s, want unsupported type rejection", channelType, createResp.Code, createResp.Body.String())
		}
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

func TestAlertRuleWriteEnableDisableAndDryRunEndpoints(t *testing.T) {
	server := setupTestServer(t)
	if err := server.db.Create(&db.AlertChannel{
		ID:         "channel-ops-webhook",
		Name:       "ops-webhook",
		Type:       "webhook",
		Enabled:    true,
		WebhookURL: "https://alerts.example.com/hook",
	}).Error; err != nil {
		t.Fatalf("create webhook channel: %v", err)
	}
	if err := server.db.Create(&db.AlertChannel{
		ID:         "channel-ops-slack",
		Name:       "ops-slack",
		Type:       "slack",
		Enabled:    true,
		WebhookURL: "https://alerts.example.com/slack",
	}).Error; err != nil {
		t.Fatalf("create slack channel: %v", err)
	}

	rejectResp := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/rules", gin.H{
		"name":        "chat rule",
		"channel_ids": []string{"channel-ops-slack"},
	}, "")
	if rejectResp.Code != http.StatusBadRequest || !strings.Contains(rejectResp.Body.String(), "webhook alert channels") {
		t.Fatalf("chat rule status = %d body = %s, want webhook-only rejection", rejectResp.Code, rejectResp.Body.String())
	}

	createResp := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/rules", gin.H{
		"name":                   "critical webhook rule",
		"priority":               10,
		"event_types":            []string{db.AlertEventIncidentOpened},
		"severities":             []string{"high"},
		"agent_ids":              []string{"agent-prod"},
		"monitor_types":          []string{"http"},
		"channel_ids":            []string{"channel-ops-webhook"},
		"grouping_policy":        db.AlertGroupingPolicyDelayedSummary,
		"grouping_delay_seconds": 90,
	}, "")
	if createResp.Code != http.StatusCreated {
		t.Fatalf("create rule status = %d, body = %s", createResp.Code, createResp.Body.String())
	}
	var created struct {
		Data struct {
			Rule struct {
				ID                   string   `json:"id"`
				Name                 string   `json:"name"`
				Enabled              bool     `json:"enabled"`
				Priority             int      `json:"priority"`
				EventTypes           []string `json:"event_types"`
				Severities           []string `json:"severities"`
				AgentIDs             []string `json:"agent_ids"`
				MonitorTypes         []string `json:"monitor_types"`
				ChannelIDs           []string `json:"channel_ids"`
				GroupingPolicy       string   `json:"grouping_policy"`
				GroupingDelaySeconds int      `json:"grouping_delay_seconds"`
			} `json:"rule"`
		} `json:"data"`
	}
	decodeResponse(t, createResp, &created)
	if created.Data.Rule.ID == "" || created.Data.Rule.Name != "critical webhook rule" || !created.Data.Rule.Enabled || created.Data.Rule.Priority != 10 ||
		created.Data.Rule.GroupingPolicy != db.AlertGroupingPolicyDelayedSummary || created.Data.Rule.GroupingDelaySeconds != 90 {
		t.Fatalf("created rule = %+v, want critical webhook rule", created.Data.Rule)
	}
	if len(created.Data.Rule.EventTypes) != 1 || created.Data.Rule.EventTypes[0] != db.AlertEventIncidentOpened ||
		len(created.Data.Rule.Severities) != 1 || created.Data.Rule.Severities[0] != "high" ||
		len(created.Data.Rule.AgentIDs) != 1 || created.Data.Rule.AgentIDs[0] != "agent-prod" ||
		len(created.Data.Rule.MonitorTypes) != 1 || created.Data.Rule.MonitorTypes[0] != "http" ||
		len(created.Data.Rule.ChannelIDs) != 1 || created.Data.Rule.ChannelIDs[0] != "channel-ops-webhook" {
		t.Fatalf("created rule filters = %+v, want requested filters and webhook channel", created.Data.Rule)
	}

	listResp := performJSONRequest(t, server, http.MethodGet, "/v1/alerts/rules", nil, "")
	if listResp.Code != http.StatusOK {
		t.Fatalf("list rules status = %d, body = %s", listResp.Code, listResp.Body.String())
	}
	var listed struct {
		Data struct {
			Rules []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"rules"`
			Count int `json:"count"`
		} `json:"data"`
	}
	decodeResponse(t, listResp, &listed)
	if listed.Data.Count != 1 || len(listed.Data.Rules) != 1 || listed.Data.Rules[0].ID != created.Data.Rule.ID {
		t.Fatalf("listed rules = %+v, want created rule", listed.Data)
	}

	invalidUpdateResp := performJSONRequest(t, server, http.MethodPatch, "/v1/alerts/rules/"+created.Data.Rule.ID, gin.H{
		"event_types": []string{"not-an-alert-event"},
	}, "")
	if invalidUpdateResp.Code != http.StatusBadRequest || !strings.Contains(invalidUpdateResp.Body.String(), "invalid event_types") {
		t.Fatalf("invalid update status = %d, body = %s, want invalid event rejection", invalidUpdateResp.Code, invalidUpdateResp.Body.String())
	}

	disableResp := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/rules/"+created.Data.Rule.ID+"/disable", nil, "")
	if disableResp.Code != http.StatusOK {
		t.Fatalf("disable rule status = %d, body = %s", disableResp.Code, disableResp.Body.String())
	}
	var disabled struct {
		Data struct {
			Rule struct {
				Enabled bool `json:"enabled"`
			} `json:"rule"`
		} `json:"data"`
	}
	decodeResponse(t, disableResp, &disabled)
	if disabled.Data.Rule.Enabled {
		t.Fatalf("disabled rule enabled = true, want false")
	}

	enableResp := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/rules/"+created.Data.Rule.ID+"/enable", nil, "")
	if enableResp.Code != http.StatusOK {
		t.Fatalf("enable rule status = %d, body = %s", enableResp.Code, enableResp.Body.String())
	}

	updateResp := performJSONRequest(t, server, http.MethodPatch, "/v1/alerts/rules/"+created.Data.Rule.ID, gin.H{
		"name":            "suppress webhook recovery",
		"event_types":     []string{db.AlertEventIncidentResolved},
		"severities":      []string{"medium"},
		"channel_ids":     []string{},
		"suppress":        true,
		"grouping_policy": db.AlertGroupingPolicyNone,
	}, "")
	if updateResp.Code != http.StatusOK {
		t.Fatalf("update rule status = %d, body = %s", updateResp.Code, updateResp.Body.String())
	}
	var updated struct {
		Data struct {
			Rule struct {
				Name           string   `json:"name"`
				Enabled        bool     `json:"enabled"`
				EventTypes     []string `json:"event_types"`
				ChannelIDs     []string `json:"channel_ids"`
				Suppress       bool     `json:"suppress"`
				GroupingPolicy string   `json:"grouping_policy"`
			} `json:"rule"`
		} `json:"data"`
	}
	decodeResponse(t, updateResp, &updated)
	if updated.Data.Rule.Name != "suppress webhook recovery" || !updated.Data.Rule.Enabled || !updated.Data.Rule.Suppress ||
		updated.Data.Rule.GroupingPolicy != db.AlertGroupingPolicyNone || len(updated.Data.Rule.ChannelIDs) != 0 ||
		len(updated.Data.Rule.EventTypes) != 1 || updated.Data.Rule.EventTypes[0] != db.AlertEventIncidentResolved {
		t.Fatalf("updated rule = %+v, want enabled suppress recovery rule", updated.Data.Rule)
	}

	dryRunResp := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/rules/dry-run", gin.H{
		"event_type":   db.AlertEventIncidentResolved,
		"severity":     "medium",
		"agent_id":     "agent-prod",
		"monitor_type": "http",
	}, "")
	if dryRunResp.Code != http.StatusOK {
		t.Fatalf("dry-run rule status = %d, body = %s", dryRunResp.Code, dryRunResp.Body.String())
	}
	var dryRun struct {
		Data struct {
			DryRun struct {
				Suppressed      bool `json:"suppressed"`
				RuleEvaluations []struct {
					Rule struct {
						ID string `json:"id"`
					} `json:"rule"`
					Matched    bool `json:"matched"`
					Suppressed bool `json:"suppressed"`
				} `json:"rule_evaluations"`
				DestinationDecisions []struct {
					RuleID   string `json:"rule_id"`
					RuleName string `json:"rule_name"`
					Status   string `json:"status"`
				} `json:"destination_decisions"`
			} `json:"dry_run"`
		} `json:"data"`
	}
	decodeResponse(t, dryRunResp, &dryRun)
	assertNotContains(t, dryRunResp.Body.String(), "route_id")
	assertNotContains(t, dryRunResp.Body.String(), "route_name")
	if !dryRun.Data.DryRun.Suppressed || len(dryRun.Data.DryRun.RuleEvaluations) != 1 ||
		dryRun.Data.DryRun.RuleEvaluations[0].Rule.ID != created.Data.Rule.ID ||
		!dryRun.Data.DryRun.RuleEvaluations[0].Matched || !dryRun.Data.DryRun.RuleEvaluations[0].Suppressed ||
		len(dryRun.Data.DryRun.DestinationDecisions) != 0 {
		t.Fatalf("dry-run rule response = %+v, want suppressing matched rule without destinations", dryRun.Data.DryRun)
	}
	var deliveryCount int64
	if err := server.db.Model(&db.AlertDelivery{}).Count(&deliveryCount).Error; err != nil {
		t.Fatalf("count deliveries: %v", err)
	}
	if deliveryCount != 0 {
		t.Fatalf("delivery count = %d, want rule dry-run to avoid side effects", deliveryCount)
	}

	deleteResp := performJSONRequest(t, server, http.MethodDelete, "/v1/alerts/rules/"+created.Data.Rule.ID, nil, "")
	if deleteResp.Code != http.StatusOK {
		t.Fatalf("delete rule status = %d, body = %s", deleteResp.Code, deleteResp.Body.String())
	}
	var ruleCount int64
	if err := server.db.Model(&db.AlertRoute{}).Count(&ruleCount).Error; err != nil {
		t.Fatalf("count alert rules: %v", err)
	}
	if ruleCount != 0 {
		t.Fatalf("alert rule count = %d, want 0", ruleCount)
	}
}

func TestRemovedAlertDestinationEndpointsAreUnavailable(t *testing.T) {
	server := setupTestServer(t)
	removedEndpoints := []struct {
		method string
		path   string
		body   interface{}
	}{
		{method: http.MethodGet, path: "/v1/alerts/smtp-services"},
		{method: http.MethodPost, path: "/v1/alerts/smtp-services", body: gin.H{"name": "SMTP"}},
		{method: http.MethodGet, path: "/v1/alerts/email-destinations"},
		{method: http.MethodPost, path: "/v1/alerts/email-destinations", body: gin.H{"name": "Ops Email"}},
	}
	for _, endpoint := range removedEndpoints {
		resp := performJSONRequest(t, server, endpoint.method, endpoint.path, endpoint.body, "")
		if resp.Code != http.StatusNotFound {
			t.Fatalf("%s %s status = %d, body = %s, want 404", endpoint.method, endpoint.path, resp.Code, resp.Body.String())
		}
	}
}
