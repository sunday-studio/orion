// internal/config/config.go
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type SubProcess struct {
	Name string `yaml:"name"`
	Cmd  string `yaml:"cmd"`
}

// Config defines all configuration options for the agent.
type Config struct {
	CoreURL      string       `yaml:"core_url"`      // Core server endpoint
	Token        string       `yaml:"token"`         // Auth token for API calls
	Interval     string       `yaml:"interval"`      // e.g., "60s", "5m"
	SubProcesses []SubProcess `yaml:"subprocesses"`  // Optional subprocess definitions
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &cfg, nil
}

// Validate checks that required fields are set and the interval is valid.
func (c *Config) Validate() error {
	if c.CoreURL == "" {
		return errors.New("core_url is required")
	}
	if c.Token == "" {
		return errors.New("token is required")
	}
	if c.Interval == "" {
		return errors.New("interval is required")
	}
	if _, err := time.ParseDuration(c.Interval); err != nil {
		return fmt.Errorf("invalid interval format: %w", err)
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
