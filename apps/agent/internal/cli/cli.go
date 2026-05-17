package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"orion/agent/internal/logging"
)

// DetectServiceManager detects which service manager is available
func DetectServiceManager() string {
	if _, err := exec.LookPath("systemctl"); err == nil {
		return "systemd"
	}
	if _, err := exec.LookPath("launchctl"); err == nil {
		return "launchd"
	}
	return "none"
}

// GetServiceStatus checks if the agent service is running
func GetServiceStatus() (bool, string, error) {
	manager := DetectServiceManager()

	switch manager {
	case "systemd":
		cmd := exec.Command("systemctl", "is-active", "orion-agent")
		output, err := cmd.Output()
		if err != nil {
			return false, "inactive", nil
		}
		status := strings.TrimSpace(string(output))
		return status == "active", status, nil

	case "launchd":
		cmd := exec.Command("launchctl", "list")
		output, err := cmd.Output()
		if err != nil {
			return false, "unknown", nil
		}
		if strings.Contains(string(output), "com.orion.agent") {
			return true, "running", nil
		}
		return false, "stopped", nil

	default:
		// Check if process is running by looking for the binary
		cmd := exec.Command("pgrep", "-f", "orion-agent")
		err := cmd.Run()
		return err == nil, "unknown", nil
	}
}

// StartService starts the agent service
func StartService() error {
	manager := DetectServiceManager()

	switch manager {
	case "systemd":
		cmd := exec.Command("systemctl", "start", "orion-agent")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to start service: %s", string(output))
		}
		logging.Infof("Service started successfully")
		return nil

	case "launchd":
		// Try user LaunchAgent first
		plistPath := filepath.Join(os.Getenv("HOME"), "Library/LaunchAgents/com.orion.agent.plist")
		if _, err := os.Stat(plistPath); err == nil {
			cmd := exec.Command("launchctl", "load", plistPath)
			return cmd.Run()
		}
		// Try system LaunchDaemon
		plistPath = "/Library/LaunchDaemons/com.orion.agent.plist"
		if _, err := os.Stat(plistPath); err == nil {
			cmd := exec.Command("sudo", "launchctl", "load", plistPath)
			return cmd.Run()
		}
		return fmt.Errorf("service file not found")

	default:
		return fmt.Errorf("no service manager detected. Please run the agent manually or install as a service")
	}
}

// StopService stops the agent service
func StopService() error {
	manager := DetectServiceManager()

	switch manager {
	case "systemd":
		cmd := exec.Command("systemctl", "stop", "orion-agent")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to stop service: %s", string(output))
		}
		logging.Infof("Service stopped successfully")
		return nil

	case "launchd":
		// Try user LaunchAgent first
		plistPath := filepath.Join(os.Getenv("HOME"), "Library/LaunchAgents/com.orion.agent.plist")
		if _, err := os.Stat(plistPath); err == nil {
			cmd := exec.Command("launchctl", "unload", plistPath)
			return cmd.Run()
		}
		// Try system LaunchDaemon
		plistPath = "/Library/LaunchDaemons/com.orion.agent.plist"
		if _, err := os.Stat(plistPath); err == nil {
			cmd := exec.Command("sudo", "launchctl", "unload", plistPath)
			return cmd.Run()
		}
		return fmt.Errorf("service file not found")

	default:
		// Try to kill the process
		cmd := exec.Command("pkill", "-f", "orion-agent")
		return cmd.Run()
	}
}

// RestartService restarts the agent service
func RestartService() error {
	if err := StopService(); err != nil {
		logging.Warnf("Error stopping service (may not be running): %v", err)
	}
	return StartService()
}
