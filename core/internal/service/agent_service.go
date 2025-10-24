package service

import (
	"orion/core/internal/db"
	"orion/core/internal/logging"
	"orion/core/internal/utils"
	"time"

	"gorm.io/gorm"
)

// AgentService handles agent-related operations
type AgentService struct {
	db     *gorm.DB
	logger *logging.Logger
}

// NewAgentService creates a new agent service
func NewAgentService(database *gorm.DB, logger *logging.Logger) *AgentService {
	return &AgentService{
		db:     database,
		logger: logger,
	}
}

// RegisterRequest represents the registration request payload
type RegisterRequest struct {
	UUID string `json:"uuid" binding:"required"`
	Name string `json:"name" binding:"required"`
	OS   string `json:"os" binding:"required"`
	Arch string `json:"arch" binding:"required"`
}

// RegisterResponse represents the registration response
type RegisterResponse struct {
	AgentID uint   `json:"agent_id"`
	Token   string `json:"token"`
}

// RegisterAgent registers a new agent or returns existing agent info
func (s *AgentService) RegisterAgent(req *RegisterRequest) (*RegisterResponse, error) {
	var agent db.Agent
	
	// Check if agent with this UUID already exists
	if err := s.db.Where("uuid = ?", req.UUID).First(&agent).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			// Create new agent
			return s.createNewAgent(req)
		}
		s.logger.Error("Database error during agent lookup", "error", err)
		return nil, err
	}

	// Agent exists, return existing info
	s.logger.Info("Agent already registered", "uuid", req.UUID, "agent_id", agent.ID, "name", agent.Name)
	
	return &RegisterResponse{
		AgentID: agent.ID,
		Token:   agent.Token,
	}, nil
}

// createNewAgent creates a new agent record
func (s *AgentService) createNewAgent(req *RegisterRequest) (*RegisterResponse, error) {
	// Generate new token
	token, err := utils.GenerateToken()
	if err != nil {
		s.logger.Error("Failed to generate token", "error", err)
		return nil, err
	}

	// Create agent record
	agent := db.Agent{
		UUID:      req.UUID,
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

	s.logger.Info("New agent registered", "uuid", req.UUID, "agent_id", agent.ID, "name", agent.Name)

	return &RegisterResponse{
		AgentID: agent.ID,
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
