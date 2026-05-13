package config

import (
	"fmt"
	"os"
	"path/filepath"
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
)

var UserMonitorTypes = []UserMonitorType{
	UserMonitorTypeHTTPHealthcheck,
	UserMonitorInternalService,
	UserMonitorTypeCommand,
	UserMonitorTypeWebsite,
	UserMonitorTypePM2,
	UserMonitorTypeTCP,
	UserMonitorTypeResource,
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
	Command string `yaml:"command"`
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
}

type UserConfig struct {
	Meta     map[string]interface{} `yaml:"meta,omitempty"`
	CoreURL  string                 `yaml:"core_url"`
	Interval string                 `yaml:"interval"`
	Monitors []UserMonitor          `yaml:"monitors"`
}

func LoadInternalState(path string) (*InternalState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		// First run: state file does not exist
		if os.IsNotExist(err) {
			state := &InternalState{
				Monitors: []InternalStateMonitor{},
			}

			if err := state.Save(path); err != nil {
				return nil, fmt.Errorf("failed to create internal state file: %w", err)
			}

			return state, nil
		}

		return nil, fmt.Errorf("failed to read internal state file: %w", err)
	}

	var internalState InternalState
	if err := yaml.Unmarshal(data, &internalState); err != nil {
		return nil, fmt.Errorf("failed to parse internal state: %w", err)
	}

	return &internalState, nil
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

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

func (c *InternalState) IsRegistered() bool {
	return c.AgentID != "" && c.Token != ""
}

func (c *InternalState) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func (c *InternalState) UpdateRegistration(agentID string, token string, coreURL string) {
	c.AgentID = agentID
	c.Token = token
	c.Registered = true
	c.LastSync = time.Now()
	c.CoreURL = coreURL
}

func (c *InternalState) UpdateMonitors(monitors []InternalStateMonitor) {
	c.Monitors = monitors
}

func (c *InternalState) GetMonitorByName(name string) *InternalStateMonitor {
	for _, monitor := range c.Monitors {
		if monitor.Name == name {
			return &monitor
		}
	}
	return nil
}

// DefaultPath returns the default path for the agent config.
// On Linux: /etc/orion/config.yaml
// On macOS: /usr/local/etc/orion/config.yaml
func DefaultPath() string {
	// Try Linux-style first
	if _, err := os.Stat("/etc/orion/config.yaml"); err == nil {
		return "/etc/orion/config.yaml"
	}

	// Fallback for macOS
	if _, err := os.Stat("/usr/local/etc/orion/config.yaml"); err == nil {
		return "/usr/local/etc/orion/config.yaml"
	}

	// Fallback to current working directory
	cwd, _ := os.Getwd()
	return filepath.Join(cwd, "config.yaml")
}
