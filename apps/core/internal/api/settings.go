package api

import (
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"orion/core/internal/utils"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
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

	actorType, actorID := statusPageAuditActor(c)
	var settings *db.DataLifecycleSettings
	err := s.db.Transaction(func(tx *gorm.DB) error {
		settingsService := service.NewSettingsService(tx, s.logger, s.cfg.DataDir)
		current, err := settingsService.GetDataLifecycleSettings()
		if err != nil {
			return err
		}
		updated, err := settingsService.UpdateDataLifecycleSettings(payload)
		if err != nil {
			return err
		}
		settings = updated
		_, err = service.NewAuditService(tx, s.logger).RecordDataLifecycleEvent(service.DataLifecycleAuditEventInput{
			Action:           service.DataLifecycleAuditActionSettingsUpdated,
			AffectedObjectID: "settings",
			ActorType:        actorType,
			ActorID:          actorID,
			Metadata: map[string]interface{}{
				"changed_fields": dataLifecycleChangedFields(current, updated),
				"previous":       dataLifecycleSettingsSnapshot(current),
				"current":        dataLifecycleSettingsSnapshot(updated),
			},
		})
		return err
	})
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
		_ = s.recordDataLifecycleAuditEvent(c, service.DataLifecycleAuditEventInput{
			Action:           service.DataLifecycleAuditActionRollupFailed,
			AffectedObjectID: "rollup",
			Metadata: map[string]interface{}{
				"date":  payload.Date,
				"error": err.Error(),
			},
		})
		s.logger.Error("Failed to run data lifecycle rollup", "error", err)
		utils.InternalError(c, "Failed to run data lifecycle rollup", err)
		return
	}
	if err := s.recordDataLifecycleAuditEvent(c, service.DataLifecycleAuditEventInput{
		Action:           service.DataLifecycleAuditActionRollupRan,
		AffectedObjectID: "rollup",
		Metadata: map[string]interface{}{
			"date":          result.Date,
			"monitor_days":  result.MonitorDays,
			"report_count":  result.ReportCount,
			"skipped_today": result.SkippedToday,
		},
	}); err != nil {
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
		_ = s.recordDataLifecycleAuditEvent(c, service.DataLifecycleAuditEventInput{
			Action:           service.DataLifecycleAuditActionArchiveFailed,
			AffectedObjectID: "archive",
			Metadata: map[string]interface{}{
				"error": err.Error(),
			},
		})
		s.logger.Error("Failed to run data lifecycle archive", "error", err)
		utils.InternalError(c, "Failed to run data lifecycle archive", err)
		return
	}
	if err := s.recordDataLifecycleAuditEvent(c, service.DataLifecycleAuditEventInput{
		Action:           service.DataLifecycleAuditActionArchiveRan,
		AffectedObjectID: "archive",
		Metadata: map[string]interface{}{
			"archive_path":               result.ArchivePath,
			"cutoff":                     result.Cutoff,
			"agent_reports_archived":     result.AgentReportsArchived,
			"monitor_reports_archived":   result.MonitorReportsArchived,
			"archive_raw_reports":        result.ArchiveRawReports,
			"skipped_because_disabled":   result.SkippedBecauseDisabled,
			"skipped_because_no_reports": result.SkippedBecauseNoReports,
		},
	}); err != nil {
		s.logger.Error("Failed to record data lifecycle archive audit event", "error", err)
		utils.InternalError(c, "Failed to record data lifecycle archive audit event", err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Data lifecycle archive completed successfully", gin.H{
		"result": result,
	})
}

func (s *Server) recordDataLifecycleAuditEvent(c *gin.Context, input service.DataLifecycleAuditEventInput) error {
	input.ActorType, input.ActorID = statusPageAuditActor(c)
	_, err := service.NewAuditService(s.db, s.logger).RecordDataLifecycleEvent(input)
	return err
}

func dataLifecycleSettingsSnapshot(settings *db.DataLifecycleSettings) map[string]interface{} {
	if settings == nil {
		return map[string]interface{}{}
	}
	return map[string]interface{}{
		"raw_report_hot_days":   settings.RawReportHotDays,
		"archive_raw_reports":   settings.ArchiveRawReports,
		"archive_dir":           settings.ArchiveDir,
		"rollups_enabled":       settings.RollupsEnabled,
		"rollup_retention_days": dataLifecycleIntPointerValue(settings.RollupRetentionDays),
		"archive_schedule":      settings.ArchiveSchedule,
		"last_rollup_run_at":    settings.LastRollupRunAt,
		"last_archive_run_at":   settings.LastArchiveRunAt,
		"last_archive_status":   settings.LastArchiveStatus,
		"last_archive_error":    settings.LastArchiveError,
	}
}

func dataLifecycleChangedFields(previous *db.DataLifecycleSettings, current *db.DataLifecycleSettings) []string {
	before := dataLifecycleSettingsSnapshot(previous)
	after := dataLifecycleSettingsSnapshot(current)
	fields := make([]string, 0)
	for _, field := range []string{
		"raw_report_hot_days",
		"archive_raw_reports",
		"archive_dir",
		"rollups_enabled",
		"rollup_retention_days",
		"archive_schedule",
	} {
		if before[field] != after[field] {
			fields = append(fields, field)
		}
	}
	return fields
}

func dataLifecycleIntPointerValue(value *int) interface{} {
	if value == nil {
		return nil
	}
	return *value
}
