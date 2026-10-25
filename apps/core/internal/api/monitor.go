package api

import (
	"errors"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"orion/core/internal/utils"

	"github.com/gin-gonic/gin"
)

// registerMonitor registers a new monitor for a node
// @Summary      Register a monitor
// @Description  Register a new monitor for a specific node
// @Tags         monitors
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           registerMonitor
// @Param        node_id  path      string                      true  "Node ID"
// @Param        request   body      service.RegisterMonitorRequest  true  "Monitor registration request"
// @Success      200       {object}  utils.APIResponse{data=service.RegisterMonitorResponse}
// @Failure      400       {object}  utils.APIResponse
// @Failure      401       {object}  utils.APIResponse
// @Failure      500       {object}  utils.APIResponse
// @Router       /v1/nodes/{node_id}/register-monitor [post]
func (s *Server) registerMonitor(c *gin.Context) {
	var req service.RegisterMonitorRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		s.logger.Error("Invalid register monitor request", "error", err)
		utils.BadRequest(c, "Invalid request payload")
		return
	}

	nodeID := c.Param("node_id")
	if nodeID == "" {
		utils.BadRequest(c, "Node ID is required")
		return
	}
	if req.NodeID != "" && req.NodeID != nodeID {
		utils.BadRequest(c, "Request node_id must match route node_id")
		return
	}
	req.NodeID = nodeID

	response, err := s.monitorService.RegisterMonitor(&req)
	if err != nil {
		s.logger.Error("Failed to register monitor", "error", err)
		utils.InternalError(c, "Failed to register monitor", err)
		return
	}

	utils.SuccessResponse(c, 200, "Monitor registered successfully", response)
}

// unregisterMonitor unregisters a monitor for a node
// @Summary      Unregister a monitor
// @Description  Unregister a monitor for a specific node
// @Tags         monitors
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           unregisterMonitor
// @Param        node_id   path      string                        true  "Node ID"
// @Param        request    body      service.UnregisterMonitorRequest true  "Monitor unregistration request"
// @Success      200        {object}  utils.APIResponse{data=service.UnregisterMonitorResponse}
// @Failure      400        {object}  utils.APIResponse
// @Failure      401        {object}  utils.APIResponse
// @Failure      404        {object}  utils.APIResponse
// @Failure      500        {object}  utils.APIResponse
// @Router       /v1/nodes/{node_id}/unregister-monitor [post]
func (s *Server) unregisterMonitor(c *gin.Context) {
	var req service.UnregisterMonitorRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		s.logger.Error("Invalid unregister monitor request", "error", err)
		utils.BadRequest(c, "Invalid request payload")
		return
	}

	nodeID := c.Param("node_id")
	if nodeID == "" {
		utils.BadRequest(c, "Node ID is required")
		return
	}
	if req.NodeID != "" && req.NodeID != nodeID {
		utils.BadRequest(c, "Request node_id must match route node_id")
		return
	}
	req.NodeID = nodeID

	response, err := s.monitorService.UnregisterMonitor(&req)
	if errors.Is(err, service.ErrMonitorNotOwned) {
		utils.NotFound(c, "Monitor not found")
		return
	}
	if err != nil {
		s.logger.Error("Failed to unregister monitor", "error", err)
		utils.InternalError(c, "Failed to unregister monitor", err)
		return
	}
	if err := service.NewIncidentService(s.db, s.logger, s.cfg).ResolveMonitorRemoved(req.MonitorID); err != nil {
		s.logger.Error("Failed to resolve monitor incidents after unregister", "error", err, "monitor_id", req.MonitorID)
		utils.InternalError(c, "Failed to unregister monitor", err)
		return
	}

	utils.SuccessResponse(c, 200, "Monitor unregistered successfully", response)
}

// listMonitors retrieves a paginated list of monitors for a node
// @Summary      List monitors for a node
// @Description  Get a paginated list of monitors for a specific node with optional filters
// @Tags         monitors
// @Accept       json
// @Produce      json
// @ID           getNodeMonitors
// @Param        id         path      string  true   "Node ID"
// @Param        health     query     string  false  "Filter by health status (up|down|degraded|unknown)"
// @Param        lifecycle  query     string  false  "Filter by lifecycle status (active|disabled|deleted)"
// @Param        status     query     string  false  "Comma-separated computed statuses to show: up,down,degraded,stale,unknown. total and hidden_count report what the filter hides"
// @Param        limit      query     int     false  "Maximum number of monitors to return" default(50)
// @Param        offset     query     int     false  "Number of monitors to skip" default(0)
// @Success      200        {object}  utils.APIResponse{data=object{monitors=[]MonitorResponse,count=int64,total=int64,hidden_count=int64,limit=int,offset=int,pagination=utils.PaginationMeta}}
// @Failure      400        {object}  utils.APIResponse
// @Failure      404        {object}  utils.APIResponse
// @Failure      500        {object}  utils.APIResponse
// @Router       /v1/nodes/{id}/monitors [get]
func (s *Server) listMonitors(c *gin.Context) {
	nodeID := c.Param("id")
	if nodeID == "" {
		utils.BadRequest(c, "Node ID is required")
		return
	}

	healthFilter := c.Query("health")
	lifecycleFilter := c.Query("lifecycle")

	limit := utils.QueryInt(c, "limit", 50)
	offset := utils.QueryInt(c, "offset", 0)

	if _, err := s.nodeService.GetNode(nodeID); err != nil {
		utils.NotFound(c, "Node not found")
		return
	}

	statusSet := parseStatusSet(c.Query("status"))
	var monitors []db.Monitor
	var count, total int64
	var err error
	if len(statusSet) > 0 {
		// Multi-status filter (down,degraded,stale,up,unknown): filter on computed health, then page.
		all, listErr := s.monitorService.ListMonitors(nodeID, healthFilter, lifecycleFilter, 5000, 0)
		if listErr != nil {
			s.logger.Error("Failed to list monitors", "error", listErr, "node_id", nodeID)
			utils.InternalError(c, "Failed to list monitors", listErr)
			return
		}
		total = int64(len(all))
		matched := all[:0:0]
		for _, m := range all {
			if statusSet[effectiveMonitorHealth(m)] {
				matched = append(matched, m)
			}
		}
		count = int64(len(matched))
		monitors = matched[min(offset, len(matched)):min(offset+limit, len(matched))]
	} else {
		monitors, err = s.monitorService.ListMonitors(nodeID, healthFilter, lifecycleFilter, limit, offset)
		if err != nil {
			s.logger.Error("Failed to list monitors", "error", err, "node_id", nodeID)
			utils.InternalError(c, "Failed to list monitors", err)
			return
		}
		count, err = s.monitorService.GetMonitorCount(nodeID, healthFilter, lifecycleFilter)
		if err != nil {
			s.logger.Error("Failed to get monitor count", "error", err, "node_id", nodeID)
		}
		total = count
	}

	responses := monitorResponses(monitors)
	for i := range responses {
		responses[i].StatusText, responses[i].StatusDetail = s.monitorStatusLine(monitors[i], effectiveMonitorHealth(monitors[i]))
	}
	utils.SuccessResponse(c, 200, "Monitors retrieved successfully", gin.H{
		"monitors":     responses,
		"count":        count,
		"total":        total,
		"hidden_count": total - count,
		"limit":        limit,
		"offset":       offset,
		"pagination":   utils.NewPaginationMeta(count, limit, offset, len(responses)),
	})
}

// getMonitorSummary retrieves aggregate counts for the monitor list.
// @Summary      Get monitor summary
// @Description  Get aggregate counts for active monitors by health, stale, and incident status
// @Tags         monitors
// @Accept       json
// @Produce      json
// @ID           getMonitorSummary
// @Success      200  {object}  utils.APIResponse{data=service.MonitorSummary}
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/monitors/summary [get]
func (s *Server) getMonitorSummary(c *gin.Context) {
	summary, err := s.monitorService.GetMonitorSummary()
	if err != nil {
		s.logger.Error("Failed to get monitor summary", "error", err)
		utils.InternalError(c, "Failed to get monitor summary", err)
		return
	}

	utils.SuccessResponse(c, 200, "Monitor summary retrieved successfully", summary)
}

func (s *Server) monitorNodesByID(monitors []db.Monitor) map[string]db.Node {
	nodeIDs := make([]string, 0, len(monitors))
	seen := make(map[string]struct{}, len(monitors))
	for _, monitor := range monitors {
		if monitor.NodeID == "" {
			continue
		}
		if _, ok := seen[monitor.NodeID]; ok {
			continue
		}
		seen[monitor.NodeID] = struct{}{}
		nodeIDs = append(nodeIDs, monitor.NodeID)
	}

	nodesByID := make(map[string]db.Node, len(nodeIDs))
	if len(nodeIDs) == 0 {
		return nodesByID
	}

	var nodes []db.Node
	if err := s.db.Where("id IN ?", nodeIDs).Find(&nodes).Error; err != nil {
		s.logger.Error("Failed to load monitor nodes", "error", err)
		return nodesByID
	}
	for _, node := range nodes {
		nodesByID[node.ID] = node
	}

	return nodesByID
}

func (s *Server) coreMonitorIDsByID(monitors []db.Monitor) map[string]struct{} {
	monitorIDs := make([]string, 0, len(monitors))
	seen := make(map[string]struct{}, len(monitors))
	for _, monitor := range monitors {
		if monitor.ID == "" {
			continue
		}
		if _, ok := seen[monitor.ID]; ok {
			continue
		}
		seen[monitor.ID] = struct{}{}
		monitorIDs = append(monitorIDs, monitor.ID)
	}

	coreIDs := make(map[string]struct{}, len(monitorIDs))
	if len(monitorIDs) == 0 {
		return coreIDs
	}

	var configs []db.CoreMonitorConfig
	if err := s.db.Select("monitor_id").Where("monitor_id IN ?", monitorIDs).Find(&configs).Error; err != nil {
		s.logger.Error("Failed to load core monitor configs", "error", err)
		return coreIDs
	}
	for _, config := range configs {
		coreIDs[config.MonitorID] = struct{}{}
	}
	return coreIDs
}

// getMonitorDetail retrieves detailed information about a specific monitor
// @Summary      Get monitor details
// @Description  Get detailed information about a specific monitor including recent reports and computed health
// @Tags         monitors
// @Accept       json
// @Produce      json
// @ID           getMonitor
// @Param        id   path      string  true  "Monitor ID"
// @Success      200  {object}  utils.APIResponse{data=object{monitor=MonitorResponse,summary=MonitorDetailSummary,recent_reports=[]MonitorReportResponse,computed_health=string}}
// @Failure      400  {object}  utils.APIResponse
// @Failure      404  {object}  utils.APIResponse
// @Router       /v1/monitors/{id} [get]
func (s *Server) getMonitorDetail(c *gin.Context) {
	monitorID := c.Param("id")
	if monitorID == "" {
		utils.BadRequest(c, "Monitor ID is required")
		return
	}

	monitor, err := s.monitorService.GetMonitor(monitorID)
	if err != nil {
		s.logger.Error("Failed to get monitor", "error", err, "monitor_id", monitorID)
		utils.NotFound(c, "Monitor not found")
		return
	}

	// Get recent reports
	reports, err := s.reportService.GetMonitorReports(monitorID, 10, 0)
	if err != nil {
		s.logger.Error("Failed to get monitor reports", "error", err, "monitor_id", monitorID)
		// Don't fail if reports can't be retrieved
	}

	// Compute derived health
	healthService := service.NewHealthService(s.db, s.logger)
	config := service.DefaultHealthConfig()
	computedHealth := monitor.Health
	if monitor.Health != "stale" {
		var err error
		computedHealth, err = healthService.ComputeMonitorHealth(monitorID, config)
		if err != nil {
			s.logger.Error("Failed to compute monitor health", "error", err, "monitor_id", monitorID)
			// Don't fail if health computation fails
			computedHealth = monitor.Health
		}
	}
	monitor.ComputedHealth = computedHealth
	response := monitorResponse(*monitor)
	if node, err := s.nodeService.GetNode(monitor.NodeID); err == nil {
		response.NodeName = node.Name
		response.OwnerName = node.Name
	} else {
		s.logger.Error("Failed to load monitor node", "error", err, "monitor_id", monitorID, "node_id", monitor.NodeID)
	}
	if _, ok := s.coreMonitorIDsByID([]db.Monitor{*monitor})[monitor.ID]; ok {
		response.OwnerKind = "core"
		response.Source = "core"
	}

	rowsForExtras := []MonitorResponse{response}
	s.enrichMonitorRows(rowsForExtras, []db.Monitor{*monitor})
	response = rowsForExtras[0]
	utils.SuccessResponse(c, 200, "Monitor retrieved successfully", gin.H{
		"summary":         s.monitorDetailSummary(c, *monitor, computedHealth),
		"monitor":         response,
		"recent_reports":  monitorReportResponses(reports),
		"computed_health": computedHealth,
	})
}

// getMonitorHistory retrieves the report history for a specific monitor
// @Summary      Get monitor history
// @Description  Get a paginated list of reports for a specific monitor
// @Tags         monitors
// @Accept       json
// @Produce      json
// @ID           getMonitorHistory
// @Param        id      path      string  true   "Monitor ID"
// @Param        limit   query     int     false  "Maximum number of reports to return" default(50)
// @Param        offset  query     int     false  "Number of reports to skip" default(0)
// @Success      200     {object}  utils.APIResponse{data=object{reports=[]MonitorReportResponse,count=int64,limit=int,offset=int,pagination=utils.PaginationMeta}}
// @Failure      400     {object}  utils.APIResponse
// @Failure      404     {object}  utils.APIResponse
// @Failure      500     {object}  utils.APIResponse
// @Router       /v1/monitors/{id}/history [get]
func (s *Server) getMonitorHistory(c *gin.Context) {
	monitorID := c.Param("id")
	if monitorID == "" {
		utils.BadRequest(c, "Monitor ID is required")
		return
	}

	limit := utils.QueryInt(c, "limit", 50)
	offset := utils.QueryInt(c, "offset", 0)

	if _, err := s.monitorService.GetMonitor(monitorID); err != nil {
		utils.NotFound(c, "Monitor not found")
		return
	}

	reports, err := s.reportService.GetMonitorReports(monitorID, limit, offset)
	if err != nil {
		s.logger.Error("Failed to get monitor history", "error", err, "monitor_id", monitorID)
		utils.InternalError(c, "Failed to get monitor history", err)
		return
	}

	count, err := s.reportService.GetMonitorReportCount(monitorID)
	if err != nil {
		s.logger.Error("Failed to get monitor report count", "error", err, "monitor_id", monitorID)
		// Don't fail the request if count fails
	}

	responses := monitorReportResponses(reports)
	utils.SuccessResponse(c, 200, "Monitor history retrieved successfully", gin.H{
		"reports":    responses,
		"count":      count,
		"limit":      limit,
		"offset":     offset,
		"pagination": utils.NewPaginationMeta(count, limit, offset, len(responses)),
	})
}

// getMonitorUptime returns monitor uptime over a period.
// @Summary      Get monitor uptime
// @Description  Returns daily uptime buckets and overall uptime percentage for a monitor.
// @Tags         monitors
// @Produce      json
// @ID           getMonitorUptime
// @Param        id      path      string  true   "Monitor ID"
// @Param        period  query     string  false  "Uptime period such as 7d, 30d, or 90d"
// @Success      200     {object}  object{daily_buckets=[]UptimeDayBucketResponse,uptime_percent=number}
// @Failure      400     {object}  utils.APIResponse
// @Failure      404     {object}  utils.APIResponse
// @Failure      500     {object}  utils.APIResponse
// @Router       /v1/monitors/{id}/uptime [get]
func (s *Server) getMonitorUptime(c *gin.Context) {
	monitorID := c.Param("id")
	if monitorID == "" {
		utils.BadRequest(c, "Monitor ID is required")
		return
	}

	period := c.DefaultQuery("period", "90d")

	// Verify monitor exists
	if _, err := s.monitorService.GetMonitor(monitorID); err != nil {
		utils.NotFound(c, "Monitor not found")
		return
	}

	result, err := s.reportService.GetMonitorUptime(monitorID, period)
	if err != nil {
		s.logger.Error("Failed to get monitor uptime", "error", err, "monitor_id", monitorID)
		utils.InternalError(c, "Failed to get monitor uptime", err)
		return
	}

	utils.SuccessResponse(c, 200, "Monitor uptime retrieved successfully", gin.H{
		"daily_buckets":  result.DailyBuckets,
		"uptime_percent": result.UptimePercent,
	})
}
