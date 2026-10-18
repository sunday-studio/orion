package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	apiTokenPrefix        = "orn_api_"
	apiTokenKindKey       = "frontend_auth_kind"
	apiTokenLastUsedEvery = time.Minute
	maxAPITokens          = 50
)

// APIToken is one credential for scripts and integrations (never includes the secret).
type APIToken struct {
	ID         string     `json:"id"`
	Name       string     `json:"name" example:"ci-deploys"`
	Prefix     string     `json:"prefix" example:"orn_api_3Fk9"`
	Scope      string     `json:"scope" enums:"read,write"`
	CreatedBy  string     `json:"created_by,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
}

type APITokenRequest struct {
	Name string `json:"name" binding:"required"`
	// Scope is read (GET requests only) or write (everything except token management).
	Scope string `json:"scope" enums:"read,write"`
}

// APITokenCreated is returned once, with the secret.
type APITokenCreated struct {
	APIToken
	Token string `json:"token" example:"orn_api_3Fk9d2a7c1e5b8046a9d3f2e1c7b5a40"`
}

func hashAPIToken(token string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(token)))
	return hex.EncodeToString(sum[:])
}

func toAPIToken(t db.APIToken) APIToken {
	return APIToken{ID: t.ID, Name: t.Name, Prefix: t.Prefix, Scope: t.Scope, CreatedBy: t.CreatedBy, CreatedAt: t.CreatedAt, LastUsedAt: t.LastUsedAt}
}

// requireSession refuses API-token callers: tokens cannot manage tokens or the registration token.
// It reports whether the request may continue.
func (s *Server) requireSession(c *gin.Context) bool {
	if kind, _ := c.Get(apiTokenKindKey); kind == "api_token" {
		utils.ErrorResponse(c, http.StatusForbidden, "API tokens cannot manage tokens or registration. Sign in to the Console.", nil)
		return false
	}
	return true
}

// authenticateAPIToken resolves an orn_api_ bearer value, refusing revoked tokens. It updates last_used_at at most once a minute.
func (s *Server) authenticateAPIToken(token string) (db.APIToken, bool) {
	var row db.APIToken
	if err := s.db.Where("token_hash = ? AND revoked_at IS NULL", hashAPIToken(token)).First(&row).Error; err != nil {
		return row, false
	}
	now := time.Now().UTC()
	if row.LastUsedAt == nil || now.Sub(*row.LastUsedAt) > apiTokenLastUsedEvery {
		s.db.Model(&db.APIToken{}).Where("id = ?", row.ID).Update("last_used_at", now)
	}
	return row, true
}

func (s *Server) listAPITokensImpl(c *gin.Context) {
	if !s.requireSession(c) {
		return
	}
	var rows []db.APIToken
	if err := s.db.Where("revoked_at IS NULL").Order("created_at DESC").Find(&rows).Error; err != nil {
		utils.InternalError(c, "Failed to list API tokens", err)
		return
	}
	out := make([]APIToken, 0, len(rows))
	for _, r := range rows {
		out = append(out, toAPIToken(r))
	}
	utils.SuccessResponse(c, http.StatusOK, "API tokens retrieved", gin.H{"tokens": out})
}

func (s *Server) createAPITokenImpl(c *gin.Context) {
	if !s.requireSession(c) {
		return
	}
	var req APITokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "name is required")
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" || len([]rune(name)) > 60 {
		utils.BadRequest(c, "name is required (60 characters at most)")
		return
	}
	scope := strings.ToLower(strings.TrimSpace(req.Scope))
	if scope == "" {
		scope = "read"
	}
	if scope != "read" && scope != "write" {
		utils.BadRequest(c, "scope must be read or write")
		return
	}
	var taken, total int64
	s.db.Model(&db.APIToken{}).Where("revoked_at IS NULL AND lower(name) = ?", strings.ToLower(name)).Count(&taken)
	s.db.Model(&db.APIToken{}).Where("revoked_at IS NULL").Count(&total)
	if taken > 0 {
		utils.ErrorResponse(c, http.StatusConflict, "A token with that name already exists", nil)
		return
	}
	if total >= maxAPITokens {
		utils.BadRequest(c, "At most 50 API tokens; revoke one you no longer use")
		return
	}
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		utils.InternalError(c, "Failed to generate a token", err)
		return
	}
	token := apiTokenPrefix + hex.EncodeToString(raw[:])
	row := db.APIToken{ID: utils.GenerateID("api_token"), Name: name, Prefix: token[:len(apiTokenPrefix)+4], TokenHash: hashAPIToken(token), Scope: scope, CreatedBy: frontendActorID(c), CreatedAt: time.Now().UTC()}
	if err := s.db.Create(&row).Error; err != nil {
		utils.InternalError(c, "Failed to store the token", err)
		return
	}
	s.logger.Info("API token created", "name", name, "scope", scope, "actor", row.CreatedBy)
	utils.SuccessResponse(c, http.StatusCreated, "API token created. Copy it now: Orion only shows it once.", gin.H{"token": APITokenCreated{APIToken: toAPIToken(row), Token: token}})
}

func (s *Server) revokeAPITokenImpl(c *gin.Context) {
	if !s.requireSession(c) {
		return
	}
	res := s.db.Model(&db.APIToken{}).Where("id = ? AND revoked_at IS NULL", c.Param("id")).Update("revoked_at", time.Now().UTC())
	if res.Error != nil {
		utils.InternalError(c, "Failed to revoke the token", res.Error)
		return
	}
	if res.RowsAffected == 0 {
		utils.NotFound(c, "API token not found")
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "API token revoked", nil)
}
