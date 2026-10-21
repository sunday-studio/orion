package api

import (
	"errors"
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"orion/core/internal/utils"
	"time"

	"github.com/gin-gonic/gin"
)

// registerNode registers a new node or reconnects an existing one.
// @Summary      Register a node
// @Description  Register a new node or reconnect an existing node by machine ID
// @Tags         nodes
// @Accept       json
// @Produce      json
// @ID           registerNode
// @Param        request  body      service.RegisterRequest  true  "Node registration request"
// @Success      200      {object}  utils.APIResponse{data=service.RegisterResponse}
// @Param        X-Orion-Enrollment-Token  header  string  false  "Enrollment secret; required when ORION_ENROLLMENT_TOKEN is set on Core"
// @Failure      400      {object}  utils.APIResponse
// @Failure      401      {object}  utils.APIResponse
// @Failure      429      {object}  utils.APIResponse
// @Failure      500      {object}  utils.APIResponse
// @Router       /v1/register [post]
func (s *Server) registerNode(c *gin.Context) {
	if !s.registerLimiter.Allow(c.ClientIP()) {
		utils.ErrorResponse(c, http.StatusTooManyRequests, "Too many registration attempts", nil)
		return
	}
	var req service.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		s.logger.Error("Invalid registration request", "error", err)
		utils.BadRequest(c, "Invalid request payload")
		return
	}
	// A one-time wizard token (orn_enr_...) is authorized per machine; anything else is the
	// shared ORION_ENROLLMENT_TOKEN (or open registration when none is configured).
	presented := c.GetHeader(enrollmentTokenHeader)
	enrollmentID := ""
	if service.IsEnrollmentToken(presented) {
		enrollmentID = s.enrollments.Authorize(presented, req.MachineId, time.Now().UTC())
		if enrollmentID == "" {
			utils.Unauthorized(c, "Valid enrollment token required")
			return
		}
	} else if !s.enrollmentTokenValid(presented) {
		utils.Unauthorized(c, "Valid enrollment token required")
		return
	}
	s.logger.Info("Node registration request", "machine_id", req.MachineId, "name", req.Name, "os", req.OS, "arch", req.Arch)
	response, err := s.nodeService.RegisterNode(&req)
	if errors.Is(err, service.ErrNodeTokenReissueRequired) && enrollmentID != "" {
		// A fresh one-time Add server session is explicit admin approval to
		// reconnect this known machine. Shared registration credentials are
		// not sufficient because they may be long-lived.
		response, err = s.nodeService.ReconnectNode(&req, enrollmentID)
	}
	if err != nil {
		switch {
		case errors.Is(err, service.ErrNodeTokenRevoked):
			utils.ErrorResponse(c, http.StatusConflict, "node_token_revoked", nil)
			return
		case errors.Is(err, service.ErrNodeTokenReissueRequired):
			utils.ErrorResponse(c, http.StatusConflict, "node_token_reissue_required", nil)
			return
		}
		s.logger.Error("Failed to register node", "error", err)
		utils.InternalError(c, "Failed to register node", err)
		return
	}

	if enrollmentID != "" {
		s.enrollments.LinkNode(enrollmentID, response.NodeID)
	}
	s.logger.Info("Node registered successfully", "node_id", response.NodeID, "machine_id", req.MachineId)
	utils.SuccessResponse(c, 200, "Node registered successfully", response)
}

type NodeTokenActionRequest struct {
	Reason string `json:"reason,omitempty"`
}
type NodeTokenStatusResponse struct {
	NodeID                string     `json:"node_id"`
	State                 string     `json:"state"`
	TokenVersion          int        `json:"token_version"`
	TokenRotatedAt        *time.Time `json:"token_rotated_at,omitempty"`
	TokenRevokedAt        *time.Time `json:"token_revoked_at,omitempty"`
	TokenRevocationReason string     `json:"token_revocation_reason,omitempty"`
	TokenExists           bool       `json:"token_exists"`
	LastUsedAt            *time.Time `json:"last_used_at,omitempty"`
	// PreviousTokenValidUntil is set for 10 minutes after a rotation while the old token still works.
	PreviousTokenValidUntil *time.Time `json:"previous_token_valid_until,omitempty"`
	RequestID               string     `json:"request_id,omitempty"`
}
type NodeTokenIssuedResponse struct {
	Token  string                  `json:"token"`
	Status NodeTokenStatusResponse `json:"status"`
}

// getNodeTokenStatus returns non-secret token lifecycle metadata for a node.
// @Summary      Get node token status
// @Description  Get non-secret token lifecycle metadata for a node
// @Tags         nodes
// @Accept       json
// @Produce      json
// @ID           getNodeTokenStatus
// @Param        node_id  path      string  true  "Node ID"
// @Success      200       {object}  utils.APIResponse{data=NodeTokenStatusResponse}
// @Failure      400       {object}  utils.APIResponse
// @Failure      404       {object}  utils.APIResponse
// @Router       /v1/nodes/{node_id}/token/status [get]
func (s *Server) getNodeTokenStatus(c *gin.Context) {
	nodeID := nodeIDParam(c)
	if nodeID == "" {
		utils.BadRequest(c, "Node ID is required")
		return
	}
	status, err := s.nodeService.NodeTokenStatus(nodeID)
	if err != nil {
		utils.NotFound(c, "Node not found")
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Node token status retrieved successfully", nodeTokenStatusResponse(*status, requestIDFromContext(c)))
}

// rotateNodeToken creates a replacement token for an active node.
// @Summary      Rotate node token
// @Description  Create a replacement token for an active node and return it once
// @Tags         nodes
// @Accept       json
// @Produce      json
// @ID           rotateNodeToken
// @Param        node_id  path      string                   true   "Node ID"
// @Param        request   body      NodeTokenActionRequest  false  "Token action request"
// @Success      200       {object}  utils.APIResponse{data=NodeTokenIssuedResponse}
// @Failure      400       {object}  utils.APIResponse
// @Failure      404       {object}  utils.APIResponse
// @Failure      409       {object}  utils.APIResponse
// @Router       /v1/nodes/{node_id}/token/rotate [post]
func (s *Server) rotateNodeToken(c *gin.Context) {
	s.nodeTokenIssueAction(c, s.nodeService.RotateNodeToken, "Node token rotated successfully")
}

// reissueNodeToken creates a replacement token for a revoked node.
// @Summary      Reissue node token
// @Description  Create a replacement token for a revoked node and return it once
// @Tags         nodes
// @Accept       json
// @Produce      json
// @ID           reissueNodeToken
// @Param        node_id  path      string                   true   "Node ID"
// @Param        request   body      NodeTokenActionRequest  false  "Token action request"
// @Success      200       {object}  utils.APIResponse{data=NodeTokenIssuedResponse}
// @Failure      400       {object}  utils.APIResponse
// @Failure      404       {object}  utils.APIResponse
// @Failure      409       {object}  utils.APIResponse
// @Router       /v1/nodes/{node_id}/token/reissue [post]
func (s *Server) reissueNodeToken(c *gin.Context) {
	s.nodeTokenIssueAction(c, s.nodeService.ReissueNodeToken, "Node token reissued successfully")
}

// revokeNodeToken revokes the active token for a node.
// @Summary      Revoke node token
// @Description  Revoke the active token for a node without deleting the node or monitors
// @Tags         nodes
// @Accept       json
// @Produce      json
// @ID           revokeNodeToken
// @Param        node_id  path      string                   true   "Node ID"
// @Param        request   body      NodeTokenActionRequest  false  "Token action request"
// @Success      200       {object}  utils.APIResponse{data=NodeTokenStatusResponse}
// @Failure      400       {object}  utils.APIResponse
// @Failure      404       {object}  utils.APIResponse
// @Failure      409       {object}  utils.APIResponse
// @Router       /v1/nodes/{node_id}/token/revoke [post]
func (s *Server) revokeNodeToken(c *gin.Context) {
	nodeID := nodeIDParam(c)
	if nodeID == "" {
		utils.BadRequest(c, "Node ID is required")
		return
	}
	req, ok := bindNodeTokenActionRequest(c)
	if !ok {
		return
	}
	status, err := s.nodeService.RevokeNodeToken(nodeID, nodeTokenActionInput(c, req))
	if err != nil {
		s.respondNodeTokenActionError(c, err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Node token revoked successfully", nodeTokenStatusResponse(*status, requestIDFromContext(c)))
}
func (s *Server) nodeTokenIssueAction(c *gin.Context, action func(string, service.NodeTokenActionInput) (*service.NodeTokenIssueResult, error), message string) {
	nodeID := nodeIDParam(c)
	if nodeID == "" {
		utils.BadRequest(c, "Node ID is required")
		return
	}
	req, ok := bindNodeTokenActionRequest(c)
	if !ok {
		return
	}
	result, err := action(nodeID, nodeTokenActionInput(c, req))
	if err != nil {
		s.respondNodeTokenActionError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	utils.SuccessResponse(c, http.StatusOK, message, NodeTokenIssuedResponse{Token: result.Token, Status: nodeTokenStatusResponse(result.Status, requestIDFromContext(c))})
}
func bindNodeTokenActionRequest(c *gin.Context) (NodeTokenActionRequest, bool) {
	var req NodeTokenActionRequest
	if c.Request.Body == nil || c.Request.ContentLength == 0 {
		return req, true
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "Invalid request payload")
		return NodeTokenActionRequest{}, false
	}
	return req, true
}
func (s *Server) respondNodeTokenActionError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrNodeTokenRevoked):
		utils.ErrorResponse(c, http.StatusConflict, "node_token_revoked", nil)
	case errors.Is(err, service.ErrNodeTokenNotRevoked):
		utils.ErrorResponse(c, http.StatusConflict, "node_token_not_revoked", nil)
	default:
		utils.NotFound(c, "Node not found")
	}
}
func nodeTokenActionInput(c *gin.Context, req NodeTokenActionRequest) service.NodeTokenActionInput {
	actorID := "admin"
	if value, ok := c.Get("frontend_actor_id"); ok {
		if subject, ok := value.(string); ok && subject != "" {
			actorID = subject
		}
	}
	return service.NodeTokenActionInput{ActorType: "user", ActorID: actorID, Reason: req.Reason, RequestID: requestIDFromContext(c)}
}
func nodeTokenStatusResponse(status service.NodeTokenStatus, requestID string) NodeTokenStatusResponse {
	return NodeTokenStatusResponse{NodeID: status.NodeID, State: status.State, TokenVersion: status.TokenVersion, TokenRotatedAt: status.TokenRotatedAt, TokenRevokedAt: status.TokenRevokedAt, TokenRevocationReason: status.TokenRevocationReason, TokenExists: status.TokenExists, LastUsedAt: status.LastUsedAt, PreviousTokenValidUntil: status.PreviousTokenValidUntil, RequestID: requestID}
}
func requestIDFromContext(c *gin.Context) string {
	if value, ok := c.Get("request_id"); ok {
		if requestID, ok := value.(string); ok {
			return requestID
		}
	}
	return ""
}
func nodeIDParam(c *gin.Context) string {
	if id := c.Param("node_id"); id != "" {
		return id
	}
	return c.Param("id")
}

// setMaintenanceMode sets the maintenance mode for a node.
// @Summary      Set node maintenance mode
// @Description  Enable or disable maintenance mode for a specific node. Enabling lasts duration_seconds (default 24h, max 30 days) and then expires automatically; every change is audited.
// @Tags         nodes
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           setMaintenanceMode
// @Param        node_id  path      string                            true  "Node ID"
// @Param        request   body      service.SetMaintenanceModeRequest  true  "Maintenance mode request"
// @Success      200       {object}  utils.APIResponse
// @Failure      400       {object}  utils.APIResponse
// @Failure      401       {object}  utils.APIResponse
// @Failure      500       {object}  utils.APIResponse
// @Router       /v1/nodes/{node_id}/maintenance [put]
func (s *Server) setMaintenanceMode(c *gin.Context) {
	nodeID := c.Param("node_id")
	if nodeID == "" {
		utils.BadRequest(c, "Node ID is required")
		return
	}
	var req service.SetMaintenanceModeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		s.logger.Error("Invalid maintenance mode request", "error", err)
		utils.BadRequest(c, "Invalid request payload")
		return
	}
	var duration time.Duration
	if req.DurationSeconds != nil {
		if *req.DurationSeconds < 1 {
			utils.BadRequest(c, service.ErrInvalidMaintenanceDuration.Error())
			return
		}
		duration = time.Duration(*req.DurationSeconds) * time.Second
	}
	actor := service.NodeTokenActionInput{ActorType: "node", ActorID: nodeID, RequestID: requestIDFromContext(c)}
	if err := s.nodeService.SetMaintenanceMode(nodeID, *req.MaintenanceMode, duration, actor); err != nil {
		if errors.Is(err, service.ErrInvalidMaintenanceDuration) {
			utils.BadRequest(c, err.Error())
			return
		}
		s.logger.Error("Failed to set maintenance mode", "error", err, "node_id", nodeID)
		utils.InternalError(c, "Failed to set maintenance mode", err)
		return
	}
	s.logger.Info("Maintenance mode set", "node_id", nodeID, "maintenance_mode", *req.MaintenanceMode)
	var updated db.Node
	_ = s.db.Select("id", "maintenance_mode", "maintenance_until").Where("id = ?", nodeID).First(&updated).Error
	utils.SuccessResponse(c, 200, "Maintenance mode updated successfully", gin.H{"node_id": nodeID, "maintenance_mode": *req.MaintenanceMode, "maintenance_until": updated.MaintenanceUntil})
}

// listNodes retrieves a paginated list of nodes.
// @Summary      List nodes
// @Description  Get a paginated list of all registered nodes
// @Tags         nodes
// @Accept       json
// @Produce      json
// @ID           getNodes
// @Param        limit          query  int     false  "Maximum number of nodes to return" default(50)
// @Param        offset         query  int     false  "Number of nodes to skip" default(0)
// @Param        search         query  string  false  "Search by server or monitor name"
// @Param        status         query  string  false  "Filter by computed server status"
// @Param        maintenance    query  string  false  "Filter by maintenance mode true or false"
// @Param        stale_only     query  bool    false  "Only return stale servers"
// @Param        has_incidents  query  bool    false  "Only return servers with active incidents"
// @Param        include_system query  bool    false  "Include Core's own system node (default false)"
// @Success      200            {object}  utils.APIResponse{data=object{nodes=[]NodeResponse,count=int64,limit=int,offset=int,pagination=utils.PaginationMeta}}
// @Failure      500            {object}  utils.APIResponse
// @Router       /v1/nodes [get]
func (s *Server) listNodes(c *gin.Context) {
	limit := utils.QueryInt(c, "limit", 50)
	offset := utils.QueryInt(c, "offset", 0)
	opts := service.ListNodesOpts{Limit: limit, Offset: offset, IncludeSystem: c.Query("include_system") == "true", Search: c.Query("search"), Status: c.Query("status"), Maintenance: c.Query("maintenance"), StaleOnly: c.Query("stale_only") == "true", HasIncidents: c.Query("has_incidents") == "true", LastSeen: c.Query("last_seen"), Uptime: c.Query("uptime"), Sort: c.DefaultQuery("sort", "last_seen"), Order: c.DefaultQuery("order", "desc")}
	nodes, count, err := s.nodeService.ListNodes(opts)
	if err != nil {
		s.logger.Error("Failed to list nodes", "error", err)
		utils.InternalError(c, "Failed to list nodes", err)
		return
	}
	responses := nodeListResponses(nodes)
	utils.SuccessResponse(c, 200, "Nodes retrieved successfully", gin.H{"nodes": responses, "count": count, "limit": limit, "offset": offset, "pagination": utils.NewPaginationMeta(count, limit, offset, len(responses))})
}

// getNodeSummary retrieves aggregate counts for the node list.
// @Summary      Get node summary
// @Description  Get aggregate counts for registered nodes by health, maintenance, stale, and incident status
// @Tags         nodes
// @Accept       json
// @Produce      json
// @ID           getNodeSummary
// @Success      200  {object}  utils.APIResponse{data=NodeSummaryResponse}
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/nodes/summary [get]
func (s *Server) getNodeSummary(c *gin.Context) {
	healthService := service.NewHealthService(s.db, s.logger)
	config := service.DefaultHealthConfig()
	var nodes []db.Node
	if err := s.db.Where("(deleted_at IS NULL OR deleted_at = ?) AND system = ?", time.Time{}, false).Find(&nodes).Error; err != nil {
		s.logger.Error("Failed to load node summary", "error", err)
		utils.InternalError(c, "Failed to get node summary", err)
		return
	}
	summary := NodeSummaryResponse{Total: int64(len(nodes))}
	nodeIDs := make([]string, 0, len(nodes))
	for _, node := range nodes {
		nodeIDs = append(nodeIDs, node.ID)
		if node.InMaintenance(time.Now().UTC()) {
			summary.Maintenance++
		}
		health := "unknown"
		if computedHealth, _, _, _, err := healthService.ComputeNodeHealth(node.ID, config); err == nil {
			health = computedHealth
		}
		switch health {
		case "up":
			summary.Up++
		case "down":
			summary.Down++
		case "degraded":
			summary.Degraded++
		case "maintenance":
		case "stale":
			summary.Stale++
		default:
			summary.Unknown++
		}
	}
	if len(nodeIDs) > 0 {
		var rows []struct{ NodeID string }
		if err := s.db.Model(&db.Incident{}).Select("node_id").Where("node_id IN ? AND status IN ?", nodeIDs, []string{"open", "acknowledged", "covered"}).Group("node_id").Find(&rows).Error; err != nil {
			s.logger.Error("Failed to load node incident summary", "error", err)
			utils.InternalError(c, "Failed to get node summary", err)
			return
		}
		summary.HasIncidents = int64(len(rows))
	}
	utils.SuccessResponse(c, 200, "Node summary retrieved successfully", summary)
}
