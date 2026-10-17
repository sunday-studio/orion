package api

import (
	"errors"
	"io"
	"net/http"
	"orion/core/internal/logstore"
	"orion/core/internal/utils"
	"strconv"

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

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			utils.ErrorResponse(c, http.StatusRequestEntityTooLarge, "Request body too large", nil)
			return
		}
		utils.BadRequest(c, "Could not read the request body")
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
