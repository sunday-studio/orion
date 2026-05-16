package service

import (
	"errors"
	"orion/core/internal/db"
	"orion/core/internal/logging"
	"orion/core/internal/utils"
	"strings"
	"time"

	"gorm.io/gorm"
)

type RegisterMonitorRequest struct {
	AgentID                  string    `json:"agent_id,omitempty"`
	Name                     string    `json:"name" binding:"required"`
	Description              *string   `json:"description" binding:"required"`
	Type                     string    `json:"type" binding:"required"`
	LastChecked              time.Time `json:"last_checked" binding:"required"`
	ReportingIntervalSeconds int       `json:"reporting_interval_seconds"` // Monitor check interval in seconds
	Meta                     string    `json:"meta,omitempty"`
}

type UnregisterMonitorRequest struct {
	AgentID   string `json:"agent_id,omitempty"`
	MonitorID string `json:"monitor_id" binding:"required"`
}

type UnregisterMonitorResponse struct {
	Success bool `json:"success"`
}

type RegisterMonitorResponse struct {
	MonitorID string `json:"monitor_id"`
}

type ListAllMonitorsOpts struct {
	Limit        int
	Offset       int
	Search       string
	Health       string
	Lifecycle    string
	StaleOnly    bool
	HasIncidents bool
	Sort         string
	Order        string
}

type MonitorSummary struct {
	Total        int64 `json:"total"`
	Up           int64 `json:"up"`
	Down         int64 `json:"down"`
	Degraded     int64 `json:"degraded"`
	Unknown      int64 `json:"unknown"`
	Stale        int64 `json:"stale"`
	HasIncidents int64 `json:"has_incidents"`
}

type MonitorService struct {
	db     *gorm.DB
	logger *logging.Logger
}

func NewMonitorService(database *gorm.DB, logger *logging.Logger) *MonitorService {
	return &MonitorService{
		db:     database,
		logger: logger,
	}
}

func (s *MonitorService) RegisterMonitor(req *RegisterMonitorRequest) (*RegisterMonitorResponse, error) {
	var monitor db.Monitor

	err := s.db.
		Where("agent_id = ? AND name = ?", req.AgentID, req.Name).
		First(&monitor).Error

	switch {
	case err == nil:
		if monitor.Lifecycle == "deleted" {
			// Default interval to 60 seconds if not provided
			interval := req.ReportingIntervalSeconds
			if interval == 0 {
				interval = 60
			}

			updates := map[string]interface{}{
				"lifecycle":                  "active",
				"health":                     "up",
				"updated_at":                 time.Now(),
				"reporting_interval_seconds": interval,
			}
			if req.Meta != "" {
				updates["meta"] = req.Meta
			}

			if err := s.db.Model(&monitor).Updates(updates).Error; err != nil {
				return nil, err
			}

			// exist but deleted → revive
			return &RegisterMonitorResponse{
				MonitorID: monitor.ID,
			}, nil
		}

		// already exist
		s.logger.Error("Monitor with this name already exists for agent", "agent_id", req.AgentID, "name", req.Name)
		return nil, gorm.ErrRegistered

	case errors.Is(err, gorm.ErrRecordNotFound):
		resp, err := s.createNewMonitor(req)
		if err != nil {
			return nil, err
		}
		return resp, nil

	default:
		s.logger.Error("Database error during monitor lookup", "error", err)
		return nil, err
	}
}

func (s *MonitorService) UnregisterMonitor(req *UnregisterMonitorRequest) (*UnregisterMonitorResponse, error) {
	now := time.Now()

	if err := s.db.Model(&db.Monitor{}).
		Where("agent_id = ? AND id = ? AND (deleted_at IS NULL OR deleted_at = ?)", req.AgentID, req.MonitorID, time.Time{}).
		Updates(map[string]interface{}{
			"lifecycle":  "deleted",
			"health":     "unknown",
			"updated_at": now,
			"deleted_at": &now,
		}).Error; err != nil {

		s.logger.Error(
			"Failed to unregister monitor",
			"agent_id", req.AgentID,
			"monitor_id", req.MonitorID,
			"error", err,
		)
		return nil, err
	}

	return &UnregisterMonitorResponse{Success: true}, nil
}

func (s *MonitorService) createNewMonitor(req *RegisterMonitorRequest) (*RegisterMonitorResponse, error) {
	monitorID := utils.GenerateID("monitor")

	// Default interval to 60 seconds if not provided
	interval := req.ReportingIntervalSeconds
	if interval == 0 {
		interval = 60
	}

	monitor := db.Monitor{
		ID:                       monitorID,
		AgentID:                  req.AgentID,
		Description:              req.Description,
		Type:                     req.Type,
		Name:                     req.Name,
		Meta:                     req.Meta,
		Lifecycle:                "active",
		Health:                   "unknown",
		ComputedHealth:           "unknown",
		ReportingIntervalSeconds: interval,
		CreatedAt:                time.Now(),
		UpdatedAt:                time.Now(),
	}

	if err := s.db.Create(&monitor).Error; err != nil {
		s.logger.Error("Failed to create monitor", "error", err)
		return nil, err
	}

	return &RegisterMonitorResponse{
		MonitorID: monitorID,
	}, nil
}

func (s *MonitorService) ListMonitors(agentID string, healthFilter string, lifecycleFilter string, limit int, offset int) ([]db.Monitor, error) {
	var monitors []db.Monitor

	staleMonitorIDs, err := s.staleMonitorIDs()
	if err != nil {
		s.logger.Error("Failed to load stale monitor IDs", "error", err)
		return nil, err
	}
	query := s.applyMonitorListFilters(s.db, agentID, healthFilter, lifecycleFilter, staleMonitorIDs)

	query = query.Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	if offset > 0 {
		query = query.Offset(offset)
	}

	if err := query.Find(&monitors).Error; err != nil {
		s.logger.Error("Failed to list monitors", "agent_id", agentID, "error", err)
		return nil, err
	}

	s.logger.Debug("Retrieved monitors", "agent_id", agentID, "count", len(monitors))
	return monitorsWithDerivedStaleHealth(monitors, staleMonitorIDs), nil
}

func (s *MonitorService) ListAllMonitors(opts ListAllMonitorsOpts) ([]db.Monitor, int64, error) {
	var monitors []db.Monitor
	staleMonitorIDs, err := s.staleMonitorIDs()
	if err != nil {
		s.logger.Error("Failed to load stale monitor IDs", "error", err)
		return nil, 0, err
	}
	query := s.applyAllMonitorListFilters(s.db.Model(&db.Monitor{}), opts, staleMonitorIDs)

	var count int64
	if err := query.Count(&count).Error; err != nil {
		s.logger.Error("Failed to count all monitors", "error", err)
		return nil, 0, err
	}

	sortColumn := "updated_at"
	switch strings.ToLower(strings.TrimSpace(opts.Sort)) {
	case "name", "type", "health", "lifecycle", "created_at", "updated_at", "last_successful_report_at":
		sortColumn = strings.ToLower(strings.TrimSpace(opts.Sort))
	}

	order := "desc"
	if strings.EqualFold(opts.Order, "asc") {
		order = "asc"
	}

	query = query.Order(sortColumn + " " + order)

	if opts.Limit > 0 {
		query = query.Limit(opts.Limit)
	}
	if opts.Offset > 0 {
		query = query.Offset(opts.Offset)
	}

	if err := query.Find(&monitors).Error; err != nil {
		s.logger.Error("Failed to list all monitors", "error", err)
		return nil, 0, err
	}

	return monitorsWithDerivedStaleHealth(monitors, staleMonitorIDs), count, nil
}

func (s *MonitorService) GetMonitorSummary() (MonitorSummary, error) {
	var monitors []db.Monitor
	if err := s.db.Where("lifecycle = ?", "active").Find(&monitors).Error; err != nil {
		s.logger.Error("Failed to load monitor summary", "error", err)
		return MonitorSummary{}, err
	}

	summary := MonitorSummary{Total: int64(len(monitors))}
	monitorIDs := make([]string, 0, len(monitors))
	staleMonitorIDs, err := s.staleMonitorIDs()
	if err != nil {
		s.logger.Error("Failed to load stale monitor summary", "error", err)
		return MonitorSummary{}, err
	}
	staleMonitorIDSet := make(map[string]struct{}, len(staleMonitorIDs))
	for _, monitorID := range staleMonitorIDs {
		staleMonitorIDSet[monitorID] = struct{}{}
	}

	for _, monitor := range monitors {
		monitorIDs = append(monitorIDs, monitor.ID)
		if _, stale := staleMonitorIDSet[monitor.ID]; stale {
			summary.Stale++
			continue
		}

		switch strings.ToLower(monitor.Health) {
		case "up":
			summary.Up++
		case "down":
			summary.Down++
		case "degraded":
			summary.Degraded++
		default:
			summary.Unknown++
		}
	}

	if len(monitorIDs) > 0 {
		var rows []struct {
			MonitorID string
		}
		if err := s.db.Model(&db.Incident{}).
			Select("monitor_id").
			Where("monitor_id IN ? AND status IN ?", monitorIDs, []string{"open", "acknowledged"}).
			Group("monitor_id").
			Find(&rows).Error; err != nil {
			s.logger.Error("Failed to load monitor incident summary", "error", err)
			return MonitorSummary{}, err
		}
		summary.HasIncidents = int64(len(rows))
	}

	return summary, nil
}

func (s *MonitorService) GetMonitor(monitorID string) (*db.Monitor, error) {
	var monitor db.Monitor
	if err := s.db.Where("id = ?", monitorID).First(&monitor).Error; err != nil {
		s.logger.Error("Failed to get monitor", "monitor_id", monitorID, "error", err)
		return nil, err
	}
	monitors, err := s.monitorsWithDerivedHealth([]db.Monitor{monitor})
	if err != nil {
		return nil, err
	}
	return &monitors[0], nil
}

func (s *MonitorService) GetMonitorCount(agentID string, healthFilter string, lifecycleFilter string) (int64, error) {
	var count int64
	staleMonitorIDs, err := s.staleMonitorIDs()
	if err != nil {
		s.logger.Error("Failed to load stale monitor IDs", "error", err)
		return 0, err
	}
	if err := s.applyMonitorListFilters(s.db.Model(&db.Monitor{}), agentID, healthFilter, lifecycleFilter, staleMonitorIDs).Count(&count).Error; err != nil {
		s.logger.Error("Failed to count monitors", "agent_id", agentID, "error", err)
		return 0, err
	}
	return count, nil
}

func (s *MonitorService) applyMonitorListFilters(query *gorm.DB, agentID string, healthFilter string, lifecycleFilter string, staleMonitorIDs []string) *gorm.DB {
	query = query.Where("agent_id = ?", agentID)

	if healthFilter != "" {
		health := strings.ToLower(strings.TrimSpace(healthFilter))
		if health == "stale" {
			if len(staleMonitorIDs) == 0 {
				query = query.Where("1 = 0")
			} else {
				query = query.Where("id IN ?", staleMonitorIDs)
			}
		} else {
			query = query.Where("health = ?", healthFilter)
			if len(staleMonitorIDs) > 0 {
				query = query.Where("id NOT IN ?", staleMonitorIDs)
			}
		}
	}

	if lifecycleFilter != "" {
		return query.Where("lifecycle = ?", lifecycleFilter)
	}

	return query.Where("lifecycle = ?", "active")
}

func (s *MonitorService) applyAllMonitorListFilters(query *gorm.DB, opts ListAllMonitorsOpts, staleMonitorIDs []string) *gorm.DB {
	if opts.Search != "" {
		like := "%" + opts.Search + "%"
		query = query.Where(
			"name LIKE ? OR type LIKE ? OR id LIKE ? OR agent_id IN (?)",
			like,
			like,
			like,
			s.db.Model(&db.Agent{}).Select("id").Where("name LIKE ? OR machine_id LIKE ?", like, like),
		)
	}

	health := strings.ToLower(strings.TrimSpace(opts.Health))
	if opts.Health != "" {
		if health == "stale" {
			if len(staleMonitorIDs) == 0 {
				query = query.Where("1 = 0")
			} else {
				query = query.Where("id IN ?", staleMonitorIDs)
			}
		} else {
			query = query.Where("health = ?", opts.Health)
			if len(staleMonitorIDs) > 0 {
				query = query.Where("id NOT IN ?", staleMonitorIDs)
			}
		}
	}

	if opts.Lifecycle != "" {
		query = query.Where("lifecycle = ?", opts.Lifecycle)
	} else {
		query = query.Where("lifecycle = ?", "active")
	}

	if opts.StaleOnly {
		if len(staleMonitorIDs) == 0 {
			query = query.Where("1 = 0")
		} else {
			query = query.Where("id IN ?", staleMonitorIDs)
		}
	}

	if opts.HasIncidents {
		query = query.Where(
			"id IN (?)",
			s.db.Model(&db.Incident{}).Select("monitor_id").Where("status IN ?", []string{"open", "acknowledged"}),
		)
	}

	return query
}

func (s *MonitorService) staleMonitorIDs() ([]string, error) {
	healthService := NewHealthService(s.db, s.logger)
	staleMonitors, err := healthService.DetectStaleMonitors(DefaultHealthConfig())
	if err != nil {
		return nil, err
	}

	monitorIDs := make([]string, 0, len(staleMonitors))
	for _, monitor := range staleMonitors {
		monitorIDs = append(monitorIDs, monitor.ID)
	}
	return monitorIDs, nil
}

func monitorsWithDerivedStaleHealth(monitors []db.Monitor, staleMonitorIDs []string) []db.Monitor {
	if len(staleMonitorIDs) == 0 {
		return monitors
	}

	staleMonitorIDSet := make(map[string]struct{}, len(staleMonitorIDs))
	for _, monitorID := range staleMonitorIDs {
		staleMonitorIDSet[monitorID] = struct{}{}
	}

	for index := range monitors {
		if _, stale := staleMonitorIDSet[monitors[index].ID]; !stale {
			continue
		}
		monitors[index].Health = "stale"
		monitors[index].ComputedHealth = "stale"
	}

	return monitors
}

func (s *MonitorService) monitorsWithDerivedHealth(monitors []db.Monitor) ([]db.Monitor, error) {
	staleMonitorIDs, err := s.staleMonitorIDs()
	if err != nil {
		s.logger.Error("Failed to derive monitor health", "error", err)
		return nil, err
	}
	return monitorsWithDerivedStaleHealth(monitors, staleMonitorIDs), nil
}
