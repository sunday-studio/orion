package api

import (
	"net/http"
	"orion/core/internal/utils"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	defaultMaxBodyBytes    int64 = 1 << 20 // 1 MiB
	serviceLogMaxBodyBytes int64 = 5 << 20 // 5 MiB for log batches
	maxRequestIDLength           = 64
)

var safeRequestID = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// maxBodyBytesFor returns the request body cap for a route.
func maxBodyBytesFor(fullPath string, contentType string) int64 {
	if strings.HasSuffix(fullPath, "/logs/batch") {
		return serviceLogMaxBodyBytes
	}
	if strings.HasPrefix(fullPath, "/v1/ingest/") && strings.Contains(strings.ToLower(contentType), "ndjson") {
		return serviceLogMaxBodyBytes // 5 MiB for NDJSON ingest batches
	}
	return defaultMaxBodyBytes
}

// BodyLimitMiddleware rejects oversized bodies with 413 before a handler reads
// them and caps what a handler can read when the length is not declared.
func BodyLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		limit := maxBodyBytesFor(c.FullPath(), c.GetHeader("Content-Type"))
		if c.Request.ContentLength > limit {
			utils.ErrorResponse(c, http.StatusRequestEntityTooLarge, "Request body too large", nil)
			c.Abort()
			return
		}
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		}
		c.Next()
	}
}

// SecurityHeadersMiddleware sets conservative browser security headers. The CSP
// applies to the SPA and public status pages, not the JSON API or Swagger UI.
func SecurityHeadersMiddleware() gin.HandlerFunc {
	const csp = "default-src 'self'; img-src 'self' data: https:; style-src 'self' 'unsafe-inline'; font-src 'self' data:; connect-src 'self'; frame-ancestors 'self'; base-uri 'self'; form-action 'self'"
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "SAMEORIGIN")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		if c.Request.TLS != nil {
			h.Set("Strict-Transport-Security", "max-age=31536000")
		}
		path := c.Request.URL.Path
		if !strings.HasPrefix(path, "/v1/") && !strings.HasPrefix(path, "/swagger") {
			h.Set("Content-Security-Policy", csp)
		}
		c.Next()
	}
}

// AccessLogMiddleware logs one line per request using the route template, never
// the raw URL, so path-embedded secrets (heartbeat and subscriber tokens) are not
// written to logs. Errors attached with c.Error are logged here, not returned to clients.
func AccessLogMiddleware(logger *utils.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		started := time.Now()
		c.Next()

		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}
		requestID, _ := c.Get("request_id")
		args := []any{"method", c.Request.Method, "route", route, "status", c.Writer.Status(), "duration_ms", time.Since(started).Milliseconds(), "request_id", requestID}
		if len(c.Errors) > 0 {
			args = append(args, "error", c.Errors.String())
		}
		logger.Info("HTTP request", args...)
	}
}

// requestIDFrom accepts a caller-supplied request id only when it is short and
// made of safe characters; otherwise a fresh one is generated (no log injection).
func requestIDFrom(header string) string {
	header = strings.TrimSpace(header)
	if header != "" && len(header) <= maxRequestIDLength && safeRequestID.MatchString(header) {
		return header
	}
	return uuid.New().String()
}
