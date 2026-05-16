package api

import (
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"sort"

	"github.com/gin-gonic/gin"
)

// listOrionEvents retrieves Orion operational events derived from stored records.
// @Summary      List Orion events
// @Description  Get a paginated operational event log for Core, agents, monitors, incidents, alerts, and lifecycle actions
// @Tags         events
// @Accept       json
// @Produce      json
// @ID           getOrionEvents
// @Param        limit   query     int  false  "Maximum number of events to return" default(50)
// @Param        offset  query     int  false  "Number of events to skip" default(0)
// @Success      200     {object}  utils.APIResponse{data=object{events=[]OrionEventResponse,count=int,limit=int,offset=int,pagination=utils.PaginationMeta}}
// @Failure      500     {object}  utils.APIResponse
// @Router       /v1/events [get]
func (s *Server) listOrionEvents(c *gin.Context) {
	limit := queryInt(c, "limit", 50)
	offset := queryInt(c, "offset", 0)

	events, err := s.orionEvents(limit + offset + 1)
	if err != nil {
		s.logger.Error("Failed to list Orion events", "error", err)
		utils.InternalError(c, "Failed to list Orion events", err)
		return
	}
	count, err := s.orionEventCount()
	if err != nil {
		s.logger.Error("Failed to count Orion events", "error", err)
		utils.InternalError(c, "Failed to list Orion events", err)
		return
	}

	start := offset
	if start > len(events) {
		start = len(events)
	}
	end := start + limit
	if end > len(events) {
		end = len(events)
	}

	responses := events[start:end]
	utils.SuccessResponse(c, http.StatusOK, "Orion events retrieved successfully", gin.H{
		"events":     responses,
		"count":      count,
		"limit":      limit,
		"offset":     offset,
		"pagination": utils.NewPaginationMeta(int64(count), limit, offset, len(responses)),
	})
}

func (s *Server) orionEventCount() (int, error) {
	total := int64(0)
	for _, model := range []interface{}{
		&db.Agent{},
		&db.Monitor{},
		&db.AgentReport{},
		&db.MonitorReport{},
		&db.IncidentEvent{},
		&db.AlertDelivery{},
	} {
		var count int64
		if err := s.db.Model(model).Count(&count).Error; err != nil {
			return 0, err
		}
		total += count
	}

	var settings db.DataLifecycleSettings
	result := s.db.First(&settings, 1)
	if result.Error == nil {
		if settings.LastRollupRunAt != nil {
			total++
		}
		if settings.LastArchiveRunAt != nil {
			total++
		}
	}

	return int(total), nil
}

func (s *Server) orionEvents(fetchLimit int) ([]OrionEventResponse, error) {
	if fetchLimit <= 0 {
		fetchLimit = 50
	}

	events := make([]OrionEventResponse, 0, fetchLimit*4)
	var agents []db.Agent
	if err := s.db.Order("created_at DESC").Limit(fetchLimit).Find(&agents).Error; err != nil {
		return nil, err
	}
	for _, agent := range agents {
		events = append(events, OrionEventResponse{
			ID:        agent.ID + ":registered",
			Type:      "agent_registered",
			Source:    "agent",
			Message:   "Agent " + agent.Name + " registered",
			AgentID:   agent.ID,
			CreatedAt: agent.CreatedAt,
		})
	}

	var monitors []db.Monitor
	if err := s.db.Order("created_at DESC").Limit(fetchLimit).Find(&monitors).Error; err != nil {
		return nil, err
	}
	for _, monitor := range monitors {
		events = append(events, OrionEventResponse{
			ID:        monitor.ID + ":registered",
			Type:      "monitor_registered",
			Source:    "monitor",
			Message:   "Monitor " + monitor.Name + " registered",
			AgentID:   monitor.AgentID,
			MonitorID: monitor.ID,
			CreatedAt: monitor.CreatedAt,
		})
	}

	var agentReports []db.AgentReport
	if err := s.db.Order("created_at DESC").Limit(fetchLimit).Find(&agentReports).Error; err != nil {
		return nil, err
	}
	for _, report := range agentReports {
		events = append(events, OrionEventResponse{
			ID:        report.ID,
			Type:      "agent_report_received",
			Source:    "agent_report",
			Message:   "Agent report received",
			AgentID:   report.AgentID,
			CreatedAt: report.CreatedAt,
		})
	}

	var monitorReports []db.MonitorReport
	if err := s.db.Order("created_at DESC").Limit(fetchLimit).Find(&monitorReports).Error; err != nil {
		return nil, err
	}
	for _, report := range monitorReports {
		events = append(events, OrionEventResponse{
			ID:        report.ID,
			Type:      "monitor_report_received",
			Source:    "monitor_report",
			Message:   "Monitor report received with " + report.Health + " health",
			MonitorID: report.MonitorID,
			CreatedAt: report.CreatedAt,
		})
	}

	var incidentEvents []db.IncidentEvent
	if err := s.db.Order("created_at DESC").Limit(fetchLimit).Find(&incidentEvents).Error; err != nil {
		return nil, err
	}
	for _, event := range incidentEvents {
		events = append(events, OrionEventResponse{
			ID:         event.ID,
			Type:       event.Type,
			Source:     "incident_event",
			Message:    event.Message,
			IncidentID: event.IncidentID,
			CreatedAt:  event.CreatedAt,
		})
	}

	var deliveries []db.AlertDelivery
	if err := s.db.Order("created_at DESC").Limit(fetchLimit).Find(&deliveries).Error; err != nil {
		return nil, err
	}
	for _, delivery := range deliveries {
		events = append(events, OrionEventResponse{
			ID:         delivery.ID,
			Type:       "alert_" + delivery.Status,
			Source:     "alert_delivery",
			Message:    delivery.Channel + " notification " + delivery.Status,
			IncidentID: delivery.IncidentID,
			CreatedAt:  delivery.CreatedAt,
		})
	}

	var settings db.DataLifecycleSettings
	result := s.db.First(&settings, 1)
	if result.Error == nil {
		if settings.LastRollupRunAt != nil {
			events = append(events, OrionEventResponse{
				ID:        "data-lifecycle:rollup:" + settings.LastRollupRunAt.Format("20060102150405"),
				Type:      "retention_rollup_ran",
				Source:    "data_lifecycle",
				Message:   "Data lifecycle rollup ran",
				CreatedAt: *settings.LastRollupRunAt,
			})
		}
		if settings.LastArchiveRunAt != nil {
			message := "Data lifecycle archive ran"
			if settings.LastArchiveStatus != "" {
				message += " with " + settings.LastArchiveStatus + " status"
			}
			events = append(events, OrionEventResponse{
				ID:        "data-lifecycle:archive:" + settings.LastArchiveRunAt.Format("20060102150405"),
				Type:      "retention_archive_ran",
				Source:    "data_lifecycle",
				Message:   message,
				CreatedAt: *settings.LastArchiveRunAt,
			})
		}
	}

	sort.SliceStable(events, func(i, j int) bool {
		return events[i].CreatedAt.After(events[j].CreatedAt)
	})
	if len(events) > fetchLimit {
		return events[:fetchLimit], nil
	}
	return events, nil
}
