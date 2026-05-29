package service

import (
	"errors"
	"strings"
	"time"

	"orion/core/internal/db"
	"orion/core/internal/utils"

	"gorm.io/gorm"
)

const (
	DefaultStatusPageSubscriberPendingRetention     = 7 * 24 * time.Hour
	DefaultStatusPageSubscriberSuppressionRetention = 90 * 24 * time.Hour
	DefaultStatusPageSubscriberDeliveryRetention    = 180 * 24 * time.Hour
	statusPageSubscriberStatePending                = "pending"
	statusPageSubscriberStateUnsubscribed           = "unsubscribed"
	statusPageSubscriberStateBounced                = "bounced"
	statusPageSubscriberStateDisabled               = "disabled"
	statusPageSubscriberDeletedMaskedDestination    = "deleted subscriber"
	statusPageSubscriberRetentionActorType          = "system"
	statusPageSubscriberRetentionActorID            = "subscriber-retention"
	statusPageSubscriberAffectedObjectType          = "status_page_subscriber"
	statusPageSubscriberDeliveryAffectedType        = "status_page_subscriber_delivery"
	statusPageSubscriberDeletionModeAnonymize       = "anonymize"
	statusPageSubscriberDeletionModeHardDelete      = "hard_delete"
	statusPageSubscriberRetentionBasisPending       = "pending_confirmation_expired"
	statusPageSubscriberRetentionBasisSuppression   = "suppression_retention_expired"
	statusPageSubscriberRetentionBasisDelivery      = "delivery_retention_expired"
)

type StatusPageSubscriberLifecycleService struct {
	db     *gorm.DB
	logger interface {
		Error(message string, fields ...interface{})
	}
}

type StatusPageSubscriberPrivacyActionInput struct {
	SubscriberID string
	ActorType    string
	ActorID      string
	Reason       string
	Basis        string
}

type StatusPageSubscriberRetentionOptions struct {
	PendingRetention     time.Duration
	SuppressionRetention time.Duration
	DeliveryRetention    time.Duration
}

type StatusPageSubscriberRetentionResult struct {
	PendingSubscribersDeleted     int `json:"pending_subscribers_deleted"`
	SubscribersAnonymized         int `json:"subscribers_anonymized"`
	SubscriberDeliveriesDeleted   int `json:"subscriber_deliveries_deleted"`
	SubscriberDeliveryAuditEvents int `json:"subscriber_delivery_audit_events"`
}

func NewStatusPageSubscriberLifecycleService(database *gorm.DB, logger interface {
	Error(message string, fields ...interface{})
}) *StatusPageSubscriberLifecycleService {
	return &StatusPageSubscriberLifecycleService{db: database, logger: logger}
}

func (s *StatusPageSubscriberLifecycleService) RunRetentionCleanup(now time.Time, options StatusPageSubscriberRetentionOptions) (*StatusPageSubscriberRetentionResult, error) {
	options = normalizeStatusPageSubscriberRetentionOptions(options)
	result := &StatusPageSubscriberRetentionResult{}
	var runErrors []error

	pendingCutoff := now.Add(-options.PendingRetention)
	var pendingSubscribers []db.StatusPageSubscriber
	if err := s.db.Where("state = ? AND confirmation_token_expires_at IS NOT NULL AND confirmation_token_expires_at < ?", statusPageSubscriberStatePending, pendingCutoff).Find(&pendingSubscribers).Error; err != nil {
		return result, err
	}
	for _, subscriber := range pendingSubscribers {
		if err := s.HardDeleteSubscriber(StatusPageSubscriberPrivacyActionInput{
			SubscriberID: subscriber.ID,
			ActorType:    statusPageSubscriberRetentionActorType,
			ActorID:      statusPageSubscriberRetentionActorID,
			Reason:       "pending subscriber confirmation retention expired",
			Basis:        statusPageSubscriberRetentionBasisPending,
		}); err != nil {
			runErrors = append(runErrors, err)
			continue
		}
		result.PendingSubscribersDeleted++
	}

	suppressionCutoff := now.Add(-options.SuppressionRetention)
	var suppressedSubscribers []db.StatusPageSubscriber
	if err := s.db.Where(
		"(state = ? AND unsubscribed_at IS NOT NULL AND unsubscribed_at < ?) OR (state = ? AND updated_at < ?)",
		statusPageSubscriberStateUnsubscribed,
		suppressionCutoff,
		statusPageSubscriberStateBounced,
		suppressionCutoff,
	).Find(&suppressedSubscribers).Error; err != nil {
		runErrors = append(runErrors, err)
		return result, errors.Join(runErrors...)
	}
	for _, subscriber := range suppressedSubscribers {
		if err := s.AnonymizeSubscriber(StatusPageSubscriberPrivacyActionInput{
			SubscriberID: subscriber.ID,
			ActorType:    statusPageSubscriberRetentionActorType,
			ActorID:      statusPageSubscriberRetentionActorID,
			Reason:       "subscriber suppression retention expired",
			Basis:        statusPageSubscriberRetentionBasisSuppression,
		}); err != nil {
			runErrors = append(runErrors, err)
			continue
		}
		result.SubscribersAnonymized++
	}

	deletedDeliveries, deliveryAuditEvents, err := s.purgeExpiredDeliveries(now.Add(-options.DeliveryRetention))
	result.SubscriberDeliveriesDeleted = deletedDeliveries
	result.SubscriberDeliveryAuditEvents = deliveryAuditEvents
	if err != nil {
		runErrors = append(runErrors, err)
	}

	return result, errors.Join(runErrors...)
}

func (s *StatusPageSubscriberLifecycleService) AnonymizeSubscriber(input StatusPageSubscriberPrivacyActionInput) error {
	normalized := normalizeStatusPageSubscriberPrivacyActionInput(input)
	if normalized.SubscriberID == "" {
		return errors.New("subscriber id is required")
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		var subscriber db.StatusPageSubscriber
		if err := tx.Where("id = ?", normalized.SubscriberID).First(&subscriber).Error; err != nil {
			return err
		}
		if err := recordStatusPageSubscriberPrivacyAuditEvent(tx, subscriber, StatusPageAuditActionSubscriberAnonymized, normalized, statusPageSubscriberDeletionModeAnonymize); err != nil {
			return err
		}
		if err := tx.Where("subscriber_id = ?", subscriber.ID).Delete(&db.StatusPageSubscriberComponent{}).Error; err != nil {
			return err
		}
		if err := tx.Model(&db.StatusPageSubscriberDelivery{}).Where("subscriber_id = ?", subscriber.ID).Updates(map[string]interface{}{
			"provider_message_id": "",
			"error_code":          "",
			"safe_error_summary":  "",
		}).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		return tx.Model(&db.StatusPageSubscriber{}).Where("id = ?", subscriber.ID).Updates(map[string]interface{}{
			"destination_hash":              deletedStatusPageSubscriberDestinationHash(subscriber.ID),
			"destination_value_ciphertext":  "",
			"masked_destination":            statusPageSubscriberDeletedMaskedDestination,
			"state":                         statusPageSubscriberStateDisabled,
			"confirmation_token_hash":       "",
			"confirmation_token_expires_at": nil,
			"manage_token_hash":             "",
			"manage_token_version":          subscriber.ManageTokenVersion + 1,
			"unsubscribe_token_hash":        "",
			"unsubscribe_token_version":     subscriber.UnsubscribeTokenVersion + 1,
			"disabled_at":                   now,
		}).Error
	})
}

func (s *StatusPageSubscriberLifecycleService) HardDeleteSubscriber(input StatusPageSubscriberPrivacyActionInput) error {
	normalized := normalizeStatusPageSubscriberPrivacyActionInput(input)
	if normalized.SubscriberID == "" {
		return errors.New("subscriber id is required")
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		var subscriber db.StatusPageSubscriber
		if err := tx.Where("id = ?", normalized.SubscriberID).First(&subscriber).Error; err != nil {
			return err
		}
		action := StatusPageAuditActionSubscriberHardDeleted
		if normalized.Basis == statusPageSubscriberRetentionBasisPending {
			action = StatusPageAuditActionSubscriberPendingPurged
		}
		if err := recordStatusPageSubscriberPrivacyAuditEvent(tx, subscriber, action, normalized, statusPageSubscriberDeletionModeHardDelete); err != nil {
			return err
		}
		if err := tx.Where("subscriber_id = ?", subscriber.ID).Delete(&db.StatusPageSubscriberComponent{}).Error; err != nil {
			return err
		}
		if err := tx.Where("subscriber_id = ?", subscriber.ID).Delete(&db.StatusPageSubscriberDelivery{}).Error; err != nil {
			return err
		}
		return tx.Delete(&db.StatusPageSubscriber{}, "id = ?", subscriber.ID).Error
	})
}

func (s *StatusPageSubscriberLifecycleService) RecordUnsubscribe(subscriber db.StatusPageSubscriber, actorType string, actorID string) error {
	input := normalizeStatusPageSubscriberPrivacyActionInput(StatusPageSubscriberPrivacyActionInput{
		SubscriberID: subscriber.ID,
		ActorType:    actorType,
		ActorID:      actorID,
		Reason:       "public subscriber unsubscribe",
		Basis:        "public_unsubscribe",
	})
	return recordStatusPageSubscriberPrivacyAuditEvent(s.db, subscriber, StatusPageAuditActionSubscriberUnsubscribed, input, "")
}

func (s *StatusPageSubscriberLifecycleService) purgeExpiredDeliveries(cutoff time.Time) (int, int, error) {
	type deliveryGroup struct {
		StatusPageID string
		Count        int
	}
	var groups []deliveryGroup
	if err := s.db.Model(&db.StatusPageSubscriberDelivery{}).
		Select("status_page_id, count(*) as count").
		Where("created_at < ?", cutoff).
		Group("status_page_id").
		Scan(&groups).Error; err != nil {
		return 0, 0, err
	}
	if len(groups) == 0 {
		return 0, 0, nil
	}

	var deleted int
	var auditEvents int
	err := s.db.Transaction(func(tx *gorm.DB) error {
		for _, group := range groups {
			event := db.AuditEvent{
				ID:                 generateStatusPageSubscriberAuditID(),
				Action:             StatusPageAuditActionSubscriberDeliveriesPurged,
				StatusPageID:       group.StatusPageID,
				AffectedObjectType: statusPageSubscriberDeliveryAffectedType,
				AffectedObjectID:   group.StatusPageID,
				ActorType:          statusPageSubscriberRetentionActorType,
				ActorID:            statusPageSubscriberRetentionActorID,
				MetadataJSON: statusPageSubscriberAuditMetadata(map[string]interface{}{
					"retention_basis": statusPageSubscriberRetentionBasisDelivery,
					"deleted_count":   group.Count,
				}),
				CreatedAt: time.Now().UTC(),
			}
			if err := tx.Create(&event).Error; err != nil {
				return err
			}
			auditEvents++
		}
		result := tx.Where("created_at < ?", cutoff).Delete(&db.StatusPageSubscriberDelivery{})
		if result.Error != nil {
			return result.Error
		}
		deleted = int(result.RowsAffected)
		return nil
	})
	return deleted, auditEvents, err
}

func normalizeStatusPageSubscriberRetentionOptions(options StatusPageSubscriberRetentionOptions) StatusPageSubscriberRetentionOptions {
	if options.PendingRetention <= 0 {
		options.PendingRetention = DefaultStatusPageSubscriberPendingRetention
	}
	if options.SuppressionRetention <= 0 {
		options.SuppressionRetention = DefaultStatusPageSubscriberSuppressionRetention
	}
	if options.DeliveryRetention <= 0 {
		options.DeliveryRetention = DefaultStatusPageSubscriberDeliveryRetention
	}
	return options
}

func normalizeStatusPageSubscriberPrivacyActionInput(input StatusPageSubscriberPrivacyActionInput) StatusPageSubscriberPrivacyActionInput {
	actorType := strings.TrimSpace(input.ActorType)
	if actorType == "" {
		actorType = "user"
	}
	actorID := strings.TrimSpace(input.ActorID)
	if actorID == "" {
		actorID = "admin"
	}
	return StatusPageSubscriberPrivacyActionInput{
		SubscriberID: strings.TrimSpace(input.SubscriberID),
		ActorType:    actorType,
		ActorID:      actorID,
		Reason:       sanitizeStatusPageSubscriberAuditMetadataValue(input.Reason),
		Basis:        sanitizeStatusPageSubscriberAuditMetadataValue(input.Basis),
	}
}

func recordStatusPageSubscriberPrivacyAuditEvent(tx *gorm.DB, subscriber db.StatusPageSubscriber, action string, input StatusPageSubscriberPrivacyActionInput, deletionMode string) error {
	metadata := map[string]interface{}{
		"previous_state":   subscriber.State,
		"destination_type": subscriber.DestinationType,
	}
	if input.Reason != "" {
		metadata["reason"] = input.Reason
	}
	if input.Basis != "" {
		metadata["retention_basis"] = input.Basis
	}
	if deletionMode != "" {
		metadata["deletion_mode"] = deletionMode
	}
	event := db.AuditEvent{
		ID:                 generateStatusPageSubscriberAuditID(),
		Action:             action,
		StatusPageID:       subscriber.StatusPageID,
		AffectedObjectType: statusPageSubscriberAffectedObjectType,
		AffectedObjectID:   subscriber.ID,
		ActorType:          input.ActorType,
		ActorID:            input.ActorID,
		MetadataJSON:       statusPageSubscriberAuditMetadata(metadata),
		CreatedAt:          time.Now().UTC(),
	}
	return tx.Create(&event).Error
}

func statusPageSubscriberAuditMetadata(metadata map[string]interface{}) string {
	encoded, err := statusPageAuditMetadataJSON(metadata)
	if err != nil {
		return "{}"
	}
	return encoded
}

func sanitizeStatusPageSubscriberAuditMetadataValue(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 500 {
		value = value[:500]
	}
	return value
}

func deletedStatusPageSubscriberDestinationHash(subscriberID string) string {
	return "deleted:" + strings.TrimSpace(subscriberID)
}

func generateStatusPageSubscriberAuditID() string {
	return utils.GenerateID("audit_event")
}
