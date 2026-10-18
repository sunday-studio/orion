package main

import (
	"fmt"
	"time"

	"orion/core/internal/db"

	"gorm.io/gorm"
)

type alertDetailStats struct{ channels, recipients, rules, attempts int }

// seedAlertDetails gives the Alerts screens realistic data: an unhealthy webhook with failed attempts,
// a healthy email channel with confirmed and pending recipients, an untested webhook, named rules,
// and attempt history (status codes and durations) on the seeded deliveries.
func seedAlertDetails(database *gorm.DB, now time.Time) (alertDetailStats, error) {
	stats := alertDetailStats{}
	events := db.EncodeAlertEvents(db.DefaultAlertEvents())
	channels := []db.AlertChannel{
		{ID: "seed-alert-channel-oncall", Name: "oncall webhook", Type: "webhook", Enabled: true, WebhookURL: "https://pager.example.com/orion", WebhookSigningSecret: "whsec_seed0000000000000000e04b", SubscribedEvents: events, HeadersJSON: `{"X-Team":"ops"}`, EmailDeliveryMode: "instant", CreatedAt: now.AddDate(0, 0, -30), UpdatedAt: now},
		{ID: "seed-alert-channel-ops-team", Name: "ops-team", Type: "email", Enabled: true, SubscribedEvents: events, HeadersJSON: `{}`, EmailDeliveryMode: "instant", EmailSubjectPrefix: "[prod]", CreatedAt: now.AddDate(0, 0, -30), UpdatedAt: now},
		{ID: "seed-alert-channel-staging", Name: "staging webhook", Type: "webhook", Enabled: true, WebhookURL: "https://staging.example.com/hooks/orion", SubscribedEvents: events, HeadersJSON: `{}`, EmailDeliveryMode: "instant", CreatedAt: now.AddDate(0, 0, -3), UpdatedAt: now},
	}
	if err := database.Create(&channels).Error; err != nil {
		return stats, err
	}
	stats.channels = len(channels)
	confirmed := now.Add(-48 * time.Hour)
	recipients := []db.AlertChannelRecipient{
		{ID: "seed-alert-recipient-ops", ChannelID: "seed-alert-channel-ops-team", Email: "ops@example.com", Status: "confirmed", ConfirmedAt: &confirmed, CreatedAt: now.AddDate(0, 0, -2), UpdatedAt: now},
		{ID: "seed-alert-recipient-oncall", ChannelID: "seed-alert-channel-ops-team", Email: "oncall@example.com", Status: "pending", CreatedAt: now.AddDate(0, 0, -1), UpdatedAt: now},
	}
	if err := database.Create(&recipients).Error; err != nil {
		return stats, err
	}
	stats.recipients = len(recipients)
	rules := []db.AlertRoute{
		{ID: "seed-alert-rule-critical", Name: "Critical down", Enabled: true, Priority: 1, EventTypes: `["incident_opened"]`, Severities: `["critical","high"]`, ChannelIDs: `["seed-alert-channel-ops-team","seed-alert-channel-oncall"]`, GroupingPolicy: "suppress", GroupingDelaySeconds: 300, CooldownSeconds: 600},
		{ID: "seed-alert-rule-disk", Name: "Disk high", Enabled: true, Priority: 2, EventTypes: `["incident_opened"]`, Severities: `["medium"]`, MonitorTypes: `["disk"]`, ChannelIDs: `["seed-alert-channel-ops-team"]`, GroupingPolicy: "suppress", GroupingDelaySeconds: 300},
		{ID: "seed-alert-rule-latency", Name: "Latency", Enabled: true, Priority: 3, EventTypes: `["incident_opened"]`, Severities: `["low","medium"]`, ChannelIDs: `["seed-alert-channel-ops-team"]`, GroupingPolicy: "suppress", GroupingDelaySeconds: 300},
	}
	for i := range rules {
		rules[i].CreatedAt, rules[i].UpdatedAt = now.AddDate(0, 0, -30), now
	}
	if err := database.Create(&rules).Error; err != nil {
		return stats, err
	}
	stats.rules = len(rules)
	labelRoutes := []db.AlertLabelRoute{
		{ID: "seed-label-route-1", Position: 1, Enabled: true, Matchers: `[{"key":"env","op":"=","value":"prod"},{"key":"severity","op":"=","value":"critical"}]`, ChannelIDs: `["seed-alert-channel-oncall"]`, ContinueMatching: true},
		{ID: "seed-label-route-2", Position: 2, Enabled: true, Matchers: `[{"key":"env","op":"=","value":"prod"}]`, ChannelIDs: `["seed-alert-channel-ops-team"]`},
		{ID: "seed-label-route-3", Position: 3, Enabled: true, Matchers: `[]`, ChannelIDs: `["seed-alert-channel-staging"]`},
	}
	for i := range labelRoutes {
		labelRoutes[i].CreatedAt, labelRoutes[i].UpdatedAt = now.AddDate(0, 0, -10), now
	}
	if err := database.Create(&labelRoutes).Error; err != nil {
		return stats, err
	}

	// Attach rules and the oncall channel to seeded deliveries, with attempt history.
	var deliveries []db.AlertDelivery
	if err := database.Where("id LIKE ?", "seed-alert-delivery-%").Order("created_at ASC").Find(&deliveries).Error; err != nil {
		return stats, err
	}
	for i, d := range deliveries {
		channel, route := "oncall webhook", "seed-alert-rule-critical"
		if i%2 == 1 {
			channel, route = "ops-team", "seed-alert-rule-latency"
		}
		typ := choose(channel == "ops-team", "email", "webhook")
		if err := database.Model(&db.AlertDelivery{}).Where("id = ?", d.ID).Updates(map[string]any{"channel": channel, "type": typ, "route_id": route}).Error; err != nil {
			return stats, err
		}
		n := 1
		status, code := 200, 200
		if d.Status == "failed" {
			n, status = 3, 502
		}
		if d.Status == "pending" || d.Status == "suppressed" || d.Status == "cooldown" {
			continue
		}
		if typ == "email" {
			code = 0
		} else {
			code = status
		}
		for a := 1; a <= n; a++ {
			start := d.CreatedAt.Add(time.Duration((a-1)*2) * time.Minute)
			attemptStatus := choose(d.Status == "failed", "failed", "sent")
			errText := choose(d.Status == "failed", fmt.Sprintf("webhook returned status %d", code), "")
			if err := database.Create(&db.AlertDeliveryAttempt{ID: fmt.Sprintf("seed-alert-attempt-%s-%d", d.ID, a), AlertDeliveryID: d.ID, AttemptNumber: a, Status: attemptStatus, Stage: "http_response",
				Error: errText, HTTPStatus: code, StartedAt: start, CompletedAt: ptrTime(start.Add(time.Duration(300+a*1900) * time.Millisecond)), CreatedAt: start, UpdatedAt: start}).Error; err != nil {
				return stats, err
			}
			stats.attempts++
		}
		if d.Status == "failed" {
			database.Model(&db.AlertDelivery{}).Where("id = ?", d.ID).Updates(map[string]any{"attempt_count": n, "max_attempts": 8, "error": "webhook returned status 502"})
		}
	}
	return stats, nil
}
