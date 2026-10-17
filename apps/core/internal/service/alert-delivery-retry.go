package service

import (
	"errors"
	"orion/core/internal/db"

	"gorm.io/gorm"
)

var (
	ErrAlertDeliveryNotFound     = errors.New("alert delivery not found")
	ErrAlertDeliveryNotRetryable = errors.New("alert delivery is not in a retryable state")
)

// RetryDelivery re-queues a failed delivery for the dispatcher. A delivery that
// exhausted its attempts gets exactly one more, so an operator retry is always
// honoured without lifting the retry cap for automatic retries. Only failed
// deliveries can be retried; sent, in-flight and suppressed ones are left alone.
func (s *AlertService) RetryDelivery(deliveryID string) (db.AlertDelivery, error) {
	var delivery db.AlertDelivery
	if err := s.db.Where("id = ?", deliveryID).First(&delivery).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return db.AlertDelivery{}, ErrAlertDeliveryNotFound
		}
		return db.AlertDelivery{}, err
	}
	if delivery.Status != "failed" {
		return db.AlertDelivery{}, ErrAlertDeliveryNotRetryable
	}

	maxAttempts := delivery.MaxAttempts
	if delivery.AttemptCount >= maxAttempts {
		maxAttempts = delivery.AttemptCount + 1
	}
	result := s.db.Model(&db.AlertDelivery{}).
		Where("id = ? AND status = ?", deliveryID, "failed").
		Updates(map[string]any{
			"status":          "pending",
			"max_attempts":    maxAttempts,
			"next_attempt_at": nil,
			"locked_by":       "",
			"locked_until":    nil,
		})
	if result.Error != nil {
		return db.AlertDelivery{}, result.Error
	}
	if result.RowsAffected == 0 {
		return db.AlertDelivery{}, ErrAlertDeliveryNotRetryable
	}

	if err := s.db.Preload("Attempts").Where("id = ?", deliveryID).First(&delivery).Error; err != nil {
		return db.AlertDelivery{}, err
	}
	return delivery, nil
}
