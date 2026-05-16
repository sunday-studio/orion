package service

import (
	"errors"
	"orion/core/internal/db"
	"orion/core/internal/logging"
	"orion/core/internal/utils"
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

	query := s.applyMonitorListFilters(s.db, agentID, healthFilter, lifecycleFilter)

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
	return monitors, nil
}

func (s *MonitorService) GetMonitor(monitorID string) (*db.Monitor, error) {
	var monitor db.Monitor
	if err := s.db.Where("id = ?", monitorID).First(&monitor).Error; err != nil {
		s.logger.Error("Failed to get monitor", "monitor_id", monitorID, "error", err)
		return nil, err
	}
	return &monitor, nil
}

func (s *MonitorService) GetMonitorCount(agentID string, healthFilter string, lifecycleFilter string) (int64, error) {
	var count int64
	if err := s.applyMonitorListFilters(s.db.Model(&db.Monitor{}), agentID, healthFilter, lifecycleFilter).Count(&count).Error; err != nil {
		s.logger.Error("Failed to count monitors", "agent_id", agentID, "error", err)
		return 0, err
	}
	return count, nil
}

func (s *MonitorService) applyMonitorListFilters(query *gorm.DB, agentID string, healthFilter string, lifecycleFilter string) *gorm.DB {
	query = query.Where("agent_id = ?", agentID)

	if healthFilter != "" {
		query = query.Where("health = ?", healthFilter)
	}

	if lifecycleFilter != "" {
		return query.Where("lifecycle = ?", lifecycleFilter)
	}

	return query.Where("lifecycle = ?", "active")
}
