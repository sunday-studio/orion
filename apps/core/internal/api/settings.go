package api

import (
	"net/http"
	"orion/core/internal/service"
	"orion/core/internal/utils"
	"time"

	"github.com/gin-gonic/gin"
)

type DataLifecycleRollupRequest struct {
	Date string `json:"date,omitempty" example:"2026-05-12"`
}

// getDataLifecycleSettings retrieves current data lifecycle settings.
// @Summary      Get data lifecycle settings
// @Description  Get persisted raw report archive and uptime rollup settings
// @Tags         settings
// @Accept       json
// @Produce      json
// @ID           getDataLifecycleSettings
// @Success      200  {object}  utils.APIResponse{data=object{settings=db.DataLifecycleSettings}}
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/settings/data-lifecycle [get]
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

// updateDataLifecycleSettings updates current data lifecycle settings.
// @Summary      Update data lifecycle settings
// @Description  Update persisted raw report archive and uptime rollup settings
// @Tags         settings
// @Accept       json
// @Produce      json
// @ID           updateDataLifecycleSettings
// @Param        request  body      service.DataLifecycleSettingsPayload  true  "Data lifecycle settings"
// @Success      200      {object}  utils.APIResponse{data=object{settings=db.DataLifecycleSettings}}
// @Failure      400      {object}  utils.APIResponse
// @Failure      500      {object}  utils.APIResponse
// @Router       /v1/settings/data-lifecycle [put]
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

// runDataLifecycleRollup manually computes monitor uptime rollups.
// @Summary      Run data lifecycle rollup
// @Description  Compute daily monitor uptime rollups. When date is omitted, rolls up yesterday.
// @Tags         settings
// @Accept       json
// @Produce      json
// @ID           runDataLifecycleRollup
// @Param        request  body      DataLifecycleRollupRequest  false  "Optional rollup date"
// @Success      200      {object}  utils.APIResponse{data=object{result=service.RollupRunResult}}
// @Failure      400      {object}  utils.APIResponse
// @Failure      500      {object}  utils.APIResponse
// @Router       /v1/settings/data-lifecycle/actions/rollup [post]
func (s *Server) runDataLifecycleRollup(c *gin.Context) {
	var payload DataLifecycleRollupRequest
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&payload); err != nil {
			utils.BadRequest(c, "Invalid data lifecycle rollup payload")
			return
		}
	}

	var result *service.RollupRunResult
	var err error
	if payload.Date == "" {
		result, err = s.rollupService.RunDailyMonitorUptimeRollup(time.Now())
	} else {
		date, parseErr := time.Parse("2006-01-02", payload.Date)
		if parseErr != nil {
			utils.BadRequest(c, "date must use YYYY-MM-DD format")
			return
		}
		result, err = s.rollupService.RollupMonitorUptimeDay(date)
	}
	if err != nil {
		s.logger.Error("Failed to run data lifecycle rollup", "error", err)
		utils.InternalError(c, "Failed to run data lifecycle rollup", err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Data lifecycle rollup completed successfully", gin.H{
		"result": result,
	})
}

// runDataLifecycleArchive manually archives old raw reports.
// @Summary      Run data lifecycle archive
// @Description  Archive old raw reports to the configured local SQLite archive file
// @Tags         settings
// @Accept       json
// @Produce      json
// @ID           runDataLifecycleArchive
// @Success      200  {object}  utils.APIResponse{data=object{result=service.ArchiveRunResult}}
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/settings/data-lifecycle/actions/archive [post]
func (s *Server) runDataLifecycleArchive(c *gin.Context) {
	result, err := s.archiveService.RunRawReportArchive(time.Now())
	if err != nil {
		s.logger.Error("Failed to run data lifecycle archive", "error", err)
		utils.InternalError(c, "Failed to run data lifecycle archive", err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Data lifecycle archive completed successfully", gin.H{
		"result": result,
	})
}
