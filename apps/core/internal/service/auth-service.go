package service

import (
	"orion/core/internal/db"
	"orion/core/internal/logging"

	"gorm.io/gorm"
)

// AuthService handles authentication-related operations
type AuthService struct {
	db     *gorm.DB
	logger *logging.Logger
}

// NewAuthService creates a new authentication service
func NewAuthService(database *gorm.DB, logger *logging.Logger) *AuthService {
	return &AuthService{
		db:     database,
		logger: logger,
	}
}

// ValidateToken checks if a token is valid and belongs to the specified agent
func (s *AuthService) ValidateToken(agentID string, token string) (*db.Agent, error) {
	var agent db.Agent

	if err := s.db.Where("id = ?", agentID).First(&agent).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			s.logger.Warn("Invalid token for agent", "agent_id", agentID)
			return nil, err
		}
		s.logger.Error("Database error during token validation", "error", err)
		return nil, err
	}
	if agent.TokenRevokedAt != nil {
		s.logger.Warn("Rejected revoked token for agent", "agent_id", agentID)
		return nil, ErrAgentTokenRevoked
	}
	if !agentTokenMatches(agent, token) {
		s.logger.Warn("Invalid token for agent", "agent_id", agentID)
		return nil, gorm.ErrRecordNotFound
	}

	s.logger.Debug("Token validated successfully", "agent_id", agentID, "agent_name", agent.Name)
	return &agent, nil
}
