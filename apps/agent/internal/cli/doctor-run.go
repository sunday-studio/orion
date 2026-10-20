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
		Version: agent.Version, GOOS: runtime.GOOS, Hostname: hostnameOrEmpty(),
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
		e.Daemon = doctor.DaemonInfo{Reachable: true, Version: st.Version, LastSync: st.LastSync, Uptime: time.Duration(st.UptimeSeconds) * time.Second, PID: st.PID}
		e.StateOpen = func() error { return nil } // the daemon has it open
		e.Discovery = func() (doctor.DiscoveryInfo, bool) {
			d, err := client.Discovery()
			if err != nil {
				return doctor.DiscoveryInfo{}, false
			}
			info := doctor.DiscoveryInfo{ScannedAt: d.ScannedAt}
			for _, it := range d.Items {
				if !it.Dismissed {
					info.Things++
				}
			}
			for _, k := range d.Kinds {
				if k.State == "blocked" || k.State == "failed" {
					info.Blocked = append(info.Blocked, doctor.BlockedKind{Kind: k.Kind, Message: k.Message, Fix: k.Fix, Why: k.Why})
				}
			}
			return info, true
		}
		e.Registered = st.Registered
		e.Queue = func() (int, bool) { return st.QueuePending, true }
		e.TokenProbe = func(context.Context) (int, error) { return client.TokenCheck() }
	} else {
		e.Daemon = doctor.DaemonInfo{Err: err}
	}
	if e.StateOpen == nil {
		e.StateOpen = func() error { _, err := agentstate.InspectReadOnly(opts.StatePath); return err }
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
			return &CommandError{Summary: "doctor found errors", ExitCode: 1, Silent: true}
		}
		return nil
	}

	host := hostnameOrEmpty()
	live := !theme.Plain() && stdoutIsTerminal() && terminalWidth() > panelWidth+1 // redraw in place only where lines cannot wrap
	var (
		results []doctor.Result
		running *doctor.Check
		drawn   int // lines of the live screen on the terminal
		frame   int
	)
	redraw := func() {
		screen := doctorGrouped(theme, host, results, len(checks), running, frame)
		if drawn > 0 {
			fmt.Fprintf(outputWriter, "\x1b[%dA\r\x1b[J", drawn)
		}
		fmt.Fprint(outputWriter, screen)
		drawn = strings.Count(screen, "\n")
	}
	started := time.Now()
	start := func(c doctor.Check) {
		cc := c
		running, frame = &cc, frame+1
		if live {
			redraw()
		}
	}
	emit := func(r doctor.Result) {
		results = append(results, r)
		running = nil
		if live {
			redraw()
		}
	}
	rep := doctor.RunWithStart(ctx, env, checks, start, emit)
	if live && drawn > 0 {
		fmt.Fprintf(outputWriter, "\x1b[%dA\r\x1b[J", drawn)
	}
	c := countDoctor(rep.Checks)
	switch {
	case c.fail > 0:
		fmt.Fprint(outputWriter, doctorFailing(theme, rep.Checks))
	case c.warn == 0 && !opts.Verbose && !theme.Plain():
		fmt.Fprint(outputWriter, doctorCollapsed(theme, host, c, time.Since(started)))
	default:
		fmt.Fprint(outputWriter, doctorGrouped(theme, host, rep.Checks, len(rep.Checks), nil, 0))
	}
	if c.fail > 0 {
		return &CommandError{Summary: "doctor found errors", ExitCode: 1, Silent: true}
	}
	return nil
}

type doctorReportJSON struct {
	doctor.Report
	ConfigPath string `json:"config_path"`
	StatePath  string `json:"state_path"`
	Version    string `json:"version"`
}
