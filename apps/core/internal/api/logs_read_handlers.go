package api

import (
	"orion/core/internal/utils"

	"github.com/gin-gonic/gin"
)

// swag resolves utils.APIResponse through this file's imports.
var _ utils.APIResponse

// searchLogs Search logs.
// @Summary      Search logs
// @Description  Newest-first, cursor-paginated log list. source=app (default) searches product logs; source=agent lists agent service logs (severity, q, from, to and agent_id apply).
// @Tags         logs
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           searchLogs
// @Param        source query string false "app (default) or agent"
// @Param        app_id query string false "Filter by app"
// @Param        agent_id query string false "Filter by agent (agent service logs)"
// @Param        severity query string false "Comma-separated severities, or a minimum with >=warn"
// @Param        event query string false "Exact event name"
// @Param        q query string false "Full-text search over body and attributes"
// @Param        from query string false "Start (RFC 3339, or relative like -1h)"
// @Param        to query string false "End (RFC 3339); default now"
// @Param        trace_id query string false "Exact trace id"
// @Param        request_id query string false "Exact request id"
// @Param        attr query string false "Attribute filter key:value, repeatable (attr=http.status_code:500)"
// @Param        limit query int false "Page size, max 200" default(50)
// @Param        cursor query string false "Cursor from a previous page"
// @Success      200  {object}  utils.APIResponse{data=LogListResponse}
// @Failure      400  {object}  utils.APIResponse
// @Failure      401  {object}  utils.APIResponse
// @Failure      501  {object}  utils.APIResponse
// @Router       /v1/logs [get]
func (s *Server) searchLogs(c *gin.Context) { s.searchLogsImpl(c) }

// getLogHistogram Log volume histogram.
// @Summary      Log volume histogram
// @Description  Counts per time bucket, split by severity, for the same filters as the search.
// @Tags         logs
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           getLogHistogram
// @Param        source query string false "app (default) or agent"
// @Param        app_id query string false "Filter by app"
// @Param        agent_id query string false "Filter by agent (agent service logs)"
// @Param        severity query string false "Comma-separated severities, or a minimum with >=warn"
// @Param        event query string false "Exact event name"
// @Param        q query string false "Full-text search over body and attributes"
// @Param        from query string false "Start (RFC 3339, or relative like -1h)"
// @Param        to query string false "End (RFC 3339); default now"
// @Param        trace_id query string false "Exact trace id"
// @Param        request_id query string false "Exact request id"
// @Param        attr query string false "Attribute filter key:value, repeatable (attr=http.status_code:500)"
// @Param        buckets query int false "Approximate number of buckets" default(60)
// @Success      200  {object}  utils.APIResponse{data=LogHistogramResponse}
// @Failure      400  {object}  utils.APIResponse
// @Failure      401  {object}  utils.APIResponse
// @Failure      501  {object}  utils.APIResponse
// @Router       /v1/logs/histogram [get]
func (s *Server) getLogHistogram(c *gin.Context) { s.getLogHistogramImpl(c) }

// getLogFieldCatalog Log field catalog.
// @Summary      Log field catalog
// @Description  Attribute keys seen for an app with dominant type and approximate cardinality; powers facets and columns.
// @Tags         logs
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           getLogFieldCatalog
// @Param        app_id query string true "App ID"
// @Success      200  {object}  utils.APIResponse{data=LogFieldCatalogResponse}
// @Failure      400  {object}  utils.APIResponse
// @Failure      401  {object}  utils.APIResponse
// @Failure      501  {object}  utils.APIResponse
// @Router       /v1/logs/fields [get]
func (s *Server) getLogFieldCatalog(c *gin.Context) { s.getLogFieldCatalogImpl(c) }

// streamLogs Stream logs live.
// @Summary      Stream logs live
// @Description  Server-Sent Events: each event named log carries one LogEntry as JSON. Accepts the same filters as the search (no from/to/cursor). Sends a comment heartbeat every 15s. Reconnect with Last-Event-ID to resume.
// @Tags         logs
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           streamLogs
// @Param        source query string false "app (default) or agent"
// @Param        app_id query string false "Filter by app"
// @Param        agent_id query string false "Filter by agent (agent service logs)"
// @Param        severity query string false "Comma-separated severities, or a minimum with >=warn"
// @Param        event query string false "Exact event name"
// @Param        q query string false "Full-text search over body and attributes"
// @Param        trace_id query string false "Exact trace id"
// @Param        request_id query string false "Exact request id"
// @Param        attr query string false "Attribute filter key:value, repeatable (attr=http.status_code:500)"
// @Success      200  {string}  string
// @Failure      400  {object}  utils.APIResponse
// @Failure      401  {object}  utils.APIResponse
// @Failure      501  {object}  utils.APIResponse
// @Router       /v1/logs/stream [get]
func (s *Server) streamLogs(c *gin.Context) { s.streamLogsImpl(c) }

// listLogViews List saved log views.
// @Summary      List saved log views
// @Description
// @Tags         logs
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           listLogViews
// @Success      200  {object}  utils.APIResponse{data=object{views=[]LogSavedView}}
// @Failure      401  {object}  utils.APIResponse
// @Failure      501  {object}  utils.APIResponse
// @Router       /v1/logs/views [get]
func (s *Server) listLogViews(c *gin.Context) { s.listLogViewsImpl(c) }

// createLogView Save a log view.
// @Summary      Save a log view
// @Description
// @Tags         logs
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           createLogView
// @Param        request body LogSavedViewRequest true "View"
// @Success      201  {object}  utils.APIResponse{data=object{view=LogSavedView}}
// @Failure      400  {object}  utils.APIResponse
// @Failure      401  {object}  utils.APIResponse
// @Failure      501  {object}  utils.APIResponse
// @Router       /v1/logs/views [post]
func (s *Server) createLogView(c *gin.Context) { s.createLogViewImpl(c) }

// deleteLogView Delete a saved log view.
// @Summary      Delete a saved log view
// @Description
// @Tags         logs
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           deleteLogView
// @Param        id path string true "View ID"
// @Success      200  {object}  utils.APIResponse
// @Failure      404  {object}  utils.APIResponse
// @Failure      401  {object}  utils.APIResponse
// @Failure      501  {object}  utils.APIResponse
// @Router       /v1/logs/views/{id} [delete]
func (s *Server) deleteLogView(c *gin.Context) { s.deleteLogViewImpl(c) }

// getLogEntry Get one log entry.
// @Summary      Get one log entry
// @Description
// @Tags         logs
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           getLogEntry
// @Param        id path string true "Log entry ID"
// @Success      200  {object}  utils.APIResponse{data=object{entry=LogEntry}}
// @Failure      404  {object}  utils.APIResponse
// @Failure      401  {object}  utils.APIResponse
// @Failure      501  {object}  utils.APIResponse
// @Router       /v1/logs/{id} [get]
func (s *Server) getLogEntry(c *gin.Context) { s.getLogEntryImpl(c) }
