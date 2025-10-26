package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type InternalState struct {
	AgentID      string                     `yaml:"agent_id"`
	Token        string                     `yaml:"token"`
	Registered   bool                       `yaml:"registered"`
	CoreURL      string                     `yaml:"core_url"`
	LastSync     time.Time                  `yaml:"last_sync"`
	Applications []InternalStateApplication `yaml:"applications"`
}

type InternalStateApplication struct {
	Name        string    `yaml:"name"`
	ID          string    `yaml:"id"`
	Status      string    `yaml:"status"`
	LastChecked time.Time `yaml:"last_checked"`
}

type UserApplicationType string

const (
	UserApplicationTypeNginx             UserApplicationType = "nginx"
	UserApplicationTypeServerHealthcheck UserApplicationType = "server-healthcheck"
)

var UserApplicationTypes = []UserApplicationType{
	UserApplicationTypeNginx,
	UserApplicationTypeServerHealthcheck,
}

type UserApplication struct {
	Name        string              `yaml:"name"`
	Description string              `yaml:"description"`
	Type        UserApplicationType `yaml:"type"`
	Interval    string              `yaml:"interval"`
	Cmd         *string             `yaml:"cmd"`
	Url         *string             `yaml:"url"`
}

type UserConfig struct {
	CoreURL      string            `yaml:"core_url"`
	Interval     string            `yaml:"interval"`
	Applications []UserApplication `yaml:"applications"`
}

func LoadInternalState(path string) (*InternalState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
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

func (c *UserConfig) Validate() error {
	if c.CoreURL == "" {
		return errors.New("core_url is required")
	}
	if c.Interval == "" {
		return errors.New("interval is required")
	}
	if _, err := time.ParseDuration(c.Interval); err != nil {
		return fmt.Errorf("invalid interval format: %w", err)
	}
	return nil
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

func (c *InternalState) UpdateApplications(applications []InternalStateApplication) {
	c.Applications = applications
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
