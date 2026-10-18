package service

import (
	"orion/core/internal/db"
	"strings"
	"time"
)

func alertTestIncident() db.Incident {
	return db.Incident{ID: "alert-channel-test", Status: "test", Severity: "info", Title: "Alert channel test", LatestEvent: "Manual alert channel test"}
}

// SamplePayload is the body a webhook receives for a test event.
func (s *AlertService) SamplePayload() AlertPayload {
	return s.buildAlertPayload(alertTestIncident(), "test", time.Now().UTC())
}

// PayloadForDelivery rebuilds the payload a delivery sent (or will send).
func (s *AlertService) PayloadForDelivery(d db.AlertDelivery) (AlertPayload, error) {
	now := time.Now().UTC()
	if d.EventType == alertEventGroupSummary && d.AlertGroupID != "" {
		return s.buildAlertGroupSummaryPayload(d.AlertGroupID, now)
	}
	if d.EventType == "test" || d.IncidentID == "alert-channel-test" {
		return s.buildAlertPayload(alertTestIncident(), "test", now), nil
	}
	var incident db.Incident
	if err := s.db.Where("id = ?", d.IncidentID).First(&incident).Error; err != nil {
		return AlertPayload{}, err
	}
	return s.buildAlertPayload(incident, d.EventType, now), nil
}

// WebhookTestResult is the outcome of one test POST that was not stored anywhere.
type WebhookTestResult struct {
	Record   SendRecord
	Duration time.Duration
	Stage    string
	Err      error
}

// RunWebhookTest posts a test payload to an unsaved webhook channel and returns what happened.
// Nothing is written to the database.
func (s *AlertService) RunWebhookTest(channel db.AlertChannel) WebhookTestResult {
	start := time.Now()
	rec := &SendRecord{}
	err := s.deliverWebhookPayload(channel, s.SamplePayload(), rec)
	out := WebhookTestResult{Record: *rec, Duration: time.Since(start), Err: err}
	if err != nil {
		out.Stage = alertDeliveryErrorStage(err)
	}
	return out
}

// TestHint turns a failed send into the sentence under "Test failed".
func TestHint(stage string, err error) string {
	if err == nil {
		return ""
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "x509") || strings.Contains(msg, "tls") || strings.Contains(msg, "certificate"):
		return "DNS resolved. The TLS handshake did not complete. Check the URL, its certificate and your firewall."
	case strings.Contains(msg, "no such host"):
		return "The host name did not resolve. Check the URL."
	case strings.Contains(msg, "deadline exceeded") || strings.Contains(msg, "timeout"):
		return "The endpoint did not answer in time. Check the URL and your firewall."
	case strings.Contains(msg, "refused"):
		return "The connection was refused. Check that the service is listening."
	case strings.Contains(msg, "status 4"):
		return "The endpoint rejected the request. Check the URL and any authentication header."
	case strings.Contains(msg, "status 5"):
		return "The endpoint failed on its side. Check its logs and retry."
	}
	return ""
}
