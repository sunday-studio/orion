package cli

import (
	"fmt"
	"os"
	"os/exec"
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
		cmd := exec.Command("launchctl", "print", "system/com.orion.agent")
		if err := cmd.Run(); err == nil {
			return true, "loaded", nil
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
		if os.Geteuid() != 0 {
			return serviceRootError("start")
		}
		cmd := exec.Command("systemctl", "start", "orion-agent")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return serviceCommandError("start", string(output))
		}
		logging.Infof("Service started successfully")
		return nil

	case "launchd":
		plistPath := "/Library/LaunchDaemons/com.orion.agent.plist"
		if _, err := os.Stat(plistPath); err == nil {
			cmd := exec.Command("sudo", "launchctl", "bootstrap", "system", plistPath)
			if err := cmd.Run(); err != nil {
				kickstart := exec.Command("sudo", "launchctl", "kickstart", "-k", "system/com.orion.agent")
				return kickstart.Run()
			}
			return nil
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
		if os.Geteuid() != 0 {
			return serviceRootError("stop")
		}
		cmd := exec.Command("systemctl", "stop", "orion-agent")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return serviceCommandError("stop", string(output))
		}
		logging.Infof("Service stopped successfully")
		return nil

	case "launchd":
		plistPath := "/Library/LaunchDaemons/com.orion.agent.plist"
		if _, err := os.Stat(plistPath); err == nil {
			cmd := exec.Command("sudo", "launchctl", "bootout", "system", plistPath)
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

func serviceRootError(action string) error {
	return fmt.Errorf("systemd service control requires root; rerun with sudo: sudo orion-agent %s", action)
}

func serviceCommandError(action string, output string) error {
	message := strings.TrimSpace(output)
	if strings.Contains(message, "Unit orion-agent.service not found") {
		return fmt.Errorf("orion-agent systemd service is not installed; run the Agent installer to create /etc/systemd/system/orion-agent.service, or use orion-agent run -once for a one-shot check")
	}
	if message == "" {
		return fmt.Errorf("failed to %s service", action)
	}
	return fmt.Errorf("failed to %s service: %s", action, message)
}
