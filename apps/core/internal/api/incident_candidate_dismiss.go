package api

import (
	"errors"
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type dismissIncidentCandidateRequest struct {
	Reason string `json:"reason,omitempty"`
}

// dismissIncidentCandidate hides a candidate until its monitor reports healthy again.
// @Summary      Dismiss an incident candidate
// @Description  Hide a failing monitor from the incident candidates list. The candidate id is the monitor id. It reappears if the monitor recovers and fails again.
// @Tags         incidents
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           dismissIncidentCandidate
// @Param        id       path      string                           true   "Candidate (monitor) ID"
// @Param        request  body      dismissIncidentCandidateRequest  false  "Optional reason"
// @Success      200      {object}  utils.APIResponse{data=object{monitor_id=string,dismissed_at=string}}
// @Failure      401      {object}  utils.APIResponse
// @Failure      404      {object}  utils.APIResponse
// @Failure      500      {object}  utils.APIResponse
// @Router       /v1/incidents/candidates/{id}/dismiss [post]
func (s *Server) dismissIncidentCandidate(c *gin.Context) {
	monitorID := c.Param("id")
	var monitor db.Monitor
	if err := s.db.Select("id").Where("id = ?", monitorID).First(&monitor).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			utils.NotFound(c, "Incident candidate not found")
			return
		}
		utils.InternalError(c, "Failed to dismiss incident candidate", err)
		return
	}

	var req dismissIncidentCandidateRequest
	_ = c.ShouldBindJSON(&req) // body is optional

	dismissal := db.IncidentCandidateDismissal{
		MonitorID:   monitorID,
		DismissedBy: frontendActorID(c),
		Reason:      strings.TrimSpace(req.Reason),
		DismissedAt: time.Now().UTC(),
	}
	if err := s.db.Clauses(clause.OnConflict{UpdateAll: true}).Create(&dismissal).Error; err != nil {
		utils.InternalError(c, "Failed to dismiss incident candidate", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Incident candidate dismissed", gin.H{"monitor_id": monitorID, "dismissed_at": dismissal.DismissedAt})
}

// activeCandidateDismissals returns monitor ids whose dismissal still applies:
// no healthy report has arrived since the dismissal.
func (s *Server) activeCandidateDismissals() (map[string]struct{}, error) {
	var dismissals []db.IncidentCandidateDismissal
	if err := s.db.Find(&dismissals).Error; err != nil {
		return nil, err
	}
	active := make(map[string]struct{}, len(dismissals))
	for _, dismissal := range dismissals {
		var recovered int64
		if err := s.db.Model(&db.MonitorReport{}).
			Where("monitor_id = ? AND health = ? AND created_at > ?", dismissal.MonitorID, "up", dismissal.DismissedAt).
			Count(&recovered).Error; err != nil {
			return nil, err
		}
		if recovered == 0 {
			active[dismissal.MonitorID] = struct{}{}
		}
	}
	return active, nil
}

func filterDismissedCandidates(candidates []gin.H, dismissed map[string]struct{}) []gin.H {
	kept := make([]gin.H, 0, len(candidates))
	for _, candidate := range candidates {
		if id, _ := candidate["monitor_id"].(string); id != "" {
			if _, hidden := dismissed[id]; hidden {
				continue
			}
		}
		kept = append(kept, candidate)
	}
	return kept
}
