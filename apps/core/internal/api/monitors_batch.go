package api

import (
	"errors"
	"fmt"
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"orion/core/internal/utils"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type MonitorMaintenanceRequest struct {
	// StartsAt defaults to now. Give EndsAt or DurationSeconds.
	StartsAt        *time.Time `json:"starts_at,omitempty"`
	EndsAt          *time.Time `json:"ends_at,omitempty"`
	DurationSeconds int        `json:"duration_seconds,omitempty"`
	Reason          string     `json:"reason,omitempty"`
}

func (r MonitorMaintenanceRequest) window(now time.Time) (service.MaintenanceWindow, error) {
	w := service.MaintenanceWindow{StartsAt: r.StartsAt, Reason: r.Reason}
	switch {
	case r.EndsAt != nil:
		w.EndsAt = *r.EndsAt
	case r.DurationSeconds > 0:
		start := now
		if r.StartsAt != nil {
			start = *r.StartsAt
		}
		w.EndsAt = start.Add(time.Duration(r.DurationSeconds) * time.Second)
	default:
		return w, errors.New("give ends_at or duration_seconds")
	}
	return w, nil
}

// setMonitorMaintenance starts or schedules a maintenance window.
// @Summary      Set a monitor maintenance window
// @Description  Checks keep running but incidents and alerts are held between starts_at (default now) and ends_at, at most 30 days. Replaces any existing window.
// @Tags         monitors
// @Accept       json
// @Produce      json
// @ID           setMonitorMaintenance
// @Param        id       path      string                     true  "Monitor ID"
// @Param        request  body      MonitorMaintenanceRequest  true  "Window"
// @Success      200      {object}  utils.APIResponse{data=object{maintenance=MonitorMaintenanceInfo}}
// @Failure      400      {object}  utils.APIResponse
// @Failure      404      {object}  utils.APIResponse
// @Router       /v1/monitors/{id}/maintenance [put]
func (s *Server) setMonitorMaintenance(c *gin.Context) {
	var req MonitorMaintenanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid maintenance payload")
		return
	}
	window, err := req.window(time.Now().UTC())
	if err == nil {
		err = s.monitorService.SetMonitorMaintenance(c.Param("id"), window)
	}
	switch {
	case err == nil:
		var m db.Monitor
		s.db.Where("id = ?", c.Param("id")).First(&m)
		utils.SuccessResponse(c, http.StatusOK, "Maintenance window set", gin.H{"maintenance": maintenanceInfo(m, time.Now().UTC())})
	case errors.Is(err, service.ErrMonitorNotFound):
		utils.NotFound(c, "Monitor not found")
	default:
		utils.BadRequest(c, strings.TrimPrefix(err.Error(), service.ErrIncidentInvalid.Error()+": "))
	}
}

// clearMonitorMaintenance ends a maintenance window immediately.
// @Summary      Clear a monitor maintenance window
// @Tags         monitors
// @Produce      json
// @ID           clearMonitorMaintenance
// @Param        id   path      string  true  "Monitor ID"
// @Success      200  {object}  utils.APIResponse
// @Failure      404  {object}  utils.APIResponse
// @Router       /v1/monitors/{id}/maintenance [delete]
func (s *Server) clearMonitorMaintenance(c *gin.Context) {
	if err := s.monitorService.ClearMonitorMaintenance(c.Param("id")); err != nil {
		if errors.Is(err, service.ErrMonitorNotFound) {
			utils.NotFound(c, "Monitor not found")
			return
		}
		utils.InternalError(c, "Failed to clear maintenance", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Maintenance window cleared", gin.H{})
}

type BulkMonitorRequest struct {
	IDs    []string `json:"ids" binding:"required"`
	Action string   `json:"action" binding:"required" enums:"pause,resume,maintenance,delete,clone"`
	// Maintenance applies to the maintenance action.
	Maintenance *MonitorMaintenanceRequest `json:"maintenance,omitempty"`
	// KeepHistory applies to delete (default true).
	KeepHistory *bool `json:"keep_history,omitempty"`
}

type BulkMonitorResult struct {
	ID    string `json:"id"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
	// NewID is the clone's id for the clone action.
	NewID string `json:"new_id,omitempty"`
}

// bulkMonitorAction applies one action to many monitors.
// @Summary      Bulk monitor action
// @Description  Pause, resume, set maintenance, delete or clone up to 100 monitors. Each id succeeds or fails on its own. Pause, resume, delete and clone apply to Core-managed monitors only.
// @Tags         monitors
// @Accept       json
// @Produce      json
// @ID           bulkMonitorAction
// @Param        request  body      BulkMonitorRequest  true  "Bulk action"
// @Success      200      {object}  utils.APIResponse{data=object{results=[]BulkMonitorResult,succeeded=int,failed=int}}
// @Failure      400      {object}  utils.APIResponse
// @Router       /v1/monitors/bulk [post]
func (s *Server) bulkMonitorAction(c *gin.Context) {
	var req BulkMonitorRequest
	if err := c.ShouldBindJSON(&req); err != nil || len(req.IDs) == 0 || len(req.IDs) > maxBulkIncidents {
		utils.BadRequest(c, "ids is required (at most 100)")
		return
	}
	var window service.MaintenanceWindow
	switch req.Action {
	case "pause", "resume", "delete", "clone":
	case "maintenance":
		if req.Maintenance == nil {
			utils.BadRequest(c, "maintenance settings are required")
			return
		}
		w, err := req.Maintenance.window(time.Now().UTC())
		if err != nil {
			utils.BadRequest(c, err.Error())
			return
		}
		window = w
	default:
		utils.BadRequest(c, "action must be pause, resume, maintenance, delete or clone")
		return
	}
	keep := req.KeepHistory == nil || *req.KeepHistory
	actor := frontendActorID(c)
	results := make([]BulkMonitorResult, 0, len(req.IDs))
	succeeded := 0
	seen := map[string]bool{}
	for _, id := range req.IDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		result := BulkMonitorResult{ID: id}
		var err error
		switch req.Action {
		case "pause":
			_, err = s.coreMonitorManagementService.PauseCoreMonitorBy(id, actor)
		case "resume":
			_, err = s.coreMonitorManagementService.ResumeCoreMonitor(id)
		case "delete":
			err = s.coreMonitorManagementService.DeleteCoreMonitorWith(id, keep)
		case "clone":
			var record *service.CoreManagedMonitorRecord
			if record, err = s.coreMonitorManagementService.CloneCoreMonitor(id); err == nil {
				result.NewID = record.Monitor.ID
			}
		case "maintenance":
			err = s.monitorService.SetMonitorMaintenance(id, window)
		}
		if err != nil {
			result.Error = bulkMonitorError(err, req.Action)
		} else {
			result.OK = true
			succeeded++
		}
		results = append(results, result)
	}
	utils.SuccessResponse(c, http.StatusOK, "Bulk action finished", gin.H{"results": results, "succeeded": succeeded, "failed": len(results) - succeeded})
}

func bulkMonitorError(err error, action string) string {
	switch {
	case errors.Is(err, service.ErrCoreManagedMonitorNotFound) && action != "maintenance":
		return "Only Core-managed monitors can be " + map[string]string{"pause": "paused", "resume": "resumed", "delete": "deleted", "clone": "cloned"}[action] + " here"
	case errors.Is(err, service.ErrMonitorNotFound):
		return "Monitor not found"
	case errors.Is(err, service.ErrIncidentInvalid):
		return strings.TrimPrefix(err.Error(), service.ErrIncidentInvalid.Error()+": ")
	default:
		return fmt.Sprintf("%s failed", action)
	}
}
