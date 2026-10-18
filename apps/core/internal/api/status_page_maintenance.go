package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const maxMaintenanceDuration = 30 * 24 * time.Hour

type StatusPageMaintenanceRequest struct {
	Title             *string    `json:"title"`
	Description       *string    `json:"description"`
	StartsAt          *time.Time `json:"starts_at"`
	EndsAt            *time.Time `json:"ends_at"`
	ComponentIDs      []string   `json:"component_ids"`
	HoldAlerts        *bool      `json:"hold_alerts"`
	NotifySubscribers *bool      `json:"notify_subscribers"`
}

type StatusPageMaintenanceResponse struct {
	ID                string    `json:"id"`
	StatusPageID      string    `json:"status_page_id"`
	Title             string    `json:"title"`
	Description       string    `json:"description,omitempty"`
	StartsAt          time.Time `json:"starts_at"`
	EndsAt            time.Time `json:"ends_at"`
	ComponentIDs      []string  `json:"component_ids"`
	ComponentNames    []string  `json:"component_names"`
	HoldAlerts        bool      `json:"hold_alerts"`
	NotifySubscribers bool      `json:"notify_subscribers"`
	NoticeSent        bool      `json:"notice_sent"`
	// Status is scheduled, in_progress or completed.
	Status    string    `json:"status" enums:"scheduled,in_progress,completed"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func maintenanceStatus(w db.StatusPageMaintenance, now time.Time) string {
	switch {
	case now.Before(w.StartsAt):
		return "scheduled"
	case now.Before(w.EndsAt):
		return "in_progress"
	default:
		return "completed"
	}
}

func decodeComponentIDs(raw string) []string {
	var ids []string
	if json.Unmarshal([]byte(raw), &ids) != nil || ids == nil {
		return []string{}
	}
	return ids
}

func (s *Server) maintenanceResponse(w db.StatusPageMaintenance, names map[string]string) StatusPageMaintenanceResponse {
	ids := decodeComponentIDs(w.ComponentIDs)
	componentNames := make([]string, 0, len(ids))
	for _, id := range ids {
		if n := names[id]; n != "" {
			componentNames = append(componentNames, n)
		}
	}
	return StatusPageMaintenanceResponse{ID: w.ID, StatusPageID: w.StatusPageID, Title: w.Title, Description: w.Description, StartsAt: w.StartsAt, EndsAt: w.EndsAt, ComponentIDs: ids, ComponentNames: componentNames,
		HoldAlerts: w.HoldAlerts, NotifySubscribers: w.NotifySubscribers, NoticeSent: w.NoticeSentAt != nil, Status: maintenanceStatus(w, time.Now().UTC()), CreatedAt: w.CreatedAt, UpdatedAt: w.UpdatedAt}
}

func (s *Server) statusPageComponentNames(pageID string) map[string]string {
	var comps []db.StatusPageComponent
	s.db.Select("id", "public_name").Where("status_page_id = ?", pageID).Find(&comps)
	out := make(map[string]string, len(comps))
	for _, c := range comps {
		out[c.ID] = c.PublicName
	}
	return out
}

// applyMaintenanceRequest validates the request onto w; create requires every field.
func (s *Server) applyMaintenanceRequest(w *db.StatusPageMaintenance, req StatusPageMaintenanceRequest, create bool) error {
	if req.Title != nil {
		w.Title = strings.TrimSpace(*req.Title)
	}
	if req.Description != nil {
		w.Description = strings.TrimSpace(*req.Description)
	}
	if req.StartsAt != nil {
		w.StartsAt = req.StartsAt.UTC()
	}
	if req.EndsAt != nil {
		w.EndsAt = req.EndsAt.UTC()
	}
	if req.HoldAlerts != nil {
		w.HoldAlerts = *req.HoldAlerts
	}
	if req.NotifySubscribers != nil {
		w.NotifySubscribers = *req.NotifySubscribers
	}
	if req.ComponentIDs != nil {
		names := s.statusPageComponentNames(w.StatusPageID)
		seen := map[string]bool{}
		ids := make([]string, 0, len(req.ComponentIDs))
		for _, id := range req.ComponentIDs {
			if names[id] == "" {
				return &requestValidationError{message: "component_ids must belong to this status page"}
			}
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
		body, _ := json.Marshal(ids)
		w.ComponentIDs = string(body)
	}
	switch {
	case w.Title == "" || len([]rune(w.Title)) > 120:
		return &requestValidationError{message: "title is required (120 characters at most)"}
	case len([]rune(w.Description)) > 2000:
		return &requestValidationError{message: "description must be 2000 characters or fewer"}
	case len(decodeComponentIDs(w.ComponentIDs)) == 0:
		return &requestValidationError{message: "choose at least one affected component"}
	case w.StartsAt.IsZero() || w.EndsAt.IsZero():
		return &requestValidationError{message: "starts_at and ends_at are required"}
	case !w.EndsAt.After(w.StartsAt):
		return &requestValidationError{message: "ends_at must be after starts_at"}
	case w.EndsAt.Sub(w.StartsAt) > maxMaintenanceDuration:
		return &requestValidationError{message: "a maintenance window can last 30 days at most"}
	case create && !w.EndsAt.After(time.Now().UTC()):
		return &requestValidationError{message: "ends_at must be in the future"}
	}
	return nil
}

// listStatusPageMaintenance lists a page's maintenance windows.
// @Summary      List status page maintenance windows
// @Tags         status-pages
// @Produce      json
// @Security     BearerAuth
// @ID           listStatusPageMaintenance
// @Param        id   path      string  true  "Status page ID"
// @Success      200  {object}  utils.APIResponse{data=object{maintenance=[]StatusPageMaintenanceResponse,count=int}}
// @Failure      404  {object}  utils.APIResponse
// @Router       /v1/status-pages/{id}/maintenance [get]
func (s *Server) listStatusPageMaintenance(c *gin.Context) {
	pageID := c.Param("id")
	if !s.statusPageExists(c, pageID) {
		return
	}
	var rows []db.StatusPageMaintenance
	if err := s.db.Where("status_page_id = ?", pageID).Order("starts_at DESC").Find(&rows).Error; err != nil {
		utils.InternalError(c, "Failed to list maintenance windows", err)
		return
	}
	names := s.statusPageComponentNames(pageID)
	out := make([]StatusPageMaintenanceResponse, 0, len(rows))
	for _, r := range rows {
		out = append(out, s.maintenanceResponse(r, names))
	}
	sort.SliceStable(out, func(i, j int) bool { return maintenanceRank(out[i].Status) < maintenanceRank(out[j].Status) })
	utils.SuccessResponse(c, http.StatusOK, "Maintenance windows retrieved", gin.H{"maintenance": out, "count": len(out)})
}

func maintenanceRank(status string) int {
	switch status {
	case "in_progress":
		return 0
	case "scheduled":
		return 1
	default:
		return 2
	}
}

// createStatusPageMaintenance schedules a maintenance window.
// @Summary      Schedule status page maintenance
// @Tags         status-pages
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           createStatusPageMaintenance
// @Param        id       path      string                        true  "Status page ID"
// @Param        request  body      StatusPageMaintenanceRequest  true  "Maintenance window"
// @Success      201      {object}  utils.APIResponse{data=object{window=StatusPageMaintenanceResponse}}
// @Failure      400      {object}  utils.APIResponse
// @Failure      404      {object}  utils.APIResponse
// @Router       /v1/status-pages/{id}/maintenance [post]
func (s *Server) createStatusPageMaintenance(c *gin.Context) {
	pageID := c.Param("id")
	if !s.statusPageExists(c, pageID) {
		return
	}
	var req StatusPageMaintenanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid maintenance payload")
		return
	}
	w := db.StatusPageMaintenance{ID: utils.GenerateID("status_page_maintenance"), StatusPageID: pageID, ComponentIDs: "[]"}
	if err := s.applyMaintenanceRequest(&w, req, true); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if err := s.db.Create(&w).Error; err != nil {
		utils.InternalError(c, "Failed to schedule maintenance", err)
		return
	}
	utils.SuccessResponse(c, http.StatusCreated, "Maintenance scheduled", gin.H{"window": s.maintenanceResponse(w, s.statusPageComponentNames(pageID))})
}

func (s *Server) loadMaintenance(c *gin.Context) (db.StatusPageMaintenance, bool) {
	var w db.StatusPageMaintenance
	err := s.db.Where("id = ? AND status_page_id = ?", c.Param("window_id"), c.Param("id")).First(&w).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		utils.NotFound(c, "Maintenance window not found")
		return w, false
	}
	if err != nil {
		utils.InternalError(c, "Failed to load maintenance window", err)
		return w, false
	}
	return w, true
}

// updateStatusPageMaintenance edits a maintenance window.
// @Summary      Update status page maintenance
// @Tags         status-pages
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           updateStatusPageMaintenance
// @Param        id         path      string                        true  "Status page ID"
// @Param        window_id  path      string                        true  "Window ID"
// @Param        request    body      StatusPageMaintenanceRequest  true  "Fields to change"
// @Success      200        {object}  utils.APIResponse{data=object{window=StatusPageMaintenanceResponse}}
// @Failure      400        {object}  utils.APIResponse
// @Failure      404        {object}  utils.APIResponse
// @Router       /v1/status-pages/{id}/maintenance/{window_id} [patch]
func (s *Server) updateStatusPageMaintenance(c *gin.Context) {
	w, ok := s.loadMaintenance(c)
	if !ok {
		return
	}
	var req StatusPageMaintenanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid maintenance payload")
		return
	}
	startsBefore := w.StartsAt
	if err := s.applyMaintenanceRequest(&w, req, false); err != nil {
		utils.BadRequest(c, err.Error())
		return
	}
	if !w.StartsAt.Equal(startsBefore) {
		w.NoticeSentAt = nil // a moved window earns a fresh notice
	}
	if err := s.db.Save(&w).Error; err != nil {
		utils.InternalError(c, "Failed to update maintenance", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Maintenance updated", gin.H{"window": s.maintenanceResponse(w, s.statusPageComponentNames(w.StatusPageID))})
}

// deleteStatusPageMaintenance cancels a maintenance window.
// @Summary      Delete status page maintenance
// @Tags         status-pages
// @Produce      json
// @Security     BearerAuth
// @ID           deleteStatusPageMaintenance
// @Param        id         path      string  true  "Status page ID"
// @Param        window_id  path      string  true  "Window ID"
// @Success      200        {object}  utils.APIResponse
// @Failure      404        {object}  utils.APIResponse
// @Router       /v1/status-pages/{id}/maintenance/{window_id} [delete]
func (s *Server) deleteStatusPageMaintenance(c *gin.Context) {
	w, ok := s.loadMaintenance(c)
	if !ok {
		return
	}
	if err := s.db.Delete(&w).Error; err != nil {
		utils.InternalError(c, "Failed to delete maintenance", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Maintenance deleted", gin.H{})
}
