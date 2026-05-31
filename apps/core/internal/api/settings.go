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

const (
	dataLifecycleAuditObjectType = "data_lifecycle"
	dataLifecycleAuditObjectID   = "settings"
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

	previous, err := s.settingsService.GetDataLifecycleSettings()
	if err != nil {
		s.logger.Error("Failed to get data lifecycle settings before update", "error", err)
		utils.InternalError(c, "Failed to get data lifecycle settings", err)
		return
	}

	settings, err := s.settingsService.UpdateDataLifecycleSettings(payload)
	if err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := s.recordDataLifecycleAuditEvent(c, service.DataLifecycleAuditActionSettingsUpdated, map[string]interface{}{
		"action_type":        "settings_update",
		"result_status":      "success",
		"changed_fields":     dataLifecycleChangedFields(previous, settings),
		"archive_configured": strings.TrimSpace(settings.ArchiveDir) != "",
	}); err != nil {
		s.logger.Error("Failed to record data lifecycle settings audit event", "error", err)
		utils.InternalError(c, "Failed to record data lifecycle settings audit event", err)
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
		_ = s.recordDataLifecycleAuditEvent(c, service.DataLifecycleAuditActionRollupRun, dataLifecycleRollupAuditMetadata(payload.Date, nil, "failed"))
		utils.InternalError(c, "Failed to run data lifecycle rollup", err)
		return
	}
	if err := s.recordDataLifecycleAuditEvent(c, service.DataLifecycleAuditActionRollupRun, dataLifecycleRollupAuditMetadata(payload.Date, result, dataLifecycleRollupStatus(result))); err != nil {
		s.logger.Error("Failed to record data lifecycle rollup audit event", "error", err)
		utils.InternalError(c, "Failed to record data lifecycle rollup audit event", err)
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
		_ = s.recordDataLifecycleAuditEvent(c, service.DataLifecycleAuditActionArchiveRun, dataLifecycleArchiveAuditMetadata(nil, "failed"))
		utils.InternalError(c, "Failed to run data lifecycle archive", nil)
		return
	}
	if err := s.recordDataLifecycleAuditEvent(c, service.DataLifecycleAuditActionArchiveRun, dataLifecycleArchiveAuditMetadata(result, dataLifecycleArchiveStatus(result))); err != nil {
		s.logger.Error("Failed to record data lifecycle archive audit event", "error", err)
		utils.InternalError(c, "Failed to record data lifecycle archive audit event", err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Data lifecycle archive completed successfully", gin.H{
		"result": result,
	})
}

func (s *Server) recordDataLifecycleAuditEvent(c *gin.Context, action string, metadata map[string]interface{}) error {
	actorType, actorID := dataLifecycleAuditActor(c)
	_, err := service.NewAuditService(s.db, s.logger).RecordEvent(service.AuditEventInput{
		Action:             action,
		AffectedObjectType: dataLifecycleAuditObjectType,
		AffectedObjectID:   dataLifecycleAuditObjectID,
		ActorType:          actorType,
		ActorID:            actorID,
		Metadata:           metadata,
	})
	return err
}

func dataLifecycleAuditActor(c *gin.Context) (string, string) {
	if actor, ok := c.Get("frontend_actor_id"); ok {
		if actorID, ok := actor.(string); ok && strings.TrimSpace(actorID) != "" {
			return "user", strings.TrimSpace(actorID)
		}
	}
	return "system", "console"
}

func dataLifecycleChangedFields(previous *db.DataLifecycleSettings, updated *db.DataLifecycleSettings) []string {
	if previous == nil || updated == nil {
		return []string{}
	}
	fields := make([]string, 0, 6)
	if previous.RawReportHotDays != updated.RawReportHotDays {
		fields = append(fields, "raw_report_hot_days")
	}
	if previous.ArchiveRawReports != updated.ArchiveRawReports {
		fields = append(fields, "archive_raw_reports")
	}
	if previous.ArchiveDir != updated.ArchiveDir {
		fields = append(fields, "archive_dir")
	}
	if previous.RollupsEnabled != updated.RollupsEnabled {
		fields = append(fields, "rollups_enabled")
	}
	if dataLifecycleOptionalInt(previous.RollupRetentionDays) != dataLifecycleOptionalInt(updated.RollupRetentionDays) {
		fields = append(fields, "rollup_retention_days")
	}
	if previous.ArchiveSchedule != updated.ArchiveSchedule {
		fields = append(fields, "archive_schedule")
	}
	return fields
}

func dataLifecycleOptionalInt(value *int) int {
	if value == nil {
		return -1
	}
	return *value
}

func dataLifecycleRollupAuditMetadata(requestedDate string, result *service.RollupRunResult, status string) map[string]interface{} {
	metadata := map[string]interface{}{
		"action_type":   "manual_rollup",
		"result_status": status,
	}
	if strings.TrimSpace(requestedDate) != "" {
		metadata["requested_date"] = strings.TrimSpace(requestedDate)
	}
	if result != nil {
		metadata["date"] = result.Date
		metadata["monitor_days"] = result.MonitorDays
		metadata["report_count"] = result.ReportCount
		metadata["skipped_today"] = result.SkippedToday
	}
	return metadata
}

func dataLifecycleRollupStatus(result *service.RollupRunResult) string {
	if result != nil && result.SkippedToday {
		return "skipped"
	}
	return "success"
}

func dataLifecycleArchiveAuditMetadata(result *service.ArchiveRunResult, status string) map[string]interface{} {
	metadata := map[string]interface{}{
		"action_type":   "manual_archive",
		"result_status": status,
	}
	if result != nil {
		metadata["agent_reports_archived"] = result.AgentReportsArchived
		metadata["monitor_reports_archived"] = result.MonitorReportsArchived
		metadata["archive_raw_reports"] = result.ArchiveRawReports
		metadata["skipped_because_disabled"] = result.SkippedBecauseDisabled
		metadata["skipped_because_no_reports"] = result.SkippedBecauseNoReports
	}
	return metadata
}

func dataLifecycleArchiveStatus(result *service.ArchiveRunResult) string {
	if result != nil && (result.SkippedBecauseDisabled || result.SkippedBecauseNoReports) {
		return "skipped"
	}
	return "success"
}
