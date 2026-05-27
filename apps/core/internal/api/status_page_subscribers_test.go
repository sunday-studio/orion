package api

import (
	"net/http"
	"testing"
	"time"

	"orion/core/internal/db"
	"orion/core/internal/utils"

	"github.com/gin-gonic/gin"
)

func TestPublicStatusPageSubscriptionRequestStoresHashesAndMasksDestination(t *testing.T) {
	server := setupTestServer(t)
	page, visibleComponent, _ := createPublishedStatusPageForSubscriberTest(t, server, "subscriber-status")

	rawDestination := "Alice.Observer@example.com"
	resp := performJSONRequest(t, server, http.MethodPost, "/status/"+page.Slug+"/subscribers", gin.H{
		"destination":   rawDestination,
		"component_ids": []string{visibleComponent.ID},
	}, "")
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
	for name, value := range map[string]string{
		"confirmation": subscriber.ConfirmationTokenHash,
		"manage":       subscriber.ManageTokenHash,
		"unsubscribe":  subscriber.UnsubscribeTokenHash,
	} {
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

	updateResp := performJSONRequest(t, server, http.MethodPut, "/status/"+page.Slug+"/subscribers/manage/"+manageToken, gin.H{
		"component_ids": []string{hiddenComponent.ID},
	}, "")
	if updateResp.Code != http.StatusBadRequest {
		t.Fatalf("hidden component preference status = %d, body = %s, want 400", updateResp.Code, updateResp.Body.String())
	}
	assertContains(t, updateResp.Body.String(), "visible components")
}

func createPublishedStatusPageForSubscriberTest(t *testing.T, server *Server, slug string) (db.StatusPage, db.StatusPageComponent, db.StatusPageComponent) {
	t.Helper()
	now := time.Now().UTC()
	page := db.StatusPage{
		ID:                        utils.GenerateID("status_page"),
		Slug:                      slug,
		Title:                     "Subscriber Test Status",
		Visibility:                statusPageVisibilityPublic,
		ThemeSettings:             "{}",
		DefaultIncidentVisibility: statusPageIncidentVisibilityDraft,
		PublishedAt:               &now,
	}
	if err := server.db.Create(&page).Error; err != nil {
		t.Fatalf("create status page: %v", err)
	}
	section := db.StatusPageSection{
		ID:           utils.GenerateID("status_page_section"),
		StatusPageID: page.ID,
		Name:         "Services",
	}
	if err := server.db.Create(&section).Error; err != nil {
		t.Fatalf("create section: %v", err)
	}
	visibleComponent := db.StatusPageComponent{
		ID:           utils.GenerateID("status_page_component"),
		StatusPageID: page.ID,
		SectionID:    section.ID,
		PublicName:   "Visible API",
		DisplayMode:  "manual",
		ManualStatus: "operational",
		Visible:      true,
	}
	hiddenComponent := db.StatusPageComponent{
		ID:           utils.GenerateID("status_page_component"),
		StatusPageID: page.ID,
		SectionID:    section.ID,
		PublicName:   "Hidden Database",
		DisplayMode:  "manual",
		ManualStatus: "operational",
		Visible:      false,
	}
	if err := server.db.Create(&visibleComponent).Error; err != nil {
		t.Fatalf("create visible component: %v", err)
	}
	if err := server.db.Create(&hiddenComponent).Error; err != nil {
		t.Fatalf("create hidden component: %v", err)
	}
	if err := server.db.Model(&db.StatusPageComponent{}).Where("id = ?", hiddenComponent.ID).Update("visible", false).Error; err != nil {
		t.Fatalf("hide component: %v", err)
	}
	hiddenComponent.Visible = false
	return page, visibleComponent, hiddenComponent
}

func seedStatusPageSubscriberForTest(t *testing.T, server *Server, pageID string, destination string, state string, confirmationToken string, manageToken string, unsubscribeToken string, componentIDs []string) db.StatusPageSubscriber {
	t.Helper()
	destinationTypeInput := statusPageSubscriberDestinationEmail
	destinationType, normalizedDestination, maskedDestination, err := normalizeStatusPageSubscriberDestination(&destinationTypeInput, &destination)
	if err != nil {
		t.Fatalf("normalize destination: %v", err)
	}
	expiresAt := time.Now().UTC().Add(time.Hour)
	subscriber := db.StatusPageSubscriber{
		ID:                         utils.GenerateID("status_page_subscriber"),
		StatusPageID:               pageID,
		DestinationType:            destinationType,
		DestinationHash:            hashStatusPageSubscriberValue(destinationType + ":" + normalizedDestination),
		DestinationValueCiphertext: "",
		MaskedDestination:          maskedDestination,
		State:                      state,
		ConfirmationTokenHash:      hashStatusPageSubscriberToken(confirmationToken),
		ConfirmationTokenExpiresAt: &expiresAt,
		ManageTokenHash:            hashStatusPageSubscriberToken(manageToken),
		ManageTokenVersion:         1,
		UnsubscribeTokenHash:       hashStatusPageSubscriberToken(unsubscribeToken),
		UnsubscribeTokenVersion:    1,
		Source:                     statusPageSubscriberSourcePublicPage,
	}
	if state == statusPageSubscriberStateConfirmed {
		now := time.Now().UTC()
		subscriber.ConfirmedAt = &now
	}
	if err := server.db.Create(&subscriber).Error; err != nil {
		t.Fatalf("create subscriber: %v", err)
	}
	if err := replaceStatusPageSubscriberComponents(server.db, subscriber.ID, componentIDs); err != nil {
		t.Fatalf("create subscriber preferences: %v", err)
	}
	return subscriber
}
