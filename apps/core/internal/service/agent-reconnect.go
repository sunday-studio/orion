package service

import (
	"orion/core/internal/db"

	"gorm.io/gorm"
)

// ReconnectAgent uses a fresh, one-time Add server authorization to restore a
// known machine whose local token was lost. Identity and monitor ownership stay
// intact; only the credential and current host metadata change.
func (s *AgentService) ReconnectAgent(req *RegisterRequest, enrollmentID string) (*RegisterResponse, error) {
	var agent db.Agent
	if err := s.db.Where("machine_id = ?", req.MachineId).First(&agent).Error; err != nil {
		return nil, err
	}
	if agent.TokenRevokedAt != nil {
		return nil, ErrAgentTokenRevoked
	}

	interval := req.ReportingIntervalSeconds
	if interval <= 0 {
		interval = 60
	}
	if err := s.db.Model(&agent).Updates(map[string]any{
		"name":                       req.Name,
		"os":                         req.OS,
		"arch":                       req.Arch,
		"reporting_interval_seconds": interval,
		"meta":                       req.Meta,
	}).Error; err != nil {
		return nil, err
	}

	issued, err := s.RotateAgentToken(agent.ID, AgentTokenActionInput{
		ActorType: "enrollment",
		ActorID:   enrollmentID,
		Reason:    "host reconnected with a one-time Add server token",
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrAgentTokenReissueRequired
		}
		return nil, err
	}
	return &RegisterResponse{AgentID: agent.ID, Token: issued.Token}, nil
}
