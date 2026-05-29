package api

import (
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"orion/core/internal/utils"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type ServiceLogBatchResponse struct {
	Received int `json:"received"`
	Stored   int `json:"stored"`
}

// receiveAgentLogBatch receives structured service logs from an agent.
// @Summary      Receive agent service logs
// @Description  Receive and deduplicate a bounded batch of structured Orion Agent service logs
// @Tags         service-logs
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           receiveAgentLogBatch
// @Param        agent_id  path      string                         true  "Agent ID"
// @Param        data      body      service.ServiceLogBatchPayload  true  "Service log batch"
// @Success      200       {object}  utils.APIResponse{data=api.ServiceLogBatchResponse}
// @Failure      400       {object}  utils.APIResponse
// @Failure      401       {object}  utils.APIResponse
// @Failure      500       {object}  utils.APIResponse
// @Router       /v1/agents/{agent_id}/logs/batch [post]
func (s *Server) receiveAgentLogBatch(c *gin.Context) {
	agentID := c.Param("agent_id")
	if agentID == "" {
		utils.BadRequest(c, "Agent ID is required")
		return
	}

	var payload service.ServiceLogBatchPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		s.logger.Error("Invalid service log batch", "error", err, "agent_id", agentID)
		utils.BadRequest(c, "Invalid request payload")
		return
	}

	result, err := s.serviceLogService.StoreAgentLogBatch(agentID, payload, time.Now().UTC())
	if err != nil {
		if strings.Contains(err.Error(), "max is") || strings.Contains(err.Error(), "timestamp") {
			utils.BadRequest(c, err.Error())
			return
		}
		s.logger.Error("Failed to store service log batch", "error", err, "agent_id", agentID)
		utils.InternalError(c, "Failed to store service log batch", err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Service logs received successfully", ServiceLogBatchResponse{
		Received: result.Received,
		Stored:   result.Stored,
	})
}

// listServiceLogs retrieves service logs across agents.
// @Summary      List service logs
// @Description  Get paginated structured service logs shipped by Orion services
// @Tags         service-logs
// @Accept       json
// @Produce      json
// @ID           getServiceLogs
// @Param        limit      query     int     false  "Maximum number of logs to return" default(50)
// @Param        offset     query     int     false  "Number of logs to skip" default(0)
// @Param        agent_id   query     string  false  "Filter by agent ID"
// @Param        monitor_id query     string  false  "Filter by monitor ID"
// @Param        source     query     string  false  "Filter by service log source"
// @Param        level      query     string  false  "Filter by level"
// @Param        component  query     string  false  "Filter by component"
// @Param        q          query     string  false  "Search message, component, monitor, and related IDs"
// @Success      200        {object}  utils.APIResponse{data=object{logs=[]ServiceLogEntryResponse,count=int64,limit=int,offset=int,pagination=utils.PaginationMeta}}
// @Failure      500        {object}  utils.APIResponse
// @Router       /v1/logs/service [get]
func (s *Server) listServiceLogs(c *gin.Context) {
	s.listServiceLogsWithFilters(c, service.ServiceLogListFilters{
		AgentID:   strings.TrimSpace(c.Query("agent_id")),
		MonitorID: strings.TrimSpace(c.Query("monitor_id")),
		Source:    strings.TrimSpace(c.Query("source")),
		Level:     strings.TrimSpace(c.Query("level")),
		Component: strings.TrimSpace(c.Query("component")),
		Search:    strings.TrimSpace(c.Query("q")),
		Limit:     queryInt(c, "limit", 50),
		Offset:    queryInt(c, "offset", 0),
	})
}

// listAgentServiceLogs retrieves service logs for one agent.
// @Summary      List agent service logs
// @Description  Get paginated structured service logs for a specific Orion Agent
// @Tags         service-logs
// @Accept       json
// @Produce      json
// @ID           getAgentServiceLogs
// @Param        id         path      string  true   "Agent ID"
// @Param        limit      query     int     false  "Maximum number of logs to return" default(50)
// @Param        offset     query     int     false  "Number of logs to skip" default(0)
// @Param        level      query     string  false  "Filter by level"
// @Param        component  query     string  false  "Filter by component"
// @Param        q          query     string  false  "Search message, component, monitor, and related IDs"
// @Success      200        {object}  utils.APIResponse{data=object{logs=[]ServiceLogEntryResponse,count=int64,limit=int,offset=int,pagination=utils.PaginationMeta}}
// @Failure      400        {object}  utils.APIResponse
// @Failure      404        {object}  utils.APIResponse
// @Failure      500        {object}  utils.APIResponse
// @Router       /v1/agents/{id}/service-logs [get]
func (s *Server) listAgentServiceLogs(c *gin.Context) {
	agentID := c.Param("id")
	if agentID == "" {
		utils.BadRequest(c, "Agent ID is required")
		return
	}
	if _, err := s.agentService.GetAgent(agentID); err != nil {
		utils.NotFound(c, "Agent not found")
		return
	}

	s.listServiceLogsWithFilters(c, service.ServiceLogListFilters{
		AgentID:   agentID,
		Level:     strings.TrimSpace(c.Query("level")),
		Component: strings.TrimSpace(c.Query("component")),
		Search:    strings.TrimSpace(c.Query("q")),
		Limit:     queryInt(c, "limit", 50),
		Offset:    queryInt(c, "offset", 0),
	})
}

func (s *Server) listServiceLogsWithFilters(c *gin.Context, filters service.ServiceLogListFilters) {
	limit := filters.Limit
	if limit <= 0 {
		limit = 50
	}
	filters.Limit = limit

	result, err := s.serviceLogService.ListServiceLogs(filters)
	if err != nil {
		s.logger.Error("Failed to list service logs", "error", err)
		utils.InternalError(c, "Failed to list service logs", err)
		return
	}

	agentsByID := s.serviceLogAgentsByID(result.Entries)
	responses := serviceLogEntryResponses(result.Entries, agentsByID)
	utils.SuccessResponse(c, http.StatusOK, "Service logs retrieved successfully", gin.H{
		"logs":       responses,
		"count":      result.Count,
		"limit":      filters.Limit,
		"offset":     filters.Offset,
		"pagination": utils.NewPaginationMeta(result.Count, filters.Limit, filters.Offset, len(responses)),
	})
}

func (s *Server) serviceLogAgentsByID(entries []db.ServiceLogEntry) map[string]db.Agent {
	ids := make([]string, 0, len(entries))
	seen := map[string]bool{}
	for _, entry := range entries {
		if entry.AgentID == "" || seen[entry.AgentID] {
			continue
		}
		seen[entry.AgentID] = true
		ids = append(ids, entry.AgentID)
	}
	agentsByID := map[string]db.Agent{}
	if len(ids) == 0 {
		return agentsByID
	}

	var agents []db.Agent
	if err := s.db.Where("id IN ?", ids).Find(&agents).Error; err != nil {
		s.logger.Error("Failed to load service log agents", "error", err)
		return agentsByID
	}
	for _, agent := range agents {
		agentsByID[agent.ID] = agent
	}
	return agentsByID
}
