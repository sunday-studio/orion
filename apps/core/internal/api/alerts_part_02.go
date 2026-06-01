package api

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"orion/core/internal/utils"
	"strings"
)

func (s *Server) createAlertRoute(c *gin.Context) {
	var request alertRouteRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.BadRequest(c, "Invalid alert route payload")
		return
	}
	route := db.AlertRoute{ID: utils.GenerateID("alert_route"), Name: strings.TrimSpace(request.Name), Enabled: true, Priority: 100, EventTypes: encodeStringList(normalizeAlertEvents(request.EventTypes)), Severities: encodeStringList(normalizeStringList(request.Severities)), AgentIDs: encodeStringList(normalizeStringList(request.AgentIDs)), MonitorIDs: encodeStringList(normalizeStringList(request.MonitorIDs)), MonitorTypes: encodeStringList(normalizeStringList(request.MonitorTypes)), ChannelIDs: encodeStringList(normalizeStringList(request.ChannelIDs)), GroupingPolicy: normalizeAlertGroupingPolicy(request.GroupingPolicy), GroupingDelaySeconds: db.DefaultAlertGroupingDelaySeconds}
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
	utils.SuccessResponse(c, http.StatusCreated, "Alert route created successfully", gin.H{"route": alertRouteResponse(route)})
}

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
	utils.SuccessResponse(c, http.StatusOK, "Alert route updated successfully", gin.H{"route": alertRouteResponse(route)})
}

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
	event := service.AlertRouteContext{IncidentID: strings.TrimSpace(request.IncidentID), EventType: strings.TrimSpace(request.EventType), Severity: strings.TrimSpace(request.Severity), AgentID: strings.TrimSpace(request.AgentID), MonitorID: strings.TrimSpace(request.MonitorID), MonitorType: strings.TrimSpace(request.MonitorType)}
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
	utils.SuccessResponse(c, http.StatusOK, "Alert routes dry-run evaluated successfully", gin.H{"dry_run": alertRouteDryRunResponse(result)})
}

func (s *Server) listAlertRules(c *gin.Context) {
	var rules []db.// createAlertRoute creates an explicit alert route.
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
	AlertRoute
	if err := s.db.Order("priority ASC, name ASC").Find(&rules).Error; err != nil {
		s.logger.Error("Failed to list alert rules", "error", err)
		utils.InternalError(c, "Failed to list alert rules", err)
		return
	}
	responses := alertRuleResponses(rules)
	utils.SuccessResponse(c, http.StatusOK, "Alert rules retrieved successfully", gin.H{"rules": responses, "count": len(responses)})
}

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
	utils.SuccessResponse(c, http.StatusCreated, "Alert rule created successfully", gin.H{"rule": alertRuleResponse(rule)})
}

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
	utils.SuccessResponse(c, http.StatusOK, "Alert rule updated successfully", gin.H{"rule": alertRuleResponse(rule)})
}

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

func (s *Server) enableAlertRule(c *gin.Context) {
	s.setAlertRuleEnabled(c, true)
}
