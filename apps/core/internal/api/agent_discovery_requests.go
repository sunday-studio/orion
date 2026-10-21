package api

import (
	"errors"
	"net/http"
	"orion/core/internal/service"
	"orion/core/internal/utils"

	"github.com/gin-gonic/gin"
)

// createAgentDiscoveryRequest queues a request for the agent.
// @Summary      Ask the agent to do something about discovery
// @Description  The Console never changes the host. A request is queued here and the agent applies it at its next check-in (about every minute), then reports the outcome. Types: rescan, dismiss, restore, ignore, unignore (payload.ids), set_scanning (payload.enabled), set_kind (payload.kind, payload.enabled), track_storage (payload.paths). An agent that has not said it can take requests gets the request stored as unsupported. Requests expire after 24 hours.
// @Tags         agents
// @Accept       json
// @Produce      json
// @ID           createAgentDiscoveryRequest
// @Param        agent_id path      string                                 true  "Agent ID"
// @Param        request  body      service.AgentDiscoveryRequestInput     true  "Request"
// @Success      201      {object}  utils.APIResponse{data=object{request=service.AgentDiscoveryRequestView}}
// @Failure      400      {object}  utils.APIResponse
// @Failure      404      {object}  utils.APIResponse
// @Router       /v1/agents/{agent_id}/discovery/requests [post]
func (s *Server) createAgentDiscoveryRequest(c *gin.Context) {
	var in service.AgentDiscoveryRequestInput
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "type is required")
		return
	}
	v, err := s.agentService.CreateDiscoveryRequest(c.Param("agent_id"), in, frontendActorID(c))
	switch {
	case err == nil:
		utils.SuccessResponse(c, http.StatusCreated, "Request queued", gin.H{"request": v})
	case errors.Is(err, service.ErrAgentNotFound):
		utils.NotFound(c, "Agent not found")
	case errors.Is(err, service.ErrRequestInvalid):
		utils.BadRequest(c, err.Error())
	default:
		utils.InternalError(c, "Failed to queue the request", err)
	}
}

// listAgentDiscoveryRequests lists the most recent requests with their outcome.
// @Summary      Requests sent to the agent about discovery
// @Tags         agents
// @Produce      json
// @ID           listAgentDiscoveryRequests
// @Param        id   path      string  true  "Agent ID"
// @Success      200  {object}  utils.APIResponse{data=object{requests=[]service.AgentDiscoveryRequestView}}
// @Failure      404  {object}  utils.APIResponse
// @Router       /v1/agents/{id}/discovery/requests [get]
func (s *Server) listAgentDiscoveryRequests(c *gin.Context) {
	if _, err := s.agentService.GetAgent(c.Param("id")); err != nil {
		utils.NotFound(c, "Agent not found")
		return
	}
	reqs, err := s.agentService.ListDiscoveryRequests(c.Param("id"), 50)
	if err != nil {
		utils.InternalError(c, "Failed to list requests", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Requests retrieved", gin.H{"requests": reqs})
}

// cancelAgentDiscoveryRequest withdraws a request the agent has not picked up.
// @Summary      Cancel a request the agent has not picked up
// @Tags         agents
// @Produce      json
// @ID           cancelAgentDiscoveryRequest
// @Param        id          path      string  true  "Agent ID"
// @Param        request_id  path      string  true  "Request ID"
// @Success      200         {object}  utils.APIResponse
// @Failure      404         {object}  utils.APIResponse
// @Failure      409         {object}  utils.APIResponse
// @Router       /v1/agents/{id}/discovery/requests/{request_id} [delete]
func (s *Server) cancelAgentDiscoveryRequest(c *gin.Context) {
	err := s.agentService.CancelDiscoveryRequest(c.Param("id"), c.Param("request_id"))
	switch {
	case err == nil:
		utils.SuccessResponse(c, http.StatusOK, "Request cancelled", gin.H{})
	case errors.Is(err, service.ErrRequestNotFound):
		utils.NotFound(c, "Request not found")
	case errors.Is(err, service.ErrRequestNotCancelable):
		utils.ErrorResponse(c, http.StatusConflict, err.Error(), nil)
	default:
		utils.InternalError(c, "Failed to cancel the request", err)
	}
}

// AcceptDiscoveredRequest chooses suggestions to accept.
type AcceptDiscoveredRequest struct {
	ItemIDs []string `json:"item_ids" binding:"required"`
}

// acceptDiscoveredItems turns suggestions into monitors.
// @Summary      Accept detected suggestions
// @Description  Creates a monitor definition (Managed in Console) for each selected service, container, pm2 app, port or disk from the agent's latest scan, with the defaults interval 30 s and fail after 2. Storage rows are queued as one track_storage request. Log files are skipped: they need a log key. Core does not run anything; the agent fetches the definitions at its next sync and reports under the same names.
// @Tags         agents
// @Accept       json
// @Produce      json
// @ID           acceptDiscoveredItems
// @Param        agent_id path      string                   true  "Agent ID"
// @Param        request  body      AcceptDiscoveredRequest  true  "Suggestions"
// @Success      200      {object}  utils.APIResponse{data=service.AcceptResult}
// @Failure      400      {object}  utils.APIResponse
// @Failure      404      {object}  utils.APIResponse
// @Router       /v1/agents/{agent_id}/discovery/accept [post]
func (s *Server) acceptDiscoveredItems(c *gin.Context) {
	var in AcceptDiscoveredRequest
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "item_ids is required")
		return
	}
	if _, err := s.agentService.GetAgent(c.Param("agent_id")); err != nil {
		utils.NotFound(c, "Agent not found")
		return
	}
	res, err := s.agentService.AcceptDiscoveredItems(c.Param("agent_id"), in.ItemIDs, frontendActorID(c))
	switch {
	case err == nil:
		utils.SuccessResponse(c, http.StatusOK, "Suggestions accepted", res)
	case errors.Is(err, service.ErrRequestInvalid):
		utils.BadRequest(c, err.Error())
	default:
		utils.InternalError(c, "Failed to accept the suggestions", err)
	}
}

// pullAgentDiscoveryRequests hands the agent what is waiting for it.
// @Summary      Agent: collect queued discovery requests
// @Description  Agent-to-Core. Returns the requests the Console queued and marks them delivered. One delivered but never finished is offered again after five minutes.
// @Tags         agents
// @Produce      json
// @Security     BearerAuth
// @ID           pullAgentDiscoveryRequests
// @Param        agent_id  path      string  true  "Agent ID"
// @Success      200       {object}  utils.APIResponse{data=object{requests=[]service.AgentDiscoveryRequestView}}
// @Failure      401       {object}  utils.APIResponse
// @Router       /v1/agents/{agent_id}/discovery/requests/pull [post]
func (s *Server) pullAgentDiscoveryRequests(c *gin.Context) {
	reqs, err := s.agentService.PullDiscoveryRequests(c.Param("agent_id"))
	if err != nil {
		utils.InternalError(c, "Failed to load requests", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Requests retrieved", gin.H{"requests": reqs})
}

// DiscoveryRequestResult is the agent's outcome for one request.
type DiscoveryRequestResult struct {
	Status string `json:"status" binding:"required" enums:"applied,failed,unsupported"`
	// Result is the agent's own words (a reason when it failed), shown verbatim in the Console.
	Result string `json:"result,omitempty"`
}

// finishAgentDiscoveryRequest records what the agent did.
// @Summary      Agent: report the outcome of a discovery request
// @Tags         agents
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           finishAgentDiscoveryRequest
// @Param        agent_id    path      string                  true  "Agent ID"
// @Param        request_id  path      string                  true  "Request ID"
// @Param        data        body      DiscoveryRequestResult  true  "Outcome"
// @Success      200         {object}  utils.APIResponse
// @Failure      400         {object}  utils.APIResponse
// @Failure      404         {object}  utils.APIResponse
// @Router       /v1/agents/{agent_id}/discovery/requests/{request_id}/result [post]
func (s *Server) finishAgentDiscoveryRequest(c *gin.Context) {
	var in DiscoveryRequestResult
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "status is required")
		return
	}
	err := s.agentService.FinishDiscoveryRequest(c.Param("agent_id"), c.Param("request_id"), in.Status, in.Result)
	switch {
	case err == nil:
		utils.SuccessResponse(c, http.StatusOK, "Outcome recorded", gin.H{})
	case errors.Is(err, service.ErrRequestInvalid):
		utils.BadRequest(c, err.Error())
	case errors.Is(err, service.ErrRequestNotFound):
		utils.NotFound(c, "Request not found")
	default:
		utils.InternalError(c, "Failed to record the outcome", err)
	}
}

// DefinitionResultsRequest is what the agent says about running its definitions.
type DefinitionResultsRequest struct {
	Results []service.DefinitionResult `json:"results" binding:"required"`
}

// recordAgentDefinitionResults stores why a definition could not run.
// @Summary      Agent: report which monitor definitions could not run
// @Description  Agent-to-Core. For each definition the agent could not run it sends ok=false and the reason in its own words; ok=true clears an earlier failure.
// @Tags         agents
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           recordAgentDefinitionResults
// @Param        agent_id  path      string                    true  "Agent ID"
// @Param        data      body      DefinitionResultsRequest  true  "Results"
// @Success      200       {object}  utils.APIResponse
// @Failure      400       {object}  utils.APIResponse
// @Router       /v1/agents/{agent_id}/monitor-definitions/results [post]
func (s *Server) recordAgentDefinitionResults(c *gin.Context) {
	var in DefinitionResultsRequest
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "results is required")
		return
	}
	if err := s.agentService.RecordDefinitionResults(c.Param("agent_id"), in.Results); err != nil {
		if errors.Is(err, service.ErrDefinitionInvalid) {
			utils.BadRequest(c, err.Error())
			return
		}
		utils.InternalError(c, "Failed to record the results", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Results recorded", gin.H{})
}
