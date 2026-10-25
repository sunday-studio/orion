package service

import (
	"context"
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
}

// DefaultHealthConfig returns default configuration
func DefaultHealthConfig() HealthComputationConfig {
	return HealthComputationConfig{
		StaleDataThresholdMinutes: 15,  // 15 minutes
		FlappingThreshold:         3,   // 3 transitions
		DegradedFailureRate:       0.3, // 30% failure rate
	}
}

// CachedHealth returns the stored computed health for an already-loaded monitor without querying.
func (s *HealthService) CachedHealth(m db.Monitor) string {
	if m.LastHealthComputation == nil || m.ComputedHealth == "" {
		return "unknown"
	}
	return m.ComputedHealth
}

// ComputeMonitorHealth returns the monitor's computed health from the stored value.
// It is read-only: health is refreshed on report ingest and by the background refresher,
// so request handlers never write. It only computes inline if a value was never stored.
func (s *HealthService) ComputeMonitorHealth(monitorID string, config HealthComputationConfig) (string, error) {
	var monitor db.Monitor
	if err := s.db.Where("id = ?", monitorID).First(&monitor).Error; err != nil {
		s.logger.Error("Failed to get monitor", "monitor_id", monitorID, "error", err)
		return "unknown", err
	}

	if monitor.LastHealthComputation != nil && monitor.ComputedHealth != "" {
		return monitor.ComputedHealth, nil
	}
	return s.RecomputeMonitorHealth(monitorID, config)
}

// RecomputeMonitorHealth derives health from recent reports and stores it. It writes only
// when the value changed (or was never stored) to keep write volume low.
func (s *HealthService) RecomputeMonitorHealth(monitorID string, config HealthComputationConfig) (string, error) {
	var monitor db.Monitor
	if err := s.db.Where("id = ?", monitorID).First(&monitor).Error; err != nil {
		s.logger.Error("Failed to get monitor", "monitor_id", monitorID, "error", err)
		return "unknown", err
	}

	computedHealth, err := s.computeMonitorHealthInternal(monitorID, config)
	if err != nil {
		return "unknown", err
	}

	if monitor.LastHealthComputation != nil && monitor.ComputedHealth == computedHealth {
		return computedHealth, nil
	}

	now := time.Now()
	if err := s.db.Model(&monitor).Updates(map[string]interface{}{
		"computed_health":         computedHealth,
		"last_health_computation": &now,
	}).Error; err != nil {
		s.logger.Error("Failed to update health cache", "monitor_id", monitorID, "error", err)
		// Don't fail, just return computed value
	}

	return computedHealth, nil
}

// RefreshAll recomputes health for every active monitor so time-based states (stale data) advance
// even when no reports arrive.
func (s *HealthService) RefreshAll(config HealthComputationConfig) {
	var ids []string
	if err := s.db.Model(&db.Monitor{}).Where("lifecycle = ?", "active").Pluck("id", &ids).Error; err != nil {
		s.logger.Error("Failed to list monitors for health refresh", "error", err)
		return
	}
	for _, id := range ids {
		if _, err := s.RecomputeMonitorHealth(id, config); err != nil {
			s.logger.Error("Health refresh failed", "monitor_id", id, "error", err)
		}
	}
}

// StartRefresher runs RefreshAll on an interval until ctx is cancelled.
func (s *HealthService) StartRefresher(ctx context.Context, interval time.Duration) {
	go func() {
		config := DefaultHealthConfig()
		s.RefreshAll(config)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.RefreshAll(config)
			}
		}
	}()
}

// computeMonitorHealthInternal performs the actual health computation
func (s *HealthService) computeMonitorHealthInternal(monitorID string, config HealthComputationConfig) (string, error) {
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

	staleThreshold := time.Now().Add(-time.Duration(config.StaleDataThresholdMinutes) * time.Minute)
	if latestTime.Before(staleThreshold) {
		return "unknown", nil // Stale data
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

// ComputeAgentHealth computes the overall health for an agent based on its monitors
func (s *HealthService) ComputeAgentHealth(agentID string, config HealthComputationConfig) (string, int, int, int, error) {
	var monitors []db.Monitor
	if err := s.db.Where("agent_id = ? AND lifecycle = ?", agentID, "active").
		Find(&monitors).Error; err != nil {
		s.logger.Error("Failed to get agent monitors", "agent_id", agentID, "error", err)
		return "unknown", 0, 0, 0, err
	}

	if len(monitors) == 0 {
		return "unknown", 0, 0, 0, nil
	}

	upCount := 0
	downCount := 0
	degradedCount := 0
	unknownCount := 0

	// Health per monitor comes from the stored value on the already-loaded rows
	for _, monitor := range monitors {
		computedHealth := s.CachedHealth(monitor)

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

	// Determine overall agent health (priority: down > degraded > unknown > up)
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

	return overallHealth, upCount, downCount, degradedCount, nil
}

// DetectStaleMonitors finds monitors with stale data using one query for the latest report per monitor.
func (s *HealthService) DetectStaleMonitors(config HealthComputationConfig) ([]db.Monitor, error) {
	var monitors []db.Monitor
	threshold := time.Now().Add(-time.Duration(config.StaleDataThresholdMinutes) * time.Minute)

	if err := s.db.Where("lifecycle = ?", "active").Find(&monitors).Error; err != nil {
		return nil, err
	}
	if len(monitors) == 0 {
		return nil, nil
	}

	ids := make([]string, len(monitors))
	for i, m := range monitors {
		ids[i] = m.ID
	}
	latest, err := s.latestMonitorReports(ids)
	if err != nil {
		return nil, err
	}

	var staleMonitors []db.Monitor
	for _, monitor := range monitors {
		latestReport, ok := latest[monitor.ID]
		if !ok {
			// No reports yet
			staleMonitors = append(staleMonitors, monitor)
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

		if latestTime.Before(threshold) {
			staleMonitors = append(staleMonitors, monitor)
		}
	}

	return staleMonitors, nil
}

// latestMonitorReports returns the newest report per monitor in one query (uses the
// (monitor_id, created_at) index).
func (s *HealthService) latestMonitorReports(monitorIDs []string) (map[string]db.MonitorReport, error) {
	var reports []db.MonitorReport
	err := s.db.Raw(`
		SELECT mr.* FROM monitor_reports mr
		JOIN (SELECT monitor_id, MAX(created_at) AS m FROM monitor_reports WHERE monitor_id IN ? GROUP BY monitor_id) x
		  ON mr.monitor_id = x.monitor_id AND mr.created_at = x.m`, monitorIDs).Scan(&reports).Error
	if err != nil {
		return nil, err
	}
	out := make(map[string]db.MonitorReport, len(reports))
	for _, r := range reports {
		out[r.MonitorID] = r
	}
	return out, nil
}
