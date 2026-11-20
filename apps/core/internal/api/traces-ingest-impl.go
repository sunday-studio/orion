package api

import (
	"errors"
	"net/http"
	"orion/core/internal/tracestore"
	"orion/core/internal/utils"
	"strconv"

	"github.com/gin-gonic/gin"
)

// EnableTraces turns on product traces using an opened store. Core's API binary calls it at startup;
// until then the trace routes answer 503.
func (s *Server) EnableTraces(store *tracestore.Store) {
	s.traces = store
}

// requireTraces answers 503 when traces are not enabled and reports whether to continue.
func (s *Server) requireTraces(c *gin.Context) bool {
	if s.traces != nil {
		return true
	}
	utils.ErrorResponse(c, http.StatusServiceUnavailable, "Traces are not enabled on this Core instance", nil)
	return false
}

// ingestOTLPTracesImpl takes OTLP/HTTP spans with the same ingest key, body limits and rate limiter as
// logs (each span counts as one entry). Spans that break a rule are refused one by one and reported in
// partial_success; the rest of the request is stored.
func (s *Server) ingestOTLPTracesImpl(c *gin.Context) {
	if !s.requireLogs(c) {
		return
	}
	app, err := s.logs.ingester.Authenticate(c.GetHeader("Authorization"))
	if err != nil {
		c.Header("WWW-Authenticate", `Bearer realm="orion-ingest"`)
		utils.Unauthorized(c, "Valid ingest key required")
		return
	}
	if !s.requireTraces(c) {
		return
	}
	body, ok := readIngestBody(c)
	if !ok {
		return
	}
	request, format, err := tracestore.DecodeOTLP(c.GetHeader("Content-Type"), body)
	switch {
	case errors.Is(err, tracestore.ErrUnsupportedMedia):
		utils.ErrorResponse(c, http.StatusUnsupportedMediaType, "OTLP/HTTP needs application/x-protobuf or application/json", nil)
		return
	case errors.Is(err, tracestore.ErrInvalidBody):
		utils.BadRequest(c, "Request body is not a valid ExportTraceServiceRequest")
		return
	case err != nil:
		utils.InternalError(c, "Failed to read the trace request", err)
		return
	}
	result, failure := s.exportTraces(app, request)
	switch {
	case failure == nil:
	case failure.kind == failTooMany:
		utils.ErrorResponse(c, http.StatusRequestEntityTooLarge, "Too many spans in one request (limit 10000)", nil)
		return
	case failure.kind == failInvalid:
		utils.BadRequest(c, "Request body is not a valid ExportTraceServiceRequest")
		return
	case failure.kind == failRateLimited:
		c.Header("Retry-After", strconv.Itoa(retryAfterSeconds(failure.retryAfter)))
		utils.ErrorResponse(c, http.StatusTooManyRequests, "Ingest rate limit exceeded; retry after the indicated seconds", nil)
		return
	case failure.kind == failStorageFull:
		c.Header("Retry-After", strconv.Itoa(retryAfterSeconds(failure.retryAfter)))
		utils.ErrorResponse(c, http.StatusServiceUnavailable, "Trace storage is full; spans were not accepted. Free disk space or lower retention.", nil)
		return
	case failure.kind == failQueueFull:
		c.Header("Retry-After", strconv.Itoa(retryAfterSeconds(failure.retryAfter)))
		utils.ErrorResponse(c, http.StatusTooManyRequests, "Trace write queue is full; retry shortly", nil)
		return
	default:
		utils.InternalError(c, failure.message, failure.err)
		return
	}
	payload, contentType, err := tracestore.OTLPResponse(format, result.Rejected, result.Errors)
	if err != nil {
		utils.InternalError(c, "Failed to encode the response", err)
		return
	}
	c.Data(http.StatusOK, contentType, payload)
}
