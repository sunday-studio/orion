package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"orion/core/internal/utils"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type incidentCoverageRequest struct {
	CoveredUntil *time.Time `json:"covered_until"`
	Note         string     `json:"note"`
}

// listIncidents retrieves persisted incidents.
// @Summary      List incidents
// @Description  Get a paginated list of persisted incidents. Defaults to active incidents.
// @Tags         incidents
// @Accept       json
// @Produce      json
// @ID           getIncidents
// @Param        status  query     string  false  "Comma-separated incident statuses" default(open,acknowledged,covered)
// @Param        agent_id  query   string  false  "Filter incidents by agent ID"
// @Param        monitor_id  query  string  false  "Filter incidents by monitor ID"
// @Param        needs_review  query  bool  false  "Filter to incidents with failed notifications or high/critical/error severity"
// @Param        limit   query     int     false  "Maximum number of incidents to return" default(50)
// @Param        offset  query     int     false  "Number of incidents to skip" default(0)
// @Success      200     {object}  utils.APIResponse{data=object{incidents=[]IncidentResponse,count=int64,limit=int,offset=int,pagination=utils.PaginationMeta,status=[]string}}
// @Failure      500     {object}  utils.APIResponse
// @Router       /v1/incidents [get]
func (s *Server) listIncidents(c *gin.Context) {
	limit := queryInt(c, "limit", 50)
	offset := queryInt(c, "offset", 0)
	statuses := queryStatuses(c.DefaultQuery("status", "open,acknowledged,covered"))
	agentID := strings.TrimSpace(c.Query("agent_id"))
	monitorID := strings.TrimSpace(c.Query("monitor_id"))
	needsReview := queryBool(c, "needs_review", false)

	query := s.db.Model(&db.Incident{}).Where("status IN ?", statuses)
	if agentID != "" {
		query = query.Where("agent_id = ?", agentID)
	}
	if monitorID != "" {
		query = query.Where("monitor_id = ?", monitorID)
	}
	if needsReview {
		query = query.Where("notification_status = ? OR severity IN ?", "failed", []string{"high", "critical", "error"})
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		s.logger.Error("Failed to count incidents", "error", err)
		utils.InternalError(c, "Failed to list incidents", err)
		return
	}

	var incidents []db.Incident
	if err := query.Order("last_event_at DESC").Limit(limit).Offset(offset).Find(&incidents).Error; err != nil {
		s.logger.Error("Failed to list incidents", "error", err)
		utils.InternalError(c, "Failed to list incidents", err)
		return
	}

	responses := make([]IncidentResponse, 0, len(incidents))
	for _, incident := range incidents {
		var agent db.Agent
		if err := s.db.Where("id = ?", incident.AgentID).First(&agent).Error; err != nil {
			agent = db.Agent{ID: incident.AgentID, Name: incident.AgentID}
		}

		var monitor db.Monitor
		if err := s.db.Where("id = ?", incident.MonitorID).First(&monitor).Error; err != nil {
			monitor = db.Monitor{ID: incident.MonitorID, Name: incident.MonitorID}
		}

		responses = append(responses, incidentResponse(incident, agent, monitor))
	}

	utils.SuccessResponse(c, 200, "Incidents retrieved successfully", gin.H{
		"incidents":  responses,
		"count":      count,
		"limit":      limit,
		"offset":     offset,
		"pagination": utils.NewPaginationMeta(count, limit, offset, len(responses)),
		"status":     statuses,
	})
}

// getIncidentDetail retrieves one incident with linked operational data.
// @Summary      Get incident detail
// @Description  Get one incident with related timeline events, alert deliveries, and monitor reports
// @Tags         incidents
// @Accept       json
// @Produce      json
// @ID           getIncident
// @Param        id   path      string  true  "Incident ID"
// @Success      200  {object}  utils.APIResponse{data=object{incident=IncidentResponse,timeline=[]IncidentTimelineItemResponse,events=[]IncidentEventResponse,alert_deliveries=[]AlertDeliveryResponse,monitor_reports=[]MonitorReportResponse}}
// @Failure      404  {object}  utils.APIResponse
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/incidents/{id} [get]
func (s *Server) getIncidentDetail(c *gin.Context) {
	incident, agent, monitor, ok := s.loadIncidentContext(c)
	if !ok {
		return
	}

	events, err := s.incidentEvents(incident.ID)
	if err != nil {
		s.logger.Error("Failed to load incident events", "incident_id", incident.ID, "error", err)
		utils.InternalError(c, "Failed to get incident detail", err)
		return
	}

	deliveries, err := s.incidentAlertDeliveries(incident.ID)
	if err != nil {
		s.logger.Error("Failed to load incident alert deliveries", "incident_id", incident.ID, "error", err)
		utils.InternalError(c, "Failed to get incident detail", err)
		return
	}

	reports, err := s.incidentMonitorReports(incident.MonitorID, events)
	if err != nil {
		s.logger.Error("Failed to load incident monitor reports", "incident_id", incident.ID, "error", err)
		utils.InternalError(c, "Failed to get incident detail", err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Incident retrieved successfully", gin.H{
		"incident":         incidentResponse(incident, agent, monitor),
		"timeline":         incidentTimeline(events, deliveries, reports),
		"events":           incidentEventResponses(events),
		"alert_deliveries": alertDeliveryResponses(deliveries),
		"monitor_reports":  monitorReportResponses(reports),
	})
}

// getIncidentTimeline retrieves one incident timeline.
// @Summary      Get incident timeline
// @Description  Get normalized incident timeline events including alert delivery attempts
// @Tags         incidents
// @Accept       json
// @Produce      json
// @ID           getIncidentTimeline
// @Param        id   path      string  true  "Incident ID"
// @Success      200  {object}  utils.APIResponse{data=object{timeline=[]IncidentTimelineItemResponse,count=int}}
// @Failure      404  {object}  utils.APIResponse
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/incidents/{id}/timeline [get]
func (s *Server) getIncidentTimeline(c *gin.Context) {
	incident, _, _, ok := s.loadIncidentContext(c)
	if !ok {
		return
	}

	events, err := s.incidentEvents(incident.ID)
	if err != nil {
		s.logger.Error("Failed to load incident events", "incident_id", incident.ID, "error", err)
		utils.InternalError(c, "Failed to get incident timeline", err)
		return
	}
	deliveries, err := s.incidentAlertDeliveries(incident.ID)
	if err != nil {
		s.logger.Error("Failed to load incident alert deliveries", "incident_id", incident.ID, "error", err)
		utils.InternalError(c, "Failed to get incident timeline", err)
		return
	}
	reports, err := s.incidentMonitorReports(incident.MonitorID, events)
	if err != nil {
		s.logger.Error("Failed to load incident monitor reports", "incident_id", incident.ID, "error", err)
		utils.InternalError(c, "Failed to get incident timeline", err)
		return
	}

	timeline := incidentTimeline(events, deliveries, reports)
	utils.SuccessResponse(c, http.StatusOK, "Incident timeline retrieved successfully", gin.H{
		"timeline": timeline,
		"count":    len(timeline),
	})
}

// acknowledgeIncident manually acknowledges an active incident.
// @Summary      Acknowledge incident
// @Description  Mark an active incident as acknowledged and record a manual incident event
// @Tags         incidents
// @Accept       json
// @Produce      json
// @ID           acknowledgeIncident
// @Param        id   path      string  true  "Incident ID"
// @Success      200  {object}  utils.APIResponse{data=object{incident=IncidentResponse}}
// @Failure      400  {object}  utils.APIResponse
// @Failure      404  {object}  utils.APIResponse
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/incidents/{id}/acknowledge [post]
func (s *Server) acknowledgeIncident(c *gin.Context) {
	incident, err := service.NewIncidentService(s.db, s.logger, s.cfg).AcknowledgeIncident(c.Param("id"))
	if err != nil {
		s.handleIncidentActionError(c, err, "Failed to acknowledge incident")
		return
	}
	s.writeIncidentActionResponse(c, "Incident acknowledged successfully", incident)
}

// resolveIncident manually resolves an active incident.
// @Summary      Resolve incident
// @Description  Mark an active incident as resolved, clear its monitor active incident path, and record a manual incident event
// @Tags         incidents
// @Accept       json
// @Produce      json
// @ID           resolveIncident
// @Param        id   path      string  true  "Incident ID"
// @Success      200  {object}  utils.APIResponse{data=object{incident=IncidentResponse}}
// @Failure      404  {object}  utils.APIResponse
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/incidents/{id}/resolve [post]
func (s *Server) resolveIncident(c *gin.Context) {
	incident, err := service.NewIncidentService(s.db, s.logger, s.cfg).ResolveIncident(c.Param("id"))
	if err != nil {
		s.handleIncidentActionError(c, err, "Failed to resolve incident")
		return
	}
	s.writeIncidentActionResponse(c, "Incident resolved successfully", incident)
}

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
func (s *Server) coverIncident(c *gin.Context) {
	var request incidentCoverageRequest
	if c.Request.Body != nil && c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&request); err != nil {
			utils.BadRequest(c, "Invalid incident coverage payload")
			return
		}
	}
	incident, err := service.NewIncidentService(s.db, s.logger, s.cfg).CoverIncident(c.Param("id"), request.CoveredUntil, request.Note)
	if err != nil {
		s.handleIncidentActionError(c, err, "Failed to cover incident")
		return
	}
	s.writeIncidentActionResponse(c, "Incident covered successfully", incident)
}

// reopenIncident reopens a resolved or covered incident.
// @Summary      Reopen incident
// @Description  Reopen a covered or resolved incident, restore its monitor active incident path, and record a manual incident event
// @Tags         incidents
// @Accept       json
// @Produce      json
// @ID           reopenIncident
// @Param        id   path      string  true  "Incident ID"
// @Success      200  {object}  utils.APIResponse{data=object{incident=IncidentResponse}}
// @Failure      404  {object}  utils.APIResponse
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/incidents/{id}/reopen [post]
func (s *Server) reopenIncident(c *gin.Context) {
	incident, err := service.NewIncidentService(s.db, s.logger, s.cfg).ReopenIncident(c.Param("id"))
	if err != nil {
		s.handleIncidentActionError(c, err, "Failed to reopen incident")
		return
	}
	s.writeIncidentActionResponse(c, "Incident reopened successfully", incident)
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
	utils.SuccessResponse(c, http.StatusOK, message, gin.H{
		"incident": incidentResponse(incident, agent, monitor),
	})
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
	err := s.db.
		Preload("Attempts", func(tx *gorm.DB) *gorm.DB {
			return tx.Order("attempt_number ASC")
		}).
		Where("incident_id = ?", incidentID).
		Order("created_at ASC").
		Find(&deliveries).Error
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

func incidentTimeline(events []db.IncidentEvent, deliveries []db.AlertDelivery, reports []db.MonitorReport) []IncidentTimelineItemResponse {
	timeline := make([]IncidentTimelineItemResponse, 0, len(events)+len(deliveries))
	evidenceByReportID := incidentTimelineEvidenceByReportID(reports)
	for _, event := range events {
		timeline = append(timeline, IncidentTimelineItemResponse{
			ID:              event.ID,
			Type:            event.Type,
			Source:          "incident_event",
			Message:         event.Message,
			Evidence:        evidenceByReportID[event.MonitorReportID],
			MonitorReportID: event.MonitorReportID,
			CreatedAt:       event.CreatedAt,
		})
	}
	for _, delivery := range deliveries {
		message := delivery.Channel + " notification " + delivery.Status
		if delivery.Error != "" {
			message += ": " + safeAlertDeliveryError(delivery.Error)
		}
		timeline = append(timeline, IncidentTimelineItemResponse{
			ID:              delivery.ID,
			Type:            "alert_delivery",
			Source:          "alert_delivery",
			Message:         message,
			AlertDeliveryID: delivery.ID,
			Channel:         delivery.Channel,
			Status:          delivery.Status,
			CreatedAt:       delivery.CreatedAt,
		})
	}

	sort.SliceStable(timeline, func(i, j int) bool {
		return timeline[i].CreatedAt.Before(timeline[j].CreatedAt)
	})
	return timeline
}

func incidentTimelineEvidenceByReportID(reports []db.MonitorReport) map[string]string {
	evidenceByReportID := make(map[string]string, len(reports))
	for _, report := range reports {
		evidence := monitorReportEvidence(report)
		if evidence != "" {
			evidenceByReportID[report.ID] = evidence
		}
	}
	return evidenceByReportID
}

func monitorReportEvidence(report db.MonitorReport) string {
	var fields map[string]interface{}
	if err := json.Unmarshal([]byte(safeMonitorReportPayload(report.Payload)), &fields); err != nil {
		return ""
	}
	for _, key := range []string{"payload", "failure_stage", "failure_reason", "message", "error", "summary", "status", "status_code"} {
		if evidence := monitorReportEvidenceValue(fields[key]); evidence != "" {
			return evidence
		}
	}
	return ""
}

func monitorReportEvidenceValue(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(typed)
	case map[string]interface{}, []interface{}:
		body, err := json.Marshal(typed)
		if err != nil {
			return ""
		}
		return string(body)
	default:
		return ""
	}
}

func queryInt(c *gin.Context, key string, fallback int) int {
	value, err := strconv.Atoi(c.DefaultQuery(key, strconv.Itoa(fallback)))
	if err != nil || value < 0 {
		return fallback
	}
	return value
}

func queryBool(c *gin.Context, key string, fallback bool) bool {
	value := strings.TrimSpace(c.Query(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func queryStatuses(value string) []string {
	statuses := []string{}
	for _, status := range strings.Split(value, ",") {
		status = strings.TrimSpace(status)
		if status != "" {
			statuses = append(statuses, status)
		}
	}
	if len(statuses) == 0 {
		return []string{"open", "acknowledged", "covered"}
	}
	return statuses
}
