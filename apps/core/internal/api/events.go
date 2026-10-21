package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"orion/core/internal/utils"
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
// @Description  Operational event log for Core, nodes, monitors, incidents, alerts, and lifecycle actions, newest first. Page with the keyset `cursor` (preferred: constant cost per page); `offset` is kept for compatibility, capped at 5000 rows, and makes `count` a lower bound.
// @Tags         events
// @Accept       json
// @Produce      json
// @ID           getOrionEvents
// @Param        limit   query     int  false  "Maximum number of events to return (max 200)" default(50)
// @Param        cursor  query     string  false  "Opaque cursor from a previous response's next_cursor"
// @Param        offset  query     int  false  "Deprecated: number of events to skip"  default(0)
// @Param        source  query     string  false  "Filter by event source"
// @Param        type    query     string  false  "Filter by event type"
// @Param        q       query     string  false  "Search event type, source, message, and related IDs"
// @Success      200     {object}  utils.APIResponse{data=object{events=[]OrionEventResponse,count=int,limit=int,offset=int,next_cursor=string,has_more=bool,pagination=utils.PaginationMeta}}
// @Failure      400     {object}  utils.APIResponse
// @Failure      500     {object}  utils.APIResponse
// @Router       /v1/events [get]
func (s *Server) listOrionEvents(c *gin.Context) {
	limit := utils.QueryInt(c, "limit", 50)
	if limit <= 0 {
		limit = 50
	}
	if limit > maxOrionEventsLimit {
		limit = maxOrionEventsLimit
	}
	offset := utils.QueryInt(c, "offset", 0)
	if offset < 0 {
		offset = 0
	}
	if offset+limit > maxOrionEventsWindow {
		utils.BadRequest(c, "offset too large; page with cursor instead")
		return
	}
	var cursor *orionEventCursor
	if raw := strings.TrimSpace(c.Query("cursor")); raw != "" {
		parsed, err := decodeOrionEventCursor(raw)
		if err != nil {
			utils.BadRequest(c, "Invalid cursor")
			return
		}
		cursor = &parsed
	}
	filters := orionEventFilters{
		Source: strings.TrimSpace(c.Query("source")),
		Type:   strings.TrimSpace(c.Query("type")),
		Search: strings.TrimSpace(c.Query("q")),
	}

	events, frontier, err := s.orionEventsPage(cursor, offset+limit+1, filters)
	if err != nil {
		s.logger.Error("Failed to list Orion events", "error", err)
		utils.InternalError(c, "Failed to list Orion events", err)
		return
	}
	hasMore := len(events) > offset+limit
	start := min(offset, len(events))
	end := min(offset+limit, len(events))
	page := events[start:end]
	nextCursor := ""
	switch {
	case hasMore && len(page) > 0:
		nextCursor = encodeOrionEventCursor(page[len(page)-1])
	case frontier != nil:
		// A source was cut off by the scan cap: resume from where scanning stopped, even if
		// this page is short or empty, so no older match is skipped.
		hasMore = true
		nextCursor = encodeOrionEventPosition(*frontier)
	}
	total := int64(offset + len(page))
	if hasMore {
		total++
	}
	utils.SuccessResponse(c, http.StatusOK, "Orion events retrieved successfully", gin.H{
		"events":      page,
		"count":       len(page),
		"limit":       limit,
		"offset":      offset,
		"next_cursor": nextCursor,
		"has_more":    hasMore,
		"pagination":  utils.NewPaginationMeta(total, limit, offset, len(page)),
	})
}

func auditEventMessage(event db.AuditEvent) string {
	switch event.Action {
	case service.NodeTokenAuditActionRotated:
		return "Node token rotated"
	case service.NodeTokenAuditActionRevoked:
		return "Node token revoked"
	case service.NodeTokenAuditActionReissued:
		return "Node token reissued"
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
		nodeReports := auditEventMetadataInt(metadata, "node_reports_archived")
		monitorReports := auditEventMetadataInt(metadata, "monitor_reports_archived")
		return fmt.Sprintf("Manual data lifecycle archive finished with %s status (%d node reports, %d monitor reports)", status, nodeReports, monitorReports)
	default:
		return strings.ReplaceAll(event.Action, "_", " ")
	}
}

func auditEventNodeID(event db.AuditEvent) string {
	if event.AffectedObjectType == "node" {
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
		event.NodeID,
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
