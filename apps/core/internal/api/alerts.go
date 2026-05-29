package api

import (
	"encoding/json"
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"orion/core/internal/utils"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type alertChannelRequest struct {
	Name                 string   `json:"name"`
	Type                 string   `json:"type"`
	Enabled              *bool    `json:"enabled"`
	WebhookURL           string   `json:"webhook_url"`
	WebhookSigningSecret *string  `json:"webhook_signing_secret"`
	EmailTo              string   `json:"email_to"`
	EmailFrom            string   `json:"email_from"`
	SMTPHost             string   `json:"smtp_host"`
	SMTPPort             int      `json:"smtp_port"`
	SMTPUsername         string   `json:"smtp_username"`
	SMTPPassword         string   `json:"smtp_password"`
	SubscribedEvents     []string `json:"subscribed_events"`
}

type alertSMTPServiceRequest struct {
	Name      string `json:"name"`
	Enabled   *bool  `json:"enabled"`
	Host      string `json:"host"`
	Port      int    `json:"port"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	FromEmail string `json:"from_email"`
}

type alertEmailDestinationRequest struct {
	SMTPServiceID    string   `json:"smtp_service_id"`
	Name             string   `json:"name"`
	Enabled          *bool    `json:"enabled"`
	EmailTo          string   `json:"email_to"`
	SubscribedEvents []string `json:"subscribed_events"`
}

type alertRouteRequest struct {
	Name                 string   `json:"name"`
	Enabled              *bool    `json:"enabled"`
	Priority             *int     `json:"priority"`
	EventTypes           []string `json:"event_types"`
	Severities           []string `json:"severities"`
	AgentIDs             []string `json:"agent_ids"`
	MonitorIDs           []string `json:"monitor_ids"`
	MonitorTypes         []string `json:"monitor_types"`
	ChannelIDs           []string `json:"channel_ids"`
	Suppress             *bool    `json:"suppress"`
	GroupingPolicy       string   `json:"grouping_policy"`
	GroupingDelaySeconds *int     `json:"grouping_delay_seconds"`
}

type alertRouteDryRunRequest struct {
	IncidentID  string `json:"incident_id"`
	EventType   string `json:"event_type"`
	Severity    string `json:"severity"`
	AgentID     string `json:"agent_id"`
	MonitorID   string `json:"monitor_id"`
	MonitorType string `json:"monitor_type"`
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
// @Param        type         query     string  false  "Filter by delivery channel type"
// @Param        channel      query     string  false  "Filter by delivery channel name"
// @Param        event_type   query     string  false  "Filter by alert event type"
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
	if deliveryType := c.Query("type"); deliveryType != "" {
		query = query.Where("type = ?", deliveryType)
	}
	if channel := c.Query("channel"); channel != "" {
		query = query.Where("channel = ?", channel)
	}
	if eventType := c.Query("event_type"); eventType != "" {
		query = query.Where("event_type = ?", eventType)
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		s.logger.Error("Failed to count alert deliveries", "error", err)
		utils.InternalError(c, "Failed to list alert deliveries", err)
		return
	}

	var deliveries []db.AlertDelivery
	if err := query.
		Preload("Attempts", func(tx *gorm.DB) *gorm.DB {
			return tx.Order("attempt_number ASC")
		}).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Find(&deliveries).Error; err != nil {
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
// @Description  Create a webhook, Slack, Discord, or email alert channel. Secret values are stored but never returned by the API.
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
		ID:                   utils.GenerateID("alert_channel"),
		Name:                 strings.TrimSpace(request.Name),
		Type:                 strings.TrimSpace(request.Type),
		Enabled:              true,
		WebhookURL:           strings.TrimSpace(request.WebhookURL),
		WebhookSigningSecret: alertOptionalSecret(request.WebhookSigningSecret),
		EmailTo:              strings.TrimSpace(request.EmailTo),
		EmailFrom:            strings.TrimSpace(request.EmailFrom),
		SMTPHost:             strings.TrimSpace(request.SMTPHost),
		SMTPPort:             request.SMTPPort,
		SMTPUsername:         strings.TrimSpace(request.SMTPUsername),
		SMTPPassword:         request.SMTPPassword,
		SubscribedEvents:     db.EncodeAlertEvents(normalizeAlertEvents(request.SubscribedEvents)),
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
	if request.WebhookSigningSecret != nil {
		channel.WebhookSigningSecret = strings.TrimSpace(*request.WebhookSigningSecret)
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

// testAlertChannel sends a manual test notification through a persisted alert channel.
// @Summary      Test alert channel
// @Description  Send a manual test notification through a configured webhook or email alert channel.
// @Tags         alerts
// @Accept       json
// @Produce      json
// @ID           testAlertChannel
// @Param        id   path      string  true  "Alert channel ID"
// @Success      200  {object}  utils.APIResponse{data=object{delivery=AlertDeliveryResponse}}
// @Failure      404  {object}  utils.APIResponse
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/alerts/channels/{id}/test [post]
func (s *Server) testAlertChannel(c *gin.Context) {
	delivery, err := service.NewAlertService(s.db, s.logger, s.cfg).TestChannel(c.Param("id"))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			utils.NotFound(c, "Alert channel not found")
			return
		}
		s.logger.Error("Failed to test alert channel", "error", err)
		utils.InternalError(c, "Failed to test alert channel", err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Alert channel test completed", gin.H{
		"delivery": alertDeliveryResponse(*delivery),
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

// listAlertSMTPServices retrieves reusable SMTP service records.
// @Summary      List alert SMTP services
// @Description  Get reusable SMTP services without secret values
// @Tags         alerts
// @Accept       json
// @Produce      json
// @ID           getAlertSMTPServices
// @Success      200  {object}  utils.APIResponse{data=object{smtp_services=[]AlertSMTPServiceResponse,count=int}}
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/alerts/smtp-services [get]
func (s *Server) listAlertSMTPServices(c *gin.Context) {
	var services []db.AlertSMTPService
	if err := s.db.Order("name ASC").Find(&services).Error; err != nil {
		s.logger.Error("Failed to list alert SMTP services", "error", err)
		utils.InternalError(c, "Failed to list alert SMTP services", err)
		return
	}

	responses := make([]AlertSMTPServiceResponse, 0, len(services))
	for _, service := range services {
		responses = append(responses, alertSMTPServiceResponse(service))
	}

	utils.SuccessResponse(c, http.StatusOK, "Alert SMTP services retrieved successfully", gin.H{
		"smtp_services": responses,
		"count":         len(responses),
	})
}

// createAlertSMTPService creates a reusable SMTP service.
// @Summary      Create alert SMTP service
// @Description  Create a reusable SMTP service. Secret values are stored but never returned by the API.
// @Tags         alerts
// @Accept       json
// @Produce      json
// @ID           createAlertSMTPService
// @Param        request  body      alertSMTPServiceRequest  true  "SMTP service payload"
// @Success      201      {object}  utils.APIResponse{data=object{smtp_service=AlertSMTPServiceResponse}}
// @Failure      400      {object}  utils.APIResponse
// @Failure      409      {object}  utils.APIResponse
// @Failure      500      {object}  utils.APIResponse
// @Router       /v1/alerts/smtp-services [post]
func (s *Server) createAlertSMTPService(c *gin.Context) {
	var request alertSMTPServiceRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.BadRequest(c, "Invalid alert SMTP service payload")
		return
	}

	smtpService := db.AlertSMTPService{
		ID:        utils.GenerateID("alert_smtp_service"),
		Name:      strings.TrimSpace(request.Name),
		Enabled:   true,
		Host:      strings.TrimSpace(request.Host),
		Port:      request.Port,
		Username:  strings.TrimSpace(request.Username),
		Password:  request.Password,
		FromEmail: strings.TrimSpace(request.FromEmail),
	}
	if request.Enabled != nil {
		smtpService.Enabled = *request.Enabled
	}
	if err := validateAlertSMTPService(smtpService); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := s.ensureUniqueAlertSMTPServiceName(smtpService.Name, ""); err != nil {
		writeAlertNameConflict(c, err, "Alert SMTP service name already exists")
		return
	}

	if err := s.db.Create(&smtpService).Error; err != nil {
		s.logger.Error("Failed to create alert SMTP service", "error", err)
		utils.InternalError(c, "Failed to create alert SMTP service", err)
		return
	}

	utils.SuccessResponse(c, http.StatusCreated, "Alert SMTP service created successfully", gin.H{
		"smtp_service": alertSMTPServiceResponse(smtpService),
	})
}

// updateAlertSMTPService updates a reusable SMTP service.
// @Summary      Update alert SMTP service
// @Description  Update a reusable SMTP service. Omit password to keep the existing stored value.
// @Tags         alerts
// @Accept       json
// @Produce      json
// @ID           updateAlertSMTPService
// @Param        id       path      string                   true  "SMTP service ID"
// @Param        request  body      alertSMTPServiceRequest  true  "SMTP service payload"
// @Success      200      {object}  utils.APIResponse{data=object{smtp_service=AlertSMTPServiceResponse}}
// @Failure      400      {object}  utils.APIResponse
// @Failure      404      {object}  utils.APIResponse
// @Failure      409      {object}  utils.APIResponse
// @Failure      500      {object}  utils.APIResponse
// @Router       /v1/alerts/smtp-services/{id} [patch]
func (s *Server) updateAlertSMTPService(c *gin.Context) {
	var smtpService db.AlertSMTPService
	if err := s.db.Where("id = ?", c.Param("id")).First(&smtpService).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			utils.NotFound(c, "Alert SMTP service not found")
			return
		}
		s.logger.Error("Failed to load alert SMTP service", "error", err)
		utils.InternalError(c, "Failed to update alert SMTP service", err)
		return
	}

	var request alertSMTPServiceRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.BadRequest(c, "Invalid alert SMTP service payload")
		return
	}
	if strings.TrimSpace(request.Name) != "" {
		smtpService.Name = strings.TrimSpace(request.Name)
	}
	if request.Enabled != nil {
		smtpService.Enabled = *request.Enabled
	}
	if strings.TrimSpace(request.Host) != "" {
		smtpService.Host = strings.TrimSpace(request.Host)
	}
	if request.Port > 0 {
		smtpService.Port = request.Port
	}
	if strings.TrimSpace(request.Username) != "" {
		smtpService.Username = strings.TrimSpace(request.Username)
	}
	if request.Password != "" {
		smtpService.Password = request.Password
	}
	if strings.TrimSpace(request.FromEmail) != "" {
		smtpService.FromEmail = strings.TrimSpace(request.FromEmail)
	}
	if err := validateAlertSMTPService(smtpService); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := s.ensureUniqueAlertSMTPServiceName(smtpService.Name, smtpService.ID); err != nil {
		writeAlertNameConflict(c, err, "Alert SMTP service name already exists")
		return
	}

	if err := s.db.Save(&smtpService).Error; err != nil {
		s.logger.Error("Failed to update alert SMTP service", "error", err)
		utils.InternalError(c, "Failed to update alert SMTP service", err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Alert SMTP service updated successfully", gin.H{
		"smtp_service": alertSMTPServiceResponse(smtpService),
	})
}

// deleteAlertSMTPService deletes a reusable SMTP service when no destinations reference it.
// @Summary      Delete alert SMTP service
// @Description  Delete a reusable SMTP service. Existing delivery history is preserved.
// @Tags         alerts
// @Accept       json
// @Produce      json
// @ID           deleteAlertSMTPService
// @Param        id   path      string  true  "SMTP service ID"
// @Success      200  {object}  utils.APIResponse
// @Failure      404  {object}  utils.APIResponse
// @Failure      409  {object}  utils.APIResponse
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/alerts/smtp-services/{id} [delete]
func (s *Server) deleteAlertSMTPService(c *gin.Context) {
	var destinationCount int64
	if err := s.db.Model(&db.AlertEmailDestination{}).Where("smtp_service_id = ?", c.Param("id")).Count(&destinationCount).Error; err != nil {
		s.logger.Error("Failed to count alert email destinations", "error", err)
		utils.InternalError(c, "Failed to delete alert SMTP service", err)
		return
	}
	if destinationCount > 0 {
		utils.ErrorResponse(c, http.StatusConflict, "Alert SMTP service is used by email destinations", nil)
		return
	}

	result := s.db.Where("id = ?", c.Param("id")).Delete(&db.AlertSMTPService{})
	if result.Error != nil {
		s.logger.Error("Failed to delete alert SMTP service", "error", result.Error)
		utils.InternalError(c, "Failed to delete alert SMTP service", result.Error)
		return
	}
	if result.RowsAffected == 0 {
		utils.NotFound(c, "Alert SMTP service not found")
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Alert SMTP service deleted successfully", gin.H{})
}

// testAlertSMTPService verifies direct SMTP service connectivity.
// @Summary      Test alert SMTP service
// @Description  Connect to a reusable SMTP service and return a sanitized connectivity result without secret values.
// @Tags         alerts
// @Accept       json
// @Produce      json
// @ID           testAlertSMTPService
// @Param        id   path      string  true  "SMTP service ID"
// @Success      200  {object}  utils.APIResponse{data=object{test=service.AlertSMTPServiceTestResult}}
// @Failure      404  {object}  utils.APIResponse
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/alerts/smtp-services/{id}/test [post]
func (s *Server) testAlertSMTPService(c *gin.Context) {
	result, err := service.NewAlertService(s.db, s.logger, s.cfg).TestSMTPService(c.Param("id"))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			utils.NotFound(c, "Alert SMTP service not found")
			return
		}
		s.logger.Error("Failed to test alert SMTP service", "error", err)
		utils.InternalError(c, "Failed to test alert SMTP service", err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Alert SMTP service test completed", gin.H{
		"test": result,
	})
}

// listAlertEmailDestinations retrieves reusable email alert destinations.
// @Summary      List alert email destinations
// @Description  Get reusable email destinations and their last delivery status
// @Tags         alerts
// @Accept       json
// @Produce      json
// @ID           getAlertEmailDestinations
// @Success      200  {object}  utils.APIResponse{data=object{email_destinations=[]AlertEmailDestinationResponse,count=int}}
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/alerts/email-destinations [get]
func (s *Server) listAlertEmailDestinations(c *gin.Context) {
	var destinations []db.AlertEmailDestination
	if err := s.db.Order("name ASC").Find(&destinations).Error; err != nil {
		s.logger.Error("Failed to list alert email destinations", "error", err)
		utils.InternalError(c, "Failed to list alert email destinations", err)
		return
	}
	servicesByID, err := s.alertSMTPServicesByID(destinations)
	if err != nil {
		s.logger.Error("Failed to load alert SMTP services", "error", err)
		utils.InternalError(c, "Failed to list alert email destinations", err)
		return
	}

	responses := make([]AlertEmailDestinationResponse, 0, len(destinations))
	for _, destination := range destinations {
		responses = append(responses, s.alertEmailDestinationResponse(destination, servicesByID[destination.SMTPServiceID]))
	}

	utils.SuccessResponse(c, http.StatusOK, "Alert email destinations retrieved successfully", gin.H{
		"email_destinations": responses,
		"count":              len(responses),
	})
}

// createAlertEmailDestination creates a reusable email destination.
// @Summary      Create alert email destination
// @Description  Create a reusable email destination that sends through an SMTP service.
// @Tags         alerts
// @Accept       json
// @Produce      json
// @ID           createAlertEmailDestination
// @Param        request  body      alertEmailDestinationRequest  true  "Email destination payload"
// @Success      201      {object}  utils.APIResponse{data=object{email_destination=AlertEmailDestinationResponse}}
// @Failure      400      {object}  utils.APIResponse
// @Failure      404      {object}  utils.APIResponse
// @Failure      409      {object}  utils.APIResponse
// @Failure      500      {object}  utils.APIResponse
// @Router       /v1/alerts/email-destinations [post]
func (s *Server) createAlertEmailDestination(c *gin.Context) {
	var request alertEmailDestinationRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.BadRequest(c, "Invalid alert email destination payload")
		return
	}
	if err := validateAlertEvents(request.SubscribedEvents); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	destination := db.AlertEmailDestination{
		ID:               utils.GenerateID("alert_email_destination"),
		SMTPServiceID:    strings.TrimSpace(request.SMTPServiceID),
		Name:             strings.TrimSpace(request.Name),
		Enabled:          true,
		EmailTo:          strings.TrimSpace(request.EmailTo),
		SubscribedEvents: db.EncodeAlertEvents(normalizeAlertEvents(request.SubscribedEvents)),
	}
	if request.Enabled != nil {
		destination.Enabled = *request.Enabled
	}
	if err := validateAlertEmailDestination(destination); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	smtpService, ok, err := s.findAlertSMTPService(destination.SMTPServiceID)
	if err != nil {
		s.logger.Error("Failed to load alert SMTP service", "error", err)
		utils.InternalError(c, "Failed to create alert email destination", err)
		return
	}
	if !ok {
		utils.NotFound(c, "Alert SMTP service not found")
		return
	}
	if err := s.ensureUniqueAlertEmailDestinationName(destination.Name, ""); err != nil {
		writeAlertNameConflict(c, err, "Alert email destination name already exists")
		return
	}

	if err := s.db.Create(&destination).Error; err != nil {
		s.logger.Error("Failed to create alert email destination", "error", err)
		utils.InternalError(c, "Failed to create alert email destination", err)
		return
	}

	utils.SuccessResponse(c, http.StatusCreated, "Alert email destination created successfully", gin.H{
		"email_destination": s.alertEmailDestinationResponse(destination, smtpService),
	})
}

// updateAlertEmailDestination updates a reusable email destination.
// @Summary      Update alert email destination
// @Description  Update a reusable email destination.
// @Tags         alerts
// @Accept       json
// @Produce      json
// @ID           updateAlertEmailDestination
// @Param        id       path      string                         true  "Email destination ID"
// @Param        request  body      alertEmailDestinationRequest   true  "Email destination payload"
// @Success      200      {object}  utils.APIResponse{data=object{email_destination=AlertEmailDestinationResponse}}
// @Failure      400      {object}  utils.APIResponse
// @Failure      404      {object}  utils.APIResponse
// @Failure      409      {object}  utils.APIResponse
// @Failure      500      {object}  utils.APIResponse
// @Router       /v1/alerts/email-destinations/{id} [patch]
func (s *Server) updateAlertEmailDestination(c *gin.Context) {
	var destination db.AlertEmailDestination
	if err := s.db.Where("id = ?", c.Param("id")).First(&destination).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			utils.NotFound(c, "Alert email destination not found")
			return
		}
		s.logger.Error("Failed to load alert email destination", "error", err)
		utils.InternalError(c, "Failed to update alert email destination", err)
		return
	}

	var request alertEmailDestinationRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.BadRequest(c, "Invalid alert email destination payload")
		return
	}
	if strings.TrimSpace(request.SMTPServiceID) != "" {
		destination.SMTPServiceID = strings.TrimSpace(request.SMTPServiceID)
	}
	if strings.TrimSpace(request.Name) != "" {
		destination.Name = strings.TrimSpace(request.Name)
	}
	if request.Enabled != nil {
		destination.Enabled = *request.Enabled
	}
	if strings.TrimSpace(request.EmailTo) != "" {
		destination.EmailTo = strings.TrimSpace(request.EmailTo)
	}
	if request.SubscribedEvents != nil {
		if err := validateAlertEvents(request.SubscribedEvents); err != nil {
			utils.BadRequest(c, err.Error())
			return
		}
		destination.SubscribedEvents = db.EncodeAlertEvents(normalizeAlertEvents(request.SubscribedEvents))
	}
	if err := validateAlertEmailDestination(destination); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	smtpService, ok, err := s.findAlertSMTPService(destination.SMTPServiceID)
	if err != nil {
		s.logger.Error("Failed to load alert SMTP service", "error", err)
		utils.InternalError(c, "Failed to update alert email destination", err)
		return
	}
	if !ok {
		utils.NotFound(c, "Alert SMTP service not found")
		return
	}
	if err := s.ensureUniqueAlertEmailDestinationName(destination.Name, destination.ID); err != nil {
		writeAlertNameConflict(c, err, "Alert email destination name already exists")
		return
	}

	if err := s.db.Save(&destination).Error; err != nil {
		s.logger.Error("Failed to update alert email destination", "error", err)
		utils.InternalError(c, "Failed to update alert email destination", err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Alert email destination updated successfully", gin.H{
		"email_destination": s.alertEmailDestinationResponse(destination, smtpService),
	})
}

// deleteAlertEmailDestination deletes a reusable email destination.
// @Summary      Delete alert email destination
// @Description  Delete a reusable email destination. Existing delivery history is preserved.
// @Tags         alerts
// @Accept       json
// @Produce      json
// @ID           deleteAlertEmailDestination
// @Param        id   path      string  true  "Email destination ID"
// @Success      200  {object}  utils.APIResponse
// @Failure      404  {object}  utils.APIResponse
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/alerts/email-destinations/{id} [delete]
func (s *Server) deleteAlertEmailDestination(c *gin.Context) {
	result := s.db.Where("id = ?", c.Param("id")).Delete(&db.AlertEmailDestination{})
	if result.Error != nil {
		s.logger.Error("Failed to delete alert email destination", "error", result.Error)
		utils.InternalError(c, "Failed to delete alert email destination", result.Error)
		return
	}
	if result.RowsAffected == 0 {
		utils.NotFound(c, "Alert email destination not found")
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Alert email destination deleted successfully", gin.H{})
}

// testAlertEmailDestination sends a manual test email through a reusable destination.
// @Summary      Test alert email destination
// @Description  Send a manual test notification through a reusable email destination. Delivery errors are sanitized in the response.
// @Tags         alerts
// @Accept       json
// @Produce      json
// @ID           testAlertEmailDestination
// @Param        id   path      string  true  "Email destination ID"
// @Success      200  {object}  utils.APIResponse{data=object{delivery=AlertDeliveryResponse}}
// @Failure      404  {object}  utils.APIResponse
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/alerts/email-destinations/{id}/test [post]
func (s *Server) testAlertEmailDestination(c *gin.Context) {
	delivery, err := service.NewAlertService(s.db, s.logger, s.cfg).TestEmailDestination(c.Param("id"))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			utils.NotFound(c, "Alert email destination not found")
			return
		}
		s.logger.Error("Failed to test alert email destination", "error", err)
		utils.InternalError(c, "Failed to test alert email destination", err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Alert email destination test completed", gin.H{
		"delivery": alertDeliveryResponse(*delivery),
	})
}

// listAlertRoutes retrieves explicit alert routes.
// @Summary      List alert routes
// @Description  Get explicit alert routes ordered by priority
// @Tags         alerts
// @Accept       json
// @Produce      json
// @ID           getAlertRoutes
// @Success      200  {object}  utils.APIResponse{data=object{routes=[]AlertRouteResponse,count=int}}
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/alerts/routes [get]
func (s *Server) listAlertRoutes(c *gin.Context) {
	var routes []db.AlertRoute
	if err := s.db.Order("priority ASC, name ASC").Find(&routes).Error; err != nil {
		s.logger.Error("Failed to list alert routes", "error", err)
		utils.InternalError(c, "Failed to list alert routes", err)
		return
	}

	responses := alertRouteResponses(routes)
	utils.SuccessResponse(c, http.StatusOK, "Alert routes retrieved successfully", gin.H{
		"routes": responses,
		"count":  len(responses),
	})
}

// createAlertRoute creates an explicit alert route.
// @Summary      Create alert route
// @Description  Create a priority-ordered alert route that can target channels or suppress matching events
// @Tags         alerts
// @Accept       json
// @Produce      json
// @ID           createAlertRoute
// @Param        request  body      alertRouteRequest  true  "Alert route payload"
// @Success      201      {object}  utils.APIResponse{data=object{route=AlertRouteResponse}}
// @Failure      400      {object}  utils.APIResponse
// @Failure      409      {object}  utils.APIResponse
// @Failure      500      {object}  utils.APIResponse
// @Router       /v1/alerts/routes [post]
func (s *Server) createAlertRoute(c *gin.Context) {
	var request alertRouteRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.BadRequest(c, "Invalid alert route payload")
		return
	}

	route := db.AlertRoute{
		ID:                   utils.GenerateID("alert_route"),
		Name:                 strings.TrimSpace(request.Name),
		Enabled:              true,
		Priority:             100,
		EventTypes:           encodeStringList(normalizeAlertEvents(request.EventTypes)),
		Severities:           encodeStringList(normalizeStringList(request.Severities)),
		AgentIDs:             encodeStringList(normalizeStringList(request.AgentIDs)),
		MonitorIDs:           encodeStringList(normalizeStringList(request.MonitorIDs)),
		MonitorTypes:         encodeStringList(normalizeStringList(request.MonitorTypes)),
		ChannelIDs:           encodeStringList(normalizeStringList(request.ChannelIDs)),
		GroupingPolicy:       normalizeAlertGroupingPolicy(request.GroupingPolicy),
		GroupingDelaySeconds: db.DefaultAlertGroupingDelaySeconds,
	}
	if request.Enabled != nil {
		route.Enabled = *request.Enabled
	}
	if request.Priority != nil {
		route.Priority = *request.Priority
	}
	if request.Suppress != nil {
		route.Suppress = *request.Suppress
	}
	if request.GroupingDelaySeconds != nil {
		route.GroupingDelaySeconds = *request.GroupingDelaySeconds
	}
	if err := validateAlertEvents(request.EventTypes); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := validateAlertRoute(route); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := s.ensureAlertRouteChannelsExist(route); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := s.ensureUniqueAlertRouteName(route.Name, route.ID); err != nil {
		writeAlertNameConflict(c, err, "Alert route name already exists")
		return
	}

	if err := s.db.Create(&route).Error; err != nil {
		s.logger.Error("Failed to create alert route", "error", err)
		utils.InternalError(c, "Failed to create alert route", err)
		return
	}

	utils.SuccessResponse(c, http.StatusCreated, "Alert route created successfully", gin.H{
		"route": alertRouteResponse(route),
	})
}

// updateAlertRoute updates an explicit alert route.
// @Summary      Update alert route
// @Description  Update alert route match filters, destination channels, or suppression behavior
// @Tags         alerts
// @Accept       json
// @Produce      json
// @ID           updateAlertRoute
// @Param        id       path      string             true  "Alert route ID"
// @Param        request  body      alertRouteRequest  true  "Alert route payload"
// @Success      200      {object}  utils.APIResponse{data=object{route=AlertRouteResponse}}
// @Failure      400      {object}  utils.APIResponse
// @Failure      404      {object}  utils.APIResponse
// @Failure      409      {object}  utils.APIResponse
// @Failure      500      {object}  utils.APIResponse
// @Router       /v1/alerts/routes/{id} [patch]
func (s *Server) updateAlertRoute(c *gin.Context) {
	var route db.AlertRoute
	if err := s.db.Where("id = ?", c.Param("id")).First(&route).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			utils.NotFound(c, "Alert route not found")
			return
		}
		s.logger.Error("Failed to load alert route", "error", err)
		utils.InternalError(c, "Failed to update alert route", err)
		return
	}

	var request alertRouteRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.BadRequest(c, "Invalid alert route payload")
		return
	}

	if strings.TrimSpace(request.Name) != "" {
		route.Name = strings.TrimSpace(request.Name)
	}
	if request.Enabled != nil {
		route.Enabled = *request.Enabled
	}
	if request.Priority != nil {
		route.Priority = *request.Priority
	}
	if request.EventTypes != nil {
		if err := validateAlertEvents(request.EventTypes); err != nil {
			utils.BadRequest(c, err.Error())
			return
		}
		route.EventTypes = encodeStringList(normalizeAlertEvents(request.EventTypes))
	}
	if request.Severities != nil {
		route.Severities = encodeStringList(normalizeStringList(request.Severities))
	}
	if request.AgentIDs != nil {
		route.AgentIDs = encodeStringList(normalizeStringList(request.AgentIDs))
	}
	if request.MonitorIDs != nil {
		route.MonitorIDs = encodeStringList(normalizeStringList(request.MonitorIDs))
	}
	if request.MonitorTypes != nil {
		route.MonitorTypes = encodeStringList(normalizeStringList(request.MonitorTypes))
	}
	if request.ChannelIDs != nil {
		route.ChannelIDs = encodeStringList(normalizeStringList(request.ChannelIDs))
	}
	if request.Suppress != nil {
		route.Suppress = *request.Suppress
	}
	if strings.TrimSpace(request.GroupingPolicy) != "" {
		route.GroupingPolicy = normalizeAlertGroupingPolicy(request.GroupingPolicy)
	}
	if request.GroupingDelaySeconds != nil {
		route.GroupingDelaySeconds = *request.GroupingDelaySeconds
	}
	if err := validateAlertRoute(route); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := s.ensureAlertRouteChannelsExist(route); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := s.ensureUniqueAlertRouteName(route.Name, route.ID); err != nil {
		writeAlertNameConflict(c, err, "Alert route name already exists")
		return
	}

	if err := s.db.Save(&route).Error; err != nil {
		s.logger.Error("Failed to update alert route", "error", err)
		utils.InternalError(c, "Failed to update alert route", err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Alert route updated successfully", gin.H{
		"route": alertRouteResponse(route),
	})
}

// deleteAlertRoute deletes an explicit alert route.
// @Summary      Delete alert route
// @Description  Delete an alert route. Existing delivery history is preserved.
// @Tags         alerts
// @Accept       json
// @Produce      json
// @ID           deleteAlertRoute
// @Param        id   path      string  true  "Alert route ID"
// @Success      200  {object}  utils.APIResponse
// @Failure      404  {object}  utils.APIResponse
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/alerts/routes/{id} [delete]
func (s *Server) deleteAlertRoute(c *gin.Context) {
	result := s.db.Where("id = ?", c.Param("id")).Delete(&db.AlertRoute{})
	if result.Error != nil {
		s.logger.Error("Failed to delete alert route", "error", result.Error)
		utils.InternalError(c, "Failed to delete alert route", result.Error)
		return
	}
	if result.RowsAffected == 0 {
		utils.NotFound(c, "Alert route not found")
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Alert route deleted successfully", gin.H{})
}

// dryRunAlertRoutes evaluates alert routes without sending notifications.
// @Summary      Dry-run alert routes
// @Description  Explain route event matching, suppression, cooldown, and destination decisions without creating deliveries or sending notifications
// @Tags         alerts
// @Accept       json
// @Produce      json
// @ID           dryRunAlertRoutes
// @Param        request  body      alertRouteDryRunRequest  true  "Alert route dry-run payload"
// @Success      200      {object}  utils.APIResponse{data=object{dry_run=AlertRouteDryRunResponse}}
// @Failure      400      {object}  utils.APIResponse
// @Failure      404      {object}  utils.APIResponse
// @Failure      500      {object}  utils.APIResponse
// @Router       /v1/alerts/routes/dry-run [post]
func (s *Server) dryRunAlertRoutes(c *gin.Context) {
	var request alertRouteDryRunRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.BadRequest(c, "Invalid alert route dry-run payload")
		return
	}
	if !db.ValidAlertEvent(strings.TrimSpace(request.EventType)) {
		utils.BadRequest(c, "unsupported alert route event")
		return
	}

	alertService := service.NewAlertService(s.db, s.logger, s.cfg)
	event := service.AlertRouteContext{
		IncidentID:  strings.TrimSpace(request.IncidentID),
		EventType:   strings.TrimSpace(request.EventType),
		Severity:    strings.TrimSpace(request.Severity),
		AgentID:     strings.TrimSpace(request.AgentID),
		MonitorID:   strings.TrimSpace(request.MonitorID),
		MonitorType: strings.TrimSpace(request.MonitorType),
	}
	if event.IncidentID != "" {
		loaded, err := alertService.LoadAlertRouteContext(event.IncidentID, event.EventType)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				utils.NotFound(c, "Incident not found")
				return
			}
			s.logger.Error("Failed to load incident for alert route dry-run", "incident_id", event.IncidentID, "error", err)
			utils.InternalError(c, "Failed to dry-run alert routes", err)
			return
		}
		event = mergeAlertRouteContext(*loaded, event)
	}

	result, err := alertService.DryRunRoutes(event)
	if err != nil {
		s.logger.Error("Failed to dry-run alert routes", "error", err)
		utils.InternalError(c, "Failed to dry-run alert routes", err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Alert routes dry-run evaluated successfully", gin.H{
		"dry_run": alertRouteDryRunResponse(result),
	})
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
	var destinations []db.AlertEmailDestination
	if err := s.db.Order("name ASC").Find(&destinations).Error; err != nil {
		s.logger.Error("Failed to list alert email destinations for rules", "error", err)
		utils.InternalError(c, "Failed to list alert rules", err)
		return
	}
	servicesByID, err := s.alertSMTPServicesByID(destinations)
	if err != nil {
		s.logger.Error("Failed to load alert SMTP services for rules", "error", err)
		utils.InternalError(c, "Failed to list alert rules", err)
		return
	}
	for _, destination := range destinations {
		if smtpService, ok := servicesByID[destination.SMTPServiceID]; ok && destination.Enabled && smtpService.Enabled {
			targetChannels = append(targetChannels, destination.Name)
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
		ID:                         channel.ID,
		Name:                       channel.Name,
		Type:                       channel.Type,
		Enabled:                    channel.Enabled,
		WebhookURL:                 channel.WebhookURL,
		WebhookConfigured:          channel.WebhookURL != "",
		WebhookSignatureConfigured: channel.WebhookSigningSecret != "",
		EmailToConfigured:          channel.EmailTo != "",
		EmailFromConfigured:        channel.EmailFrom != "",
		SMTPHostConfigured:         channel.SMTPHost != "",
		SMTPPortConfigured:         channel.SMTPPort > 0,
		SMTPUsernameConfigured:     channel.SMTPUsername != "",
		SubscribedEvents:           db.DecodeAlertEvents(channel.SubscribedEvents),
		CreatedAt:                  channel.CreatedAt,
		UpdatedAt:                  channel.UpdatedAt,
	}

	var delivery db.AlertDelivery
	result := s.db.Where("channel = ?", channel.Name).Order("created_at DESC").Limit(1).Find(&delivery)
	if result.Error == nil && result.RowsAffected > 0 {
		response.LastDeliveryStatus = delivery.Status
		response.LastDeliveryAt = &delivery.CreatedAt
	}

	return response
}

func alertSMTPServiceResponse(smtpService db.AlertSMTPService) AlertSMTPServiceResponse {
	return AlertSMTPServiceResponse{
		ID:                 smtpService.ID,
		Name:               smtpService.Name,
		Enabled:            smtpService.Enabled,
		Host:               smtpService.Host,
		Port:               smtpService.Port,
		FromEmail:          smtpService.FromEmail,
		UsernameConfigured: smtpService.Username != "",
		PasswordConfigured: smtpService.Password != "",
		CreatedAt:          smtpService.CreatedAt,
		UpdatedAt:          smtpService.UpdatedAt,
	}
}

func (s *Server) alertEmailDestinationResponse(destination db.AlertEmailDestination, smtpService db.AlertSMTPService) AlertEmailDestinationResponse {
	response := AlertEmailDestinationResponse{
		ID:               destination.ID,
		SMTPServiceID:    destination.SMTPServiceID,
		SMTPServiceName:  smtpService.Name,
		Name:             destination.Name,
		Enabled:          destination.Enabled,
		EmailTo:          destination.EmailTo,
		SubscribedEvents: db.DecodeAlertEvents(destination.SubscribedEvents),
		CreatedAt:        destination.CreatedAt,
		UpdatedAt:        destination.UpdatedAt,
	}

	var delivery db.AlertDelivery
	result := s.db.Where("channel = ?", destination.Name).Order("created_at DESC").Limit(1).Find(&delivery)
	if result.Error == nil && result.RowsAffected > 0 {
		response.LastDeliveryStatus = delivery.Status
		response.LastDeliveryAt = &delivery.CreatedAt
	}

	return response
}

func (s *Server) alertSMTPServicesByID(destinations []db.AlertEmailDestination) (map[string]db.AlertSMTPService, error) {
	ids := make([]string, 0, len(destinations))
	seen := map[string]bool{}
	for _, destination := range destinations {
		if destination.SMTPServiceID == "" || seen[destination.SMTPServiceID] {
			continue
		}
		seen[destination.SMTPServiceID] = true
		ids = append(ids, destination.SMTPServiceID)
	}
	servicesByID := map[string]db.AlertSMTPService{}
	if len(ids) == 0 {
		return servicesByID, nil
	}

	var services []db.AlertSMTPService
	if err := s.db.Where("id IN ?", ids).Find(&services).Error; err != nil {
		return nil, err
	}
	for _, service := range services {
		servicesByID[service.ID] = service
	}
	return servicesByID, nil
}

func (s *Server) findAlertSMTPService(id string) (db.AlertSMTPService, bool, error) {
	var smtpService db.AlertSMTPService
	if err := s.db.Where("id = ?", id).First(&smtpService).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return db.AlertSMTPService{}, false, nil
		}
		return db.AlertSMTPService{}, false, err
	}
	return smtpService, true, nil
}

func (s *Server) ensureUniqueAlertSMTPServiceName(name string, excludedID string) error {
	query := s.db.Model(&db.AlertSMTPService{}).Where("name = ?", name)
	if excludedID != "" {
		query = query.Where("id <> ?", excludedID)
	}
	var existing int64
	if err := query.Count(&existing).Error; err != nil {
		s.logger.Error("Failed to check alert SMTP service name", "error", err)
		return err
	}
	if existing > 0 {
		return gorm.ErrDuplicatedKey
	}
	return nil
}

func (s *Server) ensureUniqueAlertEmailDestinationName(name string, excludedID string) error {
	query := s.db.Model(&db.AlertEmailDestination{}).Where("name = ?", name)
	if excludedID != "" {
		query = query.Where("id <> ?", excludedID)
	}
	var existing int64
	if err := query.Count(&existing).Error; err != nil {
		s.logger.Error("Failed to check alert email destination name", "error", err)
		return err
	}
	if existing > 0 {
		return gorm.ErrDuplicatedKey
	}
	return nil
}

func (s *Server) ensureUniqueAlertRouteName(name string, excludedID string) error {
	query := s.db.Model(&db.AlertRoute{}).Where("name = ?", name)
	if excludedID != "" {
		query = query.Where("id <> ?", excludedID)
	}
	var existing int64
	if err := query.Count(&existing).Error; err != nil {
		s.logger.Error("Failed to check alert route name", "error", err)
		return err
	}
	if existing > 0 {
		return gorm.ErrDuplicatedKey
	}
	return nil
}

func (s *Server) ensureAlertRouteChannelsExist(route db.AlertRoute) error {
	channelIDs := decodeResponseList(route.ChannelIDs, nil)
	if len(channelIDs) == 0 || route.Suppress {
		return nil
	}

	var channelCount int64
	if err := s.db.Model(&db.AlertChannel{}).Where("id IN ?", channelIDs).Count(&channelCount).Error; err != nil {
		return err
	}
	var destinationCount int64
	if err := s.db.Model(&db.AlertEmailDestination{}).Where("id IN ?", channelIDs).Count(&destinationCount).Error; err != nil {
		return err
	}
	if int(channelCount+destinationCount) != len(channelIDs) {
		return &requestValidationError{message: "alert route channel_ids must reference existing alert channels or email destinations"}
	}
	return nil
}

func writeAlertNameConflict(c *gin.Context, err error, message string) {
	if err == gorm.ErrDuplicatedKey {
		utils.ErrorResponse(c, http.StatusConflict, message, nil)
		return
	}
	utils.InternalError(c, message, err)
}

func alertOptionalSecret(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func validateAlertChannel(channel db.AlertChannel) error {
	if strings.TrimSpace(channel.Name) == "" {
		return &requestValidationError{message: "alert channel name is required"}
	}
	switch channel.Type {
	case "webhook", "slack", "discord":
		if strings.TrimSpace(channel.WebhookURL) == "" {
			return &requestValidationError{message: channel.Type + " alert channel requires webhook_url"}
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

func validateAlertSMTPService(smtpService db.AlertSMTPService) error {
	if strings.TrimSpace(smtpService.Name) == "" {
		return &requestValidationError{message: "alert SMTP service name is required"}
	}
	if strings.TrimSpace(smtpService.Host) == "" || smtpService.Port <= 0 || strings.TrimSpace(smtpService.FromEmail) == "" {
		return &requestValidationError{message: "alert SMTP service requires host, port, and from_email"}
	}
	return nil
}

func validateAlertEmailDestination(destination db.AlertEmailDestination) error {
	if strings.TrimSpace(destination.Name) == "" {
		return &requestValidationError{message: "alert email destination name is required"}
	}
	if strings.TrimSpace(destination.SMTPServiceID) == "" {
		return &requestValidationError{message: "alert email destination requires smtp_service_id"}
	}
	if strings.TrimSpace(destination.EmailTo) == "" {
		return &requestValidationError{message: "alert email destination requires email_to"}
	}
	for _, event := range db.DecodeAlertEvents(destination.SubscribedEvents) {
		if !db.ValidAlertEvent(event) {
			return &requestValidationError{message: "unsupported alert email destination event"}
		}
	}
	return nil
}

func validateAlertRoute(route db.AlertRoute) error {
	if strings.TrimSpace(route.Name) == "" {
		return &requestValidationError{message: "alert route name is required"}
	}
	if route.Priority < 0 {
		return &requestValidationError{message: "alert route priority must be zero or greater"}
	}
	for _, event := range decodeResponseList(route.EventTypes, db.DefaultAlertEvents()) {
		if !db.ValidAlertEvent(event) {
			return &requestValidationError{message: "unsupported alert route event"}
		}
	}
	for _, severity := range decodeResponseList(route.Severities, nil) {
		if !validAlertRouteSeverity(severity) {
			return &requestValidationError{message: "unsupported alert route severity"}
		}
	}
	if !validAlertGroupingPolicy(route.GroupingPolicy) {
		return &requestValidationError{message: "unsupported alert route grouping_policy"}
	}
	if route.GroupingDelaySeconds <= 0 {
		return &requestValidationError{message: "alert route grouping_delay_seconds must be greater than zero"}
	}
	if !route.Suppress && len(decodeResponseList(route.ChannelIDs, nil)) == 0 {
		return &requestValidationError{message: "alert route requires channel_ids unless suppress is true"}
	}
	return nil
}

func validAlertRouteSeverity(severity string) bool {
	switch severity {
	case "low", "medium", "high", "critical", "error":
		return true
	default:
		return false
	}
}

func validAlertGroupingPolicy(policy string) bool {
	switch normalizeAlertGroupingPolicy(policy) {
	case db.AlertGroupingPolicySuppress, db.AlertGroupingPolicyDelayedSummary, db.AlertGroupingPolicyNone:
		return true
	default:
		return false
	}
}

func normalizeAlertGroupingPolicy(policy string) string {
	switch strings.TrimSpace(policy) {
	case "", db.AlertGroupingPolicySuppress:
		return db.AlertGroupingPolicySuppress
	case db.AlertGroupingPolicyDelayedSummary:
		return db.AlertGroupingPolicyDelayedSummary
	case db.AlertGroupingPolicyNone:
		return db.AlertGroupingPolicyNone
	default:
		return strings.TrimSpace(policy)
	}
}

func normalizeAlertGroupingDelaySeconds(value int) int {
	if value <= 0 {
		return db.DefaultAlertGroupingDelaySeconds
	}
	return value
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

func normalizeStringList(values []string) []string {
	normalized := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		normalized = append(normalized, value)
	}
	return normalized
}

func encodeStringList(values []string) string {
	if len(values) == 0 {
		return ""
	}
	body, err := json.Marshal(values)
	if err != nil {
		return ""
	}
	return string(body)
}

func mergeAlertRouteContext(loaded service.AlertRouteContext, overrides service.AlertRouteContext) service.AlertRouteContext {
	loaded.EventType = overrides.EventType
	if overrides.Severity != "" {
		loaded.Severity = overrides.Severity
	}
	if overrides.AgentID != "" {
		loaded.AgentID = overrides.AgentID
	}
	if overrides.MonitorID != "" {
		loaded.MonitorID = overrides.MonitorID
	}
	if overrides.MonitorType != "" {
		loaded.MonitorType = overrides.MonitorType
	}
	return loaded
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
