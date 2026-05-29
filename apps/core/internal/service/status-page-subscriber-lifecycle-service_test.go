package service

import (
	"strings"
	"testing"
	"time"

	"orion/core/internal/db"
	"orion/core/internal/logging"
	"orion/core/internal/utils"

	"gorm.io/gorm"
)

func TestStatusPageSubscriberRetentionCleanupPurgesAndAnonymizesExpiredRows(t *testing.T) {
	database := openArchiveTestDatabase(t)
	now := time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC)
	service := NewStatusPageSubscriberLifecycleService(database, logging.NewLogger())

	expiredPending := insertLifecycleSubscriber(t, database, lifecycleSubscriberSeed{
		ID:              "status_page_subscriber_pending_expired",
		StatusPageID:    "status_page_a",
		State:           statusPageSubscriberStatePending,
		Email:           "pending-expired@example.com",
		ConfirmationDue: ptrTime(now.Add(-8 * 24 * time.Hour)),
		CreatedAt:       now.Add(-10 * 24 * time.Hour),
		UpdatedAt:       now.Add(-10 * 24 * time.Hour),
	})
	insertLifecyclePreference(t, database, expiredPending.ID, "component_a")
	insertLifecycleDelivery(t, database, expiredPending.ID, expiredPending.StatusPageID, now.Add(-2*24*time.Hour), "provider-pending", "pending raw error")

	recentPending := insertLifecycleSubscriber(t, database, lifecycleSubscriberSeed{
		ID:              "status_page_subscriber_pending_recent",
		StatusPageID:    "status_page_a",
		State:           statusPageSubscriberStatePending,
		Email:           "pending-recent@example.com",
		ConfirmationDue: ptrTime(now.Add(-2 * 24 * time.Hour)),
		CreatedAt:       now.Add(-3 * 24 * time.Hour),
		UpdatedAt:       now.Add(-3 * 24 * time.Hour),
	})

	expiredUnsubscribed := insertLifecycleSubscriber(t, database, lifecycleSubscriberSeed{
		ID:             "status_page_subscriber_unsubscribed_expired",
		StatusPageID:   "status_page_a",
		State:          statusPageSubscriberStateUnsubscribed,
		Email:          "expired-user@example.com",
		UnsubscribedAt: ptrTime(now.Add(-91 * 24 * time.Hour)),
		CreatedAt:      now.Add(-120 * 24 * time.Hour),
		UpdatedAt:      now.Add(-91 * 24 * time.Hour),
	})
	insertLifecyclePreference(t, database, expiredUnsubscribed.ID, "component_b")
	insertLifecycleDelivery(t, database, expiredUnsubscribed.ID, expiredUnsubscribed.StatusPageID, now.Add(-20*24*time.Hour), "provider-sensitive", "email expired-user@example.com failed")

	expiredBounced := insertLifecycleSubscriber(t, database, lifecycleSubscriberSeed{
		ID:           "status_page_subscriber_bounced_expired",
		StatusPageID: "status_page_b",
		State:        statusPageSubscriberStateBounced,
		Email:        "bounced-user@example.com",
		CreatedAt:    now.Add(-140 * 24 * time.Hour),
		UpdatedAt:    now.Add(-92 * 24 * time.Hour),
	})

	confirmed := insertLifecycleSubscriber(t, database, lifecycleSubscriberSeed{
		ID:           "status_page_subscriber_confirmed",
		StatusPageID: "status_page_b",
		State:        "confirmed",
		Email:        "confirmed@example.com",
		CreatedAt:    now.Add(-200 * 24 * time.Hour),
		UpdatedAt:    now.Add(-2 * 24 * time.Hour),
	})
	oldDelivery := insertLifecycleDelivery(t, database, confirmed.ID, confirmed.StatusPageID, now.Add(-181*24*time.Hour), "provider-old", "old delivery summary")

	result, err := service.RunRetentionCleanup(now, StatusPageSubscriberRetentionOptions{})
	if err != nil {
		t.Fatalf("RunRetentionCleanup() error = %v", err)
	}
	if result.PendingSubscribersDeleted != 1 || result.SubscribersAnonymized != 2 || result.SubscriberDeliveriesDeleted != 1 || result.SubscriberDeliveryAuditEvents != 1 {
		t.Fatalf("RunRetentionCleanup() result = %+v, want one pending delete, two anonymized, one delivery purge", result)
	}

	assertLifecycleSubscriberMissing(t, database, expiredPending.ID)
	assertLifecycleSubscriberExists(t, database, recentPending.ID)
	assertLifecycleDeliveryMissing(t, database, oldDelivery.ID)
	assertLifecycleSubscriberAnonymized(t, database, expiredUnsubscribed.ID)
	assertLifecycleSubscriberAnonymized(t, database, expiredBounced.ID)
	assertLifecycleSubscriberPreferencesDeleted(t, database, expiredUnsubscribed.ID)
	assertLifecycleSubscriberDeliveriesRedacted(t, database, expiredUnsubscribed.ID)
	assertLifecycleAuditEventsSafe(t, database, []string{
		"pending-expired@example.com",
		"expired-user@example.com",
		"bounced-user@example.com",
		"confirmed@example.com",
		"provider-sensitive",
		"unsubscribe-token",
		"manage-token",
		"confirmation-token",
	})
}

func TestStatusPageSubscriberLifecycleDirectHardDeleteRecordsAudit(t *testing.T) {
	database := openArchiveTestDatabase(t)
	now := time.Date(2026, 5, 29, 12, 0, 0, 0, time.UTC)
	service := NewStatusPageSubscriberLifecycleService(database, logging.NewLogger())
	subscriber := insertLifecycleSubscriber(t, database, lifecycleSubscriberSeed{
		ID:           "status_page_subscriber_delete_me",
		StatusPageID: "status_page_direct",
		State:        statusPageSubscriberStateUnsubscribed,
		Email:        "delete-me@example.com",
		CreatedAt:    now.Add(-100 * 24 * time.Hour),
		UpdatedAt:    now.Add(-100 * 24 * time.Hour),
	})
	insertLifecyclePreference(t, database, subscriber.ID, "component_direct")
	insertLifecycleDelivery(t, database, subscriber.ID, subscriber.StatusPageID, now.Add(-10*24*time.Hour), "provider-direct", "direct delete summary")

	err := service.HardDeleteSubscriber(StatusPageSubscriberPrivacyActionInput{
		SubscriberID: subscriber.ID,
		ActorType:    "user",
		ActorID:      "admin@example.com",
		Reason:       "verified erasure request",
		Basis:        "operator_privacy_erasure",
	})
	if err != nil {
		t.Fatalf("HardDeleteSubscriber() error = %v", err)
	}

	assertLifecycleSubscriberMissing(t, database, subscriber.ID)
	assertLifecycleSubscriberPreferencesDeleted(t, database, subscriber.ID)
	assertLifecycleSubscriberDeliveryCount(t, database, subscriber.ID, 0)

	var auditEvent db.AuditEvent
	if err := database.Where("action = ? AND affected_object_id = ?", StatusPageAuditActionSubscriberHardDeleted, subscriber.ID).First(&auditEvent).Error; err != nil {
		t.Fatalf("load hard delete audit event: %v", err)
	}
	if auditEvent.ActorType != "user" || auditEvent.ActorID != "admin@example.com" || !strings.Contains(auditEvent.MetadataJSON, "operator_privacy_erasure") {
		t.Fatalf("audit event = %+v, want operator hard delete metadata", auditEvent)
	}
	assertLifecycleAuditEventsSafe(t, database, []string{"delete-me@example.com", "provider-direct"})
}

type lifecycleSubscriberSeed struct {
	ID              string
	StatusPageID    string
	State           string
	Email           string
	ConfirmationDue *time.Time
	UnsubscribedAt  *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func insertLifecycleSubscriber(t *testing.T, database *gorm.DB, seed lifecycleSubscriberSeed) db.StatusPageSubscriber {
	t.Helper()

	subscriber := db.StatusPageSubscriber{
		ID:                         seed.ID,
		StatusPageID:               seed.StatusPageID,
		DestinationType:            "email",
		DestinationHash:            "hash:" + seed.Email,
		DestinationValueCiphertext: "ciphertext:" + seed.Email,
		MaskedDestination:          "m***@" + strings.Split(seed.Email, "@")[1],
		State:                      seed.State,
		ConfirmationTokenHash:      "confirmation-token:" + seed.Email,
		ConfirmationTokenExpiresAt: seed.ConfirmationDue,
		ManageTokenHash:            "manage-token:" + seed.Email,
		ManageTokenVersion:         1,
		UnsubscribeTokenHash:       "unsubscribe-token:" + seed.Email,
		UnsubscribeTokenVersion:    1,
		Source:                     "public_page",
		UnsubscribedAt:             seed.UnsubscribedAt,
		CreatedAt:                  seed.CreatedAt,
		UpdatedAt:                  seed.UpdatedAt,
	}
	if err := database.Create(&subscriber).Error; err != nil {
		t.Fatalf("create subscriber: %v", err)
	}
	return subscriber
}

func insertLifecyclePreference(t *testing.T, database *gorm.DB, subscriberID string, componentID string) {
	t.Helper()

	preference := db.StatusPageSubscriberComponent{
		ID:           utils.GenerateID("status_page_subscriber_component"),
		SubscriberID: subscriberID,
		ComponentID:  componentID,
		EventScope:   "all_updates",
	}
	if err := database.Create(&preference).Error; err != nil {
		t.Fatalf("create subscriber preference: %v", err)
	}
}

func insertLifecycleDelivery(t *testing.T, database *gorm.DB, subscriberID string, pageID string, createdAt time.Time, providerID string, summary string) db.StatusPageSubscriberDelivery {
	t.Helper()

	delivery := db.StatusPageSubscriberDelivery{
		ID:                utils.GenerateID("status_page_delivery"),
		SubscriberID:      subscriberID,
		StatusPageID:      pageID,
		DeliveryType:      "email",
		DeliveryState:     "failed",
		ProviderMessageID: providerID,
		ErrorCode:         "provider_error",
		SafeErrorSummary:  summary,
		AttemptCount:      1,
		CreatedAt:         createdAt,
		UpdatedAt:         createdAt,
	}
	if err := database.Create(&delivery).Error; err != nil {
		t.Fatalf("create subscriber delivery: %v", err)
	}
	return delivery
}

func assertLifecycleSubscriberMissing(t *testing.T, database *gorm.DB, subscriberID string) {
	t.Helper()

	var count int64
	if err := database.Model(&db.StatusPageSubscriber{}).Where("id = ?", subscriberID).Count(&count).Error; err != nil {
		t.Fatalf("count subscriber: %v", err)
	}
	if count != 0 {
		t.Fatalf("subscriber %q count = %d, want missing", subscriberID, count)
	}
}

func assertLifecycleSubscriberExists(t *testing.T, database *gorm.DB, subscriberID string) {
	t.Helper()

	var count int64
	if err := database.Model(&db.StatusPageSubscriber{}).Where("id = ?", subscriberID).Count(&count).Error; err != nil {
		t.Fatalf("count subscriber: %v", err)
	}
	if count != 1 {
		t.Fatalf("subscriber %q count = %d, want present", subscriberID, count)
	}
}

func assertLifecycleSubscriberAnonymized(t *testing.T, database *gorm.DB, subscriberID string) {
	t.Helper()

	var subscriber db.StatusPageSubscriber
	if err := database.Where("id = ?", subscriberID).First(&subscriber).Error; err != nil {
		t.Fatalf("load anonymized subscriber: %v", err)
	}
	if subscriber.DestinationValueCiphertext != "" || subscriber.ConfirmationTokenHash != "" || subscriber.ManageTokenHash != "" || subscriber.UnsubscribeTokenHash != "" {
		t.Fatalf("subscriber = %+v, want destination and token hashes cleared", subscriber)
	}
	if subscriber.DestinationHash != deletedStatusPageSubscriberDestinationHash(subscriberID) || subscriber.MaskedDestination != statusPageSubscriberDeletedMaskedDestination || subscriber.State != statusPageSubscriberStateDisabled {
		t.Fatalf("subscriber = %+v, want anonymized disabled subscriber", subscriber)
	}
}

func assertLifecycleSubscriberPreferencesDeleted(t *testing.T, database *gorm.DB, subscriberID string) {
	t.Helper()

	var count int64
	if err := database.Model(&db.StatusPageSubscriberComponent{}).Where("subscriber_id = ?", subscriberID).Count(&count).Error; err != nil {
		t.Fatalf("count preferences: %v", err)
	}
	if count != 0 {
		t.Fatalf("subscriber preference count = %d, want zero", count)
	}
}

func assertLifecycleSubscriberDeliveryCount(t *testing.T, database *gorm.DB, subscriberID string, expected int64) {
	t.Helper()

	var count int64
	if err := database.Model(&db.StatusPageSubscriberDelivery{}).Where("subscriber_id = ?", subscriberID).Count(&count).Error; err != nil {
		t.Fatalf("count deliveries: %v", err)
	}
	if count != expected {
		t.Fatalf("subscriber delivery count = %d, want %d", count, expected)
	}
}

func assertLifecycleSubscriberDeliveriesRedacted(t *testing.T, database *gorm.DB, subscriberID string) {
	t.Helper()

	var deliveries []db.StatusPageSubscriberDelivery
	if err := database.Where("subscriber_id = ?", subscriberID).Find(&deliveries).Error; err != nil {
		t.Fatalf("load deliveries: %v", err)
	}
	if len(deliveries) == 0 {
		t.Fatal("expected redacted delivery rows to remain")
	}
	for _, delivery := range deliveries {
		if delivery.ProviderMessageID != "" || delivery.ErrorCode != "" || delivery.SafeErrorSummary != "" {
			t.Fatalf("delivery = %+v, want provider and error fields redacted", delivery)
		}
	}
}

func assertLifecycleDeliveryMissing(t *testing.T, database *gorm.DB, deliveryID string) {
	t.Helper()

	var count int64
	if err := database.Model(&db.StatusPageSubscriberDelivery{}).Where("id = ?", deliveryID).Count(&count).Error; err != nil {
		t.Fatalf("count delivery: %v", err)
	}
	if count != 0 {
		t.Fatalf("delivery %q count = %d, want missing", deliveryID, count)
	}
}

func assertLifecycleAuditEventsSafe(t *testing.T, database *gorm.DB, forbidden []string) {
	t.Helper()

	var auditEvents []db.AuditEvent
	if err := database.Find(&auditEvents).Error; err != nil {
		t.Fatalf("load audit events: %v", err)
	}
	if len(auditEvents) == 0 {
		t.Fatal("expected audit events")
	}
	for _, event := range auditEvents {
		for _, value := range forbidden {
			if value != "" && strings.Contains(event.MetadataJSON, value) {
				t.Fatalf("audit metadata %q contains forbidden value %q", event.MetadataJSON, value)
			}
		}
	}
}

func ptrTime(value time.Time) *time.Time {
	return &value
}
