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

// nodeTokenRotationGrace is how long a rotated-away token keeps working.
const nodeTokenRotationGrace = 10 * time.Minute

type NodeTokenStatus struct {
	NodeID                string     `json:"node_id"`
	State                 string     `json:"state"`
	TokenVersion          int        `json:"token_version"`
	TokenRotatedAt        *time.Time `json:"token_rotated_at,omitempty"`
	TokenRevokedAt        *time.Time `json:"token_revoked_at,omitempty"`
	TokenRevocationReason string     `json:"token_revocation_reason,omitempty"`
	TokenExists           bool       `json:"token_exists"`
	// LastUsedAt is when the node last authenticated (its last report).
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	// PreviousTokenValidUntil is set for a few minutes after a rotation.
	PreviousTokenValidUntil *time.Time `json:"previous_token_valid_until,omitempty"`
}

type NodeTokenActionInput struct {
	ActorType string
	ActorID   string
	Reason    string
	RequestID string
}

type NodeTokenIssueResult struct {
	Token  string
	Status NodeTokenStatus
}

func (s *NodeService) ValidateNodeToken(nodeID string, token string) (*db.Node, error) {
	var node db.Node
	if err := s.db.Where("id = ?", nodeID).First(&node).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			s.logger.Warn("Invalid token for missing node", "node_id", nodeID)
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

func (s *NodeService) RotateNodeToken(nodeID string, input NodeTokenActionInput) (*NodeTokenIssueResult, error) {
	return s.issueNodeToken(nodeID, input, NodeTokenAuditActionRotated, false)
}

func (s *NodeService) ReissueNodeToken(nodeID string, input NodeTokenActionInput) (*NodeTokenIssueResult, error) {
	return s.issueNodeToken(nodeID, input, NodeTokenAuditActionReissued, true)
}

func (s *NodeService) RevokeNodeToken(nodeID string, input NodeTokenActionInput) (*NodeTokenStatus, error) {
	now := time.Now().UTC()
	reason := sanitizeNodeTokenReason(input.Reason)

	var status NodeTokenStatus
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var node db.Node
		if err := tx.Where("id = ?", nodeID).First(&node).Error; err != nil {
			return err
		}
		if node.TokenRevokedAt != nil {
			status = nodeTokenStatus(node)
			return nil
		}

		updates := map[string]any{
			"token":                     revokedNodeTokenMarker(node.ID, node.TokenVersion+1),
			"token_hash":                "",
			"token_version":             normalizedNodeTokenVersion(node.TokenVersion) + 1,
			"token_revoked_at":          now,
			"token_revocation_reason":   reason,
			"previous_token_hash":       "",
			"previous_token_expires_at": nil,
		}
		if err := tx.Model(&db.Node{}).Where("id = ?", node.ID).Updates(updates).Error; err != nil {
			return err
		}
		var updatedNode db.Node
		if err := tx.Where("id = ?", node.ID).First(&updatedNode).Error; err != nil {
			return err
		}
		status = nodeTokenStatus(updatedNode)
		return recordNodeTokenAuditEvent(tx, NodeTokenAuditActionRevoked, node.ID, input, status.TokenVersion)
	})
	if err != nil {
		return nil, err
	}
	return &status, nil
}

func (s *NodeService) issueNodeToken(nodeID string, input NodeTokenActionInput, auditAction string, requireRevoked bool) (*NodeTokenIssueResult, error) {
	token, err := utils.GenerateToken()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()

	var result NodeTokenIssueResult
	err = s.db.Transaction(func(tx *gorm.DB) error {
		var node db.Node
		if err := tx.Where("id = ?", nodeID).First(&node).Error; err != nil {
			return err
		}
		if requireRevoked && node.TokenRevokedAt == nil {
			return ErrNodeTokenNotRevoked
		}
		if !requireRevoked && node.TokenRevokedAt != nil {
			return ErrNodeTokenRevoked
		}

		nextVersion := normalizedNodeTokenVersion(node.TokenVersion) + 1
		updates := map[string]any{
			"token":                     storedNodeTokenMarker(token),
			"token_hash":                hashNodeToken(token),
			"token_version":             nextVersion,
			"token_rotated_at":          now,
			"token_revocation_reason":   "",
			"previous_token_hash":       "",
			"previous_token_expires_at": nil,
		}
		if !requireRevoked && node.TokenHash != "" {
			// A plain rotation keeps the old token alive briefly; reissuing after revocation never does.
			updates["previous_token_hash"] = node.TokenHash
			updates["previous_token_expires_at"] = now.Add(nodeTokenRotationGrace)
		}
		if err := tx.Model(&db.Node{}).Where("id = ?", node.ID).Updates(updates).Error; err != nil {
			return err
		}
		if err := tx.Model(&db.Node{}).Where("id = ?", node.ID).UpdateColumn("token_revoked_at", gorm.Expr("NULL")).Error; err != nil {
			return err
		}
		var updatedNode db.Node
		if err := tx.Where("id = ?", node.ID).First(&updatedNode).Error; err != nil {
			return err
		}
		result = NodeTokenIssueResult{
			Token:  token,
			Status: nodeTokenStatus(updatedNode),
		}
		return recordNodeTokenAuditEvent(tx, auditAction, updatedNode.ID, input, result.Status.TokenVersion)
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func nodeTokenMatches(node db.Node, token string) bool {
	token = strings.TrimSpace(token)
	if token == "" {
		return false
	}
	// Only the hash is ever compared. Legacy plaintext rows are converted to
	// hashes at startup by MigrateLegacyNodeTokens; a row without a hash cannot authenticate.
	if node.TokenHash == "" {
		return false
	}
	presented := []byte(hashNodeToken(token))
	if subtle.ConstantTimeCompare([]byte(node.TokenHash), presented) == 1 {
		return true
	}
	return node.PreviousTokenHash != "" && node.PreviousTokenExpiresAt != nil && time.Now().UTC().Before(*node.PreviousTokenExpiresAt) &&
		subtle.ConstantTimeCompare([]byte(node.PreviousTokenHash), presented) == 1
}

func nodeTokenStatus(node db.Node) NodeTokenStatus {
	state := NodeTokenStateActive
	tokenExists := node.TokenHash != "" || (node.Token != "" && !strings.HasPrefix(node.Token, "revoked:"))
	if node.TokenRevokedAt != nil {
		state = NodeTokenStateRevoked
		tokenExists = false
	}
	lastUsed := node.LastSeen
	var graceUntil *time.Time
	if node.PreviousTokenHash != "" && node.PreviousTokenExpiresAt != nil && time.Now().UTC().Before(*node.PreviousTokenExpiresAt) {
		graceUntil = node.PreviousTokenExpiresAt
	}
	return NodeTokenStatus{
		LastUsedAt:              &lastUsed,
		PreviousTokenValidUntil: graceUntil,
		NodeID:                  node.ID,
		State:                   state,
		TokenVersion:            normalizedNodeTokenVersion(node.TokenVersion),
		TokenRotatedAt:          node.TokenRotatedAt,
		TokenRevokedAt:          node.TokenRevokedAt,
		TokenRevocationReason:   node.TokenRevocationReason,
		TokenExists:             tokenExists,
	}
}

func normalizedNodeTokenVersion(version int) int {
	if version <= 0 {
		return 1
	}
	return version
}

// MigrateLegacyNodeTokens converts nodes that still store a plaintext token
// (token_hash empty) to the hashed form, replacing the plaintext with a marker.
// Existing nodes keep working: their stored token hashes to the same value they
// already present. It is idempotent and returns how many rows were converted.
func MigrateLegacyNodeTokens(database *gorm.DB, logger *utils.Logger) (int, error) {
	var legacy []db.Node
	if err := database.Where("token_hash = ? AND token <> ? AND token NOT LIKE ? AND token NOT LIKE ?", "", "", "revoked:%", "sha256:%").Find(&legacy).Error; err != nil {
		return 0, err
	}
	for _, node := range legacy {
		updates := map[string]any{
			"token_hash": hashNodeToken(node.Token),
			"token":      storedNodeTokenMarker(node.Token),
		}
		if err := database.Model(&db.Node{}).Where("id = ? AND token_hash = ?", node.ID, "").Updates(updates).Error; err != nil {
			return 0, err
		}
	}
	if len(legacy) > 0 {
		logger.Info("Migrated legacy plaintext node tokens to hashes", "count", len(legacy))
	}
	return len(legacy), nil
}

func hashNodeToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

func storedNodeTokenMarker(token string) string {
	return "sha256:" + hashNodeToken(token)
}

func revokedNodeTokenMarker(nodeID string, version int) string {
	return fmt.Sprintf("revoked:%s:%d", nodeID, version)
}

func sanitizeNodeTokenReason(reason string) string {
	reason = strings.TrimSpace(reason)
	if len(reason) > 500 {
		reason = reason[:500]
	}
	return reason
}

func recordNodeTokenAuditEvent(tx *gorm.DB, action string, nodeID string, input NodeTokenActionInput, tokenVersion int) error {
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
		"reason":        sanitizeNodeTokenReason(input.Reason),
		"request_id":    strings.TrimSpace(input.RequestID),
	})
	if err != nil {
		return err
	}
	event := db.AuditEvent{
		ID:                 utils.GenerateID("audit_event"),
		Action:             action,
		StatusPageID:       "",
		AffectedObjectType: "node",
		AffectedObjectID:   nodeID,
		ActorType:          actorType,
		ActorID:            actorID,
		MetadataJSON:       string(metadata),
		CreatedAt:          time.Now().UTC(),
	}
	return tx.Create(&event).Error
}
