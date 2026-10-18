package api

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"orion/core/internal/utils"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AlertDeliveryDetailResponse is the "Delivery to <channel>" screen.
type AlertDeliveryDetailResponse struct {
	Delivery AlertDeliveryResponse `json:"delivery"`
	Payload  *service.AlertPayload `json:"payload"`
	Channel  *AlertChannelSummary  `json:"channel"`
	// RetryNote repeats the screen's promise: a retry resends the same payload and creates no new record.
	RetryNote string `json:"retry_note"`
	Retryable bool   `json:"retryable"`
}

type AlertChannelSummary struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Enabled bool   `json:"enabled"`
}

// getAlertDelivery returns one delivery with attempts and payload.
// @Summary      Get alert delivery
// @Description  One delivery with its attempts (result, duration), the payload that is or was sent, the channel and the incident's INC id.
// @Tags         alerts
// @Produce      json
// @ID           getAlertDelivery
// @Param        id   path      string  true  "Delivery ID"
// @Success      200  {object}  utils.APIResponse{data=AlertDeliveryDetailResponse}
// @Failure      404  {object}  utils.APIResponse
// @Router       /v1/alerts/deliveries/{id} [get]
func (s *Server) getAlertDelivery(c *gin.Context) {
	var delivery db.AlertDelivery
	if err := s.db.Preload("Attempts", func(tx *gorm.DB) *gorm.DB { return tx.Order("attempt_number ASC") }).Where("id = ?", c.Param("id")).First(&delivery).Error; err != nil {
		utils.NotFound(c, "Alert delivery not found")
		return
	}
	rows := alertDeliveryResponses([]db.AlertDelivery{delivery})
	s.enrichDeliveries(rows)
	out := AlertDeliveryDetailResponse{Delivery: rows[0], RetryNote: "Retrying sends the same payload again. It does not create a new notification record.", Retryable: delivery.Status == "failed"}
	if payload, err := service.NewAlertService(s.db, s.logger, s.cfg).PayloadForDelivery(delivery); err == nil {
		out.Payload = &payload
	}
	var channel db.AlertChannel
	if s.db.Where("name = ?", delivery.Channel).Limit(1).Find(&channel).RowsAffected > 0 {
		out.Channel = &AlertChannelSummary{ID: channel.ID, Name: channel.Name, Type: channel.Type, Enabled: channel.Enabled}
	}
	utils.SuccessResponse(c, http.StatusOK, "Alert delivery retrieved", out)
}

type AlertRuleRef struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Summary string `json:"summary"`
}

type AlertChannelDetailResponse struct {
	Channel AlertChannelResponse `json:"channel"`
	// PayloadPreview is a sample of what a webhook receives.
	PayloadPreview    service.AlertPayload    `json:"payload_preview"`
	RecentDeliveries  []AlertDeliveryResponse `json:"recent_deliveries"`
	RecentTotal       int64                   `json:"recent_total"`
	RulesUsing        []AlertRuleRef          `json:"rules_using"`
	ConsecutiveFailed int                     `json:"consecutive_failures"`
	// Alert is set while the channel is failing: the banner text.
	Alert string `json:"alert,omitempty"`
}

// getAlertChannel returns one channel with health, recent deliveries and the rules using it.
// @Summary      Get alert channel
// @Description  Channel configuration with masked secrets, header names, recent deliveries (paged with limit and offset), and the rules that notify it.
// @Tags         alerts
// @Produce      json
// @ID           getAlertChannel
// @Param        id      path      string  true   "Channel ID"
// @Param        limit   query     int     false  "Recent deliveries to return" default(10)
// @Param        offset  query     int     false  "Recent deliveries to skip" default(0)
// @Success      200     {object}  utils.APIResponse{data=AlertChannelDetailResponse}
// @Failure      404     {object}  utils.APIResponse
// @Router       /v1/alerts/channels/{id} [get]
func (s *Server) getAlertChannel(c *gin.Context) {
	var channel db.AlertChannel
	if err := s.db.Where("id = ?", c.Param("id")).First(&channel).Error; err != nil {
		utils.NotFound(c, "Alert channel not found")
		return
	}
	limit := min(utils.QueryInt(c, "limit", 10), 100)
	offset := utils.QueryInt(c, "offset", 0)
	var deliveries []db.AlertDelivery
	s.db.Preload("Attempts", func(tx *gorm.DB) *gorm.DB { return tx.Order("attempt_number ASC") }).Where("channel = ?", channel.Name).Order("created_at DESC").Limit(limit).Offset(offset).Find(&deliveries)
	var total int64
	s.db.Model(&db.AlertDelivery{}).Where("channel = ?", channel.Name).Count(&total)
	rows := alertDeliveryResponses(deliveries)
	s.enrichDeliveries(rows)
	response := s.alertChannelResponse(channel)
	out := AlertChannelDetailResponse{Channel: response, PayloadPreview: service.NewAlertService(s.db, s.logger, s.cfg).SamplePayload(), RecentDeliveries: rows, RecentTotal: total,
		RulesUsing: s.rulesUsingChannel(channel.ID), ConsecutiveFailed: response.ConsecutiveFailures}
	if response.Health == "failing" {
		out.Alert = "Last " + itoa(response.ConsecutiveFailures) + " deliveries failed. Alerts for rules using only this channel may not be reaching anyone."
	}
	utils.SuccessResponse(c, http.StatusOK, "Alert channel retrieved", out)
}

func (s *Server) rulesUsingChannel(channelID string) []AlertRuleRef {
	var routes []db.AlertRoute
	s.db.Order("priority ASC, name ASC").Find(&routes)
	names := s.channelNamesByID()
	refs := make([]AlertRuleRef, 0)
	for _, route := range routes {
		for _, id := range decodeResponseList(route.ChannelIDs, nil) {
			if id == channelID {
				refs = append(refs, AlertRuleRef{ID: route.ID, Name: route.Name, Summary: ruleSummary(route, names)})
				break
			}
		}
	}
	return refs
}

func (s *Server) channelNamesByID() map[string]string {
	var channels []db.AlertChannel
	s.db.Select("id", "name").Find(&channels)
	out := make(map[string]string, len(channels))
	for _, c := range channels {
		out[c.ID] = c.Name
	}
	return out
}

// regenerateAlertChannelSecret issues a new webhook signing secret.
// @Summary      Regenerate a webhook signing secret
// @Description  Replaces the signing secret and returns the new one once. Receivers must be updated before the next delivery verifies.
// @Tags         alerts
// @Produce      json
// @ID           regenerateAlertChannelSecret
// @Param        id   path      string  true  "Channel ID"
// @Success      200  {object}  utils.APIResponse{data=object{secret=string,channel=AlertChannelResponse}}
// @Failure      400  {object}  utils.APIResponse
// @Failure      404  {object}  utils.APIResponse
// @Router       /v1/alerts/channels/{id}/secret/regenerate [post]
func (s *Server) regenerateAlertChannelSecret(c *gin.Context) {
	var channel db.AlertChannel
	if err := s.db.Where("id = ?", c.Param("id")).First(&channel).Error; err != nil {
		utils.NotFound(c, "Alert channel not found")
		return
	}
	if channel.Type != "webhook" {
		utils.BadRequest(c, "Only webhook channels have a signing secret")
		return
	}
	secret, err := newWebhookSecret()
	if err != nil {
		utils.InternalError(c, "Failed to generate a secret", err)
		return
	}
	if err := s.db.Model(&db.AlertChannel{}).Where("id = ?", channel.ID).Updates(map[string]any{"webhook_signing_secret": secret, "updated_at": time.Now().UTC()}).Error; err != nil {
		utils.InternalError(c, "Failed to save the secret", err)
		return
	}
	channel.WebhookSigningSecret = secret
	utils.SuccessResponse(c, http.StatusOK, "Signing secret regenerated", gin.H{"secret": secret, "channel": s.alertChannelResponse(channel)})
}

func newWebhookSecret() (string, error) {
	var raw [24]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return "whsec_" + hex.EncodeToString(raw[:]), nil
}

// getAlertPayloadPreview returns a sample webhook body.
// @Summary      Webhook payload preview
// @Tags         alerts
// @Produce      json
// @ID           getAlertPayloadPreview
// @Success      200  {object}  utils.APIResponse{data=object{payload=service.AlertPayload,signature_header=string}}
// @Router       /v1/alerts/payload-preview [get]
func (s *Server) getAlertPayloadPreview(c *gin.Context) {
	utils.SuccessResponse(c, http.StatusOK, "Payload preview", gin.H{"payload": service.NewAlertService(s.db, s.logger, s.cfg).SamplePayload(), "signature_header": "X-Orion-Signature"})
}

// AlertTestResult is the "Test sent" / "Test failed" dialog.
type AlertTestResult struct {
	Passed     bool                `json:"passed"`
	Status     string              `json:"status" enums:"passed,failed"`
	StatusCode int                 `json:"status_code,omitempty"`
	DurationMs *int64              `json:"duration_ms,omitempty"`
	Body       string              `json:"body,omitempty"`
	Expected   IncidentExpectation `json:"expected"`
	Actual     IncidentExpectation `json:"actual"`
	Hint       string              `json:"hint,omitempty"`
	// Recipients is set for email channels: confirmed addresses mailed vs addresses skipped as unconfirmed.
	Recipients *struct {
		Delivered int `json:"delivered"`
		Skipped   int `json:"skipped"`
	} `json:"recipients,omitempty"`
}

func webhookTestResult(res service.WebhookTestResult) AlertTestResult {
	out := AlertTestResult{Expected: IncidentExpectation{Title: "2xx", Detail: "within 10 s"}, Passed: res.Err == nil, StatusCode: res.Record.StatusCode, Body: res.Record.Snippet}
	ms := res.Duration.Milliseconds()
	out.DurationMs = &ms
	out.Status = map[bool]string{true: "passed", false: "failed"}[out.Passed]
	switch {
	case res.Record.StatusCode > 0:
		out.Actual = IncidentExpectation{Title: itoa(res.Record.StatusCode) + " " + http.StatusText(res.Record.StatusCode)}
	case res.Err != nil:
		out.Actual = IncidentExpectation{Title: shortError(res.Err.Error()), Detail: "no response"}
	}
	out.Hint = service.TestHint(res.Stage, res.Err)
	return out
}

type TestAlertChannelRequest struct {
	Type          string            `json:"type" binding:"required" enums:"webhook"`
	WebhookURL    string            `json:"webhook_url" binding:"required"`
	SigningSecret string            `json:"signing_secret,omitempty"`
	Headers       map[string]string `json:"headers,omitempty"`
}

// testRunAlertChannel tests a webhook that is not saved yet.
// @Summary      Test an unsaved webhook
// @Description  Posts a sample payload to the URL (same URL policy as saving) and returns the result. Nothing is stored. Limited to 20 runs per minute.
// @Tags         alerts
// @Accept       json
// @Produce      json
// @ID           testRunAlertChannel
// @Param        request  body      TestAlertChannelRequest  true  "Webhook to test"
// @Success      200      {object}  utils.APIResponse{data=AlertTestResult}
// @Failure      400      {object}  utils.APIResponse
// @Failure      429      {object}  utils.APIResponse
// @Router       /v1/alerts/channels/test-run [post]
func (s *Server) testRunAlertChannel(c *gin.Context) {
	if !s.testRunLimiter.Allow("test-run") {
		utils.ErrorResponse(c, http.StatusTooManyRequests, "Too many test runs; wait a minute", nil)
		return
	}
	var req TestAlertChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Type != "webhook" {
		utils.BadRequest(c, "type webhook and webhook_url are required")
		return
	}
	channel := db.AlertChannel{Type: "webhook", WebhookURL: strings.TrimSpace(req.WebhookURL), WebhookSigningSecret: strings.TrimSpace(req.SigningSecret)}
	if err := service.ValidateChannelHeaders(req.Headers); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	headers, _ := jsonMarshal(req.Headers)
	channel.HeadersJSON = headers
	if err := service.ValidateAlertWebhookURL(s.cfg, channel.WebhookURL); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	res := service.NewAlertService(s.db, s.logger, s.cfg).RunWebhookTest(channel)
	utils.SuccessResponse(c, http.StatusOK, "Test finished", webhookTestResult(res))
}

// alertTestResultFor builds the dialog data for a stored test delivery.
func (s *Server) alertTestResultFor(channel db.AlertChannel, delivery db.AlertDelivery) AlertTestResult {
	var attempt db.AlertDeliveryAttempt
	s.db.Where("alert_delivery_id = ?", delivery.ID).Order("attempt_number DESC").Limit(1).Find(&attempt)
	out := AlertTestResult{Passed: delivery.Status == "sent", Expected: IncidentExpectation{Title: "2xx", Detail: "within 10 s"}}
	out.Status = map[bool]string{true: "passed", false: "failed"}[out.Passed]
	if attempt.ID != "" {
		out.StatusCode, out.Body = attempt.HTTPStatus, attempt.ResponseSnippet
		if attempt.CompletedAt != nil {
			ms := attempt.CompletedAt.Sub(attempt.StartedAt).Milliseconds()
			out.DurationMs = &ms
		}
		out.Actual = IncidentExpectation{Title: attemptResultText(attempt)}
		if attempt.HTTPStatus == 0 && attempt.Error != "" {
			out.Actual.Detail = "no response"
		}
		out.Hint = service.TestHint(attempt.Stage, errors.New(attempt.Error))
	}
	if channel.Type == "email" {
		expected := IncidentExpectation{Title: "Mail accepted", Detail: "by the SMTP server"}
		out.Expected = expected
		var delivered, skipped int64
		s.db.Model(&db.AlertDeliveryRecipient{}).Where("delivery_id = ?", delivery.ID).Count(&delivered)
		s.db.Model(&db.AlertChannelRecipient{}).Where("channel_id = ? AND status <> ?", channel.ID, "confirmed").Count(&skipped)
		out.Recipients = &struct {
			Delivered int `json:"delivered"`
			Skipped   int `json:"skipped"`
		}{int(delivered), int(skipped)}
		if out.Passed {
			out.Actual = IncidentExpectation{Title: "Sent", Detail: itoa(int(delivered)) + " delivered, " + itoa(int(skipped)) + " skipped"}
		} else if delivery.Error != "" {
			out.Actual = IncidentExpectation{Title: "Failed", Detail: safeAlertDeliveryError(delivery.Error)}
		}
	}
	return out
}
