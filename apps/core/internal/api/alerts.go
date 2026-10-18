package api

import (
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"orion/core/internal/utils"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type alertChannelRequest struct {
	Name                 string   `json:"name"`
	Type                 string   `json:"type"`
	Enabled              *bool    `json:"enabled"`
	WebhookURL           string   `json:"webhook_url"`
	WebhookSigningSecret *string  `json:"webhook_signing_secret"`
	SubscribedEvents     []string `json:"subscribed_events"`
	// Headers are custom webhook request headers; sending the field replaces all of them. Values are never returned.
	Headers *map[string]string `json:"headers"`
	// EmailDeliveryMode is instant or digest_15m (email channels).
	EmailDeliveryMode  *string `json:"email_delivery_mode"`
	EmailSubjectPrefix *string `json:"email_subject_prefix"`
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
type alertRuleRequest struct {
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
	// CooldownSeconds limits repeat notifications for this rule (0 = global default, max 7 days).
	CooldownSeconds *int `json:"cooldown_seconds"`
}
type alertRouteDryRunRequest struct {
	IncidentID  string `json:"incident_id"`
	EventType   string `json:"event_type"`
	Severity    string `json:"severity"`
	AgentID     string `json:"agent_id"`
	MonitorID   string `json:"monitor_id"`
	MonitorType string `json:"monitor_type"`
}
type alertRuleDryRunRequest struct {
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
// @Param        status       query     string  false  "Comma-separated delivery statuses (sent,failed,cooldown,suppressed,pending)"
// @Param        type         query     string  false  "Filter by delivery channel type"
// @Param        channel      query     string  false  "Filter by delivery channel name"
// @Param        channel_id   query     string  false  "Filter by channel id"
// @Param        since        query     string  false  "Only deliveries since: 1h, 24h, 7d style duration or RFC3339"
// @Param        q            query     string  false  "Text search over channel, rule name and incident title"
// @Param        event_type   query     string  false  "Filter by alert event type"
// @Param        limit        query     int     false  "Maximum number of deliveries to return" default(50)
// @Param        offset       query     int     false  "Number of deliveries to skip" default(0)
// @Success      200          {object}  utils.APIResponse{data=object{deliveries=[]AlertDeliveryResponse,count=int64,limit=int,offset=int,pagination=utils.PaginationMeta}}
// @Failure      500          {object}  utils.APIResponse
// @Router       /v1/alerts/deliveries [get]
func (s *Server) listAlertDeliveries(c *gin.Context) {
	limit := utils.QueryInt(c, "limit", 50)
	offset := utils.QueryInt(c, "offset", 0)
	query := s.db.Model(&db.AlertDelivery{})
	if incidentID := c.Query("incident_id"); incidentID != "" {
		query = query.Where("incident_id = ?", incidentID)
	}
	if statuses := parseStatusSet(c.Query("status")); len(statuses) > 0 {
		query = query.Where("status IN ?", setKeys(statuses))
	}
	if channelID := strings.TrimSpace(c.Query("channel_id")); channelID != "" {
		query = query.Where("channel IN (SELECT name FROM alert_channels WHERE id = ?)", channelID)
	}
	if raw := strings.TrimSpace(c.Query("since")); raw != "" {
		since, err := parseSinceParam(raw, time.Now().UTC())
		if err != nil {
			utils.BadRequest(c, "since must be a duration like 24h or 7d, or an RFC3339 time")
			return
		}
		query = query.Where("created_at >= ?", since)
	}
	if q := strings.TrimSpace(c.Query("q")); q != "" {
		like := "%" + incidentLikeEscaper.Replace(strings.ToLower(q)) + "%"
		query = query.Where(`LOWER(channel) LIKE ? ESCAPE '\' OR route_id IN (SELECT id FROM alert_routes WHERE LOWER(name) LIKE ? ESCAPE '\') OR incident_id IN (SELECT id FROM incidents WHERE LOWER(title) LIKE ? ESCAPE '\')`, like, like, like)
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
	if err := query.Preload("Attempts", func(tx *gorm.DB) *gorm.DB {
		return tx.Order("attempt_number ASC")
	}).Order("created_at DESC").Limit(limit).Offset(offset).Find(&deliveries).Error; err != nil {
		s.logger.Error("Failed to list alert deliveries", "error", err)
		utils.InternalError(c, "Failed to list alert deliveries", err)
		return
	}
	responses := alertDeliveryResponses(deliveries)
	s.enrichDeliveries(responses)
	utils.SuccessResponse(c, http.StatusOK, "Alert deliveries retrieved successfully", gin.H{"deliveries": responses, "count": count, "total": count, "limit": limit, "offset": offset, "pagination": utils.NewPaginationMeta(count, limit, offset, len(responses))})
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
	if err := s.db.Where("type IN ?", []string{"webhook", "email"}).Order("name ASC").Find(&dbChannels).Error; err != nil {
		s.logger.Error("Failed to list alert channels", "error", err)
		utils.InternalError(c, "Failed to list alert channels", err)
		return
	}
	channels := make([]AlertChannelResponse, 0, len(dbChannels))
	for _, channel := range dbChannels {
		channels = append(channels, s.alertChannelResponse(channel))
	}
	utils.SuccessResponse(c, http.StatusOK, "Alert channels retrieved successfully", gin.H{"channels": channels, "count": len(channels)})
}

// createAlertChannel creates a persisted alert channel.
// @Summary      Create alert channel
// @Description  Create a webhook or email alert channel. Email channels need SMTP configured and recipients confirmed via double opt-in. Secret values are stored but never returned by the API.
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
	channel := db.AlertChannel{ID: utils.GenerateID("alert_channel"), Name: strings.TrimSpace(request.Name), Type: strings.TrimSpace(request.Type), Enabled: true, WebhookURL: strings.TrimSpace(request.WebhookURL), WebhookSigningSecret: alertOptionalSecret(request.WebhookSigningSecret), SubscribedEvents: db.EncodeAlertEvents(normalizeAlertEvents(request.SubscribedEvents))}
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
	if err := applyChannelExtras(&channel, request); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := validateAlertChannel(s.cfg, channel); err != nil {
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
	utils.SuccessResponse(c, http.StatusCreated, "Alert channel created successfully", gin.H{"channel": s.alertChannelResponse(channel)})
}

// updateAlertChannel updates a persisted alert channel.
// @Summary      Update alert channel
// @Description  Update a generic webhook alert channel.
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
	if request.SubscribedEvents != nil {
		if err := validateAlertEvents(request.SubscribedEvents); err != nil {
			utils.BadRequest(c, err.Error())
			return
		}
		channel.SubscribedEvents = db.EncodeAlertEvents(normalizeAlertEvents(request.SubscribedEvents))
	}
	if err := applyChannelExtras(&channel, request); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := validateAlertChannel(s.cfg, channel); err != nil {
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
	utils.SuccessResponse(c, http.StatusOK, "Alert channel updated successfully", gin.H{"channel": s.alertChannelResponse(channel)})
}

// testAlertChannel sends a manual test notification through a persisted alert channel.
// @Summary      Test alert channel
// @Description  Send a manual test notification through a configured generic webhook alert channel.
// @Tags         alerts
// @Accept       json
// @Produce      json
// @ID           testAlertChannel
// @Param        id   path      string  true  "Alert channel ID"
// @Success      200  {object}  utils.APIResponse{data=object{delivery=AlertDeliveryResponse,result=AlertTestResult}}
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
	var channel db.AlertChannel
	s.db.Where("id = ?", c.Param("id")).First(&channel)
	var stored db.AlertDelivery
	s.db.Preload("Attempts").Where("id = ?", delivery.ID).First(&stored)
	utils.SuccessResponse(c, http.StatusOK, "Alert channel test completed", gin.H{"delivery": alertDeliveryResponse(stored), "result": s.alertTestResultFor(channel, stored)})
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
	_ = s.db.Where("channel_id = ?", c.Param("id")).Delete(&db.AlertChannelRecipient{}).Error
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
	utils.SuccessResponse(c, http.StatusOK, "Alert routes retrieved successfully", gin.H{"routes": responses, "count": len(responses)})
}
