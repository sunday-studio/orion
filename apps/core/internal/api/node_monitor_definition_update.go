package api

import (
	"errors"
	"net/http"
	"orion/core/internal/service"
	"orion/core/internal/utils"

	"github.com/gin-gonic/gin"
)

// UpdateNodeMonitorDefinitionRequest changes the timing of one definition. Give at least one field.
type UpdateNodeMonitorDefinitionRequest struct {
	// IntervalSeconds is how often the check runs (10 to 86400).
	IntervalSeconds int `json:"interval_seconds,omitempty"`
	// FailAfter is how many failed checks in a row raise an alert (1 to 20).
	FailAfter int `json:"fail_after,omitempty"`
}

// BulkUpdateNodeMonitorDefinitionsRequest changes the timing of several definitions at once.
type BulkUpdateNodeMonitorDefinitionsRequest struct {
	IDs []string `json:"ids" binding:"required"`
	UpdateNodeMonitorDefinitionRequest
}

func (s *Server) updateDefinitions(c *gin.Context, ids []string, req UpdateNodeMonitorDefinitionRequest) {
	nodeID := c.Param("id")
	if _, err := s.nodeService.GetNode(nodeID); err != nil {
		utils.NotFound(c, "Node not found")
		return
	}
	if _, err := s.nodeService.UpdateMonitorDefinitions(nodeID, ids, req.IntervalSeconds, req.FailAfter); err != nil {
		switch {
		case errors.Is(err, service.ErrDefinitionNotFound):
			utils.NotFound(c, "Monitor definition not found")
		case errors.Is(err, service.ErrDefinitionInvalid):
			utils.BadRequest(c, err.Error())
		default:
			utils.InternalError(c, "Failed to update the monitor definitions", err)
		}
		return
	}
	views, err := s.nodeService.ListMonitorDefinitions(nodeID)
	if err != nil {
		utils.InternalError(c, "Failed to list monitor definitions", err)
		return
	}
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	out := []NodeMonitorDefinitionResponse{}
	for _, v := range views {
		if want[v.ID] {
			out = append(out, definitionResponse(v))
		}
	}
	utils.SuccessResponse(c, http.StatusOK, "Monitor definitions updated", gin.H{"definitions": out})
}

// updateNodeMonitorDefinition changes how often a Console-managed node monitor runs.
// @Summary      Update a node monitor definition
// @Description  Changes interval_seconds and/or fail_after of a Console-managed node monitor. The definition keeps its id and name, so the monitor keeps its history; the node picks up the new values at its next sync (within about a minute).
// @Tags         nodes
// @Accept       json
// @Produce      json
// @ID           updateNodeMonitorDefinition
// @Param        id             path      string                              true  "Node ID"
// @Param        definition_id  path      string                              true  "Definition ID"
// @Param        request        body      UpdateNodeMonitorDefinitionRequest  true  "Timing"
// @Success      200            {object}  utils.APIResponse{data=object{definitions=[]NodeMonitorDefinitionResponse}}
// @Failure      400            {object}  utils.APIResponse
// @Failure      404            {object}  utils.APIResponse
// @Router       /v1/nodes/{id}/monitor-definitions/{definition_id} [patch]
func (s *Server) updateNodeMonitorDefinition(c *gin.Context) {
	var req UpdateNodeMonitorDefinitionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "interval_seconds and fail_after must be numbers")
		return
	}
	s.updateDefinitions(c, []string{c.Param("definition_id")}, req)
}

// bulkUpdateNodeMonitorDefinitions changes the timing of several node monitors at once.
// @Summary      Update several node monitor definitions
// @Description  Applies interval_seconds and/or fail_after to every listed definition, all or nothing: if one id is unknown or a value is out of range nothing changes.
// @Tags         nodes
// @Accept       json
// @Produce      json
// @ID           bulkUpdateNodeMonitorDefinitions
// @Param        id       path      string                                   true  "Node ID"
// @Param        request  body      BulkUpdateNodeMonitorDefinitionsRequest  true  "Definitions and timing"
// @Success      200      {object}  utils.APIResponse{data=object{definitions=[]NodeMonitorDefinitionResponse}}
// @Failure      400      {object}  utils.APIResponse
// @Failure      404      {object}  utils.APIResponse
// @Router       /v1/nodes/{id}/monitor-definitions [patch]
func (s *Server) bulkUpdateNodeMonitorDefinitions(c *gin.Context) {
	var req BulkUpdateNodeMonitorDefinitionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "ids is required")
		return
	}
	s.updateDefinitions(c, req.IDs, req.UpdateNodeMonitorDefinitionRequest)
}
