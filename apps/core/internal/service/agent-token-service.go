package service

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"orion/core/internal/db"
	"orion/core/internal/utils"
)

type AgentTokenStatus struct {
	AgentID               string     `json:"agent_id"`
	State                 string     `json:"state"`
	TokenVersion          int        `json:"token_version"`
	TokenRotatedAt        *time.Time `json:"token_rotated_at,omitempty"`
	TokenRevokedAt        *time.Time `json:"token_revoked_at,omitempty"`
	TokenRevocationReason string     `json:"token_revocation_reason,omitempty"`
	TokenExists           bool       `json:"token_exists"`
}

type AgentTokenActionInput struct {
	ActorType string
	ActorID   string
	Reason    string
	RequestID string
}

type AgentTokenIssueResult struct {
	Token  string
	Status AgentTokenStatus
}

func (s *AgentService) ValidateAgentToken(agentID string, token string) (*db.Agent, error) {
	var agent db.Agent
	if err := s.db.Where("id = ?", agentID).First(&agent).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			s.logger.Warn("Invalid token for missing agent", "agent_id", agentID)
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

func (s *AgentService) RotateAgentToken(agentID string, input AgentTokenActionInput) (*AgentTokenIssueResult, error) {
	return s.issueAgentToken(agentID, input, AgentTokenAuditActionRotated, false)
}

func (s *AgentService) ReissueAgentToken(agentID string, input AgentTokenActionInput) (*AgentTokenIssueResult, error) {
	return s.issueAgentToken(agentID, input, AgentTokenAuditActionReissued, true)
}

func (s *AgentService) RevokeAgentToken(agentID string, input AgentTokenActionInput) (*AgentTokenStatus, error) {
	now := time.Now().UTC()
	reason := sanitizeAgentTokenReason(input.Reason)

	var status AgentTokenStatus
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var agent db.Agent
		if err := tx.Where("id = ?", agentID).First(&agent).Error; err != nil {
			return err
		}
		if agent.TokenRevokedAt != nil {
			status = agentTokenStatus(agent)
			return nil
		}

		updates := map[string]any{
			"token":                   revokedAgentTokenMarker(agent.ID, agent.TokenVersion+1),
			"token_hash":              "",
			"token_version":           normalizedAgentTokenVersion(agent.TokenVersion) + 1,
			"token_revoked_at":        now,
			"token_revocation_reason": reason,
		}
		if err := tx.Model(&db.Agent{}).Where("id = ?", agent.ID).Updates(updates).Error; err != nil {
			return err
		}
		var updatedAgent db.Agent
		if err := tx.Where("id = ?", agent.ID).First(&updatedAgent).Error; err != nil {
			return err
		}
		status = agentTokenStatus(updatedAgent)
		return recordAgentTokenAuditEvent(tx, AgentTokenAuditActionRevoked, agent.ID, input, status.TokenVersion)
	})
	if err != nil {
		return nil, err
	}
	return &status, nil
}

func (s *AgentService) issueAgentToken(agentID string, input AgentTokenActionInput, auditAction string, requireRevoked bool) (*AgentTokenIssueResult, error) {
	token, err := utils.GenerateToken()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()

	var result AgentTokenIssueResult
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var agent db.Agent
		if err := tx.Where("id = ?", agentID).First(&agent).Error; err != nil {
			return err
		}
		if requireRevoked && agent.TokenRevokedAt == nil {
			return ErrAgentTokenNotRevoked
		}
		if !requireRevoked && agent.TokenRevokedAt != nil {
			return ErrAgentTokenRevoked
		}

		nextVersion := normalizedAgentTokenVersion(agent.TokenVersion) + 1
		updates := map[string]any{
			"token":                   storedAgentTokenMarker(token),
			"token_hash":              hashAgentToken(token),
			"token_version":           nextVersion,
			"token_rotated_at":        now,
			"token_revocation_reason": "",
		}
		if err := tx.Model(&db.Agent{}).Where("id = ?", agent.ID).Updates(updates).Error; err != nil {
			return err
		}
		if err := tx.Model(&db.Agent{}).Where("id = ?", agent.ID).UpdateColumn("token_revoked_at", gorm.Expr("NULL")).Error; err != nil {
			return err
		}
		var updatedAgent db.Agent
		if err := tx.Where("id = ?", agent.ID).First(&updatedAgent).Error; err != nil {
			return err
		}
		result = AgentTokenIssueResult{
			Token:  token,
			Status: agentTokenStatus(updatedAgent),
		}
		return recordAgentTokenAuditEvent(tx, auditAction, updatedAgent.ID, input, result.Status.TokenVersion)
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func agentTokenMatches(agent db.Agent, token string) bool {
	token = strings.TrimSpace(token)
	if token == "" {
		return false
	}
	if agent.TokenHash != "" {
		return subtle.ConstantTimeCompare([]byte(agent.TokenHash), []byte(hashAgentToken(token))) == 1
	}
	return subtle.ConstantTimeCompare([]byte(agent.Token), []byte(token)) == 1
}

func agentTokenStatus(agent db.Agent) AgentTokenStatus {
	state := AgentTokenStateActive
	tokenExists := agent.TokenHash != "" || (agent.Token != "" && !strings.HasPrefix(agent.Token, "revoked:"))
	if agent.TokenRevokedAt != nil {
		state = AgentTokenStateRevoked
		tokenExists = false
	}
	return AgentTokenStatus{
		AgentID:               agent.ID,
		State:                 state,
		TokenVersion:          normalizedAgentTokenVersion(agent.TokenVersion),
		TokenRotatedAt:        agent.TokenRotatedAt,
		TokenRevokedAt:        agent.TokenRevokedAt,
		TokenRevocationReason: agent.TokenRevocationReason,
		TokenExists:           tokenExists,
	}
}

func normalizedAgentTokenVersion(version int) int {
	if version <= 0 {
		return 1
	}
	return version
}

func hashAgentToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

func storedAgentTokenMarker(token string) string {
	return "sha256:" + hashAgentToken(token)
}

func revokedAgentTokenMarker(agentID string, version int) string {
	return fmt.Sprintf("revoked:%s:%d", agentID, version)
}

func sanitizeAgentTokenReason(reason string) string {
	reason = strings.TrimSpace(reason)
	if len(reason) > 500 {
		reason = reason[:500]
	}
	return reason
}

func recordAgentTokenAuditEvent(tx *gorm.DB, action string, agentID string, input AgentTokenActionInput, tokenVersion int) error {
	actorType := strings.TrimSpace(input.ActorType)
	if actorType == "" {
		actorType = "user"
	}
	actorID := strings.TrimSpace(input.ActorID)
	if actorID == "" {
		actorID = "admin"
	}
	metadata, err := json.Marshal(map[string]any{
		"token_version": tokenVersion,
		"reason":        sanitizeAgentTokenReason(input.Reason),
		"request_id":    strings.TrimSpace(input.RequestID),
	})
	if err != nil {
		return err
	}
	event := db.AuditEvent{
		ID:                 utils.GenerateID("audit_event"),
		Action:             action,
		StatusPageID:       "",
		AffectedObjectType: "agent",
		AffectedObjectID:   agentID,
		ActorType:          actorType,
		ActorID:            actorID,
		MetadataJSON:       string(metadata),
		CreatedAt:          time.Now().UTC(),
	}
	return tx.Create(&event).Error
}
