package api

import (
	"net/http"
	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/testkit"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestEmailChannelAPIFlow(t *testing.T) {
	smtpServer := testkit.NewSMTPServer(t, "none")
	unconfigured := setupTestServer(t)
	resp := performJSONRequest(t, unconfigured, http.MethodPost, "/v1/alerts/channels", gin.H{"name": "mail", "type": "email"}, "")
	if resp.Code != http.StatusBadRequest || !strings.Contains(resp.Body.String(), "not configured") {
		t.Fatalf("create without SMTP = %d %s", resp.Code, resp.Body.String())
	}

	server := setupTestServerWithConfig(t, &config.Config{AlertRecoveryNotifications: true, AlertSMTPHost: smtpServer.Host, AlertSMTPPort: smtpServer.Port,
		AlertSMTPFrom: "orion@example.com", AlertSMTPTLS: "none", PublicURL: "https://orion.test", AlertEmailSecret: "s"})
	resp = performJSONRequest(t, server, http.MethodPost, "/v1/alerts/channels", gin.H{"name": "mail", "type": "email"}, "")
	if resp.Code != http.StatusCreated {
		t.Fatalf("create email channel = %d %s", resp.Code, resp.Body.String())
	}
	var channel db.AlertChannel
	server.db.Where("name = ?", "mail").First(&channel)

	resp = performJSONRequest(t, server, http.MethodPost, "/v1/alerts/channels/"+channel.ID+"/recipients", gin.H{"email": "not an address"}, "")
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("bad address = %d", resp.Code)
	}
	resp = performJSONRequest(t, server, http.MethodPost, "/v1/alerts/channels/"+channel.ID+"/recipients", gin.H{"email": "dev@example.com"}, "")
	if resp.Code != http.StatusCreated || !strings.Contains(resp.Body.String(), `"status":"pending"`) {
		t.Fatalf("add recipient = %d %s", resp.Code, resp.Body.String())
	}
	msgs := smtpServer.Messages()
	if len(msgs) != 1 {
		t.Fatalf("confirmation mails = %d", len(msgs))
	}
	token := regexp.MustCompile(`/alerts/email/confirm/([A-Za-z0-9_.\-]+)`).FindStringSubmatch(msgs[0].Data)[1]

	// GET (link scanners) must not confirm.
	if r := performRequestWithHeaders(t, server, http.MethodGet, "/alerts/email/confirm/"+token, nil); r.Code != http.StatusOK || !strings.Contains(r.Body.String(), "<form") {
		t.Fatalf("GET confirm page = %d %s", r.Code, r.Body.String())
	}
	var recipient db.AlertChannelRecipient
	server.db.Where("channel_id = ?", channel.ID).First(&recipient)
	if recipient.Status != "pending" {
		t.Fatalf("GET changed status to %s", recipient.Status)
	}
	if r := performRequestWithHeaders(t, server, http.MethodPost, "/alerts/email/confirm/bogus", nil); r.Code != http.StatusBadRequest {
		t.Fatalf("bogus token = %d", r.Code)
	}
	if r := performRequestWithHeaders(t, server, http.MethodPost, "/alerts/email/confirm/"+token, nil); r.Code != http.StatusOK {
		t.Fatalf("POST confirm = %d", r.Code)
	}
	server.db.Where("channel_id = ?", channel.ID).First(&recipient)
	if recipient.Status != "confirmed" {
		t.Fatalf("status after POST = %s", recipient.Status)
	}
	resp = performJSONRequest(t, server, http.MethodGet, "/v1/alerts/channels", nil, "")
	if !strings.Contains(resp.Body.String(), `"confirmed_recipient_count":1`) {
		t.Fatalf("channel list lacks recipient counts: %s", resp.Body.String())
	}

	// Test mail goes to the confirmed recipient with an unsubscribe link; unsubscribing stops it.
	resp = performJSONRequest(t, server, http.MethodPost, "/v1/alerts/channels/"+channel.ID+"/test", nil, "")
	if resp.Code != http.StatusOK || len(smtpServer.Messages()) != 2 {
		t.Fatalf("test channel = %d, mails = %d", resp.Code, len(smtpServer.Messages()))
	}
	unsub := regexp.MustCompile(`/alerts/email/unsubscribe/([A-Za-z0-9_.\-]+)`).FindStringSubmatch(smtpServer.Messages()[1].Data)[1]
	if r := performRequestWithHeaders(t, server, http.MethodPost, "/alerts/email/unsubscribe/"+unsub, nil); r.Code != http.StatusOK {
		t.Fatalf("unsubscribe = %d", r.Code)
	}
	resp = performJSONRequest(t, server, http.MethodPost, "/v1/alerts/channels/"+channel.ID+"/test", nil, "")
	if len(smtpServer.Messages()) != 2 || !strings.Contains(resp.Body.String(), "failed") {
		t.Fatalf("mail sent after unsubscribe or test did not fail: mails=%d body=%s", len(smtpServer.Messages()), resp.Body.String())
	}

	resp = performJSONRequest(t, server, http.MethodDelete, "/v1/alerts/channels/"+channel.ID, nil, "")
	var left int64
	server.db.Model(&db.AlertChannelRecipient{}).Count(&left)
	if resp.Code != http.StatusOK || left != 0 {
		t.Fatalf("delete channel = %d, recipients left = %d", resp.Code, left)
	}
}
