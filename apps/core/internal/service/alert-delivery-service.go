package service

import (
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"strings"
	"time"

	"gorm.io/gorm"
)

func (s *AlertService) ProcessDueDeliveries(limit int) (int, error) {
	if limit <= 0 {
		limit = 50
	}

	now := time.Now().UTC()
	var deliveries []db.AlertDelivery
	if err := s.db.
		Where("status IN ? AND attempt_count < max_attempts AND (next_attempt_at IS NULL OR next_attempt_at <= ?)", []string{"pending", "failed"}, now).
		Order("created_at ASC").
		Limit(limit).
		Find(&deliveries).Error; err != nil {
		return 0, err
	}

	processed := 0
	for i := range deliveries {
		delivery := &deliveries[i]
		if retiredAlertDeliveryType(delivery.Type) {
			if err := s.updateDelivery(delivery.ID, "suppressed", retiredAlertDeliveryMessage); err != nil {
				s.logger.Error("Failed to suppress retired alert delivery", "delivery_id", delivery.ID, "type", delivery.Type, "error", err)
			}
			processed++
			continue
		}
		channel, err := s.channelForDelivery(*delivery)
		if err != nil {
			if attemptErr := s.attemptDelivery(delivery, func() error {
				return newAlertDeliveryError("channel_lookup", err)
			}); attemptErr != nil {
				s.logger.Error("Alert delivery retry failed", "delivery_id", delivery.ID, "channel", delivery.Channel, "error", attemptErr)
			}
			processed++
			continue
		}
		if err := s.attemptDelivery(delivery, func() error {
			if delivery.EventType == alertEventGroupSummary && delivery.AlertGroupID != "" {
				return s.deliverGroupSummary(channel, delivery.AlertGroupID)
			}
			return s.deliver(channel, delivery.IncidentID, delivery.EventType)
		}); err != nil {
			s.logger.Error("Alert delivery retry failed", "delivery_id", delivery.ID, "channel", delivery.Channel, "error", err)
		}
		processed++
	}
	return processed, nil
}

func (s *AlertService) channelForDelivery(delivery db.AlertDelivery) (db.AlertChannel, error) {
	channels, err := s.deliveryChannels()
	if err != nil {
		return db.AlertChannel{}, err
	}
	for _, channel := range channels {
		if channel.Name == delivery.Channel && channel.Type == delivery.Type {
			return channel, nil
		}
	}
	return db.AlertChannel{}, gorm.ErrRecordNotFound
}

func retiredAlertDeliveryType(deliveryType string) bool {
	switch strings.TrimSpace(deliveryType) {
	case "email", "slack", "discord":
		return true
	default:
		return false
	}
}

func (s *AlertService) createDelivery(delivery db.AlertDelivery) (*db.AlertDelivery, error) {
	delivery.ID = utils.GenerateID("alert_delivery")
	if delivery.Status == "pending" && delivery.MaxAttempts == 0 {
		delivery.MaxAttempts = defaultAlertDeliveryMaxAttempts
	}
	if err := s.db.Create(&delivery).Error; err != nil {
		s.logger.Error("Failed to create alert delivery", "incident_id", delivery.IncidentID, "event_type", delivery.EventType, "error", err)
		return nil, err
	}
	return &delivery, nil
}

func (s *AlertService) updateDelivery(deliveryID string, status string, message string) error {
	return s.db.Model(&db.AlertDelivery{}).Where("id = ?", deliveryID).Updates(map[string]any{
		"status": status,
		"error":  message,
	}).Error
}

func (s *AlertService) inCooldown(incidentID string, channelName string, eventType string, currentDeliveryID string) bool {
	if s.cfg == nil || s.cfg.AlertCooldownSeconds <= 0 {
		return false
	}
	since := time.Now().UTC().Add(-time.Duration(s.cfg.AlertCooldownSeconds) * time.Second)

	var count int64
	if err := s.db.Model(&db.AlertDelivery{}).
		Where("incident_id = ? AND channel = ? AND event_type = ? AND id <> ? AND status = ? AND created_at >= ?", incidentID, channelName, eventType, currentDeliveryID, "sent", since).
		Count(&count).Error; err != nil {
		s.logger.Error("Failed to check alert cooldown", "incident_id", incidentID, "channel", channelName, "error", err)
		return false
	}
	return count > 0
}
