package service

import (
	"orion/core/internal/db"
	"orion/core/internal/logging"
	"orion/core/internal/utils"
	"time"

	"gorm.io/gorm"
)

type RegisterApplicationRequest struct {
	AgentID     string    `json:"agent_id" binding:"required"`
	Name        string    `json:"name" binding:"required"`
	Description *string   `json:"description" binding:"required"`
	Type        string    `json:"type" binding:"required"`
	LastChecked time.Time `json:"last_checked" binding:"required"`
}

type RegisterApplicationResponse struct {
	ApplicationID string `json:"application_id"`
}

type AppsService struct {
	db     *gorm.DB
	logger *logging.Logger
}

func NewAppsService(database *gorm.DB, logger *logging.Logger) *AppsService {
	return &AppsService{
		db:     database,
		logger: logger,
	}
}

func (s *AppsService) RegisterApplication(req *RegisterApplicationRequest) (*RegisterApplicationResponse, error) {
	var application db.Application

	if err := s.db.Where("agent_id = ? AND name = ?", req.AgentID, req.Name).First(&application).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			applicationID, err := s.createNewApplication(req)
			if err != nil {
				return nil, err
			}
			return applicationID, nil
		}
		s.logger.Error("Database error during application lookup", "error", err)
		return nil, err
	}

	s.logger.Error("Application with this name already exists for agent", "agent_id", req.AgentID, "name", req.Name)
	return nil, gorm.ErrRegistered
}

func (s *AppsService) createNewApplication(req *RegisterApplicationRequest) (*RegisterApplicationResponse, error) {
	applicationID := utils.GenerateID("app")

	application := db.Application{
		ID:          applicationID,
		AgentID:     req.AgentID,
		Description: req.Description,
		Type:        req.Type,
		Name:        req.Name,
		Status:      "running",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.db.Create(&application).Error; err != nil {
		s.logger.Error("Failed to create application", "error", err)
		return nil, err
	}

	return &RegisterApplicationResponse{
		ApplicationID: applicationID,
	}, nil
}
