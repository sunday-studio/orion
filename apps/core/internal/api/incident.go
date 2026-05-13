package api

import (
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// listIncidents retrieves persisted incidents.
// @Summary      List incidents
// @Description  Get a paginated list of persisted incidents. Defaults to active incidents.
// @Tags         incidents
// @Accept       json
// @Produce      json
// @ID           getIncidents
// @Param        status  query     string  false  "Comma-separated incident statuses" default(open,acknowledged)
// @Param        limit   query     int     false  "Maximum number of incidents to return" default(50)
// @Param        offset  query     int     false  "Number of incidents to skip" default(0)
// @Success      200     {object}  utils.APIResponse{data=object{incidents=[]IncidentResponse,count=int64,limit=int,offset=int,status=[]string}}
// @Failure      500     {object}  utils.APIResponse
// @Router       /v1/incidents [get]
func (s *Server) listIncidents(c *gin.Context) {
	limit := queryInt(c, "limit", 50)
	offset := queryInt(c, "offset", 0)
	statuses := queryStatuses(c.DefaultQuery("status", "open,acknowledged"))

	query := s.db.Model(&db.Incident{}).Where("status IN ?", statuses)

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
		"incidents": responses,
		"count":     count,
		"limit":     limit,
		"offset":    offset,
		"status":    statuses,
	})
}

func queryInt(c *gin.Context, key string, fallback int) int {
	value, err := strconv.Atoi(c.DefaultQuery(key, strconv.Itoa(fallback)))
	if err != nil || value < 0 {
		return fallback
	}
	return value
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
		return []string{"open", "acknowledged"}
	}
	return statuses
}
