package api

import (
	"orion/core/internal/utils"

	"github.com/gin-gonic/gin"
)

// swag resolves utils.APIResponse through this file's imports.
var _ utils.APIResponse

func (s *Server) registerSettingsRoutes(frontend *gin.RouterGroup) {
	frontend.GET("/settings/general", s.getGeneralSettings)
	frontend.PUT("/settings/general", s.updateGeneralSettings)
	frontend.GET("/settings/system", s.getSystemSettings)
	frontend.POST("/settings/system/registration-token/rotate", s.rotateRegistrationToken)
	frontend.GET("/settings/api-tokens", s.listAPITokens)
	frontend.POST("/settings/api-tokens", s.createAPIToken)
	frontend.DELETE("/settings/api-tokens/:id", s.revokeAPIToken)
	frontend.GET("/settings/audit", s.listAuditLog)
}

// getGeneralSettings Instance-wide settings.
// @Summary      Get general settings
// @Description  Instance name, Public Core URL (with whether it comes from settings or the environment), time zone and time format.
// @Tags         settings
// @Produce      json
// @Security     BearerAuth
// @ID           getGeneralSettings
// @Success      200  {object}  utils.APIResponse{data=GeneralSettings}
// @Router       /v1/settings/general [get]
func (s *Server) getGeneralSettings(c *gin.Context) { s.getGeneralSettingsImpl(c) }

// updateGeneralSettings Change instance-wide settings.
// @Summary      Update general settings
// @Description  Send only the fields to change. The Public Core URL applies to emails, install commands and status page links at once.
// @Tags         settings
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           updateGeneralSettings
// @Param        request  body      GeneralSettingsRequest  true  "Fields to change"
// @Success      200  {object}  utils.APIResponse{data=GeneralSettings}
// @Failure      400  {object}  utils.APIResponse
// @Router       /v1/settings/general [put]
func (s *Server) updateGeneralSettings(c *gin.Context) { s.updateGeneralSettingsImpl(c) }

// getSystemSettings Console sign-in, registration token, worker and database.
// @Summary      Get system settings
// @Description  Core version, the Console sign-in username and session length, the token agents register with (empty when registration is open), the worker's last heartbeat, and the database size.
// @Tags         settings
// @Produce      json
// @Security     BearerAuth
// @ID           getSystemSettings
// @Success      200  {object}  utils.APIResponse{data=SystemSettings}
// @Router       /v1/settings/system [get]
func (s *Server) getSystemSettings(c *gin.Context) { s.getSystemSettingsImpl(c) }

// rotateRegistrationToken New agent registration token.
// @Summary      Rotate the registration token
// @Description  Replaces the secret new agents register with. Connected servers are not affected. Not available to API tokens.
// @Tags         settings
// @Produce      json
// @Security     BearerAuth
// @ID           rotateRegistrationToken
// @Success      200  {object}  utils.APIResponse{data=RegistrationTokenResponse}
// @Failure      403  {object}  utils.APIResponse
// @Router       /v1/settings/system/registration-token/rotate [post]
func (s *Server) rotateRegistrationToken(c *gin.Context) { s.rotateRegistrationTokenImpl(c) }

// listAPITokens Tokens for scripts and integrations.
// @Summary      List API tokens
// @Description  Name, prefix, scope and last use. Secrets are never returned. Not available to API tokens.
// @Tags         settings
// @Produce      json
// @Security     BearerAuth
// @ID           listAPITokens
// @Success      200  {object}  utils.APIResponse{data=object{tokens=[]APIToken}}
// @Failure      403  {object}  utils.APIResponse
// @Router       /v1/settings/api-tokens [get]
func (s *Server) listAPITokens(c *gin.Context) { s.listAPITokensImpl(c) }

// createAPIToken Create a token.
// @Summary      Create an API token
// @Description  Returns the token once. scope read allows GET requests only; write allows everything except managing tokens and the registration token. Send it as Authorization: Bearer orn_api_...
// @Tags         settings
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           createAPIToken
// @Param        request  body      APITokenRequest  true  "Token"
// @Success      201  {object}  utils.APIResponse{data=object{token=APITokenCreated}}
// @Failure      400  {object}  utils.APIResponse
// @Failure      403  {object}  utils.APIResponse
// @Failure      409  {object}  utils.APIResponse
// @Router       /v1/settings/api-tokens [post]
func (s *Server) createAPIToken(c *gin.Context) { s.createAPITokenImpl(c) }

// revokeAPIToken Revoke a token.
// @Summary      Revoke an API token
// @Tags         settings
// @Produce      json
// @Security     BearerAuth
// @ID           revokeAPIToken
// @Param        id   path      string  true  "Token id"
// @Success      200  {object}  utils.APIResponse
// @Failure      403  {object}  utils.APIResponse
// @Failure      404  {object}  utils.APIResponse
// @Router       /v1/settings/api-tokens/{id} [delete]
func (s *Server) revokeAPIToken(c *gin.Context) { s.revokeAPITokenImpl(c) }

// listAuditLog Who changed what.
// @Summary      List the audit log
// @Description  Admin actions (resolved an incident, paused a monitor, rotated a token, changed settings...), newest first, with the people and tokens that appear for the Actor filter. Page with cursor. Export with POST /v1/logs/export {kind: "audit"}.
// @Tags         settings
// @Produce      json
// @Security     BearerAuth
// @ID           listAuditLog
// @Param        actor query string false "Only this actor (username or API token name)"
// @Param        q query string false "Search actor, action and target"
// @Param        action query string false "Exact action key, for example incident_resolved"
// @Param        object_type query string false "Only this kind of object"
// @Param        from query string false "Start (RFC 3339, or relative like -7d)"
// @Param        to query string false "End"
// @Param        limit query int false "Rows per page" default(50)
// @Param        cursor query string false "Cursor from a previous page"
// @Success      200  {object}  utils.APIResponse{data=AuditListResponse}
// @Failure      400  {object}  utils.APIResponse
// @Router       /v1/settings/audit [get]
func (s *Server) listAuditLog(c *gin.Context) { s.listAuditLogImpl(c) }
