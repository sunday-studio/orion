package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"orion/core/internal/utils"
	"slices"
	"strings"

	"github.com/gin-gonic/gin"
)

type orionEventFilters struct {
	Source string
	Type   string
	Search string
}

// listOrionEvents retrieves Orion operational events derived from stored records.
// @Summary      List Orion events
// @Description  Get a paginated operational event log for Core, agents, monitors, incidents, alerts, and lifecycle actions
// @Tags         events
// @Accept       json
// @Produce      json
// @ID           getOrionEvents
// @Param        limit   query     int  false  "Maximum number of events to return" default(50)
// @Param        offset  query     int  false  "Number of events to skip" default(0)
// @Param        source  query     string  false  "Filter by event source"
// @Param        type    query     string  false  "Filter by event type"
// @Param        q       query     string  false  "Search event type, source, message, and related IDs"
// @Success      200     {object}  utils.APIResponse{data=object{events=[]OrionEventResponse,count=int,limit=int,offset=int,pagination=utils.PaginationMeta}}
// @Failure      500     {object}  utils.APIResponse
// @Router       /v1/events [get]
func (s *Server) listOrionEvents(c *gin.Context) {
	limit := queryInt(c, "limit", 50)
	offset := queryInt(c, "offset", 0)
	filters := orionEventFilters{
		Source: strings.TrimSpace(c.Query("source")),
		Type:   strings.TrimSpace(c.Query("type")),
		Search: strings.TrimSpace(c.Query("q")),
	}

	events, err := s.orionEvents(0, filters)
	if err != nil {
		s.logger.Error("Failed to list Orion events", "error", err)
		utils.InternalError(c, "Failed to list Orion events", err)
		return
	}
	count := len(events)

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

func (s *Server) orionEvents(fetchLimit int, filters orionEventFilters) ([]OrionEventResponse, error) {
	if fetchLimit <= 0 {
		fetchLimit = -1
	}

	events := make([]OrionEventResponse, 0)
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

	var auditEvents []db.AuditEvent
	if err := s.db.Order("created_at DESC").Limit(fetchLimit).Find(&auditEvents).Error; err != nil {
		return nil, err
	}
	for _, event := range auditEvents {
		source := "audit"
		if event.AffectedObjectType == "data_lifecycle" {
			source = "data_lifecycle"
		} else if event.AffectedObjectType != "" {
			source = event.AffectedObjectType + "_lifecycle"
		}
		events = append(events, OrionEventResponse{
			ID:        event.ID,
			Type:      event.Action,
			Source:    source,
			Message:   auditEventMessage(event),
			AgentID:   auditEventAgentID(event),
			CreatedAt: event.CreatedAt,
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

	slices.SortStableFunc(events, func(a, b OrionEventResponse) int {
		return b.CreatedAt.Compare(a.CreatedAt)
	})
	events = filterOrionEvents(events, filters)
	if fetchLimit > 0 && len(events) > fetchLimit {
		return events[:fetchLimit], nil
	}
	return events, nil
}

func auditEventMessage(event db.AuditEvent) string {
	switch event.Action {
	case service.AgentTokenAuditActionRotated:
		return "Agent token rotated"
	case service.AgentTokenAuditActionRevoked:
		return "Agent token revoked"
	case service.AgentTokenAuditActionReissued:
		return "Agent token reissued"
	case service.DataLifecycleAuditActionSettingsUpdated:
		return "Data lifecycle settings updated"
	case service.DataLifecycleAuditActionRollupRun:
		metadata := auditEventMetadata(event)
		status := auditEventMetadataString(metadata, "result_status", "unknown")
		reportCount := auditEventMetadataInt(metadata, "report_count")
		monitorDays := auditEventMetadataInt(metadata, "monitor_days")
		return fmt.Sprintf("Manual data lifecycle rollup finished with %s status (%d reports across %d monitor days)", status, reportCount, monitorDays)
	case service.DataLifecycleAuditActionArchiveRun:
		metadata := auditEventMetadata(event)
		status := auditEventMetadataString(metadata, "result_status", "unknown")
		agentReports := auditEventMetadataInt(metadata, "agent_reports_archived")
		monitorReports := auditEventMetadataInt(metadata, "monitor_reports_archived")
		return fmt.Sprintf("Manual data lifecycle archive finished with %s status (%d agent reports, %d monitor reports)", status, agentReports, monitorReports)
	default:
		return strings.ReplaceAll(event.Action, "_", " ")
	}
}

func auditEventAgentID(event db.AuditEvent) string {
	if event.AffectedObjectType == "agent" {
		return event.AffectedObjectID
	}
	return ""
}

func filterOrionEvents(events []OrionEventResponse, filters orionEventFilters) []OrionEventResponse {
	source := strings.ToLower(filters.Source)
	eventType := strings.ToLower(filters.Type)
	search := strings.ToLower(filters.Search)
	if source == "" && eventType == "" && search == "" {
		return events
	}

	filtered := make([]OrionEventResponse, 0, len(events))
	for _, event := range events {
		if source != "" && strings.ToLower(event.Source) != source {
			continue
		}
		if eventType != "" && strings.ToLower(event.Type) != eventType {
			continue
		}
		if search != "" && !orionEventMatchesSearch(event, search) {
			continue
		}
		filtered = append(filtered, event)
	}
	return filtered
}

func orionEventMatchesSearch(event OrionEventResponse, search string) bool {
	values := []string{
		event.ID,
		event.Type,
		event.Source,
		event.Message,
		event.AgentID,
		event.MonitorID,
		event.IncidentID,
	}
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), search) {
			return true
		}
	}
	return false
}

func auditEventMetadata(event db.AuditEvent) map[string]interface{} {
	metadata := map[string]interface{}{}
	if strings.TrimSpace(event.MetadataJSON) == "" {
		return metadata
	}
	if err := json.Unmarshal([]byte(event.MetadataJSON), &metadata); err != nil {
		return map[string]interface{}{}
	}
	return metadata
}

func auditEventMetadataString(metadata map[string]interface{}, key string, fallback string) string {
	if value, ok := metadata[key].(string); ok && strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return fallback
}

func auditEventMetadataInt(metadata map[string]interface{}, key string) int {
	switch value := metadata[key].(type) {
	case float64:
		return int(value)
	case int:
		return value
	default:
		return 0
	}
}
