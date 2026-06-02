package api

import (
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"strings"
	"testing"
	"time"
)

func TestDeleteStatusPageSubscriberHardDeletesDependentRows(t *testing.T) {
	server := setupTestServer(t)
	page, visibleComponent, _ := createPublishedStatusPageForSubscriberTest(t, server, "admin-delete-status")
	subscriber := seedStatusPageSubscriberForTest(t, server, page.ID, "Delete.User@example.com", statusPageSubscriberStateConfirmed, "delete-confirm-token", "delete-manage-token", "delete-unsubscribe-token", []string{visibleComponent.ID})
	delivery := db.StatusPageSubscriberDelivery{ID: utils.GenerateID("status_page_delivery"), SubscriberID: subscriber.ID, StatusPageID: page.ID, DeliveryType: statusPageSubscriberDeliveryTypeEmail, DeliveryState: statusPageSubscriberDeliveryStateSent, ProviderMessageID: "provider-message-id"}
	if err := server.db.Create(&delivery).Error; err != nil {
		t.Fatalf("create delivery: %v", err)
	}
	resp := performJSONRequest(t, server, http.MethodDelete, "/v1/status-pages/"+page.ID+"/subscribers/"+subscriber.ID, nil, "")
	if resp.Code != http.StatusOK {
		t.Fatalf("delete subscriber status = %d, body = %s", resp.Code, resp.Body.String())
	}
	assertContains(t, resp.Body.String(), `"deleted":true`)
	assertNotContains(t, resp.Body.String(), "Delete.User")
	assertNotContains(t, resp.Body.String(), subscriber.DestinationHash)
	for name, model := range map[string]interface{}{"subscriber": &db.StatusPageSubscriber{}, "preference": &db.StatusPageSubscriberComponent{}, "delivery": &db.StatusPageSubscriberDelivery{}} {
		var count int64
		query := server.db.Model(model)
		if name == "subscriber" {
			query = query.Where("id = ?", subscriber.ID)
		} else {
			query = query.Where("subscriber_id = ?", subscriber.ID)
		}
		if err := query.Count(&count).Error; err != nil {
			t.Fatalf("count %s rows: %v", name, err)
		}
		if count != 0 {
			t.Fatalf("%s count = %d, want 0", name, count)
		}
	}
}
func createPublishedStatusPageForSubscriberTest(t *testing.T, server *Server, slug string) (db.StatusPage, db.StatusPageComponent, db.StatusPageComponent) {
	t.Helper()
	now := time.Now().UTC()
	page := db.StatusPage{ID: utils.GenerateID("status_page"), Slug: slug, Title: "Subscriber Test Status", Visibility: statusPageVisibilityPublic, ThemeSettings: "{}", DefaultIncidentVisibility: statusPageIncidentVisibilityDraft, PublishedAt: &now}
	if err := server.db.Create(&page).Error; err != nil {
		t.Fatalf("create status page: %v", err)
	}
	section := db.StatusPageSection{ID: utils.GenerateID("status_page_section"), StatusPageID: page.ID, Name: "Services"}
	if err := server.db.Create(&section).Error; err != nil {
		t.Fatalf("create section: %v", err)
	}
	visibleComponent := db.StatusPageComponent{ID: utils.GenerateID("status_page_component"), StatusPageID: page.ID, SectionID: section.ID, PublicName: "Visible API", DisplayMode: "manual", ManualStatus: "operational", Visible: true}
	hiddenComponent := db.StatusPageComponent{ID: utils.GenerateID("status_page_component"), StatusPageID: page.ID, SectionID: section.ID, PublicName: "Hidden Database", DisplayMode: "manual", ManualStatus: "operational", Visible: false}
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
	destinationCiphertext, err := server.encryptStatusPageSubscriberDestination(normalizedDestination)
	if err != nil {
		t.Fatalf("encrypt destination: %v", err)
	}
	expiresAt := time.Now().UTC().Add(time.Hour)
	subscriber := db.StatusPageSubscriber{ID: utils.GenerateID("status_page_subscriber"), StatusPageID: pageID, DestinationType: destinationType, DestinationHash: hashStatusPageSubscriberValue(destinationType + ":" + normalizedDestination), DestinationValueCiphertext: destinationCiphertext, MaskedDestination: maskedDestination, State: state, ConfirmationTokenHash: hashStatusPageSubscriberToken(confirmationToken), ConfirmationTokenExpiresAt: &expiresAt, ManageTokenHash: hashStatusPageSubscriberToken(manageToken), ManageTokenVersion: 1, UnsubscribeTokenHash: hashStatusPageSubscriberToken(unsubscribeToken), UnsubscribeTokenVersion: 1, Source: statusPageSubscriberSourcePublicPage}
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
func assertStatusPageSubscriberAdminResponseRedacted(t *testing.T, body string, rawDestination string, subscriber db.StatusPageSubscriber) {
	t.Helper()
	assertNotContainsIfPresent(t, body, rawDestination)
	assertNotContainsIfPresent(t, body, strings.Split(rawDestination, "@")[0])
	assertNotContainsIfPresent(t, body, subscriber.DestinationHash)
	assertNotContainsIfPresent(t, body, subscriber.DestinationValueCiphertext)
	assertNotContainsIfPresent(t, body, subscriber.ConfirmationTokenHash)
	assertNotContainsIfPresent(t, body, subscriber.ManageTokenHash)
	assertNotContainsIfPresent(t, body, subscriber.UnsubscribeTokenHash)
	assertNotContains(t, body, "destination_hash")
	assertNotContains(t, body, "destination_value_ciphertext")
	assertNotContains(t, body, "confirmation_token")
	assertNotContains(t, body, "manage_token")
	assertNotContains(t, body, "unsubscribe_token")
	assertNotContains(t, body, "token_version")
}
func assertNotContainsIfPresent(t *testing.T, body string, value string) {
	t.Helper()
	if value == "" {
		return
	}
	assertNotContains(t, body, value)
}
func configurePublicStatusMailForTest(server *Server) {
	server.cfg.PublicStatusMailEnabled = true
	server.cfg.PublicStatusMailHost = "smtp.example.com"
	server.cfg.PublicStatusMailPort = 587
	server.cfg.PublicStatusMailFromEmail = "status@example.com"
	server.cfg.PublicStatusMailFromName = "Orion Status"
	server.cfg.PublicStatusMailReplyTo = "support@example.com"
	server.cfg.PublicStatusMailUsername = "status-user"
	server.cfg.PublicStatusMailPassword = "status-password-secret"
	server.cfg.PublicStatusURLOrigin = "https://status.example.com"
	server.cfg.PublicStatusSubscriberSecret = "test-subscriber-secret"
}
