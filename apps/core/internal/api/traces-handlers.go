package api

import (
	"orion/core/internal/utils"

	"github.com/gin-gonic/gin"
)

// swag resolves utils.APIResponse through this file's imports.
var _ utils.APIResponse

// ingestOTLPTraces Ingest traces over OTLP/HTTP.
// @Summary      Ingest traces over OTLP/HTTP
// @Description  OpenTelemetry Traces endpoint (ExportTraceServiceRequest) as JSON (application/json) or protobuf (application/x-protobuf), optionally gzip-compressed, so any OpenTelemetry SDK or Collector can send spans with no Orion-specific code (OTEL_EXPORTER_OTLP_TRACES_ENDPOINT). Authenticate with an app ingest key; the same body limits and rate limit as logs apply (each span counts as one entry). Limits: 10,000 spans per request, 256 KiB per span, 128 attributes, events and links per span, text values cut at 4,096 bytes. A span with an invalid trace or span id, a missing or reversed time, a start more than an hour ahead, or attributes nested deeper than 8 levels is refused and counted in partial_success; the rest of the request is stored. Spans of a trace may arrive in any order and in separate requests.
// @Tags         ingest
// @Accept       json
// @Produce      json
// @Security     IngestKey
// @ID           ingestOTLPTraces
// @Param        request body object true "ExportTraceServiceRequest (OTLP/HTTP JSON; protobuf is also accepted)"
// @Success      200  {object}  object
// @Failure      400  {object}  utils.APIResponse
// @Failure      401  {object}  utils.APIResponse
// @Failure      413  {object}  utils.APIResponse
// @Failure      415  {object}  utils.APIResponse
// @Failure      429  {object}  utils.APIResponse
// @Failure      503  {object}  utils.APIResponse
// @Router       /v1/ingest/otlp/v1/traces [post]
func (s *Server) ingestOTLPTraces(c *gin.Context) { s.ingestOTLPTracesImpl(c) }

// listTraces Search traces.
// @Summary      Search traces
// @Description  Newest-first, cursor-paginated list with one row per trace and app: root span name, service, duration, span and error counts. A trace whose root span has not arrived is listed from its earliest span (has_root false). app_id and attr take at most 50 values each (more is a 400). Without to the search runs up to five minutes past now, so spans stamped by a clock a little ahead still show. service matches the service.name of the root span; name matches a substring of the name the list shows (the root span name, or the earliest span name while the root has not arrived); attr matches a trace when any of its spans or its resource carries the value.
// @Tags         traces
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           listTraces
// @Param        app_id query string false "Filter by app (comma-separated ids, at most 50)"
// @Param        service_id query string false "Filter by the apps linked to this service"
// @Param        service query string false "Filter by resource service.name of the root span"
// @Param        status query string false "ok (no span failed) or error (at least one span failed)" Enums(ok, error)
// @Param        min_duration query string false "Minimum duration: 250ms, 2s, or a number of milliseconds"
// @Param        name query string false "Substring of the trace name the list shows: the root span name, or the earliest span name while the root has not arrived"
// @Param        from query string false "Start (RFC 3339, or relative like -1h)"
// @Param        to query string false "End (RFC 3339, or relative like -1h); default now plus 5 minutes, which allows for clocks a little ahead"
// @Param        attr query []string false "Attribute filter key:value, repeat the parameter for several (all must match), at most 50 (attr=http.status_code:500&attr=db.system:postgres)" collectionFormat(multi)
// @Param        limit query int false "Page size, max 200" default(50)
// @Param        cursor query string false "Cursor from a previous page"
// @Success      200  {object}  utils.APIResponse{data=TraceListResponse}
// @Failure      400  {object}  utils.APIResponse
// @Failure      401  {object}  utils.APIResponse
// @Failure      404  {object}  utils.APIResponse
// @Failure      503  {object}  utils.APIResponse
// @Failure      504  {object}  utils.APIResponse
// @Router       /v1/traces [get]
func (s *Server) listTraces(c *gin.Context) { s.listTracesImpl(c) }

// getTraceFacets Trace filter facets.
// @Summary      Trace facets
// @Description  The choices for a trace filter bar over the traces matching the same filters as the search: total, the number of traces matching every filter (status included, so it equals what the search returns), the ok and error counts of the traces matching all filters except status (so both choices stay visible), and at most 50 services and 50 root span names of the traces matching every filter, most traces first. Without from the last 24 hours are counted; without to the window ends five minutes past now. app_id and attr take at most 50 values each.
// @Tags         traces
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           getTraceFacets
// @Param        app_id query string false "Filter by app (comma-separated ids, at most 50)"
// @Param        service_id query string false "Filter by the apps linked to this service"
// @Param        service query string false "Filter by resource service.name of the root span"
// @Param        status query string false "ok or error" Enums(ok, error)
// @Param        min_duration query string false "Minimum duration: 250ms, 2s, or a number of milliseconds"
// @Param        name query string false "Substring of the trace name the list shows: the root span name, or the earliest span name while the root has not arrived"
// @Param        from query string false "Start (RFC 3339, or relative like -1h); default -24h"
// @Param        to query string false "End (RFC 3339, or relative like -1h); default now plus 5 minutes, which allows for clocks a little ahead"
// @Param        attr query []string false "Attribute filter key:value, repeat the parameter for several (all must match), at most 50" collectionFormat(multi)
// @Success      200  {object}  utils.APIResponse{data=TraceFacetsResponse}
// @Failure      400  {object}  utils.APIResponse
// @Failure      401  {object}  utils.APIResponse
// @Failure      404  {object}  utils.APIResponse
// @Failure      503  {object}  utils.APIResponse
// @Failure      504  {object}  utils.APIResponse
// @Router       /v1/traces/facets [get]
func (s *Server) getTraceFacets(c *gin.Context) { s.getTraceFacetsImpl(c) }

// getTrace Get one trace.
// @Summary      Get a trace
// @Description  Every stored span of a trace id ordered by start time (at most 5,000; truncated says when more exist), with each span's offset from the trace start for drawing a waterfall. Apps that instrument both sides of a call share the trace id, so spans of all apps are returned. log_count is how many product log entries carry the trace id.
// @Tags         traces
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           getTrace
// @Param        trace_id path string true "Trace id (32 hexadecimal characters)"
// @Success      200  {object}  utils.APIResponse{data=TraceDetailResponse}
// @Failure      400  {object}  utils.APIResponse
// @Failure      401  {object}  utils.APIResponse
// @Failure      404  {object}  utils.APIResponse
// @Failure      503  {object}  utils.APIResponse
// @Failure      504  {object}  utils.APIResponse
// @Router       /v1/traces/{trace_id} [get]
func (s *Server) getTrace(c *gin.Context) { s.getTraceImpl(c) }

// getServiceTraceSummary Trace summary of a service.
// @Summary      Service trace summary
// @Description  Trace count, error count and rate, and root-span latency percentiles (p50, p95, p99; nearest rank) of the traces the apps linked to a service received between from and to (default the last hour, at most 7 days). Counts are exact; percentiles use the newest 50,000 traces with a root span (sampled says when that cut applies).
// @Tags         services
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @ID           getServiceTraceSummary
// @Param        id path string true "Service ID"
// @Param        from query string false "Start (RFC 3339, or relative like -1h); default -1h"
// @Param        to query string false "End (RFC 3339); default now"
// @Success      200  {object}  utils.APIResponse{data=ServiceTraceSummaryResponse}
// @Failure      400  {object}  utils.APIResponse
// @Failure      401  {object}  utils.APIResponse
// @Failure      404  {object}  utils.APIResponse
// @Failure      503  {object}  utils.APIResponse
// @Failure      504  {object}  utils.APIResponse
// @Router       /v1/services/{id}/traces/summary [get]
func (s *Server) getServiceTraceSummary(c *gin.Context) { s.getServiceTraceSummaryImpl(c) }
