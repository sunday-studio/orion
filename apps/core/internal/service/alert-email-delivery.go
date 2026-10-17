package service

import (
	"errors"
	"fmt"
	"orion/core/internal/db"
	"strings"
	"time"
)

// deliverEmailPayload mails every confirmed recipient once. Recipients already
// recorded for this delivery are skipped, so a retry after a partial SMTP
// failure only resends to the ones that missed it.
func (s *AlertService) deliverEmailPayload(channel db.AlertChannel, deliveryID string, payload AlertPayload) error {
	if err := AlertEmailConfigured(s.cfg); err != nil {
		return newAlertDeliveryError("configure", err)
	}
	var recipients []db.AlertChannelRecipient
	if err := s.db.Where("channel_id = ? AND status = ?", channel.ID, "confirmed").Order("email ASC").Find(&recipients).Error; err != nil {
		return newAlertDeliveryError("channel_lookup", err)
	}
	if len(recipients) == 0 {
		return newAlertDeliveryError("configure", errors.New("no confirmed email recipients"))
	}
	var failures []string
	for _, recipient := range recipients {
		var already int64
		s.db.Model(&db.AlertDeliveryRecipient{}).Where("delivery_id = ? AND recipient_id = ?", deliveryID, recipient.ID).Count(&already)
		if already > 0 {
			continue
		}
		now := time.Now().UTC()
		unsubscribe := AlertEmailLink(s.cfg, "unsubscribe", SignAlertEmailToken(s.cfg, AlertEmailPurposeUnsubscribe, recipient.ID, 0, now))
		message := AlertEmailMessage{
			To:      recipient.Email,
			Subject: alertEmailSubject(payload),
			Text:    alertEmailBody(payload, unsubscribe),
			Headers: map[string]string{
				"List-Unsubscribe":      "<" + unsubscribe + ">",
				"List-Unsubscribe-Post": "List-Unsubscribe=One-Click",
			},
		}
		if err := SendAlertEmail(s.cfg, message); err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", recipient.Email, err))
			continue
		}
		s.db.Create(&db.AlertDeliveryRecipient{DeliveryID: deliveryID, RecipientID: recipient.ID, SentAt: now})
	}
	if len(failures) > 0 {
		return newAlertDeliveryError("smtp", errors.New(strings.Join(failures, "; ")))
	}
	return nil
}

func alertEmailSubject(p AlertPayload) string {
	prefix := "[Orion]"
	switch p.EventType {
	case "incident_opened":
		prefix = "[Orion] ALERT"
	case "incident_resolved":
		prefix = "[Orion] Resolved"
	case "test":
		prefix = "[Orion] Test"
	}
	return prefix + ": " + p.Incident.Title
}

func alertEmailBody(p AlertPayload, unsubscribeURL string) string {
	var b strings.Builder
	title := p.Summary.Title
	if title == "" {
		title = p.Incident.Title
	}
	fmt.Fprintf(&b, "%s\n\n", title)
	if p.Summary.Text != "" {
		fmt.Fprintf(&b, "%s\n\n", p.Summary.Text)
	}
	fmt.Fprintf(&b, "Status: %s\nSeverity: %s\nIncident: %s\n", p.Incident.Status, p.Incident.Severity, p.Incident.ID)
	if p.Incident.LatestEvent != "" {
		fmt.Fprintf(&b, "Latest event: %s\n", p.Incident.LatestEvent)
	}
	fmt.Fprintf(&b, "Opened: %s\n", p.Incident.OpenedAt.UTC().Format(time.RFC1123))
	fmt.Fprintf(&b, "\n--\nYou receive this because you confirmed alert emails from Orion.\nUnsubscribe: %s\n", unsubscribeURL)
	return b.String()
}
