package config

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"gopkg.in/yaml.v3"
)

type InternalState struct {
	AgentID           string                 `yaml:"agent_id"`
	Token             string                 `yaml:"token"`
	Registered        bool                   `yaml:"registered"`
	CoreURL           string                 `yaml:"core_url"`
	LastSync          time.Time              `yaml:"last_sync"`
	MaintenanceMode   bool                   `yaml:"maintenance_mode"`
	MaintenanceReason *string                `yaml:"maintenance_reason,omitempty"`
	Monitors          []InternalStateMonitor `yaml:"monitors"`
}

type InternalStateMonitor struct {
	Name        string    `yaml:"name"`
	ID          string    `yaml:"id"`
	Status      string    `yaml:"status"`
	LastChecked time.Time `yaml:"last_checked"`
}

type UserMonitorType string

const (
	UserMonitorTypeHTTPHealthcheck UserMonitorType = "http-healthcheck"
	UserMonitorInternalService     UserMonitorType = "internal-service"
	UserMonitorTypeCommand         UserMonitorType = "command"
	UserMonitorTypeWebsite         UserMonitorType = "website"
	UserMonitorTypePM2             UserMonitorType = "pm2"
	UserMonitorTypeTCP             UserMonitorType = "tcp"
	UserMonitorTypeResource        UserMonitorType = "resource-threshold"
	UserMonitorTypeDocker          UserMonitorType = "docker-container"
	UserMonitorTypeSystemd         UserMonitorType = "systemd-service"
)

var UserMonitorTypes = []UserMonitorType{
	UserMonitorTypeHTTPHealthcheck,
	UserMonitorInternalService,
	UserMonitorTypeCommand,
	UserMonitorTypeWebsite,
	UserMonitorTypePM2,
	UserMonitorTypeTCP,
	UserMonitorTypeResource,
	UserMonitorTypeDocker,
	UserMonitorTypeSystemd,
}

type HTTPHealthcheckConfig struct {
	URL               string `yaml:"url"`
	Timeout           string `yaml:"timeout"`
	ExpectedStatus    int    `yaml:"expected_status"`
	ExpectedBody      string `yaml:"expected_body,omitempty"`
	ExpectedBodyRegex string `yaml:"expected_body_regex,omitempty"`
}

type InternalServiceConfig struct {
	Ping    PingConfig    `yaml:"ping"`
	Process ProcessConfig `yaml:"process"`
}

type PingConfig struct {
	URL     string `yaml:"url"`
	Timeout string `yaml:"timeout"`
}

type ProcessConfig struct {
	Port int `yaml:"port"`
}

type CommandMonitorConfig struct {
	Command string   `yaml:"command"`
	Args    []string `yaml:"args,omitempty"`
	Timeout string   `yaml:"timeout,omitempty"`
}

type WebsiteMonitorConfig struct {
	URL            string `yaml:"url"`
	Timeout        string `yaml:"timeout,omitempty"`
	ExpectedStatus int    `yaml:"expected_status,omitempty"`
}

type PM2MonitorConfig struct {
	AppName string `yaml:"app_name"`
}

type TCPMonitorConfig struct {
	Host    string `yaml:"host"`
	Port    int    `yaml:"port"`
	Timeout string `yaml:"timeout,omitempty"`
}

type ResourceThresholdConfig struct {
	MaxCPUPercent    float64 `yaml:"max_cpu_percent,omitempty"`
	MaxMemoryPercent float64 `yaml:"max_memory_percent,omitempty"`
	MaxDiskPercent   float64 `yaml:"max_disk_percent,omitempty"`
	MaxLoad1         float64 `yaml:"max_load_1,omitempty"`
}

type DockerContainerConfig struct {
	Name string `yaml:"name"`
}

type SystemdServiceConfig struct {
	Name string `yaml:"name"`
}

type UserMonitor struct {
	Name        string                 `yaml:"name"`
	Description string                 `yaml:"description"`
	Type        UserMonitorType        `yaml:"type"`
	Interval    string                 `yaml:"interval"`
	Meta        map[string]interface{} `yaml:"meta,omitempty"`

	HTTP            *HTTPHealthcheckConfig   `yaml:"http,omitempty"`
	InternalService *InternalServiceConfig   `yaml:"internal_service,omitempty"`
	Command         *CommandMonitorConfig    `yaml:"command,omitempty"`
	Website         *WebsiteMonitorConfig    `yaml:"website,omitempty"`
	PM2             *PM2MonitorConfig        `yaml:"pm2,omitempty"`
	TCP             *TCPMonitorConfig        `yaml:"tcp,omitempty"`
	Resource        *ResourceThresholdConfig `yaml:"resource,omitempty"`
	Docker          *DockerContainerConfig   `yaml:"docker,omitempty"`
	Systemd         *SystemdServiceConfig    `yaml:"systemd,omitempty"`
}

type UserConfig struct {
	Meta        map[string]interface{} `yaml:"meta,omitempty"`
	CoreURL     string                 `yaml:"core_url"`
	Interval    string                 `yaml:"interval"`
	GeoLocation bool                   `yaml:"geo_location,omitempty"`
	Logging     LoggingConfig          `yaml:"logging,omitempty"`
	Monitors    []UserMonitor          `yaml:"monitors"`
}

type LoggingConfig struct {
	Level      string `yaml:"level,omitempty"`
	Path       string `yaml:"path,omitempty"`
	Format     string `yaml:"format,omitempty"`
	MaxSizeMB  int    `yaml:"max_size_mb,omitempty"`
	MaxBackups int    `yaml:"max_backups,omitempty"`
	MaxAgeDays int    `yaml:"max_age_days,omitempty"`
	Compress   *bool  `yaml:"compress,omitempty"`
}

func LoadUserConfig(path string) (*UserConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg UserConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	cfg.ApplyDefaults()

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

func (c *UserConfig) ApplyDefaults() {
	c.Logging = c.Logging.WithDefaults()
}

func DefaultLoggingConfig() LoggingConfig {
	compress := true
	return LoggingConfig{
		Level:      "info",
		Path:       DefaultLogPath(),
		Format:     "json",
		MaxSizeMB:  25,
		MaxBackups: 5,
		MaxAgeDays: 14,
		Compress:   &compress,
	}
}

func (l LoggingConfig) WithDefaults() LoggingConfig {
	defaults := DefaultLoggingConfig()
	if l.Level == "" {
		l.Level = defaults.Level
	}
	if l.Path == "" {
		l.Path = defaults.Path
	}
	if l.Format == "" {
		l.Format = defaults.Format
	}
	if l.MaxSizeMB == 0 {
		l.MaxSizeMB = defaults.MaxSizeMB
	}
	if l.MaxBackups == 0 {
		l.MaxBackups = defaults.MaxBackups
	}
	if l.MaxAgeDays == 0 {
		l.MaxAgeDays = defaults.MaxAgeDays
	}
	if l.Compress == nil {
		l.Compress = defaults.Compress
	}
	return l
}

func (l LoggingConfig) CompressEnabled() bool {
	if l.Compress == nil {
		return true
	}
	return *l.Compress
}

func (c *InternalState) IsRegistered() bool {
	return c.AgentID != "" && c.Token != ""
}

// DefaultPath returns the default path for the agent config.
// On Linux: /etc/orion/config.yaml
// On macOS: /usr/local/etc/orion/config.yaml
func DefaultPath() string {
	switch runtime.GOOS {
	case "linux":
		return "/etc/orion/config.yaml"
	case "darwin":
		return "/usr/local/etc/orion/config.yaml"
	default:
		return "config.yaml"
	}
}

// DefaultLogPath returns the default structured runtime log path.
// On Linux: /var/log/orion/agent.log
// On macOS: /usr/local/var/log/orion/agent.log
func DefaultLogPath() string {
	switch runtime.GOOS {
	case "linux":
		return "/var/log/orion/agent.log"
	case "darwin":
		return "/usr/local/var/log/orion/agent.log"
	default:
		return "agent.log"
	}
}
