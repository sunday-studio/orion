package node

import (
	"testing"

	"orion/node/internal/collector"
	"orion/node/internal/config"
)

func TestResultDetail(t *testing.T) {
	cases := []struct {
		name          string
		r             *collector.MonitorResult
		detail, value string
	}{
		{"failure reason", &collector.MonitorResult{Metrics: map[string]interface{}{"failure_reason": "connection refused"}}, "connection refused", ""},
		{"error message", &collector.MonitorResult{Error: &collector.MonitorError{Message: "timeout"}}, "timeout", ""},
		{"reason wins over error", &collector.MonitorResult{Metrics: map[string]interface{}{"failure_reason": "a"}, Error: &collector.MonitorError{Message: "b"}}, "a", ""},
		{"disk reading", &collector.MonitorResult{Metrics: &map[string]interface{}{"used_percent": 91.4}}, "", "91%"},
		{"nothing", &collector.MonitorResult{}, "", ""},
	}
	for _, c := range cases {
		d, v := resultDetail(c.r)
		if d != c.detail || v != c.value {
			t.Errorf("%s: got (%q,%q), want (%q,%q)", c.name, d, v, c.detail, c.value)
		}
	}
}

func TestMonitorTarget(t *testing.T) {
	cases := []struct {
		m    config.UserMonitor
		want string
	}{
		{config.UserMonitor{Systemd: &config.SystemdServiceConfig{Name: "nginx"}}, "nginx"},
		{config.UserMonitor{Docker: &config.DockerContainerConfig{Name: "api"}}, "api"},
		{config.UserMonitor{PM2: &config.PM2MonitorConfig{AppName: "web"}}, "web"},
		{config.UserMonitor{TCP: &config.TCPMonitorConfig{Port: 6379}}, "port 6379"},
		{config.UserMonitor{DiskUsage: &config.DiskUsageConfig{Path: "/var"}}, "disk /var"},
		{config.UserMonitor{ProcessRunning: &config.ProcessRunningConfig{Name: "sshd"}}, "sshd"},
		{config.UserMonitor{}, ""},
	}
	for _, c := range cases {
		if got := monitorTarget(c.m); got != c.want {
			t.Errorf("got %q, want %q", got, c.want)
		}
	}
}
