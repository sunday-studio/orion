package api

import (
	"context"
	"errors"
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"orion/core/internal/utils"
	"orion/core/internal/worker"
	"time"

	"github.com/gin-gonic/gin"
)

type CoreMonitorConfigResponse struct {
	MonitorID                 string                 `json:"monitor_id"`
	Kind                      string                 `json:"kind"`
	Config                    map[string]interface{} `json:"config"`
	SecretRefs                map[string]interface{} `json:"secret_refs"`
	IntervalSeconds           int                    `json:"interval_seconds"`
	TimeoutSeconds            int                    `json:"timeout_seconds"`
	ConfirmationPeriodSeconds int                    `json:"confirmation_period_seconds"`
	RecoveryPeriodSeconds     int                    `json:"recovery_period_seconds"`
	Paused                    bool                   `json:"paused"`
	NextRunAt                 time.Time              `json:"next_run_at"`
	LastRunAt                 *time.Time             `json:"last_run_at,omitempty"`
	LastSuccessAt             *time.Time             `json:"last_success_at,omitempty"`
	LastFailureAt             *time.Time             `json:"last_failure_at,omitempty"`
	LeaseOwner                string                 `json:"lease_owner,omitempty"`
	LeaseExpiresAt            *time.Time             `json:"lease_expires_at,omitempty"`
}

type CoreMonitorManagementResponse struct {
	Monitor MonitorResponse             `json:"monitor"`
	Config  CoreMonitorConfigResponse   `json:"config"`
	Result  *CoreMonitorTestNowResponse `json:"result,omitempty"`
}

type CoreMonitorTestNowResponse struct {
	MonitorID string `json:"monitor_id"`
	Status    string `json:"status"`
}

// createCoreMonitor creates a Core-managed monitor.
// @Summary      Create a Core monitor
// @Description  Create a Core-managed monitor and its runner configuration.
// @Tags         monitors
// @Accept       json
// @Produce      json
// @ID           createCoreMonitor
// @Param        request  body      service.CoreManagedMonitorCreateRequest true "Core monitor create request"
// @Success      201      {object}  utils.APIResponse{data=CoreMonitorManagementResponse}
// @Failure      400      {object}  utils.APIResponse
// @Failure      500      {object}  utils.APIResponse
// @Router       /v1/monitors [post]
func (s *Server) createCoreMonitor(c *gin.Context) {
	var req service.CoreManagedMonitorCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid request payload")
		return
	}

	record, err := s.coreMonitorManagementService.CreateCoreMonitor(req)
	if err != nil {
		s.handleCoreMonitorManagementError(c, err, "Failed to create core monitor")
		return
	}

	utils.SuccessResponse(c, http.StatusCreated, "Core monitor created successfully", s.coreMonitorManagementResponse(record))
}

// updateCoreMonitor updates a Core-managed monitor.
// @Summary      Update a Core monitor
// @Description  Update a Core-managed monitor and its runner configuration.
// @Tags         monitors
// @Accept       json
// @Produce      json
// @ID           updateCoreMonitor
// @Param        id       path      string true "Monitor ID"
// @Param        request  body      service.CoreManagedMonitorUpdateRequest true "Core monitor update request"
// @Success      200      {object}  utils.APIResponse{data=CoreMonitorManagementResponse}
// @Failure      400      {object}  utils.APIResponse
// @Failure      404      {object}  utils.APIResponse
// @Failure      500      {object}  utils.APIResponse
// @Router       /v1/monitors/{id} [patch]
func (s *Server) updateCoreMonitor(c *gin.Context) {
	var req service.CoreManagedMonitorUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid request payload")
		return
	}

	record, err := s.coreMonitorManagementService.UpdateCoreMonitor(c.Param("id"), req)
	if err != nil {
		s.handleCoreMonitorManagementError(c, err, "Failed to update core monitor")
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Core monitor updated successfully", s.coreMonitorManagementResponse(record))
}

// deleteCoreMonitor deletes a Core-managed monitor.
// @Summary      Delete a Core monitor
// @Description  Soft delete a Core-managed monitor and pause its runner configuration.
// @Tags         monitors
// @Produce      json
// @ID           deleteCoreMonitor
// @Param        id   path      string true "Monitor ID"
// @Success      200  {object}  utils.APIResponse{data=object{success=bool}}
// @Failure      404  {object}  utils.APIResponse
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/monitors/{id} [delete]
func (s *Server) deleteCoreMonitor(c *gin.Context) {
	if err := s.coreMonitorManagementService.DeleteCoreMonitor(c.Param("id")); err != nil {
		s.handleCoreMonitorManagementError(c, err, "Failed to delete core monitor")
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Core monitor deleted successfully", gin.H{"success": true})
}

// pauseCoreMonitor pauses a Core-managed monitor.
// @Summary      Pause a Core monitor
// @Description  Pause a Core-managed monitor so the worker will not claim it.
// @Tags         monitors
// @Produce      json
// @ID           pauseCoreMonitor
// @Param        id   path      string true "Monitor ID"
// @Success      200  {object}  utils.APIResponse{data=CoreMonitorManagementResponse}
// @Failure      404  {object}  utils.APIResponse
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/monitors/{id}/pause [post]
func (s *Server) pauseCoreMonitor(c *gin.Context) {
	record, err := s.coreMonitorManagementService.PauseCoreMonitor(c.Param("id"))
	if err != nil {
		s.handleCoreMonitorManagementError(c, err, "Failed to pause core monitor")
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Core monitor paused successfully", s.coreMonitorManagementResponse(record))
}

// resumeCoreMonitor resumes a Core-managed monitor.
// @Summary      Resume a Core monitor
// @Description  Resume a Core-managed monitor and make it due immediately.
// @Tags         monitors
// @Produce      json
// @ID           resumeCoreMonitor
// @Param        id   path      string true "Monitor ID"
// @Success      200  {object}  utils.APIResponse{data=CoreMonitorManagementResponse}
// @Failure      404  {object}  utils.APIResponse
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/monitors/{id}/resume [post]
func (s *Server) resumeCoreMonitor(c *gin.Context) {
	record, err := s.coreMonitorManagementService.ResumeCoreMonitor(c.Param("id"))
	if err != nil {
		s.handleCoreMonitorManagementError(c, err, "Failed to resume core monitor")
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Core monitor resumed successfully", s.coreMonitorManagementResponse(record))
}

// testCoreMonitor executes one immediate Core monitor check.
// @Summary      Test a Core monitor
// @Description  Execute a Core-managed monitor immediately and store the resulting monitor report.
// @Tags         monitors
// @Produce      json
// @ID           testCoreMonitor
// @Param        id   path      string true "Monitor ID"
// @Success      200  {object}  utils.APIResponse{data=CoreMonitorManagementResponse}
// @Failure      404  {object}  utils.APIResponse
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/monitors/{id}/test [post]
func (s *Server) testCoreMonitor(c *gin.Context) {
	record, err := s.coreMonitorManagementService.GetCoreMonitorConfig(c.Param("id"))
	if err != nil {
		s.handleCoreMonitorManagementError(c, err, "Failed to load core monitor")
		return
	}

	workerApp := worker.NewApp(s.db, s.logger, worker.Options{
		WorkerID: "core-monitor-test-now",
		Config:   s.cfg,
	})
	checkCtx, cancel := context.WithTimeout(c.Request.Context(), time.Duration(record.Config.TimeoutSeconds+2)*time.Second)
	defer cancel()
	if err := workerApp.RunImmediateCheck(checkCtx, record.Config); err != nil {
		s.logger.Error("Core monitor test-now failed", "monitor_id", record.Monitor.ID, "error", err)
		utils.InternalError(c, "Failed to test core monitor", err)
		return
	}

	refreshed, err := s.coreMonitorManagementService.GetCoreMonitorConfig(record.Monitor.ID)
	if err != nil {
		s.handleCoreMonitorManagementError(c, err, "Failed to load core monitor")
		return
	}
	response := s.coreMonitorManagementResponse(refreshed)
	response.Result = &CoreMonitorTestNowResponse{MonitorID: record.Monitor.ID, Status: "completed"}
	utils.SuccessResponse(c, http.StatusOK, "Core monitor tested successfully", response)
}

// getCoreMonitorConfig returns redacted Core monitor configuration.
// @Summary      Get Core monitor config
// @Description  Return redacted Core-managed monitor configuration.
// @Tags         monitors
// @Produce      json
// @ID           getCoreMonitorConfig
// @Param        id   path      string true "Monitor ID"
// @Success      200  {object}  utils.APIResponse{data=CoreMonitorManagementResponse}
// @Failure      404  {object}  utils.APIResponse
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/monitors/{id}/config [get]
func (s *Server) getCoreMonitorConfig(c *gin.Context) {
	record, err := s.coreMonitorManagementService.GetCoreMonitorConfig(c.Param("id"))
	if err != nil {
		s.handleCoreMonitorManagementError(c, err, "Failed to load core monitor config")
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Core monitor config retrieved successfully", s.coreMonitorManagementResponse(record))
}

func (s *Server) coreMonitorManagementResponse(record *service.CoreManagedMonitorRecord) CoreMonitorManagementResponse {
	monitor := monitorResponse(record.Monitor)
	monitor.OwnerKind = "core"
	monitor.Source = "core"
	if agent, err := s.agentService.GetAgent(record.Monitor.AgentID); err == nil {
		monitor.AgentName = agent.Name
		monitor.OwnerName = agent.Name
	}
	return CoreMonitorManagementResponse{
		Monitor: monitor,
		Config:  coreMonitorConfigResponse(record.Config),
	}
}

func coreMonitorConfigResponse(config db.CoreMonitorConfig) CoreMonitorConfigResponse {
	return CoreMonitorConfigResponse{
		MonitorID:                 config.MonitorID,
		Kind:                      config.Kind,
		Config:                    service.RedactCoreMonitorConfigJSON(config.ConfigJSON),
		SecretRefs:                service.RedactCoreMonitorSecretRefJSON(config.SecretRefJSON),
		IntervalSeconds:           config.IntervalSeconds,
		TimeoutSeconds:            config.TimeoutSeconds,
		ConfirmationPeriodSeconds: config.ConfirmationPeriodSeconds,
		RecoveryPeriodSeconds:     config.RecoveryPeriodSeconds,
		Paused:                    config.Paused,
		NextRunAt:                 config.NextRunAt,
		LastRunAt:                 config.LastRunAt,
		LastSuccessAt:             config.LastSuccessAt,
		LastFailureAt:             config.LastFailureAt,
		LeaseOwner:                config.LeaseOwner,
		LeaseExpiresAt:            config.LeaseExpiresAt,
	}
}

func (s *Server) handleCoreMonitorManagementError(c *gin.Context, err error, message string) {
	switch {
	case errors.Is(err, service.ErrCoreManagedMonitorUnsupportedKind):
		utils.BadRequest(c, "Unsupported core monitor type")
	case errors.Is(err, service.ErrCoreManagedMonitorValidation):
		utils.BadRequest(c, err.Error())
	case errors.Is(err, service.ErrCoreManagedMonitorNotFound):
		utils.NotFound(c, "Core monitor not found")
	default:
		s.logger.Error(message, "error", err)
		utils.InternalError(c, message, err)
	}
}
