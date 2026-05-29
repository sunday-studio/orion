package api

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/mail"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	statusPageSubscriberDestinationEmail = "email"

	statusPageSubscriberStatePending      = "pending"
	statusPageSubscriberStateConfirmed    = "confirmed"
	statusPageSubscriberStateUnsubscribed = "unsubscribed"
	statusPageSubscriberStateBounced      = "bounced"
	statusPageSubscriberStateDisabled     = "disabled"

	statusPageSubscriberSourcePublicPage = "public_page"
	statusPageSubscriberEventScopeAll    = "all_updates"
)

type statusPageSubscriptionRequest struct {
	DestinationType *string  `json:"destination_type"`
	Destination     *string  `json:"destination"`
	ComponentIDs    []string `json:"component_ids"`
}

type statusPageSubscriberPreferencesRequest struct {
	ComponentIDs []string `json:"component_ids"`
}

type StatusPageSubscriberPublicResponse struct {
	State               string                                  `json:"state"`
	DestinationType     string                                  `json:"destination_type"`
	MaskedDestination   string                                  `json:"masked_destination"`
	ComponentIDs        []string                                `json:"component_ids"`
	AvailableComponents []StatusPageSubscriberComponentResponse `json:"available_components"`
}

type StatusPageSubscriberComponentResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// createPublicStatusPageSubscriber records a pending public subscriber.
// @Summary      Request public status page subscription
// @Description  Create or refresh a pending public subscriber without exposing confirmation tokens
// @Tags         public-status
// @Accept       json
// @Produce      json
// @ID           createPublicStatusPageSubscriber
// @Param        slug     path      string                         true  "Status page slug"
// @Param        request  body      statusPageSubscriptionRequest  true  "Subscription payload"
// @Success      202      {object}  utils.APIResponse{data=object{subscriber=StatusPageSubscriberPublicResponse,confirmation_required=bool}}
// @Failure      400      {object}  utils.APIResponse
// @Failure      404      {object}  utils.APIResponse
// @Failure      429      {object}  utils.APIResponse
// @Failure      500      {object}  utils.APIResponse
// @Router       /status/{slug}/subscribers [post]
func (s *Server) createPublicStatusPageSubscriber(c *gin.Context) {
	if !s.allowPublicSubscriberRequest(c, "create") {
		return
	}
	page, ok := s.loadPublicStatusPageForSubscriberRequest(c)
	if !ok {
		return
	}

	var request statusPageSubscriptionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.BadRequest(c, "Invalid status page subscription payload")
		return
	}
	destinationType, normalizedDestination, maskedDestination, err := normalizeStatusPageSubscriberDestination(request.DestinationType, request.Destination)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	componentIDs, err := s.validatePublicSubscriberComponentIDs(page.ID, request.ComponentIDs)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	confirmationToken, manageToken, unsubscribeToken, err := generateStatusPageSubscriberTokens()
	if err != nil {
		s.logger.Error("Failed to generate status page subscriber tokens", "error", err)
		utils.InternalError(c, "Failed to create status page subscription", err)
		return
	}
	expiresAt := time.Now().UTC().Add(24 * time.Hour)
	destinationHash := hashStatusPageSubscriberValue(destinationType + ":" + normalizedDestination)
	destinationCiphertext, err := s.encryptStatusPageSubscriberDestination(normalizedDestination)
	if err != nil {
		s.logger.Error("Failed to encrypt status page subscriber destination", "status_page_id", page.ID, "error", err)
		utils.InternalError(c, "Failed to create status page subscription", err)
		return
	}
	var subscriber db.StatusPageSubscriber
	confirmationRequired := true

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		err := tx.Where("status_page_id = ? AND destination_type = ? AND destination_hash = ?", page.ID, destinationType, destinationHash).First(&subscriber).Error
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			subscriber = db.StatusPageSubscriber{
				ID:                         utils.GenerateID("status_page_subscriber"),
				StatusPageID:               page.ID,
				DestinationType:            destinationType,
				DestinationHash:            destinationHash,
				DestinationValueCiphertext: destinationCiphertext,
				MaskedDestination:          maskedDestination,
				State:                      statusPageSubscriberStatePending,
				ConfirmationTokenHash:      hashStatusPageSubscriberToken(confirmationToken),
				ConfirmationTokenExpiresAt: &expiresAt,
				ManageTokenHash:            hashStatusPageSubscriberToken(manageToken),
				ManageTokenVersion:         1,
				UnsubscribeTokenHash:       hashStatusPageSubscriberToken(unsubscribeToken),
				UnsubscribeTokenVersion:    1,
				Source:                     statusPageSubscriberSourcePublicPage,
			}
			if err := tx.Create(&subscriber).Error; err != nil {
				return err
			}
		case err != nil:
			return err
		default:
			if subscriber.State == statusPageSubscriberStateDisabled {
				confirmationRequired = false
				return nil
			}
			if subscriber.State != statusPageSubscriberStateConfirmed {
				subscriber.State = statusPageSubscriberStatePending
				subscriber.ConfirmationTokenHash = hashStatusPageSubscriberToken(confirmationToken)
				subscriber.ConfirmationTokenExpiresAt = &expiresAt
				subscriber.UnsubscribedAt = nil
			} else {
				confirmationRequired = false
			}
			subscriber.MaskedDestination = maskedDestination
			if destinationCiphertext != "" {
				subscriber.DestinationValueCiphertext = destinationCiphertext
			}
			if subscriber.ManageTokenHash == "" {
				subscriber.ManageTokenHash = hashStatusPageSubscriberToken(manageToken)
				subscriber.ManageTokenVersion++
			}
			if subscriber.UnsubscribeTokenHash == "" {
				subscriber.UnsubscribeTokenHash = hashStatusPageSubscriberToken(unsubscribeToken)
				subscriber.UnsubscribeTokenVersion++
			}
			if err := tx.Save(&subscriber).Error; err != nil {
				return err
			}
		}
		return replaceStatusPageSubscriberComponents(tx, subscriber.ID, componentIDs)
	}); err != nil {
		s.logger.Error("Failed to create status page subscriber", "status_page_id", page.ID, "error", err)
		utils.InternalError(c, "Failed to create status page subscription", err)
		return
	}

	confirmationDelivery := "not_required"
	if confirmationRequired {
		if err := s.sendStatusPageSubscriberConfirmation(page, normalizedDestination, confirmationToken); err != nil {
			state, errorCode, summary := safePublicStatusMailFailure(err)
			s.recordStatusPageSubscriberDelivery(page.ID, subscriber.ID, state, errorCode, summary)
			if state == statusPageSubscriberDeliveryStatePendingSenderConfig {
				confirmationDelivery = "not_configured"
			} else {
				confirmationDelivery = "failed"
			}
		} else {
			s.recordStatusPageSubscriberDelivery(page.ID, subscriber.ID, statusPageSubscriberDeliveryStateSent, "", "")
			confirmationDelivery = "sent"
		}
	}
	response, err := s.statusPageSubscriberPublicResponse(page.ID, subscriber)
	if err != nil {
		s.logger.Error("Failed to load status page subscriber response", "status_page_id", page.ID, "subscriber_id", subscriber.ID, "error", err)
		utils.InternalError(c, "Failed to create status page subscription", err)
		return
	}
	utils.SuccessResponse(c, http.StatusAccepted, "Status page subscription requested successfully", gin.H{
		"subscriber":             response,
		"confirmation_required":  confirmationRequired,
		"confirmation_delivery":  confirmationDelivery,
		"production_fanout_live": s.ensurePublicStatusMailConfigured() == nil,
	})
}

// confirmPublicStatusPageSubscriber confirms a pending public subscriber.
// @Summary      Confirm public status page subscription
// @Description  Confirm a pending public subscriber by one-time token hash
// @Tags         public-status
// @Accept       json
// @Produce      json
// @ID           confirmPublicStatusPageSubscriber
// @Param        slug   path      string  true  "Status page slug"
// @Param        token  path      string  true  "Confirmation token"
// @Success      200    {object}  utils.APIResponse{data=object{subscriber=StatusPageSubscriberPublicResponse}}
// @Failure      404    {object}  utils.APIResponse
// @Failure      429    {object}  utils.APIResponse
// @Failure      500    {object}  utils.APIResponse
// @Router       /status/{slug}/subscribers/confirm/{token} [get]
func (s *Server) confirmPublicStatusPageSubscriber(c *gin.Context) {
	if !s.allowPublicSubscriberRequest(c, "confirm") {
		return
	}
	page, ok := s.loadPublicStatusPageForSubscriberRequest(c)
	if !ok {
		return
	}
	tokenHash, ok := publicSubscriberTokenHashFromParam(c)
	if !ok {
		return
	}

	var subscriber db.StatusPageSubscriber
	if err := s.db.Where("status_page_id = ? AND confirmation_token_hash = ?", page.ID, tokenHash).First(&subscriber).Error; err != nil {
		writeStatusPageLoadError(c, err, "Status page subscriber confirmation not found")
		return
	}
	if subscriber.State != statusPageSubscriberStatePending || subscriber.ConfirmationTokenExpiresAt == nil || subscriber.ConfirmationTokenExpiresAt.Before(time.Now().UTC()) {
		utils.NotFound(c, "Status page subscriber confirmation not found")
		return
	}

	now := time.Now().UTC()
	subscriber.State = statusPageSubscriberStateConfirmed
	subscriber.ConfirmedAt = &now
	subscriber.ConfirmationTokenHash = ""
	subscriber.ConfirmationTokenExpiresAt = nil
	if err := s.db.Save(&subscriber).Error; err != nil {
		s.logger.Error("Failed to confirm status page subscriber", "status_page_id", page.ID, "subscriber_id", subscriber.ID, "error", err)
		utils.InternalError(c, "Failed to confirm status page subscription", err)
		return
	}

	response, err := s.statusPageSubscriberPublicResponse(page.ID, subscriber)
	if err != nil {
		s.logger.Error("Failed to load confirmed subscriber response", "status_page_id", page.ID, "subscriber_id", subscriber.ID, "error", err)
		utils.InternalError(c, "Failed to confirm status page subscription", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Status page subscription confirmed successfully", gin.H{"subscriber": response})
}

// getPublicStatusPageSubscriberPreferences returns masked subscriber preferences.
// @Summary      Get public status page subscriber preferences
// @Description  Get masked subscriber preferences by manage token
// @Tags         public-status
// @Accept       json
// @Produce      json
// @ID           getPublicStatusPageSubscriberPreferences
// @Param        slug   path      string  true  "Status page slug"
// @Param        token  path      string  true  "Manage token"
// @Success      200    {object}  utils.APIResponse{data=object{subscriber=StatusPageSubscriberPublicResponse}}
// @Failure      404    {object}  utils.APIResponse
// @Failure      429    {object}  utils.APIResponse
// @Failure      500    {object}  utils.APIResponse
// @Router       /status/{slug}/subscribers/manage/{token} [get]
func (s *Server) getPublicStatusPageSubscriberPreferences(c *gin.Context) {
	if !s.allowPublicSubscriberRequest(c, "manage") {
		return
	}
	page, subscriber, ok := s.loadPublicSubscriberByManageToken(c)
	if !ok {
		return
	}
	response, err := s.statusPageSubscriberPublicResponse(page.ID, subscriber)
	if err != nil {
		s.logger.Error("Failed to load subscriber preferences", "status_page_id", page.ID, "subscriber_id", subscriber.ID, "error", err)
		utils.InternalError(c, "Failed to load status page subscriber preferences", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Status page subscriber preferences retrieved successfully", gin.H{"subscriber": response})
}

// updatePublicStatusPageSubscriberPreferences updates component preferences.
// @Summary      Update public status page subscriber preferences
// @Description  Update visible component preferences by manage token
// @Tags         public-status
// @Accept       json
// @Produce      json
// @ID           updatePublicStatusPageSubscriberPreferences
// @Param        slug     path      string                                  true  "Status page slug"
// @Param        token    path      string                                  true  "Manage token"
// @Param        request  body      statusPageSubscriberPreferencesRequest  true  "Preference payload"
// @Success      200      {object}  utils.APIResponse{data=object{subscriber=StatusPageSubscriberPublicResponse}}
// @Failure      400      {object}  utils.APIResponse
// @Failure      404      {object}  utils.APIResponse
// @Failure      429      {object}  utils.APIResponse
// @Failure      500      {object}  utils.APIResponse
// @Router       /status/{slug}/subscribers/manage/{token} [put]
func (s *Server) updatePublicStatusPageSubscriberPreferences(c *gin.Context) {
	if !s.allowPublicSubscriberRequest(c, "manage") {
		return
	}
	page, subscriber, ok := s.loadPublicSubscriberByManageToken(c)
	if !ok {
		return
	}
	if subscriber.State == statusPageSubscriberStateUnsubscribed || subscriber.State == statusPageSubscriberStateDisabled {
		utils.NotFound(c, "Status page subscriber not found")
		return
	}
	var request statusPageSubscriberPreferencesRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.BadRequest(c, "Invalid status page subscriber preference payload")
		return
	}
	componentIDs, err := s.validatePublicSubscriberComponentIDs(page.ID, request.ComponentIDs)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	manageToken, unsubscribeToken, err := generateStatusPageSubscriberManageTokens()
	if err != nil {
		s.logger.Error("Failed to rotate status page subscriber tokens", "error", err)
		utils.InternalError(c, "Failed to update status page subscriber preferences", err)
		return
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := replaceStatusPageSubscriberComponents(tx, subscriber.ID, componentIDs); err != nil {
			return err
		}
		subscriber.ManageTokenHash = hashStatusPageSubscriberToken(manageToken)
		subscriber.ManageTokenVersion++
		subscriber.UnsubscribeTokenHash = hashStatusPageSubscriberToken(unsubscribeToken)
		subscriber.UnsubscribeTokenVersion++
		return tx.Save(&subscriber).Error
	}); err != nil {
		s.logger.Error("Failed to update subscriber preferences", "status_page_id", page.ID, "subscriber_id", subscriber.ID, "error", err)
		utils.InternalError(c, "Failed to update status page subscriber preferences", err)
		return
	}
	response, err := s.statusPageSubscriberPublicResponse(page.ID, subscriber)
	if err != nil {
		s.logger.Error("Failed to load updated subscriber preferences", "status_page_id", page.ID, "subscriber_id", subscriber.ID, "error", err)
		utils.InternalError(c, "Failed to update status page subscriber preferences", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Status page subscriber preferences updated successfully", gin.H{"subscriber": response})
}

// unsubscribePublicStatusPageSubscriber unsubscribes by bearer token.
// @Summary      Unsubscribe public status page subscriber
// @Description  Idempotently unsubscribe a public status page subscriber by token
// @Tags         public-status
// @Accept       json
// @Produce      json
// @ID           unsubscribePublicStatusPageSubscriber
// @Param        slug   path      string  true  "Status page slug"
// @Param        token  path      string  true  "Unsubscribe token"
// @Success      200    {object}  utils.APIResponse{data=object{unsubscribed=bool}}
// @Failure      429    {object}  utils.APIResponse
// @Failure      500    {object}  utils.APIResponse
// @Router       /status/{slug}/subscribers/unsubscribe/{token} [post]
func (s *Server) unsubscribePublicStatusPageSubscriber(c *gin.Context) {
	if !s.allowPublicSubscriberRequest(c, "unsubscribe") {
		return
	}
	page, ok := s.loadPublicStatusPageForSubscriberRequest(c)
	if !ok {
		return
	}
	token := strings.TrimSpace(c.Param("token"))
	if token == "" {
		writeGenericUnsubscribeSuccess(c)
		return
	}

	var subscriber db.StatusPageSubscriber
	err := s.db.Where("status_page_id = ? AND unsubscribe_token_hash = ?", page.ID, hashStatusPageSubscriberToken(token)).First(&subscriber).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		writeGenericUnsubscribeSuccess(c)
		return
	}
	if err != nil {
		s.logger.Error("Failed to load subscriber for unsubscribe", "status_page_id", page.ID, "error", err)
		utils.InternalError(c, "Failed to unsubscribe status page subscriber", err)
		return
	}
	if subscriber.State != statusPageSubscriberStateUnsubscribed {
		now := time.Now().UTC()
		subscriber.State = statusPageSubscriberStateUnsubscribed
		subscriber.UnsubscribedAt = &now
		if err := s.db.Save(&subscriber).Error; err != nil {
			s.logger.Error("Failed to unsubscribe status page subscriber", "status_page_id", page.ID, "subscriber_id", subscriber.ID, "error", err)
			utils.InternalError(c, "Failed to unsubscribe status page subscriber", err)
			return
		}
	}
	writeGenericUnsubscribeSuccess(c)
}

func (s *Server) allowPublicSubscriberRequest(c *gin.Context, scope string) bool {
	key := "status_page_subscriber:" + scope + ":" + strings.TrimSpace(c.Param("slug")) + ":" + c.ClientIP()
	if s.publicSubscriberLimiter.TooManyFailures(key) {
		utils.ErrorResponse(c, http.StatusTooManyRequests, "Too many status page subscriber requests", nil)
		return false
	}
	s.publicSubscriberLimiter.RecordFailure(key)
	return true
}

func (s *Server) loadPublicStatusPageForSubscriberRequest(c *gin.Context) (db.StatusPage, bool) {
	var page db.StatusPage
	if err := s.db.Where("slug = ? AND visibility IN ?", strings.TrimSpace(c.Param("slug")), []string{statusPageVisibilityPublic, statusPageVisibilityUnlisted}).First(&page).Error; err != nil {
		writeStatusPageLoadError(c, err, "Status page not found")
		return db.StatusPage{}, false
	}
	return page, true
}

func (s *Server) loadPublicSubscriberByManageToken(c *gin.Context) (db.StatusPage, db.StatusPageSubscriber, bool) {
	page, ok := s.loadPublicStatusPageForSubscriberRequest(c)
	if !ok {
		return db.StatusPage{}, db.StatusPageSubscriber{}, false
	}
	tokenHash, ok := publicSubscriberTokenHashFromParam(c)
	if !ok {
		return db.StatusPage{}, db.StatusPageSubscriber{}, false
	}
	var subscriber db.StatusPageSubscriber
	if err := s.db.Where("status_page_id = ? AND manage_token_hash = ?", page.ID, tokenHash).First(&subscriber).Error; err != nil {
		writeStatusPageLoadError(c, err, "Status page subscriber not found")
		return db.StatusPage{}, db.StatusPageSubscriber{}, false
	}
	return page, subscriber, true
}

func (s *Server) statusPageSubscriberPublicResponse(pageID string, subscriber db.StatusPageSubscriber) (StatusPageSubscriberPublicResponse, error) {
	available, err := s.publicSubscriberAvailableComponents(pageID)
	if err != nil {
		return StatusPageSubscriberPublicResponse{}, err
	}
	selected, err := s.publicSubscriberSelectedComponentIDs(subscriber.ID, visibleStatusPageComponentIDSet(available))
	if err != nil {
		return StatusPageSubscriberPublicResponse{}, err
	}
	return StatusPageSubscriberPublicResponse{
		State:               subscriber.State,
		DestinationType:     subscriber.DestinationType,
		MaskedDestination:   subscriber.MaskedDestination,
		ComponentIDs:        selected,
		AvailableComponents: available,
	}, nil
}

func (s *Server) publicSubscriberAvailableComponents(pageID string) ([]StatusPageSubscriberComponentResponse, error) {
	var components []db.StatusPageComponent
	if err := s.db.Where("status_page_id = ? AND visible = ?", pageID, true).Order("sort_order ASC, public_name ASC").Find(&components).Error; err != nil {
		return nil, err
	}
	responses := make([]StatusPageSubscriberComponentResponse, 0, len(components))
	for _, component := range components {
		responses = append(responses, StatusPageSubscriberComponentResponse{
			ID:   component.ID,
			Name: component.PublicName,
		})
	}
	return responses, nil
}

func (s *Server) publicSubscriberSelectedComponentIDs(subscriberID string, visibleComponentIDs map[string]bool) ([]string, error) {
	var preferences []db.StatusPageSubscriberComponent
	if err := s.db.Where("subscriber_id = ?", subscriberID).Order("created_at ASC").Find(&preferences).Error; err != nil {
		return nil, err
	}
	componentIDs := make([]string, 0, len(preferences))
	for _, preference := range preferences {
		if visibleComponentIDs[preference.ComponentID] {
			componentIDs = append(componentIDs, preference.ComponentID)
		}
	}
	return componentIDs, nil
}

func (s *Server) validatePublicSubscriberComponentIDs(pageID string, requested []string) ([]string, error) {
	componentIDs := normalizeStringList(requested)
	if len(componentIDs) == 0 {
		return []string{}, nil
	}
	var count int64
	if err := s.db.Model(&db.StatusPageComponent{}).Where("status_page_id = ? AND visible = ? AND id IN ?", pageID, true, componentIDs).Count(&count).Error; err != nil {
		return nil, err
	}
	if int(count) != len(componentIDs) {
		return nil, &requestValidationError{message: "component_ids must reference visible components on this status page"}
	}
	return componentIDs, nil
}

func replaceStatusPageSubscriberComponents(tx *gorm.DB, subscriberID string, componentIDs []string) error {
	if err := tx.Where("subscriber_id = ?", subscriberID).Delete(&db.StatusPageSubscriberComponent{}).Error; err != nil {
		return err
	}
	for _, componentID := range componentIDs {
		preference := db.StatusPageSubscriberComponent{
			ID:           utils.GenerateID("status_page_subscriber_component"),
			SubscriberID: subscriberID,
			ComponentID:  componentID,
			EventScope:   statusPageSubscriberEventScopeAll,
		}
		if err := tx.Create(&preference).Error; err != nil {
			return err
		}
	}
	return nil
}

func normalizeStatusPageSubscriberDestination(destinationTypeInput *string, destinationInput *string) (string, string, string, error) {
	destinationType := statusPageSubscriberDestinationEmail
	if destinationTypeInput != nil {
		destinationType = strings.ToLower(strings.TrimSpace(*destinationTypeInput))
	}
	if destinationType != statusPageSubscriberDestinationEmail {
		return "", "", "", &requestValidationError{message: "unsupported status page subscriber destination_type"}
	}
	if destinationInput == nil || strings.TrimSpace(*destinationInput) == "" {
		return "", "", "", &requestValidationError{message: "destination is required"}
	}
	address, err := mail.ParseAddress(strings.TrimSpace(*destinationInput))
	if err != nil {
		return "", "", "", &requestValidationError{message: "destination must be a valid email address"}
	}
	normalized := strings.ToLower(strings.TrimSpace(address.Address))
	if normalized == "" || !strings.Contains(normalized, "@") {
		return "", "", "", &requestValidationError{message: "destination must be a valid email address"}
	}
	return destinationType, normalized, maskStatusPageSubscriberEmail(normalized), nil
}

func maskStatusPageSubscriberEmail(value string) string {
	local, domain, ok := strings.Cut(value, "@")
	if !ok || local == "" || domain == "" {
		return "***"
	}
	prefix := local[:1]
	return prefix + "***@" + domain
}

func generateStatusPageSubscriberTokens() (string, string, string, error) {
	confirmationToken, err := utils.GenerateToken()
	if err != nil {
		return "", "", "", err
	}
	manageToken, unsubscribeToken, err := generateStatusPageSubscriberManageTokens()
	if err != nil {
		return "", "", "", err
	}
	return confirmationToken, manageToken, unsubscribeToken, nil
}

func generateStatusPageSubscriberManageTokens() (string, string, error) {
	manageToken, err := utils.GenerateToken()
	if err != nil {
		return "", "", err
	}
	unsubscribeToken, err := utils.GenerateToken()
	if err != nil {
		return "", "", err
	}
	return manageToken, unsubscribeToken, nil
}

func publicSubscriberTokenHashFromParam(c *gin.Context) (string, bool) {
	token := strings.TrimSpace(c.Param("token"))
	if token == "" {
		utils.NotFound(c, "Status page subscriber not found")
		return "", false
	}
	return hashStatusPageSubscriberToken(token), true
}

func hashStatusPageSubscriberValue(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func hashStatusPageSubscriberToken(token string) string {
	return hashStatusPageSubscriberValue(strings.TrimSpace(token))
}

func visibleStatusPageComponentIDSet(components []StatusPageSubscriberComponentResponse) map[string]bool {
	result := make(map[string]bool, len(components))
	for _, component := range components {
		result[component.ID] = true
	}
	return result
}

func writeGenericUnsubscribeSuccess(c *gin.Context) {
	utils.SuccessResponse(c, http.StatusOK, "Status page subscriber unsubscribed successfully", gin.H{"unsubscribed": true})
}
