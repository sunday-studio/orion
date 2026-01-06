package api

import (
	"orion/core/internal/service"
	"orion/core/internal/utils"

	"github.com/gin-gonic/gin"
)

func (s *Server) registerMonitor(c *gin.Context) {
	var req service.RegisterMonitorRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		s.logger.Error("Invalid register monitor request", "error", err)
		utils.BadRequest(c, "Invalid request payload")
		return
	}

	response, err := s.monitorService.RegisterMonitor(&req)
	if err != nil {
		s.logger.Error("Failed to register monitor", "error", err)
		utils.InternalError(c, "Failed to register monitor", err)
		return
	}

	utils.SuccessResponse(c, 200, "Monitor registered successfully", response)
}

func (s *Server) unregisterMonitor(c *gin.Context) {
	var req service.UnregisterMonitorRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		s.logger.Error("Invalid unregister monitor request", "error", err)
		utils.BadRequest(c, "Invalid request payload")
		return
	}

	response, err := s.monitorService.UnregisterMonitor(&req)
	if err != nil {
		s.logger.Error("Failed to unregister monitor", "error", err)
		utils.InternalError(c, "Failed to unregister monitor", err)
		return
	}

	utils.SuccessResponse(c, 200, "Monitor unregistered successfully", response)
}
