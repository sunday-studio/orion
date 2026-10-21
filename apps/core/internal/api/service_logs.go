package api

import (
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"orion/core/internal/utils"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type ServiceLogBatchResponse struct {
	Received int `json:"received"`
	Stored   int `json:"stored"`
}

// receiveNodeLogBatch receives structured service logs from a node.
// @Summary      Receive node service logs
// @Description  Receive and deduplicate a bounded batch of structured Orion Node service logs
// @Tags         service-logs
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           receiveNodeLogBatch
// @Param        node_id  path      string                         true  "Node ID"
// @Param        data      body      service.ServiceLogBatchPayload  true  "Service log batch"
// @Success      200       {object}  utils.APIResponse{data=api.ServiceLogBatchResponse}
// @Failure      400       {object}  utils.APIResponse
// @Failure      401       {object}  utils.APIResponse
// @Failure      500       {object}  utils.APIResponse
// @Router       /v1/nodes/{node_id}/logs/batch [post]
func (s *Server) receiveNodeLogBatch(c *gin.Context) {
	nodeID := c.Param("node_id")
	if nodeID == "" {
		utils.BadRequest(c, "Node ID is required")
		return
	}

	var payload service.ServiceLogBatchPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		s.logger.Error("Invalid service log batch", "error", err, "node_id", nodeID)
		utils.BadRequest(c, "Invalid request payload")
		return
	}

	result, err := s.serviceLogService.StoreNodeLogBatch(nodeID, payload, time.Now().UTC())
	if err != nil {
		if strings.Contains(err.Error(), "max is") || strings.Contains(err.Error(), "timestamp") {
			utils.BadRequest(c, err.Error())
			return
		}
		s.logger.Error("Failed to store service log batch", "error", err, "node_id", nodeID)
		utils.InternalError(c, "Failed to store service log batch", err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "Service logs received successfully", ServiceLogBatchResponse{
		Received: result.Received,
		Stored:   result.Stored,
	})
}

// listServiceLogs retrieves service logs across nodes.
// @Summary      List service logs
// @Description  Get paginated structured service logs shipped by Orion services
// @Tags         service-logs
// @Accept       json
// @Produce      json
// @ID           getServiceLogs
// @Param        limit      query     int     false  "Maximum number of logs to return" default(50)
// @Param        offset     query     int     false  "Number of logs to skip" default(0)
// @Param        node_id   query     string  false  "Filter by node ID"
// @Param        monitor_id query     string  false  "Filter by monitor ID"
// @Param        source     query     string  false  "Filter by service log source"
// @Param        level      query     string  false  "Filter by level"
// @Param        component  query     string  false  "Filter by component"
// @Param        q          query     string  false  "Search message, component, monitor, and related IDs"
// @Success      200        {object}  utils.APIResponse{data=object{logs=[]ServiceLogEntryResponse,count=int64,limit=int,offset=int,pagination=utils.PaginationMeta}}
// @Failure      500        {object}  utils.APIResponse
// @Router       /v1/logs/service [get]
func (s *Server) listServiceLogs(c *gin.Context) {
	s.listServiceLogsWithFilters(c, service.ServiceLogListFilters{
		NodeID:    strings.TrimSpace(c.Query("node_id")),
		MonitorID: strings.TrimSpace(c.Query("monitor_id")),
		Source:    strings.TrimSpace(c.Query("source")),
		Level:     strings.TrimSpace(c.Query("level")),
		Component: strings.TrimSpace(c.Query("component")),
		Search:    strings.TrimSpace(c.Query("q")),
		Limit:     utils.QueryInt(c, "limit", 50),
		Offset:    utils.QueryInt(c, "offset", 0),
	})
}

// listNodeServiceLogs retrieves service logs for one node.
// @Summary      List node service logs
// @Description  Get paginated structured service logs for a specific Orion Node
// @Tags         service-logs
// @Accept       json
// @Produce      json
// @ID           getNodeServiceLogs
// @Param        id         path      string  true   "Node ID"
// @Param        limit      query     int     false  "Maximum number of logs to return" default(50)
// @Param        offset     query     int     false  "Number of logs to skip" default(0)
// @Param        level      query     string  false  "Filter by level"
// @Param        component  query     string  false  "Filter by component"
// @Param        q          query     string  false  "Search message, component, monitor, and related IDs"
// @Success      200        {object}  utils.APIResponse{data=object{logs=[]ServiceLogEntryResponse,count=int64,limit=int,offset=int,pagination=utils.PaginationMeta}}
// @Failure      400        {object}  utils.APIResponse
// @Failure      404        {object}  utils.APIResponse
// @Failure      500        {object}  utils.APIResponse
// @Router       /v1/nodes/{id}/service-logs [get]
func (s *Server) listNodeServiceLogs(c *gin.Context) {
	nodeID := c.Param("id")
	if nodeID == "" {
		utils.BadRequest(c, "Node ID is required")
		return
	}
	if _, err := s.nodeService.GetNode(nodeID); err != nil {
		utils.NotFound(c, "Node not found")
		return
	}

	s.listServiceLogsWithFilters(c, service.ServiceLogListFilters{
		NodeID:    nodeID,
		Level:     strings.TrimSpace(c.Query("level")),
		Component: strings.TrimSpace(c.Query("component")),
		Search:    strings.TrimSpace(c.Query("q")),
		Limit:     utils.QueryInt(c, "limit", 50),
		Offset:    utils.QueryInt(c, "offset", 0),
	})
}

func (s *Server) listServiceLogsWithFilters(c *gin.Context, filters service.ServiceLogListFilters) {
	limit := filters.Limit
	if limit <= 0 {
		limit = 50
	}
	filters.Limit = limit

	result, err := s.serviceLogService.ListServiceLogs(filters)
	if err != nil {
		s.logger.Error("Failed to list service logs", "error", err)
		utils.InternalError(c, "Failed to list service logs", err)
		return
	}

	nodesByID := s.serviceLogNodesByID(result.Entries)
	responses := serviceLogEntryResponses(result.Entries, nodesByID)
	utils.SuccessResponse(c, http.StatusOK, "Service logs retrieved successfully", gin.H{
		"logs":       responses,
		"count":      result.Count,
		"limit":      filters.Limit,
		"offset":     filters.Offset,
		"pagination": utils.NewPaginationMeta(result.Count, filters.Limit, filters.Offset, len(responses)),
	})
}

func (s *Server) serviceLogNodesByID(entries []db.ServiceLogEntry) map[string]db.Node {
	ids := make([]string, 0, len(entries))
	seen := map[string]bool{}
	for _, entry := range entries {
		if entry.NodeID == "" || seen[entry.NodeID] {
			continue
		}
		seen[entry.NodeID] = true
		ids = append(ids, entry.NodeID)
	}
	nodesByID := map[string]db.Node{}
	if len(ids) == 0 {
		return nodesByID
	}

	var nodes []db.Node
	if err := s.db.Where("id IN ?", ids).Find(&nodes).Error; err != nil {
		s.logger.Error("Failed to load service log nodes", "error", err)
		return nodesByID
	}
	for _, node := range nodes {
		nodesByID[node.ID] = node
	}
	return nodesByID
}
