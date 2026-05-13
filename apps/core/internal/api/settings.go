package api

import (
	"net/http"
	"orion/core/internal/service"
	"orion/core/internal/utils"

	"github.com/gin-gonic/gin"
)

func (s *Server) getDataLifecycleSettings(c *gin.Context) {
	settings, err := s.settingsService.GetDataLifecycleSettings()
	if err != nil {
		s.logger.Error("Failed to get data lifecycle settings", "error", err)
		utils.InternalError(c, "Failed to get data lifecycle settings", err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Data lifecycle settings retrieved successfully", gin.H{
		"settings": settings,
	})
}

func (s *Server) updateDataLifecycleSettings(c *gin.Context) {
	var payload service.DataLifecycleSettingsPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		utils.BadRequest(c, "Invalid data lifecycle settings payload")
		return
	}

	settings, err := s.settingsService.UpdateDataLifecycleSettings(payload)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Data lifecycle settings updated successfully", gin.H{
		"settings": settings,
	})
}
