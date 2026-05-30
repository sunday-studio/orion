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
	SubscribedEvents     []string `json:"subscribed_events"`
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
	if err := s.db.Where("type = ?", "webhook").Order("name ASC").Find(&dbChannels).Error; err != nil {
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
// @Description  Create a generic webhook alert channel. Secret values are stored but never returned by the API.
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
// @Description  Send a manual test notification through a configured generic webhook alert channel.
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
		if isAlertNameConflict(err) {
			writeAlertNameConflict(c, err, "Alert route name already exists")
			return
		}
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
		if isAlertNameConflict(err) {
			writeAlertNameConflict(c, err, "Alert route name already exists")
			return
		}
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

// listAlertRules retrieves webhook alert rules.
// @Summary      List alert rules
// @Description  Get persisted webhook alert rules ordered by priority
// @Tags         alerts
// @Accept       json
// @Produce      json
// @ID           getAlertRules
// @Success      200  {object}  utils.APIResponse{data=object{rules=[]AlertRuleResponse,count=int}}
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/alerts/rules [get]
func (s *Server) listAlertRules(c *gin.Context) {
	var rules []db.AlertRoute
	if err := s.db.Order("priority ASC, name ASC").Find(&rules).Error; err != nil {
		s.logger.Error("Failed to list alert rules", "error", err)
		utils.InternalError(c, "Failed to list alert rules", err)
		return
	}

	responses := alertRuleResponses(rules)
	utils.SuccessResponse(c, http.StatusOK, "Alert rules retrieved successfully", gin.H{
		"rules": responses,
		"count": len(responses),
	})
}

// createAlertRule creates a webhook alert rule.
// @Summary      Create alert rule
// @Description  Create a priority-ordered alert rule that targets webhook channels or suppresses matching events
// @Tags         alerts
// @Accept       json
// @Produce      json
// @ID           createAlertRule
// @Param        request  body      alertRuleRequest  true  "Alert rule payload"
// @Success      201      {object}  utils.APIResponse{data=object{rule=AlertRuleResponse}}
// @Failure      400      {object}  utils.APIResponse
// @Failure      409      {object}  utils.APIResponse
// @Failure      500      {object}  utils.APIResponse
// @Router       /v1/alerts/rules [post]
func (s *Server) createAlertRule(c *gin.Context) {
	var request alertRuleRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.BadRequest(c, "Invalid alert rule payload")
		return
	}

	rule := alertRouteFromRuleRequest(request)
	rule.ID = utils.GenerateID("alert_rule")
	if err := validateAlertEvents(request.EventTypes); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := validateAlertRule(rule); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := s.ensureAlertRuleWebhookChannelsExist(rule); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := s.ensureUniqueAlertRouteName(rule.Name, rule.ID); err != nil {
		writeAlertNameConflict(c, err, "Alert rule name already exists")
		return
	}

	if err := s.db.Create(&rule).Error; err != nil {
		if isAlertNameConflict(err) {
			writeAlertNameConflict(c, err, "Alert rule name already exists")
			return
		}
		s.logger.Error("Failed to create alert rule", "error", err)
		utils.InternalError(c, "Failed to create alert rule", err)
		return
	}

	utils.SuccessResponse(c, http.StatusCreated, "Alert rule created successfully", gin.H{
		"rule": alertRuleResponse(rule),
	})
}

// updateAlertRule updates a webhook alert rule.
// @Summary      Update alert rule
// @Description  Update alert rule match filters, webhook channels, or suppression behavior
// @Tags         alerts
// @Accept       json
// @Produce      json
// @ID           updateAlertRule
// @Param        id       path      string            true  "Alert rule ID"
// @Param        request  body      alertRuleRequest  true  "Alert rule payload"
// @Success      200      {object}  utils.APIResponse{data=object{rule=AlertRuleResponse}}
// @Failure      400      {object}  utils.APIResponse
// @Failure      404      {object}  utils.APIResponse
// @Failure      409      {object}  utils.APIResponse
// @Failure      500      {object}  utils.APIResponse
// @Router       /v1/alerts/rules/{id} [patch]
func (s *Server) updateAlertRule(c *gin.Context) {
	var rule db.AlertRoute
	if err := s.db.Where("id = ?", c.Param("id")).First(&rule).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			utils.NotFound(c, "Alert rule not found")
			return
		}
		s.logger.Error("Failed to load alert rule", "error", err)
		utils.InternalError(c, "Failed to update alert rule", err)
		return
	}

	var request alertRuleRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.BadRequest(c, "Invalid alert rule payload")
		return
	}
	if request.EventTypes != nil {
		if err := validateAlertEvents(request.EventTypes); err != nil {
			utils.BadRequest(c, "invalid event_types")
			return
		}
	}

	mergeAlertRuleRequest(&rule, request)
	if err := validateAlertRule(rule); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := s.ensureAlertRuleWebhookChannelsExist(rule); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := s.ensureUniqueAlertRouteName(rule.Name, rule.ID); err != nil {
		writeAlertNameConflict(c, err, "Alert rule name already exists")
		return
	}

	if err := s.db.Save(&rule).Error; err != nil {
		if isAlertNameConflict(err) {
			writeAlertNameConflict(c, err, "Alert rule name already exists")
			return
		}
		s.logger.Error("Failed to update alert rule", "error", err)
		utils.InternalError(c, "Failed to update alert rule", err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Alert rule updated successfully", gin.H{
		"rule": alertRuleResponse(rule),
	})
}

// deleteAlertRule deletes a webhook alert rule.
// @Summary      Delete alert rule
// @Description  Delete an alert rule. Existing delivery history is preserved.
// @Tags         alerts
// @Accept       json
// @Produce      json
// @ID           deleteAlertRule
// @Param        id   path      string  true  "Alert rule ID"
// @Success      200  {object}  utils.APIResponse
// @Failure      404  {object}  utils.APIResponse
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/alerts/rules/{id} [delete]
func (s *Server) deleteAlertRule(c *gin.Context) {
	result := s.db.Where("id = ?", c.Param("id")).Delete(&db.AlertRoute{})
	if result.Error != nil {
		s.logger.Error("Failed to delete alert rule", "error", result.Error)
		utils.InternalError(c, "Failed to delete alert rule", result.Error)
		return
	}
	if result.RowsAffected == 0 {
		utils.NotFound(c, "Alert rule not found")
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Alert rule deleted successfully", gin.H{})
}

// enableAlertRule enables a webhook alert rule.
// @Summary      Enable alert rule
// @Description  Enable an alert rule without changing its filters or webhook destinations.
// @Tags         alerts
// @Accept       json
// @Produce      json
// @ID           enableAlertRule
// @Param        id   path      string  true  "Alert rule ID"
// @Success      200  {object}  utils.APIResponse{data=object{rule=AlertRuleResponse}}
// @Failure      404  {object}  utils.APIResponse
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/alerts/rules/{id}/enable [post]
func (s *Server) enableAlertRule(c *gin.Context) {
	s.setAlertRuleEnabled(c, true)
}

// disableAlertRule disables a webhook alert rule.
// @Summary      Disable alert rule
// @Description  Disable an alert rule without changing its filters or webhook destinations.
// @Tags         alerts
// @Accept       json
// @Produce      json
// @ID           disableAlertRule
// @Param        id   path      string  true  "Alert rule ID"
// @Success      200  {object}  utils.APIResponse{data=object{rule=AlertRuleResponse}}
// @Failure      404  {object}  utils.APIResponse
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/alerts/rules/{id}/disable [post]
func (s *Server) disableAlertRule(c *gin.Context) {
	s.setAlertRuleEnabled(c, false)
}

func (s *Server) setAlertRuleEnabled(c *gin.Context, enabled bool) {
	var rule db.AlertRoute
	if err := s.db.Where("id = ?", c.Param("id")).First(&rule).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			utils.NotFound(c, "Alert rule not found")
			return
		}
		s.logger.Error("Failed to load alert rule", "error", err)
		utils.InternalError(c, "Failed to update alert rule", err)
		return
	}
	rule.Enabled = enabled
	if err := s.db.Save(&rule).Error; err != nil {
		s.logger.Error("Failed to update alert rule", "error", err)
		utils.InternalError(c, "Failed to update alert rule", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Alert rule updated successfully", gin.H{
		"rule": alertRuleResponse(rule),
	})
}

// dryRunAlertRules evaluates webhook alert rules without sending notifications.
// @Summary      Dry-run alert rules
// @Description  Explain rule event matching, suppression, cooldown, grouping, and webhook destination decisions without creating deliveries or sending notifications
// @Tags         alerts
// @Accept       json
// @Produce      json
// @ID           dryRunAlertRules
// @Param        request  body      alertRuleDryRunRequest  true  "Alert rule dry-run payload"
// @Success      200      {object}  utils.APIResponse{data=object{dry_run=AlertRuleDryRunResponse}}
// @Failure      400      {object}  utils.APIResponse
// @Failure      404      {object}  utils.APIResponse
// @Failure      500      {object}  utils.APIResponse
// @Router       /v1/alerts/rules/dry-run [post]
func (s *Server) dryRunAlertRules(c *gin.Context) {
	event, ok := s.alertRuleDryRunEventFromRequest(c)
	if !ok {
		return
	}
	result, err := service.NewAlertService(s.db, s.logger, s.cfg).DryRunRoutes(event)
	if err != nil {
		s.logger.Error("Failed to dry-run alert rules", "error", err)
		utils.InternalError(c, "Failed to dry-run alert rules", err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Alert rules dry-run evaluated successfully", gin.H{
		"dry_run": alertRuleDryRunResponse(result),
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
	if err := s.db.Model(&db.AlertChannel{}).Where("id IN ? AND type = ?", channelIDs, "webhook").Count(&channelCount).Error; err != nil {
		return err
	}
	if int(channelCount) != len(channelIDs) {
		return &requestValidationError{message: "alert route channel_ids must reference existing webhook alert channels"}
	}
	return nil
}

func (s *Server) ensureAlertRuleWebhookChannelsExist(rule db.AlertRoute) error {
	channelIDs := decodeResponseList(rule.ChannelIDs, nil)
	if len(channelIDs) == 0 || rule.Suppress {
		return nil
	}

	var channelCount int64
	if err := s.db.Model(&db.AlertChannel{}).Where("id IN ? AND type = ?", channelIDs, "webhook").Count(&channelCount).Error; err != nil {
		return err
	}
	if int(channelCount) != len(channelIDs) {
		return &requestValidationError{message: "alert rule channel_ids must reference existing webhook alert channels"}
	}
	return nil
}

func alertRouteFromRuleRequest(request alertRuleRequest) db.AlertRoute {
	rule := db.AlertRoute{
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
		rule.Enabled = *request.Enabled
	}
	if request.Priority != nil {
		rule.Priority = *request.Priority
	}
	if request.Suppress != nil {
		rule.Suppress = *request.Suppress
	}
	if request.GroupingDelaySeconds != nil {
		rule.GroupingDelaySeconds = *request.GroupingDelaySeconds
	}
	return rule
}

func mergeAlertRuleRequest(rule *db.AlertRoute, request alertRuleRequest) {
	if strings.TrimSpace(request.Name) != "" {
		rule.Name = strings.TrimSpace(request.Name)
	}
	if request.Enabled != nil {
		rule.Enabled = *request.Enabled
	}
	if request.Priority != nil {
		rule.Priority = *request.Priority
	}
	if request.EventTypes != nil {
		rule.EventTypes = encodeStringList(normalizeAlertEvents(request.EventTypes))
	}
	if request.Severities != nil {
		rule.Severities = encodeStringList(normalizeStringList(request.Severities))
	}
	if request.AgentIDs != nil {
		rule.AgentIDs = encodeStringList(normalizeStringList(request.AgentIDs))
	}
	if request.MonitorIDs != nil {
		rule.MonitorIDs = encodeStringList(normalizeStringList(request.MonitorIDs))
	}
	if request.MonitorTypes != nil {
		rule.MonitorTypes = encodeStringList(normalizeStringList(request.MonitorTypes))
	}
	if request.ChannelIDs != nil {
		rule.ChannelIDs = encodeStringList(normalizeStringList(request.ChannelIDs))
	}
	if request.Suppress != nil {
		rule.Suppress = *request.Suppress
	}
	if strings.TrimSpace(request.GroupingPolicy) != "" {
		rule.GroupingPolicy = normalizeAlertGroupingPolicy(request.GroupingPolicy)
	}
	if request.GroupingDelaySeconds != nil {
		rule.GroupingDelaySeconds = *request.GroupingDelaySeconds
	}
}

func (s *Server) alertDryRunEventFromRequest(c *gin.Context, resourceName string) (service.AlertRouteContext, bool) {
	var request alertRouteDryRunRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.BadRequest(c, "Invalid alert "+resourceName+" dry-run payload")
		return service.AlertRouteContext{}, false
	}
	if !db.ValidAlertEvent(strings.TrimSpace(request.EventType)) {
		utils.BadRequest(c, "unsupported alert "+resourceName+" event")
		return service.AlertRouteContext{}, false
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
				return service.AlertRouteContext{}, false
			}
			s.logger.Error("Failed to load incident for alert "+resourceName+" dry-run", "incident_id", event.IncidentID, "error", err)
			utils.InternalError(c, "Failed to dry-run alert "+resourceName+"s", err)
			return service.AlertRouteContext{}, false
		}
		event = mergeAlertRouteContext(*loaded, event)
	}
	return event, true
}

func (s *Server) alertRuleDryRunEventFromRequest(c *gin.Context) (service.AlertRouteContext, bool) {
	var request alertRuleDryRunRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.BadRequest(c, "Invalid alert rule dry-run payload")
		return service.AlertRouteContext{}, false
	}
	if !db.ValidAlertEvent(strings.TrimSpace(request.EventType)) {
		utils.BadRequest(c, "unsupported alert rule event")
		return service.AlertRouteContext{}, false
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
				return service.AlertRouteContext{}, false
			}
			s.logger.Error("Failed to load incident for alert rule dry-run", "incident_id", event.IncidentID, "error", err)
			utils.InternalError(c, "Failed to dry-run alert rules", err)
			return service.AlertRouteContext{}, false
		}
		event = mergeAlertRouteContext(*loaded, event)
	}
	return event, true
}

func writeAlertNameConflict(c *gin.Context, err error, message string) {
	if isAlertNameConflict(err) {
		utils.ErrorResponse(c, http.StatusConflict, message, nil)
		return
	}
	utils.InternalError(c, message, err)
}

func isAlertNameConflict(err error) bool {
	if err == nil {
		return false
	}
	if err == gorm.ErrDuplicatedKey {
		return true
	}
	errText := strings.ToLower(err.Error())
	return strings.Contains(errText, "unique") || strings.Contains(errText, "duplicate")
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
	if channel.Type != "webhook" {
		return &requestValidationError{message: "unsupported alert channel type"}
	}
	if strings.TrimSpace(channel.WebhookURL) == "" {
		return &requestValidationError{message: "webhook alert channel requires webhook_url"}
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

func validateAlertRule(rule db.AlertRoute) error {
	if err := validateAlertRoute(rule); err != nil {
		if validationErr, ok := err.(*requestValidationError); ok {
			return &requestValidationError{message: strings.ReplaceAll(validationErr.message, "alert route", "alert rule")}
		}
		return err
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
