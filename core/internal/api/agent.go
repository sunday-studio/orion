package api

import (
	"orion/core/internal/service"
	"orion/core/internal/utils"
	"strconv"

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
// @Success      200     {object}  utils.APIResponse{data=object{agents=[]db.Agent,count=int64,limit=int,offset=int}}
// @Failure      500     {object}  utils.APIResponse
// @Router       /v1/agents [get]
func (s *Server) listAgents(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 0 {
		limit = 50
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	agents, err := s.agentService.ListAgents(limit, offset)
	if err != nil {
		s.logger.Error("Failed to list agents", "error", err)
		utils.InternalError(c, "Failed to list agents", err)
		return
	}

	count, err := s.agentService.GetAgentCount()
	if err != nil {
		s.logger.Error("Failed to get agent count", "error", err)
		// Don't fail the request if count fails
	}

	utils.SuccessResponse(c, 200, "Agents retrieved successfully", gin.H{
		"agents": agents,
		"count":  count,
		"limit":  limit,
		"offset": offset,
	})
}

// getAgentDetail retrieves detailed information about a specific agent
// @Summary      Get agent details
// @Description  Get detailed information about a specific agent including latest report
// @Tags         agents
// @Accept       json
// @Produce      json
// @ID           getAgent
// @Param        id   path      string  true  "Agent ID"
// @Success      200  {object}  utils.APIResponse{data=object{agent=db.Agent,latest_report=object}}
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
		latestReport = reports[0]
	}

	utils.SuccessResponse(c, 200, "Agent retrieved successfully", gin.H{
		"agent":         agent,
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
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/agents/{id}/health [get]
func (s *Server) getAgentHealth(c *gin.Context) {
	agentID := c.Param("id")
	if agentID == "" {
		utils.BadRequest(c, "Agent ID is required")
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
