package api

import (
	"compress/gzip"
	"errors"
	"io"
	"net/http"
	"orion/core/internal/logstore"
	"orion/core/internal/utils"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

func (s *Server) ingestLogsImpl(c *gin.Context) {
	if !s.requireLogs(c) {
		return
	}
	app, err := s.logs.ingester.Authenticate(c.GetHeader("Authorization"))
	if err != nil {
		c.Header("WWW-Authenticate", `Bearer realm="orion-ingest"`)
		utils.Unauthorized(c, "Valid ingest key required")
		return
	}

	body, ok := readIngestBody(c)
	if !ok {
		return
	}

	result, err := s.logs.ingester.Ingest(app, c.GetHeader("Content-Type"), body)
	var limited *logstore.RateLimitedError
	switch {
	case errors.Is(err, logstore.ErrUnsupportedMedia):
		utils.ErrorResponse(c, http.StatusUnsupportedMediaType, "Use application/json, application/x-ndjson or OTLP", nil)
	case errors.Is(err, logstore.ErrTooManyEntries):
		utils.ErrorResponse(c, http.StatusRequestEntityTooLarge, "Too many entries in one request (limit 10000); send smaller batches", nil)
	case errors.Is(err, logstore.ErrInvalidBody):
		utils.BadRequest(c, "Request body is not valid JSON")
	case errors.As(err, &limited):
		seconds := int((limited.RetryAfter + 999_999_999) / 1_000_000_000)
		c.Header("Retry-After", strconv.Itoa(max(seconds, 1)))
		utils.ErrorResponse(c, http.StatusTooManyRequests, "Ingest rate limit exceeded; retry after the indicated seconds", nil)
	case errors.Is(err, logstore.ErrStorageFull):
		c.Header("Retry-After", "5")
		utils.ErrorResponse(c, http.StatusServiceUnavailable, "Log storage is full; entries were not accepted. Free disk space or lower retention.", nil)
	case err != nil:
		utils.InternalError(c, "Failed to ingest logs", err)
	default:
		errs := make([]IngestError, 0, len(result.Errors))
		for _, e := range result.Errors {
			errs = append(errs, IngestError{Index: e.Index, Code: e.Code, Message: e.Message})
		}
		c.JSON(http.StatusAccepted, IngestResponse{Accepted: result.Accepted, Rejected: result.Rejected, Errors: errs})
	}
}

// readIngestBody reads the request body, transparently gunzipping Content-Encoding: gzip
// (the OpenTelemetry exporters send gzip by default). The decompressed size obeys the
// same cap as an uncompressed body. On failure it has already written the response.
func readIngestBody(c *gin.Context) ([]byte, bool) {
	limit := maxBodyBytesFor(c.FullPath(), c.GetHeader("Content-Type"))
	var reader io.Reader = c.Request.Body
	if strings.EqualFold(strings.TrimSpace(c.GetHeader("Content-Encoding")), "gzip") {
		gz, err := gzip.NewReader(c.Request.Body)
		if err != nil {
			utils.BadRequest(c, "Request body is not valid gzip")
			return nil, false
		}
		defer gz.Close()
		reader = io.LimitReader(gz, limit+1)
	}
	body, err := io.ReadAll(reader)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			utils.ErrorResponse(c, http.StatusRequestEntityTooLarge, "Request body too large", nil)
		} else {
			utils.BadRequest(c, "Could not read the request body")
		}
		return nil, false
	}
	if int64(len(body)) > limit {
		utils.ErrorResponse(c, http.StatusRequestEntityTooLarge, "Request body too large", nil)
		return nil, false
	}
	return body, true
}

func (s *Server) ingestOTLPLogsImpl(c *gin.Context) {
	if !s.requireLogs(c) {
		return
	}
	app, err := s.logs.ingester.Authenticate(c.GetHeader("Authorization"))
	if err != nil {
		c.Header("WWW-Authenticate", `Bearer realm="orion-ingest"`)
		utils.Unauthorized(c, "Valid ingest key required")
		return
	}
	body, ok := readIngestBody(c)
	if !ok {
		return
	}
	result, format, err := s.logs.ingester.IngestOTLP(app, c.GetHeader("Content-Type"), body)
	var limited *logstore.RateLimitedError
	switch {
	case errors.Is(err, logstore.ErrUnsupportedMedia):
		utils.ErrorResponse(c, http.StatusUnsupportedMediaType, "OTLP/HTTP needs application/x-protobuf or application/json", nil)
	case errors.Is(err, logstore.ErrTooManyEntries):
		utils.ErrorResponse(c, http.StatusRequestEntityTooLarge, "Too many log records in one request (limit 10000)", nil)
	case errors.Is(err, logstore.ErrInvalidBody):
		utils.BadRequest(c, "Request body is not a valid ExportLogsServiceRequest")
	case errors.As(err, &limited):
		seconds := int((limited.RetryAfter + 999_999_999) / 1_000_000_000)
		c.Header("Retry-After", strconv.Itoa(max(seconds, 1)))
		utils.ErrorResponse(c, http.StatusTooManyRequests, "Ingest rate limit exceeded; retry after the indicated seconds", nil)
	case errors.Is(err, logstore.ErrStorageFull):
		c.Header("Retry-After", "5")
		utils.ErrorResponse(c, http.StatusServiceUnavailable, "Log storage is full; records were not accepted", nil)
	case err != nil:
		utils.InternalError(c, "Failed to ingest logs", err)
	default:
		payload, contentType, encodeErr := logstore.OTLPResponse(format, result.Rejected, result.Errors)
		if encodeErr != nil {
			utils.InternalError(c, "Failed to encode the response", encodeErr)
			return
		}
		c.Data(http.StatusOK, contentType, payload)
	}
}
