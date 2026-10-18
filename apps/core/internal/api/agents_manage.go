package api

import (
	"errors"
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"orion/core/internal/utils"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func applyAgentMetrics(r *AgentResponse, m service.AgentMetrics) {
	r.CPUPercent, r.MemoryPercent, r.DiskPercent = m.CPUPercent, m.MemoryPercent, m.DiskPercent
	if m.CPUSeries != nil {
		r.CPUSeries = m.CPUSeries
	}
	r.AgentVersion, r.LastReportAt = m.AgentVersion, m.LastReportAt
}

type UpdateAgentRequest struct {
	Name   *string   `json:"name,omitempty"`
	Labels *[]string `json:"labels,omitempty" example:"env=prod"`
	Notes  *string   `json:"notes,omitempty"`
}

// updateAgent edits a server's name, labels and notes.
// @Summary      Update server details
// @Description  Change name, labels (key=value, at most 20) and notes. Omitted fields stay as they are.
// @Tags         agents
// @Accept       json
// @Produce      json
// @ID           updateAgent
// @Param        id       path      string              true  "Agent ID"
// @Param        request  body      UpdateAgentRequest  true  "Fields to change"
// @Success      200      {object}  utils.APIResponse{data=object{agent=AgentResponse}}
// @Failure      400      {object}  utils.APIResponse
// @Failure      404      {object}  utils.APIResponse
// @Router       /v1/agents/{id} [patch]
func (s *Server) updateAgent(c *gin.Context) {
	var request UpdateAgentRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		utils.BadRequest(c, "Invalid server payload")
		return
	}
	agent, err := s.agentService.UpdateAgentDetails(c.Param("id"), service.AgentDetailsInput{Name: request.Name, Labels: request.Labels, Notes: request.Notes})
	switch {
	case err == nil:
		utils.SuccessResponse(c, http.StatusOK, "Server updated", gin.H{"agent": s.agentDetailResponse(*agent)})
	case errors.Is(err, service.ErrAgentNotFound):
		utils.NotFound(c, "Agent not found")
	case errors.Is(err, service.ErrAgentInvalid):
		utils.BadRequest(c, err.Error())
	default:
		s.logger.Error("Failed to update agent", "error", err)
		utils.InternalError(c, "Failed to update server", err)
	}
}

// decommissionAgent removes a server.
// @Summary      Remove (decommission) a server
// @Description  Revokes the token, deletes the server's monitors, closes its open incidents and hides it. keep_history=true (default) keeps reports for 90 days then purges them; false deletes everything now. The same host can be added again.
// @Tags         agents
// @Produce      json
// @ID           decommissionAgent
// @Param        id            path      string  true   "Agent ID"
// @Param        keep_history  query     bool    false  "Keep reports for 90 days (default true)"
// @Success      200           {object}  utils.APIResponse{data=object{result=service.DecommissionResult}}
// @Failure      400           {object}  utils.APIResponse
// @Failure      404           {object}  utils.APIResponse
// @Router       /v1/agents/{id} [delete]
func (s *Server) decommissionAgent(c *gin.Context) {
	keep := utils.QueryBool(c, "keep_history", true)
	actorType, actorID := incidentLifecycleActor(c)
	result, err := s.agentService.DecommissionAgent(c.Param("id"), keep, service.AgentTokenActionInput{ActorType: actorType, ActorID: actorID, Reason: "decommissioned", RequestID: requestIDFromContext(c)})
	switch {
	case err == nil:
		utils.SuccessResponse(c, http.StatusOK, "Server removed", gin.H{"result": result})
	case errors.Is(err, service.ErrAgentNotFound):
		utils.NotFound(c, "Agent not found")
	case errors.Is(err, service.ErrAgentInvalid):
		utils.BadRequest(c, err.Error())
	default:
		s.logger.Error("Failed to decommission agent", "error", err)
		utils.InternalError(c, "Failed to remove server", err)
	}
}

// agentDetailResponse is the full single-server response: details plus latest metrics and status.
func (s *Server) agentDetailResponse(agent db.Agent) AgentResponse {
	r := agentResponse(agent)
	m := s.agentService.LatestAgentMetrics(agent.ID)
	applyAgentMetrics(&r, m)
	if m.UptimeSeconds != nil {
		r.UptimeSeconds = m.UptimeSeconds
	}
	if m.IP != "" {
		ip := m.IP
		r.IP = &ip
	}
	if snapshot, err := service.NewHealthService(s.db, s.logger).ComputeAgentHealthSnapshot(agent.ID, service.DefaultHealthConfig()); err == nil {
		r.Status, r.AvailabilityHealth, r.MonitorHealth, r.StatusReason = snapshot.OverallHealth, snapshot.AgentHealth, snapshot.MonitorHealth, snapshot.Reason
		r.StatusText = service.AgentStatusText(snapshot.OverallHealth, agent.LastSeen, m, time.Now().UTC())
		r.StatusDetail = service.AgentStatusDetail(snapshot.OverallHealth, m, snapshot.Reason)
	}
	return r
}

// parseStatusSet reads a comma-separated status filter.
func parseStatusSet(raw string) map[string]bool {
	set := map[string]bool{}
	for _, part := range strings.Split(raw, ",") {
		if part = strings.ToLower(strings.TrimSpace(part)); part != "" {
			set[part] = true
		}
	}
	return set
}

// monitorStatusText renders "Down 30m", "Degraded 4m" or "No report 12m".
func (s *Server) monitorStatusLine(m db.Monitor, health string) (text string, detail string) {
	since, detail := s.monitorFailureContext(m.ID)
	switch health {
	case "up":
		return "Up", ""
	case "stale":
		if m.LastSuccessfulReportAt != nil {
			return "No report " + service.CompactDuration(time.Since(*m.LastSuccessfulReportAt)), "last report " + m.LastSuccessfulReportAt.UTC().Format("15:04 UTC")
		}
		return "No report", detail
	case "down", "degraded":
		label := strings.ToUpper(health[:1]) + health[1:]
		if since != nil {
			return label + " " + service.CompactDuration(time.Since(*since)), detail
		}
		return label, detail
	default:
		return "Unknown", detail
	}
}

// effectiveMonitorHealth is the computed health, falling back to the stored one.
func effectiveMonitorHealth(m db.Monitor) string {
	if m.ComputedHealth != "" {
		return m.ComputedHealth
	}
	if m.Health != "" {
		return m.Health
	}
	return "unknown"
}
