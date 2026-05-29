package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html"
	"orion/core/internal/db"
	"strings"
	"time"
)

const AlertPayloadVersion = "orion.alert.v1"

type AlertPayload struct {
	Version     string               `json:"version"`
	EventType   string               `json:"event_type"`
	DeliveredAt time.Time            `json:"delivered_at"`
	Incident    AlertPayloadIncident `json:"incident"`
	Agent       *AlertPayloadAgent   `json:"agent,omitempty"`
	Monitor     *AlertPayloadMonitor `json:"monitor,omitempty"`
	Summary     AlertPayloadSummary  `json:"summary"`
	Test        bool                 `json:"test,omitempty"`
}

type AlertPayloadIncident struct {
	ID          string     `json:"id"`
	Status      string     `json:"status"`
	Severity    string     `json:"severity"`
	Title       string     `json:"title"`
	AgentID     string     `json:"agent_id,omitempty"`
	MonitorID   string     `json:"monitor_id,omitempty"`
	OpenedAt    time.Time  `json:"opened_at"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
	LastEventAt time.Time  `json:"last_event_at"`
	LatestEvent string     `json:"latest_event,omitempty"`
}

type AlertPayloadAgent struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

type AlertPayloadMonitor struct {
	ID             string `json:"id"`
	Name           string `json:"name,omitempty"`
	Type           string `json:"type,omitempty"`
	Health         string `json:"health,omitempty"`
	ComputedHealth string `json:"computed_health,omitempty"`
}

type AlertPayloadSummary struct {
	Title string `json:"title"`
	Text  string `json:"text"`
}

type AlertEmailTemplate struct {
	Subject  string
	Body     string
	HTMLBody string
}

type AlertWebhookSignature struct {
	Timestamp string
	Header    string
	Value     string
}

func (s *AlertService) buildAlertPayload(incident db.Incident, eventType string, deliveredAt time.Time) AlertPayload {
	payload := AlertPayload{
		Version:     AlertPayloadVersion,
		EventType:   eventType,
		DeliveredAt: deliveredAt.UTC(),
		Incident: AlertPayloadIncident{
			ID:          incident.ID,
			Status:      incident.Status,
			Severity:    incident.Severity,
			Title:       incident.Title,
			AgentID:     incident.AgentID,
			MonitorID:   incident.MonitorID,
			OpenedAt:    incident.OpenedAt,
			ResolvedAt:  incident.ResolvedAt,
			LastEventAt: incident.LastEventAt,
			LatestEvent: incident.LatestEvent,
		},
		Test: incident.ID == "alert-channel-test" || eventType == "test",
	}

	if strings.TrimSpace(incident.MonitorID) != "" {
		var monitor db.Monitor
		if err := s.db.Where("id = ?", incident.MonitorID).First(&monitor).Error; err == nil {
			payload.Monitor = &AlertPayloadMonitor{
				ID:             monitor.ID,
				Name:           monitor.Name,
				Type:           monitor.Type,
				Health:         monitor.Health,
				ComputedHealth: monitor.ComputedHealth,
			}
		}
	}

	if strings.TrimSpace(incident.AgentID) != "" {
		var agent db.Agent
		if err := s.db.Where("id = ?", incident.AgentID).First(&agent).Error; err == nil {
			payload.Agent = &AlertPayloadAgent{
				ID:   agent.ID,
				Name: agent.Name,
			}
		}
	}

	payload.Summary = alertPayloadSummary(payload)
	return payload
}

func RenderAlertEmail(payload AlertPayload) AlertEmailTemplate {
	body := strings.Join([]string{
		payload.Summary.Text,
		"",
		"Event: " + payload.EventType,
		"Incident: " + payload.Incident.ID,
		"Status: " + payload.Incident.Status,
		"Severity: " + payload.Incident.Severity,
		"Monitor: " + alertPayloadMonitorLabel(payload),
		"Agent: " + alertPayloadAgentLabel(payload),
		"Latest event: " + payload.Incident.LatestEvent,
		"Payload version: " + payload.Version,
	}, "\n") + "\n"

	return AlertEmailTemplate{
		Subject:  sanitizeEmailHeader("Orion alert: " + payload.Summary.Title),
		Body:     body,
		HTMLBody: renderAlertEmailHTML(payload),
	}
}

func SignAlertWebhookPayload(secret string, timestamp time.Time, body []byte) AlertWebhookSignature {
	stamp := timestamp.UTC().Format(time.RFC3339)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(stamp))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write(body)
	digest := hex.EncodeToString(mac.Sum(nil))
	return AlertWebhookSignature{
		Timestamp: stamp,
		Header:    "X-Orion-Signature",
		Value:     "t=" + stamp + ",v1=" + digest,
	}
}

func alertPayloadSummary(payload AlertPayload) AlertPayloadSummary {
	title := payload.Incident.Title
	if strings.TrimSpace(title) == "" {
		title = payload.Incident.ID
	}
	return AlertPayloadSummary{
		Title: title,
		Text:  fmt.Sprintf("%s: %s is %s (%s)", payload.EventType, title, payload.Incident.Status, payload.Incident.Severity),
	}
}

func alertPayloadMonitorLabel(payload AlertPayload) string {
	if payload.Monitor == nil {
		return payload.Incident.MonitorID
	}
	if payload.Monitor.Name != "" {
		return fmt.Sprintf("%s (%s)", payload.Monitor.Name, payload.Monitor.ID)
	}
	return payload.Monitor.ID
}

func alertPayloadAgentLabel(payload AlertPayload) string {
	if payload.Agent == nil {
		return payload.Incident.AgentID
	}
	if payload.Agent.Name != "" {
		return fmt.Sprintf("%s (%s)", payload.Agent.Name, payload.Agent.ID)
	}
	return payload.Agent.ID
}

func renderAlertEmailHTML(payload AlertPayload) string {
	rows := []struct {
		label string
		value string
	}{
		{label: "Event", value: payload.EventType},
		{label: "Incident", value: payload.Incident.ID},
		{label: "Status", value: payload.Incident.Status},
		{label: "Severity", value: payload.Incident.Severity},
		{label: "Monitor", value: alertPayloadMonitorLabel(payload)},
		{label: "Agent", value: alertPayloadAgentLabel(payload)},
		{label: "Latest event", value: payload.Incident.LatestEvent},
		{label: "Payload version", value: payload.Version},
	}

	var builder strings.Builder
	builder.WriteString(`<!doctype html><html><body style="font-family:Arial,sans-serif;color:#111827;background:#ffffff;margin:0;padding:24px;">`)
	builder.WriteString(`<main style="max-width:640px;margin:0 auto;">`)
	builder.WriteString(`<h1 style="font-size:20px;line-height:1.3;margin:0 0 12px;">`)
	builder.WriteString(html.EscapeString(payload.Summary.Title))
	builder.WriteString(`</h1><p style="font-size:14px;line-height:1.5;margin:0 0 20px;">`)
	builder.WriteString(html.EscapeString(payload.Summary.Text))
	builder.WriteString(`</p><table role="presentation" style="border-collapse:collapse;width:100%;font-size:14px;">`)
	for _, row := range rows {
		builder.WriteString(`<tr><th align="left" style="border-top:1px solid #e5e7eb;padding:10px 12px 10px 0;color:#4b5563;font-weight:600;width:36%;">`)
		builder.WriteString(html.EscapeString(row.label))
		builder.WriteString(`</th><td style="border-top:1px solid #e5e7eb;padding:10px 0;color:#111827;">`)
		builder.WriteString(html.EscapeString(row.value))
		builder.WriteString(`</td></tr>`)
	}
	builder.WriteString(`</table></main></body></html>`)
	return builder.String()
}

func sanitizeEmailHeader(value string) string {
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	return strings.TrimSpace(value)
}
