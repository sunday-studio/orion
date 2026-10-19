package api

import (
	"net/http"
	"orion/core/internal/service"
	"orion/core/internal/utils"
	"time"

	"github.com/gin-gonic/gin"
)

type RegistrationPreflightResponse struct {
	Valid bool `json:"valid"`
}

// preflightAgentRegistration validates a first-registration credential without
// consuming a one-time Add server session.
// @Summary      Validate an Agent enrollment token
// @Description  Checks that the presented enrollment credential can register an Agent. This does not claim or consume a one-time Add server token.
// @Tags         agents
// @Produce      json
// @Param        X-Orion-Enrollment-Token header string false "Enrollment credential"
// @Success      200 {object} utils.APIResponse{data=RegistrationPreflightResponse}
// @Failure      401 {object} utils.APIResponse
// @Failure      429 {object} utils.APIResponse
// @Router       /v1/register/preflight [post]
func (s *Server) preflightAgentRegistration(c *gin.Context) {
	if !s.registerLimiter.Allow(c.ClientIP()) {
		utils.ErrorResponse(c, http.StatusTooManyRequests, "Too many registration attempts", nil)
		return
	}
	presented := c.GetHeader(enrollmentTokenHeader)
	valid := false
	if service.IsEnrollmentToken(presented) {
		valid = s.enrollments.Validate(presented, time.Now().UTC())
	} else {
		valid = s.enrollmentTokenValid(presented)
	}
	if !valid {
		utils.Unauthorized(c, "Valid enrollment token required")
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Enrollment token is valid", RegistrationPreflightResponse{Valid: true})
}
