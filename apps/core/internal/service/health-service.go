package service

import (
	"fmt"
	"orion/core/internal/db"
	"orion/core/internal/logging"
	"time"

	"gorm.io/gorm"
)

type HealthService struct {
	db     *gorm.DB
	logger *logging.Logger
}

func NewHealthService(database *gorm.DB, logger *logging.Logger) *HealthService {
	return &HealthService{
		db:     database,
		logger: logger,
	}
}

// HealthComputationConfig holds configuration for health computation
type HealthComputationConfig struct {
	StaleDataThresholdMinutes int     // Minutes before data is considered stale
	FlappingThreshold         int     // Number of transitions to consider flapping
	DegradedFailureRate       float64 // Failure rate threshold for degraded (0.0-1.0)
	StaleIntervalMultiplier   int     // Reporting intervals missed before data is stale
	MinimumStaleWindow        time.Duration
}

// DefaultHealthConfig returns default configuration
func DefaultHealthConfig() HealthComputationConfig {
	return HealthComputationConfig{
		StaleDataThresholdMinutes: 15,  // 15 minutes
		FlappingThreshold:         3,   // 3 transitions
		DegradedFailureRate:       0.3, // 30% failure rate
		StaleIntervalMultiplier:   5,
		MinimumStaleWindow:        5 * time.Minute,
	}
}

func staleWindow(intervalSeconds int, config HealthComputationConfig) time.Duration {
	if intervalSeconds <= 0 {
		intervalSeconds = 60
	}

	multiplier := config.StaleIntervalMultiplier
	if multiplier <= 0 {
		multiplier = 5
	}

	window := time.Duration(intervalSeconds*multiplier) * time.Second
	if config.MinimumStaleWindow > 0 && window < config.MinimumStaleWindow {
		return config.MinimumStaleWindow
	}
	if window <= 0 {
		return time.Duration(config.StaleDataThresholdMinutes) * time.Minute
	}
	return window
}

func isStaleAt(timestamp time.Time, intervalSeconds int, config HealthComputationConfig) bool {
	if timestamp.IsZero() {
		return true
	}
	return time.Since(timestamp) > staleWindow(intervalSeconds, config)
}

// AgentHealthSnapshot separates Agent availability from monitor rollup health.
type AgentHealthSnapshot struct {
	AgentID       string
	OverallHealth string
	AgentHealth   string
	MonitorHealth string
	Reason        string
	UpCount       int
	DownCount     int
	DegradedCount int
	StaleCount    int
	UnknownCount  int
	TotalCount    int
}

// ComputeMonitorHealth computes the derived health state for a monitor with TTL caching
func (s *HealthService) ComputeMonitorHealth(monitorID string, config HealthComputationConfig) (string, error) {
	// Get monitor to check cache
	var monitor db.Monitor
	if err := s.db.Where("id = ?", monitorID).First(&monitor).Error; err != nil {
		s.logger.Error("Failed to get monitor", "monitor_id", monitorID, "error", err)
		return "unknown", err
	}

	// Check if cache is fresh (within reporting interval)
	now := time.Now()
	if monitor.LastHealthComputation != nil && monitor.ReportingIntervalSeconds > 0 {
		cacheExpiry := monitor.LastHealthComputation.Add(time.Duration(monitor.ReportingIntervalSeconds) * time.Second)
		if now.Before(cacheExpiry) && monitor.ComputedHealth != "" {
			// Cache is fresh, return cached value
			s.logger.Debug("Returning cached health", "monitor_id", monitorID, "computed_health", monitor.ComputedHealth)
			return monitor.ComputedHealth, nil
		}
	}

	// Cache is stale or missing, recompute
	computedHealth, err := s.computeMonitorHealthInternal(monitorID, monitor.ReportingIntervalSeconds, monitor.CreatedAt, config)
	if err != nil {
		return "unknown", err
	}

	// Update cache
	now = time.Now()
	if err := s.db.Model(&monitor).Updates(map[string]interface{}{
		"computed_health":         computedHealth,
		"last_health_computation": &now,
	}).Error; err != nil {
		s.logger.Error("Failed to update health cache", "monitor_id", monitorID, "error", err)
		// Don't fail, just return computed value
	}

	return computedHealth, nil
}

// computeMonitorHealthInternal performs the actual health computation
func (s *HealthService) computeMonitorHealthInternal(monitorID string, reportingIntervalSeconds int, monitorCreatedAt time.Time, config HealthComputationConfig) (string, error) {
	// Get recent reports for the monitor
	var reports []db.MonitorReport
	if err := s.db.Where("monitor_id = ?", monitorID).
		Order("created_at DESC").
		Limit(20). // Check last 20 reports
		Find(&reports).Error; err != nil {
		s.logger.Error("Failed to get monitor reports", "monitor_id", monitorID, "error", err)
		return "unknown", err
	}

	if len(reports) == 0 {
		if isStaleAt(monitorCreatedAt, reportingIntervalSeconds, config) {
			return "stale", nil
		}
		return "unknown", nil
	}

	// Check for stale data
	latestReport := reports[0]
	latestTime, err := time.Parse(time.RFC3339, latestReport.CollectedAt)
	if err != nil {
		// Try alternative format
		latestTime, err = time.Parse("2006-01-02T15:04:05Z", latestReport.CollectedAt)
		if err != nil {
			s.logger.Warn("Failed to parse report timestamp", "monitor_id", monitorID, "timestamp", latestReport.CollectedAt)
			return "unknown", nil
		}
	}

	if isStaleAt(latestTime, reportingIntervalSeconds, config) {
		return "stale", nil
	}

	// Check current health
	currentHealth := latestReport.Health

	// Check for flapping (rapid up/down transitions)
	if s.isFlapping(reports, config.FlappingThreshold) {
		return "degraded", nil
	}

	// Check for degraded state (intermittent failures)
	if s.isDegraded(reports, config.DegradedFailureRate) {
		return "degraded", nil
	}

	// Return current health if no derived states apply
	return currentHealth, nil
}

// isFlapping checks if a monitor is flapping (rapid up/down transitions)
func (s *HealthService) isFlapping(reports []db.MonitorReport, threshold int) bool {
	if len(reports) < threshold {
		return false
	}

	transitions := 0
	lastHealth := ""
	for _, report := range reports {
		if lastHealth != "" && lastHealth != report.Health {
			transitions++
		}
		lastHealth = report.Health
	}

	return transitions >= threshold
}

// isDegraded checks if a monitor is degraded (intermittent failures)
func (s *HealthService) isDegraded(reports []db.MonitorReport, failureRateThreshold float64) bool {
	if len(reports) < 5 {
		return false // Need at least 5 reports to determine degraded state
	}

	failures := 0
	for _, report := range reports {
		if report.Health == "down" {
			failures++
		}
	}

	failureRate := float64(failures) / float64(len(reports))
	return failureRate >= failureRateThreshold && failureRate < 1.0 // Not all failures (that would be "down")
}

// ComputeAgentHealthSnapshot computes Agent availability and monitor rollup health separately.
func (s *HealthService) ComputeAgentHealthSnapshot(agentID string, config HealthComputationConfig) (AgentHealthSnapshot, error) {
	snapshot := AgentHealthSnapshot{
		AgentID:       agentID,
		OverallHealth: "unknown",
		AgentHealth:   "unknown",
		MonitorHealth: "unknown",
		Reason:        "health has not been computed yet",
	}

	var agent db.Agent
	if err := s.db.Where("id = ?", agentID).First(&agent).Error; err != nil {
		s.logger.Error("Failed to get agent", "agent_id", agentID, "error", err)
		return snapshot, err
	}

	var monitors []db.Monitor
	if err := s.db.Where("agent_id = ? AND lifecycle = ?", agentID, "active").
		Find(&monitors).Error; err != nil {
		s.logger.Error("Failed to get agent monitors", "agent_id", agentID, "error", err)
		return snapshot, err
	}

	snapshot.TotalCount = len(monitors)

	if agent.MaintenanceMode {
		snapshot.AgentHealth = "maintenance"
		snapshot.OverallHealth = "maintenance"
		snapshot.MonitorHealth = storedMonitorRollup(monitors, &snapshot)
		snapshot.Reason = "agent is in maintenance"
		return snapshot, nil
	}

	if isStaleAt(agent.LastSeen, agent.ReportingIntervalSeconds, config) {
		snapshot.AgentHealth = "stale"
		snapshot.OverallHealth = "stale"
		snapshot.MonitorHealth = storedMonitorRollup(monitors, &snapshot)
		snapshot.Reason = "agent reports are stale"
		return snapshot, nil
	}

	snapshot.AgentHealth = "up"
	if len(monitors) == 0 {
		snapshot.OverallHealth = "up"
		snapshot.MonitorHealth = "unknown"
		snapshot.Reason = "agent is reporting and has no active monitors"
		return snapshot, nil
	}

	// Compute health for each monitor (uses cached value if fresh)
	for _, monitor := range monitors {
		computedHealth, err := s.ComputeMonitorHealth(monitor.ID, config)
		if err != nil {
			snapshot.UnknownCount++
			continue
		}

		switch computedHealth {
		case "up":
			snapshot.UpCount++
		case "down":
			snapshot.DownCount++
		case "degraded":
			snapshot.DegradedCount++
		case "stale":
			snapshot.StaleCount++
		default:
			snapshot.UnknownCount++
		}
	}

	snapshot.MonitorHealth = monitorRollupHealth(snapshot)
	snapshot.OverallHealth = agentOverallHealth(snapshot)
	snapshot.Reason = agentHealthReason(snapshot)

	return snapshot, nil
}

// ComputeAgentHealth computes the legacy overall health and monitor counts.
func (s *HealthService) ComputeAgentHealth(agentID string, config HealthComputationConfig) (string, int, int, int, error) {
	snapshot, err := s.ComputeAgentHealthSnapshot(agentID, config)
	if err != nil {
		return "unknown", 0, 0, 0, err
	}
	return snapshot.OverallHealth, snapshot.UpCount, snapshot.DownCount, snapshot.DegradedCount, nil
}

func storedMonitorRollup(monitors []db.Monitor, snapshot *AgentHealthSnapshot) string {
	if len(monitors) == 0 {
		return "unknown"
	}

	for _, monitor := range monitors {
		health := monitor.Health
		if health == "" {
			health = monitor.ComputedHealth
		}

		switch health {
		case "up":
			snapshot.UpCount++
		case "down":
			snapshot.DownCount++
		case "degraded":
			snapshot.DegradedCount++
		case "stale":
			snapshot.StaleCount++
		default:
			snapshot.UnknownCount++
		}
	}

	return monitorRollupHealth(*snapshot)
}

func monitorRollupHealth(snapshot AgentHealthSnapshot) string {
	if snapshot.TotalCount == 0 {
		return "unknown"
	}
	if snapshot.DownCount == snapshot.TotalCount {
		return "down"
	}
	if snapshot.StaleCount == snapshot.TotalCount {
		return "stale"
	}
	if snapshot.UnknownCount == snapshot.TotalCount {
		return "unknown"
	}
	if snapshot.DownCount > 0 || snapshot.DegradedCount > 0 || snapshot.StaleCount > 0 {
		return "degraded"
	}
	if snapshot.UnknownCount > 0 {
		return "unknown"
	}
	return "up"
}

func agentOverallHealth(snapshot AgentHealthSnapshot) string {
	if snapshot.AgentHealth != "up" {
		return snapshot.AgentHealth
	}
	switch snapshot.MonitorHealth {
	case "down":
		return "down"
	case "degraded", "stale":
		return "degraded"
	case "unknown":
		return "unknown"
	default:
		return "up"
	}
}

func agentHealthReason(snapshot AgentHealthSnapshot) string {
	switch {
	case snapshot.TotalCount == 0:
		return "agent is reporting and has no active monitors"
	case snapshot.DownCount == snapshot.TotalCount:
		return "all active monitors are failing"
	case snapshot.StaleCount == snapshot.TotalCount:
		return "agent is reporting but all active monitor reports are stale"
	case snapshot.DownCount > 0:
		return fmt.Sprintf("agent is reporting; %d active monitor%s failing", snapshot.DownCount, plural(snapshot.DownCount))
	case snapshot.DegradedCount > 0:
		return fmt.Sprintf("agent is reporting; %d active monitor%s degraded", snapshot.DegradedCount, plural(snapshot.DegradedCount))
	case snapshot.StaleCount > 0:
		return fmt.Sprintf("agent is reporting; %d active monitor report%s stale", snapshot.StaleCount, plural(snapshot.StaleCount))
	case snapshot.UnknownCount > 0:
		return fmt.Sprintf("agent is reporting; %d active monitor%s unknown", snapshot.UnknownCount, plural(snapshot.UnknownCount))
	default:
		return "agent is reporting and all active monitors are healthy"
	}
}

func plural(count int) string {
	if count == 1 {
		return " is"
	}
	return "s are"
}

// DetectStaleMonitors finds monitors with stale data
func (s *HealthService) DetectStaleMonitors(config HealthComputationConfig) ([]db.Monitor, error) {
	var monitors []db.Monitor

	// Get all active monitors
	if err := s.db.Where("lifecycle = ?", "active").Find(&monitors).Error; err != nil {
		return nil, err
	}

	var staleMonitors []db.Monitor
	for _, monitor := range monitors {
		var latestReport db.MonitorReport
		if err := s.db.Where("monitor_id = ?", monitor.ID).
			Order("created_at DESC").
			First(&latestReport).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				if isStaleAt(monitor.CreatedAt, monitor.ReportingIntervalSeconds, config) {
					staleMonitors = append(staleMonitors, monitor)
				}
			}
			continue
		}

		latestTime, err := time.Parse(time.RFC3339, latestReport.CollectedAt)
		if err != nil {
			latestTime, err = time.Parse("2006-01-02T15:04:05Z", latestReport.CollectedAt)
			if err != nil {
				staleMonitors = append(staleMonitors, monitor)
				continue
			}
		}

		if isStaleAt(latestTime, monitor.ReportingIntervalSeconds, config) {
			staleMonitors = append(staleMonitors, monitor)
		}
	}

	return staleMonitors, nil
}
