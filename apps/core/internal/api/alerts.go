package api

import (
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type alertChannelRequest struct {
	Name             string   `json:"name"`
	Type             string   `json:"type"`
	Enabled          *bool    `json:"enabled"`
	WebhookURL       string   `json:"webhook_url"`
	EmailTo          string   `json:"email_to"`
	EmailFrom        string   `json:"email_from"`
	SMTPHost         string   `json:"smtp_host"`
	SMTPPort         int      `json:"smtp_port"`
	SMTPUsername     string   `json:"smtp_username"`
	SMTPPassword     string   `json:"smtp_password"`
	SubscribedEvents []string `json:"subscribed_events"`
}

// listAlertDeliveries retrieves alert delivery attempts.
// @Summary      List alert deliveries
// @Description  Get a paginated list of alert delivery attempts
// @Tags         alerts
// @Accept       json
// @Produce      json
// @ID           getAlertDeliveries
// @Param        incident_id  query     string  false  "Filter by incident ID"
// @Param        status       query     string  false  "Filter by delivery status"
// @Param        limit        query     int     false  "Maximum number of deliveries to return" default(50)
// @Param        offset       query     int     false  "Number of deliveries to skip" default(0)
// @Success      200          {object}  utils.APIResponse{data=object{deliveries=[]AlertDeliveryResponse,count=int64,limit=int,offset=int,pagination=utils.PaginationMeta}}
// @Failure      500          {object}  utils.APIResponse
// @Router       /v1/alerts/deliveries [get]
func (s *Server) listAlertDeliveries(c *gin.Context) {
	limit := queryInt(c, "limit", 50)
	offset := queryInt(c, "offset", 0)

	query := s.db.Model(&db.AlertDelivery{})
	if incidentID := c.Query("incident_id"); incidentID != "" {
		query = query.Where("incident_id = ?", incidentID)
	}
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		s.logger.Error("Failed to count alert deliveries", "error", err)
		utils.InternalError(c, "Failed to list alert deliveries", err)
		return
	}

	var deliveries []db.AlertDelivery
	if err := query.Order("created_at DESC").Limit(limit).Offset(offset).Find(&deliveries).Error; err != nil {
		s.logger.Error("Failed to list alert deliveries", "error", err)
		utils.InternalError(c, "Failed to list alert deliveries", err)
		return
	}

	responses := alertDeliveryResponses(deliveries)
	utils.SuccessResponse(c, http.StatusOK, "Alert deliveries retrieved successfully", gin.H{
		"deliveries": responses,
		"count":      count,
		"limit":      limit,
		"offset":     offset,
		"pagination": utils.NewPaginationMeta(count, limit, offset, len(responses)),
	})
}

// listAlertChannels retrieves alert channel configuration.
// @Summary      List alert channels
// @Description  Get persisted alert channels and their last delivery status
// @Tags         alerts
// @Accept       json
// @Produce      json
// @ID           getAlertChannels
// @Success      200  {object}  utils.APIResponse{data=object{channels=[]AlertChannelResponse,count=int}}
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/alerts/channels [get]
func (s *Server) listAlertChannels(c *gin.Context) {
	var dbChannels []db.AlertChannel
	if err := s.db.Order("name ASC").Find(&dbChannels).Error; err != nil {
		s.logger.Error("Failed to list alert channels", "error", err)
		utils.InternalError(c, "Failed to list alert channels", err)
		return
	}

	channels := make([]AlertChannelResponse, 0, len(dbChannels))
	for _, channel := range dbChannels {
		channels = append(channels, s.alertChannelResponse(channel))
	}

	utils.SuccessResponse(c, http.StatusOK, "Alert channels retrieved successfully", gin.H{
		"channels": channels,
		"count":    len(channels),
	})
}

// createAlertChannel creates a persisted alert channel.
// @Summary      Create alert channel
// @Description  Create a webhook or email alert channel. Secret values are stored but never returned by the API.
// @Tags         alerts
// @Accept       json
// @Produce      json
// @ID           createAlertChannel
// @Param        request  body      alertChannelRequest  true  "Alert channel payload"
// @Success      201      {object}  utils.APIResponse{data=object{channel=AlertChannelResponse}}
// @Failure      400      {object}  utils.APIResponse
// @Failure      409      {object}  utils.APIResponse
// @Failure      500      {object}  utils.APIResponse
// @Router       /v1/alerts/channels [post]
func (s *Server) createAlertChannel(c *gin.Context) {
	var request alertChannelRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.BadRequest(c, "Invalid alert channel payload")
		return
	}

	channel := db.AlertChannel{
		ID:               utils.GenerateID("alert_channel"),
		Name:             strings.TrimSpace(request.Name),
		Type:             strings.TrimSpace(request.Type),
		Enabled:          true,
		WebhookURL:       strings.TrimSpace(request.WebhookURL),
		EmailTo:          strings.TrimSpace(request.EmailTo),
		EmailFrom:        strings.TrimSpace(request.EmailFrom),
		SMTPHost:         strings.TrimSpace(request.SMTPHost),
		SMTPPort:         request.SMTPPort,
		SMTPUsername:     strings.TrimSpace(request.SMTPUsername),
		SMTPPassword:     request.SMTPPassword,
		SubscribedEvents: db.EncodeAlertEvents(normalizeAlertEvents(request.SubscribedEvents)),
	}
	if request.Enabled != nil {
		channel.Enabled = *request.Enabled
	}
	if channel.Type == "" {
		channel.Type = "webhook"
	}
	if err := validateAlertEvents(request.SubscribedEvents); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := validateAlertChannel(channel); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	var existing int64
	if err := s.db.Model(&db.AlertChannel{}).Where("name = ?", channel.Name).Count(&existing).Error; err != nil {
		s.logger.Error("Failed to check alert channel name", "error", err)
		utils.InternalError(c, "Failed to create alert channel", err)
		return
	}
	if existing > 0 {
		utils.ErrorResponse(c, http.StatusConflict, "Alert channel name already exists", nil)
		return
	}

	if err := s.db.Create(&channel).Error; err != nil {
		s.logger.Error("Failed to create alert channel", "error", err)
		utils.InternalError(c, "Failed to create alert channel", err)
		return
	}

	utils.SuccessResponse(c, http.StatusCreated, "Alert channel created successfully", gin.H{
		"channel": s.alertChannelResponse(channel),
	})
}

// updateAlertChannel updates a persisted alert channel.
// @Summary      Update alert channel
// @Description  Update an alert channel. Omit email secret values to keep existing stored values.
// @Tags         alerts
// @Accept       json
// @Produce      json
// @ID           updateAlertChannel
// @Param        id       path      string               true  "Alert channel ID"
// @Param        request  body      alertChannelRequest  true  "Alert channel payload"
// @Success      200      {object}  utils.APIResponse{data=object{channel=AlertChannelResponse}}
// @Failure      400      {object}  utils.APIResponse
// @Failure      404      {object}  utils.APIResponse
// @Failure      409      {object}  utils.APIResponse
// @Failure      500      {object}  utils.APIResponse
// @Router       /v1/alerts/channels/{id} [patch]
func (s *Server) updateAlertChannel(c *gin.Context) {
	var channel db.AlertChannel
	if err := s.db.Where("id = ?", c.Param("id")).First(&channel).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			utils.NotFound(c, "Alert channel not found")
			return
		}
		s.logger.Error("Failed to load alert channel", "error", err)
		utils.InternalError(c, "Failed to update alert channel", err)
		return
	}

	var request alertChannelRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.BadRequest(c, "Invalid alert channel payload")
		return
	}

	if strings.TrimSpace(request.Name) != "" {
		channel.Name = strings.TrimSpace(request.Name)
	}
	if strings.TrimSpace(request.Type) != "" {
		channel.Type = strings.TrimSpace(request.Type)
	}
	if request.Enabled != nil {
		channel.Enabled = *request.Enabled
	}
	if strings.TrimSpace(request.WebhookURL) != "" {
		channel.WebhookURL = strings.TrimSpace(request.WebhookURL)
	}
	if strings.TrimSpace(request.EmailTo) != "" {
		channel.EmailTo = strings.TrimSpace(request.EmailTo)
	}
	if strings.TrimSpace(request.EmailFrom) != "" {
		channel.EmailFrom = strings.TrimSpace(request.EmailFrom)
	}
	if strings.TrimSpace(request.SMTPHost) != "" {
		channel.SMTPHost = strings.TrimSpace(request.SMTPHost)
	}
	if request.SMTPPort > 0 {
		channel.SMTPPort = request.SMTPPort
	}
	if strings.TrimSpace(request.SMTPUsername) != "" {
		channel.SMTPUsername = strings.TrimSpace(request.SMTPUsername)
	}
	if request.SMTPPassword != "" {
		channel.SMTPPassword = request.SMTPPassword
	}
	if request.SubscribedEvents != nil {
		if err := validateAlertEvents(request.SubscribedEvents); err != nil {
			utils.BadRequest(c, err.Error())
			return
		}
		channel.SubscribedEvents = db.EncodeAlertEvents(normalizeAlertEvents(request.SubscribedEvents))
	}
	if err := validateAlertChannel(channel); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	var existing int64
	if err := s.db.Model(&db.AlertChannel{}).Where("name = ? AND id <> ?", channel.Name, channel.ID).Count(&existing).Error; err != nil {
		s.logger.Error("Failed to check alert channel name", "error", err)
		utils.InternalError(c, "Failed to update alert channel", err)
		return
	}
	if existing > 0 {
		utils.ErrorResponse(c, http.StatusConflict, "Alert channel name already exists", nil)
		return
	}

	if err := s.db.Save(&channel).Error; err != nil {
		s.logger.Error("Failed to update alert channel", "error", err)
		utils.InternalError(c, "Failed to update alert channel", err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Alert channel updated successfully", gin.H{
		"channel": s.alertChannelResponse(channel),
	})
}

// deleteAlertChannel deletes a persisted alert channel.
// @Summary      Delete alert channel
// @Description  Delete an alert channel. Existing delivery history is preserved.
// @Tags         alerts
// @Accept       json
// @Produce      json
// @ID           deleteAlertChannel
// @Param        id   path      string  true  "Alert channel ID"
// @Success      200  {object}  utils.APIResponse
// @Failure      404  {object}  utils.APIResponse
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/alerts/channels/{id} [delete]
func (s *Server) deleteAlertChannel(c *gin.Context) {
	result := s.db.Where("id = ?", c.Param("id")).Delete(&db.AlertChannel{})
	if result.Error != nil {
		s.logger.Error("Failed to delete alert channel", "error", result.Error)
		utils.InternalError(c, "Failed to delete alert channel", result.Error)
		return
	}
	if result.RowsAffected == 0 {
		utils.NotFound(c, "Alert channel not found")
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Alert channel deleted successfully", gin.H{})
}

// listAlertRules retrieves effective alert behavior.
// @Summary      List alert rules
// @Description  Get effective built-in alert rules derived from Core configuration
// @Tags         alerts
// @Accept       json
// @Produce      json
// @ID           getAlertRules
// @Success      200  {object}  utils.APIResponse{data=object{rules=[]AlertRuleResponse,count=int}}
// @Router       /v1/alerts/rules [get]
func (s *Server) listAlertRules(c *gin.Context) {
	var channels []db.AlertChannel
	if err := s.db.Order("name ASC").Find(&channels).Error; err != nil {
		s.logger.Error("Failed to list alert channels for rules", "error", err)
		utils.InternalError(c, "Failed to list alert rules", err)
		return
	}

	targetChannels := make([]string, 0, len(channels))
	for _, channel := range channels {
		if channel.Enabled {
			targetChannels = append(targetChannels, channel.Name)
		}
	}

	rules := []AlertRuleResponse{
		{
			Name:                          "monitor failure",
			TriggerCondition:              "Monitor reports down, degraded, stale, or TLS expiry threshold breach",
			Severity:                      "derived from monitor health",
			Enabled:                       len(targetChannels) > 0,
			CooldownSeconds:               s.cfg.AlertCooldownSeconds,
			RecoveryNotificationEnabled:   s.cfg.AlertRecoveryNotifications,
			MaintenanceSuppressionEnabled: true,
			TargetChannels:                targetChannels,
		},
	}

	utils.SuccessResponse(c, http.StatusOK, "Alert rules retrieved successfully", gin.H{
		"rules": rules,
		"count": len(rules),
	})
}

func (s *Server) alertChannelResponse(channel db.AlertChannel) AlertChannelResponse {
	response := AlertChannelResponse{
		ID:                     channel.ID,
		Name:                   channel.Name,
		Type:                   channel.Type,
		Enabled:                channel.Enabled,
		WebhookURL:             channel.WebhookURL,
		WebhookConfigured:      channel.WebhookURL != "",
		EmailToConfigured:      channel.EmailTo != "",
		EmailFromConfigured:    channel.EmailFrom != "",
		SMTPHostConfigured:     channel.SMTPHost != "",
		SMTPPortConfigured:     channel.SMTPPort > 0,
		SMTPUsernameConfigured: channel.SMTPUsername != "",
		SubscribedEvents:       db.DecodeAlertEvents(channel.SubscribedEvents),
		CreatedAt:              channel.CreatedAt,
		UpdatedAt:              channel.UpdatedAt,
	}

	var delivery db.AlertDelivery
	result := s.db.Where("channel = ?", channel.Name).Order("created_at DESC").Limit(1).Find(&delivery)
	if result.Error == nil && result.RowsAffected > 0 {
		response.LastDeliveryStatus = delivery.Status
		response.LastDeliveryAt = &delivery.CreatedAt
	}

	return response
}

func validateAlertChannel(channel db.AlertChannel) error {
	if strings.TrimSpace(channel.Name) == "" {
		return &requestValidationError{message: "alert channel name is required"}
	}
	switch channel.Type {
	case "webhook":
		if strings.TrimSpace(channel.WebhookURL) == "" {
			return &requestValidationError{message: "webhook alert channel requires webhook_url"}
		}
	case "email":
		if strings.TrimSpace(channel.EmailTo) == "" || strings.TrimSpace(channel.EmailFrom) == "" || strings.TrimSpace(channel.SMTPHost) == "" || channel.SMTPPort <= 0 {
			return &requestValidationError{message: "email alert channel requires email_to, email_from, smtp_host, and smtp_port"}
		}
	default:
		return &requestValidationError{message: "unsupported alert channel type"}
	}
	for _, event := range db.DecodeAlertEvents(channel.SubscribedEvents) {
		if !db.ValidAlertEvent(event) {
			return &requestValidationError{message: "unsupported alert channel event"}
		}
	}
	return nil
}

func normalizeAlertEvents(events []string) []string {
	if len(events) == 0 {
		return db.DefaultAlertEvents()
	}
	normalized := make([]string, 0, len(events))
	seen := map[string]bool{}
	for _, event := range events {
		event = strings.TrimSpace(event)
		if event == "" || seen[event] {
			continue
		}
		if !db.ValidAlertEvent(event) {
			continue
		}
		seen[event] = true
		normalized = append(normalized, event)
	}
	if len(normalized) == 0 {
		return db.DefaultAlertEvents()
	}
	return normalized
}

func validateAlertEvents(events []string) error {
	for _, event := range events {
		if event = strings.TrimSpace(event); event != "" && !db.ValidAlertEvent(event) {
			return &requestValidationError{message: "unsupported alert channel event"}
		}
	}
	return nil
}

type requestValidationError struct {
	message string
}

func (e *requestValidationError) Error() string {
	return e.message
}
