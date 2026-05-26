package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type ServiceManager string

const (
	ServiceManagerSystemd ServiceManager = "systemd"
	ServiceManagerLaunchd ServiceManager = "launchd"
	ServiceManagerNone    ServiceManager = "none"
)

type ServiceStatus struct {
	Manager     ServiceManager
	Installed   bool
	Running     bool
	State       string
	Unit        string
	ServiceFile string
	Output      string
	Error       error
}

type ServiceControlResult struct {
	Action  string
	Manager ServiceManager
	OK      bool
	Output  string
	Error   error
}

const (
	systemdUnitName = "orion-agent"
	launchdLabel    = "system/com.orion.agent"
)

// DetectServiceManager detects which service manager is available
func DetectServiceManager() string {
	return string(DetectServiceManagerType())
}

func DetectServiceManagerType() ServiceManager {
	if _, err := exec.LookPath("systemctl"); err == nil {
		return ServiceManagerSystemd
	}
	if _, err := exec.LookPath("launchctl"); err == nil {
		return ServiceManagerLaunchd
	}
	return ServiceManagerNone
}

// GetServiceStatus checks if the agent service is running
func GetServiceStatus() (bool, string, error) {
	status := GetServiceStatusResult()
	return status.Running, status.State, status.Error
}

func GetServiceStatusResult() ServiceStatus {
	manager := DetectServiceManagerType()
	status := ServiceStatus{
		Manager: manager,
		Unit:    systemdUnitName,
		State:   "unknown",
	}

	switch manager {
	case ServiceManagerSystemd:
		status.ServiceFile = "/etc/systemd/system/orion-agent.service"
		if _, err := os.Stat(status.ServiceFile); err == nil {
			status.Installed = true
		}

		cmd := exec.Command("systemctl", "is-active", systemdUnitName)
		output, err := cmd.CombinedOutput()
		status.Output = strings.TrimSpace(string(output))
		if status.Output != "" {
			status.State = status.Output
		}
		if err != nil {
			if strings.Contains(status.Output, "could not be found") || strings.Contains(status.Output, "not-found") {
				status.Installed = false
				status.State = "not-found"
			} else if status.State == "unknown" {
				status.State = "inactive"
			}
			return status
		}
		status.Installed = true
		status.Running = status.State == "active"
		return status

	case ServiceManagerLaunchd:
		status.Unit = launchdLabel
		status.ServiceFile = "/Library/LaunchDaemons/com.orion.agent.plist"
		if _, err := os.Stat(status.ServiceFile); err == nil {
			status.Installed = true
		}
		cmd := exec.Command("launchctl", "print", launchdLabel)
		output, err := cmd.CombinedOutput()
		status.Output = strings.TrimSpace(string(output))
		if err == nil {
			status.Installed = true
			status.Running, status.State = parseLaunchdStatus(status.Output)
			return status
		}
		status.State = "stopped"
		return status

	default:
		// Check if process is running by looking for the binary
		cmd := exec.Command("pgrep", "-f", "orion-agent")
		err := cmd.Run()
		status.Running = err == nil
		if status.Running {
			status.State = "process-running"
		} else {
			status.State = "not-running"
		}
		return status
	}
}

func parseLaunchdStatus(output string) (bool, string) {
	normalized := strings.ToLower(output)
	for _, line := range strings.Split(normalized, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "state =") {
			state := strings.TrimSpace(strings.TrimPrefix(line, "state ="))
			state = strings.Trim(state, "\"")
			switch state {
			case "running":
				return true, "running"
			case "waiting":
				return false, "waiting"
			case "not running":
				return false, "stopped"
			default:
				if state != "" {
					return false, state
				}
			}
		}
	}
	if strings.Contains(normalized, "pid =") {
		return true, "running"
	}
	if strings.Contains(normalized, "spawn scheduled") {
		return false, "waiting"
	}
	return false, "loaded"
}

// StartService starts the agent service
func StartService() error {
	return StartServiceResult().Error
}

func StartServiceResult() ServiceControlResult {
	manager := DetectServiceManagerType()
	result := ServiceControlResult{Action: "start", Manager: manager}

	switch manager {
	case ServiceManagerSystemd:
		if os.Geteuid() != 0 {
			result.Error = serviceRootError("start")
			return result
		}
		cmd := exec.Command("systemctl", "start", systemdUnitName)
		output, err := cmd.CombinedOutput()
		result.Output = strings.TrimSpace(string(output))
		if err != nil {
			result.Error = serviceCommandError("start", string(output))
			return result
		}
		result.OK = true
		return result

	case ServiceManagerLaunchd:
		plistPath := "/Library/LaunchDaemons/com.orion.agent.plist"
		if _, err := os.Stat(plistPath); err == nil {
			cmd := exec.Command("sudo", "launchctl", "bootstrap", "system", plistPath)
			if err := cmd.Run(); err != nil {
				kickstart := exec.Command("sudo", "launchctl", "kickstart", "-k", launchdLabel)
				if err := kickstart.Run(); err != nil {
					result.Error = err
					return result
				}
				result.OK = true
				return result
			}
			result.OK = true
			return result
		}
		result.Error = fmt.Errorf("service file not found")
		return result

	default:
		result.Error = fmt.Errorf("no service manager detected. Please run the agent manually or install as a service")
		return result
	}
}

// StopService stops the agent service
func StopService() error {
	return StopServiceResult().Error
}

func StopServiceResult() ServiceControlResult {
	manager := DetectServiceManagerType()
	result := ServiceControlResult{Action: "stop", Manager: manager}

	switch manager {
	case ServiceManagerSystemd:
		if os.Geteuid() != 0 {
			result.Error = serviceRootError("stop")
			return result
		}
		cmd := exec.Command("systemctl", "stop", systemdUnitName)
		output, err := cmd.CombinedOutput()
		result.Output = strings.TrimSpace(string(output))
		if err != nil {
			result.Error = serviceCommandError("stop", string(output))
			return result
		}
		result.OK = true
		return result

	case ServiceManagerLaunchd:
		plistPath := "/Library/LaunchDaemons/com.orion.agent.plist"
		if _, err := os.Stat(plistPath); err == nil {
			cmd := exec.Command("sudo", "launchctl", "bootout", "system", plistPath)
			if err := cmd.Run(); err != nil {
				result.Error = err
				return result
			}
			result.OK = true
			return result
		}
		result.Error = fmt.Errorf("service file not found")
		return result

	default:
		// Try to kill the process
		cmd := exec.Command("pkill", "-f", "orion-agent")
		if err := cmd.Run(); err != nil {
			result.Error = err
			return result
		}
		result.OK = true
		return result
	}
}

// RestartService restarts the agent service
func RestartService() error {
	return RestartServiceResult().Error
}

func RestartServiceResult() ServiceControlResult {
	stopResult := StopServiceResult()
	startResult := StartServiceResult()
	startResult.Action = "restart"
	if stopResult.Output != "" && startResult.Output != "" {
		startResult.Output = stopResult.Output + "\n" + startResult.Output
	} else if stopResult.Output != "" {
		startResult.Output = stopResult.Output
	}
	return startResult
}

// ResetServiceFailures clears service-manager failure throttles after repeated crashes.
func ResetServiceFailures() error {
	manager := DetectServiceManagerType()

	switch manager {
	case ServiceManagerSystemd:
		if os.Geteuid() != 0 {
			return serviceRootError("reset-failed")
		}
		cmd := exec.Command("systemctl", "reset-failed", systemdUnitName)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return serviceCommandError("reset-failed", string(output))
		}
		return nil
	case ServiceManagerLaunchd:
		return nil
	default:
		return nil
	}
}

// PrintServiceDiagnostics prints the same post-start details an operator would usually check.
func PrintServiceDiagnostics(lines int) {
	status := GetServiceStatusResult()
	PrintInfo("service_manager", status.Manager)
	PrintInfo("service_state", status.State)
	if status.ServiceFile != "" {
		PrintInfo("service_file", status.ServiceFile)
	}

	PrintRecentLogDiagnostics(JSONLRecentLogReader{Path: DefaultAgentLogPath()}, lines)

	switch status.Manager {
	case ServiceManagerSystemd:
		PrintStep("checking service status")
		printCommandOutput("systemctl", "status", systemdUnitName, "--no-pager")
		PrintStep("showing recent service logs")
		printCommandOutput("journalctl", "-u", systemdUnitName, "-n", strconv.Itoa(lines), "--no-pager")
	case ServiceManagerLaunchd:
		PrintStep("checking service status")
		printCommandOutput("launchctl", "print", launchdLabel)
		PrintStep("showing recent service logs")
		printCommandOutput("tail", "-n", strconv.Itoa(lines), "/usr/local/var/log/orion-agent.log")
		printCommandOutput("tail", "-n", strconv.Itoa(lines), "/usr/local/var/log/orion-agent.error.log")
	default:
		PrintSkip("no service diagnostics available without a service manager")
	}
}

func serviceRootError(action string) error {
	if action == "reset-failed" {
		return fmt.Errorf("systemd service control requires root to reset service failure state; rerun this command from an interactive shell so it can prompt for privileges")
	}
	return fmt.Errorf("systemd service control requires root; run orion-agent %s from an interactive shell so it can prompt for privileges", action)
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

func printCommandOutput(name string, args ...string) {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if len(output) > 0 {
		fmt.Fprint(outputWriter, string(output))
		if !strings.HasSuffix(string(output), "\n") {
			fmt.Fprintln(outputWriter)
		}
	}
	if err != nil {
		PrintSkip(fmt.Sprintf("%s %s failed: %v", name, strings.Join(args, " "), err))
	}
}
