package api

import (
	"errors"
	"net/http"
	"orion/core/internal/service"
	"orion/core/internal/utils"
	"time"

	"github.com/gin-gonic/gin"
)

type AgentMonitorDefinitionRequest struct {
	Name string `json:"name" binding:"required"`
	// CheckType: disk (params path, threshold_percent), memory (threshold_percent), process (process_name), port (port), systemd (unit), docker (container), pm2 (app).
	CheckType       string         `json:"check_type" binding:"required" enums:"disk,memory,process,port,systemd,docker,pm2"`
	Params          map[string]any `json:"params,omitempty"`
	IntervalSeconds int            `json:"interval_seconds,omitempty"`
	FailAfter       int            `json:"fail_after,omitempty"`
}

type AgentMonitorDefinitionResponse struct {
	ID              string         `json:"id"`
	Name            string         `json:"name"`
	CheckType       string         `json:"check_type"`
	Params          map[string]any `json:"params"`
	IntervalSeconds int            `json:"interval_seconds"`
	FailAfter       int            `json:"fail_after"`
	CreatedAt       time.Time      `json:"created_at"`
	// State is waiting until the agent has fetched the definition and reported under this name, then reporting.
	State string `json:"state" enums:"waiting,failed,reporting"`
	// FailureReason is why the agent could not run this check, in its own words (state failed).
	FailureReason string `json:"failure_reason,omitempty"`
	// DeliveredAt is when the agent first fetched this definition (Requested becomes Applied).
	DeliveredAt *time.Time `json:"delivered_at,omitempty"`
	MonitorID   string     `json:"monitor_id,omitempty"`
	FirstReport *time.Time `json:"first_report_at,omitempty"`
}

func definitionResponse(v service.AgentMonitorDefinitionView) AgentMonitorDefinitionResponse {
	return AgentMonitorDefinitionResponse{ID: v.ID, Name: v.Name, CheckType: v.CheckType, Params: v.ParamsParsed, IntervalSeconds: v.IntervalSeconds, FailAfter: v.FailAfter,
		CreatedAt: v.CreatedAt, State: v.State, FailureReason: v.FailureReason, DeliveredAt: v.DeliveredAt, MonitorID: v.MonitorID, FirstReport: v.FirstReport}
}

// createAgentMonitorDefinition configures a check for a server's agent to run.
// @Summary      Create an agent monitor definition
// @Description  Defines a disk, memory, process or port check. The agent fetches its definitions with POST /v1/agents/{agent_id}/monitor-definitions/sync and reports under the same name; state turns reporting after the first report.
// @Tags         agents
// @Accept       json
// @Produce      json
// @ID           createAgentMonitorDefinition
// @Param        agent_id path      string                         true  "Agent ID"
// @Param        request  body      AgentMonitorDefinitionRequest  true  "Check"
// @Success      201      {object}  utils.APIResponse{data=object{definition=AgentMonitorDefinitionResponse}}
// @Failure      400      {object}  utils.APIResponse
// @Failure      404      {object}  utils.APIResponse
// @Failure      409      {object}  utils.APIResponse
// @Router       /v1/agents/{agent_id}/monitor-definitions [post]
func (s *Server) createAgentMonitorDefinition(c *gin.Context) {
	var req AgentMonitorDefinitionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "name and check_type are required")
		return
	}
	def, err := s.agentService.CreateMonitorDefinition(agentIDParam(c), service.AgentMonitorDefinitionInput{Name: req.Name, CheckType: req.CheckType, Params: req.Params, IntervalSeconds: req.IntervalSeconds, FailAfter: req.FailAfter}, frontendActorID(c))
	switch {
	case err == nil:
		views, _ := s.agentService.ListMonitorDefinitions(def.AgentID)
		for _, v := range views {
			if v.ID == def.ID {
				utils.SuccessResponse(c, http.StatusCreated, "Monitor definition created", gin.H{"definition": definitionResponse(v)})
				return
			}
		}
		utils.SuccessResponse(c, http.StatusCreated, "Monitor definition created", gin.H{})
	case errors.Is(err, service.ErrAgentNotFound):
		utils.NotFound(c, "Agent not found")
	case errors.Is(err, service.ErrDefinitionExists):
		utils.ErrorResponse(c, http.StatusConflict, err.Error(), nil)
	case errors.Is(err, service.ErrDefinitionInvalid):
		utils.BadRequest(c, err.Error())
	default:
		utils.InternalError(c, "Failed to create the monitor definition", err)
	}
}

// listAgentMonitorDefinitions lists a server's configured agent checks with their live state.
// @Summary      List agent monitor definitions
// @Tags         agents
// @Produce      json
// @ID           listAgentMonitorDefinitions
// @Param        id   path      string  true  "Agent ID"
// @Success      200  {object}  utils.APIResponse{data=object{definitions=[]AgentMonitorDefinitionResponse}}
// @Failure      404  {object}  utils.APIResponse
// @Router       /v1/agents/{id}/monitor-definitions [get]
func (s *Server) listAgentMonitorDefinitions(c *gin.Context) {
	if _, err := s.agentService.GetAgent(c.Param("id")); err != nil {
		utils.NotFound(c, "Agent not found")
		return
	}
	views, err := s.agentService.ListMonitorDefinitions(c.Param("id"))
	if err != nil {
		utils.InternalError(c, "Failed to list monitor definitions", err)
		return
	}
	out := make([]AgentMonitorDefinitionResponse, 0, len(views))
	for _, v := range views {
		out = append(out, definitionResponse(v))
	}
	utils.SuccessResponse(c, http.StatusOK, "Monitor definitions retrieved", gin.H{"definitions": out})
}

// deleteAgentMonitorDefinition removes a configured check.
// @Summary      Delete an agent monitor definition
// @Tags         agents
// @Produce      json
// @ID           deleteAgentMonitorDefinition
// @Param        id             path      string  true  "Agent ID"
// @Param        definition_id  path      string  true  "Definition ID"
// @Success      200            {object}  utils.APIResponse
// @Failure      404            {object}  utils.APIResponse
// @Router       /v1/agents/{id}/monitor-definitions/{definition_id} [delete]
func (s *Server) deleteAgentMonitorDefinition(c *gin.Context) {
	if err := s.agentService.DeleteMonitorDefinition(c.Param("id"), c.Param("definition_id")); err != nil {
		if errors.Is(err, service.ErrDefinitionNotFound) {
			utils.NotFound(c, "Monitor definition not found")
			return
		}
		utils.InternalError(c, "Failed to delete the monitor definition", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Monitor definition deleted", gin.H{})
}

// AgentPulledDefinition is what the agent runs.
type AgentPulledDefinition struct {
	ID              string         `json:"id"`
	Name            string         `json:"name"`
	CheckType       string         `json:"check_type"`
	Params          map[string]any `json:"params"`
	IntervalSeconds int            `json:"interval_seconds"`
	FailAfter       int            `json:"fail_after"`
}

// pullAgentMonitorDefinitions is called by the agent with its token.
// @Summary      Agent: sync configured monitor definitions
// @Description  Agent-to-Core. Returns the checks configured for this agent in the Console; the agent registers each as a monitor with the same name and reports to it.
// @Tags         agents
// @Produce      json
// @ID           pullAgentMonitorDefinitions
// @Param        agent_id  path      string  true  "Agent ID"
// @Success      200       {object}  utils.APIResponse{data=object{definitions=[]AgentPulledDefinition}}
// @Failure      401       {object}  utils.APIResponse
// @Router       /v1/agents/{agent_id}/monitor-definitions/sync [post]
func (s *Server) pullAgentMonitorDefinitions(c *gin.Context) {
	s.agentService.MarkDefinitionsDelivered(c.Param("agent_id"))
	views, err := s.agentService.ListMonitorDefinitions(c.Param("agent_id"))
	if err != nil {
		utils.InternalError(c, "Failed to load monitor definitions", err)
		return
	}
	out := make([]AgentPulledDefinition, 0, len(views))
	for _, v := range views {
		out = append(out, AgentPulledDefinition{ID: v.ID, Name: v.Name, CheckType: v.CheckType, Params: v.ParamsParsed, IntervalSeconds: v.IntervalSeconds, FailAfter: v.FailAfter})
	}
	utils.SuccessResponse(c, http.StatusOK, "Monitor definitions retrieved", gin.H{"definitions": out})
}
