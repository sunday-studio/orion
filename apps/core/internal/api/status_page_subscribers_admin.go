package api

import (
	"errors"
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type StatusPageSubscriberAdminComponentResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type StatusPageSubscriberAdminResponse struct {
	ID                 string                                       `json:"id"`
	StatusPageID       string                                       `json:"status_page_id"`
	DestinationType    string                                       `json:"destination_type"`
	MaskedDestination  string                                       `json:"masked_destination"`
	State              string                                       `json:"state"`
	Components         []StatusPageSubscriberAdminComponentResponse `json:"components"`
	BounceCount        int                                          `json:"bounce_count"`
	LastDeliveryStatus string                                       `json:"last_delivery_status,omitempty"`
	LastDeliveryAt     *time.Time                                   `json:"last_delivery_at,omitempty"`
	Source             string                                       `json:"source"`
	ConfirmedAt        *time.Time                                   `json:"confirmed_at,omitempty"`
	UnsubscribedAt     *time.Time                                   `json:"unsubscribed_at,omitempty"`
	DisabledAt         *time.Time                                   `json:"disabled_at,omitempty"`
	CreatedAt          time.Time                                    `json:"created_at"`
	UpdatedAt          time.Time                                    `json:"updated_at"`
}

// listStatusPageSubscribers retrieves redacted subscriber records for operators.
// @Summary      List status page subscribers
// @Description  Get masked status page subscribers without raw destinations, hashes, ciphertext, or token metadata
// @Tags         status-pages
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           listStatusPageSubscribers
// @Param        id     path   string  true   "Status page ID"
// @Param        state  query  string  false  "Subscriber state filter"
// @Success      200    {object}  utils.APIResponse{data=object{subscribers=[]StatusPageSubscriberAdminResponse,count=int}}
// @Failure      404    {object}  utils.APIResponse
// @Failure      500    {object}  utils.APIResponse
// @Router       /v1/status-pages/{id}/subscribers [get]
func (s *Server) listStatusPageSubscribers(c *gin.Context) {
	pageID := strings.TrimSpace(c.Param("id"))
	if !s.statusPageExists(c, pageID) {
		return
	}

	query := s.db.Where("status_page_id = ?", pageID).Order("created_at DESC")
	if state := strings.TrimSpace(c.Query("state")); state != "" {
		query = query.Where("state = ?", state)
	}

	var subscribers []db.StatusPageSubscriber
	if err := query.Find(&subscribers).Error; err != nil {
		s.logger.Error("Failed to list status page subscribers", "status_page_id", pageID, "error", err)
		utils.InternalError(c, "Failed to list status page subscribers", err)
		return
	}

	responses, err := s.statusPageSubscriberAdminResponses(pageID, subscribers)
	if err != nil {
		s.logger.Error("Failed to build status page subscriber responses", "status_page_id", pageID, "error", err)
		utils.InternalError(c, "Failed to list status page subscribers", err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Status page subscribers retrieved successfully", gin.H{
		"subscribers": responses,
		"count":       len(responses),
	})
}

// disableStatusPageSubscriber disables future subscriber deliveries.
// @Summary      Disable status page subscriber
// @Description  Disable a subscriber and clear bearer token hashes without exposing sensitive fields
// @Tags         status-pages
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           disableStatusPageSubscriber
// @Param        id             path  string  true  "Status page ID"
// @Param        subscriber_id  path  string  true  "Subscriber ID"
// @Success      200            {object}  utils.APIResponse{data=object{subscriber=StatusPageSubscriberAdminResponse}}
// @Failure      404            {object}  utils.APIResponse
// @Failure      500            {object}  utils.APIResponse
// @Router       /v1/status-pages/{id}/subscribers/{subscriber_id}/disable [post]
func (s *Server) disableStatusPageSubscriber(c *gin.Context) {
	pageID := strings.TrimSpace(c.Param("id"))
	subscriber, ok := s.loadStatusPageSubscriberForAdmin(c, pageID)
	if !ok {
		return
	}

	now := time.Now().UTC()
	subscriber.State = statusPageSubscriberStateDisabled
	if subscriber.DisabledAt == nil {
		subscriber.DisabledAt = &now
	}
	s.clearStatusPageSubscriberTokenHashes(&subscriber)

	if err := s.db.Save(&subscriber).Error; err != nil {
		s.logger.Error("Failed to disable status page subscriber", "status_page_id", pageID, "subscriber_id", subscriber.ID, "error", err)
		utils.InternalError(c, "Failed to disable status page subscriber", err)
		return
	}
	s.writeStatusPageSubscriberAdminResponse(c, http.StatusOK, "Status page subscriber disabled successfully", subscriber)
}

// anonymizeStatusPageSubscriber removes subscriber contact data while preserving an operational tombstone.
// @Summary      Anonymize status page subscriber
// @Description  Irreversibly remove subscriber contact data, tokens, and component preferences
// @Tags         status-pages
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           anonymizeStatusPageSubscriber
// @Param        id             path  string  true  "Status page ID"
// @Param        subscriber_id  path  string  true  "Subscriber ID"
// @Success      200            {object}  utils.APIResponse{data=object{subscriber=StatusPageSubscriberAdminResponse}}
// @Failure      404            {object}  utils.APIResponse
// @Failure      500            {object}  utils.APIResponse
// @Router       /v1/status-pages/{id}/subscribers/{subscriber_id}/anonymize [post]
func (s *Server) anonymizeStatusPageSubscriber(c *gin.Context) {
	pageID := strings.TrimSpace(c.Param("id"))
	subscriber, ok := s.loadStatusPageSubscriberForAdmin(c, pageID)
	if !ok {
		return
	}

	now := time.Now().UTC()
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		subscriber.DestinationHash = hashStatusPageSubscriberValue("anonymized:" + subscriber.ID)
		subscriber.DestinationValueCiphertext = ""
		subscriber.MaskedDestination = "anonymized"
		subscriber.State = statusPageSubscriberStateDisabled
		subscriber.ConfirmationTokenExpiresAt = nil
		subscriber.ConfirmedAt = nil
		subscriber.UnsubscribedAt = nil
		if subscriber.DisabledAt == nil {
			subscriber.DisabledAt = &now
		}
		s.clearStatusPageSubscriberTokenHashes(&subscriber)
		if err := tx.Save(&subscriber).Error; err != nil {
			return err
		}
		return tx.Where("subscriber_id = ?", subscriber.ID).Delete(&db.StatusPageSubscriberComponent{}).Error
	}); err != nil {
		s.logger.Error("Failed to anonymize status page subscriber", "status_page_id", pageID, "subscriber_id", subscriber.ID, "error", err)
		utils.InternalError(c, "Failed to anonymize status page subscriber", err)
		return
	}

	s.writeStatusPageSubscriberAdminResponse(c, http.StatusOK, "Status page subscriber anonymized successfully", subscriber)
}

// deleteStatusPageSubscriber hard-deletes a subscriber and dependent rows.
// @Summary      Delete status page subscriber
// @Description  Hard-delete a subscriber plus component preferences and delivery ledger rows
// @Tags         status-pages
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           deleteStatusPageSubscriber
// @Param        id             path  string  true  "Status page ID"
// @Param        subscriber_id  path  string  true  "Subscriber ID"
// @Success      200            {object}  utils.APIResponse{data=object{deleted=bool}}
// @Failure      404            {object}  utils.APIResponse
// @Failure      500            {object}  utils.APIResponse
// @Router       /v1/status-pages/{id}/subscribers/{subscriber_id} [delete]
func (s *Server) deleteStatusPageSubscriber(c *gin.Context) {
	pageID := strings.TrimSpace(c.Param("id"))
	subscriber, ok := s.loadStatusPageSubscriberForAdmin(c, pageID)
	if !ok {
		return
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("subscriber_id = ?", subscriber.ID).Delete(&db.StatusPageSubscriberComponent{}).Error; err != nil {
			return err
		}
		if err := tx.Where("subscriber_id = ?", subscriber.ID).Delete(&db.StatusPageSubscriberDelivery{}).Error; err != nil {
			return err
		}
		return tx.Delete(&subscriber).Error
	}); err != nil {
		s.logger.Error("Failed to delete status page subscriber", "status_page_id", pageID, "subscriber_id", subscriber.ID, "error", err)
		utils.InternalError(c, "Failed to delete status page subscriber", err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Status page subscriber deleted successfully", gin.H{"deleted": true})
}

func (s *Server) loadStatusPageSubscriberForAdmin(c *gin.Context, pageID string) (db.StatusPageSubscriber, bool) {
	if !s.statusPageExists(c, pageID) {
		return db.StatusPageSubscriber{}, false
	}
	var subscriber db.StatusPageSubscriber
	err := s.db.Where("status_page_id = ? AND id = ?", pageID, strings.TrimSpace(c.Param("subscriber_id"))).First(&subscriber).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		utils.NotFound(c, "Status page subscriber not found")
		return db.StatusPageSubscriber{}, false
	}
	if err != nil {
		s.logger.Error("Failed to load status page subscriber", "status_page_id", pageID, "error", err)
		utils.InternalError(c, "Failed to load status page subscriber", err)
		return db.StatusPageSubscriber{}, false
	}
	return subscriber, true
}

func (s *Server) statusPageSubscriberAdminResponses(pageID string, subscribers []db.StatusPageSubscriber) ([]StatusPageSubscriberAdminResponse, error) {
	componentNamesBySubscriberID, err := s.statusPageSubscriberComponentNames(pageID, subscribers)
	if err != nil {
		return nil, err
	}
	responses := make([]StatusPageSubscriberAdminResponse, 0, len(subscribers))
	for _, subscriber := range subscribers {
		responses = append(responses, statusPageSubscriberAdminResponse(subscriber, componentNamesBySubscriberID[subscriber.ID]))
	}
	return responses, nil
}

func (s *Server) statusPageSubscriberComponentNames(pageID string, subscribers []db.StatusPageSubscriber) (map[string][]StatusPageSubscriberAdminComponentResponse, error) {
	result := map[string][]StatusPageSubscriberAdminComponentResponse{}
	if len(subscribers) == 0 {
		return result, nil
	}
	subscriberIDs := make([]string, 0, len(subscribers))
	for _, subscriber := range subscribers {
		subscriberIDs = append(subscriberIDs, subscriber.ID)
	}
	var rows []struct {
		SubscriberID string
		ComponentID  string
		PublicName   string
	}
	err := s.db.Table("status_page_subscriber_components").
		Select("status_page_subscriber_components.subscriber_id, status_page_components.id AS component_id, status_page_components.public_name").
		Joins("JOIN status_page_components ON status_page_components.id = status_page_subscriber_components.component_id").
		Where("status_page_subscriber_components.subscriber_id IN ? AND status_page_components.status_page_id = ?", subscriberIDs, pageID).
		Order("status_page_components.sort_order ASC, status_page_components.public_name ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.SubscriberID] = append(result[row.SubscriberID], StatusPageSubscriberAdminComponentResponse{
			ID:   row.ComponentID,
			Name: row.PublicName,
		})
	}
	return result, nil
}

func statusPageSubscriberAdminResponse(subscriber db.StatusPageSubscriber, components []StatusPageSubscriberAdminComponentResponse) StatusPageSubscriberAdminResponse {
	if components == nil {
		components = []StatusPageSubscriberAdminComponentResponse{}
	}
	return StatusPageSubscriberAdminResponse{
		ID:                 subscriber.ID,
		StatusPageID:       subscriber.StatusPageID,
		DestinationType:    subscriber.DestinationType,
		MaskedDestination:  subscriber.MaskedDestination,
		State:              subscriber.State,
		Components:         components,
		BounceCount:        subscriber.BounceCount,
		LastDeliveryStatus: subscriber.LastDeliveryStatus,
		LastDeliveryAt:     subscriber.LastDeliveryAt,
		Source:             subscriber.Source,
		ConfirmedAt:        subscriber.ConfirmedAt,
		UnsubscribedAt:     subscriber.UnsubscribedAt,
		DisabledAt:         subscriber.DisabledAt,
		CreatedAt:          subscriber.CreatedAt,
		UpdatedAt:          subscriber.UpdatedAt,
	}
}

func (s *Server) writeStatusPageSubscriberAdminResponse(c *gin.Context, status int, message string, subscriber db.StatusPageSubscriber) {
	responses, err := s.statusPageSubscriberAdminResponses(subscriber.StatusPageID, []db.StatusPageSubscriber{subscriber})
	if err != nil {
		s.logger.Error("Failed to build status page subscriber response", "status_page_id", subscriber.StatusPageID, "subscriber_id", subscriber.ID, "error", err)
		utils.InternalError(c, "Failed to load status page subscriber", err)
		return
	}
	utils.SuccessResponse(c, status, message, gin.H{"subscriber": responses[0]})
}

func (s *Server) clearStatusPageSubscriberTokenHashes(subscriber *db.StatusPageSubscriber) {
	subscriber.ConfirmationTokenHash = ""
	subscriber.ConfirmationTokenExpiresAt = nil
	subscriber.ManageTokenHash = ""
	subscriber.ManageTokenVersion++
	subscriber.UnsubscribeTokenHash = ""
	subscriber.UnsubscribeTokenVersion++
}
