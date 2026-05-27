package api

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/mail"
	"net/smtp"
	"net/url"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"strings"
	"time"
)

const (
	statusPageSubscriberDeliveryStateQueued = "queued"
	statusPageSubscriberDeliveryStateSent   = "sent"
	statusPageSubscriberDeliveryStateFailed = "failed"

	statusPageSubscriberDeliveryErrorPublicSenderInvalid      = "public_mail_sender_invalid_configuration"
	statusPageSubscriberDeliveryErrorPublicSenderFailed       = "public_mail_sender_delivery_failed"
	statusPageSubscriberDeliveryErrorPublicDestinationMissing = "public_subscriber_destination_unavailable"
	statusPageSubscriberDeliverySummaryPublicSenderInvalid    = "Public status page mail sender configuration is invalid."
	statusPageSubscriberDeliverySummaryPublicSenderFailed     = "Public status page mail delivery failed."
	statusPageSubscriberDeliverySummaryDestinationUnavailable = "Public status page subscriber destination is unavailable."
)

type publicStatusMailMessage struct {
	To      string
	Subject string
	Text    string
}

type publicStatusMailError struct {
	code    string
	summary string
	state   string
}

func (e *publicStatusMailError) Error() string { return e.summary }

func (s *Server) sendStatusPageSubscriberConfirmation(page db.StatusPage, destination string, token string) error {
	if err := s.ensurePublicStatusMailConfigured(); err != nil {
		return err
	}
	confirmationURL, err := s.publicStatusPageSubscriberActionURL(page, "confirm", token)
	if err != nil {
		return err
	}
	message := publicStatusMailMessage{
		To:      destination,
		Subject: sanitizePublicStatusMailHeader(fmt.Sprintf("Confirm your %s subscription", page.Title)),
		Text: strings.Join([]string{
			fmt.Sprintf("Confirm your subscription to %s.", page.Title),
			"",
			"Use this link to confirm your subscription:",
			confirmationURL,
			"",
			"If you did not request this subscription, you can ignore this email.",
		}, "\n"),
	}
	if err := s.publicStatusMailSend(message); err != nil {
		return &publicStatusMailError{
			code:    statusPageSubscriberDeliveryErrorPublicSenderFailed,
			summary: statusPageSubscriberDeliverySummaryPublicSenderFailed,
			state:   statusPageSubscriberDeliveryStateFailed,
		}
	}
	return nil
}

func (s *Server) sendStatusPageIncidentUpdateMail(page db.StatusPage, incident db.StatusPageIncident, update db.StatusPageIncidentUpdate, subscriber db.StatusPageSubscriber, destination string, unsubscribeToken string) error {
	if err := s.ensurePublicStatusMailConfigured(); err != nil {
		return err
	}
	incidentURL, err := s.publicStatusPageIncidentURL(page, incident.ID)
	if err != nil {
		return err
	}
	unsubscribeURL, err := s.publicStatusPageSubscriberActionURL(page, "unsubscribe", unsubscribeToken)
	if err != nil {
		return err
	}
	subject := sanitizePublicStatusMailHeader(fmt.Sprintf("%s: %s", page.Title, incident.Title))
	text := strings.Join([]string{
		fmt.Sprintf("%s status update", page.Title),
		"",
		incident.Title,
		fmt.Sprintf("Status: %s", update.Status),
		"",
		update.Message,
		"",
		fmt.Sprintf("Incident: %s", incidentURL),
		fmt.Sprintf("Unsubscribe: %s", unsubscribeURL),
	}, "\n")
	if err := s.publicStatusMailSend(publicStatusMailMessage{To: destination, Subject: subject, Text: text}); err != nil {
		return &publicStatusMailError{
			code:    statusPageSubscriberDeliveryErrorPublicSenderFailed,
			summary: statusPageSubscriberDeliverySummaryPublicSenderFailed,
			state:   statusPageSubscriberDeliveryStateFailed,
		}
	}
	return nil
}

func (s *Server) deliverPublicStatusMail(message publicStatusMailMessage) error {
	if err := s.ensurePublicStatusMailConfigured(); err != nil {
		return err
	}
	fromEmail := strings.TrimSpace(s.cfg.PublicStatusMailFromEmail)
	address := fmt.Sprintf("%s:%d", strings.TrimSpace(s.cfg.PublicStatusMailHost), s.cfg.PublicStatusMailPort)
	var auth smtp.Auth
	if s.cfg.PublicStatusMailUsername != "" || s.cfg.PublicStatusMailPassword != "" {
		auth = smtp.PlainAuth("", s.cfg.PublicStatusMailUsername, s.cfg.PublicStatusMailPassword, strings.TrimSpace(s.cfg.PublicStatusMailHost))
	}
	return smtp.SendMail(address, auth, fromEmail, []string{message.To}, s.publicStatusMailMessageBytes(message))
}

func (s *Server) publicStatusMailMessageBytes(message publicStatusMailMessage) []byte {
	from := mail.Address{Name: sanitizePublicStatusMailHeader(s.cfg.PublicStatusMailFromName), Address: strings.TrimSpace(s.cfg.PublicStatusMailFromEmail)}
	headers := []string{
		"From: " + from.String(),
		"To: " + sanitizePublicStatusMailHeader(message.To),
		"Subject: " + sanitizePublicStatusMailHeader(message.Subject),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=utf-8",
	}
	if strings.TrimSpace(s.cfg.PublicStatusMailReplyTo) != "" {
		headers = append(headers, "Reply-To: "+sanitizePublicStatusMailHeader(s.cfg.PublicStatusMailReplyTo))
	}
	return []byte(strings.Join(headers, "\r\n") + "\r\n\r\n" + strings.ReplaceAll(message.Text, "\n", "\r\n"))
}

func (s *Server) ensurePublicStatusMailConfigured() error {
	if s == nil || s.cfg == nil || !s.cfg.PublicStatusMailEnabled {
		return publicStatusMailNotConfiguredError()
	}
	if strings.TrimSpace(s.cfg.PublicStatusMailHost) == "" ||
		s.cfg.PublicStatusMailPort <= 0 ||
		strings.TrimSpace(s.cfg.PublicStatusMailFromEmail) == "" ||
		strings.TrimSpace(s.cfg.PublicStatusURLOrigin) == "" ||
		strings.TrimSpace(s.cfg.PublicStatusSubscriberSecret) == "" {
		return publicStatusMailNotConfiguredError()
	}
	if _, err := mail.ParseAddress(strings.TrimSpace(s.cfg.PublicStatusMailFromEmail)); err != nil {
		return publicStatusMailInvalidError()
	}
	if strings.TrimSpace(s.cfg.PublicStatusMailReplyTo) != "" {
		if _, err := mail.ParseAddress(strings.TrimSpace(s.cfg.PublicStatusMailReplyTo)); err != nil {
			return publicStatusMailInvalidError()
		}
	}
	if _, err := s.publicStatusPageOriginURL(); err != nil {
		return publicStatusMailInvalidError()
	}
	return nil
}

func publicStatusMailNotConfiguredError() error {
	return &publicStatusMailError{
		code:    statusPageSubscriberDeliveryErrorPublicSenderMissing,
		summary: statusPageSubscriberDeliverySummaryPublicSenderMissing,
		state:   statusPageSubscriberDeliveryStatePendingSenderConfig,
	}
}

func publicStatusMailInvalidError() error {
	return &publicStatusMailError{
		code:    statusPageSubscriberDeliveryErrorPublicSenderInvalid,
		summary: statusPageSubscriberDeliverySummaryPublicSenderInvalid,
		state:   statusPageSubscriberDeliveryStateFailed,
	}
}

func publicStatusDestinationUnavailableError() error {
	return &publicStatusMailError{
		code:    statusPageSubscriberDeliveryErrorPublicDestinationMissing,
		summary: statusPageSubscriberDeliverySummaryDestinationUnavailable,
		state:   statusPageSubscriberDeliveryStateFailed,
	}
}

func safePublicStatusMailFailure(err error) (string, string, string) {
	var mailErr *publicStatusMailError
	if errors.As(err, &mailErr) {
		return mailErr.state, mailErr.code, mailErr.summary
	}
	return statusPageSubscriberDeliveryStateFailed, statusPageSubscriberDeliveryErrorPublicSenderFailed, statusPageSubscriberDeliverySummaryPublicSenderFailed
}

func (s *Server) publicStatusPageSubscriberActionURL(page db.StatusPage, action string, token string) (string, error) {
	base, err := s.publicStatusPageBaseURL(page)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(token) == "" {
		return "", publicStatusMailInvalidError()
	}
	return base + "/status/" + url.PathEscape(page.Slug) + "/subscribers/" + url.PathEscape(action) + "/" + url.PathEscape(token), nil
}

func (s *Server) publicStatusPageIncidentURL(page db.StatusPage, incidentID string) (string, error) {
	base, err := s.publicStatusPageBaseURL(page)
	if err != nil {
		return "", err
	}
	return base + "/status/" + url.PathEscape(page.Slug) + "/incidents/" + url.PathEscape(incidentID), nil
}

func (s *Server) publicStatusPageBaseURL(page db.StatusPage) (string, error) {
	if strings.TrimSpace(page.CustomDomain) != "" {
		if domain, err := normalizeStatusPageCustomDomain(page.CustomDomain); err == nil && domain != "" {
			return "https://" + domain, nil
		}
		return "", publicStatusMailInvalidError()
	}
	origin, err := s.publicStatusPageOriginURL()
	if err != nil {
		return "", err
	}
	return origin, nil
}

func (s *Server) publicStatusPageOriginURL() (string, error) {
	if s == nil || s.cfg == nil {
		return "", publicStatusMailNotConfiguredError()
	}
	origin := strings.TrimRight(strings.TrimSpace(s.cfg.PublicStatusURLOrigin), "/")
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", publicStatusMailInvalidError()
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", publicStatusMailInvalidError()
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" || (parsed.Path != "" && parsed.Path != "/") {
		return "", publicStatusMailInvalidError()
	}
	return origin, nil
}

func (s *Server) encryptStatusPageSubscriberDestination(destination string) (string, error) {
	secret := ""
	if s != nil && s.cfg != nil {
		secret = strings.TrimSpace(s.cfg.PublicStatusSubscriberSecret)
	}
	if secret == "" {
		return "", nil
	}
	block, err := aes.NewCipher(publicStatusSubscriberEncryptionKey(secret))
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nil, nonce, []byte(destination), nil)
	payload := append(nonce, ciphertext...)
	return base64.RawURLEncoding.EncodeToString(payload), nil
}

func (s *Server) decryptStatusPageSubscriberDestination(subscriber db.StatusPageSubscriber) (string, error) {
	secret := ""
	if s != nil && s.cfg != nil {
		secret = strings.TrimSpace(s.cfg.PublicStatusSubscriberSecret)
	}
	if secret == "" || strings.TrimSpace(subscriber.DestinationValueCiphertext) == "" {
		return "", publicStatusDestinationUnavailableError()
	}
	payload, err := base64.RawURLEncoding.DecodeString(subscriber.DestinationValueCiphertext)
	if err != nil {
		return "", publicStatusDestinationUnavailableError()
	}
	block, err := aes.NewCipher(publicStatusSubscriberEncryptionKey(secret))
	if err != nil {
		return "", publicStatusDestinationUnavailableError()
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", publicStatusDestinationUnavailableError()
	}
	if len(payload) <= gcm.NonceSize() {
		return "", publicStatusDestinationUnavailableError()
	}
	nonce := payload[:gcm.NonceSize()]
	ciphertext := payload[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", publicStatusDestinationUnavailableError()
	}
	return string(plaintext), nil
}

func publicStatusSubscriberEncryptionKey(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}

func sanitizePublicStatusMailHeader(value string) string {
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	return strings.TrimSpace(value)
}

func (s *Server) recordStatusPageSubscriberDelivery(pageID string, subscriberID string, state string, errorCode string, summary string) {
	now := time.Now().UTC()
	delivery := db.StatusPageSubscriberDelivery{
		ID:               utils.GenerateID("status_page_delivery"),
		SubscriberID:     subscriberID,
		StatusPageID:     pageID,
		DeliveryType:     statusPageSubscriberDeliveryTypeEmail,
		DeliveryState:    state,
		ErrorCode:        errorCode,
		SafeErrorSummary: summary,
		AttemptCount:     1,
		QueuedAt:         &now,
	}
	if state == statusPageSubscriberDeliveryStatePendingSenderConfig {
		delivery.AttemptCount = 0
	}
	if state == statusPageSubscriberDeliveryStateSent {
		delivery.SentAt = &now
	}
	if state == statusPageSubscriberDeliveryStateFailed {
		delivery.FailedAt = &now
	}
	if err := s.db.Create(&delivery).Error; err != nil {
		s.logger.Error("Failed to record status page subscriber delivery", "status_page_id", pageID, "subscriber_id", subscriberID, "error", err)
		return
	}
	if err := s.db.Model(&db.StatusPageSubscriber{}).
		Where("id = ?", subscriberID).
		Updates(map[string]interface{}{
			"last_delivery_status": state,
			"last_delivery_at":     now,
		}).Error; err != nil {
		s.logger.Error("Failed to update status page subscriber delivery summary", "subscriber_id", subscriberID, "error", err)
	}
}
