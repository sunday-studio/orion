package api

import (
	"orion/core/internal/db"
	"orion/core/internal/service"
	"orion/core/internal/utils"

	"github.com/gin-gonic/gin"
)

// getSystemHealth retrieves overall system health summary
// @Summary      Get system health summary
// @Description  Get overall system health summary including agent and monitor counts
// @Tags         health
// @Accept       json
// @Produce      json
// @ID           getHealthSummary
// @Success      200  {object}  utils.APIResponse{data=object{overall_health=string,agents=object{total=int},monitors=object{total=int,up=int,down=int,degraded=int,unknown=int}}}
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/health/summary [get]
func (s *Server) getSystemHealth(c *gin.Context) {
	healthService := service.NewHealthService(s.db, s.logger)

	// Get all active agents
	var agents []db.Agent
	if err := s.db.Where("deleted_at IS NULL").Find(&agents).Error; err != nil {
		s.logger.Error("Failed to get agents", "error", err)
		utils.InternalError(c, "Failed to get system health", err)
		return
	}

	totalAgents := len(agents)
	totalMonitors := 0
	upCount := 0
	downCount := 0
	degradedCount := 0
	unknownCount := 0

	// Get all active monitors
	var monitors []db.Monitor
	if err := s.db.Where("lifecycle = ?", "active").Find(&monitors).Error; err != nil {
		s.logger.Error("Failed to get monitors", "error", err)
		utils.InternalError(c, "Failed to get system health", err)
		return
	}

	totalMonitors = len(monitors)

	// Health per monitor is the stored value on the loaded rows (refreshed on ingest and in the background)
	for _, monitor := range monitors {
		switch healthService.CachedHealth(monitor) {
		case "up":
			upCount++
		case "down":
			downCount++
		case "degraded":
			degradedCount++
		default:
			unknownCount++
		}
	}

	// Determine overall system health
	var overallHealth string
	if downCount > 0 {
		overallHealth = "down"
	} else if degradedCount > 0 {
		overallHealth = "degraded"
	} else if unknownCount > 0 {
		overallHealth = "unknown"
	} else {
		overallHealth = "up"
	}

	utils.SuccessResponse(c, 200, "System health retrieved successfully", gin.H{
		"overall_health": overallHealth,
		"agents": gin.H{
			"total": totalAgents,
		},
		"monitors": gin.H{
			"total":    totalMonitors,
			"up":       upCount,
			"down":     downCount,
			"degraded": degradedCount,
			"unknown":  unknownCount,
		},
	})
}

// getHealthIssues retrieves all health issues in the system
// @Summary      Get health issues
// @Description  Get a list of all monitors with health issues (down, degraded, or stale)
// @Tags         health
// @Accept       json
// @Produce      json
// @ID           getHealthIssues
// @Success      200  {object}  utils.APIResponse{data=object{issues=[]object,count=int}}
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/health/issues [get]
func (s *Server) getHealthIssues(c *gin.Context) {
	healthService := service.NewHealthService(s.db, s.logger)
	config := service.DefaultHealthConfig()

	// Get all active monitors
	var monitors []db.Monitor
	if err := s.db.Where("lifecycle = ?", "active").Find(&monitors).Error; err != nil {
		s.logger.Error("Failed to get monitors", "error", err)
		utils.InternalError(c, "Failed to get health issues", err)
		return
	}

	var issues []gin.H
	agents := s.agentsByID(monitorAgentIDs(monitors))

	// Check each monitor for issues
	for _, monitor := range monitors {
		computedHealth := healthService.CachedHealth(monitor)

		if computedHealth == "down" || computedHealth == "degraded" {
			agent, ok := agents[monitor.AgentID]
			if !ok {
				continue
			}

			issues = append(issues, gin.H{
				"monitor_id":   monitor.ID,
				"monitor_name": monitor.Name,
				"monitor_type": monitor.Type,
				"health":       computedHealth,
				"agent_id":     monitor.AgentID,
				"agent_name":   agent.Name,
			})
		}
	}

	// Check for stale monitors
	staleMonitors, err := healthService.DetectStaleMonitors(config)
	if err != nil {
		s.logger.Error("Failed to detect stale monitors", "error", err)
	} else {
		for _, monitor := range staleMonitors {
			agent, ok := agents[monitor.AgentID]
			if !ok {
				continue
			}

			issues = append(issues, gin.H{
				"monitor_id":   monitor.ID,
				"monitor_name": monitor.Name,
				"monitor_type": monitor.Type,
				"health":       "unknown",
				"issue_type":   "stale_data",
				"agent_id":     monitor.AgentID,
				"agent_name":   agent.Name,
			})
		}
	}

	utils.SuccessResponse(c, 200, "Health issues retrieved successfully", gin.H{
		"issues": issues,
		"count":  len(issues),
	})
}

// getIncidentCandidates retrieves monitors that are candidates for incidents
// @Summary      Get incident candidates
// @Description  Get a list of monitors that are candidates for incidents based on health status and recent reports
// @Tags         health
// @Accept       json
// @Produce      json
// @ID           getIncidentsCandidates
// @Success      200  {object}  utils.APIResponse{data=object{candidates=[]object,count=int}}
// @Failure      500  {object}  utils.APIResponse
// @Router       /v1/incidents/candidates [get]
func (s *Server) getIncidentCandidates(c *gin.Context) {
	healthService := service.NewHealthService(s.db, s.logger)

	// Get all active monitors
	var monitors []db.Monitor
	if err := s.db.Where("lifecycle = ?", "active").Find(&monitors).Error; err != nil {
		s.logger.Error("Failed to get monitors", "error", err)
		utils.InternalError(c, "Failed to get incident candidates", err)
		return
	}

	var candidates []gin.H

	// Only down/degraded monitors need agent and recent-report lookups; fetch them in bulk.
	var flagged []db.Monitor
	for _, monitor := range monitors {
		if h := healthService.CachedHealth(monitor); h == "down" || h == "degraded" {
			flagged = append(flagged, monitor)
		}
	}
	agents := s.agentsByID(monitorAgentIDs(flagged))
	recent := s.recentReportsByMonitor(flagged, 5)

	for _, monitor := range flagged {
		agent, ok := agents[monitor.AgentID]
		if !ok {
			continue
		}

		downCount := 0
		for _, report := range recent[monitor.ID] {
			if report.Health == "down" {
				downCount++
			}
		}

		severity := "low"
		if downCount >= 3 {
			severity = "high"
		} else if downCount >= 1 {
			severity = "medium"
		}

		candidates = append(candidates, gin.H{
			"monitor_id":   monitor.ID,
			"monitor_name": monitor.Name,
			"monitor_type": monitor.Type,
			"health":       healthService.CachedHealth(monitor),
			"severity":     severity,
			"agent_id":     monitor.AgentID,
			"agent_name":   agent.Name,
			"down_count":   downCount,
		})
	}

	utils.SuccessResponse(c, 200, "Incident candidates retrieved successfully", gin.H{
		"candidates": candidates,
		"count":      len(candidates),
	})
}

func monitorAgentIDs(monitors []db.Monitor) []string {
	seen := make(map[string]struct{}, len(monitors))
	ids := make([]string, 0, len(monitors))
	for _, m := range monitors {
		if _, ok := seen[m.AgentID]; !ok {
			seen[m.AgentID] = struct{}{}
			ids = append(ids, m.AgentID)
		}
	}
	return ids
}

// agentsByID loads the given agents in one query.
func (s *Server) agentsByID(ids []string) map[string]db.Agent {
	out := make(map[string]db.Agent, len(ids))
	if len(ids) == 0 {
		return out
	}
	var agents []db.Agent
	if err := s.db.Where("id IN ?", ids).Find(&agents).Error; err != nil {
		s.logger.Error("Failed to load agents", "error", err)
		return out
	}
	for _, a := range agents {
		out[a.ID] = a
	}
	return out
}

// recentReportsByMonitor returns the newest n reports per monitor in one query.
func (s *Server) recentReportsByMonitor(monitors []db.Monitor, n int) map[string][]db.MonitorReport {
	out := make(map[string][]db.MonitorReport, len(monitors))
	if len(monitors) == 0 {
		return out
	}
	ids := make([]string, len(monitors))
	for i, m := range monitors {
		ids[i] = m.ID
	}
	var reports []db.MonitorReport
	err := s.db.Raw(`
		SELECT * FROM (
			SELECT *, ROW_NUMBER() OVER (PARTITION BY monitor_id ORDER BY created_at DESC) AS rn
			FROM monitor_reports WHERE monitor_id IN ?
		) WHERE rn <= ?`, ids, n).Scan(&reports).Error
	if err != nil {
		s.logger.Error("Failed to load recent reports", "error", err)
		return out
	}
	for _, r := range reports {
		out[r.MonitorID] = append(out[r.MonitorID], r)
	}
	return out
}
