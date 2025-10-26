package api

import (
	"orion/core/internal/service"
	"orion/core/internal/utils"

	"github.com/gin-gonic/gin"
)

func (s *Server) registerApplication(c *gin.Context) {
	var req service.RegisterApplicationRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		s.logger.Error("Invalid register application request", "error", err)
		utils.BadRequest(c, "Invalid request payload")
		return
	}

	response, err := s.appsService.RegisterApplication(&req)
	if err != nil {
		s.logger.Error("Failed to register application", "error", err)
		utils.InternalError(c, "Failed to register application", err)
		return
	}

	utils.SuccessResponse(c, 200, "Application registered successfully", response)
}
