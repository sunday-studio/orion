package api

import (
	"orion/core/internal/utils"

	"github.com/gin-gonic/gin"
)

// swag resolves utils.APIResponse through this file's imports.
var _ utils.APIResponse

// searchInternalLogs Core's own log lines.
// @Summary      Search internal logs
// @Description  Core's own log lines (API, worker, scheduler, node gateway), newest first with a cursor. Same filters and shape as GET /v1/logs; the component of each line is attributes.component. Routine successful HTTP requests are not recorded; failed and slow ones are.
// @Tags         internal-logs
// @Produce      json
// @Security     BearerAuth
// @ID           searchInternalLogs
// @Param        severity query string false "Comma-separated severities, or a minimum with >=warn"
// @Param        q query string false "Full-text search"
// @Param        from query string false "Start (RFC 3339, or relative like -1h)"
// @Param        to query string false "End (RFC 3339); default now"
// @Param        attr query string false "Attribute filter key:value, repeatable (attr=component:worker)"
// @Param        limit query int false "Page size, max 200" default(50)
// @Param        cursor query string false "Cursor from a previous page"
// @Success      200  {object}  utils.APIResponse{data=LogListResponse}
// @Failure      400  {object}  utils.APIResponse
// @Failure      504  {object}  utils.APIResponse{data=SearchTimeoutBody}
// @Router       /v1/internal-logs [get]
func (s *Server) searchInternalLogs(c *gin.Context) { internalScope(s.searchLogsImpl)(c) }

// getInternalLogHistogram Internal log volume.
// @Summary      Internal log histogram
// @Tags         internal-logs
// @Produce      json
// @Security     BearerAuth
// @ID           getInternalLogHistogram
// @Param        severity query string false "Comma-separated severities, or a minimum with >=warn"
// @Param        q query string false "Full-text search"
// @Param        from query string false "Start"
// @Param        to query string false "End"
// @Param        attr query string false "Attribute filter key:value, repeatable"
// @Param        buckets query int false "Number of buckets" default(60)
// @Success      200  {object}  utils.APIResponse{data=LogHistogramResponse}
// @Router       /v1/internal-logs/histogram [get]
func (s *Server) getInternalLogHistogram(c *gin.Context) { internalScope(s.getLogHistogramImpl)(c) }

// getInternalLogFacets Internal log filter rail.
// @Summary      Internal log facets
// @Description  Severity counts and, in values, the components (core api, worker, scheduler, node gateway) with counts; pass field=<key> to list another attribute.
// @Tags         internal-logs
// @Produce      json
// @Security     BearerAuth
// @ID           getInternalLogFacets
// @Param        severity query string false "Comma-separated severities, or a minimum with >=warn"
// @Param        q query string false "Full-text search"
// @Param        from query string false "Start"
// @Param        to query string false "End"
// @Param        attr query string false "Attribute filter key:value, repeatable"
// @Param        field query string false "Attribute to list values for" default(component)
// @Success      200  {object}  utils.APIResponse{data=LogFacetsResponse}
// @Router       /v1/internal-logs/facets [get]
func (s *Server) getInternalLogFacets(c *gin.Context) { internalScope(s.getLogFacetsImpl)(c) }

// getInternalLogPatterns Internal log patterns.
// @Summary      Internal log patterns
// @Tags         internal-logs
// @Produce      json
// @Security     BearerAuth
// @ID           getInternalLogPatterns
// @Param        severity query string false "Comma-separated severities, or a minimum with >=warn"
// @Param        attr query string false "Attribute filter key:value, repeatable"
// @Param        from query string false "Start; default 24 hours ago"
// @Param        to query string false "End; default now"
// @Success      200  {object}  utils.APIResponse{data=LogPatternsResponse}
// @Router       /v1/internal-logs/patterns [get]
func (s *Server) getInternalLogPatterns(c *gin.Context) { internalScope(s.getLogPatternsImpl)(c) }

// streamInternalLogs Live tail of Core's own log lines.
// @Summary      Stream internal logs live
// @Description  Server-sent events; each event is one log entry. Resume with Last-Event-ID.
// @Tags         internal-logs
// @Produce      text/event-stream
// @Security     BearerAuth
// @ID           streamInternalLogs
// @Param        severity query string false "Comma-separated severities, or a minimum with >=warn"
// @Param        q query string false "Full-text search"
// @Param        attr query string false "Attribute filter key:value, repeatable"
// @Success      200  {string}  string  "event stream"
// @Router       /v1/internal-logs/stream [get]
func (s *Server) streamInternalLogs(c *gin.Context) { internalScope(s.streamLogsImpl)(c) }
