package api

import (
	"net/http"
	"testing"

	"orion/core/internal/db"

	"github.com/gin-gonic/gin"
)

func TestPublicStatusPageSubscriberPreferenceUpdateRotatesPrivateTokens(t *testing.T) {
	server := setupTestServer(t)
	page, visibleComponent, _ := createPublishedStatusPageForSubscriberTest(t, server, "privacy-rotate-status")
	manageToken := "manage-token-before-rotation"
	unsubscribeToken := "unsubscribe-token-before-rotation"
	subscriber := seedStatusPageSubscriberForTest(t, server, page.ID, "Rotate.User@example.com", statusPageSubscriberStateConfirmed, "confirm-token", manageToken, unsubscribeToken, []string{})

	updateResp := performJSONRequest(t, server, http.MethodPut, "/status/"+page.Slug+"/subscribers/manage/"+manageToken, gin.H{
		"component_ids": []string{visibleComponent.ID},
	}, "")
	if updateResp.Code != http.StatusOK {
		t.Fatalf("update preferences status = %d, body = %s", updateResp.Code, updateResp.Body.String())
	}
	body := updateResp.Body.String()
	assertContains(t, body, `"masked_destination":"r***@example.com"`)
	assertNotContains(t, body, "Rotate.User")
	assertNotContains(t, body, manageToken)
	assertNotContains(t, body, unsubscribeToken)
	assertNotContains(t, body, "token")

	var stored db.StatusPageSubscriber
	if err := server.db.Where("id = ?", subscriber.ID).First(&stored).Error; err != nil {
		t.Fatalf("load updated subscriber: %v", err)
	}
	if stored.ManageTokenHash == hashStatusPageSubscriberToken(manageToken) || stored.UnsubscribeTokenHash == hashStatusPageSubscriberToken(unsubscribeToken) {
		t.Fatalf("subscriber token hashes were not rotated: %+v", stored)
	}
	if stored.ManageTokenVersion != 2 || stored.UnsubscribeTokenVersion != 2 {
		t.Fatalf("subscriber token versions = manage:%d unsubscribe:%d, want 2/2", stored.ManageTokenVersion, stored.UnsubscribeTokenVersion)
	}

	oldManageResp := performJSONRequest(t, server, http.MethodGet, "/status/"+page.Slug+"/subscribers/manage/"+manageToken, nil, "")
	if oldManageResp.Code != http.StatusNotFound {
		t.Fatalf("old manage token status = %d, body = %s, want 404", oldManageResp.Code, oldManageResp.Body.String())
	}
	assertNotContains(t, oldManageResp.Body.String(), manageToken)

	oldUnsubscribeResp := performJSONRequest(t, server, http.MethodPost, "/status/"+page.Slug+"/subscribers/unsubscribe/"+unsubscribeToken, nil, "")
	if oldUnsubscribeResp.Code != http.StatusOK {
		t.Fatalf("old unsubscribe token status = %d, body = %s, want generic success", oldUnsubscribeResp.Code, oldUnsubscribeResp.Body.String())
	}

	var afterOldUnsubscribe db.StatusPageSubscriber
	if err := server.db.Where("id = ?", subscriber.ID).First(&afterOldUnsubscribe).Error; err != nil {
		t.Fatalf("reload subscriber after old unsubscribe: %v", err)
	}
	if afterOldUnsubscribe.State != statusPageSubscriberStateConfirmed || afterOldUnsubscribe.UnsubscribedAt != nil {
		t.Fatalf("subscriber after old unsubscribe = %+v, want still confirmed", afterOldUnsubscribe)
	}
}
