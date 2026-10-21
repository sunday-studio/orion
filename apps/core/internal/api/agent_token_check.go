package api

import (
	"net/http"
	"orion/core/internal/utils"

	"github.com/gin-gonic/gin"
)

// checkAgentToken tells an agent whether Core accepts its credentials.
// @Summary      Check an agent token
// @Description  Succeeds when the bearer token belongs to this agent. It changes nothing and returns nothing, so an agent (or its doctor and setup) can ask "does Core accept my token?" without calling an admin route, which an agent token cannot use once Console authentication is on.
// @Tags         agents
// @Produce      json
// @Security     BearerAuth
// @ID           checkAgentToken
// @Param        agent_id  path      string  true  "Agent ID"
// @Success      200       {object}  utils.APIResponse
// @Failure      401       {object}  utils.APIResponse
// @Router       /v1/agents/{agent_id}/token-check [post]
func (s *Server) checkAgentToken(c *gin.Context) {
	utils.SuccessResponse(c, http.StatusOK, "Token accepted", gin.H{})
}
