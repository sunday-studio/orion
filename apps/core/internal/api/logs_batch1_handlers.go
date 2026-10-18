package api

import "github.com/gin-gonic/gin"

// getLogFacets Facets for the filter rail.
// @Summary      Log facets
// @Description  Counts per severity, source, event and environment for the same filters as the search, plus the field catalog for a single app. Pass field=<key> to also get that attribute's most common values.
// @Tags         logs
// @Produce      json
// @Security     BearerAuth
// @ID           getLogFacets
// @Param        app_id query string false "Filter by app"
// @Param        severity query string false "Comma-separated severities, or a minimum with >=warn"
// @Param        event query string false "Exact event name"
// @Param        q query string false "Full-text search"
// @Param        from query string false "Start (RFC 3339, or relative like -1h)"
// @Param        to query string false "End (RFC 3339); default now"
// @Param        trace_id query string false "Exact trace id"
// @Param        request_id query string false "Exact request id"
// @Param        attr query string false "Attribute filter key:value, repeatable"
// @Param        events query int false "How many events to list" default(5)
// @Param        field query string false "Attribute key to list top values for"
// @Param        values query int false "How many values" default(10)
// @Success      200  {object}  utils.APIResponse{data=LogFacetsResponse}
// @Failure      400  {object}  utils.APIResponse
// @Failure      504  {object}  utils.APIResponse{data=SearchTimeoutBody}
// @Router       /v1/logs/facets [get]
func (s *Server) getLogFacets(c *gin.Context) { s.getLogFacetsImpl(c) }

// getLogPatterns Clustered messages.
// @Summary      Log patterns
// @Description  Groups similar messages (numbers, ids and addresses become placeholders) so a flood of one error is one row. Reads up to 20,000 newest matching lines of the window; is_new marks patterns absent from the previous window of the same length.
// @Tags         logs
// @Produce      json
// @Security     BearerAuth
// @ID           getLogPatterns
// @Param        app_id query string false "Filter by app"
// @Param        severity query string false "Comma-separated severities, or a minimum with >=warn"
// @Param        event query string false "Exact event name"
// @Param        q query string false "Full-text search"
// @Param        from query string false "Start; default 24 hours ago"
// @Param        to query string false "End; default now"
// @Param        limit query int false "Maximum patterns" default(50)
// @Success      200  {object}  utils.APIResponse{data=LogPatternsResponse}
// @Failure      400  {object}  utils.APIResponse
// @Failure      504  {object}  utils.APIResponse{data=SearchTimeoutBody}
// @Router       /v1/logs/patterns [get]
func (s *Server) getLogPatterns(c *gin.Context) { s.getLogPatternsImpl(c) }

// getLogRelated Entries that belong with one entry.
// @Summary      Related logs
// @Description  The entries sharing the request id, else the trace id; for plain lines with neither, the lines just before and after in the same app (by=nearby).
// @Tags         logs
// @Produce      json
// @Security     BearerAuth
// @ID           getLogRelated
// @Param        id path string true "Log entry id"
// @Param        limit query int false "Maximum entries" default(10)
// @Success      200  {object}  utils.APIResponse{data=LogRelatedResponse}
// @Failure      404  {object}  utils.APIResponse
// @Router       /v1/logs/{id}/related [get]
func (s *Server) getLogRelated(c *gin.Context) { s.getLogRelatedImpl(c) }

// getLogIngestStatus Ingest health and storage.
// @Summary      Log ingest status
// @Description  Whether ingest is behind (queue depth, lag estimate, rate limiting, storage full) plus today's volume and storage use, for the "Ingest is behind" banner and the Apps header.
// @Tags         logs
// @Produce      json
// @Security     BearerAuth
// @ID           getLogIngestStatus
// @Success      200  {object}  utils.APIResponse{data=LogIngestStatus}
// @Router       /v1/logs/ingest-status [get]
func (s *Server) getLogIngestStatus(c *gin.Context) { s.getLogIngestStatusImpl(c) }

// rotateIngestKey Replace an ingest key.
// @Summary      Rotate an ingest key
// @Description  Creates a new key with the same name and returns it once. The old key keeps working for 10 minutes (its revoked_at is set to that time) so a deploy can switch without dropping logs.
// @Tags         apps
// @Produce      json
// @Security     BearerAuth
// @ID           rotateIngestKey
// @Param        id path string true "App id"
// @Param        key_id path string true "Key id"
// @Success      201  {object}  utils.APIResponse{data=object{key=IngestKeyCreated,previous_key_valid_until=string}}
// @Failure      404  {object}  utils.APIResponse
// @Router       /v1/apps/{id}/keys/{key_id}/rotate [post]
func (s *Server) rotateIngestKey(c *gin.Context) { s.rotateIngestKeyImpl(c) }

// purgeAppLogs Delete all logs of an app.
// @Summary      Delete all logs for an app
// @Description  Removes every stored log line of the app (the app, keys and rules stay).
// @Tags         apps
// @Produce      json
// @Security     BearerAuth
// @ID           purgeAppLogs
// @Param        id path string true "App id"
// @Success      200  {object}  utils.APIResponse{data=object{deleted=int}}
// @Failure      404  {object}  utils.APIResponse
// @Router       /v1/apps/{id}/purge [post]
func (s *Server) purgeAppLogs(c *gin.Context) { s.purgeAppLogsImpl(c) }

// testAppRedaction Try redaction rules on a sample.
// @Summary      Test redaction rules
// @Description  Applies the app's saved rules, or the rules in the request, to a sample and returns the result. Nothing is saved or stored.
// @Tags         apps
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           testAppRedaction
// @Param        id path string true "App id"
// @Param        request body RedactionTestRequest true "Sample and optional unsaved rules"
// @Success      200  {object}  utils.APIResponse{data=RedactionTestResponse}
// @Failure      400  {object}  utils.APIResponse
// @Failure      404  {object}  utils.APIResponse
// @Router       /v1/apps/{id}/redaction/test [post]
func (s *Server) testAppRedaction(c *gin.Context) { s.testAppRedactionImpl(c) }

// getRedactionDefaults Built-in redacted keys.
// @Summary      Default redaction keys
// @Tags         logs
// @Produce      json
// @Security     BearerAuth
// @ID           getRedactionDefaults
// @Success      200  {object}  utils.APIResponse{data=RedactionDefaultsResponse}
// @Router       /v1/logs/redaction-defaults [get]
func (s *Server) getRedactionDefaults(c *gin.Context) { s.getRedactionDefaultsImpl(c) }

// createExport Start a background export.
// @Summary      Export logs
// @Description  Starts a CSV or JSON lines export of the rows matching a filter (capped at 100,000 rows). Poll GET /v1/exports/{id}; the download link works for 24 hours.
// @Tags         exports
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           createExport
// @Param        request body ExportRequest true "What to export"
// @Success      202  {object}  utils.APIResponse{data=object{export=ExportResponse}}
// @Failure      400  {object}  utils.APIResponse
// @Router       /v1/logs/export [post]
func (s *Server) createExport(c *gin.Context) { s.createExportImpl(c) }

// getExport Export status.
// @Summary      Get an export
// @Tags         exports
// @Produce      json
// @Security     BearerAuth
// @ID           getExport
// @Param        id path string true "Export id"
// @Success      200  {object}  utils.APIResponse{data=object{export=ExportResponse}}
// @Failure      404  {object}  utils.APIResponse
// @Router       /v1/exports/{id} [get]
func (s *Server) getExport(c *gin.Context) { s.getExportImpl(c) }

// downloadExport Download an export file.
// @Summary      Download an export
// @Tags         exports
// @Produce      text/csv
// @Security     BearerAuth
// @ID           downloadExport
// @Param        id path string true "Export id"
// @Success      200  {string}  string  "File"
// @Failure      404  {object}  utils.APIResponse
// @Router       /v1/exports/{id}/download [get]
func (s *Server) downloadExport(c *gin.Context) { s.downloadExportImpl(c) }
