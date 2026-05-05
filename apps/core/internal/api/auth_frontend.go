package api

import (
	"crypto/subtle"
	"net/http"
	"time"

	"orion/core/internal/utils"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// loginRequest is the JSON body for POST /v1/auth/login.
type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// loginResponse is returned on success.
type loginResponse struct {
	Token string `json:"token"`
}

// login handles POST /v1/auth/login. When frontend auth is disabled, returns 401.
func (s *Server) login(c *gin.Context) {
	if !s.cfg.FrontendAuthOn {
		utils.Unauthorized(c, "Frontend auth is not configured")
		return
	}

	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.BadRequest(c, "username and password are required")
		return
	}

	// Constant-time comparison to avoid timing attacks
	userOk := subtle.ConstantTimeCompare([]byte(req.Username), []byte(s.cfg.AdminUsername)) == 1
	passOk := subtle.ConstantTimeCompare([]byte(req.Password), []byte(s.cfg.AdminPassword)) == 1
	if !userOk || !passOk {
		utils.Unauthorized(c, "Invalid credentials")
		return
	}

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

	utils.SuccessResponse(c, http.StatusOK, "OK", loginResponse{Token: tokenStr})
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

		t, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
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

		c.Next()
	}
}
