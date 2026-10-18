package api

import (
	"orion/core/internal/utils"

	"github.com/gin-gonic/gin"
)

// swag resolves utils.APIResponse through this file's imports.
var _ utils.APIResponse

// listStatusLogs List status logs.
// @Summary      List status logs
// @Description  Checks, state changes, server reports, incident steps, alert deliveries and admin actions, newest first, with the total for the filter. Page with cursor (older) or cursor plus direction=newer. The default window is the last 24 hours.
// @Tags         status-logs
// @Produce      json
// @Security     BearerAuth
// @ID           listStatusLogs
// @Param        level query string false "Comma-separated: info, warn, error"
// @Param        type query string false "Comma-separated: monitor, alert, server, incident, system"
// @Param        server_id query string false "Only this server"
// @Param        monitor_id query string false "Only this monitor"
// @Param        incident_id query string false "Only this incident"
// @Param        q query string false "Search servers, monitors, titles and details"
// @Param        from query string false "Start (RFC 3339, or relative like -1h); default 24 hours ago"
// @Param        to query string false "End (RFC 3339); default now"
// @Param        limit query int false "Rows per page, max 200" default(100)
// @Param        cursor query string false "Cursor from a previous page"
// @Param        direction query string false "older (default) or newer" Enums(older, newer)
// @Success      200  {object}  utils.APIResponse{data=StatusLogListResponse}
// @Failure      400  {object}  utils.APIResponse
// @Router       /v1/status-logs [get]
func (s *Server) listStatusLogs(c *gin.Context) { s.listStatusLogsImpl(c) }

// getStatusLogFacets Status log facets.
// @Summary      Status log facets
// @Description  Counts per level and type, and the busiest servers and monitors (top=3 by default, with the distinct totals for "Show N more"), for the same filters as the list.
// @Tags         status-logs
// @Produce      json
// @Security     BearerAuth
// @ID           getStatusLogFacets
// @Param        level query string false "Comma-separated: info, warn, error"
// @Param        type query string false "Comma-separated: monitor, alert, server, incident, system"
// @Param        server_id query string false "Only this server"
// @Param        monitor_id query string false "Only this monitor"
// @Param        q query string false "Search"
// @Param        from query string false "Start; default 24 hours ago"
// @Param        to query string false "End"
// @Param        top query int false "How many servers and monitors to list" default(3)
// @Success      200  {object}  utils.APIResponse{data=StatusLogFacetsResponse}
// @Failure      400  {object}  utils.APIResponse
// @Router       /v1/status-logs/facets [get]
func (s *Server) getStatusLogFacets(c *gin.Context) { s.getStatusLogFacetsImpl(c) }

// getStatusLogHistogram Status log histogram.
// @Summary      Status log histogram
// @Description  Events per bucket split by level (info, warn, error), for "Events per 15 minutes". Empty buckets are included.
// @Tags         status-logs
// @Produce      json
// @Security     BearerAuth
// @ID           getStatusLogHistogram
// @Param        level query string false "Comma-separated: info, warn, error"
// @Param        type query string false "Comma-separated: monitor, alert, server, incident, system"
// @Param        server_id query string false "Only this server"
// @Param        monitor_id query string false "Only this monitor"
// @Param        q query string false "Search"
// @Param        from query string false "Start; default 24 hours ago"
// @Param        to query string false "End"
// @Param        buckets query int false "Target number of buckets" default(60)
// @Success      200  {object}  utils.APIResponse{data=LogHistogramResponse}
// @Failure      400  {object}  utils.APIResponse
// @Router       /v1/status-logs/histogram [get]
func (s *Server) getStatusLogHistogram(c *gin.Context) { s.getStatusLogHistogramImpl(c) }

// getStatusLogPatterns Status log patterns.
// @Summary      Status log patterns
// @Description  Groups similar events (numbers and ids become placeholders) so a flood of one failure is one row; is_new marks patterns absent from the previous window.
// @Tags         status-logs
// @Produce      json
// @Security     BearerAuth
// @ID           getStatusLogPatterns
// @Param        level query string false "Comma-separated: info, warn, error"
// @Param        type query string false "Comma-separated: monitor, alert, server, incident, system"
// @Param        server_id query string false "Only this server"
// @Param        monitor_id query string false "Only this monitor"
// @Param        from query string false "Start; default 24 hours ago"
// @Param        to query string false "End"
// @Param        limit query int false "Maximum patterns" default(50)
// @Success      200  {object}  utils.APIResponse{data=LogPatternsResponse}
// @Failure      400  {object}  utils.APIResponse
// @Router       /v1/status-logs/patterns [get]
func (s *Server) getStatusLogPatterns(c *gin.Context) { s.getStatusLogPatternsImpl(c) }

// streamStatusLogs Live tail of what Orion sees.
// @Summary      Stream status logs live
// @Description  Server-sent events, one StatusLogEntry per event; new events appear within about two seconds. Resume with Last-Event-ID.
// @Tags         status-logs
// @Produce      text/event-stream
// @Security     BearerAuth
// @ID           streamStatusLogs
// @Param        level query string false "Comma-separated: info, warn, error"
// @Param        type query string false "Comma-separated: monitor, alert, server, incident, system"
// @Param        server_id query string false "Only this server"
// @Param        monitor_id query string false "Only this monitor"
// @Param        q query string false "Search"
// @Success      200  {string}  string  "event stream"
// @Router       /v1/status-logs/stream [get]
func (s *Server) streamStatusLogs(c *gin.Context) { s.streamStatusLogsImpl(c) }
