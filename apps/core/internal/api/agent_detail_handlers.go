package api

import (
	"github.com/gin-gonic/gin"
	"orion/core/internal/service"
	"orion/core/internal/utils"
)

func (s *Server) getAgentDetail(c *gin.Context) {
	agentID := c.Param("id")
	if agentID == "" {
		utils.BadRequest(c, "Agent ID is required")
		return
	}
	agent, err := s.agentService.GetAgent(agentID)
	if err != nil {
		s.logger.Error("Failed to get agent", "error", err, "agent_id", agentID)
		utils.NotFound(c, "Agent not found")
		return
	}
	reports, err := s.reportService.GetAgentReportsById(agentID, 1, 0)
	if err != nil {
		s.logger.Error("Failed to get agent reports", "error", err, "agent_id", agentID)
	}
	var latestReport any
	if len(reports) > 0 {
		latestReport = agentReportResponse(reports[0])
	}
	utils.SuccessResponse(c, 200, "Agent retrieved successfully", gin.H{"agent": agentResponse(*agent), "latest_report": latestReport})
}

func (s *Server) getAgentHealth(c *gin.Context) {
	agentID := c.Param("id")
	if agentID == "" {
		utils.BadRequest(c, "Agent ID is required")
		return
	}
	if _, err := s.agentService.GetAgent(agentID); err != nil {
		utils.NotFound(c, "Agent not found")
		return
	}
	healthService := service.NewHealthService(s.db, s.logger)
	config := service.DefaultHealthConfig()
	snapshot, err := healthService.ComputeAgentHealthSnapshot(agentID, config)
	if err != nil {
		s.logger.Error("Failed to compute agent health", "error", err, "agent_id", agentID)
		utils.InternalError(c, "Failed to compute agent health", err)
		return
	}
	utils.SuccessResponse(c, 200, "Agent health retrieved successfully", AgentHealthResponse{AgentID: agentID, OverallHealth: snapshot.OverallHealth, AvailabilityHealth: snapshot.AgentHealth, MonitorHealth: snapshot.MonitorHealth, StatusReason: snapshot.Reason, UpCount: snapshot.UpCount, DownCount: snapshot.DownCount, DegradedCount: snapshot.DegradedCount, StaleCount: snapshot.StaleCount, UnknownCount: snapshot.UnknownCount, TotalCount: snapshot.TotalCount})
}

func (s *Server) getAgentReports(c *gin.Context) {
	agentID := c.Param("id")
	if agentID == "" {
		utils.BadRequest(c, "Agent ID is required")
		return
	}
	limit := queryInt(c, "limit", 50)
	offset := queryInt(c, "offset", 0)
	if _, err := s.agentService.GetAgent(agentID); err != nil {
		utils.NotFound(c, "Agent not found")
		return
	}
	reports, err := s.reportService.GetAgentReportsById(agentID, limit, offset)
	if err != nil {
		s.logger.Error("Failed to get agent reports", "error", err, "agent_id", agentID)
		utils.InternalError(c, "Failed to get agent reports", err)
		return
	}
	count, err := s.reportService.GetAgentReportCountById(agentID)
	if err != nil {
		s.logger.Error("Failed to get agent report count", "error", err, "agent_id", agentID)
		count = int64(len(reports))
	}
	responses := agentReportResponses(reports)
	utils.SuccessResponse(c, 200, "Agent reports retrieved successfully", gin.H{"reports": responses, "count": count, "limit": limit, "offset": offset, "pagination": utils.NewPaginationMeta(count, limit, offset, len(responses))})
}

func (s *Server) getAgentUptime(c *gin.Context) {
	agentID := c.Param("id")
	if agentID == "" {
		utils.BadRequest(c, "Agent ID is required")
		return
	}
	period := c.DefaultQuery("period", "90d")
	if _, err := s.agentService.GetAgent(agentID); err != nil {
		utils.NotFound(c, "Agent not found")
		return
	}
	result, err := s.reportService.GetAgentUptime(agentID, period)
	if err != nil {
		s.logger.Error("Failed to get agent uptime", "error", err, "agent_id", agentID)
		utils.InternalError(c, "Failed to get agent uptime", err)
		return
	}
	utils.SuccessResponse(c, 200, "Agent uptime retrieved successfully", gin.H{"daily_buckets": result.DailyBuckets, "uptime_percent": result.UptimePercent})
}

// getAgentDetail retrieves detailed information about a specific agent
// @Summary      Get agent details
// @Description  Get detailed information about a specific agent including latest report
// @Tags         agents
// @Accept       json
// @Produce      json
// @ID           getAgent
// @Param        id   path      string  true  "Agent ID"
// @Success      200  {object}  utils.APIResponse{data=object{agent=AgentResponse,latest_report=object}}
// @Failure      400  {object}  utils.APIResponse
// @Failure      404  {object}  utils.APIResponse
// @Router       /v1/agents/{id} [get]
// Get latest agent report for system metrics
// Don't fail if reports can't be retrieved
// getAgentHealth retrieves health status for a specific agent
// @Summary      Get agent health
// @Description  Get split agent availability and monitor rollup health for a specific agent
// @Tags         agents
// @Accept       json
// @Produce      json
// @ID           getAgentHealth
// @Param        id   path      string  true  "Agent ID"
// @Success      200  {object}  utils.APIResponse{data=api.AgentHealthResponse}
// @Failure      400  {object}  utils.APIResponse
// @Failure      404  {object}  utils.APIResponse
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/agents/{id}/health [get]
// getAgentReports retrieves paginated system reports for a specific agent.
// @Summary      Get agent reports
// @Description  Get a paginated list of system metric reports for a specific agent
// @Tags         agents
// @Accept       json
// @Produce      json
// @ID           getAgentReports
// @Param        id      path      string  true   "Agent ID"
// @Param        limit   query     int     false  "Maximum number of reports to return" default(50)
// @Param        offset  query     int     false  "Number of reports to skip" default(0)
// @Success      200     {object}  utils.APIResponse{data=object{reports=[]AgentReportResponse,count=int64,limit=int,offset=int,pagination=utils.PaginationMeta}}
// @Failure      400     {object}  utils.APIResponse
// @Failure      404     {object}  utils.APIResponse
// @Failure      500     {object}  utils.APIResponse
// @Router       /v1/agents/{id}/reports [get]
// Don't fail the request
// getAgentUptime returns agent uptime over a period.
// @Summary      Get agent uptime
// @Description  Returns daily uptime buckets and overall uptime percentage for an agent.
// @Tags         agents
// @Produce      json
// @ID           getAgentUptime
// @Param        id      path      string  true   "Agent ID"
// @Param        period  query     string  false  "Uptime period such as 7d, 30d, or 90d"
// @Success      200     {object}  object{daily_buckets=[]UptimeDayBucketResponse,uptime_percent=number}
// @Failure      400     {object}  utils.APIResponse
// @Failure      404     {object}  utils.APIResponse
// @Failure      500     {object}  utils.APIResponse
// @Router       /v1/agents/{id}/uptime [get]
// Verify agent exists
