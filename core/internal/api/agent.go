package api

import (
	"orion/core/internal/service"
	"orion/core/internal/utils"

	"github.com/gin-gonic/gin"
)

// registerAgent handles agent registration requests
func (s *Server) registerAgent(c *gin.Context) {
	var req service.RegisterRequest
	
	// Bind JSON request
	if err := c.ShouldBindJSON(&req); err != nil {
		s.logger.Error("Invalid registration request", "error", err)
		utils.BadRequest(c, "Invalid request payload")
		return
	}

	s.logger.Info("Agent registration request", "uuid", req.UUID, "name", req.Name, "os", req.OS, "arch", req.Arch)

	// Register agent
	response, err := s.agentService.RegisterAgent(&req)
	if err != nil {
		s.logger.Error("Failed to register agent", "error", err)
		utils.InternalError(c, "Failed to register agent", err)
		return
	}

	s.logger.Info("Agent registered successfully", "agent_id", response.AgentID, "uuid", req.UUID)
	utils.SuccessResponse(c, 200, "Agent registered successfully", response)
}
