package api

import (
	"errors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"orion/core/internal/utils"
	"sort"
	"strconv"
	"strings"
)

func incidentNextActions(incident db.Incident, monitor db.Monitor, deliveries []db.// acknowledgeIncident manually acknowledges an active incident.
// @Summary      Acknowledge incident
// @Description  Mark an active incident as acknowledged and record a manual incident event
// @Tags         incidents
// @Accept       json
// @Produce      json
// @ID           acknowledgeIncident
// @Param        id   path      string  true  "Incident ID"
// @Param        request  body  incidentLifecycleActionRequest  false  "Lifecycle action metadata"
// @Success      200  {object}  utils.APIResponse{data=object{incident=IncidentResponse}}
// @Failure      400  {object}  utils.APIResponse
// @Failure      404  {object}  utils.APIResponse
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/incidents/{id}/acknowledge [post]
// resolveIncident manually resolves an active incident.
// @Summary      Resolve incident
// @Description  Mark an active incident as resolved, clear its monitor active incident path, and record a manual incident event
// @Tags         incidents
// @Accept       json
// @Produce      json
// @ID           resolveIncident
// @Param        id   path      string  true  "Incident ID"
// @Param        request  body  incidentLifecycleActionRequest  false  "Lifecycle action metadata"
// @Success      200  {object}  utils.APIResponse{data=object{incident=IncidentResponse}}
// @Failure      400  {object}  utils.APIResponse
// @Failure      404  {object}  utils.APIResponse
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/incidents/{id}/resolve [post]
// coverIncident marks an active incident as covered.
// @Summary      Cover incident
// @Description  Mark an active incident as covered, optionally until a future timestamp, and record a manual incident event
// @Tags         incidents
// @Accept       json
// @Produce      json
// @ID           coverIncident
// @Param        id       path      string                   true   "Incident ID"
// @Param        request  body      incidentCoverageRequest  false  "Coverage payload"
// @Success      200      {object}  utils.APIResponse{data=object{incident=IncidentResponse}}
// @Failure      400      {object}  utils.APIResponse
// @Failure      404      {object}  utils.APIResponse
// @Failure      500      {object}  utils.APIResponse
// @Router       /v1/incidents/{id}/cover [post]
// reopenIncident reopens a resolved or covered incident.
// @Summary      Reopen incident
// @Description  Reopen a covered or resolved incident, restore its monitor active incident path, and record a manual incident event
// @Tags         incidents
// @Accept       json
// @Produce      json
// @ID           reopenIncident
// @Param        id   path      string  true  "Incident ID"
// @Param        request  body  incidentLifecycleActionRequest  false  "Lifecycle action metadata"
// @Success      200  {object}  utils.APIResponse{data=object{incident=IncidentResponse}}
// @Failure      400  {object}  utils.APIResponse
// @Failure      404  {object}  utils.APIResponse
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/incidents/{id}/reopen [post]
AlertDelivery, reports []db.MonitorReport, related []IncidentRelatedIncidentResponse) []IncidentNextActionResponse {
	actions := make([]IncidentNextActionResponse, 0, 5)
	switch incident.Status {
	case "open":
		actions = append(actions, IncidentNextActionResponse{ID: "acknowledge-incident", Label: "Acknowledge", Description: "Mark ownership before recovery work starts.", ActionType: "acknowledge_incident", Priority: 10, TargetKind: "incident", TargetID: incident.ID})
		fallthrough
	case "acknowledged":
		actions = append(actions, IncidentNextActionResponse{ID: "cover-incident", Label: "Cover", Description: "Suppress noise while a known mitigation is in progress.", ActionType: "cover_incident", Priority: 20, TargetKind: "incident", TargetID: incident.ID})
	case "covered", "resolved":
		actions = append(actions, IncidentNextActionResponse{ID: "reopen-incident", Label: "Reopen", Description: "Resume active incident handling if recovery did not hold.", ActionType: "reopen_incident", Priority: 20, TargetKind: "incident", TargetID: incident.ID})
	}
	if incident.Status != "resolved" {
		actions = append(actions, IncidentNextActionResponse{ID: "resolve-incident", Label: "Resolve", Description: "Close the incident after the latest check confirms recovery.", ActionType: "resolve_incident", Priority: 40, TargetKind: "incident", TargetID: incident.ID})
	}
	if incident.MonitorID != "" && incidentNeedsMonitorTuning(incident, reports, related) {
		monitorName := monitor.Name
		if strings.TrimSpace(monitorName) == "" {
			monitorName = incident.MonitorID
		}
		actions = append(actions, IncidentNextActionResponse{ID: "review-monitor-tuning", Label: "Tune monitor", Description: "Review confirmation, recovery, and timeout settings for " + monitorName + ".", ActionType: "review_monitor_tuning", Priority: 30, TargetKind: "monitor", TargetID: incident.MonitorID, TargetTab: "config"})
	}
	if failedCount := failedAlertDeliveryCount(deliveries); failedCount > 0 {
		deliveryLabel := " failed delivery attempt needs alert destination review."
		if failedCount > 1 {
			deliveryLabel = " failed delivery attempts need alert destination review."
		}
		actions = append(actions, IncidentNextActionResponse{ID: "review-failed-notifications", Label: "Recover notifications", Description: strconv.Itoa(failedCount) + deliveryLabel, ActionType: "review_failed_notifications", Priority: 35, TargetKind: "alert_deliveries", TargetID: incident.ID, TargetTab: "logs", FilterStatus: "failed"})
	}
	sort.SliceStable(actions, func(i, j int) bool {
		if actions[i].Priority == actions[j].Priority {
			return actions[i].ID < actions[j].ID
		}
		return actions[i].Priority < actions[j].Priority
	})
	return actions
}
func incidentNeedsMonitorTuning(incident db.Incident, reports []db.MonitorReport, related []IncidentRelatedIncidentResponse) bool {
	if len(related) > 0 {
		return true
	}
	if incident.Status == "open" || incident.Status == "acknowledged" || incident.Status == "covered" {
		return true
	}
	for _, report := range reports {
		switch report.Health {
		case "down", "degraded", "stale":
			return true
		}
	}
	return false
}
func failedAlertDeliveryCount(deliveries []db.AlertDelivery) int {
	count := 0
	for _, delivery := range deliveries {
		if delivery.Status == "failed" {
			count++
		}
	}
	return count
}

func (s *Server) acknowledgeIncident(c *gin.Context) {
	request, ok := bindIncidentLifecycleActionRequest(c)
	if !ok {
		return
	}
	incident, err := service.NewIncidentService(s.db, s.logger, s.cfg).AcknowledgeIncident(c.Param("id"), s.incidentLifecycleActionMetadata(c, request.Note))
	if err != nil {
		s.handleIncidentActionError(c, err, "Failed to acknowledge incident")
		return
	}
	s.writeIncidentActionResponse(c, "Incident acknowledged successfully", incident)
}

func (s *Server) resolveIncident(c *gin.Context) {
	request, ok := bindIncidentLifecycleActionRequest(c)
	if !ok {
		return
	}
	incident, err := service.NewIncidentService(s.db, s.logger, s.cfg).ResolveIncident(c.Param("id"), s.incidentLifecycleActionMetadata(c, request.Note))
	if err != nil {
		s.handleIncidentActionError(c, err, "Failed to resolve incident")
		return
	}
	s.writeIncidentActionResponse(c, "Incident resolved successfully", incident)
}

func (s *Server) coverIncident(c *gin.Context) {
	var request incidentCoverageRequest
	if c.Request.Body != nil && c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&request); err != nil {
			utils.BadRequest(c, "Invalid incident coverage payload")
			return
		}
	}
	incident, err := service.NewIncidentService(s.db, s.logger, s.cfg).CoverIncident(c.Param("id"), request.CoveredUntil, request.Note, s.incidentLifecycleActionMetadata(c, request.Note))
	if err != nil {
		s.handleIncidentActionError(c, err, "Failed to cover incident")
		return
	}
	s.writeIncidentActionResponse(c, "Incident covered successfully", incident)
}

func (s *Server) reopenIncident(c *gin.Context) {
	request, ok := bindIncidentLifecycleActionRequest(c)
	if !ok {
		return
	}
	incident, err := service.NewIncidentService(s.db, s.logger, s.cfg).ReopenIncident(c.Param("id"), s.incidentLifecycleActionMetadata(c, request.Note))
	if err != nil {
		s.handleIncidentActionError(c, err, "Failed to reopen incident")
		return
	}
	s.writeIncidentActionResponse(c, "Incident reopened successfully", incident)
}
func bindIncidentLifecycleActionRequest(c *gin.Context) (incidentLifecycleActionRequest, bool) {
	var request incidentLifecycleActionRequest
	if c.Request.Body != nil && c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&request); err != nil {
			utils.BadRequest(c, "Invalid incident lifecycle action payload")
			return incidentLifecycleActionRequest{}, false
		}
	}
	return request, true
}
func (s *Server) incidentLifecycleActionMetadata(c *gin.Context, note string) service.IncidentLifecycleActionMetadata {
	actorType, actorID := incidentLifecycleActor(c)
	return service.IncidentLifecycleActionMetadata{ActorType: actorType, ActorID: actorID, Note: note}
}
func incidentLifecycleActor(c *gin.Context) (string, string) {
	if actor, ok := c.Get("frontend_actor_id"); ok {
		if actorID, ok := actor.(string); ok && strings.TrimSpace(actorID) != "" {
			return "user", strings.TrimSpace(actorID)
		}
	}
	return "user", "console"
}
func (s *Server) handleIncidentActionError(c *gin.Context, err error, message string) {
	switch {
	case errors.Is(err, service.ErrIncidentNotFound):
		utils.NotFound(c, "Incident not found")
	case errors.Is(err, service.ErrIncidentAlreadyResolved):
		utils.BadRequest(c, "Incident is already resolved")
	default:
		s.logger.Error(message, "error", err)
		utils.InternalError(c, message, err)
	}
}
func (s *Server) writeIncidentActionResponse(c *gin.Context, message string, incident db.Incident) {
	var agent db.Agent
	if err := s.db.Where("id = ?", incident.AgentID).First(&agent).Error; err != nil {
		agent = db.Agent{ID: incident.AgentID, Name: incident.AgentID}
	}
	var monitor db.Monitor
	if err := s.db.Where("id = ?", incident.MonitorID).First(&monitor).Error; err != nil {
		monitor = db.Monitor{ID: incident.MonitorID, Name: incident.MonitorID}
	}
	utils.SuccessResponse(c, http.StatusOK, message, gin.H{"incident": incidentResponse(incident, agent, monitor)})
}
func (s *Server) loadIncidentContext(c *gin.Context) (db.Incident, db.Agent, db.Monitor, bool) {
	incidentID := c.Param("id")
	var incident db.Incident
	if err := s.db.Where("id = ?", incidentID).First(&incident).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			utils.NotFound(c, "Incident not found")
			return db.Incident{}, db.Agent{}, db.Monitor{}, false
		}
		s.logger.Error("Failed to get incident", "incident_id", incidentID, "error", err)
		utils.InternalError(c, "Failed to get incident", err)
		return db.Incident{}, db.Agent{}, db.Monitor{}, false
	}
	var agent db.Agent
	if err := s.db.Where("id = ?", incident.AgentID).First(&agent).Error; err != nil {
		agent = db.Agent{ID: incident.AgentID, Name: incident.AgentID}
	}
	var monitor db.Monitor
	if err := s.db.Where("id = ?", incident.MonitorID).First(&monitor).Error; err != nil {
		monitor = db.Monitor{ID: incident.MonitorID, Name: incident.MonitorID}
	}
	return incident, agent, monitor, true
}
func (s *Server) incidentEvents(incidentID string) ([]db.IncidentEvent, error) {
	var events []db.IncidentEvent
	err := s.db.Where("incident_id = ?", incidentID).Order("created_at ASC").Find(&events).Error
	return events, err
}
func (s *Server) incidentAlertDeliveries(incidentID string) ([]db.AlertDelivery, error) {
	var deliveries []db.AlertDelivery
	err := s.db.Preload("Attempts", func(tx *gorm.DB) *gorm.DB {
		return tx.Order("attempt_number ASC")
	}).Where("incident_id = ?", incidentID).Order("created_at ASC").Find(&deliveries).Error
	return deliveries, err
}
func (s *Server) incidentMonitorReports(monitorID string, events []db.IncidentEvent) ([]db.MonitorReport, error) {
	reportIDs := make([]string, 0, len(events))
	for _, event := range events {
		if event.MonitorReportID != "" {
			reportIDs = append(reportIDs, event.MonitorReportID)
		}
	}
	var reports []db.MonitorReport
	if len(reportIDs) > 0 {
		if err := s.db.Where("id IN ?", reportIDs).Order("created_at ASC").Find(&reports).Error; err != nil {
			return nil, err
		}
		return reports, nil
	}
	err := s.db.Where("monitor_id = ?", monitorID).Order("created_at DESC").Limit(10).Find(&reports).Error
	return reports, err
}
func (s *Server) incidentEvidence(incident db.Incident, events []db.IncidentEvent) (IncidentEvidenceResponse, error) {
	var response IncidentEvidenceResponse
	triggeringReportID := ""
	for _, event := range events {
		if event.MonitorReportID == "" {
			continue
		}
		if event.Type == "incident_opened" {
			triggeringReportID = event.MonitorReportID
			break
		}
		if triggeringReportID == "" {
			triggeringReportID = event.MonitorReportID
		}
	}
	if triggeringReportID != "" {
		var report db.MonitorReport
		err := s.db.Where("id = ? AND monitor_id = ?", triggeringReportID, incident.MonitorID).First(&report).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return response, err
		}
		if err == nil {
			reportResponse := monitorReportResponse(report)
			response.TriggeringReport = &reportResponse
		}
	}
	var latestReport db.MonitorReport
	err := s.db.Where("monitor_id = ?", incident.MonitorID).Order("created_at DESC").First(&latestReport).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return response, err
	}
	if err == nil {
		reportResponse := monitorReportResponse(latestReport)
		response.LatestReport = &reportResponse
	}
	return response, nil
}
func (s *Server) relatedIncidents(incident db.Incident) ([]IncidentRelatedIncidentResponse, error) {
	var incidents []db.Incident
	if err := s.db.Where("monitor_id = ? AND id <> ?", incident.MonitorID, incident.ID).Order("opened_at DESC").Limit(5).Find(&incidents).Error; err != nil {
		return nil, err
	}
	responses := make([]IncidentRelatedIncidentResponse, 0, len(incidents))
	for _, related := range incidents {
		responses = append(responses, IncidentRelatedIncidentResponse{ID: related.ID, Status: related.Status, Severity: related.Severity, Title: related.Title, ResolutionKind: related.ResolutionKind, OpenedAt: related.OpenedAt, ResolvedAt: related.ResolvedAt, LastEventAt: related.LastEventAt, LatestEvent: related.LatestEvent, NotificationStatus: related.NotificationStatus})
	}
	return responses, nil
}
