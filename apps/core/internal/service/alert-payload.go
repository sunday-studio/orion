package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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
	Subject string
	Body    string
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
	return AlertEmailTemplate{
		Subject: sanitizeEmailHeader("Orion alert: " + payload.Summary.Title),
		Body: strings.Join([]string{
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
		}, "\n") + "\n",
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

func sanitizeEmailHeader(value string) string {
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	return strings.TrimSpace(value)
}
