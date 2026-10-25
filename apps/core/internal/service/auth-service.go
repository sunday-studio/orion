package service

import (
	"orion/core/internal/db"
	"orion/core/internal/utils"

	"gorm.io/gorm"
)

// AuthService handles authentication-related operations
type AuthService struct {
	db     *gorm.DB
	logger *utils.Logger
}

// NewAuthService creates a new authentication service
func NewAuthService(database *gorm.DB, logger *utils.Logger) *AuthService {
	return &AuthService{
		db:     database,
		logger: logger,
	}
}

// ValidateToken checks if a token is valid and belongs to the specified node
func (s *AuthService) ValidateToken(nodeID string, token string) (*db.Node, error) {
	var node db.Node

	if err := s.db.Where("id = ?", nodeID).First(&node).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			s.logger.Warn("Invalid token for node", "node_id", nodeID)
			return nil, err
		}
		s.logger.Error("Database error during token validation", "error", err)
		return nil, err
	}
	if node.TokenRevokedAt != nil {
		s.logger.Warn("Rejected revoked token for node", "node_id", nodeID)
		return nil, ErrNodeTokenRevoked
	}
	if !nodeTokenMatches(node, token) {
		s.logger.Warn("Invalid token for node", "node_id", nodeID)
		return nil, gorm.ErrRecordNotFound
	}

	s.logger.Debug("Token validated successfully", "node_id", nodeID, "node_name", node.Name)
	return &node, nil
}
