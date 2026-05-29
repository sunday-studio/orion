package service

import (
	"errors"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"time"
)

func (s *AlertService) attemptDelivery(delivery *db.AlertDelivery, deliver func() error) error {
	attemptNumber := delivery.AttemptCount + 1
	maxAttempts := delivery.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = defaultAlertDeliveryMaxAttempts
	}

	startedAt := time.Now().UTC()
	attempt := db.AlertDeliveryAttempt{
		ID:              utils.GenerateID("alert_delivery_attempt"),
		AlertDeliveryID: delivery.ID,
		AttemptNumber:   attemptNumber,
		Status:          "pending",
		Stage:           "transport",
		StartedAt:       startedAt,
	}
	if err := s.db.Create(&attempt).Error; err != nil {
		return err
	}

	deliverErr := deliver()
	completedAt := time.Now().UTC()
	status := "sent"
	message := ""
	stage := "transport"
	var nextAttemptAt *time.Time
	if deliverErr != nil {
		status = "failed"
		message = deliverErr.Error()
		stage = alertDeliveryErrorStage(deliverErr)
		if attemptNumber < maxAttempts {
			next := completedAt.Add(defaultAlertDeliveryRetryDelay)
			nextAttemptAt = &next
		}
	}

	if updateErr := s.db.Model(&db.AlertDeliveryAttempt{}).Where("id = ?", attempt.ID).Updates(map[string]interface{}{
		"status":       status,
		"stage":        stage,
		"error":        message,
		"completed_at": completedAt,
	}).Error; updateErr != nil {
		return errors.Join(deliverErr, updateErr)
	}

	if updateErr := s.db.Model(&db.AlertDelivery{}).Where("id = ?", delivery.ID).Updates(map[string]interface{}{
		"status":          status,
		"error":           message,
		"attempt_count":   attemptNumber,
		"max_attempts":    maxAttempts,
		"next_attempt_at": nextAttemptAt,
		"last_attempt_at": completedAt,
	}).Error; updateErr != nil {
		return errors.Join(deliverErr, updateErr)
	}

	delivery.Status = status
	delivery.Error = message
	delivery.AttemptCount = attemptNumber
	delivery.MaxAttempts = maxAttempts
	delivery.NextAttemptAt = nextAttemptAt
	delivery.LastAttemptAt = &completedAt
	return deliverErr
}
