package api

import (
	"orion/core/internal/db"
	"orion/core/internal/service"
	"orion/core/internal/utils"

	"github.com/gin-gonic/gin"
)

// registerMonitor registers a new monitor for an agent
// @Summary      Register a monitor
// @Description  Register a new monitor for a specific agent
// @Tags         monitors
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           registerMonitor
// @Param        agent_id  path      string                      true  "Agent ID"
// @Param        request   body      service.RegisterMonitorRequest  true  "Monitor registration request"
// @Success      200       {object}  utils.APIResponse{data=service.RegisterMonitorResponse}
// @Failure      400       {object}  utils.APIResponse
// @Failure      401       {object}  utils.APIResponse
// @Failure      500       {object}  utils.APIResponse
// @Router       /v1/agents/{agent_id}/register-monitor [post]
func (s *Server) registerMonitor(c *gin.Context) {
	var req service.RegisterMonitorRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		s.logger.Error("Invalid register monitor request", "error", err)
		utils.BadRequest(c, "Invalid request payload")
		return
	}

	agentID := c.Param("agent_id")
	if agentID == "" {
		utils.BadRequest(c, "Agent ID is required")
		return
	}
	if req.AgentID != "" && req.AgentID != agentID {
		utils.BadRequest(c, "Request agent_id must match route agent_id")
		return
	}
	req.AgentID = agentID

	response, err := s.monitorService.RegisterMonitor(&req)
	if err != nil {
		s.logger.Error("Failed to register monitor", "error", err)
		utils.InternalError(c, "Failed to register monitor", err)
		return
	}

	utils.SuccessResponse(c, 200, "Monitor registered successfully", response)
}

// unregisterMonitor unregisters a monitor for an agent
// @Summary      Unregister a monitor
// @Description  Unregister a monitor for a specific agent
// @Tags         monitors
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           unregisterMonitor
// @Param        agent_id   path      string                        true  "Agent ID"
// @Param        request    body      service.UnregisterMonitorRequest true  "Monitor unregistration request"
// @Success      200        {object}  utils.APIResponse{data=service.UnregisterMonitorResponse}
// @Failure      400        {object}  utils.APIResponse
// @Failure      401        {object}  utils.APIResponse
// @Failure      500        {object}  utils.APIResponse
// @Router       /v1/agents/{agent_id}/unregister-monitor [post]
func (s *Server) unregisterMonitor(c *gin.Context) {
	var req service.UnregisterMonitorRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		s.logger.Error("Invalid unregister monitor request", "error", err)
		utils.BadRequest(c, "Invalid request payload")
		return
	}

	agentID := c.Param("agent_id")
	if agentID == "" {
		utils.BadRequest(c, "Agent ID is required")
		return
	}
	if req.AgentID != "" && req.AgentID != agentID {
		utils.BadRequest(c, "Request agent_id must match route agent_id")
		return
	}
	req.AgentID = agentID

	response, err := s.monitorService.UnregisterMonitor(&req)
	if err != nil {
		s.logger.Error("Failed to unregister monitor", "error", err)
		utils.InternalError(c, "Failed to unregister monitor", err)
		return
	}

	utils.SuccessResponse(c, 200, "Monitor unregistered successfully", response)
}

// listMonitors retrieves a paginated list of monitors for an agent
// @Summary      List monitors for an agent
// @Description  Get a paginated list of monitors for a specific agent with optional filters
// @Tags         monitors
// @Accept       json
// @Produce      json
// @ID           getAgentMonitors
// @Param        id         path      string  true   "Agent ID"
// @Param        health     query     string  false  "Filter by health status (up|down|degraded|unknown)"
// @Param        lifecycle  query     string  false  "Filter by lifecycle status (active|disabled|deleted)"
// @Param        limit      query     int     false  "Maximum number of monitors to return" default(50)
// @Param        offset     query     int     false  "Number of monitors to skip" default(0)
// @Success      200        {object}  utils.APIResponse{data=object{monitors=[]MonitorResponse,count=int64,limit=int,offset=int,pagination=utils.PaginationMeta}}
// @Failure      400        {object}  utils.APIResponse
// @Failure      404        {object}  utils.APIResponse
// @Failure      500        {object}  utils.APIResponse
// @Router       /v1/agents/{id}/monitors [get]
func (s *Server) listMonitors(c *gin.Context) {
	agentID := c.Param("id")
	if agentID == "" {
		utils.BadRequest(c, "Agent ID is required")
		return
	}

	healthFilter := c.Query("health")
	lifecycleFilter := c.Query("lifecycle")

	limit := queryInt(c, "limit", 50)
	offset := queryInt(c, "offset", 0)

	if _, err := s.agentService.GetAgent(agentID); err != nil {
		utils.NotFound(c, "Agent not found")
		return
	}

	monitors, err := s.monitorService.ListMonitors(agentID, healthFilter, lifecycleFilter, limit, offset)
	if err != nil {
		s.logger.Error("Failed to list monitors", "error", err, "agent_id", agentID)
		utils.InternalError(c, "Failed to list monitors", err)
		return
	}

	count, err := s.monitorService.GetMonitorCount(agentID, healthFilter, lifecycleFilter)
	if err != nil {
		s.logger.Error("Failed to get monitor count", "error", err, "agent_id", agentID)
		// Don't fail the request if count fails
	}

	responses := monitorResponses(monitors)
	utils.SuccessResponse(c, 200, "Monitors retrieved successfully", gin.H{
		"monitors":   responses,
		"count":      count,
		"limit":      limit,
		"offset":     offset,
		"pagination": utils.NewPaginationMeta(count, limit, offset, len(responses)),
	})
}

// listAllMonitors retrieves a paginated list of monitors across agents.
// @Summary      List monitors
// @Description  Get a paginated list of all monitors with optional filters
// @Tags         monitors
// @Accept       json
// @Produce      json
// @ID           getMonitors
// @Param        limit          query     int     false  "Maximum number of monitors to return" default(50)
// @Param        offset         query     int     false  "Number of monitors to skip" default(0)
// @Param        search         query     string  false  "Search by monitor, agent, or type"
// @Param        health         query     string  false  "Filter by health status (up|down|degraded|unknown)"
// @Param        type           query     string  false  "Filter by monitor type"
// @Param        lifecycle      query     string  false  "Filter by lifecycle status (active|disabled|deleted)"
// @Param        stale_only     query     bool    false  "Only return stale monitors"
// @Param        has_incidents  query     bool    false  "Only return monitors with active incidents"
// @Param        sort           query     string  false  "Sort column" default(updated_at)
// @Param        order          query     string  false  "Sort order (asc|desc)" default(desc)
// @Success      200            {object}  utils.APIResponse{data=object{monitors=[]MonitorResponse,count=int64,limit=int,offset=int,pagination=utils.PaginationMeta}}
// @Failure      500            {object}  utils.APIResponse
// @Router       /v1/monitors [get]
func (s *Server) listAllMonitors(c *gin.Context) {
	limit := queryInt(c, "limit", 50)
	offset := queryInt(c, "offset", 0)

	monitors, count, err := s.monitorService.ListAllMonitors(service.ListAllMonitorsOpts{
		Limit:        limit,
		Offset:       offset,
		Search:       c.Query("search"),
		Health:       c.Query("health"),
		Type:         c.Query("type"),
		Lifecycle:    c.Query("lifecycle"),
		StaleOnly:    c.Query("stale_only") == "true",
		HasIncidents: c.Query("has_incidents") == "true",
		Sort:         c.DefaultQuery("sort", "updated_at"),
		Order:        c.DefaultQuery("order", "desc"),
	})
	if err != nil {
		s.logger.Error("Failed to list all monitors", "error", err)
		utils.InternalError(c, "Failed to list monitors", err)
		return
	}

	responses := monitorResponsesWithAgents(monitors, s.monitorAgentsByID(monitors))
	utils.SuccessResponse(c, 200, "Monitors retrieved successfully", gin.H{
		"monitors":   responses,
		"count":      count,
		"limit":      limit,
		"offset":     offset,
		"pagination": utils.NewPaginationMeta(count, limit, offset, len(responses)),
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

func (s *Server) monitorAgentsByID(monitors []db.Monitor) map[string]db.Agent {
	agentIDs := make([]string, 0, len(monitors))
	seen := make(map[string]struct{}, len(monitors))
	for _, monitor := range monitors {
		if monitor.AgentID == "" {
			continue
		}
		if _, ok := seen[monitor.AgentID]; ok {
			continue
		}
		seen[monitor.AgentID] = struct{}{}
		agentIDs = append(agentIDs, monitor.AgentID)
	}

	agentsByID := make(map[string]db.Agent, len(agentIDs))
	if len(agentIDs) == 0 {
		return agentsByID
	}

	var agents []db.Agent
	if err := s.db.Where("id IN ?", agentIDs).Find(&agents).Error; err != nil {
		s.logger.Error("Failed to load monitor agents", "error", err)
		return agentsByID
	}
	for _, agent := range agents {
		agentsByID[agent.ID] = agent
	}

	return agentsByID
}

// getMonitorDetail retrieves detailed information about a specific monitor
// @Summary      Get monitor details
// @Description  Get detailed information about a specific monitor including recent reports and computed health
// @Tags         monitors
// @Accept       json
// @Produce      json
// @ID           getMonitor
// @Param        id   path      string  true  "Monitor ID"
// @Success      200  {object}  utils.APIResponse{data=object{monitor=MonitorResponse,recent_reports=[]MonitorReportResponse,computed_health=string}}
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
	if agent, err := s.agentService.GetAgent(monitor.AgentID); err == nil {
		response.AgentName = agent.Name
	} else {
		s.logger.Error("Failed to load monitor agent", "error", err, "monitor_id", monitorID, "agent_id", monitor.AgentID)
	}

	utils.SuccessResponse(c, 200, "Monitor retrieved successfully", gin.H{
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

	limit := queryInt(c, "limit", 50)
	offset := queryInt(c, "offset", 0)

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
