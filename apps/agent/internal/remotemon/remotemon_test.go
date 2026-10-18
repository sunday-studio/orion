package remotemon

import (
	"strings"
	"testing"

	"orion/agent/internal/config"
	"orion/agent/internal/transport"
)

func TestConvertMapsEveryConsoleCheckType(t *testing.T) {
	defs := []transport.PulledDefinition{
		{ID: "1", Name: "root-disk", CheckType: "disk", Params: map[string]any{"path": "/var", "threshold_percent": 85.0}, IntervalSeconds: 30},
		{ID: "2", Name: "mem", CheckType: "memory", Params: map[string]any{"threshold_percent": 80.0}},
		{ID: "3", Name: "nginx-up", CheckType: "process", Params: map[string]any{"process_name": "nginx"}, IntervalSeconds: 15},
		{ID: "4", Name: "web-port", CheckType: "port", Params: map[string]any{"port": 8080.0}},
	}
	ms, problems := Convert(defs)
	if len(problems) != 0 || len(ms) != 4 {
		t.Fatalf("ms=%d problems=%v", len(ms), problems)
	}
	by := map[string]config.UserMonitor{}
	for _, m := range ms {
		by[m.Name] = m
		if err := m.Validate(); err != nil {
			t.Errorf("%s does not validate: %v", m.Name, err)
		}
	}
	if d := by["root-disk"]; d.Type != config.UserMonitorTypeDiskUsage || d.DiskUsage.Path != "/var" || d.DiskUsage.MaxPercent != 85 || d.Interval != "30s" {
		t.Errorf("disk = %+v", d)
	}
	if m := by["mem"]; m.Type != config.UserMonitorTypeResource || m.Resource.MaxMemoryPercent != 80 || m.Interval != "60s" {
		t.Errorf("memory = %+v", m)
	}
	if p := by["nginx-up"]; p.ProcessRunning.Name != "nginx" {
		t.Errorf("process = %+v", p)
	}
	if p := by["web-port"]; p.TCP.Port != 8080 || p.TCP.Host != "127.0.0.1" {
		t.Errorf("port = %+v", p)
	}
}

func TestConvertReportsUnrunnableDefinitions(t *testing.T) {
	_, problems := Convert([]transport.PulledDefinition{
		{Name: "bad-port", CheckType: "port", Params: map[string]any{"port": 99999.0}},
		{Name: "no-name", CheckType: "process"},
		{Name: "future", CheckType: "gpu"},
	})
	if len(problems) != 3 || !strings.Contains(problems["future"], "update the agent") {
		t.Fatalf("problems = %v", problems)
	}
}

func TestMergeKeepsFileMonitorsAndFingerprintDetectsChange(t *testing.T) {
	cfg := &config.UserConfig{Monitors: []config.UserMonitor{{Name: "mem", Description: "hand written"}}}
	remote, _ := Convert([]transport.PulledDefinition{{Name: "mem", CheckType: "memory"}, {Name: "p", CheckType: "process", Params: map[string]any{"process_name": "x"}}})
	Merge(cfg, remote)
	if len(cfg.Monitors) != 2 || cfg.Monitors[0].Description != "hand written" {
		t.Fatalf("monitors = %+v", cfg.Monitors)
	}
	a := Fingerprint(remote)
	remote[0].Interval = "5s"
	if a == Fingerprint(remote) {
		t.Fatal("fingerprint must change when a definition changes")
	}
}
