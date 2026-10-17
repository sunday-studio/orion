package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	agent "orion/agent/internal"
	"orion/agent/internal/config"
	agentstate "orion/agent/internal/state"

	"gopkg.in/yaml.v3"
)

func runStart(_ context.Context, opts *Options) error {
	PrintHeader("start")
	preflight := BuildServicePreflight(opts.ConfigPath, opts.StatePath)
	PrintPreflightReport(preflight)
	if preflight.HasErrors() {
		PrintServiceDiagnostics(opts.LogLines)
		return NewCommandError("start preflight failed", fmt.Errorf("one or more required checks failed"), "run: orion-agent status", "run: orion-agent logs")
	}
	PrintStep("resetting service failure state")
	if err := ResetServiceFailures(); err != nil {
		PrintSkip(fmt.Sprintf("could not reset service failure state: %v", err))
	} else {
		PrintOK("service failure state reset")
	}
	PrintStep("starting service")
	if err := StartService(); err != nil {
		PrintServiceDiagnostics(opts.LogLines)
		return NewCommandError("could not start Orion Agent service", err, "run: orion-agent logs", "run: orion-agent status")
	}
	PrintOK("agent service started")
	printServiceStatus()
	return nil
}

func runStop(_ context.Context, _ *Options) error {
	manager := DetectServiceManager()
	PrintHeader("stop")
	PrintInfo("service_manager", manager)
	PrintStep("stopping service")
	if err := StopService(); err != nil {
		return NewCommandError("could not stop Orion Agent service", err)
	}
	PrintOK("agent service stopped")
	printServiceStatus()
	return nil
}

func runStatus(_ context.Context, opts *Options) error {
	report := InspectAgentStatus(opts.StatePath)
	if report.InternalState == nil {
		// The state file is owned by the orion user; ask the daemon instead.
		if st, err := controlClient(opts).Status(); err == nil {
			report.InternalState = statusFromSocket(st)
			report.StateCheck = DiagnosticCheck{Name: "state", Status: CheckOK, Detail: "via control socket"}
		}
	}
	if opts.JSON {
		return renderJSON(outputWriter, statusJSON(report))
	}

	PrintHeader("status")
	rows := [][2]string{
		{"service", theme.Status(serviceStatusName(report.Service))},
		{"manager", string(report.Service.Manager)},
	}
	if report.Service.ServiceFile != "" {
		rows = append(rows, [2]string{"service file", report.Service.ServiceFile})
	}
	if report.Service.Error != nil {
		rows = append(rows, [2]string{"service error", report.Service.Error.Error()})
	}
	rows = append(rows, [2]string{"state database", opts.StatePath})
	switch {
	case report.StateCheck.Status != CheckOK:
		detail := report.StateCheck.Detail
		if report.StateCheck.Error != nil {
			detail = report.StateCheck.Error.Error()
		}
		rows = append(rows, [2]string{"state", theme.Status("unknown") + " " + detail})
	case report.InternalState != nil:
		st := report.InternalState
		rows = append(rows, [2]string{"state", report.StateCheck.Detail})
		rows = append(rows, [2]string{"enrolled", fmt.Sprint(st.IsRegistered() || st.Registered)})
		if st.AgentID != "" {
			rows = append(rows, [2]string{"agent id", st.AgentID})
		}
		if st.CoreURL != "" {
			rows = append(rows, [2]string{"core", st.CoreURL})
		}
		maint := theme.Status("up")
		if st.MaintenanceMode {
			maint = theme.Status("maintenance")
			if st.MaintenanceReason != nil {
				maint += " " + *st.MaintenanceReason
			}
		}
		rows = append(rows, [2]string{"reporting", maint})
	}
	fmt.Fprintln(outputWriter, theme.KV(rows))

	if !report.Service.Running {
		PrintSkip("service is not running")
		return &CommandError{Summary: "service is not running", ExitCode: 1}
	}
	PrintOK("service is running")
	return nil
}

func runSetup(_ context.Context, opts *Options) error {
	PrintHeader("setup")
	PrintInfo("config", opts.ConfigPath)
	PrintInfo("state", opts.StatePath)

	coreURL := strings.TrimSpace(opts.SetupCoreURL)
	if coreURL == "" {
		return NewCommandError("could not create config", fmt.Errorf("--core-url is required"))
	}

	if _, err := os.Stat(opts.ConfigPath); err == nil && !opts.SetupForce {
		return NewCommandError("config already exists", fmt.Errorf("%s exists", opts.ConfigPath), "rerun with --force to replace it", "run: orion-agent config show")
	} else if err != nil && !os.IsNotExist(err) {
		return NewCommandError("could not inspect config path", err)
	}

	userConfig := config.UserConfig{
		CoreURL:     coreURL,
		Interval:    "60s",
		GeoLocation: false,
		Monitors:    []config.UserMonitor{},
	}
	userConfig.ApplyDefaults()
	if err := userConfig.Validate(); err != nil {
		return NewCommandError("config validation failed", err)
	}

	PrintStep("writing config")
	if err := os.MkdirAll(filepath.Dir(opts.ConfigPath), 0o750); err != nil {
		return NewCommandError("could not create config directory", err)
	}
	data, err := yaml.Marshal(userConfig)
	if err != nil {
		return NewCommandError("could not render config", err)
	}
	if err := os.WriteFile(opts.ConfigPath, data, 0o640); err != nil {
		return NewCommandError("could not write config", err)
	}
	PrintOK("config written")
	PrintInfo("core_url", coreURL)

	if opts.SetupInitState {
		PrintStep("initializing state database")
		stateStore, err := agentstate.Open(opts.StatePath)
		if err != nil {
			return NewCommandError("could not initialize state database", err)
		}
		if _, err := stateStore.Get(); err != nil {
			_ = stateStore.Close()
			return NewCommandError("could not initialize state database", err)
		}
		if err := stateStore.Close(); err != nil {
			return NewCommandError("could not close state database", err)
		}
		PrintOK("state database initialized")
	} else {
		PrintSkip("state database not initialized; run orion-agent state init when ready")
	}

	PrintOK("setup complete")
	PrintInfo("next", "orion-agent config show")
	PrintInfo("next", "orion-agent run --once")
	return nil
}

type statusJSONPayload struct {
	ServiceManager string `json:"service_manager"`
	ServiceState   string `json:"service_state"`
	ServiceRunning bool   `json:"service_running"`
	ServiceFile    string `json:"service_file,omitempty"`
	ServiceError   string `json:"service_error,omitempty"`
	StatePath      string `json:"state_path"`
	StateStatus    string `json:"state_status"`
	StateDetail    string `json:"state_detail,omitempty"`
	StateError     string `json:"state_error,omitempty"`
	Registered     bool   `json:"registered"`
	AgentID        string `json:"agent_id,omitempty"`
	CoreURL        string `json:"core_url,omitempty"`
	Maintenance    bool   `json:"maintenance"`
}

func statusJSON(report AgentStatusReport) statusJSONPayload {
	payload := statusJSONPayload{
		ServiceManager: string(report.Service.Manager),
		ServiceState:   report.Service.State,
		ServiceRunning: report.Service.Running,
		ServiceFile:    report.Service.ServiceFile,
		StatePath:      report.StatePath,
		StateStatus:    string(report.StateCheck.Status),
		StateDetail:    report.StateCheck.Detail,
	}
	if report.Service.Error != nil {
		payload.ServiceError = report.Service.Error.Error()
	}
	if report.StateCheck.Error != nil {
		payload.StateError = report.StateCheck.Error.Error()
	}
	if report.InternalState != nil {
		payload.Registered = report.InternalState.IsRegistered()
		payload.AgentID = report.InternalState.AgentID
		payload.CoreURL = report.InternalState.CoreURL
		payload.Maintenance = report.InternalState.MaintenanceMode
	}
	return payload
}

type doctorJSONPayload struct {
	ConfigPath string             `json:"config_path"`
	StatePath  string             `json:"state_path"`
	LogPath    string             `json:"log_path"`
	Service    serviceJSONPayload `json:"service"`
	Checks     []checkJSONPayload `json:"checks"`
	HasErrors  bool               `json:"has_errors"`
}

type serviceJSONPayload struct {
	Manager string `json:"manager"`
	State   string `json:"state"`
	Running bool   `json:"running"`
	File    string `json:"file,omitempty"`
	Error   string `json:"error,omitempty"`
}

type checkJSONPayload struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Path   string `json:"path,omitempty"`
	Detail string `json:"detail,omitempty"`
	Error  string `json:"error,omitempty"`
}

func doctorJSON(report DoctorReport) doctorJSONPayload {
	checks := make([]checkJSONPayload, 0, len(report.Checks))
	for _, check := range report.Checks {
		item := checkJSONPayload{
			Name:   check.Name,
			Status: string(check.Status),
			Path:   check.Path,
			Detail: check.Detail,
		}
		if check.Error != nil {
			item.Error = check.Error.Error()
		}
		checks = append(checks, item)
	}
	var serviceError string
	if report.Service.Error != nil {
		serviceError = report.Service.Error.Error()
	}
	return doctorJSONPayload{
		ConfigPath: report.ConfigPath,
		StatePath:  report.StatePath,
		LogPath:    report.LogPath,
		Service: serviceJSONPayload{
			Manager: string(report.Service.Manager),
			State:   report.Service.State,
			Running: report.Service.Running,
			File:    report.Service.ServiceFile,
			Error:   serviceError,
		},
		Checks:    checks,
		HasErrors: report.HasErrors(),
	}
}

func runRestart(_ context.Context, opts *Options) error {
	PrintHeader("restart")
	preflight := BuildServicePreflight(opts.ConfigPath, opts.StatePath)
	PrintPreflightReport(preflight)
	if preflight.HasErrors() {
		PrintServiceDiagnostics(opts.LogLines)
		return NewCommandError("restart preflight failed", fmt.Errorf("one or more required checks failed"), "run: orion-agent status", "run: orion-agent logs")
	}
	PrintStep("resetting service failure state")
	if err := ResetServiceFailures(); err != nil {
		PrintSkip(fmt.Sprintf("could not reset service failure state: %v", err))
	} else {
		PrintOK("service failure state reset")
	}
	PrintStep("restarting service")
	if err := RestartService(); err != nil {
		PrintServiceDiagnostics(opts.LogLines)
		return NewCommandError("could not restart Orion Agent service", err, "run: orion-agent logs", "run: orion-agent status")
	}
	PrintOK("agent service restarted")
	PrintServiceDiagnostics(opts.LogLines)
	printServiceStatus()
	return nil
}

func runUpdate(_ context.Context, opts *Options) error {
	if err := UpdateAgent(UpdateOptions{
		Repo:           opts.UpdateRepo,
		Version:        opts.UpdateVersion,
		CurrentVersion: agent.Version,
		LogLines:       opts.LogLines,
		BaseURL:        opts.UpdateBaseURL,
		Rollback:       opts.UpdateRollback,
		Force:          opts.UpdateForce,
		SocketPath:     opts.SocketPath,
	}); err != nil {
		return NewCommandError("could not update Orion Agent", err)
	}
	return nil
}

func printServiceStatus() {
	status := GetServiceStatusResult()
	PrintInfo("service_state", status.State)
	if status.Error != nil {
		PrintError(fmt.Sprintf("could not read service state: %v", status.Error))
	}
	if status.Running {
		PrintOK("service is running")
	} else {
		PrintSkip("service is not running")
	}
}

func serviceStatusName(s ServiceStatus) string {
	switch {
	case s.Running:
		return "up"
	case !s.Installed:
		return "unknown"
	}
	return "down"
}
