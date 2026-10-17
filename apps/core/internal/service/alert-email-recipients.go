package service

import (
	"errors"
	"fmt"
	"net/mail"
	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"strings"
	"time"

	"gorm.io/gorm"
)

var (
	ErrAlertRecipientInvalid  = errors.New("invalid email address")
	ErrAlertRecipientNotFound = errors.New("recipient not found")
	ErrAlertRecipientLimit    = errors.New("too many recipients on this channel")
)

const maxAlertRecipientsPerChannel = 50

func (s *AlertService) ListRecipients(channelID string) ([]db.AlertChannelRecipient, error) {
	var recipients []db.AlertChannelRecipient
	err := s.db.Where("channel_id = ?", channelID).Order("email ASC").Find(&recipients).Error
	return recipients, err
}

func (s *AlertService) confirmedRecipientCount(channelID string) int64 {
	var count int64
	s.db.Model(&db.AlertChannelRecipient{}).Where("channel_id = ? AND status = ?", channelID, "confirmed").Count(&count)
	return count
}

// AddRecipient registers an address and sends the confirmation mail. The address
// receives no alerts until the recipient follows the confirmation link.
func (s *AlertService) AddRecipient(channelID, address string) (*db.AlertChannelRecipient, error) {
	parsed, err := mail.ParseAddress(strings.TrimSpace(address))
	if err != nil || parsed.Name != "" || !strings.Contains(parsed.Address, "@") {
		return nil, ErrAlertRecipientInvalid
	}
	email := strings.ToLower(parsed.Address)
	var channel db.AlertChannel
	if err := s.db.Where("id = ? AND type = ?", channelID, "email").First(&channel).Error; err != nil {
		return nil, err
	}
	var count int64
	s.db.Model(&db.AlertChannelRecipient{}).Where("channel_id = ?", channelID).Count(&count)
	var recipient db.AlertChannelRecipient
	result := s.db.Where("channel_id = ? AND email = ?", channelID, email).Limit(1).Find(&recipient)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		if count >= maxAlertRecipientsPerChannel {
			return nil, ErrAlertRecipientLimit
		}
		recipient = db.AlertChannelRecipient{ID: utils.GenerateID("alert_recipient"), ChannelID: channelID, Email: email, Status: "pending"}
		if err := s.db.Create(&recipient).Error; err != nil {
			return nil, err
		}
	} else if recipient.Status == "confirmed" {
		return &recipient, nil
	} else if recipient.Status == "unsubscribed" {
		// An address that opted out can only be re-added by an operator; reset to pending so a fresh opt-in is required.
		recipient.Status = "pending"
	}
	return &recipient, s.SendConfirmation(&recipient, channel)
}

// SendConfirmation (re)sends the double opt-in mail; throttled to once a minute per recipient.
func (s *AlertService) SendConfirmation(recipient *db.AlertChannelRecipient, channel db.AlertChannel) error {
	now := time.Now().UTC()
	if recipient.ConfirmationSentAt != nil && now.Sub(*recipient.ConfirmationSentAt) < time.Minute {
		return nil
	}
	token := SignAlertEmailToken(s.cfg, AlertEmailPurposeConfirm, recipient.ID, alertEmailConfirmTTL, now)
	text := fmt.Sprintf("Someone added this address to the Orion alert channel %q.\n\nTo start receiving alerts, confirm here (valid for 72 hours):\n%s\n\nIf you did not expect this, ignore this email; no alerts will be sent.\n",
		channel.Name, AlertEmailLink(s.cfg, "confirm", token))
	if err := SendAlertEmail(s.cfg, AlertEmailMessage{To: recipient.Email, Subject: "Confirm Orion alert emails", Text: text}); err != nil {
		return err
	}
	return s.db.Model(&db.AlertChannelRecipient{}).Where("id = ?", recipient.ID).
		Updates(map[string]any{"confirmation_sent_at": now, "status": recipient.Status, "updated_at": now}).Error
}

func (s *AlertService) RemoveRecipient(channelID, recipientID string) error {
	res := s.db.Where("id = ? AND channel_id = ?", recipientID, channelID).Delete(&db.AlertChannelRecipient{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrAlertRecipientNotFound
	}
	return nil
}

func (s *AlertService) ResendConfirmation(channelID, recipientID string) error {
	var recipient db.AlertChannelRecipient
	if err := s.db.Where("id = ? AND channel_id = ?", recipientID, channelID).First(&recipient).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrAlertRecipientNotFound
		}
		return err
	}
	if recipient.Status == "confirmed" {
		return nil
	}
	var channel db.AlertChannel
	if err := s.db.Where("id = ?", channelID).First(&channel).Error; err != nil {
		return err
	}
	recipient.Status = "pending"
	return s.SendConfirmation(&recipient, channel)
}

// ConfirmRecipient completes the opt-in for a valid, unexpired token.
func (s *AlertService) ConfirmRecipient(token string) error {
	id, err := VerifyAlertEmailToken(s.cfg, AlertEmailPurposeConfirm, token, time.Now().UTC())
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	res := s.db.Model(&db.AlertChannelRecipient{}).Where("id = ?", id).
		Updates(map[string]any{"status": "confirmed", "confirmed_at": now, "updated_at": now})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrAlertEmailToken
	}
	return nil
}

// UnsubscribeRecipient honours an unsubscribe link; the address stays on the list as "unsubscribed".
func (s *AlertService) UnsubscribeRecipient(token string) error {
	id, err := VerifyAlertEmailToken(s.cfg, AlertEmailPurposeUnsubscribe, token, time.Now().UTC())
	if err != nil {
		return err
	}
	res := s.db.Model(&db.AlertChannelRecipient{}).Where("id = ?", id).
		Updates(map[string]any{"status": "unsubscribed", "updated_at": time.Now().UTC()})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrAlertEmailToken
	}
	return nil
}

// AlertEmailLink builds a public confirm/unsubscribe URL.
func AlertEmailLink(cfg *config.Config, action, token string) string {
	return strings.TrimRight(cfg.PublicURL, "/") + "/alerts/email/" + action + "/" + token
}
