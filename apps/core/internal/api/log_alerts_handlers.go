package api

import (
	"orion/core/internal/utils"

	"github.com/gin-gonic/gin"
)

// swag resolves utils.APIResponse through this file's imports.
var _ utils.APIResponse

// listLogAlertRules lists log alert rules.
// @Summary      List log alert rules
// @Description  Rules that raise an incident when an app's logs match a condition.
// @Tags         logs
// @Produce      json
// @Security     BearerAuth
// @ID           listLogAlertRules
// @Success      200  {object}  utils.APIResponse{data=object{rules=[]LogAlertRule}}
// @Failure      401  {object}  utils.APIResponse
// @Router       /v1/logs/alerts [get]
func (s *Server) listLogAlertRules(c *gin.Context) { s.listLogAlertRulesImpl(c) }

// createLogAlertRule creates a log alert rule.
// @Summary      Create a log alert rule
// @Description  Threshold rules fire when more than `threshold` matching entries arrive inside the window; absence rules fire when none do. A firing rule opens one incident (Webhook and Email alerts follow the normal routes) and resolves when the condition clears. Rules are evaluated by the Worker about every 30 seconds.
// @Tags         logs
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           createLogAlertRule
// @Param        request  body      LogAlertRuleRequest  true  "Rule"
// @Success      201  {object}  utils.APIResponse{data=object{rule=LogAlertRule}}
// @Failure      400  {object}  utils.APIResponse
// @Failure      401  {object}  utils.APIResponse
// @Router       /v1/logs/alerts [post]
func (s *Server) createLogAlertRule(c *gin.Context) { s.createLogAlertRuleImpl(c) }

// updateLogAlertRule changes a log alert rule.
// @Summary      Update a log alert rule
// @Description  Omitted fields are unchanged. Disabling a rule resolves its open incident.
// @Tags         logs
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           updateLogAlertRule
// @Param        id       path      string                     true  "Rule ID"
// @Param        request  body      LogAlertRuleUpdateRequest  true  "Fields to change"
// @Success      200  {object}  utils.APIResponse{data=object{rule=LogAlertRule}}
// @Failure      400  {object}  utils.APIResponse
// @Failure      401  {object}  utils.APIResponse
// @Failure      404  {object}  utils.APIResponse
// @Router       /v1/logs/alerts/{id} [patch]
func (s *Server) updateLogAlertRule(c *gin.Context) { s.updateLogAlertRuleImpl(c) }

// deleteLogAlertRule deletes a log alert rule.
// @Summary      Delete a log alert rule
// @Description  Resolves the rule's open incident, then removes the rule.
// @Tags         logs
// @Produce      json
// @Security     BearerAuth
// @ID           deleteLogAlertRule
// @Param        id   path      string  true  "Rule ID"
// @Success      200  {object}  utils.APIResponse
// @Failure      401  {object}  utils.APIResponse
// @Failure      404  {object}  utils.APIResponse
// @Router       /v1/logs/alerts/{id} [delete]
func (s *Server) deleteLogAlertRule(c *gin.Context) { s.deleteLogAlertRuleImpl(c) }

// getIncidentLogs returns logs around an incident.
// @Summary      Logs around an incident
// @Description  Entries from 15 minutes before the incident opened until 5 minutes after it resolved (or now): the node's service logs, plus the app's product logs when `app_id` is given or the incident came from a log alert rule. Newest first.
// @Tags         incidents
// @Produce      json
// @Security     BearerAuth
// @ID           getIncidentLogs
// @Param        id      path   string  true   "Incident ID"
// @Param        app_id  query  string  false  "Also include this app's product logs"
// @Param        limit   query  int     false  "Maximum entries, default 100, max 200"
// @Success      200  {object}  utils.APIResponse{data=LogListResponse}
// @Failure      401  {object}  utils.APIResponse
// @Failure      404  {object}  utils.APIResponse
// @Router       /v1/incidents/{id}/logs [get]
func (s *Server) getIncidentLogs(c *gin.Context) { s.getIncidentLogsImpl(c) }
