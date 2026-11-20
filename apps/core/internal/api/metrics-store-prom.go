package api

import "fmt"

// metricStorePrometheus renders the metric store's counters (OTLP metrics and resource history) in the
// Prometheus text format, so an operator can alert on dropped or failed writes.
func (s *Server) metricStorePrometheus() string {
	if s.metrics == nil {
		return ""
	}
	st := s.metrics.Stats()
	return fmt.Sprintf("# HELP orion_metrics_queue_batches Metric batches accepted but not yet written.\n# TYPE orion_metrics_queue_batches gauge\norion_metrics_queue_batches %d\n"+
		"# HELP orion_metrics_written_total Metric sample rows written since process start.\n# TYPE orion_metrics_written_total counter\norion_metrics_written_total %d\n"+
		"# HELP orion_metrics_dropped_total Metric samples dropped since process start, by reason: full is a full write queue, cap is a series cap, stale is a cumulative reading not newer than the last or a repeated delta reading.\n# TYPE orion_metrics_dropped_total counter\n"+
		"orion_metrics_dropped_total{reason=\"full\"} %d\norion_metrics_dropped_total{reason=\"cap\"} %d\norion_metrics_dropped_total{reason=\"stale\"} %d\n"+
		"# HELP orion_metrics_batches_failed_total Metric batches that could not be written since process start.\n# TYPE orion_metrics_batches_failed_total counter\norion_metrics_batches_failed_total %d\n"+
		"# HELP orion_metrics_database_bytes Size of metrics.db in bytes.\n# TYPE orion_metrics_database_bytes gauge\norion_metrics_database_bytes %d\n",
		st.QueueDepth, st.Written, st.DroppedFull, st.DroppedCap, st.DroppedStale, st.BatchesFailed, st.DatabaseBytes)
}
