package service

import (
	"crypto/tls"
	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/testkit"
	"orion/core/internal/utils"
	"regexp"
	"strings"
	"testing"
	"time"
)

var emailLinkPattern = regexp.MustCompile(`https://orion\.test/alerts/email/(confirm|unsubscribe)/([A-Za-z0-9_.\-]+)`)

func emailTestSetup(t *testing.T, mode string) (*AlertService, *testkit.SMTPServer, db.AlertChannel) {
	t.Helper()
	smtpServer := testkit.NewSMTPServer(t, mode)
	prev := alertSMTPTLSConfig
	alertSMTPTLSConfig = func(string) *tls.Config { return smtpServer.TLSConfig }
	t.Cleanup(func() { alertSMTPTLSConfig = prev })
	cfg := &config.Config{AlertSMTPHost: smtpServer.Host, AlertSMTPPort: smtpServer.Port, AlertSMTPFrom: "Orion <orion@example.com>",
		AlertSMTPTLS: mode, AlertSMTPUsername: "user", AlertSMTPPassword: "pw", PublicURL: "https://orion.test", AlertRecoveryNotifications: true, AlertEmailSecret: "test-secret"}
	database := setupAlertServiceDatabase(t)
	createTestIncident(t, database, "incident-1")
	createTestAlertChannel(t, database, db.AlertChannel{Name: "oncall-mail", Type: "email", Enabled: true})
	var channel db.AlertChannel
	database.Where("name = ?", "oncall-mail").First(&channel)
	return NewAlertService(database, utils.NewLogger(), cfg), smtpServer, channel
}

func lastLink(t *testing.T, msg testkit.SMTPMessage, kind string) string {
	t.Helper()
	for _, m := range emailLinkPattern.FindAllStringSubmatch(msg.Data, -1) {
		if m[1] == kind {
			return m[2]
		}
	}
	t.Fatalf("no %s link in message:\n%s", kind, msg.Data)
	return ""
}

func TestEmailChannelDoubleOptInAllModes(t *testing.T) {
	for _, mode := range []string{"none", "starttls", "tls"} {
		t.Run(mode, func(t *testing.T) {
			s, smtpServer, channel := emailTestSetup(t, mode)
			recipient, err := s.AddRecipient(channel.ID, "Alice@Example.com")
			if err != nil || recipient.Email != "alice@example.com" || recipient.Status != "pending" {
				t.Fatalf("AddRecipient = %+v, %v", recipient, err)
			}
			msgs := smtpServer.Messages()
			if len(msgs) != 1 || msgs[0].To != "alice@example.com" || msgs[0].Auth != "|user|pw" {
				t.Fatalf("confirmation mail = %+v", msgs)
			}
			confirmToken := lastLink(t, msgs[0], "confirm")

			// Unconfirmed recipient gets no alert.
			if err := queueAndDispatch(s, "incident-1", "incident_opened"); err != nil {
				t.Fatal(err)
			}
			if len(smtpServer.Messages()) != 1 {
				t.Fatalf("alert mailed to unconfirmed recipient: %d messages", len(smtpServer.Messages()))
			}
			var delivery db.AlertDelivery
			s.db.Where("channel = ?", "oncall-mail").Order("created_at DESC").First(&delivery)
			if delivery.Status != "suppressed" {
				t.Fatalf("delivery status = %s, want suppressed", delivery.Status)
			}

			// An unsubscribe token cannot confirm; the confirm token can.
			if err := s.ConfirmRecipient("garbage"); err == nil {
				t.Fatal("garbage token confirmed")
			}
			if err := s.ConfirmRecipient(confirmToken); err != nil {
				t.Fatalf("ConfirmRecipient: %v", err)
			}
			newIncidentForOtherNode(t, s, "incident-2", "node-2") // separate alert group
			if err := s.QueueIncidentNotifications("incident-2", "incident_opened"); err != nil {
				t.Fatal(err)
			}
			if _, err := s.ProcessDueDeliveries(10); err != nil {
				t.Fatal(err)
			}
			msgs = smtpServer.Messages()
			if len(msgs) != 2 {

				t.Fatalf("messages after confirm = %d, want 2", len(msgs))
			}
			alert := msgs[1]
			if !strings.Contains(alert.Data, "List-Unsubscribe: <https://orion.test/alerts/email/unsubscribe/") ||
				!strings.Contains(alert.Data, "List-Unsubscribe-Post: List-Unsubscribe=One-Click") ||
				!strings.Contains(alert.Data, "Unsubscribe: https://orion.test/alerts/email/unsubscribe/") {
				t.Fatalf("alert mail lacks unsubscribe:\n%s", alert.Data)
			}

			// Unsubscribe stops further mail.
			if err := s.UnsubscribeRecipient(lastLink(t, alert, "unsubscribe")); err != nil {
				t.Fatal(err)
			}
			newIncidentForOtherNode(t, s, "incident-3", "node-3")
			if err := s.QueueIncidentNotifications("incident-3", "incident_opened"); err != nil {
				t.Fatal(err)
			}
			s.db.Model(&db.AlertDelivery{}).Where("status = ?", "pending").Update("next_attempt_at", nil)
			if _, err := s.ProcessDueDeliveries(10); err != nil {
				t.Fatal(err)
			}
			if len(smtpServer.Messages()) != 2 {
				t.Fatalf("mail sent after unsubscribe: %d", len(smtpServer.Messages()))
			}
		})
	}
}

func TestEmailRetryDoesNotDuplicateToDeliveredRecipients(t *testing.T) {
	s, smtpServer, channel := emailTestSetup(t, "none")
	for _, address := range []string{"a@example.com", "b@example.com"} {
		if _, err := s.AddRecipient(channel.ID, address); err != nil {
			t.Fatal(err)
		}
	}
	s.db.Model(&db.AlertChannelRecipient{}).Where("channel_id = ?", channel.ID).Update("status", "confirmed")
	before := len(smtpServer.Messages())
	smtpServer.RejectRecipient("b@example.com")
	if err := queueAndDispatch(s, "incident-1", "incident_opened"); err != nil {
		t.Fatal(err)
	}
	var delivery db.AlertDelivery
	s.db.Where("channel = ?", "oncall-mail").First(&delivery)
	if delivery.Status != "failed" || !strings.Contains(delivery.Error, "b@example.com") {
		t.Fatalf("delivery = %s / %s, want failed naming b@example.com", delivery.Status, delivery.Error)
	}
	s.db.Model(&db.AlertDelivery{}).Where("id = ?", delivery.ID).Update("next_attempt_at", time.Now().UTC().Add(-time.Second))
	smtpServer.ClearRejects()
	if _, err := s.ProcessDueDeliveries(10); err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, m := range smtpServer.Messages()[before:] {
		counts[m.To]++
	}
	if counts["a@example.com"] != 1 || counts["b@example.com"] != 1 {
		t.Fatalf("per-recipient alert counts = %v, want exactly one each", counts)
	}
	s.db.Where("id = ?", delivery.ID).First(&delivery)
	if delivery.Status != "sent" {
		t.Fatalf("delivery status after retry = %s", delivery.Status)
	}
}

func TestEmailTokensExpireAndBindPurpose(t *testing.T) {
	cfg := &config.Config{AlertEmailSecret: "k"}
	now := time.Now()
	token := SignAlertEmailToken(cfg, AlertEmailPurposeConfirm, "r1", time.Hour, now)
	if id, err := VerifyAlertEmailToken(cfg, AlertEmailPurposeConfirm, token, now.Add(time.Minute)); err != nil || id != "r1" {
		t.Fatalf("valid token: %q %v", id, err)
	}
	if _, err := VerifyAlertEmailToken(cfg, AlertEmailPurposeConfirm, token, now.Add(2*time.Hour)); err == nil {
		t.Fatal("expired token accepted")
	}
	if _, err := VerifyAlertEmailToken(cfg, AlertEmailPurposeUnsubscribe, token, now); err == nil {
		t.Fatal("confirm token accepted as unsubscribe")
	}
	if _, err := VerifyAlertEmailToken(&config.Config{AlertEmailSecret: "other"}, AlertEmailPurposeConfirm, token, now); err == nil {
		t.Fatal("token verified under a different secret")
	}
	forever := SignAlertEmailToken(cfg, AlertEmailPurposeUnsubscribe, "r1", 0, now)
	if _, err := VerifyAlertEmailToken(cfg, AlertEmailPurposeUnsubscribe, forever, now.Add(5*365*24*time.Hour)); err != nil {
		t.Fatalf("unsubscribe token should not expire: %v", err)
	}
}

func TestEmailChannelRejectsBadAddressesAndUnconfiguredSMTP(t *testing.T) {
	s, _, channel := emailTestSetup(t, "none")
	for _, bad := range []string{"", "no-at", "Bob <bob@example.com>", "a@b.c\r\nBcc: x@y.z"} {
		if _, err := s.AddRecipient(channel.ID, bad); err == nil {
			t.Errorf("AddRecipient(%q) accepted", bad)
		}
	}
	if err := AlertEmailConfigured(&config.Config{}); err == nil {
		t.Fatal("empty config reported as configured")
	}
}

func newIncidentForOtherNode(t *testing.T, s *AlertService, id, node string) {
	t.Helper()
	createTestIncident(t, s.db, id)
	s.db.Model(&db.Incident{}).Where("id = ?", id).Update("node_id", node)
}
