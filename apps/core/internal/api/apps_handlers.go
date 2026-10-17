package api

import (
	"net/http"
	"orion/core/internal/utils"

	"github.com/gin-gonic/gin"
)

// logsNotImplemented answers the product-logs routes until phase L1 ships their
// implementation. The route, annotations and types are the contract.
func (s *Server) logsNotImplemented(c *gin.Context) {
	utils.ErrorResponse(c, http.StatusNotImplemented, "Product logs are specified but not implemented yet (phase L1)", nil)
}

// listApps List apps.
// @Summary      List apps
// @Description  Product-logging applications and their settings.
// @Tags         apps
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           listApps
// @Success      200  {object}  utils.APIResponse{data=object{apps=[]App}}
// @Failure      401  {object}  utils.APIResponse
// @Failure      501  {object}  utils.APIResponse
// @Router       /v1/apps [get]
func (s *Server) listApps(c *gin.Context) { s.logsNotImplemented(c) }

// createApp Create an app.
// @Summary      Create an app
// @Description  Creates an app. Create an ingest key next to let it send logs.
// @Tags         apps
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           createApp
// @Param        request body AppRequest true "App settings"
// @Success      201  {object}  utils.APIResponse{data=object{app=App}}
// @Failure      400  {object}  utils.APIResponse
// @Failure      401  {object}  utils.APIResponse
// @Failure      501  {object}  utils.APIResponse
// @Router       /v1/apps [post]
func (s *Server) createApp(c *gin.Context) { s.logsNotImplemented(c) }

// getApp Get an app.
// @Summary      Get an app
// @Description
// @Tags         apps
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           getApp
// @Param        id path string true "App ID"
// @Success      200  {object}  utils.APIResponse{data=object{app=App}}
// @Failure      404  {object}  utils.APIResponse
// @Failure      401  {object}  utils.APIResponse
// @Failure      501  {object}  utils.APIResponse
// @Router       /v1/apps/{id} [get]
func (s *Server) getApp(c *gin.Context) { s.logsNotImplemented(c) }

// updateApp Update an app.
// @Summary      Update an app
// @Description  Changes retention, sampling, display profile or redaction. Omitted fields are unchanged.
// @Tags         apps
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           updateApp
// @Param        id path string true "App ID"
// @Param        request body AppRequest true "Fields to change"
// @Success      200  {object}  utils.APIResponse{data=object{app=App}}
// @Failure      400  {object}  utils.APIResponse
// @Failure      404  {object}  utils.APIResponse
// @Failure      401  {object}  utils.APIResponse
// @Failure      501  {object}  utils.APIResponse
// @Router       /v1/apps/{id} [patch]
func (s *Server) updateApp(c *gin.Context) { s.logsNotImplemented(c) }

// deleteApp Delete an app.
// @Summary      Delete an app
// @Description  Deletes the app, revokes its keys and removes its stored logs.
// @Tags         apps
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           deleteApp
// @Param        id path string true "App ID"
// @Success      200  {object}  utils.APIResponse
// @Failure      404  {object}  utils.APIResponse
// @Failure      401  {object}  utils.APIResponse
// @Failure      501  {object}  utils.APIResponse
// @Router       /v1/apps/{id} [delete]
func (s *Server) deleteApp(c *gin.Context) { s.logsNotImplemented(c) }

// listIngestKeys List ingest keys.
// @Summary      List ingest keys
// @Description  Keys never include the secret; only a short prefix is shown.
// @Tags         apps
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           listIngestKeys
// @Param        id path string true "App ID"
// @Success      200  {object}  utils.APIResponse{data=object{keys=[]IngestKey}}
// @Failure      404  {object}  utils.APIResponse
// @Failure      401  {object}  utils.APIResponse
// @Failure      501  {object}  utils.APIResponse
// @Router       /v1/apps/{id}/keys [get]
func (s *Server) listIngestKeys(c *gin.Context) { s.logsNotImplemented(c) }

// createIngestKey Create an ingest key.
// @Summary      Create an ingest key
// @Description  Returns the full key once. Store it; it cannot be shown again. To rotate, create a new key, deploy it, then revoke the old one.
// @Tags         apps
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           createIngestKey
// @Param        id path string true "App ID"
// @Param        request body IngestKeyRequest true "Key name"
// @Success      201  {object}  utils.APIResponse{data=object{key=IngestKeyCreated}}
// @Failure      400  {object}  utils.APIResponse
// @Failure      404  {object}  utils.APIResponse
// @Failure      401  {object}  utils.APIResponse
// @Failure      501  {object}  utils.APIResponse
// @Router       /v1/apps/{id}/keys [post]
func (s *Server) createIngestKey(c *gin.Context) { s.logsNotImplemented(c) }

// revokeIngestKey Revoke an ingest key.
// @Summary      Revoke an ingest key
// @Description  Takes effect immediately.
// @Tags         apps
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           revokeIngestKey
// @Param        id path string true "App ID"
// @Param        key_id path string true "Key ID"
// @Success      200  {object}  utils.APIResponse
// @Failure      404  {object}  utils.APIResponse
// @Failure      401  {object}  utils.APIResponse
// @Failure      501  {object}  utils.APIResponse
// @Router       /v1/apps/{id}/keys/{key_id} [delete]
func (s *Server) revokeIngestKey(c *gin.Context) { s.logsNotImplemented(c) }

// sendAppTestLog Send a test log.
// @Summary      Send a test log
// @Description  Writes one info entry (event orion.test) for the app, to confirm setup end to end.
// @Tags         apps
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           sendAppTestLog
// @Param        id path string true "App ID"
// @Success      202  {object}  utils.APIResponse{data=TestLogResponse}
// @Failure      404  {object}  utils.APIResponse
// @Failure      401  {object}  utils.APIResponse
// @Failure      501  {object}  utils.APIResponse
// @Router       /v1/apps/{id}/test-log [post]
func (s *Server) sendAppTestLog(c *gin.Context) { s.logsNotImplemented(c) }
