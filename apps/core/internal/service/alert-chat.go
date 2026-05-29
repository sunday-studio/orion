package service

import (
	"encoding/json"
	"fmt"
)

type ChatDestination string

const (
	ChatDestinationSlack   ChatDestination = "slack"
	ChatDestinationDiscord ChatDestination = "discord"
)

type slackChatPayload struct {
	Text   string           `json:"text"`
	Blocks []slackChatBlock `json:"blocks"`
}

type slackChatBlock struct {
	Type     string                    `json:"type"`
	Text     *slackChatText            `json:"text,omitempty"`
	Elements []slackChatContextElement `json:"elements,omitempty"`
}

type slackChatText struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type slackChatContextElement struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type discordChatPayload struct {
	Content string             `json:"content"`
	Embeds  []discordChatEmbed `json:"embeds"`
}

type discordChatEmbed struct {
	Title       string             `json:"title"`
	Description string             `json:"description"`
	Fields      []discordChatField `json:"fields"`
	Footer      discordChatFooter  `json:"footer"`
}

type discordChatField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

type discordChatFooter struct {
	Text string `json:"text"`
}

func RenderChatAlert(destination ChatDestination, payload AlertPayload) ([]byte, string, error) {
	switch destination {
	case ChatDestinationSlack:
		body, err := json.Marshal(renderSlackChatAlert(payload))
		return body, "application/json", err
	case ChatDestinationDiscord:
		body, err := json.Marshal(renderDiscordChatAlert(payload))
		return body, "application/json", err
	default:
		return nil, "", fmt.Errorf("unsupported chat destination: %s", destination)
	}
}

func renderSlackChatAlert(payload AlertPayload) slackChatPayload {
	title := truncateAlertText(payload.Summary.Title, 150)
	text := truncateAlertText(payload.Summary.Text, 3000)
	context := truncateAlertText(fmt.Sprintf("event: %s | severity: %s | version: %s", payload.EventType, payload.Incident.Severity, payload.Version), 3000)
	return slackChatPayload{
		Text: truncateAlertText("Orion alert: "+payload.Summary.Title, 3000),
		Blocks: []slackChatBlock{
			{
				Type: "header",
				Text: &slackChatText{Type: "plain_text", Text: title},
			},
			{
				Type: "section",
				Text: &slackChatText{Type: "mrkdwn", Text: text},
			},
			{
				Type:     "context",
				Elements: []slackChatContextElement{{Type: "mrkdwn", Text: context}},
			},
		},
	}
}

func renderDiscordChatAlert(payload AlertPayload) discordChatPayload {
	fields := []discordChatField{
		{Name: "Event", Value: truncateAlertText(payload.EventType, 1024), Inline: true},
		{Name: "Severity", Value: truncateAlertText(payload.Incident.Severity, 1024), Inline: true},
		{Name: "Incident", Value: truncateAlertText(payload.Incident.ID, 1024), Inline: false},
	}
	if monitor := alertPayloadMonitorLabel(payload); monitor != "" {
		fields = append(fields, discordChatField{Name: "Monitor", Value: truncateAlertText(monitor, 1024), Inline: false})
	}
	if agent := alertPayloadAgentLabel(payload); agent != "" {
		fields = append(fields, discordChatField{Name: "Agent", Value: truncateAlertText(agent, 1024), Inline: false})
	}
	return discordChatPayload{
		Content: truncateAlertText("Orion alert: "+payload.Summary.Title, 2000),
		Embeds: []discordChatEmbed{{
			Title:       truncateAlertText(payload.Summary.Title, 256),
			Description: truncateAlertText(payload.Summary.Text, 4096),
			Fields:      fields,
			Footer:      discordChatFooter{Text: payload.Version},
		}},
	}
}

func truncateAlertText(value string, limit int) string {
	if limit <= 0 || len(value) <= limit {
		return value
	}
	if limit <= 3 {
		return value[:limit]
	}
	return value[:limit-3] + "..."
}
