package api

import (
	"orion/core/internal/db"
	"orion/core/internal/service"
	"orion/core/internal/utils"

	"github.com/gin-gonic/gin"
)

func (s *Server) getSystemHealth(c *gin.Context) {
	healthService := service.NewHealthService(s.db, s.logger)
	config := service.DefaultHealthConfig()

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

	// Compute health for each monitor
	for _, monitor := range monitors {
		computedHealth, err := healthService.ComputeMonitorHealth(monitor.ID, config)
		if err != nil {
			unknownCount++
			continue
		}

		switch computedHealth {
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

	// Check each monitor for issues
	for _, monitor := range monitors {
		computedHealth, err := healthService.ComputeMonitorHealth(monitor.ID, config)
		if err != nil {
			continue
		}

		if computedHealth == "down" || computedHealth == "degraded" {
			// Get agent info
			var agent db.Agent
			if err := s.db.Where("id = ?", monitor.AgentID).First(&agent).Error; err != nil {
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
			var agent db.Agent
			if err := s.db.Where("id = ?", monitor.AgentID).First(&agent).Error; err != nil {
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

func (s *Server) getIncidentCandidates(c *gin.Context) {
	healthService := service.NewHealthService(s.db, s.logger)
	config := service.DefaultHealthConfig()

	// Get all active monitors
	var monitors []db.Monitor
	if err := s.db.Where("lifecycle = ?", "active").Find(&monitors).Error; err != nil {
		s.logger.Error("Failed to get monitors", "error", err)
		utils.InternalError(c, "Failed to get incident candidates", err)
		return
	}

	var candidates []gin.H

	// Check each monitor for incident candidates (down or degraded)
	for _, monitor := range monitors {
		computedHealth, err := healthService.ComputeMonitorHealth(monitor.ID, config)
		if err != nil {
			continue
		}

		if computedHealth == "down" || computedHealth == "degraded" {
			// Get agent info
			var agent db.Agent
			if err := s.db.Where("id = ?", monitor.AgentID).First(&agent).Error; err != nil {
				continue
			}

			// Get recent reports to determine severity
			var recentReports []db.MonitorReport
			if err := s.db.Where("monitor_id = ?", monitor.ID).
				Order("created_at DESC").
				Limit(5).
				Find(&recentReports).Error; err == nil {
				downCount := 0
				for _, report := range recentReports {
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
					"health":       computedHealth,
					"severity":     severity,
					"agent_id":     monitor.AgentID,
					"agent_name":   agent.Name,
					"down_count":   downCount,
				})
			}
		}
	}

	utils.SuccessResponse(c, 200, "Incident candidates retrieved successfully", gin.H{
		"candidates": candidates,
		"count":      len(candidates),
	})
}
