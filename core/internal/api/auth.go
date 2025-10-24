package api

import (
	"fmt"
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

// ValidateAgentToken validates that the token belongs to the agent specified in the URL
func ValidateAgentToken(agentService *service.AgentService, authService *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get agent ID from URL parameter
		agentIDStr := c.Param("agent_id")
		if agentIDStr == "" {
			utils.BadRequest(c, "Agent ID is required")
			c.Abort()
			return
		}

		// Convert agent ID to uint
		var agentID uint
		if _, err := fmt.Sscanf(agentIDStr, "%d", &agentID); err != nil {
			utils.BadRequest(c, "Invalid agent ID format")
			c.Abort()
			return
		}

		// Get token from context
		token, exists := c.Get("token")
		if !exists {
			utils.Unauthorized(c, "Token not found in context")
			c.Abort()
			return
		}

		// Validate token
		agent, err := authService.ValidateToken(agentID, token.(string))
		if err != nil {
			utils.Unauthorized(c, "Invalid token for this agent")
			c.Abort()
			return
		}

		// Store agent info in context
		c.Set("agent", agent)
		c.Set("agent_id", agentID)
		c.Next()
	}
}
