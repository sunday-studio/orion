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

func ValidateAgentToken(agentService *service.AgentService, authService *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		agentIDStr := c.Param("agent_id")
		if agentIDStr == "" {
			utils.BadRequest(c, "Agent ID is required")
			c.Abort()
			return
		}

		token, exists := c.Get("token")
		if !exists {
			utils.Unauthorized(c, "Token not found in context")
			c.Abort()
			return
		}

		agent, err := authService.ValidateToken(agentIDStr, token.(string))
		if err != nil {
			if errors.Is(err, service.ErrAgentTokenRevoked) {
				utils.Unauthorized(c, "agent_token_revoked")
				c.Abort()
				return
			}
			utils.Unauthorized(c, "Invalid token for this agent")
			c.Abort()
			return
		}

		c.Set("agent", agent)
		c.Set("agent_id", agentIDStr)
		c.Next()
	}
}
