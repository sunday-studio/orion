package service

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"orion/core/internal/db"
	"orion/core/internal/utils"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestNodeTokenLifecycleSupportsLegacyPlaintextThenHashesRotation(t *testing.T) {
	database := setupNodeTokenLifecycleTestDatabase(t)
	logger := utils.NewLogger()
	now := time.Now().UTC()
	legacy := db.Node{
		ID:                       "node-legacy-token",
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
		t.Fatalf("create legacy node: %v", err)
	}

	authService := NewAuthService(database, logger)
	// A plaintext row can no longer authenticate until the startup migration hashes it.
	if _, err := authService.ValidateToken(legacy.ID, legacy.Token); err == nil {
		t.Fatal("ValidateToken() accepted an unmigrated plaintext token, want rejection")
	}
	migrated, err := MigrateLegacyNodeTokens(database, logger)
	if err != nil || migrated != 1 {
		t.Fatalf("MigrateLegacyNodeTokens() = %d, %v, want 1 row", migrated, err)
	}
	var afterMigration db.Node
	database.Where("id = ?", legacy.ID).First(&afterMigration)
	if afterMigration.Token == "legacy-plaintext-token" || afterMigration.TokenHash == "" {
		t.Fatalf("after migration token=%q hash=%q, want plaintext gone", afterMigration.Token, afterMigration.TokenHash)
	}
	if _, err := authService.ValidateToken(legacy.ID, "legacy-plaintext-token"); err != nil {
		t.Fatalf("the node's existing token must keep working after migration: %v", err)
	}
	if again, _ := MigrateLegacyNodeTokens(database, logger); again != 0 {
		t.Fatalf("second migration converted %d rows, want idempotent 0", again)
	}

	nodeService := NewNodeService(database, logger)
	rotated, err := nodeService.RotateNodeToken(legacy.ID, NodeTokenActionInput{
		ActorType: "user",
		ActorID:   "admin-user",
		Reason:    "routine rotation",
		RequestID: "request-rotate-1",
	})
	if err != nil {
		t.Fatalf("RotateNodeToken() error = %v", err)
	}
	if rotated.Token == "" || rotated.Status.TokenVersion != 2 || rotated.Status.State != NodeTokenStateActive || !rotated.Status.TokenExists {
		t.Fatalf("rotated result = %+v, want active version 2 with one-time token", rotated)
	}

	var stored db.Node
	if err := database.Where("id = ?", legacy.ID).First(&stored).Error; err != nil {
		t.Fatalf("find rotated node: %v", err)
	}
	if stored.Token == rotated.Token || !strings.HasPrefix(stored.Token, "sha256:") || stored.TokenHash == "" {
		t.Fatalf("stored token fields = token:%q hash:%q, want hashed marker and hash only", stored.Token, stored.TokenHash)
	}
	if _, err := authService.ValidateToken(legacy.ID, legacy.Token); err != nil {
		t.Fatalf("ValidateToken() with the replaced token during the grace window error = %v", err)
	}
	if err := database.Model(&db.Node{}).Where("id = ?", legacy.ID).Update("previous_token_expires_at", time.Now().UTC().Add(-time.Second)).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := authService.ValidateToken(legacy.ID, legacy.Token); err == nil {
		t.Fatal("ValidateToken() with legacy token after the grace window got nil error, want rejection")
	}
	if _, err := authService.ValidateToken(legacy.ID, rotated.Token); err != nil {
		t.Fatalf("ValidateToken() with rotated token error = %v", err)
	}

	var audit db.AuditEvent
	if err := database.Where("affected_object_id = ? AND action = ?", legacy.ID, NodeTokenAuditActionRotated).First(&audit).Error; err != nil {
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

func TestNodeTokenLifecycleRevocationClearsSecretsAndRequiresReissue(t *testing.T) {
	database := setupNodeTokenLifecycleTestDatabase(t)
	logger := utils.NewLogger()
	nodeService := NewNodeService(database, logger)
	registered, err := nodeService.RegisterNode(&RegisterRequest{
		MachineId:                "machine-revoke-token",
		Name:                     "revoked server",
		OS:                       "linux",
		Arch:                     "amd64",
		ReportingIntervalSeconds: 60,
	})
	if err != nil {
		t.Fatalf("RegisterNode() error = %v", err)
	}

	longReason := strings.Repeat("x", 550)
	revoked, err := nodeService.RevokeNodeToken(registered.NodeID, NodeTokenActionInput{Reason: longReason})
	if err != nil {
		t.Fatalf("RevokeNodeToken() error = %v", err)
	}
	if revoked.State != NodeTokenStateRevoked || revoked.TokenExists || revoked.TokenVersion != 2 {
		t.Fatalf("revoked status = %+v, want revoked version 2 without token", revoked)
	}

	var stored db.Node
	if err := database.Where("id = ?", registered.NodeID).First(&stored).Error; err != nil {
		t.Fatalf("find revoked node: %v", err)
	}
	if stored.TokenHash != "" || !strings.HasPrefix(stored.Token, "revoked:") || len(stored.TokenRevocationReason) != 500 {
		t.Fatalf("stored revoked token fields = token:%q hash:%q reason length:%d", stored.Token, stored.TokenHash, len(stored.TokenRevocationReason))
	}
	if _, err := nodeService.ValidateNodeToken(registered.NodeID, registered.Token); !errors.Is(err, ErrNodeTokenRevoked) {
		t.Fatalf("ValidateNodeToken() after revoke error = %v, want ErrNodeTokenRevoked", err)
	}
	if _, err := nodeService.RotateNodeToken(registered.NodeID, NodeTokenActionInput{}); !errors.Is(err, ErrNodeTokenRevoked) {
		t.Fatalf("RotateNodeToken() while revoked error = %v, want ErrNodeTokenRevoked", err)
	}

	reissued, err := nodeService.ReissueNodeToken(registered.NodeID, NodeTokenActionInput{Reason: "replacement installed"})
	if err != nil {
		t.Fatalf("ReissueNodeToken() error = %v", err)
	}
	if reissued.Token == "" || reissued.Token == registered.Token || reissued.Status.State != NodeTokenStateActive || reissued.Status.TokenVersion != 3 || !reissued.Status.TokenExists {
		t.Fatalf("reissued result = %+v, want active version 3 replacement token", reissued)
	}
	if _, err := nodeService.ValidateNodeToken(registered.NodeID, reissued.Token); err != nil {
		t.Fatalf("ValidateNodeToken() with reissued token error = %v", err)
	}
}

func setupNodeTokenLifecycleTestDatabase(t *testing.T) *gorm.DB {
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
