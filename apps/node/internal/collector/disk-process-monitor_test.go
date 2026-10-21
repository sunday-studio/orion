package collector

import (
	"errors"
	"testing"
)

func TestDiskUsageThreshold(t *testing.T) {
	use := func(p float64) func(string) (float64, error) { return func(string) (float64, error) { return p, nil } }
	if r := runDiskUsage("/", 90, use(50)); r.Status != "up" {
		t.Fatalf("%+v", r)
	}
	r := runDiskUsage("/", 90, use(95))
	if r.Status != "down" || r.Metrics.(map[string]any)["failure_reason"] == nil {
		t.Fatalf("%+v", r)
	}
	if r := runDiskUsage("/nope", 90, func(string) (float64, error) { return 0, errors.New("no such path") }); r.Status != "down" || r.Error == nil {
		t.Fatalf("%+v", r)
	}
}

func TestProcessRunning(t *testing.T) {
	list := func() ([]string, error) { return []string{"nginx", "Postgres", "nginx"}, nil }
	if r := runProcess("nginx", list); r.Status != "up" || r.Metrics.(map[string]any)["process_count"] != 2 {
		t.Fatalf("%+v", r)
	}
	if r := runProcess("postgres", list); r.Status != "up" {
		t.Fatalf("case-insensitive match failed: %+v", r)
	}
	if r := runProcess("redis", list); r.Status != "down" {
		t.Fatalf("%+v", r)
	}
}
