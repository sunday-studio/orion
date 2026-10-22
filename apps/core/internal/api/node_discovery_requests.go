package api

import (
	"errors"
	"net/http"
	"orion/core/internal/service"
	"orion/core/internal/utils"

	"github.com/gin-gonic/gin"
)

// createNodeDiscoveryRequest queues a request for the node.
// @Summary      Ask the node to do something about discovery
// @Description  The Console never changes the host. A request is queued here and the node applies it at its next check-in (about every minute), then reports the outcome. Types: rescan, dismiss, restore, ignore, unignore (payload.ids), set_scanning (payload.enabled), set_kind (payload.kind, payload.enabled), track_storage (payload.paths). A node that has not said it can take requests gets the request stored as unsupported. Requests expire after 24 hours.
// @Tags         nodes
// @Accept       json
// @Produce      json
// @ID           createNodeDiscoveryRequest
// @Param        node_id path      string                                 true  "Node ID"
// @Param        request  body      service.NodeDiscoveryRequestInput     true  "Request"
// @Success      201      {object}  utils.APIResponse{data=object{request=service.NodeDiscoveryRequestView}}
// @Failure      400      {object}  utils.APIResponse
// @Failure      404      {object}  utils.APIResponse
// @Router       /v1/nodes/{node_id}/discovery/requests [post]
func (s *Server) createNodeDiscoveryRequest(c *gin.Context) {
	var in service.NodeDiscoveryRequestInput
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "type is required")
		return
	}
	v, err := s.nodeService.CreateDiscoveryRequest(c.Param("node_id"), in, frontendActorID(c))
	switch {
	case err == nil:
		utils.SuccessResponse(c, http.StatusCreated, "Request queued", gin.H{"request": v})
	case errors.Is(err, service.ErrNodeNotFound):
		utils.NotFound(c, "Node not found")
	case errors.Is(err, service.ErrRequestInvalid):
		utils.BadRequest(c, err.Error())
	default:
		utils.InternalError(c, "Failed to queue the request", err)
	}
}

// listNodeDiscoveryRequests lists the most recent requests with their outcome.
// @Summary      Requests sent to the node about discovery
// @Tags         nodes
// @Produce      json
// @ID           listNodeDiscoveryRequests
// @Param        id   path      string  true  "Node ID"
// @Success      200  {object}  utils.APIResponse{data=object{requests=[]service.NodeDiscoveryRequestView}}
// @Failure      404  {object}  utils.APIResponse
// @Router       /v1/nodes/{id}/discovery/requests [get]
func (s *Server) listNodeDiscoveryRequests(c *gin.Context) {
	if _, err := s.nodeService.GetNode(c.Param("id")); err != nil {
		utils.NotFound(c, "Node not found")
		return
	}
	reqs, err := s.nodeService.ListDiscoveryRequests(c.Param("id"), 50)
	if err != nil {
		utils.InternalError(c, "Failed to list requests", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Requests retrieved", gin.H{"requests": reqs})
}

// cancelNodeDiscoveryRequest withdraws a request the node has not picked up.
// @Summary      Cancel a request the node has not picked up
// @Tags         nodes
// @Produce      json
// @ID           cancelNodeDiscoveryRequest
// @Param        id          path      string  true  "Node ID"
// @Param        request_id  path      string  true  "Request ID"
// @Success      200         {object}  utils.APIResponse
// @Failure      404         {object}  utils.APIResponse
// @Failure      409         {object}  utils.APIResponse
// @Router       /v1/nodes/{id}/discovery/requests/{request_id} [delete]
func (s *Server) cancelNodeDiscoveryRequest(c *gin.Context) {
	err := s.nodeService.CancelDiscoveryRequest(c.Param("id"), c.Param("request_id"))
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
	// IntervalSeconds is how often each new monitor runs (10 to 86400). Default 30.
	IntervalSeconds int `json:"interval_seconds,omitempty"`
	// FailAfter is how many failed checks in a row raise an alert (1 to 20). Default 2.
	FailAfter int `json:"fail_after,omitempty"`
}

// acceptDiscoveredItems turns suggestions into monitors.
// @Summary      Accept detected suggestions
// @Description  Creates a monitor definition (Managed in Console) for each selected service, container, pm2 app, port or disk from the node's latest scan, with interval_seconds and fail_after from the request (defaults 30 s and fail after 2, applied to every monitor created). Storage rows are queued as one track_storage request. Log files are skipped: they need a log key. Core does not run anything; the node fetches the definitions at its next sync and reports under the same names.
// @Tags         nodes
// @Accept       json
// @Produce      json
// @ID           acceptDiscoveredItems
// @Param        node_id path      string                   true  "Node ID"
// @Param        request  body      AcceptDiscoveredRequest  true  "Suggestions"
// @Success      200      {object}  utils.APIResponse{data=service.AcceptResult}
// @Failure      400      {object}  utils.APIResponse
// @Failure      404      {object}  utils.APIResponse
// @Router       /v1/nodes/{node_id}/discovery/accept [post]
func (s *Server) acceptDiscoveredItems(c *gin.Context) {
	var in AcceptDiscoveredRequest
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "item_ids is required")
		return
	}
	if _, err := s.nodeService.GetNode(c.Param("node_id")); err != nil {
		utils.NotFound(c, "Node not found")
		return
	}
	res, err := s.nodeService.AcceptDiscoveredItems(c.Param("node_id"), in.ItemIDs, in.IntervalSeconds, in.FailAfter, frontendActorID(c))
	switch {
	case err == nil:
		utils.SuccessResponse(c, http.StatusOK, "Suggestions accepted", res)
	case errors.Is(err, service.ErrRequestInvalid):
		utils.BadRequest(c, err.Error())
	default:
		utils.InternalError(c, "Failed to accept the suggestions", err)
	}
}

// pullNodeDiscoveryRequests hands the node what is waiting for it.
// @Summary      Node: collect queued discovery requests
// @Description  Node-to-Core. Returns the requests the Console queued and marks them delivered. One delivered but never finished is offered again after five minutes.
// @Tags         nodes
// @Produce      json
// @Security     BearerAuth
// @ID           pullNodeDiscoveryRequests
// @Param        node_id  path      string  true  "Node ID"
// @Success      200       {object}  utils.APIResponse{data=object{requests=[]service.NodeDiscoveryRequestView}}
// @Failure      401       {object}  utils.APIResponse
// @Router       /v1/nodes/{node_id}/discovery/requests/pull [post]
func (s *Server) pullNodeDiscoveryRequests(c *gin.Context) {
	reqs, err := s.nodeService.PullDiscoveryRequests(c.Param("node_id"))
	if err != nil {
		utils.InternalError(c, "Failed to load requests", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Requests retrieved", gin.H{"requests": reqs})
}

// DiscoveryRequestResult is the node's outcome for one request.
type DiscoveryRequestResult struct {
	Status string `json:"status" binding:"required" enums:"applied,failed,unsupported"`
	// Result is the node's own words (a reason when it failed), shown verbatim in the Console.
	Result string `json:"result,omitempty"`
}

// finishNodeDiscoveryRequest records what the node did.
// @Summary      Node: report the outcome of a discovery request
// @Tags         nodes
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           finishNodeDiscoveryRequest
// @Param        node_id    path      string                  true  "Node ID"
// @Param        request_id  path      string                  true  "Request ID"
// @Param        data        body      DiscoveryRequestResult  true  "Outcome"
// @Success      200         {object}  utils.APIResponse
// @Failure      400         {object}  utils.APIResponse
// @Failure      404         {object}  utils.APIResponse
// @Router       /v1/nodes/{node_id}/discovery/requests/{request_id}/result [post]
func (s *Server) finishNodeDiscoveryRequest(c *gin.Context) {
	var in DiscoveryRequestResult
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "status is required")
		return
	}
	err := s.nodeService.FinishDiscoveryRequest(c.Param("node_id"), c.Param("request_id"), in.Status, in.Result)
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

// DefinitionResultsRequest is what the node says about running its definitions.
type DefinitionResultsRequest struct {
	Results []service.DefinitionResult `json:"results" binding:"required"`
}

// recordNodeDefinitionResults stores why a definition could not run.
// @Summary      Node: report which monitor definitions could not run
// @Description  Node-to-Core. For each definition the node could not run it sends ok=false and the reason in its own words; ok=true clears an earlier failure.
// @Tags         nodes
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           recordNodeDefinitionResults
// @Param        node_id  path      string                    true  "Node ID"
// @Param        data      body      DefinitionResultsRequest  true  "Results"
// @Success      200       {object}  utils.APIResponse
// @Failure      400       {object}  utils.APIResponse
// @Router       /v1/nodes/{node_id}/monitor-definitions/results [post]
func (s *Server) recordNodeDefinitionResults(c *gin.Context) {
	var in DefinitionResultsRequest
	if err := c.ShouldBindJSON(&in); err != nil {
		utils.BadRequest(c, "results is required")
		return
	}
	if err := s.nodeService.RecordDefinitionResults(c.Param("node_id"), in.Results); err != nil {
		if errors.Is(err, service.ErrDefinitionInvalid) {
			utils.BadRequest(c, err.Error())
			return
		}
		utils.InternalError(c, "Failed to record the results", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Results recorded", gin.H{})
}
