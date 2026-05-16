package api

import (
	"net/http"
	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/utils"

	"github.com/gin-gonic/gin"
)

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

// listAlertChannels retrieves redacted alert channel configuration.
// @Summary      List alert channels
// @Description  Get redacted configured alert channels and their last delivery status
// @Tags         alerts
// @Accept       json
// @Produce      json
// @ID           getAlertChannels
// @Success      200  {object}  utils.APIResponse{data=object{channels=[]AlertChannelResponse,count=int}}
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/alerts/channels [get]
func (s *Server) listAlertChannels(c *gin.Context) {
	channels := make([]AlertChannelResponse, 0, len(s.cfg.AlertChannels))
	for _, channel := range s.cfg.AlertChannels {
		channels = append(channels, s.alertChannelResponse(channel))
	}

	utils.SuccessResponse(c, http.StatusOK, "Alert channels retrieved successfully", gin.H{
		"channels": channels,
		"count":    len(channels),
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
	targetChannels := make([]string, 0, len(s.cfg.AlertChannels))
	for _, channel := range s.cfg.AlertChannels {
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

func (s *Server) alertChannelResponse(channel config.AlertChannelConfig) AlertChannelResponse {
	response := AlertChannelResponse{
		Name:                   channel.Name,
		Type:                   channel.Type,
		Enabled:                channel.Enabled,
		WebhookConfigured:      channel.WebhookURL != "",
		EmailToConfigured:      channel.EmailTo != "",
		EmailFromConfigured:    channel.EmailFrom != "",
		SMTPHostConfigured:     channel.SMTPHost != "",
		SMTPPortConfigured:     channel.SMTPPort > 0,
		SMTPUsernameConfigured: channel.SMTPUsername != "",
	}

	var delivery db.AlertDelivery
	result := s.db.Where("channel = ?", channel.Name).Order("created_at DESC").Limit(1).Find(&delivery)
	if result.Error == nil && result.RowsAffected > 0 {
		response.LastDeliveryStatus = delivery.Status
		response.LastDeliveryAt = &delivery.CreatedAt
	}

	return response
}
