package api

import (
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"orion/core/internal/utils"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

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
	utils.SuccessResponse(c, http.StatusOK, "Alert rule updated successfully", gin.H{"rule": alertRuleResponse(rule)})
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
	utils.SuccessResponse(c, http.StatusOK, "Alert rules dry-run evaluated successfully", gin.H{"dry_run": alertRuleDryRunResponse(result)})
}
func (s *Server) alertChannelResponse(channel db.AlertChannel) AlertChannelResponse {
	response := AlertChannelResponse{ID: channel.ID, Name: channel.Name, Type: channel.Type, Enabled: channel.Enabled, WebhookURL: channel.WebhookURL, WebhookConfigured: channel.WebhookURL != "", WebhookSignatureConfigured: channel.WebhookSigningSecret != "", SubscribedEvents: db.DecodeAlertEvents(channel.SubscribedEvents), CreatedAt: channel.CreatedAt, UpdatedAt: channel.UpdatedAt}
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
	rule := db.AlertRoute{Name: strings.TrimSpace(request.Name), Enabled: true, Priority: 100, EventTypes: encodeStringList(normalizeAlertEvents(request.EventTypes)), Severities: encodeStringList(normalizeStringList(request.Severities)), AgentIDs: encodeStringList(normalizeStringList(request.AgentIDs)), MonitorIDs: encodeStringList(normalizeStringList(request.MonitorIDs)), MonitorTypes: encodeStringList(normalizeStringList(request.MonitorTypes)), ChannelIDs: encodeStringList(normalizeStringList(request.ChannelIDs)), GroupingPolicy: normalizeAlertGroupingPolicy(request.GroupingPolicy), GroupingDelaySeconds: db.DefaultAlertGroupingDelaySeconds}
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
	event := service.AlertRouteContext{IncidentID: strings.TrimSpace(request.IncidentID), EventType: strings.TrimSpace(request.EventType), Severity: strings.TrimSpace(request.Severity), AgentID: strings.TrimSpace(request.AgentID), MonitorID: strings.TrimSpace(request.MonitorID), MonitorType: strings.TrimSpace(request.MonitorType)}
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
	event := service.AlertRouteContext{IncidentID: strings.TrimSpace(request.IncidentID), EventType: strings.TrimSpace(request.EventType), Severity: strings.TrimSpace(request.Severity), AgentID: strings.TrimSpace(request.AgentID), MonitorID: strings.TrimSpace(request.MonitorID), MonitorType: strings.TrimSpace(request.MonitorType)}
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
