package api

import (
	"errors"
	"orion/core/internal/service"
	"orion/core/internal/utils"
	"strings"

	"github.com/gin-gonic/gin"
)

// AuthMiddleware validates the Authorization header for protected routes
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			utils.Unauthorized(c, "Authorization header required")
			c.Abort()
			return
		}

		// Check if it starts with "Bearer "
		if !strings.HasPrefix(authHeader, "Bearer ") {
			utils.Unauthorized(c, "Invalid authorization format. Expected 'Bearer <token>'")
			c.Abort()
			return
		}

		// Extract token
		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == "" {
			utils.Unauthorized(c, "Token cannot be empty")
			c.Abort()
			return
		}

		// Store token in context for use in handlers
		c.Set("token", token)
		c.Next()
	}
}

func ValidateNodeToken(nodeService *service.NodeService, authService *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		nodeIDStr := c.Param("node_id")
		if nodeIDStr == "" {
			utils.BadRequest(c, "Node ID is required")
			c.Abort()
			return
		}

		token, exists := c.Get("token")
		if !exists {
			utils.Unauthorized(c, "Token not found in context")
			c.Abort()
			return
		}

		node, err := authService.ValidateToken(nodeIDStr, token.(string))
		if err != nil {
			if errors.Is(err, service.ErrNodeTokenRevoked) {
				utils.Unauthorized(c, "node_token_revoked")
				c.Abort()
				return
			}
			utils.Unauthorized(c, "Invalid token for this node")
			c.Abort()
			return
		}

		c.Set("node", node)
		c.Set("node_id", nodeIDStr)
		c.Next()
	}
}
