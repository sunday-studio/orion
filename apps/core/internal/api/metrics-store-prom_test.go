package api

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
)

func TestMetricsEndpointExposesTheMetricStoreCounters(t *testing.T) {
	server, store := metricsServer(t)
	_, key := createAppWithKey(t, server, "checkout-api")
	body, _ := proto.Marshal(gaugeRequest(time.Now().UTC().Add(-time.Minute), 3))
	if resp := postOTLPMetrics(server, key, "application/x-protobuf", body, false); resp.Code != http.StatusOK {
		t.Fatalf("ingest status = %d %s", resp.Code, resp.Body.String())
	}
	store.Flush()

	text := metricsRequest(server, "").Body.String()
	for _, want := range []string{
		"orion_metrics_queue_batches 0",
		"orion_metrics_written_total 3",
		`orion_metrics_dropped_total{reason="full"} 0`,
		`orion_metrics_dropped_total{reason="cap"} 0`,
		`orion_metrics_dropped_total{reason="stale"} 0`,
		"orion_metrics_batches_failed_total 0",
		"# TYPE orion_metrics_database_bytes gauge",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("metrics missing %q", want)
		}
	}
	if strings.Contains(text, "orion_metrics_database_bytes 0\n") {
		t.Errorf("the database holds a schema and rows, its size cannot be 0")
	}
	// Without a metric store nothing is rendered.
	plain := setupLogsServer(t, nil)
	if strings.Contains(metricsRequest(plain, "").Body.String(), "orion_metrics_written_total") {
		t.Error("the counters are exposed without a metric store")
	}
}
