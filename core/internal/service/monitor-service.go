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
	AgentID     string    `json:"agent_id" binding:"required"`
	Name        string    `json:"name" binding:"required"`
	Description *string   `json:"description" binding:"required"`
	Type        string    `json:"type" binding:"required"`
	LastChecked time.Time `json:"last_checked" binding:"required"`
}

type UnregisterMonitorRequest struct {
	AgentID   string `json:"agent_id" binding:"required"`
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
			if err := s.db.Model(&monitor).Updates(map[string]interface{}{
				"lifecycle":  "active",
				"health":     "up",
				"updated_at": time.Now(),
			}).Error; err != nil {
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
		Where("agent_id = ? AND id = ? AND deleted_at IS NULL", req.AgentID, req.MonitorID).
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

	monitor := db.Monitor{
		ID:          monitorID,
		AgentID:     req.AgentID,
		Description: req.Description,
		Type:        req.Type,
		Name:        req.Name,
		Lifecycle:   "active",
		Health:      "unknown",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.db.Create(&monitor).Error; err != nil {
		s.logger.Error("Failed to create monitor", "error", err)
		return nil, err
	}

	return &RegisterMonitorResponse{
		MonitorID: monitorID,
	}, nil
}
