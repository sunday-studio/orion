package cli

import (
	"context"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"time"

	agent "orion/agent/internal"
	"orion/agent/internal/config"
	"orion/agent/internal/doctor"
	agentstate "orion/agent/internal/state"

	"github.com/shirou/gopsutil/v4/disk"
)

// probeToken requests the agent's own monitor list, the cheapest authenticated
// endpoint, and returns the HTTP status.
func probeToken(ctx context.Context, coreURL, agentID, token string) (int, error) {
	if coreURL == "" || agentID == "" || token == "" {
		return 0, fmt.Errorf("agent is not enrolled")
	}
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(coreURL, "/")+"/v1/agents/"+agentID+"/monitors", nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	resp.Body.Close()
	return resp.StatusCode, nil
}

func buildDoctorEnv(opts *Options) *doctor.Env {
	e := &doctor.Env{
		ConfigPath: opts.ConfigPath, StatePath: opts.StatePath, SocketPath: opts.SocketPath,
		Version: agent.Version, GOOS: runtime.GOOS,
		Service: func() (bool, bool, string) {
			s := GetServiceStatusResult()
			return s.Installed, s.Running, s.State
		},
		DiskFreeMB: func(path string) (uint64, error) {
			u, err := disk.Usage(path)
			if err != nil {
				return 0, err
			}
			return u.Free / (1024 * 1024), nil
		},
	}
	e.Config, _, e.ConfigErr = config.Load(opts.ConfigPath)

	client := controlClient(opts)
	if st, err := client.Status(); err == nil {
		e.Daemon = doctor.DaemonInfo{Reachable: true, Version: st.Version, LastSync: st.LastSync}
		e.Registered = st.Registered
		e.TokenProbe = func(context.Context) (int, error) { return client.TokenCheck() }
	} else {
		e.Daemon = doctor.DaemonInfo{Err: err}
	}
	// Prefer a direct read when the state file is readable (root or the orion user).
	if state, err := agentstate.InspectReadOnly(opts.StatePath); err == nil && state != nil {
		e.Registered = state.IsRegistered()
		e.TokenProbe = func(ctx context.Context) (int, error) {
			return probeToken(ctx, state.CoreURL, state.AgentID, state.Token)
		}
	}
	return e
}

func runDoctor(ctx context.Context, opts *Options) error {
	env := buildDoctorEnv(opts)
	checks := doctor.DefaultChecks()
	if opts.JSON {
		rep := doctor.Run(ctx, env, checks, nil)
		if err := renderJSON(outputWriter, doctorReportJSON{Report: rep, ConfigPath: opts.ConfigPath, StatePath: opts.StatePath, Version: agent.Version}); err != nil {
			return err
		}
		if !rep.OK {
			return &CommandError{Summary: "doctor found errors", ExitCode: 1}
		}
		return nil
	}

	PrintHeader("doctor")
	PrintInfo("config", opts.ConfigPath)
	PrintInfo("version", agent.Version)
	fmt.Fprintln(outputWriter)
	rep := doctor.Run(ctx, env, checks, printDoctorResult)
	fmt.Fprintln(outputWriter)
	if !rep.OK {
		return &CommandError{Summary: "doctor found errors", ExitCode: 1}
	}
	PrintOK("doctor checks passed")
	return nil
}

type doctorReportJSON struct {
	doctor.Report
	ConfigPath string `json:"config_path"`
	StatePath  string `json:"state_path"`
	Version    string `json:"version"`
}

// Status is always spelled out in text, never colour alone.
func printDoctorResult(r doctor.Result) {
	label := map[doctor.Status]string{doctor.OK: "ok  ", doctor.Warn: "warn", doctor.Fail: "FAIL", doctor.Skip: "skip"}[r.Status]
	line := fmt.Sprintf("  [%s] %s", label, r.Title)
	if r.Detail != "" {
		line += ": " + r.Detail
	}
	fmt.Fprintln(outputWriter, line)
	if r.Fix != "" && (r.Status == doctor.Fail || r.Status == doctor.Warn) {
		fmt.Fprintf(outputWriter, "         fix: %s\n", r.Fix)
	}
}
