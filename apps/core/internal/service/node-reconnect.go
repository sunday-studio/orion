package service

import (
	"orion/core/internal/db"

	"gorm.io/gorm"
)

// ReconnectNode uses a fresh, one-time Add server authorization to restore a
// known machine whose local token was lost. Identity and monitor ownership stay
// intact; only the credential and current host metadata change.
func (s *NodeService) ReconnectNode(req *RegisterRequest, enrollmentID string) (*RegisterResponse, error) {
	var node db.Node
	if err := s.db.Where("machine_id = ?", req.MachineId).First(&node).Error; err != nil {
		return nil, err
	}
	if node.TokenRevokedAt != nil {
		return nil, ErrNodeTokenRevoked
	}

	interval := req.ReportingIntervalSeconds
	if interval <= 0 {
		interval = 60
	}
	if err := s.db.Model(&node).Updates(map[string]any{
		"name":                       req.Name,
		"os":                         req.OS,
		"arch":                       req.Arch,
		"reporting_interval_seconds": interval,
		"meta":                       req.Meta,
	}).Error; err != nil {
		return nil, err
	}

	issued, err := s.RotateNodeToken(node.ID, NodeTokenActionInput{
		ActorType: "enrollment",
		ActorID:   enrollmentID,
		Reason:    "host reconnected with a one-time Add server token",
	})
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrNodeTokenReissueRequired
		}
		return nil, err
	}
	return &RegisterResponse{NodeID: node.ID, Token: issued.Token}, nil
}
