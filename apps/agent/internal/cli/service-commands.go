package cli

import (
	"context"
	"fmt"
	"time"

	agent "orion/agent/internal"
	"orion/agent/internal/control"
)

func runStatus(ctx context.Context, opts *Options) error {
	if opts.StatusWatch {
		return runStatusWatch(ctx, opts)
	}
	report := InspectAgentStatus(opts.StatePath)
	daemon, derr := controlClient(opts).Status()
	if derr == nil && report.InternalState == nil {
		// The state file is owned by the orion user; the daemon knows it.
		report.InternalState = statusFromSocket(daemon)
		report.StateCheck = DiagnosticCheck{Name: "state", Status: CheckOK, Detail: "via control socket"}
	}
	if opts.JSON {
		return renderJSON(outputWriter, statusJSON(report, daemon, derr == nil))
	}
	in := statusInput{St: daemon, Now: time.Now(), Width: terminalWidth()}
	if derr != nil {
		in.Stopped = true
		in.St = control.Status{Name: hostnameOrEmpty()}
		in.Why, in.Hint = "the service is stopped", "orion-agent service start"
		if !report.Service.Installed {
			in.Why, in.Hint = "the service is not installed", "orion-agent service install"
		}
	}
	if opts.StatusShort {
		fmt.Fprint(outputWriter, statusShort(in))
	} else {
		fmt.Fprint(outputWriter, statusScreen(theme, in))
	}
	if in.Stopped {
		return &CommandError{Summary: "agent is not running", ExitCode: 1, Silent: true}
	}
	return nil
}

type statusJSONPayload struct {
	ServiceManager    string     `json:"service_manager"`
	ServiceState      string     `json:"service_state"`
	ServiceRunning    bool       `json:"service_running"`
	ServiceFile       string     `json:"service_file,omitempty"`
	ServiceError      string     `json:"service_error,omitempty"`
	StatePath         string     `json:"state_path"`
	StateStatus       string     `json:"state_status"`
	StateDetail       string     `json:"state_detail,omitempty"`
	StateError        string     `json:"state_error,omitempty"`
	Version           string     `json:"version,omitempty"`
	PID               int        `json:"pid,omitempty"`
	StartedAt         *time.Time `json:"started_at,omitempty"`
	UptimeSeconds     int64      `json:"uptime_seconds,omitempty"`
	Registered        bool       `json:"registered"`
	AgentID           string     `json:"agent_id,omitempty"`
	CoreURL           string     `json:"core_url,omitempty"`
	Maintenance       bool       `json:"maintenance"`
	MaintenanceReason string     `json:"maintenance_reason,omitempty"`
	MaintenanceUntil  *time.Time `json:"maintenance_until,omitempty"`
	LastSync          *time.Time `json:"last_sync,omitempty"`
	LastReportAt      *time.Time `json:"last_report_at,omitempty"`
	QueuePending      int        `json:"queue_pending"`
	Monitors          int        `json:"monitors"`
	LogSources        int        `json:"log_sources"`
	Name              string     `json:"name,omitempty"`
	Mode              string     `json:"mode,omitempty"`
}

func statusJSON(report AgentStatusReport, daemon control.Status, daemonOK bool) statusJSONPayload {
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
		if report.InternalState.MaintenanceReason != nil {
			payload.MaintenanceReason = *report.InternalState.MaintenanceReason
		}
		payload.MaintenanceUntil = report.InternalState.MaintenanceUntil
	}
	if daemonOK {
		payload.Version = daemon.Version
		payload.PID = daemon.PID
		payload.StartedAt = timePtrUnlessZero(daemon.StartedAt)
		payload.UptimeSeconds = daemon.UptimeSeconds
		payload.Registered = daemon.Registered
		payload.AgentID = daemon.AgentID
		payload.CoreURL = daemon.CoreURL
		payload.Maintenance = daemon.MaintenanceMode
		payload.MaintenanceReason = daemon.MaintenanceReason
		payload.MaintenanceUntil = daemon.MaintenanceUntil
		payload.LastSync = timePtrUnlessZero(daemon.LastSync)
		payload.LastReportAt = timePtrUnlessZero(daemon.LastReportAt)
		payload.QueuePending = daemon.QueuePending
		payload.Monitors = daemon.Monitors
		payload.LogSources = daemon.LogSources
		payload.Name = daemon.Name
		payload.Mode = daemon.Mode
	}
	return payload
}

func timePtrUnlessZero(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	return &value
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

func serviceStatusName(s ServiceStatus) string {
	switch {
	case s.Running:
		return "up"
	case !s.Installed:
		return "unknown"
	}
	return "down"
}
