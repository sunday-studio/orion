package api

import (
	"net/http"
	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"orion/core/internal/testkit"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func alertServer(t *testing.T) *Server {
	t.Helper()
	return setupTestServerWithConfig(t, &config.Config{AlertAllowPrivateWebhooks: true, AlertRecoveryNotifications: true, AlertCooldownSeconds: 0})
}

type channelEnvelope struct {
	Data struct {
		Channel AlertChannelResponse `json:"channel"`
		Secret  string               `json:"secret"`
	} `json:"data"`
}

func TestChannelHealthDetailSecretHeadersAndPayloadPreview(t *testing.T) {
	hook := testkit.NewWebhook(t)
	server := alertServer(t)
	resp := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/channels", gin.H{"name": "oncall webhook", "type": "webhook", "webhook_url": hook.URL(), "webhook_signing_secret": "whsec_abcdef1234e04b", "headers": gin.H{"X-Team": "ops"}}, "")
	var created channelEnvelope
	decodeResponse(t, resp, &created)
	if resp.Code != http.StatusCreated || created.Data.Channel.Health != "untested" || created.Data.Channel.LastDeliveryText != "Never" || len(created.Data.Channel.HeaderNames) != 1 || created.Data.Channel.HeaderNames[0] != "X-Team" ||
		!strings.Contains(created.Data.Channel.SigningSecretMask, "••••") || strings.Contains(resp.Body.String(), "ops\"") || strings.Contains(resp.Body.String(), "abcdef1234e04b") {
		t.Fatalf("created = %d %+v / %s", resp.Code, created.Data.Channel, resp.Body.String())
	}
	id := created.Data.Channel.ID
	for name, body := range map[string]gin.H{
		"forbidden header": {"headers": gin.H{"Host": "evil"}},
		"orion header":     {"headers": gin.H{"X-Orion-Signature": "x"}},
		"newline value":    {"headers": gin.H{"X-A": "a\nb"}},
		"bad mode":         {"email_delivery_mode": "hourly"},
	} {
		if r := performJSONRequest(t, server, http.MethodPatch, "/v1/alerts/channels/"+id, body, ""); r.Code != http.StatusBadRequest {
			t.Errorf("%s = %d", name, r.Code)
		}
	}

	// Saved test: the custom header goes out, 502 shows as a failed attempt with status and body.
	hook.SetStatus(http.StatusBadGateway, `{"error":"upstream"}`)
	r := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/channels/"+id+"/test", nil, "")
	var tested struct {
		Data struct {
			Delivery AlertDeliveryResponse `json:"delivery"`
			Result   AlertTestResult       `json:"result"`
		} `json:"data"`
	}
	decodeResponse(t, r, &tested)
	res := tested.Data.Result
	if r.Code != http.StatusOK || res.Passed || res.Status != "failed" || res.StatusCode != 502 || !strings.Contains(res.Body, "upstream") || res.Actual.Title != "502 Bad Gateway" || res.Expected.Title != "2xx" || res.DurationMs == nil {
		t.Fatalf("failed test result = %d %+v", r.Code, res)
	}
	if hook.LastHeader("X-Team") != "ops" {
		t.Fatalf("custom header not sent: %q", hook.LastHeader("X-Team"))
	}
	hook.SetStatus(http.StatusBadGateway, "")
	performJSONRequest(t, server, http.MethodPost, "/v1/alerts/channels/"+id+"/test", nil, "")
	var detail struct {
		Data AlertChannelDetailResponse `json:"data"`
	}
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/alerts/channels/"+id, nil, ""), &detail)
	ch := detail.Data.Channel
	if ch.Health != "failing" || ch.ConsecutiveFailures != 2 || ch.LastDeliveryText != "2 failed, 502" || detail.Data.Alert == "" || len(detail.Data.RecentDeliveries) != 2 || detail.Data.RecentTotal != 2 || detail.Data.PayloadPreview.EventType != "test" {
		t.Fatalf("detail = %+v", detail.Data)
	}

	// A passing test heals the channel; the summary text switches to Sent.
	hook.SetStatus(http.StatusOK, `{"received":true}`)
	decodeResponse(t, performJSONRequest(t, server, http.MethodPost, "/v1/alerts/channels/"+id+"/test", nil, ""), &tested)
	if !tested.Data.Result.Passed || tested.Data.Result.Actual.Title != "200 OK" || !strings.Contains(tested.Data.Result.Body, "received") {
		t.Fatalf("passing test = %+v", tested.Data.Result)
	}
	decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/alerts/channels/"+id, nil, ""), &detail)
	if detail.Data.Channel.Health != "healthy" || detail.Data.Channel.LastDeliveryText != "Sent" {
		t.Fatalf("healed = %+v", detail.Data.Channel)
	}

	// Regenerate: new secret shown once, old one gone.
	var regen channelEnvelope
	rr := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/channels/"+id+"/secret/regenerate", nil, "")
	decodeResponse(t, rr, &regen)
	if rr.Code != http.StatusOK || !strings.HasPrefix(regen.Data.Secret, "whsec_") || len(regen.Data.Secret) < 20 || strings.Contains(rr.Body.String(), "abcdef1234e04b") || regen.Data.Channel.SigningSecretMask == created.Data.Channel.SigningSecretMask {
		t.Fatalf("regenerate = %d %s", rr.Code, rr.Body.String())
	}
	var stored db.AlertChannel
	server.db.First(&stored, "id = ?", id)
	if stored.WebhookSigningSecret != regen.Data.Secret {
		t.Fatal("new secret was not stored")
	}
	if r := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/channels/nope/secret/regenerate", nil, ""); r.Code != http.StatusNotFound {
		t.Fatalf("unknown channel = %d", r.Code)
	}
	if r := performJSONRequest(t, server, http.MethodGet, "/v1/alerts/payload-preview", nil, ""); r.Code != http.StatusOK || !strings.Contains(r.Body.String(), "X-Orion-Signature") {
		t.Fatalf("payload preview = %d", r.Code)
	}
}

func TestUnsavedWebhookTestRunStoresNothing(t *testing.T) {
	hook := testkit.NewWebhook(t)
	server := alertServer(t)
	var out struct {
		Data AlertTestResult `json:"data"`
	}
	r := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/channels/test-run", gin.H{"type": "webhook", "webhook_url": hook.URL()}, "")
	decodeResponse(t, r, &out)
	if r.Code != http.StatusOK || !out.Data.Passed || out.Data.StatusCode != http.StatusNoContent && out.Data.StatusCode != http.StatusOK {
		t.Fatalf("passing unsaved = %d %+v", r.Code, out.Data)
	}
	hook.SetStatus(http.StatusServiceUnavailable, "")
	decodeResponse(t, performJSONRequest(t, server, http.MethodPost, "/v1/alerts/channels/test-run", gin.H{"type": "webhook", "webhook_url": hook.URL()}, ""), &out)
	if out.Data.Passed || out.Data.StatusCode != 503 || out.Data.Hint == "" {
		t.Fatalf("failing unsaved = %+v", out.Data)
	}
	if r := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/channels/test-run", gin.H{"type": "webhook", "webhook_url": "http://169.254.169.254/latest"}, ""); r.Code != http.StatusBadRequest {
		t.Fatalf("metadata url = %d", r.Code)
	}
	var channels, deliveries int64
	server.db.Model(&db.AlertChannel{}).Count(&channels)
	server.db.Model(&db.AlertDelivery{}).Count(&deliveries)
	if channels != 0 || deliveries != 0 {
		t.Fatalf("unsaved test wrote channels=%d deliveries=%d", channels, deliveries)
	}
}

func TestDeliveryListFiltersAndDetail(t *testing.T) {
	hook := testkit.NewWebhook(t)
	server := alertServer(t)
	node := registerTestNode(t, server)
	now := time.Now().UTC()
	server.db.Create(&db.Incident{ID: "inc1", Number: 231, Status: "open", Severity: "critical", Title: "api.example.com unreachable", NodeID: node.Data.NodeID, OpenedAt: now, LastEventAt: now, Source: "monitor"})
	server.db.Create(&db.AlertChannel{ID: "ch1", Name: "oncall webhook", Type: "webhook", Enabled: true, WebhookURL: hook.URL(), SubscribedEvents: db.EncodeAlertEvents(db.DefaultAlertEvents())})
	server.db.Create(&db.AlertChannel{ID: "ch2", Name: "ops-team", Type: "email", Enabled: true, SubscribedEvents: db.EncodeAlertEvents(db.DefaultAlertEvents())})
	server.db.Create(&db.AlertRoute{ID: "rule1", Name: "Critical down", Enabled: true, Priority: 1, ChannelIDs: `["ch1"]`})
	mk := func(id, status, channel, typ string, age time.Duration, route string) {
		server.db.Create(&db.AlertDelivery{ID: id, IncidentID: "inc1", RouteID: route, EventType: "incident_opened", Channel: channel, Type: typ, Status: status, MaxAttempts: 8, CreatedAt: now.Add(-age)})
	}
	mk("d-fail", "failed", "oncall webhook", "webhook", time.Minute, "rule1")
	mk("d-sent", "sent", "ops-team", "email", 2*time.Minute, "")
	mk("d-old", "sent", "ops-team", "email", 30*time.Hour, "")
	mk("d-cool", "cooldown", "ops-team", "email", 3*time.Minute, "")
	for i, n := range []int{1, 2, 3} {
		server.db.Create(&db.AlertDeliveryAttempt{ID: "a" + itoa(n), AlertDeliveryID: "d-fail", AttemptNumber: n, Status: "failed", Stage: "http_response", Error: "webhook returned status 502", HTTPStatus: 502,
			StartedAt: now.Add(-time.Duration(10-i) * time.Minute), CompletedAt: ptrTime(now.Add(-time.Duration(10-i)*time.Minute + 5*time.Second))})
	}

	var list struct {
		Data struct {
			Deliveries []AlertDeliveryResponse `json:"deliveries"`
			Count      int64                   `json:"count"`
			Total      int64                   `json:"total"`
		} `json:"data"`
	}
	get := func(q string) {
		list.Data.Deliveries = nil
		decodeResponse(t, performJSONRequest(t, server, http.MethodGet, "/v1/alerts/deliveries"+q, nil, ""), &list)
	}
	get("?status=failed,sent&since=24h")
	if list.Data.Count != 2 || list.Data.Deliveries[0].ID != "d-fail" {
		t.Fatalf("filtered = %+v", list.Data)
	}
	row := list.Data.Deliveries[0]
	if row.AlertTitle != "Critical down" || row.IncidentDisplayID != "INC-0231" || row.ChannelID != "ch1" || row.ResultText != "502" || row.DurationMs == nil || *row.DurationMs != 5000 {
		t.Fatalf("failed row = %+v", row)
	}
	if sent := list.Data.Deliveries[1]; sent.AlertTitle != "api.example.com unreachable" || sent.ResultText != "Sent" || sent.ChannelID != "ch2" {
		t.Fatalf("sent row = %+v", sent)
	}
	if get("?channel_id=ch2"); list.Data.Count != 3 {
		t.Fatalf("channel_id = %d", list.Data.Count)
	}
	if get("?q=critical%20down"); list.Data.Count != 1 {
		t.Fatalf("q by rule name = %d", list.Data.Count)
	}
	if r := performJSONRequest(t, server, http.MethodGet, "/v1/alerts/deliveries?since=soon", nil, ""); r.Code != http.StatusBadRequest {
		t.Fatalf("bad since = %d", r.Code)
	}

	var detail struct {
		Data AlertDeliveryDetailResponse `json:"data"`
	}
	r := performJSONRequest(t, server, http.MethodGet, "/v1/alerts/deliveries/d-fail", nil, "")
	decodeResponse(t, r, &detail)
	d := detail.Data
	if r.Code != http.StatusOK || len(d.Delivery.Attempts) != 3 || d.Delivery.Attempts[2].ResultText != "502 Bad Gateway" || d.Delivery.Attempts[2].DurationMs == nil || *d.Delivery.Attempts[2].DurationMs != 5000 ||
		d.Channel == nil || d.Channel.Name != "oncall webhook" || d.Payload == nil || d.Payload.Incident.ID != "inc1" || !d.Retryable || d.RetryNote == "" {
		t.Fatalf("detail = %d %+v", r.Code, d)
	}
	if r := performJSONRequest(t, server, http.MethodGet, "/v1/alerts/deliveries/nope", nil, ""); r.Code != http.StatusNotFound {
		t.Fatalf("unknown delivery = %d", r.Code)
	}
	_ = service.AlertPayload{}
}

func TestEmailChannelTestResultDigestAndSubjectPrefix(t *testing.T) {
	smtpServer := testkit.NewSMTPServer(t, "none")
	server := setupTestServerWithConfig(t, &config.Config{AlertSMTPHost: smtpServer.Host, AlertSMTPPort: smtpServer.Port, AlertSMTPFrom: "orion@example.com", AlertSMTPTLS: "none",
		PublicURL: "https://orion.test", AlertEmailSecret: "s", AlertRecoveryNotifications: true})
	var created channelEnvelope
	decodeResponse(t, performJSONRequest(t, server, http.MethodPost, "/v1/alerts/channels", gin.H{"name": "ops-team", "type": "email", "email_delivery_mode": "digest_15m", "email_subject_prefix": "[prod]"}, ""), &created)
	id := created.Data.Channel.ID
	if created.Data.Channel.EmailDeliveryMode != "digest_15m" || created.Data.Channel.EmailSubjectPrefix != "[prod]" {
		t.Fatalf("email channel = %+v", created.Data.Channel)
	}
	for _, addr := range []string{"ops@example.com", "oncall@example.com"} {
		if r := performJSONRequest(t, server, http.MethodPost, "/v1/alerts/channels/"+id+"/recipients", gin.H{"email": addr}, ""); r.Code != http.StatusCreated {
			t.Fatalf("add %s = %d %s", addr, r.Code, r.Body.String())
		}
	}
	server.db.Model(&db.AlertChannelRecipient{}).Where("email = ?", "ops@example.com").Update("status", "confirmed")
	before := len(smtpServer.Messages())
	var tested struct {
		Data struct {
			Result AlertTestResult `json:"result"`
		} `json:"data"`
	}
	decodeResponse(t, performJSONRequest(t, server, http.MethodPost, "/v1/alerts/channels/"+id+"/test", nil, ""), &tested)
	res := tested.Data.Result
	if !res.Passed || res.Recipients == nil || res.Recipients.Delivered != 1 || res.Recipients.Skipped != 1 {
		t.Fatalf("email test result = %+v", res)
	}
	msgs := smtpServer.Messages()
	if len(msgs) != before+1 || msgs[len(msgs)-1].To != "ops@example.com" || !strings.Contains(msgs[len(msgs)-1].Data, "Subject: [prod] [Orion] Test") {
		t.Fatalf("test mail = %+v", msgs[before:])
	}

	// Digest: a non-critical alert is held to the next 15 minute mark; a critical one is sent at once.
	node := registerTestNode(t, server)
	now := time.Now().UTC()
	for n, inc := range []struct{ id, sev string }{{"i-low", "low"}, {"i-crit", "critical"}} {
		server.db.Create(&db.Incident{ID: inc.id, Number: n + 1, Status: "open", Severity: inc.sev, Title: inc.id, NodeID: node.Data.NodeID, OpenedAt: now, LastEventAt: now, Source: "monitor"})
		server.db.Create(&db.AlertDelivery{ID: "d-" + inc.id, IncidentID: inc.id, EventType: "incident_opened", Channel: "ops-team", Type: "email", Status: "pending", MaxAttempts: 8, CreatedAt: now})
	}
	if _, err := service.NewAlertService(server.db, server.logger, server.cfg).ProcessDueDeliveries(10); err != nil {
		t.Fatal(err)
	}
	var low, crit db.AlertDelivery
	server.db.First(&low, "id = ?", "d-i-low")
	server.db.First(&crit, "id = ?", "d-i-crit")
	if low.Status != "pending" || low.NextAttemptAt == nil || !low.NextAttemptAt.After(now) || low.NextAttemptAt.Minute()%15 != 0 || crit.Status != "sent" {
		t.Fatalf("digest hold: low=%s next=%v, critical=%s next=%v attempts=%d err=%q", low.Status, low.NextAttemptAt, crit.Status, crit.NextAttemptAt, crit.AttemptCount, crit.Error)
	}
}
