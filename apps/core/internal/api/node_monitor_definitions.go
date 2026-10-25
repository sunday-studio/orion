package api

import (
	"errors"
	"net/http"
	"orion/core/internal/service"
	"orion/core/internal/utils"
	"time"

	"github.com/gin-gonic/gin"
)

type NodeMonitorDefinitionRequest struct {
	Name string `json:"name" binding:"required"`
	// CheckType: disk (params path, threshold_percent), memory (threshold_percent), process (process_name), port (port), systemd (unit), docker (container), pm2 (app).
	CheckType       string         `json:"check_type" binding:"required" enums:"disk,memory,process,port,systemd,docker,pm2"`
	Params          map[string]any `json:"params,omitempty"`
	IntervalSeconds int            `json:"interval_seconds,omitempty"`
	FailAfter       int            `json:"fail_after,omitempty"`
}

type NodeMonitorDefinitionResponse struct {
	ID              string         `json:"id"`
	Name            string         `json:"name"`
	CheckType       string         `json:"check_type"`
	Params          map[string]any `json:"params"`
	IntervalSeconds int            `json:"interval_seconds"`
	FailAfter       int            `json:"fail_after"`
	CreatedAt       time.Time      `json:"created_at"`
	// State is waiting until the node has fetched the definition and reported under this name, then reporting.
	State string `json:"state" enums:"waiting,failed,reporting"`
	// FailureReason is why the node could not run this check, in its own words (state failed).
	FailureReason string `json:"failure_reason,omitempty"`
	// DeliveredAt is when the node first fetched this definition (Requested becomes Applied).
	DeliveredAt *time.Time `json:"delivered_at,omitempty"`
	MonitorID   string     `json:"monitor_id,omitempty"`
	FirstReport *time.Time `json:"first_report_at,omitempty"`
}

func definitionResponse(v service.NodeMonitorDefinitionView) NodeMonitorDefinitionResponse {
	return NodeMonitorDefinitionResponse{ID: v.ID, Name: v.Name, CheckType: v.CheckType, Params: v.ParamsParsed, IntervalSeconds: v.IntervalSeconds, FailAfter: v.FailAfter,
		CreatedAt: v.CreatedAt, State: v.State, FailureReason: v.FailureReason, DeliveredAt: v.DeliveredAt, MonitorID: v.MonitorID, FirstReport: v.FirstReport}
}

// createNodeMonitorDefinition configures a check for a server's node to run.
// @Summary      Create a node monitor definition
// @Description  Defines a disk, memory, process or port check. The node fetches its definitions with POST /v1/nodes/{node_id}/monitor-definitions/sync and reports under the same name; state turns reporting after the first report.
// @Tags         nodes
// @Accept       json
// @Produce      json
// @ID           createNodeMonitorDefinition
// @Param        node_id path      string                         true  "Node ID"
// @Param        request  body      NodeMonitorDefinitionRequest  true  "Check"
// @Success      201      {object}  utils.APIResponse{data=object{definition=NodeMonitorDefinitionResponse}}
// @Failure      400      {object}  utils.APIResponse
// @Failure      404      {object}  utils.APIResponse
// @Failure      409      {object}  utils.APIResponse
// @Router       /v1/nodes/{node_id}/monitor-definitions [post]
func (s *Server) createNodeMonitorDefinition(c *gin.Context) {
	var req NodeMonitorDefinitionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "name and check_type are required")
		return
	}
	def, err := s.nodeService.CreateMonitorDefinition(nodeIDParam(c), service.NodeMonitorDefinitionInput{Name: req.Name, CheckType: req.CheckType, Params: req.Params, IntervalSeconds: req.IntervalSeconds, FailAfter: req.FailAfter}, frontendActorID(c))
	switch {
	case err == nil:
		views, _ := s.nodeService.ListMonitorDefinitions(def.NodeID)
		for _, v := range views {
			if v.ID == def.ID {
				utils.SuccessResponse(c, http.StatusCreated, "Monitor definition created", gin.H{"definition": definitionResponse(v)})
				return
			}
		}
		utils.SuccessResponse(c, http.StatusCreated, "Monitor definition created", gin.H{})
	case errors.Is(err, service.ErrNodeNotFound):
		utils.NotFound(c, "Node not found")
	case errors.Is(err, service.ErrDefinitionExists):
		utils.ErrorResponse(c, http.StatusConflict, err.Error(), nil)
	case errors.Is(err, service.ErrDefinitionInvalid):
		utils.BadRequest(c, err.Error())
	default:
		utils.InternalError(c, "Failed to create the monitor definition", err)
	}
}

// listNodeMonitorDefinitions lists a server's configured node checks with their live state.
// @Summary      List node monitor definitions
// @Tags         nodes
// @Produce      json
// @ID           listNodeMonitorDefinitions
// @Param        id   path      string  true  "Node ID"
// @Success      200  {object}  utils.APIResponse{data=object{definitions=[]NodeMonitorDefinitionResponse}}
// @Failure      404  {object}  utils.APIResponse
// @Router       /v1/nodes/{id}/monitor-definitions [get]
func (s *Server) listNodeMonitorDefinitions(c *gin.Context) {
	if _, err := s.nodeService.GetNode(c.Param("id")); err != nil {
		utils.NotFound(c, "Node not found")
		return
	}
	views, err := s.nodeService.ListMonitorDefinitions(c.Param("id"))
	if err != nil {
		utils.InternalError(c, "Failed to list monitor definitions", err)
		return
	}
	out := make([]NodeMonitorDefinitionResponse, 0, len(views))
	for _, v := range views {
		out = append(out, definitionResponse(v))
	}
	utils.SuccessResponse(c, http.StatusOK, "Monitor definitions retrieved", gin.H{"definitions": out})
}

// deleteNodeMonitorDefinition removes a configured check.
// @Summary      Delete a node monitor definition
// @Tags         nodes
// @Produce      json
// @ID           deleteNodeMonitorDefinition
// @Param        id             path      string  true  "Node ID"
// @Param        definition_id  path      string  true  "Definition ID"
// @Success      200            {object}  utils.APIResponse
// @Failure      404            {object}  utils.APIResponse
// @Router       /v1/nodes/{id}/monitor-definitions/{definition_id} [delete]
func (s *Server) deleteNodeMonitorDefinition(c *gin.Context) {
	if err := s.nodeService.DeleteMonitorDefinition(c.Param("id"), c.Param("definition_id")); err != nil {
		if errors.Is(err, service.ErrDefinitionNotFound) {
			utils.NotFound(c, "Monitor definition not found")
			return
		}
		utils.InternalError(c, "Failed to delete the monitor definition", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Monitor definition deleted", gin.H{})
}

// NodePulledDefinition is what the node runs.
type NodePulledDefinition struct {
	ID              string         `json:"id"`
	Name            string         `json:"name"`
	CheckType       string         `json:"check_type"`
	Params          map[string]any `json:"params"`
	IntervalSeconds int            `json:"interval_seconds"`
	FailAfter       int            `json:"fail_after"`
}

// pullNodeMonitorDefinitions is called by the node with its token.
// @Summary      Node: sync configured monitor definitions
// @Description  Node-to-Core. Returns the checks configured for this node in the Console; the node registers each as a monitor with the same name and reports to it.
// @Tags         nodes
// @Produce      json
// @ID           pullNodeMonitorDefinitions
// @Param        node_id  path      string  true  "Node ID"
// @Success      200       {object}  utils.APIResponse{data=object{definitions=[]NodePulledDefinition}}
// @Failure      401       {object}  utils.APIResponse
// @Router       /v1/nodes/{node_id}/monitor-definitions/sync [post]
func (s *Server) pullNodeMonitorDefinitions(c *gin.Context) {
	s.nodeService.MarkDefinitionsDelivered(c.Param("node_id"))
	views, err := s.nodeService.ListMonitorDefinitions(c.Param("node_id"))
	if err != nil {
		utils.InternalError(c, "Failed to load monitor definitions", err)
		return
	}
	out := make([]NodePulledDefinition, 0, len(views))
	for _, v := range views {
		out = append(out, NodePulledDefinition{ID: v.ID, Name: v.Name, CheckType: v.CheckType, Params: v.ParamsParsed, IntervalSeconds: v.IntervalSeconds, FailAfter: v.FailAfter})
	}
	utils.SuccessResponse(c, http.StatusOK, "Monitor definitions retrieved", gin.H{"definitions": out})
}
