package api

import (
	"errors"
	"net/http"
	"net/url"
	"orion/core/internal/service"
	"orion/core/internal/utils"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const agentInstallerURL = "https://github.com/sunday-studio/orion/releases/latest/download/orion-agent-installer.sh"

type EnrollmentRequest struct {
	// Platform is the tile picked in the wizard; it only shapes the install command.
	Platform string `json:"platform,omitempty" enums:"linux,macos,docker"`
}

// EnrollmentResponse describes one "Add server" wizard session.
type EnrollmentResponse struct {
	ID        string    `json:"id"`
	Platform  string    `json:"platform"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
	// Status: waiting (nothing seen) | claimed (token accepted, registering) | registered | reporting (first report received) | expired | cancelled.
	Status        string     `json:"status" enums:"waiting,claimed,registered,reporting,expired,cancelled"`
	ClaimedAt     *time.Time `json:"claimed_at,omitempty"`
	AgentID       string     `json:"agent_id,omitempty"`
	AgentName     string     `json:"agent_name,omitempty"`
	AgentOS       string     `json:"agent_os,omitempty"`
	AgentArch     string     `json:"agent_arch,omitempty"`
	AgentVersion  string     `json:"agent_version,omitempty"`
	FirstReportAt *time.Time `json:"first_report_at,omitempty"`
}

// EnrollmentCreatedResponse is returned once, on creation: it is the only time the token is visible.
type EnrollmentCreatedResponse struct {
	EnrollmentResponse
	Token          string `json:"token" example:"orn_enr_0a1b2c3d4e5f"`
	CoreURL        string `json:"core_url"`
	InstallCommand string `json:"install_command"`
}

func enrollmentResponse(v *service.EnrollmentView) EnrollmentResponse {
	return EnrollmentResponse{ID: v.ID, Platform: v.Platform, ExpiresAt: v.ExpiresAt, CreatedAt: v.CreatedAt, Status: v.Status, ClaimedAt: v.ClaimedAt,
		AgentID: v.AgentID, AgentName: v.AgentName, AgentOS: v.AgentOS, AgentArch: v.AgentArch, AgentVersion: v.AgentVersion, FirstReportAt: v.FirstReportAt}
}

func (s *Server) coreURLFor(c *gin.Context) string {
	if s.cfg.PublicURL != "" {
		return s.cfg.PublicURL
	}
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + c.Request.Host
}

// shellQuote single-quotes a value for a POSIX shell so the pasted install command cannot be
// altered by characters in a Host header or configured URL.
func shellQuote(v string) string { return "'" + strings.ReplaceAll(v, "'", `'\''`) + "'" }

// safeCoreURL accepts only http(s) URLs with a plain host[:port], so the install command is safe to paste.
func safeCoreURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	for _, r := range u.Host {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '.' || r == '-' || r == ':' || r == '[' || r == ']') {
			return false
		}
	}
	return true
}

// createEnrollment starts an Add-server session.
// @Summary      Start an Add server session
// @Description  Creates a one-time registration token that expires in 15 minutes. The token is returned only here. The install command registers the agent with it; poll GET /v1/enrollments/{id} until status is reporting.
// @Tags         enrollments
// @Accept       json
// @Produce      json
// @ID           createEnrollment
// @Param        request  body      EnrollmentRequest  false  "Platform"
// @Success      201      {object}  utils.APIResponse{data=EnrollmentCreatedResponse}
// @Failure      409      {object}  utils.APIResponse
// @Failure      500      {object}  utils.APIResponse
// @Router       /v1/enrollments [post]
func (s *Server) createEnrollment(c *gin.Context) {
	var req EnrollmentRequest
	_ = c.ShouldBindJSON(&req)
	platform := strings.ToLower(strings.TrimSpace(req.Platform))
	switch platform {
	case "linux", "macos", "docker":
	default:
		platform = "linux"
	}
	session, token, err := s.enrollments.Create(platform, time.Now().UTC())
	if err != nil {
		utils.InternalError(c, "Failed to start enrollment", err)
		return
	}
	view, _ := s.enrollments.Get(session.ID, time.Now().UTC())
	coreURL := s.coreURLFor(c)
	if !safeCoreURL(coreURL) {
		_ = s.enrollments.Cancel(session.ID, time.Now().UTC())
		utils.ErrorResponse(c, http.StatusConflict, "Core URL is not usable in an install command; set ORION_PUBLIC_URL", nil)
		return
	}
	command := "curl -fsSL " + agentInstallerURL + " | sudo ORION_ENROLLMENT_TOKEN=" + shellQuote(token) + " bash -s -- --core-url " + shellQuote(coreURL)
	if platform == "macos" {
		command = strings.Replace(command, "sudo ", "", 1)
	}
	utils.SuccessResponse(c, http.StatusCreated, "Enrollment started", EnrollmentCreatedResponse{EnrollmentResponse: enrollmentResponse(view), Token: token, CoreURL: coreURL, InstallCommand: command})
}

// getEnrollment is the wizard's polling endpoint.
// @Summary      Get an Add server session
// @Tags         enrollments
// @Produce      json
// @ID           getEnrollment
// @Param        id   path      string  true  "Enrollment ID"
// @Success      200  {object}  utils.APIResponse{data=EnrollmentResponse}
// @Failure      404  {object}  utils.APIResponse
// @Router       /v1/enrollments/{id} [get]
func (s *Server) getEnrollment(c *gin.Context) {
	view, err := s.enrollments.Get(c.Param("id"), time.Now().UTC())
	if errors.Is(err, service.ErrEnrollmentNotFound) {
		utils.NotFound(c, "Enrollment not found")
		return
	}
	if err != nil {
		utils.InternalError(c, "Failed to load enrollment", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Enrollment retrieved", enrollmentResponse(view))
}

// cancelEnrollment invalidates the token (Cancel / Start over).
// @Summary      Cancel an Add server session
// @Tags         enrollments
// @Produce      json
// @ID           cancelEnrollment
// @Param        id   path      string  true  "Enrollment ID"
// @Success      200  {object}  utils.APIResponse
// @Failure      404  {object}  utils.APIResponse
// @Router       /v1/enrollments/{id} [delete]
func (s *Server) cancelEnrollment(c *gin.Context) {
	err := s.enrollments.Cancel(c.Param("id"), time.Now().UTC())
	if errors.Is(err, service.ErrEnrollmentNotFound) {
		utils.NotFound(c, "Enrollment not found")
		return
	}
	if err != nil {
		utils.InternalError(c, "Failed to cancel enrollment", err)
		return
	}
	utils.SuccessResponse(c, http.StatusOK, "Enrollment cancelled", gin.H{})
}
