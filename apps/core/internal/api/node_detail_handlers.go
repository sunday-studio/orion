package api

import (
	"orion/core/internal/service"
	"orion/core/internal/utils"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// getNodeDetail retrieves detailed information about a specific node.
// @Summary      Get node details
// @Description  Get detailed information about a specific node including latest report
// @Tags         nodes
// @Accept       json
// @Produce      json
// @ID           getNode
// @Param        id   path      string  true  "Node ID"
// @Success      200  {object}  utils.APIResponse{data=object{node=NodeResponse,latest_report=object}}
// @Failure      400  {object}  utils.APIResponse
// @Failure      404  {object}  utils.APIResponse
// @Router       /v1/nodes/{id} [get]
func (s *Server) getNodeDetail(c *gin.Context) {
	nodeID := c.Param("id")
	if nodeID == "" {
		utils.BadRequest(c, "Node ID is required")
		return
	}
	node, err := s.nodeService.GetNode(nodeID)
	if err != nil {
		s.logger.Error("Failed to get node", "error", err, "node_id", nodeID)
		utils.NotFound(c, "Node not found")
		return
	}
	reports, err := s.reportService.GetNodeReportsById(nodeID, 1, 0)
	if err != nil {
		s.logger.Error("Failed to get node reports", "error", err, "node_id", nodeID)
	}
	var latestReport any
	if len(reports) > 0 {
		latestReport = nodeReportResponse(reports[0])
	}
	utils.SuccessResponse(c, 200, "Node retrieved successfully", gin.H{"node": s.nodeDetailResponse(*node), "latest_report": latestReport})
}

// getNodeHealth retrieves health status for a specific node.
// @Summary      Get node health
// @Description  Get split node availability and monitor rollup health for a specific node
// @Tags         nodes
// @Accept       json
// @Produce      json
// @ID           getNodeHealth
// @Param        id   path      string  true  "Node ID"
// @Success      200  {object}  utils.APIResponse{data=NodeHealthResponse}
// @Failure      400  {object}  utils.APIResponse
// @Failure      404  {object}  utils.APIResponse
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/nodes/{id}/health [get]
func (s *Server) getNodeHealth(c *gin.Context) {
	nodeID := c.Param("id")
	if nodeID == "" {
		utils.BadRequest(c, "Node ID is required")
		return
	}
	if _, err := s.nodeService.GetNode(nodeID); err != nil {
		utils.NotFound(c, "Node not found")
		return
	}
	healthService := service.NewHealthService(s.db, s.logger)
	config := service.DefaultHealthConfig()
	snapshot, err := healthService.ComputeNodeHealthSnapshot(nodeID, config)
	if err != nil {
		s.logger.Error("Failed to compute node health", "error", err, "node_id", nodeID)
		utils.InternalError(c, "Failed to compute node health", err)
		return
	}
	utils.SuccessResponse(c, 200, "Node health retrieved successfully", NodeHealthResponse{NodeID: nodeID, OverallHealth: snapshot.OverallHealth, AvailabilityHealth: snapshot.NodeHealth, MonitorHealth: snapshot.MonitorHealth, StatusReason: snapshot.Reason, UpCount: snapshot.UpCount, DownCount: snapshot.DownCount, DegradedCount: snapshot.DegradedCount, StaleCount: snapshot.StaleCount, UnknownCount: snapshot.UnknownCount, TotalCount: snapshot.TotalCount})
}

// getNodeReports retrieves paginated system reports for a specific node.
// @Summary      Get node reports
// @Description  Get a paginated list of system metric reports for a specific node
// @Tags         nodes
// @Accept       json
// @Produce      json
// @ID           getNodeReports
// @Param        id      path      string  true   "Node ID"
// @Param        limit   query     int     false  "Maximum number of reports to return" default(50)
// @Param        offset  query     int     false  "Number of reports to skip" default(0)
// @Param        since   query     string  false  "Only reports since: 1h, 24h, 7d style duration or RFC3339"
// @Param        until   query     string  false  "Only reports before this RFC3339 time"
// @Success      200     {object}  utils.APIResponse{data=object{reports=[]NodeReportResponse,count=int64,limit=int,offset=int,pagination=utils.PaginationMeta}}
// @Failure      400     {object}  utils.APIResponse
// @Failure      404     {object}  utils.APIResponse
// @Failure      500     {object}  utils.APIResponse
// @Router       /v1/nodes/{id}/reports [get]
func (s *Server) getNodeReports(c *gin.Context) {
	nodeID := c.Param("id")
	if nodeID == "" {
		utils.BadRequest(c, "Node ID is required")
		return
	}
	limit := utils.QueryInt(c, "limit", 50)
	offset := utils.QueryInt(c, "offset", 0)
	if _, err := s.nodeService.GetNode(nodeID); err != nil {
		utils.NotFound(c, "Node not found")
		return
	}
	if limit > 1000 {
		limit = 1000
	}
	var since, until *time.Time
	if raw := strings.TrimSpace(c.Query("since")); raw != "" {
		t, err := parseSinceParam(raw, time.Now().UTC())
		if err != nil {
			utils.BadRequest(c, "since must be a duration like 1h or 7d, or an RFC3339 time")
			return
		}
		since = &t
	}
	if raw := strings.TrimSpace(c.Query("until")); raw != "" {
		t, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			utils.BadRequest(c, "until must be an RFC3339 time")
			return
		}
		until = &t
	}
	reports, count, err := s.reportService.GetNodeReportsInRange(nodeID, since, until, limit, offset)
	if err != nil {
		s.logger.Error("Failed to get node reports", "error", err, "node_id", nodeID)
		utils.InternalError(c, "Failed to get node reports", err)
		return
	}
	responses := nodeReportResponses(reports)
	utils.SuccessResponse(c, 200, "Node reports retrieved successfully", gin.H{"reports": responses, "count": count, "limit": limit, "offset": offset, "pagination": utils.NewPaginationMeta(count, limit, offset, len(responses))})
}

// getNodeUptime returns node uptime over a period.
// @Summary      Get node uptime
// @Description  Returns daily uptime buckets and overall uptime percentage for a node. Monitors are weighted by time, so the up and total of each bucket are seconds (check counts scaled by the check interval of each monitor), and silence counts as downtime.
// @Tags         nodes
// @Produce      json
// @ID           getNodeUptime
// @Param        id      path      string  true   "Node ID"
// @Param        period  query     string  false  "Uptime period such as 7d, 30d, or 90d"
// @Success      200     {object}  utils.APIResponse{data=object{daily_buckets=[]UptimeDayBucketResponse,uptime_percent=number}}
// @Failure      400     {object}  utils.APIResponse
// @Failure      404     {object}  utils.APIResponse
// @Failure      500     {object}  utils.APIResponse
// @Router       /v1/nodes/{id}/uptime [get]
func (s *Server) getNodeUptime(c *gin.Context) {
	nodeID := c.Param("id")
	if nodeID == "" {
		utils.BadRequest(c, "Node ID is required")
		return
	}
	period := c.DefaultQuery("period", "90d")
	if _, err := s.nodeService.GetNode(nodeID); err != nil {
		utils.NotFound(c, "Node not found")
		return
	}
	result, err := s.reportService.GetNodeUptime(nodeID, period)
	if err != nil {
		s.logger.Error("Failed to get node uptime", "error", err, "node_id", nodeID)
		utils.InternalError(c, "Failed to get node uptime", err)
		return
	}
	utils.SuccessResponse(c, 200, "Node uptime retrieved successfully", gin.H{"daily_buckets": result.DailyBuckets, "uptime_percent": result.UptimePercent})
}
