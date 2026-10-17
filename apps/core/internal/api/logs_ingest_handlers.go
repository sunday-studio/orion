package api

import (
	"orion/core/internal/utils"

	"github.com/gin-gonic/gin"
)

// swag resolves utils.APIResponse through this file's imports.
var _ utils.APIResponse

// ingestLogs Ingest logs.
// @Summary      Ingest logs
// @Description  Send one entry, a JSON array of entries, or newline-delimited JSON (Content-Type application/x-ndjson). Authenticate with an app ingest key. Entries are validated individually: invalid ones are rejected without failing the batch. Limits: 64 KiB per entry, 1 MiB per request (5 MiB for NDJSON). Responds 202 once queued; 429 with Retry-After when the app is rate limited or the queue is full.
// @Tags         ingest
// @Accept       json
// @Produce      json
// @Security     IngestKey
// @ID           ingestLogs
// @Param        entries body []LogEntryInput true "One entry object, an array of entries, or NDJSON"
// @Success      202  {object}  IngestResponse
// @Failure      400  {object}  utils.APIResponse
// @Failure      413  {object}  utils.APIResponse
// @Failure      429  {object}  utils.APIResponse
// @Failure      401  {object}  utils.APIResponse
// @Failure      501  {object}  utils.APIResponse
// @Router       /v1/ingest/logs [post]
func (s *Server) ingestLogs(c *gin.Context) { s.logsNotImplemented(c) }

// ingestOTLPLogs Ingest logs over OTLP/HTTP.
// @Summary      Ingest logs over OTLP/HTTP
// @Description  OpenTelemetry Logs endpoint (ExportLogsServiceRequest) as JSON (application/json) or protobuf (application/x-protobuf), so any OpenTelemetry SDK or Collector can send logs with no Orion-specific code. Authenticate with an app ingest key. Responds per the OTLP spec with an ExportLogsServiceResponse (partial_success when some records were rejected).
// @Tags         ingest
// @Accept       json
// @Produce      json
// @Security     IngestKey
// @ID           ingestOTLPLogs
// @Param        request body object true "ExportLogsServiceRequest (OTLP/HTTP JSON; protobuf is also accepted)"
// @Success      200  {object}  object
// @Failure      400  {object}  utils.APIResponse
// @Failure      413  {object}  utils.APIResponse
// @Failure      429  {object}  utils.APIResponse
// @Failure      401  {object}  utils.APIResponse
// @Failure      501  {object}  utils.APIResponse
// @Router       /v1/ingest/otlp/v1/logs [post]
func (s *Server) ingestOTLPLogs(c *gin.Context) { s.logsNotImplemented(c) }
