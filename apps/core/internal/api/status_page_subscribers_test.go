package api

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"strings"
	"testing"
	"time"
)

func TestPublicStatusPageSubscriptionRequestStoresHashesAndMasksDestination(t *testing.T) {
	server := setupTestServer(t)
	page, visibleComponent, _ := createPublishedStatusPageForSubscriberTest(t, server, "subscriber-status")
	rawDestination := "Alice.Observer@example.com"
	resp := performJSONRequest(t, server, http.MethodPost, "/status/"+page.Slug+"/subscribers", gin.H{"destination": rawDestination, "component_ids": []string{visibleComponent.ID}}, "")
	if resp.Code != http.StatusAccepted {
		t.Fatalf("subscription request status = %d, body = %s", resp.Code, resp.Body.String())
	}
	body := resp.Body.String()
	assertContains(t, body, `"state":"pending"`)
	assertContains(t, body, `"masked_destination":"a***@example.com"`)
	assertNotContains(t, body, rawDestination)
	assertNotContains(t, body, "Alice.Observer")
	assertNotContains(t, body, "confirmation_token")
	assertNotContains(t, body, "manage_token")
	assertNotContains(t, body, "unsubscribe_token")
	assertNotContains(t, body, "token_hash")
	var subscriber db.StatusPageSubscriber
	if err := server.db.Where("status_page_id = ?", page.ID).First(&subscriber).Error; err != nil {
		t.Fatalf("load subscriber: %v", err)
	}
	if subscriber.State != statusPageSubscriberStatePending {
		t.Fatalf("subscriber state = %q, want pending", subscriber.State)
	}
	if subscriber.DestinationHash == "" || subscriber.DestinationHash == rawDestination {
		t.Fatalf("destination hash = %q, want non-raw hash", subscriber.DestinationHash)
	}
	if subscriber.DestinationValueCiphertext != "" {
		t.Fatalf("destination value ciphertext = %q, want empty until encrypted storage is implemented", subscriber.DestinationValueCiphertext)
	}
	for name, value := range map[string]string{"confirmation": subscriber.ConfirmationTokenHash, "manage": subscriber.ManageTokenHash, "unsubscribe": subscriber.UnsubscribeTokenHash} {
		if value == "" || value == rawDestination {
			t.Fatalf("%s token hash = %q, want non-raw hash", name, value)
		}
	}
	var preferences []db.StatusPageSubscriberComponent
	if err := server.db.Where("subscriber_id = ?", subscriber.ID).Find(&preferences).Error; err != nil {
		t.Fatalf("load preferences: %v", err)
	}
	if len(preferences) != 1 || preferences[0].ComponentID != visibleComponent.ID {
		t.Fatalf("preferences = %+v, want only visible component %s", preferences, visibleComponent.ID)
	}
}
func TestPublicStatusPageSubscriptionSendsConfirmationWithConfiguredPublicSender(t *testing.T) {
	server := setupTestServer(t)
	configurePublicStatusMailForTest(server)
	var messages []publicStatusMailMessage
	server.publicStatusMailSend = func(message publicStatusMailMessage) error {
		messages = append(messages, message)
		return nil
	}
	page, visibleComponent, _ := createPublishedStatusPageForSubscriberTest(t, server, "configured-subscriber-status")
	if err := server.db.Model(&db.StatusPage{}).Where("id = ?", page.ID).Update("custom_domain", "status.customer.example").Error; err != nil {
		t.Fatalf("set custom domain: %v", err)
	}
	page.CustomDomain = "status.customer.example"
	rawDestination := "Configured.User@example.com"
	resp := performJSONRequest(t, server, http.MethodPost, "/status/"+page.Slug+"/subscribers", gin.H{"destination": rawDestination, "component_ids": []string{visibleComponent.ID}}, "")
	if resp.Code != http.StatusAccepted {
		t.Fatalf("subscription request status = %d, body = %s", resp.Code, resp.Body.String())
	}
	body := resp.Body.String()
	assertContains(t, body, `"confirmation_delivery":"sent"`)
	assertNotContains(t, body, rawDestination)
	assertNotContains(t, body, "confirm/")
	assertNotContains(t, body, "token")
	if len(messages) != 1 {
		t.Fatalf("messages len = %d, want 1", len(messages))
	}
	if messages[0].To != "configured.user@example.com" {
		t.Fatalf("message recipient = %q, want normalized destination", messages[0].To)
	}
	assertContains(t, messages[0].Text, "https://status.customer.example/status/"+page.Slug+"/subscribers/confirm/")
	assertNotContains(t, messages[0].Text, server.cfg.PublicStatusMailPassword)
	var subscriber db.StatusPageSubscriber
	if err := server.db.Where("status_page_id = ?", page.ID).First(&subscriber).Error; err != nil {
		t.Fatalf("load subscriber: %v", err)
	}
	if subscriber.DestinationValueCiphertext == "" || strings.Contains(subscriber.DestinationValueCiphertext, "configured.user") {
		t.Fatalf("destination ciphertext = %q, want encrypted non-raw value", subscriber.DestinationValueCiphertext)
	}
	decrypted, err := server.decryptStatusPageSubscriberDestination(subscriber)
	if err != nil {
		t.Fatalf("decrypt destination: %v", err)
	}
	if decrypted != "configured.user@example.com" {
		t.Fatalf("decrypted destination = %q", decrypted)
	}
	var delivery db.StatusPageSubscriberDelivery
	if err := server.db.Where("subscriber_id = ?", subscriber.ID).First(&delivery).Error; err != nil {
		t.Fatalf("load confirmation delivery: %v", err)
	}
	if delivery.DeliveryState != statusPageSubscriberDeliveryStateSent || delivery.SentAt == nil || delivery.ErrorCode != "" || delivery.PublicIncidentID != "" {
		t.Fatalf("delivery = %+v, want sent confirmation ledger row", delivery)
	}
}
func TestPublicStatusPageSubscriberConfirmationUsesOneTimeHashedToken(t *testing.T) {
	server := setupTestServer(t)
	page, visibleComponent, _ := createPublishedStatusPageForSubscriberTest(t, server, "confirm-status")
	confirmationToken := "confirm-token-for-test"
	subscriber := seedStatusPageSubscriberForTest(t, server, page.ID, "Confirm.User@example.com", statusPageSubscriberStatePending, confirmationToken, "manage-token", "unsubscribe-token", []string{visibleComponent.ID})
	resp := performJSONRequest(t, server, http.MethodGet, "/status/"+page.Slug+"/subscribers/confirm/"+confirmationToken, nil, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("confirm status = %d, body = %s", resp.Code, resp.Body.String())
	}
	body := resp.Body.String()
	assertContains(t, body, `"state":"confirmed"`)
	assertContains(t, body, `"masked_destination":"c***@example.com"`)
	assertNotContains(t, body, confirmationToken)
	assertNotContains(t, body, "Confirm.User")
	assertNotContains(t, body, "token")
	var stored db.StatusPageSubscriber
	if err := server.db.Where("id = ?", subscriber.ID).First(&stored).Error; err != nil {
		t.Fatalf("load confirmed subscriber: %v", err)
	}
	if stored.State != statusPageSubscriberStateConfirmed || stored.ConfirmedAt == nil {
		t.Fatalf("stored subscriber = %+v, want confirmed with timestamp", stored)
	}
	if stored.ConfirmationTokenHash != "" || stored.ConfirmationTokenExpiresAt != nil {
		t.Fatalf("confirmation token was not cleared after use: %+v", stored)
	}
	reuseResp := performJSONRequest(t, server, http.MethodGet, "/status/"+page.Slug+"/subscribers/confirm/"+confirmationToken, nil, "")
	if reuseResp.Code != http.StatusNotFound {
		t.Fatalf("reused confirmation status = %d, body = %s, want 404", reuseResp.Code, reuseResp.Body.String())
	}
}
func TestPublicStatusPageSubscriberConfirmationRejectsInvalidExpiredAndRateLimitedTokens(t *testing.T) {
	server := setupTestServer(t)
	page, visibleComponent, _ := createPublishedStatusPageForSubscriberTest(t, server, "confirm-invalid-status")
	expiredToken := "expired-confirm-token-for-test"
	expiredSubscriber := seedStatusPageSubscriberForTest(t, server, page.ID, "Expired.User@example.com", statusPageSubscriberStatePending, expiredToken, "manage-token", "unsubscribe-token", []string{visibleComponent.ID})
	expiredAt := time.Now().UTC().Add(-time.Minute)
	if err := server.db.Model(&db.StatusPageSubscriber{}).Where("id = ?", expiredSubscriber.ID).Update("confirmation_token_expires_at", expiredAt).Error; err != nil {
		t.Fatalf("expire confirmation token: %v", err)
	}
	invalidResp := performJSONRequest(t, server, http.MethodGet, "/status/"+page.Slug+"/subscribers/confirm/not-a-real-token", nil, "")
	if invalidResp.Code != http.StatusNotFound {
		t.Fatalf("invalid confirmation status = %d, body = %s, want 404", invalidResp.Code, invalidResp.Body.String())
	}
	assertNotContains(t, invalidResp.Body.String(), "not-a-real-token")
	expiredResp := performJSONRequest(t, server, http.MethodGet, "/status/"+page.Slug+"/subscribers/confirm/"+expiredToken, nil, "")
	if expiredResp.Code != http.StatusNotFound {
		t.Fatalf("expired confirmation status = %d, body = %s, want 404", expiredResp.Code, expiredResp.Body.String())
	}
	assertNotContains(t, expiredResp.Body.String(), expiredToken)
	limitedServer := setupTestServer(t)
	limitedPage, _, _ := createPublishedStatusPageForSubscriberTest(t, limitedServer, "confirm-rate-limit-status")
	limitedServer.publicSubscriberLimiter = NewRateLimiter(1, time.Hour)
	firstResp := performJSONRequest(t, limitedServer, http.MethodGet, "/status/"+limitedPage.Slug+"/subscribers/confirm/not-a-real-token", nil, "")
	if firstResp.Code != http.StatusNotFound {
		t.Fatalf("first limited confirmation status = %d, body = %s, want 404", firstResp.Code, firstResp.Body.String())
	}
	secondResp := performJSONRequest(t, limitedServer, http.MethodGet, "/status/"+limitedPage.Slug+"/subscribers/confirm/not-a-real-token", nil, "")
	if secondResp.Code != http.StatusTooManyRequests {
		t.Fatalf("second limited confirmation status = %d, body = %s, want 429", secondResp.Code, secondResp.Body.String())
	}
	assertNotContains(t, secondResp.Body.String(), "not-a-real-token")
}
func TestPublicStatusPageSubscriberSelfServiceEndpointsAreRateLimited(t *testing.T) {
	cases := []struct {
		name          string
		method        string
		path          func(string) string
		body          gin.H
		firstWantCode int
	}{{name: "create", method: http.MethodPost, path: func(slug string) string {
		return "/status/" + slug + "/subscribers"
	}, body: gin.H{}, firstWantCode: http.StatusBadRequest}, {name: "confirm", method: http.MethodGet, path: func(slug string) string {
		return "/status/" + slug + "/subscribers/confirm/not-a-real-token"
	}, firstWantCode: http.StatusNotFound}, {name: "manage-get", method: http.MethodGet, path: func(slug string) string {
		return "/status/" + slug + "/subscribers/manage/not-a-real-token"
	}, firstWantCode: http.StatusNotFound}, {name: "manage-put", method: http.MethodPut, path: func(slug string) string {
		return "/status/" + slug + "/subscribers/manage/not-a-real-token"
	}, body: gin.H{}, firstWantCode: http.StatusNotFound}, {name: "unsubscribe", method: http.MethodPost, path: func(slug string) string {
		return "/status/" + slug + "/subscribers/unsubscribe/not-a-real-token"
	}, firstWantCode: http.StatusOK}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := setupTestServer(t)
			page, _, _ := createPublishedStatusPageForSubscriberTest(t, server, "rate-limit-"+tc.name)
			server.publicSubscriberLimiter = NewRateLimiter(1, time.Hour)
			firstResp := performJSONRequest(t, server, tc.method, tc.path(page.Slug), tc.body, "")
			if firstResp.Code != tc.firstWantCode {
				t.Fatalf("first %s status = %d, body = %s, want %d", tc.name, firstResp.Code, firstResp.Body.String(), tc.firstWantCode)
			}
			secondResp := performJSONRequest(t, server, tc.method, tc.path(page.Slug), tc.body, "")
			if secondResp.Code != http.StatusTooManyRequests {
				t.Fatalf("second %s status = %d, body = %s, want 429", tc.name, secondResp.Code, secondResp.Body.String())
			}
			assertContains(t, secondResp.Body.String(), "Too many status page subscriber requests")
			assertNotContains(t, secondResp.Body.String(), "not-a-real-token")
		})
	}
}
func TestPublicStatusPageSubscriberUnsubscribeIsIdempotent(t *testing.T) {
	server := setupTestServer(t)
	page, visibleComponent, _ := createPublishedStatusPageForSubscriberTest(t, server, "unsubscribe-status")
	unsubscribeToken := "unsubscribe-token-for-test"
	subscriber := seedStatusPageSubscriberForTest(t, server, page.ID, "Unsub.User@example.com", statusPageSubscriberStateConfirmed, "confirm-token", "manage-token", unsubscribeToken, []string{visibleComponent.ID})
	for i := 0; i < 2; i++ {
		resp := performJSONRequest(t, server, http.MethodPost, "/status/"+page.Slug+"/subscribers/unsubscribe/"+unsubscribeToken, nil, "")
		if resp.Code != http.StatusOK {
			t.Fatalf("unsubscribe attempt %d status = %d, body = %s", i+1, resp.Code, resp.Body.String())
		}
		assertContains(t, resp.Body.String(), `"unsubscribed":true`)
		assertNotContains(t, resp.Body.String(), unsubscribeToken)
	}
	invalidResp := performJSONRequest(t, server, http.MethodPost, "/status/"+page.Slug+"/subscribers/unsubscribe/not-a-real-token", nil, "")
	if invalidResp.Code != http.StatusOK {
		t.Fatalf("invalid unsubscribe status = %d, body = %s, want generic success", invalidResp.Code, invalidResp.Body.String())
	}
	var stored db.StatusPageSubscriber
	if err := server.db.Where("id = ?", subscriber.ID).First(&stored).Error; err != nil {
		t.Fatalf("load unsubscribed subscriber: %v", err)
	}
	if stored.State != statusPageSubscriberStateUnsubscribed || stored.UnsubscribedAt == nil {
		t.Fatalf("stored subscriber = %+v, want unsubscribed with timestamp", stored)
	}
	var auditEvents []db.AuditEvent
	if err := server.db.Where("action = ? AND affected_object_id = ?", service.StatusPageAuditActionSubscriberUnsubscribed, subscriber.ID).Find(&auditEvents).Error; err != nil {
		t.Fatalf("load unsubscribe audit events: %v", err)
	}
	if len(auditEvents) != 1 {
		t.Fatalf("unsubscribe audit event count = %d, want one", len(auditEvents))
	}
	if !strings.Contains(auditEvents[0].MetadataJSON, `"previous_state":"confirmed"`) {
		t.Fatalf("unsubscribe audit metadata = %q, want previous confirmed state", auditEvents[0].MetadataJSON)
	}
	assertNotContains(t, auditEvents[0].MetadataJSON, "Unsub.User")
	assertNotContains(t, auditEvents[0].MetadataJSON, unsubscribeToken)
}
func TestPublicStatusPageSubscriberPreferencesSuppressHiddenComponents(t *testing.T) {
	server := setupTestServer(t)
	page, visibleComponent, hiddenComponent := createPublishedStatusPageForSubscriberTest(t, server, "preferences-status")
	manageToken := "manage-token-for-test"
	seedStatusPageSubscriberForTest(t, server, page.ID, "Prefs.User@example.com", statusPageSubscriberStateConfirmed, "confirm-token", manageToken, "unsubscribe-token", []string{visibleComponent.ID, hiddenComponent.ID})
	resp := performJSONRequest(t, server, http.MethodGet, "/status/"+page.Slug+"/subscribers/manage/"+manageToken, nil, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("manage status = %d, body = %s", resp.Code, resp.Body.String())
	}
	body := resp.Body.String()
	assertContains(t, body, visibleComponent.ID)
	assertContains(t, body, "Visible API")
	assertNotContains(t, body, hiddenComponent.ID)
	assertNotContains(t, body, "Hidden Database")
	assertNotContains(t, body, "Prefs.User")
	assertNotContains(t, body, "token")
	updateResp := performJSONRequest(t, server, http.MethodPut, "/status/"+page.Slug+"/subscribers/manage/"+manageToken, gin.H{"component_ids": []string{hiddenComponent.ID}}, "")
	if updateResp.Code != http.StatusBadRequest {
		t.Fatalf("hidden component preference status = %d, body = %s, want 400", updateResp.Code, updateResp.Body.String())
	}
	assertContains(t, updateResp.Body.String(), "visible components")
}
func TestStatusPageSubscriberAdminListReturnsRedactedSubscribers(t *testing.T) {
	server := setupTestServer(t)
	page, visibleComponent, _ := createPublishedStatusPageForSubscriberTest(t, server, "admin-list-status")
	rawDestination := "Admin.List@example.com"
	subscriber := seedStatusPageSubscriberForTest(t, server, page.ID, rawDestination, statusPageSubscriberStateConfirmed, "admin-confirm-token", "admin-manage-token", "admin-unsubscribe-token", []string{visibleComponent.ID})
	lastDeliveryAt := time.Now().UTC()
	if err := server.db.Model(&db.StatusPageSubscriber{}).Where("id = ?", subscriber.ID).Updates(map[string]interface{}{"last_delivery_status": statusPageSubscriberDeliveryStateSent, "last_delivery_at": lastDeliveryAt, "bounce_count": 2}).Error; err != nil {
		t.Fatalf("update subscriber delivery state: %v", err)
	}
	resp := performJSONRequest(t, server, http.MethodGet, "/v1/status-pages/"+page.ID+"/subscribers", nil, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("admin list status = %d, body = %s", resp.Code, resp.Body.String())
	}
	body := resp.Body.String()
	assertContains(t, body, subscriber.ID)
	assertContains(t, body, `"masked_destination":"a***@example.com"`)
	assertContains(t, body, `"state":"confirmed"`)
	assertContains(t, body, `"bounce_count":2`)
	assertContains(t, body, visibleComponent.ID)
	assertContains(t, body, "Visible API")
	assertStatusPageSubscriberAdminResponseRedacted(t, body, rawDestination, subscriber)
	filteredResp := performJSONRequest(t, server, http.MethodGet, "/v1/status-pages/"+page.ID+"/subscribers?state=disabled", nil, "")
	if filteredResp.Code != http.StatusOK {
		t.Fatalf("filtered admin list status = %d, body = %s", filteredResp.Code, filteredResp.Body.String())
	}
	assertContains(t, filteredResp.Body.String(), `"count":0`)
	assertNotContains(t, filteredResp.Body.String(), subscriber.ID)
}
func TestDisableStatusPageSubscriberClearsTokensAndStopsPublicManagement(t *testing.T) {
	server := setupTestServer(t)
	page, visibleComponent, _ := createPublishedStatusPageForSubscriberTest(t, server, "admin-disable-status")
	manageToken := "disable-manage-token"
	unsubscribeToken := "disable-unsubscribe-token"
	subscriber := seedStatusPageSubscriberForTest(t, server, page.ID, "Disable.User@example.com", statusPageSubscriberStateConfirmed, "disable-confirm-token", manageToken, unsubscribeToken, []string{visibleComponent.ID})
	resp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+page.ID+"/subscribers/"+subscriber.ID+"/disable", nil, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("disable subscriber status = %d, body = %s", resp.Code, resp.Body.String())
	}
	body := resp.Body.String()
	assertContains(t, body, `"state":"disabled"`)
	assertContains(t, body, `"disabled_at"`)
	assertStatusPageSubscriberAdminResponseRedacted(t, body, "Disable.User@example.com", subscriber)
	var stored db.StatusPageSubscriber
	if err := server.db.Where("id = ?", subscriber.ID).First(&stored).Error; err != nil {
		t.Fatalf("load disabled subscriber: %v", err)
	}
	if stored.State != statusPageSubscriberStateDisabled || stored.DisabledAt == nil {
		t.Fatalf("stored subscriber = %+v, want disabled with timestamp", stored)
	}
	if stored.ConfirmationTokenHash != "" || stored.ConfirmationTokenExpiresAt != nil || stored.ManageTokenHash != "" || stored.UnsubscribeTokenHash != "" {
		t.Fatalf("stored subscriber retains token hashes after disable: %+v", stored)
	}
	if stored.ManageTokenVersion != 2 || stored.UnsubscribeTokenVersion != 2 {
		t.Fatalf("token versions = %d/%d, want rotated to 2/2", stored.ManageTokenVersion, stored.UnsubscribeTokenVersion)
	}
	manageResp := performJSONRequest(t, server, http.MethodGet, "/status/"+page.Slug+"/subscribers/manage/"+manageToken, nil, "")
	if manageResp.Code != http.StatusNotFound {
		t.Fatalf("manage disabled subscriber status = %d, body = %s, want 404", manageResp.Code, manageResp.Body.String())
	}
	unsubscribeResp := performJSONRequest(t, server, http.MethodPost, "/status/"+page.Slug+"/subscribers/unsubscribe/"+unsubscribeToken, nil, "")
	if unsubscribeResp.Code != http.StatusOK {
		t.Fatalf("unsubscribe disabled subscriber status = %d, body = %s, want generic success", unsubscribeResp.Code, unsubscribeResp.Body.String())
	}
}
func TestAnonymizeStatusPageSubscriberRemovesContactDataAndPreferences(t *testing.T) {
	server := setupTestServer(t)
	page, visibleComponent, _ := createPublishedStatusPageForSubscriberTest(t, server, "admin-anonymize-status")
	rawDestination := "Anon.User@example.com"
	subscriber := seedStatusPageSubscriberForTest(t, server, page.ID, rawDestination, statusPageSubscriberStateConfirmed, "anon-confirm-token", "anon-manage-token", "anon-unsubscribe-token", []string{visibleComponent.ID})
	resp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+page.ID+"/subscribers/"+subscriber.ID+"/anonymize", nil, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("anonymize subscriber status = %d, body = %s", resp.Code, resp.Body.String())
	}
	body := resp.Body.String()
	assertContains(t, body, `"masked_destination":"anonymized"`)
	assertContains(t, body, `"state":"disabled"`)
	assertContains(t, body, `"components":[]`)
	assertStatusPageSubscriberAdminResponseRedacted(t, body, rawDestination, subscriber)
	var stored db.StatusPageSubscriber
	if err := server.db.Where("id = ?", subscriber.ID).First(&stored).Error; err != nil {
		t.Fatalf("load anonymized subscriber: %v", err)
	}
	if stored.MaskedDestination != "anonymized" || stored.DestinationValueCiphertext != "" || stored.State != statusPageSubscriberStateDisabled {
		t.Fatalf("stored subscriber = %+v, want anonymized disabled tombstone", stored)
	}
	if stored.DestinationHash == subscriber.DestinationHash || strings.Contains(stored.DestinationHash, "anon") {
		t.Fatalf("destination hash = %q, want replaced anonymous hash", stored.DestinationHash)
	}
	if stored.ConfirmationTokenHash != "" || stored.ManageTokenHash != "" || stored.UnsubscribeTokenHash != "" {
		t.Fatalf("stored subscriber retains token hashes after anonymize: %+v", stored)
	}
	var preferenceCount int64
	if err := server.db.Model(&db.StatusPageSubscriberComponent{}).Where("subscriber_id = ?", subscriber.ID).Count(&preferenceCount).Error; err != nil {
		t.Fatalf("count preferences: %v", err)
	}
	if preferenceCount != 0 {
		t.Fatalf("preference count = %d, want 0", preferenceCount)
	}
}
