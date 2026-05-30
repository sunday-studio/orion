package service

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"orion/core/internal/db"
	"orion/core/internal/logging"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAgentTokenLifecycleSupportsLegacyPlaintextThenHashesRotation(t *testing.T) {
	database := setupAgentTokenLifecycleTestDatabase(t)
	logger := logging.NewLogger()
	now := time.Now().UTC()
	legacy := db.Agent{
		ID:                       "agent-legacy-token",
		MachineId:                "machine-legacy-token",
		Name:                     "legacy server",
		OS:                       "linux",
		Arch:                     "amd64",
		Token:                    "legacy-plaintext-token",
		TokenVersion:             0,
		ReportingIntervalSeconds: 60,
		CreatedAt:                now,
		LastSeen:                 now,
	}
	if err := database.Create(&legacy).Error; err != nil {
		t.Fatalf("create legacy agent: %v", err)
	}

	authService := NewAuthService(database, logger)
	if _, err := authService.ValidateToken(legacy.ID, legacy.Token); err != nil {
		t.Fatalf("ValidateToken() legacy plaintext error = %v", err)
	}

	agentService := NewAgentService(database, logger)
	rotated, err := agentService.RotateAgentToken(legacy.ID, AgentTokenActionInput{
		ActorType: "user",
		ActorID:   "admin-user",
		Reason:    "routine rotation",
		RequestID: "request-rotate-1",
	})
	if err != nil {
		t.Fatalf("RotateAgentToken() error = %v", err)
	}
	if rotated.Token == "" || rotated.Status.TokenVersion != 2 || rotated.Status.State != AgentTokenStateActive || !rotated.Status.TokenExists {
		t.Fatalf("rotated result = %+v, want active version 2 with one-time token", rotated)
	}

	var stored db.Agent
	if err := database.Where("id = ?", legacy.ID).First(&stored).Error; err != nil {
		t.Fatalf("find rotated agent: %v", err)
	}
	if stored.Token == rotated.Token || !strings.HasPrefix(stored.Token, "sha256:") || stored.TokenHash == "" {
		t.Fatalf("stored token fields = token:%q hash:%q, want hashed marker and hash only", stored.Token, stored.TokenHash)
	}
	if _, err := authService.ValidateToken(legacy.ID, legacy.Token); err == nil {
		t.Fatal("ValidateToken() with legacy token after rotation got nil error, want rejection")
	}
	if _, err := authService.ValidateToken(legacy.ID, rotated.Token); err != nil {
		t.Fatalf("ValidateToken() with rotated token error = %v", err)
	}

	var audit db.AuditEvent
	if err := database.Where("affected_object_id = ? AND action = ?", legacy.ID, AgentTokenAuditActionRotated).First(&audit).Error; err != nil {
		t.Fatalf("find rotation audit event: %v", err)
	}
	var metadata map[string]interface{}
	if err := json.Unmarshal([]byte(audit.MetadataJSON), &metadata); err != nil {
		t.Fatalf("decode audit metadata: %v", err)
	}
	if metadata["request_id"] != "request-rotate-1" || metadata["reason"] != "routine rotation" || metadata["token_version"].(float64) != 2 {
		t.Fatalf("audit metadata = %+v, want request, reason, and token version", metadata)
	}
}

func TestAgentTokenLifecycleRevocationClearsSecretsAndRequiresReissue(t *testing.T) {
	database := setupAgentTokenLifecycleTestDatabase(t)
	logger := logging.NewLogger()
	agentService := NewAgentService(database, logger)
	registered, err := agentService.RegisterAgent(&RegisterRequest{
		MachineId:                "machine-revoke-token",
		Name:                     "revoked server",
		OS:                       "linux",
		Arch:                     "amd64",
		ReportingIntervalSeconds: 60,
	})
	if err != nil {
		t.Fatalf("RegisterAgent() error = %v", err)
	}

	longReason := strings.Repeat("x", 550)
	revoked, err := agentService.RevokeAgentToken(registered.AgentID, AgentTokenActionInput{Reason: longReason})
	if err != nil {
		t.Fatalf("RevokeAgentToken() error = %v", err)
	}
	if revoked.State != AgentTokenStateRevoked || revoked.TokenExists || revoked.TokenVersion != 2 {
		t.Fatalf("revoked status = %+v, want revoked version 2 without token", revoked)
	}

	var stored db.Agent
	if err := database.Where("id = ?", registered.AgentID).First(&stored).Error; err != nil {
		t.Fatalf("find revoked agent: %v", err)
	}
	if stored.TokenHash != "" || !strings.HasPrefix(stored.Token, "revoked:") || len(stored.TokenRevocationReason) != 500 {
		t.Fatalf("stored revoked token fields = token:%q hash:%q reason length:%d", stored.Token, stored.TokenHash, len(stored.TokenRevocationReason))
	}
	if _, err := agentService.ValidateAgentToken(registered.AgentID, registered.Token); !errors.Is(err, ErrAgentTokenRevoked) {
		t.Fatalf("ValidateAgentToken() after revoke error = %v, want ErrAgentTokenRevoked", err)
	}
	if _, err := agentService.RotateAgentToken(registered.AgentID, AgentTokenActionInput{}); !errors.Is(err, ErrAgentTokenRevoked) {
		t.Fatalf("RotateAgentToken() while revoked error = %v, want ErrAgentTokenRevoked", err)
	}

	reissued, err := agentService.ReissueAgentToken(registered.AgentID, AgentTokenActionInput{Reason: "replacement installed"})
	if err != nil {
		t.Fatalf("ReissueAgentToken() error = %v", err)
	}
	if reissued.Token == "" || reissued.Token == registered.Token || reissued.Status.State != AgentTokenStateActive || reissued.Status.TokenVersion != 3 || !reissued.Status.TokenExists {
		t.Fatalf("reissued result = %+v, want active version 3 replacement token", reissued)
	}
	if _, err := agentService.ValidateAgentToken(registered.AgentID, reissued.Token); err != nil {
		t.Fatalf("ValidateAgentToken() with reissued token error = %v", err)
	}
}

func setupAgentTokenLifecycleTestDatabase(t *testing.T) *gorm.DB {
	t.Helper()

	database, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.Migrate(database); err != nil {
		t.Fatalf("migrate database: %v", err)
	}
	return database
}
