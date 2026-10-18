package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"orion/core/internal/db"
	"strings"
	"time"
)

func (s *AlertService) deliver(channel db.AlertChannel, deliveryID string, incidentID string, eventType string, rec *SendRecord) error {
	var incident db.Incident
	if err := s.db.Where("id = ?", incidentID).First(&incident).Error; err != nil {
		return newAlertDeliveryError("load_incident", err)
	}

	return s.deliverPayload(channel, deliveryID, s.buildAlertPayload(incident, eventType, time.Now().UTC()), rec)
}

// deliverPayload sends one rendered alert through the channel's transport.
func (s *AlertService) deliverPayload(channel db.AlertChannel, deliveryID string, payload AlertPayload, rec *SendRecord) error {
	switch channel.Type {
	case "webhook":
		return s.deliverWebhookPayload(channel, payload, rec)
	case "email":
		return s.deliverEmailPayload(channel, deliveryID, payload, rec)
	default:
		return newAlertDeliveryError("channel_type", fmt.Errorf("unsupported alert channel type: %s", channel.Type))
	}
}

func (s *AlertService) deliverGroupSummary(channel db.AlertChannel, deliveryID string, groupID string, rec *SendRecord) error {
	payload, err := s.buildAlertGroupSummaryPayload(groupID, time.Now().UTC())
	if err != nil {
		return err
	}
	return s.deliverPayload(channel, deliveryID, payload, rec)
}

func (s *AlertService) buildAlertGroupSummaryPayload(groupID string, deliveredAt time.Time) (AlertPayload, error) {
	var group db.AlertGroup
	if err := s.db.Where("id = ?", groupID).First(&group).Error; err != nil {
		return AlertPayload{}, newAlertDeliveryError("load_alert_group", err)
	}

	incidentID := group.LastIncidentID
	if strings.TrimSpace(incidentID) == "" {
		incidentID = group.FirstIncidentID
	}
	var incident db.Incident
	if err := s.db.Where("id = ?", incidentID).First(&incident).Error; err != nil {
		return AlertPayload{}, newAlertDeliveryError("load_incident", err)
	}

	payload := s.buildAlertPayload(incident, alertEventGroupSummary, deliveredAt)
	title := fmt.Sprintf("%d grouped %s incident(s)", group.IncidentCount, group.Severity)
	if strings.TrimSpace(group.Summary) != "" {
		title = group.Summary
	}
	payload.Summary = AlertPayloadSummary{
		Title: title,
		Text:  fmt.Sprintf("%s across alert group %s. Latest incident: %s.", title, group.ID, incident.Title),
	}
	return payload, nil
}

func (s *AlertService) deliverTest(channel db.AlertChannel, deliveryID string, rec *SendRecord) error {
	incident := db.Incident{
		ID:          "alert-channel-test",
		Status:      "test",
		Severity:    "info",
		Title:       "Alert channel test",
		LatestEvent: "Manual alert channel test",
	}

	return s.deliverPayload(channel, deliveryID, s.buildAlertPayload(incident, "test", time.Now().UTC()), rec)
}

func (s *AlertService) deliverWebhookPayload(channel db.AlertChannel, payload AlertPayload, rec *SendRecord) error {
	if channel.WebhookURL == "" {
		return newAlertDeliveryError("configure", fmt.Errorf("webhook URL is not configured"))
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return newAlertDeliveryError("serialize", err)
	}

	request, err := http.NewRequest(http.MethodPost, channel.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return newAlertDeliveryError("http_request", err)
	}
	for name, value := range DecodeChannelHeaders(channel.HeadersJSON) {
		request.Header.Set(name, value)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "orion-core-alerts/1")
	request.Header.Set("X-Orion-Payload-Version", AlertPayloadVersion)
	if signingSecret := strings.TrimSpace(channel.WebhookSigningSecret); signingSecret != "" {
		signature := SignAlertWebhookPayload(signingSecret, payload.DeliveredAt, body)
		request.Header.Set(signature.Header, signature.Value)
		request.Header.Set("X-Orion-Timestamp", signature.Timestamp)
	}

	resp, err := s.httpClient.Do(request)
	if err != nil {
		return newAlertDeliveryError("http_request", err)
	}
	defer resp.Body.Close()
	rec.fromResponse(resp)
	rec.ScrubSnippet(DecodeChannelHeaders(channel.HeadersJSON))

	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		return newAlertDeliveryError("http_response", errAlertWebhookRedirect)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return newAlertDeliveryError("http_response", fmt.Errorf("webhook returned status %d", resp.StatusCode))
	}
	return nil
}
