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
	Type         string
	OwnerKind    string
	OwnerName    string
	Source       string
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

			updates := map[string]any{
				"lifecycle":                  "active",
				"health":                     "up",
				"description":                req.Description,
				"type":                       req.Type,
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

		updates := map[string]any{}
		if req.ReportingIntervalSeconds > 0 && monitor.ReportingIntervalSeconds != req.ReportingIntervalSeconds {
			updates["reporting_interval_seconds"] = req.ReportingIntervalSeconds
		}
		if req.Description != nil && (monitor.Description == nil || *monitor.Description != *req.Description) {
			updates["description"] = req.Description
		}
		if req.Type != "" && monitor.Type != req.Type {
			updates["type"] = req.Type
		}
		if req.Meta != "" && monitor.Meta != req.Meta {
			updates["meta"] = req.Meta
		}
		if len(updates) > 0 {
			updates["updated_at"] = time.Now()
			if err := s.db.Model(&monitor).Updates(updates).Error; err != nil {
				return nil, err
			}
		}

		return &RegisterMonitorResponse{
			MonitorID: monitor.ID,
		}, nil

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
		Updates(map[string]any{
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
	monitors, _, err := s.listAgentMonitors(agentID, healthFilter, lifecycleFilter)
	if err != nil {
		return nil, err
	}
	return paginateMonitors(monitors, limit, offset), nil
}

func (s *MonitorService) ListAllMonitors(opts ListAllMonitorsOpts) ([]db.Monitor, int64, error) {
	var monitors []db.Monitor
	query := s.applyAllMonitorListFilters(s.db.Model(&db.Monitor{}), opts)

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

	if err := query.Find(&monitors).Error; err != nil {
		s.logger.Error("Failed to list all monitors", "error", err)
		return nil, 0, err
	}

	monitors, err := s.monitorsWithComputedHealth(monitors)
	if err != nil {
		return nil, 0, err
	}
	monitors = filterMonitorsByComputedHealth(monitors, opts.Health)
	if opts.StaleOnly {
		monitors = filterMonitorsByComputedHealth(monitors, "stale")
	}

	count := int64(len(monitors))
	return paginateMonitors(monitors, opts.Limit, opts.Offset), count, nil
}

func (s *MonitorService) GetMonitorSummary() (MonitorSummary, error) {
	var monitors []db.Monitor
	if err := s.db.Where("lifecycle = ?", "active").Find(&monitors).Error; err != nil {
		s.logger.Error("Failed to load monitor summary", "error", err)
		return MonitorSummary{}, err
	}

	summary := MonitorSummary{Total: int64(len(monitors))}
	monitorIDs := make([]string, 0, len(monitors))
	healthService := NewHealthService(s.db, s.logger)
	config := DefaultHealthConfig()

	for _, monitor := range monitors {
		monitorIDs = append(monitorIDs, monitor.ID)

		health, err := healthService.ComputeMonitorHealth(monitor.ID, config)
		if err != nil {
			summary.Unknown++
			continue
		}

		switch strings.ToLower(health) {
		case "stale":
			summary.Stale++
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
			Where("monitor_id IN ? AND status IN ?", monitorIDs, activeIncidentStatuses()).
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
	monitors, err := s.monitorsWithComputedHealth([]db.Monitor{monitor})
	if err != nil {
		return nil, err
	}
	return &monitors[0], nil
}

func (s *MonitorService) GetMonitorCount(agentID string, healthFilter string, lifecycleFilter string) (int64, error) {
	_, count, err := s.listAgentMonitors(agentID, healthFilter, lifecycleFilter)
	return count, err
}

func (s *MonitorService) listAgentMonitors(agentID string, healthFilter string, lifecycleFilter string) ([]db.Monitor, int64, error) {
	var monitors []db.Monitor
	query := s.applyMonitorListFilters(s.db, agentID, lifecycleFilter).Order("created_at DESC")
	if err := query.Find(&monitors).Error; err != nil {
		s.logger.Error("Failed to list monitors", "agent_id", agentID, "error", err)
		return nil, 0, err
	}

	monitors, err := s.monitorsWithComputedHealth(monitors)
	if err != nil {
		return nil, 0, err
	}
	monitors = filterMonitorsByComputedHealth(monitors, healthFilter)

	s.logger.Debug("Retrieved monitors", "agent_id", agentID, "count", len(monitors))
	return monitors, int64(len(monitors)), nil
}

func (s *MonitorService) applyMonitorListFilters(query *gorm.DB, agentID string, lifecycleFilter string) *gorm.DB {
	query = query.Where("agent_id = ?", agentID)

	if lifecycleFilter != "" {
		return query.Where("lifecycle = ?", lifecycleFilter)
	}

	return query.Where("lifecycle = ?", "active")
}

func (s *MonitorService) applyAllMonitorListFilters(query *gorm.DB, opts ListAllMonitorsOpts) *gorm.DB {
	coreMonitorIDs := s.db.Model(&db.CoreMonitorConfig{}).Select("monitor_id")

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

	if opts.OwnerName != "" {
		like := "%" + opts.OwnerName + "%"
		query = query.Where(
			"agent_id IN (?)",
			s.db.Model(&db.Agent{}).Select("id").Where("name LIKE ? OR machine_id LIKE ? OR id LIKE ?", like, like, like),
		)
	}

	switch normalizeMonitorOwnerFilter(opts.OwnerKind) {
	case "core":
		query = query.Where("id IN (?)", coreMonitorIDs)
	case "agent":
		query = query.Where("id NOT IN (?)", coreMonitorIDs)
	}

	switch normalizeMonitorOwnerFilter(opts.Source) {
	case "core":
		query = query.Where("id IN (?)", coreMonitorIDs)
	case "agent":
		query = query.Where("id NOT IN (?)", coreMonitorIDs)
	}

	if opts.Lifecycle != "" {
		query = query.Where("lifecycle = ?", opts.Lifecycle)
	} else {
		query = query.Where("lifecycle = ?", "active")
	}

	if opts.Type != "" {
		query = query.Where("LOWER(type) = ?", normalizeMonitorTypeFilter(opts.Type))
	}

	if opts.HasIncidents {
		query = query.Where(
			"id IN (?)",
			s.db.Model(&db.Incident{}).Select("monitor_id").Where("status IN ?", activeIncidentStatuses()),
		)
	}

	return query
}

func normalizeMonitorTypeFilter(monitorType string) string {
	switch strings.ToLower(strings.TrimSpace(monitorType)) {
	case "docker":
		return "docker-container"
	case "systemd":
		return "systemd-service"
	default:
		return strings.ToLower(strings.TrimSpace(monitorType))
	}
}

func normalizeMonitorOwnerFilter(owner string) string {
	switch strings.ToLower(strings.TrimSpace(owner)) {
	case "core", "core-worker", "core_monitor", "core-monitor":
		return "core"
	case "agent", "agent-monitor", "agent_monitor":
		return "agent"
	default:
		return ""
	}
}

func filterMonitorsByComputedHealth(monitors []db.Monitor, healthFilter string) []db.Monitor {
	health := strings.ToLower(strings.TrimSpace(healthFilter))
	if health == "" {
		return monitors
	}

	filtered := make([]db.Monitor, 0, len(monitors))
	for _, monitor := range monitors {
		if strings.ToLower(monitor.ComputedHealth) == health {
			filtered = append(filtered, monitor)
		}
	}
	return filtered
}

func paginateMonitors(monitors []db.Monitor, limit int, offset int) []db.Monitor {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	if offset >= len(monitors) {
		return []db.Monitor{}
	}

	end := offset + limit
	if end > len(monitors) {
		end = len(monitors)
	}
	return monitors[offset:end]
}

func (s *MonitorService) monitorsWithComputedHealth(monitors []db.Monitor) ([]db.Monitor, error) {
	healthService := NewHealthService(s.db, s.logger)
	config := DefaultHealthConfig()

	for index := range monitors {
		health, err := healthService.ComputeMonitorHealth(monitors[index].ID, config)
		if err != nil {
			return nil, err
		}
		monitors[index].Health = health
		monitors[index].ComputedHealth = health
	}

	return monitors, nil
}
