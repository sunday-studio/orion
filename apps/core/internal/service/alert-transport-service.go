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

func (s *AlertService) deliver(channel db.AlertChannel, incidentID string, eventType string) error {
	var incident db.Incident
	if err := s.db.Where("id = ?", incidentID).First(&incident).Error; err != nil {
		return newAlertDeliveryError("load_incident", err)
	}

	switch channel.Type {
	case "webhook":
		return s.deliverWebhook(channel, incident, eventType)
	default:
		return newAlertDeliveryError("channel_type", fmt.Errorf("unsupported alert channel type: %s", channel.Type))
	}
}

func (s *AlertService) deliverGroupSummary(channel db.AlertChannel, groupID string) error {
	payload, err := s.buildAlertGroupSummaryPayload(groupID, time.Now().UTC())
	if err != nil {
		return err
	}
	switch channel.Type {
	case "webhook":
		return s.deliverWebhookPayload(channel, payload)
	default:
		return newAlertDeliveryError("channel_type", fmt.Errorf("unsupported alert channel type: %s", channel.Type))
	}
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

func (s *AlertService) deliverTest(channel db.AlertChannel) error {
	incident := db.Incident{
		ID:          "alert-channel-test",
		Status:      "test",
		Severity:    "info",
		Title:       "Alert channel test",
		LatestEvent: "Manual alert channel test",
	}

	switch channel.Type {
	case "webhook":
		return s.deliverWebhook(channel, incident, "test")
	default:
		return newAlertDeliveryError("channel_type", fmt.Errorf("unsupported alert channel type: %s", channel.Type))
	}
}

func (s *AlertService) deliverWebhook(channel db.AlertChannel, incident db.Incident, eventType string) error {
	payload := s.buildAlertPayload(incident, eventType, time.Now().UTC())
	return s.deliverWebhookPayload(channel, payload)
}

func (s *AlertService) deliverWebhookPayload(channel db.AlertChannel, payload AlertPayload) error {
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

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return newAlertDeliveryError("http_response", fmt.Errorf("webhook returned status %d", resp.StatusCode))
	}
	return nil
}
