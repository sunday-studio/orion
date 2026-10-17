package api

import (
	"encoding/json"
	"orion/core/internal/logstore"
	"orion/core/internal/utils"
	"time"

	"github.com/gin-gonic/gin"
)

// productLogs bundles the log store, the ingest pipeline and its limiter.
type productLogs struct {
	store    *logstore.Store
	ingester *logstore.Ingester
}

// EnableProductLogs turns on product logs using an opened store. Core's API binary
// calls it at startup; until then the product-log routes answer 503.
func (s *Server) EnableProductLogs(store *logstore.Store) {
	limiter := logstore.NewAppLimiter(s.cfg.LogsAppRatePerSecond)
	s.logs = &productLogs{store: store, ingester: logstore.NewIngester(s.db, store, limiter)}
}

// ProductLogStore exposes the store for background jobs (retention) started by the binary.
func (s *Server) ProductLogStore() *logstore.Store {
	if s.logs == nil {
		return nil
	}
	return s.logs.store
}

// requireLogs answers 503 when product logs are not enabled and reports whether to continue.
func (s *Server) requireLogs(c *gin.Context) bool {
	if s.logs != nil {
		return true
	}
	utils.ErrorResponse(c, 503, "Product logs are not enabled on this Core instance", nil)
	return false
}

// parseLogTime reads an RFC 3339 time or a relative offset such as -1h or -15m.
func parseLogTime(value string, now time.Time) (time.Time, bool) {
	if value == "" {
		return time.Time{}, true
	}
	if t, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return t.UTC(), true
	}
	if len(value) > 1 && value[0] == '-' {
		if d, err := time.ParseDuration(value[1:]); err == nil {
			return now.Add(-d), true
		}
	}
	return time.Time{}, false
}

func jsonUnmarshal(s string, v any) error { return json.Unmarshal([]byte(s), v) }
