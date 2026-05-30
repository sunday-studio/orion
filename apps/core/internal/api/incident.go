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

type incidentListFilters struct {
	statuses           []string
	agentID            string
	monitorID          string
	resolutionKind     string
	actor              string
	covered            *bool
	notificationStatus string
	needsReview        bool
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
// @Param        resolution_kind  query  string  false  "Filter incidents by resolution kind"
// @Param        actor  query     string  false  "Filter incidents by lifecycle actor: manual or system"
// @Param        covered  query   bool    false  "Filter incidents by covered lifecycle state"
// @Param        notification_status  query  string  false  "Filter incidents by notification status"
// @Param        needs_review  query  bool  false  "Filter to incidents with failed notifications or high/critical/error severity"
// @Param        limit   query     int     false  "Maximum number of incidents to return" default(50)
// @Param        offset  query     int     false  "Number of incidents to skip" default(0)
// @Success      200     {object}  utils.APIResponse{data=object{incidents=[]IncidentResponse,count=int64,limit=int,offset=int,pagination=utils.PaginationMeta,status=[]string,insights=IncidentInsightsResponse}}
// @Failure      500     {object}  utils.APIResponse
// @Router       /v1/incidents [get]
func (s *Server) listIncidents(c *gin.Context) {
	limit := queryInt(c, "limit", 50)
	offset := queryInt(c, "offset", 0)
	filters := incidentListFilters{
		statuses:           queryStatuses(c.DefaultQuery("status", "open,acknowledged,covered")),
		agentID:            strings.TrimSpace(c.Query("agent_id")),
		monitorID:          strings.TrimSpace(c.Query("monitor_id")),
		resolutionKind:     strings.TrimSpace(c.Query("resolution_kind")),
		actor:              strings.ToLower(strings.TrimSpace(c.Query("actor"))),
		covered:            queryOptionalBool(c, "covered"),
		notificationStatus: strings.TrimSpace(c.Query("notification_status")),
		needsReview:        queryBool(c, "needs_review", false),
	}
	query := s.incidentListQuery(filters)

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

	insights, err := s.incidentInsights(filters)
	if err != nil {
		s.logger.Error("Failed to build incident insights", "error", err)
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
		"status":     filters.statuses,
		"insights":   insights,
	})
}

func (s *Server) incidentListQuery(filters incidentListFilters) *gorm.DB {
	query := s.db.Model(&db.Incident{}).Where("status IN ?", filters.statuses)
	if filters.agentID != "" {
		query = query.Where("agent_id = ?", filters.agentID)
	}
	if filters.monitorID != "" {
		query = query.Where("monitor_id = ?", filters.monitorID)
	}
	if filters.resolutionKind != "" {
		query = query.Where("resolution_kind = ?", filters.resolutionKind)
	}
	switch filters.actor {
	case "manual":
		query = query.Where(
			"resolution_kind = ? OR EXISTS (SELECT 1 FROM incident_events WHERE incident_events.incident_id = incidents.id AND incident_events.type IN ?)",
			"manual",
			manualIncidentEventTypes(),
		)
	case "system":
		query = query.Where(
			"resolution_kind <> ? AND NOT EXISTS (SELECT 1 FROM incident_events WHERE incident_events.incident_id = incidents.id AND incident_events.type IN ?)",
			"manual",
			manualIncidentEventTypes(),
		)
	}
	if filters.covered != nil {
		if *filters.covered {
			query = query.Where("status = ? OR covered_at IS NOT NULL", "covered")
		} else {
			query = query.Where("status <> ? AND covered_at IS NULL", "covered")
		}
	}
	if filters.notificationStatus != "" {
		query = query.Where("notification_status = ?", filters.notificationStatus)
	}
	if filters.needsReview {
		query = query.Where("notification_status = ? OR severity IN ?", "failed", []string{"high", "critical", "error"})
	}
	return query
}

func manualIncidentEventTypes() []string {
	return []string{"incident_acknowledged", "incident_covered", "incident_reopened"}
}

func (s *Server) incidentInsights(filters incidentListFilters) (IncidentInsightsResponse, error) {
	var incidents []db.Incident
	if err := s.incidentListQuery(filters).Find(&incidents).Error; err != nil {
		return IncidentInsightsResponse{}, err
	}
	if len(incidents) == 0 {
		return IncidentInsightsResponse{}, nil
	}

	incidentIDs := make([]string, 0, len(incidents))
	monitorIDs := make([]string, 0, len(incidents))
	monitorSeen := map[string]bool{}
	for _, incident := range incidents {
		incidentIDs = append(incidentIDs, incident.ID)
		if incident.MonitorID != "" && !monitorSeen[incident.MonitorID] {
			monitorSeen[incident.MonitorID] = true
			monitorIDs = append(monitorIDs, incident.MonitorID)
		}
	}

	return IncidentInsightsResponse{
		RecurringFailures:       incidentRecurringFailures(incidents, s.monitorNamesByID(monitorIDs)),
		LifecycleTiming:         s.incidentLifecycleTiming(incidents, incidentIDs),
		NotificationReliability: s.incidentNotificationReliability(incidentIDs),
	}, nil
}

func (s *Server) monitorNamesByID(monitorIDs []string) map[string]string {
	if len(monitorIDs) == 0 {
		return map[string]string{}
	}
	var monitors []db.Monitor
	if err := s.db.Where("id IN ?", monitorIDs).Find(&monitors).Error; err != nil {
		return map[string]string{}
	}
	names := make(map[string]string, len(monitors))
	for _, monitor := range monitors {
		names[monitor.ID] = monitor.Name
	}
	return names
}

func incidentRecurringFailures(incidents []db.Incident, monitorNames map[string]string) []IncidentRecurringFailureResponse {
	type monitorAggregate struct {
		monitorID      string
		incidentCount  int64
		lastIncidentAt time.Time
	}
	aggregates := map[string]monitorAggregate{}
	for _, incident := range incidents {
		if incident.MonitorID == "" {
			continue
		}
		aggregate := aggregates[incident.MonitorID]
		aggregate.monitorID = incident.MonitorID
		aggregate.incidentCount++
		if aggregate.lastIncidentAt.IsZero() || incident.OpenedAt.After(aggregate.lastIncidentAt) {
			aggregate.lastIncidentAt = incident.OpenedAt
		}
		aggregates[incident.MonitorID] = aggregate
	}

	recurring := make([]IncidentRecurringFailureResponse, 0, len(aggregates))
	for _, aggregate := range aggregates {
		if aggregate.incidentCount < 2 {
			continue
		}
		monitorName := monitorNames[aggregate.monitorID]
		if monitorName == "" {
			monitorName = aggregate.monitorID
		}
		recurring = append(recurring, IncidentRecurringFailureResponse{
			MonitorID:      aggregate.monitorID,
			MonitorName:    monitorName,
			IncidentCount:  aggregate.incidentCount,
			LastIncidentAt: aggregate.lastIncidentAt,
		})
	}
	sort.SliceStable(recurring, func(i, j int) bool {
		if recurring[i].IncidentCount == recurring[j].IncidentCount {
			return recurring[i].LastIncidentAt.After(recurring[j].LastIncidentAt)
		}
		return recurring[i].IncidentCount > recurring[j].IncidentCount
	})
	if len(recurring) > 5 {
		recurring = recurring[:5]
	}
	return recurring
}

func (s *Server) incidentLifecycleTiming(incidents []db.Incident, incidentIDs []string) IncidentLifecycleTimingResponse {
	var events []db.IncidentEvent
	if err := s.db.
		Where("incident_id IN ? AND type = ?", incidentIDs, "incident_acknowledged").
		Order("created_at ASC").
		Find(&events).Error; err != nil {
		return IncidentLifecycleTimingResponse{}
	}
	firstAckByIncident := map[string]time.Time{}
	for _, event := range events {
		if _, found := firstAckByIncident[event.IncidentID]; !found {
			firstAckByIncident[event.IncidentID] = event.CreatedAt
		}
	}

	var acknowledgedCount int64
	var resolvedCount int64
	var acknowledgeSeconds int64
	var resolveSeconds int64
	for _, incident := range incidents {
		if acknowledgedAt, found := firstAckByIncident[incident.ID]; found {
			acknowledgedCount++
			acknowledgeSeconds += nonNegativeDurationSeconds(acknowledgedAt.Sub(incident.OpenedAt))
		}
		if incident.ResolvedAt != nil {
			resolvedCount++
			resolveSeconds += nonNegativeDurationSeconds(incident.ResolvedAt.Sub(incident.OpenedAt))
		}
	}

	timing := IncidentLifecycleTimingResponse{
		AcknowledgedCount: acknowledgedCount,
		ResolvedCount:     resolvedCount,
	}
	if acknowledgedCount > 0 {
		timing.MeanTimeToAcknowledgeSeconds = acknowledgeSeconds / acknowledgedCount
	}
	if resolvedCount > 0 {
		timing.MeanTimeToResolveSeconds = resolveSeconds / resolvedCount
	}
	return timing
}

func (s *Server) incidentNotificationReliability(incidentIDs []string) IncidentNotificationReliabilityStats {
	var deliveries []db.AlertDelivery
	if err := s.db.Where("incident_id IN ?", incidentIDs).Find(&deliveries).Error; err != nil {
		return IncidentNotificationReliabilityStats{}
	}

	stats := IncidentNotificationReliabilityStats{TotalDeliveries: int64(len(deliveries))}
	for _, delivery := range deliveries {
		switch delivery.Status {
		case "sent":
			stats.SentDeliveries++
		case "failed":
			stats.FailedDeliveries++
		case "suppressed":
			stats.SuppressedDeliveries++
		}
	}
	if stats.TotalDeliveries > 0 {
		stats.SuccessRatePercent = float64(stats.SentDeliveries) / float64(stats.TotalDeliveries) * 100
	}
	return stats
}

// getIncidentDetail retrieves one incident with linked operational data.
// @Summary      Get incident detail
// @Description  Get one incident with related timeline events, alert deliveries, and monitor reports
// @Tags         incidents
// @Accept       json
// @Produce      json
// @ID           getIncident
// @Param        id   path      string  true  "Incident ID"
// @Success      200  {object}  utils.APIResponse{data=object{incident=IncidentResponse,evidence=IncidentEvidenceResponse,next_actions=[]IncidentNextActionResponse,related_incidents=[]IncidentRelatedIncidentResponse,timeline=[]IncidentTimelineItemResponse,events=[]IncidentEventResponse,alert_deliveries=[]AlertDeliveryResponse,monitor_reports=[]MonitorReportResponse}}
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
	evidence, err := s.incidentEvidence(incident, events)
	if err != nil {
		s.logger.Error("Failed to load incident evidence", "incident_id", incident.ID, "error", err)
		utils.InternalError(c, "Failed to get incident detail", err)
		return
	}
	relatedIncidents, err := s.relatedIncidents(incident)
	if err != nil {
		s.logger.Error("Failed to load related incidents", "incident_id", incident.ID, "error", err)
		utils.InternalError(c, "Failed to get incident detail", err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Incident retrieved successfully", gin.H{
		"incident":          incidentResponse(incident, agent, monitor),
		"evidence":          evidence,
		"next_actions":      incidentNextActions(incident, monitor, deliveries, reports, relatedIncidents),
		"related_incidents": relatedIncidents,
		"timeline":          incidentTimeline(events, deliveries, reports),
		"events":            incidentEventResponses(events),
		"alert_deliveries":  alertDeliveryResponses(deliveries),
		"monitor_reports":   monitorReportResponses(reports),
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

func incidentNextActions(incident db.Incident, monitor db.Monitor, deliveries []db.AlertDelivery, reports []db.MonitorReport, related []IncidentRelatedIncidentResponse) []IncidentNextActionResponse {
	actions := make([]IncidentNextActionResponse, 0, 5)
	switch incident.Status {
	case "open":
		actions = append(actions, IncidentNextActionResponse{
			ID:          "acknowledge-incident",
			Label:       "Acknowledge",
			Description: "Mark ownership before recovery work starts.",
			ActionType:  "acknowledge_incident",
			Priority:    10,
			TargetKind:  "incident",
			TargetID:    incident.ID,
		})
		fallthrough
	case "acknowledged":
		actions = append(actions, IncidentNextActionResponse{
			ID:          "cover-incident",
			Label:       "Cover",
			Description: "Suppress noise while a known mitigation is in progress.",
			ActionType:  "cover_incident",
			Priority:    20,
			TargetKind:  "incident",
			TargetID:    incident.ID,
		})
	case "covered", "resolved":
		actions = append(actions, IncidentNextActionResponse{
			ID:          "reopen-incident",
			Label:       "Reopen",
			Description: "Resume active incident handling if recovery did not hold.",
			ActionType:  "reopen_incident",
			Priority:    20,
			TargetKind:  "incident",
			TargetID:    incident.ID,
		})
	}
	if incident.Status != "resolved" {
		actions = append(actions, IncidentNextActionResponse{
			ID:          "resolve-incident",
			Label:       "Resolve",
			Description: "Close the incident after the latest check confirms recovery.",
			ActionType:  "resolve_incident",
			Priority:    40,
			TargetKind:  "incident",
			TargetID:    incident.ID,
		})
	}

	if incident.MonitorID != "" && incidentNeedsMonitorTuning(incident, reports, related) {
		monitorName := monitor.Name
		if strings.TrimSpace(monitorName) == "" {
			monitorName = incident.MonitorID
		}
		actions = append(actions, IncidentNextActionResponse{
			ID:          "review-monitor-tuning",
			Label:       "Tune monitor",
			Description: "Review confirmation, recovery, and timeout settings for " + monitorName + ".",
			ActionType:  "review_monitor_tuning",
			Priority:    30,
			TargetKind:  "monitor",
			TargetID:    incident.MonitorID,
			TargetTab:   "config",
		})
	}

	if failedCount := failedAlertDeliveryCount(deliveries); failedCount > 0 {
		deliveryLabel := " failed delivery attempt needs alert destination review."
		if failedCount > 1 {
			deliveryLabel = " failed delivery attempts need alert destination review."
		}
		actions = append(actions, IncidentNextActionResponse{
			ID:           "review-failed-notifications",
			Label:        "Recover notifications",
			Description:  strconv.Itoa(failedCount) + deliveryLabel,
			ActionType:   "review_failed_notifications",
			Priority:     35,
			TargetKind:   "alert_deliveries",
			TargetID:     incident.ID,
			TargetTab:    "logs",
			FilterStatus: "failed",
		})
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
	if err := s.db.
		Where("monitor_id = ? AND id <> ?", incident.MonitorID, incident.ID).
		Order("opened_at DESC").
		Limit(5).
		Find(&incidents).Error; err != nil {
		return nil, err
	}

	responses := make([]IncidentRelatedIncidentResponse, 0, len(incidents))
	for _, related := range incidents {
		responses = append(responses, IncidentRelatedIncidentResponse{
			ID:                 related.ID,
			Status:             related.Status,
			Severity:           related.Severity,
			Title:              related.Title,
			ResolutionKind:     related.ResolutionKind,
			OpenedAt:           related.OpenedAt,
			ResolvedAt:         related.ResolvedAt,
			LastEventAt:        related.LastEventAt,
			LatestEvent:        related.LatestEvent,
			NotificationStatus: related.NotificationStatus,
		})
	}
	return responses, nil
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

func queryOptionalBool(c *gin.Context, key string) *bool {
	value := strings.TrimSpace(c.Query(key))
	if value == "" {
		return nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return nil
	}
	return &parsed
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

func nonNegativeDurationSeconds(duration time.Duration) int64 {
	if duration < 0 {
		return 0
	}
	return int64(duration.Seconds())
}
