package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"time"

	"orion/core/internal/db"

	"gorm.io/gorm"
)

// encryptSubscriberEmail mirrors Core's subscriber encryption so operators see real addresses when the
// seed runs with the same ORION_PUBLIC_STATUS_SUBSCRIBER_SECRET as Core. Without it the column stays empty
// and the admin screens show the masked address.
func encryptSubscriberEmail(email string) string {
	secret := os.Getenv("ORION_PUBLIC_STATUS_SUBSCRIBER_SECRET")
	if secret == "" {
		return ""
	}
	key := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return ""
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return ""
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(append(nonce, gcm.Seal(nil, nonce, []byte(email), nil)...))
}

func maskedEmail(email string) string {
	for i, r := range email {
		if r == '@' {
			return email[:1] + "***" + email[i:]
		}
	}
	return "***"
}

type subscriberSpec struct {
	key, email, state string
	components        []string
	bounces           int
	lastDelivery      string // sent | failed
	ageDays           int
	expired           bool
}

func seedShowcaseStatusSubscribers(database *gorm.DB, now time.Time) error {
	specs := []subscriberSpec{
		{key: "ada", email: "ada@acme-customer.test", state: "confirmed", lastDelivery: "sent", ageDays: 40},
		{key: "ops", email: "ops@partner.test", state: "confirmed", components: []string{"api"}, lastDelivery: "sent", ageDays: 25},
		{key: "cto", email: "cto@bigcorp.test", state: "confirmed", components: []string{"api", "database", "payments"}, lastDelivery: "sent", ageDays: 12},
		{key: "pending", email: "new-user@example.test", state: "pending", ageDays: 0},
		{key: "expired", email: "slow-clicker@example.test", state: "pending", ageDays: 6, expired: true},
		{key: "unsub", email: "gone@example.test", state: "unsubscribed", ageDays: 60},
		{key: "disabled", email: "blocked@example.test", state: "disabled", ageDays: 90},
		{key: "bounced", email: "typo@exmaple.test", state: "bounced", bounces: 3, lastDelivery: "failed", ageDays: 30},
	}
	for _, sp := range specs {
		created := now.AddDate(0, 0, -sp.ageDays)
		sub := db.StatusPageSubscriber{ID: "seed-state-sub-" + sp.key, StatusPageID: acmePageID, DestinationType: "email", DestinationHash: "seed-state-hash-" + sp.key, DestinationValueCiphertext: encryptSubscriberEmail(sp.email),
			MaskedDestination: maskedEmail(sp.email), State: sp.state, ManageTokenHash: sha256Hex("manage-" + sp.key), ManageTokenVersion: 1, UnsubscribeTokenHash: sha256Hex("unsub-" + sp.key), UnsubscribeTokenVersion: 1,
			BounceCount: sp.bounces, Source: "public_page", CreatedAt: created, UpdatedAt: created}
		switch sp.state {
		case "confirmed", "bounced":
			sub.ConfirmedAt = ptrTime(created.Add(5 * time.Minute))
		case "unsubscribed":
			sub.ConfirmedAt, sub.UnsubscribedAt = ptrTime(created.Add(time.Minute)), ptrTime(now.AddDate(0, 0, -5))
		case "disabled":
			sub.ConfirmedAt, sub.DisabledAt = ptrTime(created.Add(time.Minute)), ptrTime(now.AddDate(0, 0, -8))
		case "pending":
			expires := now.Add(22 * time.Hour)
			if sp.expired {
				expires = now.AddDate(0, 0, -5)
			}
			sub.ConfirmationTokenHash, sub.ConfirmationTokenExpiresAt = sha256Hex("confirm-"+sp.key), &expires
		}
		if sp.lastDelivery != "" {
			sub.LastDeliveryStatus, sub.LastDeliveryAt = sp.lastDelivery, ptrTime(now.Add(-70*time.Minute))
		}
		if err := database.Create(&sub).Error; err != nil {
			return err
		}
		for i, comp := range sp.components {
			pref := db.StatusPageSubscriberComponent{ID: fmt.Sprintf("seed-state-subpref-%s-%d", sp.key, i), SubscriberID: sub.ID, ComponentID: "seed-state-component-" + comp, EventScope: "all_updates", CreatedAt: created, UpdatedAt: created}
			if err := database.Create(&pref).Error; err != nil {
				return err
			}
		}
		if sp.lastDelivery != "" {
			d := db.StatusPageSubscriberDelivery{ID: "seed-state-subdelivery-" + sp.key, SubscriberID: sub.ID, StatusPageID: acmePageID, PublicIncidentID: "seed-state-pubinc-payments", PublicIncidentUpdateID: "seed-state-pubupd-payments-2",
				DeliveryType: "email", DeliveryState: sp.lastDelivery, AttemptCount: 1, QueuedAt: ptrTime(now.Add(-71 * time.Minute)), CreatedAt: now.Add(-71 * time.Minute), UpdatedAt: now.Add(-70 * time.Minute)}
			if sp.lastDelivery == "sent" {
				d.SentAt, d.ProviderMessageID = ptrTime(now.Add(-70*time.Minute)), "seed-msg-"+sp.key
			} else {
				d.FailedAt, d.ErrorCode, d.SafeErrorSummary = ptrTime(now.Add(-70*time.Minute)), "mailbox_unavailable", "The mailbox does not exist."
			}
			if err := database.Create(&d).Error; err != nil {
				return err
			}
		}
	}
	return nil
}
