package api

import (
	"orion/core/internal/db"
	"orion/core/internal/service"
	"orion/core/internal/utils"
	"time"

	"github.com/gin-gonic/gin"
)

// registerAgent registers a new agent or reconnects an existing one
// @Summary      Register an agent
// @Description  Register a new agent or reconnect an existing agent by machine ID
// @Tags         agents
// @Accept       json
// @Produce      json
// @ID           registerAgent
// @Param        request  body      service.RegisterRequest  true  "Agent registration request"
// @Success      200      {object}  utils.APIResponse{data=service.RegisterResponse}
// @Failure      400      {object}  utils.APIResponse
// @Failure      500      {object}  utils.APIResponse
// @Router       /v1/register [post]
func (s *Server) registerAgent(c *gin.Context) {
	var req service.RegisterRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		s.logger.Error("Invalid registration request", "error", err)
		utils.BadRequest(c, "Invalid request payload")
		return
	}

	s.logger.Info("Agent registration request", "machine_id", req.MachineId, "name", req.Name, "os", req.OS, "arch", req.Arch)

	response, err := s.agentService.RegisterAgent(&req)
	if err != nil {
		s.logger.Error("Failed to register agent", "error", err)
		utils.InternalError(c, "Failed to register agent", err)
		return
	}

	s.logger.Info("Agent registered successfully", "agent_id", response.AgentID, "machine_id", req.MachineId)
	utils.SuccessResponse(c, 200, "Agent registered successfully", response)
}

// setMaintenanceMode sets the maintenance mode for an agent
// @Summary      Set agent maintenance mode
// @Description  Enable or disable maintenance mode for a specific agent
// @Tags         agents
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           setMaintenanceMode
// @Param        agent_id  path      string                           true  "Agent ID"
// @Param        request   body      service.SetMaintenanceModeRequest true  "Maintenance mode request"
// @Success      200       {object}  utils.APIResponse
// @Failure      400       {object}  utils.APIResponse
// @Failure      401       {object}  utils.APIResponse
// @Failure      500       {object}  utils.APIResponse
// @Router       /v1/agents/{agent_id}/maintenance [put]
func (s *Server) setMaintenanceMode(c *gin.Context) {
	agentID := c.Param("agent_id")
	if agentID == "" {
		utils.BadRequest(c, "Agent ID is required")
		return
	}

	var req service.SetMaintenanceModeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		s.logger.Error("Invalid maintenance mode request", "error", err)
		utils.BadRequest(c, "Invalid request payload")
		return
	}

	if err := s.agentService.SetMaintenanceMode(agentID, req.MaintenanceMode); err != nil {
		s.logger.Error("Failed to set maintenance mode", "error", err, "agent_id", agentID)
		utils.InternalError(c, "Failed to set maintenance mode", err)
		return
	}

	s.logger.Info("Maintenance mode set", "agent_id", agentID, "maintenance_mode", req.MaintenanceMode)
	utils.SuccessResponse(c, 200, "Maintenance mode updated successfully", gin.H{
		"agent_id":         agentID,
		"maintenance_mode": req.MaintenanceMode,
	})
}

// listAgents retrieves a paginated list of agents
// @Summary      List agents
// @Description  Get a paginated list of all registered agents
// @Tags         agents
// @Accept       json
// @Produce      json
// @ID           getAgents
// @Param        limit   query     int     false  "Maximum number of agents to return" default(50)
// @Param        offset  query     int     false  "Number of agents to skip" default(0)
// @Param        search  query     string  false  "Search by server or monitor name"
// @Param        status  query     string  false  "Filter by computed server status"
// @Param        maintenance  query  string  false  "Filter by maintenance mode true or false"
// @Param        stale_only  query   bool    false  "Only return stale servers"
// @Param        has_incidents  query  bool  false  "Only return servers with active incidents"
// @Success      200     {object}  utils.APIResponse{data=object{agents=[]AgentResponse,count=int64,limit=int,offset=int,pagination=utils.PaginationMeta}}
// @Failure      500     {object}  utils.APIResponse
// @Router       /v1/agents [get]
func (s *Server) listAgents(c *gin.Context) {
	limit := queryInt(c, "limit", 50)
	offset := queryInt(c, "offset", 0)

	opts := service.ListAgentsOpts{
		Limit:        limit,
		Offset:       offset,
		Search:       c.Query("search"),
		Status:       c.Query("status"),
		Maintenance:  c.Query("maintenance"),
		StaleOnly:    c.Query("stale_only") == "true",
		HasIncidents: c.Query("has_incidents") == "true",
		LastSeen:     c.Query("last_seen"),
		Uptime:       c.Query("uptime"),
		Sort:         c.DefaultQuery("sort", "last_seen"),
		Order:        c.DefaultQuery("order", "desc"),
	}

	agents, count, err := s.agentService.ListAgents(opts)
	if err != nil {
		s.logger.Error("Failed to list agents", "error", err)
		utils.InternalError(c, "Failed to list agents", err)
		return
	}

	responses := agentListResponses(agents)
	utils.SuccessResponse(c, 200, "Agents retrieved successfully", gin.H{
		"agents":     responses,
		"count":      count,
		"limit":      limit,
		"offset":     offset,
		"pagination": utils.NewPaginationMeta(count, limit, offset, len(responses)),
	})
}

// getAgentSummary retrieves aggregate counts for the agent list.
// @Summary      Get agent summary
// @Description  Get aggregate counts for registered agents by health, maintenance, stale, and incident status
// @Tags         agents
// @Accept       json
// @Produce      json
// @ID           getAgentSummary
// @Success      200  {object}  utils.APIResponse{data=api.AgentSummaryResponse}
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/agents/summary [get]
func (s *Server) getAgentSummary(c *gin.Context) {
	healthService := service.NewHealthService(s.db, s.logger)
	config := service.DefaultHealthConfig()

	var agents []db.Agent
	if err := s.db.Where("deleted_at IS NULL OR deleted_at = ?", time.Time{}).Find(&agents).Error; err != nil {
		s.logger.Error("Failed to load agent summary", "error", err)
		utils.InternalError(c, "Failed to get agent summary", err)
		return
	}

	summary := AgentSummaryResponse{Total: int64(len(agents))}
	agentIDs := make([]string, 0, len(agents))

	for _, agent := range agents {
		agentIDs = append(agentIDs, agent.ID)
		if agent.MaintenanceMode {
			summary.Maintenance++
		}

		health := "unknown"
		if computedHealth, _, _, _, err := healthService.ComputeAgentHealth(agent.ID, config); err == nil {
			health = computedHealth
		}

		switch health {
		case "up":
			summary.Up++
		case "down":
			summary.Down++
		case "degraded":
			summary.Degraded++
		case "maintenance":
			// Maintenance is counted from agent state above and is not an unknown health state.
		case "stale":
			summary.Stale++
		default:
			summary.Unknown++
		}
	}

	if len(agentIDs) > 0 {
		var rows []struct {
			AgentID string
		}
		if err := s.db.Model(&db.Incident{}).
			Select("agent_id").
			Where("agent_id IN ? AND status IN ?", agentIDs, []string{"open", "acknowledged"}).
			Group("agent_id").
			Find(&rows).Error; err != nil {
			s.logger.Error("Failed to load agent incident summary", "error", err)
			utils.InternalError(c, "Failed to get agent summary", err)
			return
		}
		summary.HasIncidents = int64(len(rows))
	}

	utils.SuccessResponse(c, 200, "Agent summary retrieved successfully", summary)
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

	// Get latest agent report for system metrics
	reports, err := s.reportService.GetAgentReportsById(agentID, 1, 0)
	if err != nil {
		s.logger.Error("Failed to get agent reports", "error", err, "agent_id", agentID)
		// Don't fail if reports can't be retrieved
	}

	var latestReport interface{}
	if len(reports) > 0 {
		latestReport = agentReportResponse(reports[0])
	}

	utils.SuccessResponse(c, 200, "Agent retrieved successfully", gin.H{
		"agent":         agentResponse(*agent),
		"latest_report": latestReport,
	})
}

// getAgentHealth retrieves health status for a specific agent
// @Summary      Get agent health
// @Description  Get computed health status for a specific agent and its monitors
// @Tags         agents
// @Accept       json
// @Produce      json
// @ID           getAgentHealth
// @Param        id   path      string  true  "Agent ID"
// @Success      200  {object}  utils.APIResponse{data=object{agent_id=string,overall_health=string,up_count=int,down_count=int,degraded_count=int}}
// @Failure      400  {object}  utils.APIResponse
// @Failure      404  {object}  utils.APIResponse
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/agents/{id}/health [get]
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

	overallHealth, upCount, downCount, degradedCount, err := healthService.ComputeAgentHealth(agentID, config)
	if err != nil {
		s.logger.Error("Failed to compute agent health", "error", err, "agent_id", agentID)
		utils.InternalError(c, "Failed to compute agent health", err)
		return
	}

	utils.SuccessResponse(c, 200, "Agent health retrieved successfully", gin.H{
		"agent_id":       agentID,
		"overall_health": overallHealth,
		"up_count":       upCount,
		"down_count":     downCount,
		"degraded_count": degradedCount,
	})
}

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
		// Don't fail the request
		count = int64(len(reports))
	}

	responses := agentReportResponses(reports)
	utils.SuccessResponse(c, 200, "Agent reports retrieved successfully", gin.H{
		"reports":    responses,
		"count":      count,
		"limit":      limit,
		"offset":     offset,
		"pagination": utils.NewPaginationMeta(count, limit, offset, len(responses)),
	})
}

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
func (s *Server) getAgentUptime(c *gin.Context) {
	agentID := c.Param("id")
	if agentID == "" {
		utils.BadRequest(c, "Agent ID is required")
		return
	}

	period := c.DefaultQuery("period", "90d")

	// Verify agent exists
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

	utils.SuccessResponse(c, 200, "Agent uptime retrieved successfully", gin.H{
		"daily_buckets":  result.DailyBuckets,
		"uptime_percent": result.UptimePercent,
	})
}
