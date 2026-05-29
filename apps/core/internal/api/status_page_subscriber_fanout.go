package api

import (
	"errors"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"time"

	"gorm.io/gorm"
)

const (
	statusPageSubscriberDeliveryTypeEmail                  = "email"
	statusPageSubscriberDeliveryStatePendingSenderConfig   = "pending_sender_configuration"
	statusPageSubscriberDeliveryErrorPublicSenderMissing   = "public_mail_sender_not_configured"
	statusPageSubscriberDeliverySummaryPublicSenderMissing = "Public status page mail sender is not configured."
)

func (s *Server) enqueueStatusPageSubscriberIncidentUpdateDeliveries(tx *gorm.DB, incident db.StatusPageIncident, update db.StatusPageIncidentUpdate) error {
	if update.PublishedAt == nil {
		return nil
	}

	var page db.StatusPage
	if err := tx.Where("id = ? AND visibility IN ? AND published_at IS NOT NULL", incident.StatusPageID, []string{statusPageVisibilityPublic, statusPageVisibilityUnlisted}).First(&page).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}

	publicAffectedIDs, err := s.publicAffectedStatusPageComponentIDs(tx, incident)
	if err != nil || len(publicAffectedIDs) == 0 {
		return err
	}

	var subscribers []db.StatusPageSubscriber
	if err := tx.Where("status_page_id = ? AND state = ?", page.ID, statusPageSubscriberStateConfirmed).Find(&subscribers).Error; err != nil {
		return err
	}
	if len(subscribers) == 0 {
		return nil
	}

	preferencesBySubscriberID, err := statusPageSubscriberPreferencesBySubscriberID(tx, subscribers)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	affectedSet := stringSet(publicAffectedIDs)
	for _, subscriber := range subscribers {
		if !statusPageSubscriberMatchesAffectedComponents(preferencesBySubscriberID[subscriber.ID], affectedSet) {
			continue
		}
		exists, err := statusPageSubscriberDeliveryExists(tx, subscriber.ID, update.ID)
		if err != nil {
			return err
		}
		if exists {
			continue
		}
		state := statusPageSubscriberDeliveryStateSent
		errorCode := ""
		summary := ""
		sentAt := &now
		failedAt := (*time.Time)(nil)
		attemptCount := 1
		if err := s.ensurePublicStatusMailConfigured(); err != nil {
			state, errorCode, summary = safePublicStatusMailFailure(err)
			sentAt = nil
			if state == statusPageSubscriberDeliveryStateFailed {
				failedAt = &now
			}
			attemptCount = 0
		} else {
			destination, err := s.decryptStatusPageSubscriberDestination(subscriber)
			if err != nil {
				state, errorCode, summary = safePublicStatusMailFailure(err)
				sentAt = nil
				failedAt = &now
			} else {
				unsubscribeToken, err := utils.GenerateToken()
				if err != nil {
					return err
				}
				subscriber.UnsubscribeTokenHash = hashStatusPageSubscriberToken(unsubscribeToken)
				subscriber.UnsubscribeTokenVersion++
				if err := tx.Save(&subscriber).Error; err != nil {
					return err
				}
				if err := s.sendStatusPageIncidentUpdateMail(page, incident, update, subscriber, destination, unsubscribeToken); err != nil {
					state, errorCode, summary = safePublicStatusMailFailure(err)
					sentAt = nil
					failedAt = &now
				}
			}
		}
		delivery := db.StatusPageSubscriberDelivery{
			ID:                     utils.GenerateID("status_page_delivery"),
			SubscriberID:           subscriber.ID,
			StatusPageID:           page.ID,
			PublicIncidentID:       incident.ID,
			PublicIncidentUpdateID: update.ID,
			DeliveryType:           statusPageSubscriberDeliveryTypeEmail,
			DeliveryState:          state,
			ErrorCode:              errorCode,
			SafeErrorSummary:       summary,
			AttemptCount:           attemptCount,
			QueuedAt:               &now,
			SentAt:                 sentAt,
			FailedAt:               failedAt,
		}
		if err := tx.Create(&delivery).Error; err != nil {
			return err
		}
		if err := tx.Model(&db.StatusPageSubscriber{}).
			Where("id = ?", subscriber.ID).
			Updates(map[string]interface{}{
				"last_delivery_status": state,
				"last_delivery_at":     now,
			}).Error; err != nil {
			return err
		}
	}
	return nil
}

func (s *Server) publicAffectedStatusPageComponentIDs(tx *gorm.DB, incident db.StatusPageIncident) ([]string, error) {
	var components []db.StatusPageComponent
	if err := tx.Where("status_page_id = ? AND visible = ?", incident.StatusPageID, true).Find(&components).Error; err != nil {
		return nil, err
	}
	visibleIDs := make([]string, 0, len(components))
	visibleSet := map[string]bool{}
	for _, component := range components {
		visibleIDs = append(visibleIDs, component.ID)
		visibleSet[component.ID] = true
	}

	affectedIDs := decodeResponseList(incident.AffectedComponentIDs, nil)
	if len(affectedIDs) == 0 {
		return visibleIDs, nil
	}

	publicAffectedIDs := make([]string, 0, len(affectedIDs))
	for _, componentID := range affectedIDs {
		if visibleSet[componentID] {
			publicAffectedIDs = append(publicAffectedIDs, componentID)
		}
	}
	return publicAffectedIDs, nil
}

func statusPageSubscriberPreferencesBySubscriberID(tx *gorm.DB, subscribers []db.StatusPageSubscriber) (map[string][]string, error) {
	ids := make([]string, 0, len(subscribers))
	for _, subscriber := range subscribers {
		ids = append(ids, subscriber.ID)
	}

	var preferences []db.StatusPageSubscriberComponent
	if err := tx.Where("subscriber_id IN ?", ids).Find(&preferences).Error; err != nil {
		return nil, err
	}

	preferencesBySubscriberID := map[string][]string{}
	for _, preference := range preferences {
		preferencesBySubscriberID[preference.SubscriberID] = append(preferencesBySubscriberID[preference.SubscriberID], preference.ComponentID)
	}
	return preferencesBySubscriberID, nil
}

func statusPageSubscriberMatchesAffectedComponents(preferences []string, affectedSet map[string]bool) bool {
	if len(preferences) == 0 {
		return len(affectedSet) > 0
	}
	for _, componentID := range preferences {
		if affectedSet[componentID] {
			return true
		}
	}
	return false
}

func statusPageSubscriberDeliveryExists(tx *gorm.DB, subscriberID string, updateID string) (bool, error) {
	var count int64
	if err := tx.Model(&db.StatusPageSubscriberDelivery{}).Where("subscriber_id = ? AND public_incident_update_id = ?", subscriberID, updateID).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func stringSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		set[value] = true
	}
	return set
}
