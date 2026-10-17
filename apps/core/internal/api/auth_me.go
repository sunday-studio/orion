package api

import (
	"net/http"
	"orion/core/internal/utils"

	"github.com/gin-gonic/gin"
)

// MeResponse identifies the signed-in Console user.
type MeResponse struct {
	Username string `json:"username" example:"admin"`
	// AuthEnabled is false when Core runs with ORION_ALLOW_INSECURE_NO_AUTH.
	AuthEnabled bool `json:"auth_enabled"`
}

// authMe returns the account shown in the greeting and account menu.
// @Summary      Current user
// @Tags         auth
// @Produce      json
// @ID           getMe
// @Success      200  {object}  utils.APIResponse{data=MeResponse}
// @Router       /v1/auth/me [get]
func (s *Server) authMe(c *gin.Context) {
	name := s.cfg.AdminUsername
	if !s.cfg.FrontendAuthOn || name == "" {
		name = "admin"
	}
	utils.SuccessResponse(c, http.StatusOK, "Current user", MeResponse{Username: name, AuthEnabled: s.cfg.FrontendAuthOn})
}
