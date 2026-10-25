package collector

import (
	"testing"
	"time"
)

func TestWithDurationAddsMillisecondsToEveryMetricsShape(t *testing.T) {
	ptr := &map[string]interface{}{"status_code": 200}
	if got := withDuration(ptr, 42*time.Millisecond).(*map[string]interface{}); (*got)["duration_ms"] != int64(42) || (*got)["status_code"] != 200 {
		t.Fatalf("map metrics = %v", *got)
	}
	var pm2 PM2MonitorResult
	pm2.Metrics.PID = 7
	got := withDuration(&pm2.Metrics, 3*time.Millisecond).(map[string]interface{})
	if got["duration_ms"] != int64(3) || got["pid"] != float64(7) {
		t.Fatalf("struct metrics = %v", got)
	}
	if got := withDuration(nil, time.Millisecond).(map[string]interface{}); got["duration_ms"] != int64(1) {
		t.Fatalf("nil metrics = %v", got)
	}
}

func TestWithDurationKeepsADurationTheCheckReported(t *testing.T) {
	ptr := &map[string]interface{}{"duration_ms": int64(900)}
	if got := withDuration(ptr, time.Second).(*map[string]interface{}); (*got)["duration_ms"] != int64(900) {
		t.Fatalf("overwritten: %v", *got)
	}
}

func TestWithDurationNeverReportsZeroForACheckThatRan(t *testing.T) {
	got := withDuration(&map[string]interface{}{}, 200*time.Microsecond).(*map[string]interface{})
	if (*got)["duration_ms"] != int64(1) {
		t.Fatalf("got %v", *got)
	}
}
