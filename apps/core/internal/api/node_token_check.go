package api

import (
	"net/http"
	"orion/core/internal/utils"

	"github.com/gin-gonic/gin"
)

// checkNodeToken tells a node whether Core accepts its credentials.
// @Summary      Check a node token
// @Description  Succeeds when the bearer token belongs to this node. It changes nothing and returns nothing, so a node (or its doctor and setup) can ask "does Core accept my token?" without calling an admin route, which a node token cannot use once Console authentication is on.
// @Tags         nodes
// @Produce      json
// @Security     BearerAuth
// @ID           checkNodeToken
// @Param        node_id  path      string  true  "Node ID"
// @Success      200       {object}  utils.APIResponse
// @Failure      401       {object}  utils.APIResponse
// @Router       /v1/nodes/{node_id}/token-check [post]
func (s *Server) checkNodeToken(c *gin.Context) {
	utils.SuccessResponse(c, http.StatusOK, "Token accepted", gin.H{})
}
