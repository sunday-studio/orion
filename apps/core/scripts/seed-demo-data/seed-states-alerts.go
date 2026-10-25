package main

import (
	"fmt"
	"time"

	"orion/core/internal/db"

	"gorm.io/gorm"
)

type attemptSpec struct {
	status string // sent | failed
	code   int
	err    string
	stage  string
	ms     int
}

type deliverySpec struct {
	key, incident, event, channel, typ, status, route string
	agoMin                                            int
	errText                                           string
	max                                               int
	nextInHours                                       int // pending / retrying: pushed this far out so the worker does not race the review
	attempts                                          []attemptSpec
}

var (
	ok200      = attemptSpec{"sent", 200, "", "http_response", 420}
	fail503    = attemptSpec{"failed", 503, "webhook returned status 503", "http_response", 1900}
	failTO     = attemptSpec{"failed", 0, "context deadline exceeded", "http_request", 10000}
	failRefuse = attemptSpec{"failed", 0, "dial tcp: connection refused", "http_request", 40}
	smtpFail   = attemptSpec{"failed", 0, "smtp: 421 service not available", "transport", 3100}
	smtpOK     = attemptSpec{"sent", 0, "", "transport", 880}
)

func repeatAttempt(a attemptSpec, n int) []attemptSpec {
	out := make([]attemptSpec, n)
	for i := range out {
		out[i] = a
	}
	return out
}

func seedShowcaseAlerts(database *gorm.DB, now time.Time) error {
	events := db.EncodeAlertEvents(db.DefaultAlertEvents())
	created := now.AddDate(0, 0, -40)
	channels := []db.AlertChannel{
		{ID: "seed-state-ch-pager", Name: "pager-prod webhook", Type: "webhook", Enabled: true, WebhookURL: "https://pager.example.com/hooks/orion-prod", WebhookSigningSecret: "whsec_seedpager00000000a91c", SubscribedEvents: events, HeadersJSON: `{"X-Team":"platform","X-Env":"prod"}`, EmailDeliveryMode: "instant", CreatedAt: created, UpdatedAt: now},
		{ID: "seed-state-ch-legacy", Name: "legacy chat bridge", Type: "webhook", Enabled: false, WebhookURL: "https://bridge.example.com/orion", SubscribedEvents: events, HeadersJSON: `{}`, EmailDeliveryMode: "instant", CreatedAt: created, UpdatedAt: now.AddDate(0, 0, -20)},
		{ID: "seed-state-ch-digest", Name: "eng-digest email", Type: "email", Enabled: true, SubscribedEvents: events, HeadersJSON: `{}`, EmailDeliveryMode: "digest_15m", EmailSubjectPrefix: "[digest]", CreatedAt: created, UpdatedAt: now},
		{ID: "seed-state-ch-finance", Name: "finance email", Type: "email", Enabled: true, SubscribedEvents: events, HeadersJSON: `{}`, EmailDeliveryMode: "instant", CreatedAt: now.AddDate(0, 0, -2), UpdatedAt: now},
		{ID: "seed-state-ch-status", Name: "status-bot webhook", Type: "webhook", Enabled: true, WebhookURL: "https://bot.example.com/hooks/status", WebhookSigningSecret: "whsec_seedstatus0000000f42d", SubscribedEvents: events, HeadersJSON: `{"Authorization-Hint":"bot"}`, EmailDeliveryMode: "instant", CreatedAt: created, UpdatedAt: now},
	}
	if err := database.Create(&channels).Error; err != nil {
		return err
	}
	confirmed := now.AddDate(0, 0, -9)
	recipients := []db.AlertChannelRecipient{
		{ID: "seed-state-rcpt-1", ChannelID: "seed-state-ch-digest", Email: "dev-leads@example.com", Status: "confirmed", ConfirmedAt: &confirmed, ConfirmationSentAt: &confirmed, CreatedAt: confirmed, UpdatedAt: confirmed},
		{ID: "seed-state-rcpt-2", ChannelID: "seed-state-ch-digest", Email: "sre@example.com", Status: "confirmed", ConfirmedAt: &confirmed, ConfirmationSentAt: &confirmed, CreatedAt: confirmed, UpdatedAt: confirmed},
		{ID: "seed-state-rcpt-3", ChannelID: "seed-state-ch-digest", Email: "new-hire@example.com", Status: "pending", ConfirmationSentAt: ptrTime(now.Add(-6 * time.Hour)), CreatedAt: now.Add(-6 * time.Hour), UpdatedAt: now.Add(-6 * time.Hour)},
		{ID: "seed-state-rcpt-4", ChannelID: "seed-state-ch-digest", Email: "left-company@example.com", Status: "unsubscribed", ConfirmedAt: &confirmed, CreatedAt: confirmed, UpdatedAt: now.AddDate(0, 0, -3)},
		{ID: "seed-state-rcpt-5", ChannelID: "seed-state-ch-finance", Email: "billing@example.com", Status: "pending", ConfirmationSentAt: ptrTime(now.Add(-2 * time.Hour)), CreatedAt: now.Add(-2 * time.Hour), UpdatedAt: now.Add(-2 * time.Hour)},
	}
	if err := database.Create(&recipients).Error; err != nil {
		return err
	}
	rules := []db.AlertRoute{
		{ID: "seed-state-rule-pager", Name: "Critical to pager", Enabled: true, Priority: 4, EventTypes: `["incident_opened","incident_resolved"]`, Severities: `["critical"]`, ChannelIDs: `["seed-state-ch-pager","seed-state-ch-digest"]`, GroupingPolicy: "suppress", GroupingDelaySeconds: 300},
		{ID: "seed-state-rule-heartbeat", Name: "Cron heartbeats to finance", Enabled: true, Priority: 5, EventTypes: `["incident_opened"]`, MonitorTypes: `["heartbeat"]`, ChannelIDs: `["seed-state-ch-finance"]`, GroupingPolicy: "suppress", GroupingDelaySeconds: 300, CooldownSeconds: 3600},
		{ID: "seed-state-rule-quiet", Name: "Quiet low severity", Enabled: true, Priority: 6, EventTypes: `["incident_opened"]`, Severities: `["low"]`, ChannelIDs: `[]`, Suppress: true, GroupingPolicy: "suppress", GroupingDelaySeconds: 300},
		{ID: "seed-state-rule-off", Name: "Staging noise (disabled)", Enabled: false, Priority: 7, EventTypes: `["incident_opened"]`, Severities: `["low","medium"]`, ChannelIDs: `["seed-state-ch-legacy"]`, GroupingPolicy: "suppress", GroupingDelaySeconds: 300, CooldownSeconds: 86400},
	}
	for i := range rules {
		rules[i].CreatedAt, rules[i].UpdatedAt = created, now
	}
	if err := database.Create(&rules).Error; err != nil {
		return err
	}
	routes := []db.AlertLabelRoute{
		{ID: "seed-label-route-4", Position: 4, Name: "Non-prod to digest", Enabled: false, Matchers: `[{"key":"env","op":"!=","value":"prod"}]`, ChannelIDs: `["seed-state-ch-digest"]`, CreatedAt: now.AddDate(0, 0, -1), UpdatedAt: now},
		{ID: "seed-label-route-5", Position: 5, Name: "Storage to finance", Enabled: true, Matchers: `[{"key":"role","op":"=","value":"storage"}]`, ChannelIDs: `["seed-state-ch-finance","seed-state-ch-status"]`, ContinueMatching: true, CreatedAt: now.AddDate(0, 0, -1), UpdatedAt: now},
	}
	if err := database.Create(&routes).Error; err != nil {
		return err
	}
	// gorm skips a false bool on create when the column defaults to true, so disable these explicitly.
	if err := database.Model(&db.AlertRoute{}).Where("id = ?", "seed-state-rule-off").Update("enabled", false).Error; err != nil {
		return err
	}
	if err := database.Model(&db.AlertLabelRoute{}).Where("id = ?", "seed-label-route-4").Update("enabled", false).Error; err != nil {
		return err
	}
	if err := seedShowcaseDeliveries(database, now); err != nil {
		return err
	}
	// Nothing seeded may be dispatched by the worker the moment it starts: park every undelivered row.
	return database.Exec(`UPDATE alert_deliveries SET next_attempt_at = ? WHERE id LIKE 'seed-%' AND status IN ('pending','failed') AND attempt_count < max_attempts AND (next_attempt_at IS NULL OR next_attempt_at < ?)`,
		now.Add(36*time.Hour), now).Error
}

func deliverySpecs() []deliverySpec {
	return []deliverySpec{
		{"sent-1", "db-critical", "incident_opened", "pager-prod webhook", "webhook", "sent", "seed-state-rule-pager", 185, "", 8, 0, []attemptSpec{ok200}},
		{"sent-retried", "checkout-high", "incident_opened", "pager-prod webhook", "webhook", "sent", "seed-state-rule-pager", 26, "", 8, 0, []attemptSpec{fail503, fail503, ok200}},
		{"failed-final", "db-critical", "incident_opened", "legacy chat bridge", "webhook", "failed", "", 180, "context deadline exceeded", 8, 0, repeatAttempt(failTO, 8)},
		{"failed-refused", "checkout-high", "incident_opened", "pager-prod webhook", "webhook", "failed", "seed-state-rule-pager", 25, "dial tcp: connection refused", 3, 0, repeatAttempt(failRefuse, 3)},
		{"retrying", "auth-reopened", "incident_opened", "pager-prod webhook", "webhook", "failed", "seed-state-rule-pager", 40, "webhook returned status 503", 8, 36, []attemptSpec{fail503, fail503}},
		{"pending-new", "declared-open", "incident_opened", "status-bot webhook", "webhook", "pending", "", 2, "", 8, 36, nil},
		{"pending-digest", "memory-ack", "incident_opened", "eng-digest email", "email", "pending", "seed-state-rule-pager", 5, "", 8, 12, nil},
		{"email-sent", "declared-open", "incident_opened", "eng-digest email", "email", "sent", "seed-state-rule-pager", 41, "", 8, 0, []attemptSpec{smtpOK}},
		{"email-retried", "hb-low", "incident_opened", "eng-digest email", "email", "sent", "", 129, "", 8, 0, []attemptSpec{smtpFail, smtpOK}},
		{"email-failed", "resolved-manual", "incident_resolved", "eng-digest email", "email", "failed", "", 5700, "smtp: 421 service not available", 8, 0, repeatAttempt(smtpFail, 8)},
		{"suppressed-norecipients", "hb-low", "incident_opened", "finance email", "email", "suppressed", "seed-state-rule-heartbeat", 128, "no confirmed email recipients", 8, 0, nil},
		{"suppressed-removed", "resolved-removed", "incident_opened", "pager-prod webhook", "webhook", "suppressed", "", 29000, "server removed", 8, 0, nil},
		{"cooldown", "checkout-high", "incident_opened", "pager-prod webhook", "webhook", "cooldown", "seed-state-rule-pager", 10, "alert cooldown active", 8, 0, nil},
		{"test-ok", "", "test", "status-bot webhook", "webhook", "sent", "", 300, "", 1, 0, []attemptSpec{ok200}},
		{"test-failed", "", "test", "legacy chat bridge", "webhook", "failed", "", 1500, "webhook returned status 404", 1, 0, []attemptSpec{{"failed", 404, "webhook returned status 404", "http_response", 230}}},
		{"resolved-sent", "resolved-recovered", "incident_resolved", "pager-prod webhook", "webhook", "sent", "seed-state-rule-pager", 2855, "", 8, 0, []attemptSpec{ok200}},
	}
}

func seedShowcaseDeliveries(database *gorm.DB, now time.Time) error {
	for _, d := range deliverySpecs() {
		created := now.Add(-time.Duration(d.agoMin) * time.Minute)
		incident := ""
		if d.incident != "" {
			incident = showcaseIncidentID(d.incident)
		}
		row := db.AlertDelivery{ID: "seed-state-delivery-" + d.key, IncidentID: incident, RouteID: d.route, EventType: d.event, Channel: d.channel, Type: d.typ, Status: d.status, Error: d.errText,
			AttemptCount: len(d.attempts), MaxAttempts: d.max, CreatedAt: created, UpdatedAt: created.Add(time.Minute)}
		if d.nextInHours > 0 {
			row.NextAttemptAt = ptrTime(now.Add(time.Duration(d.nextInHours) * time.Hour))
		}
		if n := len(d.attempts); n > 0 {
			row.LastAttemptAt = ptrTime(created.Add(time.Duration(n) * 2 * time.Minute))
		}
		if err := database.Create(&row).Error; err != nil {
			return err
		}
		for i, a := range d.attempts {
			start := created.Add(time.Duration(i) * 2 * time.Minute)
			if err := database.Create(&db.AlertDeliveryAttempt{ID: fmt.Sprintf("seed-state-attempt-%s-%d", d.key, i+1), AlertDeliveryID: row.ID, AttemptNumber: i + 1, Status: a.status, Stage: a.stage,
				Error: a.err, HTTPStatus: a.code, ResponseSnippet: snippetFor(a.code), StartedAt: start, CompletedAt: ptrTime(start.Add(time.Duration(a.ms) * time.Millisecond)), CreatedAt: start, UpdatedAt: start}).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func snippetFor(code int) string {
	switch code {
	case 503:
		return `{"error":"upstream unavailable","retry_after":30}`
	case 404:
		return `{"error":"no such hook"}`
	case 200:
		return `{"ok":true}`
	}
	return ""
}
