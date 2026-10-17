package cli

import (
	"os"
	"path/filepath"

	"orion/agent/internal/config"
	"orion/agent/internal/control"
	agentstate "orion/agent/internal/state"
)

// Per-user ("--user") installs live under XDG-style directories. The CLI picks
// them automatically when no system install exists, so `orion-agent status`
// just works for both kinds of install.

const (
	envConfigPath = "ORION_CONFIG"
	envStatePath  = "ORION_STATE"
)

func userHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}

func userConfigPath() string {
	if h := userHomeDir(); h != "" {
		return filepath.Join(h, ".config", "orion", "config.yaml")
	}
	return ""
}

func userStateDir() string {
	if h := userHomeDir(); h != "" {
		return filepath.Join(h, ".local", "state", "orion")
	}
	return ""
}

func fileExists(p string) bool {
	if p == "" {
		return false
	}
	_, err := os.Stat(p)
	return err == nil
}

// preferUserInstall is true for non-root users with a per-user install and no
// system install.
func preferUserInstall() bool {
	return os.Geteuid() != 0 && fileExists(userConfigPath()) && !fileExists(config.DefaultPath())
}

func defaultConfigPath() string {
	if v := os.Getenv(envConfigPath); v != "" {
		return v
	}
	if preferUserInstall() {
		return userConfigPath()
	}
	return config.DefaultPath()
}

func defaultStatePath() string {
	if v := os.Getenv(envStatePath); v != "" {
		return v
	}
	if preferUserInstall() {
		return filepath.Join(userStateDir(), "state.db")
	}
	return agentstate.DefaultPath()
}

func defaultSocketPath() string {
	if os.Getenv(control.SocketEnv) == "" && preferUserInstall() {
		return filepath.Join(userStateDir(), "agent.sock")
	}
	return control.DefaultSocketPath()
}
