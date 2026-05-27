package api

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"orion/core/internal/db"
	"orion/core/internal/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func TestStatusPageSubscriberFanoutWritesSafeLedgerRowsForConfirmedMatches(t *testing.T) {
	server := setupTestServer(t)
	page, apiComponent, _ := createPublishedStatusPageForSubscriberTest(t, server, "fanout-status")
	workerComponent := createVisibleStatusPageComponentForFanoutTest(t, server, page.ID, apiComponent.SectionID, "Workers")

	allComponentsSubscriber := seedStatusPageSubscriberForTest(t, server, page.ID, "all@example.com", statusPageSubscriberStateConfirmed, "confirm-all", "manage-all", "unsubscribe-all", nil)
	scopedSubscriber := seedStatusPageSubscriberForTest(t, server, page.ID, "api@example.com", statusPageSubscriberStateConfirmed, "confirm-api", "manage-api", "unsubscribe-api", []string{apiComponent.ID})
	seedStatusPageSubscriberForTest(t, server, page.ID, "worker@example.com", statusPageSubscriberStateConfirmed, "confirm-worker", "manage-worker", "unsubscribe-worker", []string{workerComponent.ID})
	seedStatusPageSubscriberForTest(t, server, page.ID, "pending@example.com", statusPageSubscriberStatePending, "confirm-pending", "manage-pending", "unsubscribe-pending", []string{apiComponent.ID})
	seedStatusPageSubscriberForTest(t, server, page.ID, "unsubscribed@example.com", statusPageSubscriberStateUnsubscribed, "confirm-unsubscribed", "manage-unsubscribed", "unsubscribe-unsubscribed", []string{apiComponent.ID})
	seedStatusPageSubscriberForTest(t, server, page.ID, "bounced@example.com", statusPageSubscriberStateBounced, "confirm-bounced", "manage-bounced", "unsubscribe-bounced", []string{apiComponent.ID})
	seedStatusPageSubscriberForTest(t, server, page.ID, "disabled@example.com", statusPageSubscriberStateDisabled, "confirm-disabled", "manage-disabled", "unsubscribe-disabled", []string{apiComponent.ID})

	incidentID := createStatusPageIncidentForFanoutTest(t, server, page.ID, []string{apiComponent.ID})
	updateID := createPublishedStatusPageIncidentUpdateForFanoutTest(t, server, page.ID, incidentID, "Public update without internals smtp_password=secret monitor_id=monitor-private agent_id=agent-private")

	var deliveries []db.StatusPageSubscriberDelivery
	if err := server.db.Order("subscriber_id ASC").Find(&deliveries).Error; err != nil {
		t.Fatalf("load subscriber deliveries: %v", err)
	}
	if len(deliveries) != 2 {
		t.Fatalf("deliveries = %+v, want exactly two confirmed matching subscribers", deliveries)
	}

	expectedSubscriberIDs := map[string]bool{
		allComponentsSubscriber.ID: true,
		scopedSubscriber.ID:        true,
	}
	for _, delivery := range deliveries {
		if !expectedSubscriberIDs[delivery.SubscriberID] {
			t.Fatalf("delivery subscriber_id = %q, want one of %+v", delivery.SubscriberID, expectedSubscriberIDs)
		}
		if delivery.StatusPageID != page.ID || delivery.PublicIncidentID != incidentID || delivery.PublicIncidentUpdateID != updateID {
			t.Fatalf("delivery IDs = %+v, want public status page, incident, and update IDs", delivery)
		}
		if delivery.DeliveryType != statusPageSubscriberDeliveryTypeEmail || delivery.DeliveryState != statusPageSubscriberDeliveryStatePendingSenderConfig {
			t.Fatalf("delivery type/state = %s/%s, want email/%s", delivery.DeliveryType, delivery.DeliveryState, statusPageSubscriberDeliveryStatePendingSenderConfig)
		}
		if delivery.ErrorCode != statusPageSubscriberDeliveryErrorPublicSenderMissing || delivery.SafeErrorSummary != statusPageSubscriberDeliverySummaryPublicSenderMissing {
			t.Fatalf("delivery error fields = %q/%q, want sanitized sender config status", delivery.ErrorCode, delivery.SafeErrorSummary)
		}
		if delivery.ProviderMessageID != "" || delivery.AttemptCount != 0 || delivery.QueuedAt == nil || delivery.SentAt != nil || delivery.FailedAt != nil {
			t.Fatalf("delivery transport fields = %+v, want queued ledger-only row", delivery)
		}
		serialized := strings.Join([]string{
			delivery.ID,
			delivery.SubscriberID,
			delivery.StatusPageID,
			delivery.PublicIncidentID,
			delivery.PublicIncidentUpdateID,
			delivery.DeliveryType,
			delivery.DeliveryState,
			delivery.ErrorCode,
			delivery.SafeErrorSummary,
			delivery.ProviderMessageID,
		}, " ")
		for _, leaked := range []string{"smtp_password", "secret", "monitor-private", "agent-private", "all@example.com", "api@example.com"} {
			if strings.Contains(serialized, leaked) {
				t.Fatalf("delivery leaked internal or destination data %q: %+v", leaked, delivery)
			}
		}
	}

	var updatedSubscribers []db.StatusPageSubscriber
	if err := server.db.Where("id IN ?", []string{allComponentsSubscriber.ID, scopedSubscriber.ID}).Find(&updatedSubscribers).Error; err != nil {
		t.Fatalf("load updated subscribers: %v", err)
	}
	if len(updatedSubscribers) != 2 {
		t.Fatalf("updated subscribers = %+v, want two matching subscribers", updatedSubscribers)
	}
	for _, subscriber := range updatedSubscribers {
		if subscriber.LastDeliveryStatus != statusPageSubscriberDeliveryStatePendingSenderConfig || subscriber.LastDeliveryAt == nil {
			t.Fatalf("subscriber delivery status = %+v, want pending sender configuration timestamp", subscriber)
		}
	}
}

func TestStatusPageSubscriberFanoutDoesNotNotifyHiddenOnlyIncidents(t *testing.T) {
	server := setupTestServer(t)
	page, _, hiddenComponent := createPublishedStatusPageForSubscriberTest(t, server, "hidden-fanout-status")
	seedStatusPageSubscriberForTest(t, server, page.ID, "all-hidden@example.com", statusPageSubscriberStateConfirmed, "confirm-hidden-all", "manage-hidden-all", "unsubscribe-hidden-all", nil)
	seedStatusPageSubscriberForTest(t, server, page.ID, "hidden@example.com", statusPageSubscriberStateConfirmed, "confirm-hidden", "manage-hidden", "unsubscribe-hidden", []string{hiddenComponent.ID})

	incidentID := createStatusPageIncidentForFanoutTest(t, server, page.ID, []string{hiddenComponent.ID})
	createPublishedStatusPageIncidentUpdateForFanoutTest(t, server, page.ID, incidentID, "Hidden-only public update")

	var count int64
	if err := server.db.Model(&db.StatusPageSubscriberDelivery{}).Count(&count).Error; err != nil {
		t.Fatalf("count subscriber deliveries: %v", err)
	}
	if count != 0 {
		t.Fatalf("deliveries count = %d, want none for hidden-only affected components", count)
	}
}

func TestStatusPageSubscriberFanoutRequiresPublishedPageAndUpdate(t *testing.T) {
	server := setupTestServer(t)
	page, apiComponent, _ := createPublishedStatusPageForSubscriberTest(t, server, "fanout-publication-status")
	seedStatusPageSubscriberForTest(t, server, page.ID, "published@example.com", statusPageSubscriberStateConfirmed, "confirm-published", "manage-published", "unsubscribe-published", nil)

	draftIncidentID := createStatusPageIncidentForFanoutTest(t, server, page.ID, []string{apiComponent.ID})
	createDraftStatusPageIncidentUpdateForFanoutTest(t, server, page.ID, draftIncidentID, "Draft update should not fan out")
	assertStatusPageSubscriberDeliveryCount(t, server, 0)

	if err := server.db.Model(&db.StatusPage{}).Where("id = ?", page.ID).Updates(map[string]interface{}{
		"visibility":   statusPageVisibilityDraft,
		"published_at": nil,
	}).Error; err != nil {
		t.Fatalf("unpublish fanout page: %v", err)
	}
	unpublishedIncidentID := createStatusPageIncidentForFanoutTest(t, server, page.ID, []string{apiComponent.ID})
	createPublishedStatusPageIncidentUpdateForFanoutTest(t, server, page.ID, unpublishedIncidentID, "Unpublished page update should not fan out")
	assertStatusPageSubscriberDeliveryCount(t, server, 0)
}

func TestStatusPageSubscriberFanoutDeduplicatesIncidentUpdateDeliveries(t *testing.T) {
	server := setupTestServer(t)
	page, apiComponent, _ := createPublishedStatusPageForSubscriberTest(t, server, "fanout-dedupe-status")
	subscriber := seedStatusPageSubscriberForTest(t, server, page.ID, "dedupe@example.com", statusPageSubscriberStateConfirmed, "confirm-dedupe", "manage-dedupe", "unsubscribe-dedupe", []string{apiComponent.ID})
	incidentID := createStatusPageIncidentForFanoutTest(t, server, page.ID, []string{apiComponent.ID})
	updateID := createPublishedStatusPageIncidentUpdateForFanoutTest(t, server, page.ID, incidentID, "Published once")

	var incident db.StatusPageIncident
	if err := server.db.Where("id = ?", incidentID).First(&incident).Error; err != nil {
		t.Fatalf("load public incident: %v", err)
	}
	var update db.StatusPageIncidentUpdate
	if err := server.db.Where("id = ?", updateID).First(&update).Error; err != nil {
		t.Fatalf("load public incident update: %v", err)
	}
	if err := server.db.Transaction(func(tx *gorm.DB) error {
		return server.enqueueStatusPageSubscriberIncidentUpdateDeliveries(tx, incident, update)
	}); err != nil {
		t.Fatalf("re-enqueue fanout: %v", err)
	}

	var deliveries []db.StatusPageSubscriberDelivery
	if err := server.db.Where("subscriber_id = ?", subscriber.ID).Find(&deliveries).Error; err != nil {
		t.Fatalf("load dedupe deliveries: %v", err)
	}
	if len(deliveries) != 1 {
		t.Fatalf("deliveries = %+v, want one deduplicated delivery", deliveries)
	}
}

func createVisibleStatusPageComponentForFanoutTest(t *testing.T, server *Server, pageID string, sectionID string, name string) db.StatusPageComponent {
	t.Helper()
	component := db.StatusPageComponent{
		ID:           utils.GenerateID("status_page_component"),
		StatusPageID: pageID,
		SectionID:    sectionID,
		PublicName:   name,
		DisplayMode:  "manual",
		ManualStatus: "operational",
		Visible:      true,
	}
	if err := server.db.Create(&component).Error; err != nil {
		t.Fatalf("create visible status page component: %v", err)
	}
	return component
}

func createStatusPageIncidentForFanoutTest(t *testing.T, server *Server, pageID string, componentIDs []string) string {
	t.Helper()
	resp := performJSONRequest(t, server, http.MethodPost, "/v1/status-pages/"+pageID+"/incidents", gin.H{
		"title":                  "Public incident",
		"public_status":          "investigating",
		"severity":               "high",
		"impact_summary":         "Customer-safe summary",
		"visibility":             statusPageIncidentVisibilityDraft,
		"affected_component_ids": componentIDs,
	}, "")
	if resp.Code != http.StatusCreated {
		t.Fatalf("create public incident status = %d, body = %s", resp.Code, resp.Body.String())
	}
	var created struct {
		Data struct {
			Incident struct {
				ID string `json:"id"`
			} `json:"incident"`
		} `json:"data"`
	}
	decodeResponse(t, resp, &created)
	if created.Data.Incident.ID == "" {
		t.Fatalf("created incident response missing ID: %s", resp.Body.String())
	}
	return created.Data.Incident.ID
}

func createDraftStatusPageIncidentUpdateForFanoutTest(t *testing.T, server *Server, pageID string, incidentID string, message string) string {
	t.Helper()
	resp := performJSONRequest(t, server, http.MethodPost, fmt.Sprintf("/v1/status-pages/%s/incidents/%s/updates", pageID, incidentID), gin.H{
		"status":     "identified",
		"message":    message,
		"created_by": "ops",
	}, "")
	if resp.Code != http.StatusCreated {
		t.Fatalf("create draft public incident update status = %d, body = %s", resp.Code, resp.Body.String())
	}
	var created struct {
		Data struct {
			Update struct {
				ID string `json:"id"`
			} `json:"update"`
		} `json:"data"`
	}
	decodeResponse(t, resp, &created)
	if created.Data.Update.ID == "" {
		t.Fatalf("created draft incident update response missing ID: %s", resp.Body.String())
	}
	return created.Data.Update.ID
}

func createPublishedStatusPageIncidentUpdateForFanoutTest(t *testing.T, server *Server, pageID string, incidentID string, message string) string {
	t.Helper()
	resp := performJSONRequest(t, server, http.MethodPost, fmt.Sprintf("/v1/status-pages/%s/incidents/%s/updates", pageID, incidentID), gin.H{
		"status":       "identified",
		"message":      message,
		"created_by":   "ops",
		"published_at": time.Now().UTC(),
	}, "")
	if resp.Code != http.StatusCreated {
		t.Fatalf("create public incident update status = %d, body = %s", resp.Code, resp.Body.String())
	}
	var created struct {
		Data struct {
			Update struct {
				ID string `json:"id"`
			} `json:"update"`
		} `json:"data"`
	}
	decodeResponse(t, resp, &created)
	if created.Data.Update.ID == "" {
		t.Fatalf("created incident update response missing ID: %s", resp.Body.String())
	}
	return created.Data.Update.ID
}

func assertStatusPageSubscriberDeliveryCount(t *testing.T, server *Server, expected int64) {
	t.Helper()
	var count int64
	if err := server.db.Model(&db.StatusPageSubscriberDelivery{}).Count(&count).Error; err != nil {
		t.Fatalf("count subscriber deliveries: %v", err)
	}
	if count != expected {
		t.Fatalf("deliveries count = %d, want %d", count, expected)
	}
}
