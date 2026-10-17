package soak

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"
)

func envInt(name string, fallback int) int {
	if v, err := strconv.Atoi(os.Getenv(name)); err == nil && v > 0 {
		return v
	}
	return fallback
}

func envFloat(name string, fallback float64) float64 {
	if v, err := strconv.ParseFloat(os.Getenv(name), 64); err == nil && v >= 0 {
		return v
	}
	return fallback
}

func envDuration(name string, fallback time.Duration) time.Duration {
	if v, err := time.ParseDuration(os.Getenv(name)); err == nil && v > 0 {
		return v
	}
	return fallback
}

// TestSoak is the quick CI run by default. Scale it up through the environment, for
// example the plan's load profile and the 72h staging soak:
//
//	ORION_SOAK_AGENTS=50 ORION_SOAK_MONITORS=20 ORION_SOAK_INTERVAL=30s ORION_SOAK_DURATION=72h \
//	  go test ./internal/soak -run TestSoak -timeout 80h -v
func TestSoak(t *testing.T) {
	cfg := Config{
		Agents:           envInt("ORION_SOAK_AGENTS", 10),
		MonitorsPerAgent: envInt("ORION_SOAK_MONITORS", 5),
		ReportInterval:   envDuration("ORION_SOAK_INTERVAL", time.Second),
		Duration:         envDuration("ORION_SOAK_DURATION", 10*time.Second),
		Outages:          envInt("ORION_SOAK_OUTAGES", 0),
		FastBackoff:      os.Getenv("ORION_SOAK_REAL_BACKOFF") == "",
		// The short CI run injects many webhook failures so the retry path is always
		// exercised; the plan's long soak uses the realistic 5%.
		WebhookFailRate: envFloat("ORION_SOAK_WEBHOOK_FAIL_RATE", 0.35),
	}
	result, err := Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	t.Logf("reports=%d errors=%d ingest p50=%v p95=%v p99=%v sqlite_busy=%d db_bytes=%d webhook calls=%d (injected failures %d) outages=%d goroutines %d->%d",
		result.Reports, result.ReportErrors, result.P50, result.P95, result.P99, result.SQLiteBusy, result.DatabaseBytes,
		result.WebhookCalls, result.WebhookFailures, len(result.Outages), result.GoroutinesBefore, result.GoroutinesAfter)
	for route, ms := range result.ServerLatency {
		t.Logf("server-side %s = %dms", route, ms)
	}
	for _, violation := range result.Violations {
		t.Error(violation)
	}
	if result.Reports == 0 {
		t.Fatal("no reports were sent")
	}
	if cfg.ReportInterval == time.Second && result.P95 > 200*time.Millisecond {
		t.Errorf("ingest p95 = %v, want under 200ms", result.P95)
	}
}
