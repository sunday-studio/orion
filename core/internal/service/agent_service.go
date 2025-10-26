package service

import (
	"fmt"
	"orion/core/internal/db"
	"orion/core/internal/logging"
	"orion/core/internal/utils"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type AgentService struct {
	db     *gorm.DB
	logger *logging.Logger
}

func NewAgentService(database *gorm.DB, logger *logging.Logger) *AgentService {
	return &AgentService{
		db:     database,
		logger: logger,
	}
}

type RegisterRequest struct {
	MachineId string `json:"machine_id" binding:"required"`
	Name      string `json:"name" binding:"required"`
	OS        string `json:"os" binding:"required"`
	Arch      string `json:"arch" binding:"required"`
}

type RegisterResponse struct {
	AgentID string `json:"agent_id"`
	Token   string `json:"token"`
}

func (s *AgentService) RegisterAgent(req *RegisterRequest) (*RegisterResponse, error) {
	var agent db.Agent

	if err := s.db.Where("machine_id = ?", req.MachineId).First(&agent).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return s.createNewAgent(req)
		}
		s.logger.Error("Database error during agent lookup", "error", err)
		return nil, err
	}

	s.logger.Info("Agent already registered", "machine_id", req.MachineId, "agent_id", agent.ID, "name", agent.Name)

	return &RegisterResponse{
		AgentID: agent.ID,
		Token:   agent.Token,
	}, nil
}

func (s *AgentService) createNewAgent(req *RegisterRequest) (*RegisterResponse, error) {

	agentID := fmt.Sprintf("%s_%s", "agent", uuid.New().String())

	token, err := utils.GenerateToken()
	if err != nil {
		s.logger.Error("Failed to generate token", "error", err)
		return nil, err
	}

	agent := db.Agent{
		ID:        agentID,
		MachineId: req.MachineId,
		Name:      req.Name,
		OS:        req.OS,
		Arch:      req.Arch,
		Token:     token,
		CreatedAt: time.Now(),
		LastSeen:  time.Now(),
	}

	if err := s.db.Create(&agent).Error; err != nil {
		s.logger.Error("Failed to create agent", "error", err)
		return nil, err
	}

	s.logger.Info("New agent registered", "machine_id", req.MachineId, "agent_id", agent.ID, "name", agent.Name)

	return &RegisterResponse{
		AgentID: agentID,
		Token:   agent.Token,
	}, nil
}

// UpdateLastSeen updates the last_seen timestamp for an agent
func (s *AgentService) UpdateLastSeen(agentID uint) error {
	if err := s.db.Model(&db.Agent{}).Where("id = ?", agentID).Update("last_seen", time.Now()).Error; err != nil {
		s.logger.Error("Failed to update last_seen", "agent_id", agentID, "error", err)
		return err
	}

	s.logger.Debug("Updated last_seen timestamp", "agent_id", agentID)
	return nil
}
