package service

import (
	"context"
	"fmt"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
)

// MetricsService renders Prometheus text-format metrics for GET /metrics.
// Counters that live in process memory (requests, ingestion latency) come from
// RuntimeDiagnosticsService; queue and liveness gauges are read from SQLite so
// they are correct whichever process (API or Worker) wrote the data.
type MetricsService struct {
	db          *gorm.DB
	logger      *utils.Logger
	diagnostics *RuntimeDiagnosticsService
	staleAfter  time.Duration
}

func NewMetricsService(database *gorm.DB, logger *utils.Logger, diagnostics *RuntimeDiagnosticsService, workerStaleAfter time.Duration) *MetricsService {
	if workerStaleAfter <= 0 {
		workerStaleAfter = time.Minute
	}
	return &MetricsService{db: database, logger: logger, diagnostics: diagnostics, staleAfter: workerStaleAfter}
}

type metricsWriter struct {
	b     strings.Builder
	typed map[string]bool
}

func (w *metricsWriter) sample(name, help, kind string, value float64, labels ...string) {
	if !w.typed[name] {
		fmt.Fprintf(&w.b, "# HELP %s %s\n# TYPE %s %s\n", name, help, name, kind)
		w.typed[name] = true
	}
	w.b.WriteString(name)
	if len(labels) > 0 {
		pairs := make([]string, 0, len(labels)/2)
		for i := 0; i+1 < len(labels); i += 2 {
			pairs = append(pairs, fmt.Sprintf(`%s="%s"`, labels[i], escapeLabelValue(labels[i+1])))
		}
		w.b.WriteString("{" + strings.Join(pairs, ",") + "}")
	}
	fmt.Fprintf(&w.b, " %g\n", value)
}

func escapeLabelValue(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	return strings.ReplaceAll(value, "\n", `\n`)
}

// Render returns the metrics exposition text.
func (s *MetricsService) Render(ctx context.Context, now time.Time) (string, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	w := &metricsWriter{typed: map[string]bool{}}
	w.sample("orion_up", "Core API process is serving metrics.", "gauge", 1)

	if err := s.writeRuntime(ctx, w); err != nil {
		return "", err
	}
	if err := s.writeAlertQueue(w, now); err != nil {
		return "", err
	}
	if err := s.writeWorkers(ctx, w, now); err != nil {
		return "", err
	}
	if err := s.writeIncidents(w); err != nil {
		return "", err
	}
	return w.b.String(), nil
}

func (s *MetricsService) writeRuntime(ctx context.Context, w *metricsWriter) error {
	snapshot, err := s.diagnostics.Snapshot(ctx)
	if err != nil {
		return err
	}
	w.sample("orion_process_uptime_seconds", "Seconds since this API process started.", "gauge", float64(snapshot.UptimeSeconds))

	routes := make([]string, 0, len(snapshot.Requests))
	for route := range snapshot.Requests {
		routes = append(routes, route)
	}
	sort.Strings(routes)
	for _, route := range routes {
		statuses := make([]string, 0, len(snapshot.Requests[route]))
		for status := range snapshot.Requests[route] {
			statuses = append(statuses, status)
		}
		sort.Strings(statuses)
		for _, status := range statuses {
			w.sample("orion_http_requests_total", "HTTP requests by route template and status.", "counter", float64(snapshot.Requests[route][status]), "route", route, "status", status)
		}
	}

	kinds := make([]string, 0, len(snapshot.Ingestion))
	for kind := range snapshot.Ingestion {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	for _, kind := range kinds {
		p := snapshot.Ingestion[kind]
		w.sample("orion_ingestion_requests_total", "Ingestion requests handled since process start.", "counter", float64(p.Count), "kind", kind)
		for quantile, value := range map[string]int64{"0.5": p.P50, "0.95": p.P95, "0.99": p.P99} {
			w.sample("orion_ingestion_latency_ms", "Ingestion latency in milliseconds over the recent sample window.", "gauge", float64(value), "kind", kind, "quantile", quantile)
		}
	}

	w.sample("orion_sqlite_busy_total", "SQLITE_BUSY errors observed since process start.", "counter", float64(snapshot.SQLite.BusyTotal))
	w.sample("orion_sqlite_database_bytes", "Database size in bytes (page_count * page_size).", "gauge", float64(snapshot.SQLite.DatabaseBytes))
	w.sample("orion_sqlite_freelist_pages", "Free pages that retention could return to the OS.", "gauge", float64(snapshot.SQLite.FreelistCount))
	return nil
}

func (s *MetricsService) writeAlertQueue(w *metricsWriter, now time.Time) error {
	var byStatus []struct {
		Status string
		Count  int64
	}
	if err := s.db.Model(&db.AlertDelivery{}).Select("status, COUNT(*) AS count").Group("status").Scan(&byStatus).Error; err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, row := range byStatus {
		seen[row.Status] = true
		w.sample("orion_alert_deliveries", "Alert deliveries by status.", "gauge", float64(row.Count), "status", row.Status)
	}
	for _, status := range []string{"pending", "failed", "sent"} {
		if !seen[status] {
			w.sample("orion_alert_deliveries", "Alert deliveries by status.", "gauge", 0, "status", status)
		}
	}

	var oldest db.AlertDelivery
	age := 0.0
	err := s.db.Where("status = ? AND (next_attempt_at IS NULL OR next_attempt_at <= ?)", "pending", now).Order("created_at ASC").Limit(1).Find(&oldest).Error
	if err != nil {
		return err
	}
	if oldest.ID != "" {
		age = max(now.Sub(oldest.CreatedAt).Seconds(), 0)
	}
	w.sample("orion_alert_delivery_oldest_pending_age_seconds", "Age of the oldest due pending alert delivery (0 when none). Alert on this to detect a stuck dispatcher.", "gauge", age)
	return nil
}

func (s *MetricsService) writeWorkers(ctx context.Context, w *metricsWriter, now time.Time) error {
	diag, err := NewWorkerDiagnosticsService(s.db, s.logger).GetDiagnostics(ctx, s.staleAfter, now)
	if err != nil {
		return err
	}
	w.sample("orion_worker_online", "Number of Core workers with a fresh heartbeat.", "gauge", float64(diag.OnlineCount))
	w.sample("orion_worker_stale", "Number of Core workers whose heartbeat is stale.", "gauge", float64(diag.StaleCount))
	for _, row := range diag.Workers {
		w.sample("orion_worker_heartbeat_age_seconds", "Seconds since the worker's last heartbeat.", "gauge", float64(row.HeartbeatAgeSeconds), "worker_id", row.WorkerID)
	}
	return nil
}

func (s *MetricsService) writeIncidents(w *metricsWriter) error {
	var open int64
	if err := s.db.Model(&db.Incident{}).Where("status IN ?", activeIncidentStatuses()).Count(&open).Error; err != nil {
		return err
	}
	w.sample("orion_incidents_active", "Open, acknowledged or covered incidents.", "gauge", float64(open))
	return nil
}
