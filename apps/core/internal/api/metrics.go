package api

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"orion/core/internal/service"
	"orion/core/internal/utils"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// metricsAuthMiddleware protects /metrics. With ORION_METRICS_TOKEN set, a
// Prometheus-style bearer token is required (and accepted instead of a login JWT).
// Without it the route follows the admin API's authentication.
func (s *Server) metricsAuthMiddleware() gin.HandlerFunc {
	adminAuth := s.frontendAuthMiddleware()
	expected := s.cfg.MetricsToken
	return func(c *gin.Context) {
		if expected == "" {
			adminAuth(c)
			return
		}
		presented := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		if subtle.ConstantTimeCompare([]byte(presented), []byte(expected)) != 1 {
			utils.Unauthorized(c, "Valid metrics token required")
			c.Abort()
		}
	}
}

// getMetrics serves Prometheus text-format metrics (not part of the JSON API contract).
func (s *Server) getMetrics(c *gin.Context) {
	staleAfter := time.Duration(s.cfg.CoreWorkerStaleSeconds) * time.Second
	body, err := service.NewMetricsService(s.db, s.logger, s.runtimeDiagnosticsService, staleAfter).Render(c.Request.Context(), time.Now().UTC())
	if err != nil {
		utils.InternalError(c, "Failed to render metrics", err)
		return
	}
	if s.logs != nil {
		st := s.logs.store.Stats()
		body += fmt.Sprintf("# HELP orion_logs_queue_entries Product log entries accepted but not yet written.\n# TYPE orion_logs_queue_entries gauge\norion_logs_queue_entries %d\n"+
			"# HELP orion_logs_written_total Product log entries written since process start.\n# TYPE orion_logs_written_total counter\norion_logs_written_total %d\n"+
			"# HELP orion_logs_dropped_total Product log entries lost to write failures since process start.\n# TYPE orion_logs_dropped_total counter\norion_logs_dropped_total %d\n"+
			"# HELP orion_logs_storage_full 1 while the last log write failed because the disk or database is full.\n# TYPE orion_logs_storage_full gauge\norion_logs_storage_full %d\n", st.Queued, st.Written, st.Dropped, boolGauge(st.StorageFull))
	}
	c.Data(http.StatusOK, "text/plain; version=0.0.4; charset=utf-8", []byte(body))
}

// RuntimeDiagnostics exposes the shared in-process diagnostics collector so
// background services started by the API binary report into the same metrics.
func (s *Server) RuntimeDiagnostics() *service.RuntimeDiagnosticsService {
	return s.runtimeDiagnosticsService
}

// Handler returns the HTTP handler for in-process embedding (the soak harness and tests).
func (s *Server) Handler() http.Handler {
	return s.router
}

func boolGauge(v bool) int {
	if v {
		return 1
	}
	return 0
}
