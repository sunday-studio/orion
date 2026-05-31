package api

import (
	"crypto/subtle"
	"net/http"
	"time"

	"orion/core/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// LoginRequest is the JSON body for POST /v1/auth/login.
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse is returned on successful Console login.
type LoginResponse struct {
	Token string `json:"token"`
}

// login handles POST /v1/auth/login. When frontend auth is disabled, returns 401.
// @Summary      Log in to Console
// @Description  Returns a JWT for frontend API requests when frontend auth is configured.
// @Tags         auth
// @Accept       json
// @Produce      json
// @ID           login
// @Param        request  body      LoginRequest  true  "Login request"
// @Success      200      {object}  utils.APIResponse{data=LoginResponse}
// @Failure      400      {object}  utils.APIResponse
// @Failure      401      {object}  utils.APIResponse
// @Failure      500      {object}  utils.APIResponse
// @Router       /v1/auth/login [post]
func (s *Server) login(c *gin.Context) {
	if !s.cfg.FrontendAuthOn {
		utils.Unauthorized(c, "Frontend auth is not configured")
		return
	}

	clientIP := c.ClientIP()
	if s.loginLimiter.TooManyFailures(clientIP) {
		utils.ErrorResponse(c, http.StatusTooManyRequests, "Too many login attempts", nil)
		return
	}

	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "username and password are required")
		return
	}

	// Constant-time comparison to avoid timing attacks
	userOk := subtle.ConstantTimeCompare([]byte(req.Username), []byte(s.cfg.AdminUsername)) == 1
	passOk := subtle.ConstantTimeCompare([]byte(req.Password), []byte(s.cfg.AdminPassword)) == 1
	if !userOk || !passOk {
		s.loginLimiter.RecordFailure(clientIP)
		utils.Unauthorized(c, "Invalid credentials")
		return
	}
	s.loginLimiter.Reset(clientIP)

	claims := jwt.MapClaims{
		"sub": req.Username,
		"exp": time.Now().Add(24 * time.Hour).Unix(),
		"iat": time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := token.SignedString([]byte(s.cfg.JWTSecret))
	if err != nil {
		s.logger.Error("Failed to sign JWT", "error", err)
		utils.ErrorResponse(c, http.StatusInternalServerError, "Login failed", err)
		return
	}

	utils.SuccessResponse(c, http.StatusOK, "OK", LoginResponse{Token: tokenStr})
}

// frontendAuthMiddleware returns a handler that enforces JWT when frontend auth is on.
// When off, it does nothing. Do not use on /v1/register, /v1/auth/login, or agent routes.
func (s *Server) frontendAuthMiddleware() gin.HandlerFunc {
	secret := []byte(s.cfg.JWTSecret)
	enabled := s.cfg.FrontendAuthOn

	return func(c *gin.Context) {
		if !enabled {
			c.Next()
			return
		}

		auth := c.GetHeader("Authorization")
		if auth == "" || len(auth) < 8 || auth[:7] != "Bearer " {
			utils.Unauthorized(c, "Authorization header required")
			c.Abort()
			return
		}
		tokenStr := auth[7:]

		t, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return secret, nil
		})
		if err != nil || !t.Valid {
			utils.Unauthorized(c, "Invalid or expired token")
			c.Abort()
			return
		}
		if claims, ok := t.Claims.(jwt.MapClaims); ok {
			if subject, ok := claims["sub"].(string); ok && subject != "" {
				c.Set("frontend_actor_id", subject)
			}
		}

		c.Next()
	}
}
