package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"orion/core/internal/utils"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// monitorFailureContext explains a failing monitor: when the current run of non-up reports
// began and the most recent error text.
func (s *Server) monitorFailureContext(monitorID string) (since *time.Time, detail string) {
	var reports []db.MonitorReport
	if err := s.db.Where("monitor_id = ?", monitorID).Order("created_at DESC").Limit(60).Find(&reports).Error; err != nil {
		return nil, ""
	}
	failing := 0
	for _, r := range reports {
		if r.Health == "up" {
			break
		}
		failing++
		t := r.CreatedAt
		since = &t
		if detail == "" {
			var payload map[string]any
			if json.Unmarshal([]byte(r.Payload), &payload) == nil {
				if msg, ok := payload["error"].(string); ok {
					detail = strings.TrimSpace(msg)
				}
			}
		}
	}
	if detail == "" && failing > 0 {
		detail = fmt.Sprintf("%d failed checks in a row", failing)
	}
	if len(detail) > 200 {
		detail = detail[:200]
	}
	return since, detail
}

func (s *Server) activeIncidentFor(monitorID string) (id string, display string) {
	var incident db.Incident
	if s.db.Select("id", "number").Where("monitor_id = ? AND status IN ?", monitorID, []string{"open", "acknowledged", "covered"}).Limit(1).Find(&incident).RowsAffected > 0 {
		return incident.ID, incidentDisplayID(incident.Number)
	}
	return "", ""
}

// enrichHealthIssues adds kind, detail, since and the linked incident to monitor issues and
// appends server issues (an agent that stopped reporting), worst first.
func (s *Server) enrichHealthIssues(issues []gin.H) []gin.H {
	for _, issue := range issues {
		id, _ := issue["monitor_id"].(string)
		issue["kind"] = "monitor"
		issue["target_id"] = id
		issue["target_name"] = issue["monitor_name"]
		since, detail := s.monitorFailureContext(id)
		if issue["health"] == "stale" {
			var monitor db.Monitor
			if s.db.Select("last_successful_report_at").Where("id = ?", id).Limit(1).Find(&monitor).RowsAffected > 0 && monitor.LastSuccessfulReportAt != nil {
				since = monitor.LastSuccessfulReportAt
				detail = "last report " + monitor.LastSuccessfulReportAt.UTC().Format("15:04 UTC")
			}
		}
		issue["detail"] = detail
		issue["since"] = since
		incidentID, display := s.activeIncidentFor(id)
		issue["incident_id"] = incidentID
		issue["incident_display_id"] = display
	}
	var agents []db.Agent
	s.db.Where("system = ? AND (deleted_at IS NULL OR deleted_at = ?)", false, time.Time{}).Find(&agents)
	healthService := service.NewHealthService(s.db, s.logger)
	for _, agent := range agents {
		snapshot, err := healthService.ComputeAgentHealthSnapshot(agent.ID, service.DefaultHealthConfig())
		if err != nil || snapshot.AgentHealth == "up" || snapshot.AgentHealth == "unknown" || agent.InMaintenance(time.Now().UTC()) {
			continue
		}
		last := agent.LastSeen
		issues = append(issues, gin.H{"kind": "server", "target_id": agent.ID, "target_name": agent.Name, "health": snapshot.AgentHealth, "agent_id": agent.ID, "agent_name": agent.Name,
			"detail": firstNonEmptyString(snapshot.Reason, "last report "+last.UTC().Format("15:04 UTC")), "since": &last, "incident_id": "", "incident_display_id": ""})
	}
	rank := map[string]int{"down": 0, "degraded": 1, "stale": 2}
	sort.SliceStable(issues, func(i, j int) bool {
		a, _ := issues[i]["health"].(string)
		b, _ := issues[j]["health"].(string)
		return rank[a] < rank[b]
	})
	return issues
}

func firstNonEmptyString(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// enrichCandidates adds the detail line and when the signal was first seen.
func (s *Server) enrichCandidates(candidates []gin.H) []gin.H {
	for _, candidate := range candidates {
		id, _ := candidate["monitor_id"].(string)
		since, detail := s.monitorFailureContext(id)
		if candidate["health"] == "stale" {
			var monitor db.Monitor
			if s.db.Select("last_successful_report_at").Where("id = ?", id).Limit(1).Find(&monitor).RowsAffected > 0 && monitor.LastSuccessfulReportAt != nil {
				since, detail = monitor.LastSuccessfulReportAt, "No report since "+monitor.LastSuccessfulReportAt.UTC().Format("15:04 UTC")
			}
		}
		candidate["detail"] = detail
		candidate["seen_at"] = since
	}
	return candidates
}

// openIncidentCandidate turns a candidate into a real incident, notifying like a detected one.
// @Summary      Open an incident from a candidate
// @Description  The candidate id is the monitor id. Severity and title come from the candidate; alerts are queued. Fails with 409 if the monitor already has an active incident.
// @Tags         incidents
// @Produce      json
// @ID           openIncidentCandidate
// @Param        id   path      string  true  "Candidate (monitor) ID"
// @Success      201  {object}  utils.APIResponse{data=object{incident=IncidentResponse}}
// @Failure      404  {object}  utils.APIResponse
// @Failure      409  {object}  utils.APIResponse
// @Router       /v1/incidents/candidates/{id}/open [post]
func (s *Server) openIncidentCandidate(c *gin.Context) {
	var monitor db.Monitor
	if err := s.db.Where("id = ?", c.Param("id")).First(&monitor).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			utils.NotFound(c, "Incident candidate not found")
			return
		}
		utils.InternalError(c, "Failed to open incident", err)
		return
	}
	severity := "medium"
	var recent []db.MonitorReport
	s.db.Where("monitor_id = ?", monitor.ID).Order("created_at DESC").Limit(5).Find(&recent)
	down := 0
	for _, r := range recent {
		if r.Health == "down" {
			down++
		}
	}
	if down >= 3 || monitor.ComputedHealth == "stale" {
		severity = "high"
	}
	_, detail := s.monitorFailureContext(monitor.ID)
	title := monitor.Name + " is " + firstNonEmptyString(monitor.ComputedHealth, "failing")
	incident, err := service.NewIncidentService(s.db, s.logger, s.cfg).DeclareIncident(service.DeclareIncidentInput{Title: title, Severity: severity, MonitorID: monitor.ID, Summary: detail, Notify: true}, s.incidentLifecycleActionMetadata(c, ""))
	switch {
	case err == nil:
		var agent db.Agent
		s.db.Where("id = ?", incident.AgentID).First(&agent)
		utils.SuccessResponse(c, http.StatusCreated, "Incident opened", gin.H{"incident": incidentResponse(incident, agent, monitor)})
	case errors.Is(err, service.ErrIncidentAlreadyActive):
		utils.ErrorResponse(c, http.StatusConflict, err.Error(), nil)
	default:
		utils.InternalError(c, "Failed to open incident", err)
	}
}
