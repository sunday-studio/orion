package api

import (
	"orion/core/internal/service"
	"orion/core/internal/utils"
	"strconv"

	"github.com/gin-gonic/gin"
)

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
