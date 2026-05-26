package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"orion/agent/internal/config"
	agentstate "orion/agent/internal/state"
)

type CheckStatus string

const (
	CheckOK    CheckStatus = "ok"
	CheckWarn  CheckStatus = "warn"
	CheckError CheckStatus = "error"
	CheckSkip  CheckStatus = "skip"
)

type DiagnosticCheck struct {
	Name   string
	Status CheckStatus
	Path   string
	Detail string
	Error  error
}

type PreflightReport struct {
	ConfigPath string
	StatePath  string
	LogPath    string
	Service    ServiceStatus
	Checks     []DiagnosticCheck
}

type AgentStatusReport struct {
	StatePath     string
	Service       ServiceStatus
	StateCheck    DiagnosticCheck
	InternalState *config.InternalState
}

type RecentLogQuery struct {
	Lines int
}

type LogEntry struct {
	Time      time.Time
	Level     string
	Component string
	Message   string
	Fields    map[string]string
}

type RecentLogReader interface {
	RecentLogs(query RecentLogQuery) ([]LogEntry, error)
}

type NoopRecentLogReader struct{}

func (NoopRecentLogReader) RecentLogs(RecentLogQuery) ([]LogEntry, error) {
	return nil, nil
}

type JSONLRecentLogReader struct {
	Path string
}

func (r JSONLRecentLogReader) RecentLogs(query RecentLogQuery) ([]LogEntry, error) {
	path := r.Path
	if path == "" {
		path = DefaultAgentLogPath()
	}
	entries, _, err := readJSONLLogFile(path, logFilters{limit: query.Lines})
	if err != nil {
		return nil, err
	}

	recent := make([]LogEntry, 0, len(entries))
	for _, entry := range entries {
		var loggedAt time.Time
		if entry.when != nil {
			loggedAt = *entry.when
		}
		level, _ := stringLogValue(entry.values, "level")
		component, _ := stringLogValue(entry.values, "component")
		message, _ := firstStringLogValue(entry.values, "message", "msg")
		recent = append(recent, LogEntry{
			Time:      loggedAt,
			Level:     strings.ToUpper(level),
			Component: component,
			Message:   message,
			Fields:    recentLogFields(entry.values),
		})
	}
	return recent, nil
}

func BuildServicePreflight(configPath string, statePath string) PreflightReport {
	report := PreflightReport{
		ConfigPath: configPath,
		StatePath:  statePath,
		LogPath:    DefaultAgentLogPath(),
		Service:    GetServiceStatusResult(),
	}

	report.Checks = append(report.Checks, checkServiceInstall(report.Service))
	report.Checks = append(report.Checks, checkConfig(configPath))
	report.Checks = append(report.Checks, checkStateFile(statePath))
	report.Checks = append(report.Checks, checkLogDirectory(report.LogPath))

	return report
}

func (r PreflightReport) HasErrors() bool {
	for _, check := range r.Checks {
		if check.Status == CheckError {
			return true
		}
	}
	return false
}

func InspectAgentStatus(statePath string) AgentStatusReport {
	internalState, stateCheck := inspectStateReadOnly(statePath)
	return AgentStatusReport{
		StatePath:     statePath,
		Service:       GetServiceStatusResult(),
		StateCheck:    stateCheck,
		InternalState: internalState,
	}
}

func PrintPreflightReport(report PreflightReport) {
	PrintStep("running preflight checks")
	PrintInfo("service_manager", report.Service.Manager)
	for _, check := range report.Checks {
		printDiagnosticCheck(check)
	}
}

func PrintRecentLogDiagnostics(reader RecentLogReader, lines int) {
	if lines <= 0 {
		lines = 80
	}
	entries, err := reader.RecentLogs(RecentLogQuery{Lines: lines})
	if err != nil {
		PrintSkip(fmt.Sprintf("could not read Orion file logs yet: %v", err))
		return
	}
	if len(entries) == 0 {
		PrintSkip("Orion file log reader is not configured yet; falling back to service logs")
		return
	}

	PrintStep("showing recent Orion logs")
	for _, entry := range entries {
		fmt.Fprintf(outputWriter, "  %s %-5s %s %s", entry.Time.Format("2006-01-02 15:04:05"), entry.Level, entry.Component, entry.Message)
		if len(entry.Fields) > 0 {
			for key, value := range entry.Fields {
				fmt.Fprintf(outputWriter, " %s=%q", key, value)
			}
		}
		fmt.Fprintln(outputWriter)
	}
}

func recentLogFields(values map[string]any) map[string]string {
	fields := map[string]string{}
	for _, key := range []string{"monitor", "monitor_name", "monitor_id", "error", "error_kind", "status_code"} {
		if value, ok := stringLogValue(values, key); ok && value != "" {
			fields[key] = value
		}
	}
	return fields
}

func printDiagnosticCheck(check DiagnosticCheck) {
	message := check.Name
	if check.Path != "" {
		message = fmt.Sprintf("%s (%s)", message, check.Path)
	}
	if check.Detail != "" {
		message = fmt.Sprintf("%s: %s", message, check.Detail)
	}
	if check.Error != nil {
		message = fmt.Sprintf("%s: %v", message, check.Error)
	}

	switch check.Status {
	case CheckOK:
		PrintOK(message)
	case CheckWarn:
		PrintSkip(message)
	case CheckError:
		PrintError(message)
	default:
		PrintSkip(message)
	}
}

func checkServiceInstall(status ServiceStatus) DiagnosticCheck {
	check := DiagnosticCheck{Name: "service"}
	if status.Manager == ServiceManagerNone {
		check.Status = CheckWarn
		check.Detail = "no service manager detected"
		return check
	}
	if !status.Installed {
		check.Status = CheckError
		check.Path = status.ServiceFile
		check.Detail = "service is not installed"
		return check
	}
	check.Status = CheckOK
	check.Path = status.ServiceFile
	check.Detail = fmt.Sprintf("installed, state=%s", status.State)
	return check
}

func checkConfig(path string) DiagnosticCheck {
	check := DiagnosticCheck{Name: "config", Path: path}
	if path == "" {
		check.Status = CheckError
		check.Detail = "config path is empty"
		return check
	}
	if _, err := os.Stat(path); err != nil {
		check.Status = CheckError
		check.Error = err
		return check
	}
	userConfig, err := config.LoadUserConfig(path)
	if err != nil {
		check.Status = CheckError
		check.Error = err
		return check
	}
	check.Status = CheckOK
	check.Detail = fmt.Sprintf("valid with %d monitor(s)", len(userConfig.Monitors))
	return check
}

func checkStateFile(path string) DiagnosticCheck {
	check := DiagnosticCheck{Name: "state", Path: path}
	if path == "" {
		check.Status = CheckError
		check.Detail = "state path is empty"
		return check
	}
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			check.Status = CheckWarn
			check.Detail = "state database does not exist yet"
			return check
		}
		check.Status = CheckError
		check.Error = err
		return check
	}
	if info.IsDir() {
		check.Status = CheckError
		check.Detail = "state path is a directory"
		return check
	}
	file, err := os.Open(path)
	if err != nil {
		check.Status = CheckWarn
		check.Error = err
		return check
	}
	_ = file.Close()
	check.Status = CheckOK
	check.Detail = "readable"
	return check
}

func checkLogDirectory(logPath string) DiagnosticCheck {
	dir := filepath.Dir(logPath)
	check := DiagnosticCheck{Name: "logs", Path: dir}
	info, err := os.Stat(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			check.Status = CheckWarn
			check.Detail = "log directory does not exist yet"
			return check
		}
		check.Status = CheckWarn
		check.Error = err
		return check
	}
	if !info.IsDir() {
		check.Status = CheckError
		check.Detail = "log path parent is not a directory"
		return check
	}
	check.Status = CheckOK
	check.Detail = "directory exists"
	return check
}

func inspectStateReadOnly(path string) (*config.InternalState, DiagnosticCheck) {
	check := DiagnosticCheck{Name: "state", Path: path}
	if path == "" {
		check.Status = CheckError
		check.Detail = "state path is empty"
		return nil, check
	}
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			check.Status = CheckWarn
			check.Detail = "state database does not exist yet"
			return nil, check
		}
		check.Status = CheckWarn
		check.Error = err
		return nil, check
	}

	internalState, err := agentstate.InspectReadOnly(path)
	if err != nil {
		check.Status = CheckWarn
		check.Error = err
		return nil, check
	}
	if internalState == nil || !internalState.IsRegistered() {
		check.Status = CheckWarn
		check.Detail = "state database has no agent identity yet"
		return internalState, check
	}
	check.Status = CheckOK
	check.Detail = "readable"
	return internalState, check
}
