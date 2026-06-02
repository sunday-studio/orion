package api

import (
	"crypto/sha256"
	"encoding/hex"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"net/http"
	"net/mail"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"strings"
)

func (s *Server) publicSubscriberAvailableComponents(pageID string) ([]StatusPageSubscriberComponentResponse, error) {
	var components []db.StatusPageComponent
	if err := s.db.Where("status_page_id = ? AND visible = ?", pageID, true).Order("sort_order ASC, public_name ASC").Find(&components).Error; err != nil {
		return nil, err
	}
	responses := make([]StatusPageSubscriberComponentResponse, 0, len(components))
	for _, component := range components {
		responses = append(responses, StatusPageSubscriberComponentResponse{ID: component.ID, Name: component.PublicName})
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
		preference := db.StatusPageSubscriberComponent{ID: utils.GenerateID("status_page_subscriber_component"), SubscriberID: subscriberID, ComponentID: componentID, EventScope: statusPageSubscriberEventScopeAll}
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
