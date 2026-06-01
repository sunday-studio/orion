package api

import (
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"sort"
	"strings"
	"time"
)

type incidentCoverageRequest struct {
	CoveredUntil *time.Time `json:"covered_until"`
	Note         string     `json:"note"`
}
type incidentLifecycleActionRequest struct {
	Note string `json:"note"`
}
type incidentListFilters struct {
	statuses []string// listIncidents retrieves persisted incidents.
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

	agentID            string
	monitorID          string
	resolutionKind     string
	actor              string
	covered            *bool
	notificationStatus string
	needsReview        bool
}

func (s *Server) listIncidents(c *gin.Context) {
	limit := queryInt(c, "limit", 50)
	offset := queryInt(c, "offset", 0)
	filters := incidentListFilters{statuses: queryStatuses(c.DefaultQuery("status", "open,acknowledged,covered")), agentID: strings.TrimSpace(c.Query("agent_id")), monitorID: strings.TrimSpace(c.Query("monitor_id")), resolutionKind: strings.TrimSpace(c.Query("resolution_kind")), actor: strings.ToLower(strings.TrimSpace(c.Query("actor"))), covered: queryOptionalBool(c, "covered"), notificationStatus: strings.TrimSpace(c.Query("notification_status")), needsReview: queryBool(c, "needs_review", false)}
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
	utils.SuccessResponse(c, 200, "Incidents retrieved successfully", gin.H{"incidents": responses, "count": count, "limit": limit, "offset": offset, "pagination": utils.NewPaginationMeta(count, limit, offset, len(responses)), "status": filters.statuses, "insights": insights})
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
		query = query.Where("resolution_kind = ? OR EXISTS (SELECT 1 FROM incident_events WHERE incident_events.incident_id = incidents.id AND incident_events.type IN ?)", "manual", manualIncidentEventTypes())
	case "system":
		query = query.Where("resolution_kind <> ? AND NOT EXISTS (SELECT 1 FROM incident_events WHERE incident_events.incident_id = incidents.id AND incident_events.type IN ?)", "manual", manualIncidentEventTypes())
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
	return IncidentInsightsResponse{RecurringFailures: incidentRecurringFailures(incidents, s.monitorNamesByID(monitorIDs)), LifecycleTiming: s.incidentLifecycleTiming(incidents, incidentIDs), NotificationReliability: s.incidentNotificationReliability(incidentIDs)}, nil
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
		recurring = append(recurring, IncidentRecurringFailureResponse{MonitorID: aggregate.monitorID, MonitorName: monitorName, IncidentCount: aggregate.incidentCount, LastIncidentAt: aggregate.lastIncidentAt})
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
	if err := s.db.Where("incident_id IN ? AND type = ?", incidentIDs, "incident_acknowledged").Order("created_at ASC").Find(&events).Error; err != nil {
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
	timing := IncidentLifecycleTimingResponse{AcknowledgedCount: acknowledgedCount, ResolvedCount: resolvedCount}
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
	utils.SuccessResponse(c, http.StatusOK, "Incident retrieved successfully", gin.H{"incident": incidentResponse(incident, agent, monitor), "evidence": evidence, "next_actions": incidentNextActions(incident, monitor, deliveries, reports, relatedIncidents), "related_incidents": relatedIncidents, "timeline": incidentTimeline(events, deliveries, reports), "events": incidentEventResponses(events), "alert_deliveries": alertDeliveryResponses(deliveries), "monitor_reports": monitorReportResponses(reports)})
}

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
	utils.SuccessResponse(c, http.StatusOK, "Incident timeline retrieved successfully", gin.H{"timeline": timeline, "count": len(timeline)})
}
