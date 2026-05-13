package config

import (
	"strings"
	"testing"
)

func TestUserConfigValidateAcceptsValidMonitorTypes(t *testing.T) {
	cfg := UserConfig{
		CoreURL:  "http://localhost:8999",
		Interval: "60s",
		Monitors: []UserMonitor{
			{
				Name:     "api",
				Type:     UserMonitorTypeHTTPHealthcheck,
				Interval: "30s",
				HTTP: &HTTPHealthcheckConfig{
					URL:            "https://example.com/health",
					Timeout:        "5s",
					ExpectedStatus: 200,
				},
			},
			{
				Name:     "website",
				Type:     UserMonitorTypeWebsite,
				Interval: "1m",
				Website: &WebsiteMonitorConfig{
					URL:            "https://example.com",
					Timeout:        "10s",
					ExpectedStatus: 200,
				},
			},
			{
				Name:     "service",
				Type:     UserMonitorInternalService,
				Interval: "15s",
				InternalService: &InternalServiceConfig{
					Ping:    PingConfig{URL: "http://127.0.0.1:8080/health", Timeout: "2s"},
					Process: ProcessConfig{Port: 8080},
				},
			},
			{
				Name:     "backup",
				Type:     UserMonitorTypeCommand,
				Interval: "5m",
				Command:  &CommandMonitorConfig{Command: "test -f /tmp/backup-ok", Timeout: "30s"},
			},
			{
				Name:     "worker",
				Type:     UserMonitorTypePM2,
				Interval: "1m",
				PM2:      &PM2MonitorConfig{AppName: "worker"},
			},
			{
				Name:     "postgres",
				Type:     UserMonitorTypeTCP,
				Interval: "30s",
				TCP:      &TCPMonitorConfig{Host: "127.0.0.1", Port: 5432, Timeout: "2s"},
			},
			{
				Name:     "resources",
				Type:     UserMonitorTypeResource,
				Interval: "30s",
				Resource: &ResourceThresholdConfig{MaxCPUPercent: 90, MaxMemoryPercent: 90, MaxDiskPercent: 85, MaxLoad1: 4},
			},
			{
				Name:     "postgres-container",
				Type:     UserMonitorTypeDocker,
				Interval: "30s",
				Docker:   &DockerContainerConfig{Name: "postgres"},
			},
			{
				Name:     "nginx",
				Type:     UserMonitorTypeSystemd,
				Interval: "30s",
				Systemd:  &SystemdServiceConfig{Name: "nginx.service"},
			},
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestUserConfigValidateRejectsInvalidMonitorConfig(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*UserConfig)
		wantErr string
	}{
		{
			name: "invalid core url",
			mutate: func(cfg *UserConfig) {
				cfg.CoreURL = "localhost:8999"
			},
			wantErr: "core_url must be an absolute http or https URL",
		},
		{
			name: "non-positive interval",
			mutate: func(cfg *UserConfig) {
				cfg.Monitors[0].Interval = "0s"
			},
			wantErr: "interval must be > 0",
		},
		{
			name: "duplicate monitor names",
			mutate: func(cfg *UserConfig) {
				cfg.Monitors = append(cfg.Monitors, cfg.Monitors[0])
			},
			wantErr: "duplicate name",
		},
		{
			name: "http timeout required",
			mutate: func(cfg *UserConfig) {
				cfg.Monitors[0].HTTP.Timeout = ""
			},
			wantErr: "timeout is required",
		},
		{
			name: "http expected status required",
			mutate: func(cfg *UserConfig) {
				cfg.Monitors[0].HTTP.ExpectedStatus = 0
			},
			wantErr: "expected_status is required",
		},
		{
			name: "invalid http body regex",
			mutate: func(cfg *UserConfig) {
				cfg.Monitors[0].HTTP.ExpectedBodyRegex = "["
			},
			wantErr: "invalid expected_body_regex",
		},
		{
			name: "invalid website status",
			mutate: func(cfg *UserConfig) {
				cfg.Monitors[0].Type = UserMonitorTypeWebsite
				cfg.Monitors[0].HTTP = nil
				cfg.Monitors[0].Website = &WebsiteMonitorConfig{URL: "https://example.com", ExpectedStatus: 700}
			},
			wantErr: "expected_status must be between 100 and 599",
		},
		{
			name: "invalid internal service port",
			mutate: func(cfg *UserConfig) {
				cfg.Monitors[0].Type = UserMonitorInternalService
				cfg.Monitors[0].HTTP = nil
				cfg.Monitors[0].InternalService = &InternalServiceConfig{
					Ping:    PingConfig{URL: "http://127.0.0.1:8080/health", Timeout: "1s"},
					Process: ProcessConfig{Port: 70000},
				}
			},
			wantErr: "process.port must be between 1 and 65535",
		},
		{
			name: "blank command",
			mutate: func(cfg *UserConfig) {
				cfg.Monitors[0].Type = UserMonitorTypeCommand
				cfg.Monitors[0].HTTP = nil
				cfg.Monitors[0].Command = &CommandMonitorConfig{Command: "   "}
			},
			wantErr: "command is required",
		},
		{
			name: "invalid command timeout",
			mutate: func(cfg *UserConfig) {
				cfg.Monitors[0].Type = UserMonitorTypeCommand
				cfg.Monitors[0].HTTP = nil
				cfg.Monitors[0].Command = &CommandMonitorConfig{Command: "test -f /tmp/backup-ok", Timeout: "0s"}
			},
			wantErr: "timeout must be > 0",
		},
		{
			name: "invalid tcp port",
			mutate: func(cfg *UserConfig) {
				cfg.Monitors[0].Type = UserMonitorTypeTCP
				cfg.Monitors[0].HTTP = nil
				cfg.Monitors[0].TCP = &TCPMonitorConfig{Host: "localhost", Port: 0}
			},
			wantErr: "port must be between 1 and 65535",
		},
		{
			name: "missing resource threshold",
			mutate: func(cfg *UserConfig) {
				cfg.Monitors[0].Type = UserMonitorTypeResource
				cfg.Monitors[0].HTTP = nil
				cfg.Monitors[0].Resource = &ResourceThresholdConfig{}
			},
			wantErr: "at least one resource threshold is required",
		},
		{
			name: "invalid resource percent",
			mutate: func(cfg *UserConfig) {
				cfg.Monitors[0].Type = UserMonitorTypeResource
				cfg.Monitors[0].HTTP = nil
				cfg.Monitors[0].Resource = &ResourceThresholdConfig{MaxCPUPercent: 101}
			},
			wantErr: "max_cpu_percent must be between 0 and 100",
		},
		{
			name: "missing docker name",
			mutate: func(cfg *UserConfig) {
				cfg.Monitors[0].Type = UserMonitorTypeDocker
				cfg.Monitors[0].HTTP = nil
				cfg.Monitors[0].Docker = &DockerContainerConfig{}
			},
			wantErr: "name is required",
		},
		{
			name: "missing systemd name",
			mutate: func(cfg *UserConfig) {
				cfg.Monitors[0].Type = UserMonitorTypeSystemd
				cfg.Monitors[0].HTTP = nil
				cfg.Monitors[0].Systemd = &SystemdServiceConfig{}
			},
			wantErr: "name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validUserConfig()
			tt.mutate(&cfg)

			err := cfg.Validate()
			if err == nil {
				t.Fatalf("Validate() error = nil, want %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() error = %q, want substring %q", err.Error(), tt.wantErr)
			}
		})
	}
}

func validUserConfig() UserConfig {
	return UserConfig{
		CoreURL:  "http://localhost:8999",
		Interval: "60s",
		Monitors: []UserMonitor{
			{
				Name:     "api",
				Type:     UserMonitorTypeHTTPHealthcheck,
				Interval: "30s",
				HTTP: &HTTPHealthcheckConfig{
					URL:            "https://example.com/health",
					Timeout:        "5s",
					ExpectedStatus: 200,
				},
			},
		},
	}
}
