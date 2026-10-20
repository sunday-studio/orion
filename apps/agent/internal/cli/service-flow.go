package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/x/term"

	"orion/agent/internal/control"
	"orion/agent/internal/ui"
)

// preflightSteps turns the preflight report into the three checks the board
// shows: config, state database and permissions.
func preflightSteps(r PreflightReport) []opStep {
	steps := []opStep{{Mark: "ok", Name: "Config valid"}, {Mark: "ok", Name: "State database"}, {Mark: "ok", Name: "Permissions"}}
	for _, c := range r.Checks {
		switch c.Name {
		case "config":
			if c.Status == CheckError {
				steps[0] = opStep{Mark: "fail", Name: "Config is not valid", Evidence: []string{checkEvidence(c)}, Fix: "orion-agent config edit"}
			}
		case "state":
			switch c.Status {
			case CheckError:
				steps[1] = opStep{Mark: "fail", Name: "State database is not writable", Evidence: stateEvidence(c), Fix: "sudo chown -R orion " + dirOf(c.Path)}
			case CheckWarn:
				steps[1].Detail = "will be created on start"
			}
		case "service":
			if c.Status == CheckError {
				steps[2] = opStep{Mark: "fail", Name: "Service is not installed", Evidence: []string{checkEvidence(c)}, Fix: "orion-agent service install"}
			}
		case "logs":
			if c.Status == CheckError && steps[2].Mark == "ok" {
				steps[2] = opStep{Mark: "fail", Name: "Log directory is not usable", Evidence: []string{checkEvidence(c)}, Fix: "sudo mkdir -p " + c.Path + " && sudo chown orion " + c.Path}
			}
		}
	}
	return steps
}

func checkEvidence(c DiagnosticCheck) string {
	switch {
	case c.Error != nil:
		return c.Error.Error()
	case c.Detail != "":
		return c.Detail
	}
	return c.Path
}

func stateEvidence(c DiagnosticCheck) []string {
	if owner := fileOwnerName(c.Path); owner != "" && c.Error != nil && errors.Is(c.Error, os.ErrPermission) {
		return []string{fmt.Sprintf("%s is owned by %s", c.Path, owner)}
	}
	return []string{checkEvidence(c)}
}

func dirOf(path string) string {
	if i := strings.LastIndex(path, "/"); i > 0 {
		return path[:i]
	}
	return path
}

// fileOwnerName is the user that owns a file, or "" when it cannot be told.
func fileOwnerName(path string) string {
	info, err := os.Stat(path)
	if err != nil {
		return ""
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return ""
	}
	if u, err := user.LookupId(fmt.Sprint(st.Uid)); err == nil {
		return u.Username
	}
	return fmt.Sprint(st.Uid)
}

// recentServiceLogLines are the last few lines of the agent's own log, shown
// when a start fails.
func recentServiceLogLines(n int) []string {
	entries, err := JSONLRecentLogReader{Path: DefaultAgentLogPath()}.RecentLogs(RecentLogQuery{Lines: n})
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		line := e.Time.Local().Format("15:04:05") + " " + strings.ToLower(e.Level) + " " + e.Component + ": " + e.Message
		if v := e.Fields["error"]; v != "" {
			line += ": " + v
		}
		out = append(out, line)
	}
	return out
}

// waitForDaemon polls the control socket until the agent answers.
func waitForDaemon(opts *Options, within time.Duration) (control.Status, bool) {
	deadline := time.Now().Add(within)
	for {
		if st, err := controlClient(opts).Status(); err == nil {
			return st, true
		}
		if time.Now().After(deadline) {
			return control.Status{}, false
		}
		time.Sleep(300 * time.Millisecond)
	}
}

// waitForReport waits until the agent has sent a report since `since`.
func waitForReport(opts *Options, since time.Time, within time.Duration) bool {
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if st, err := controlClient(opts).Status(); err == nil && st.LastReportAt.After(since) {
			return true
		}
		time.Sleep(500 * time.Millisecond)
	}
	return false
}

func serviceDetail(st control.Status, ok bool) string {
	manager := string(DetectServiceManagerType())
	if ok && st.PID > 0 {
		return fmt.Sprintf("%s · pid %d", manager, st.PID)
	}
	return manager
}

func runningLine(st control.Status, ok bool) string {
	switch {
	case !ok:
		return "started   waiting for the agent to answer"
	case st.LastReportAt.IsZero():
		interval := max(st.ReportIntervalSeconds, 30)
		return fmt.Sprintf("running   first report due in %d s", interval)
	}
	return fmt.Sprintf("running   last report %d s ago", int(time.Since(st.LastReportAt).Seconds()))
}

func serviceFailure(opts *Options, steps []opStep, summary string, err error) error {
	fmt.Fprint(outputWriter, "\n"+opLines(theme, steps))
	if logs := recentServiceLogLines(4); len(logs) > 0 {
		fmt.Fprint(outputWriter, "\n "+theme.Muted("RECENT SERVICE LOG")+"\n")
		for _, l := range logs {
			fmt.Fprintln(outputWriter, "   "+l)
		}
	}
	return NewCommandError(summary, err, "orion-agent logs", "orion-agent doctor")
}

func runStart(_ context.Context, opts *Options) error {
	return startOrRestart(opts, "Start", false)
}

func runRestart(_ context.Context, opts *Options) error {
	return startOrRestart(opts, "Restart", true)
}

func startOrRestart(opts *Options, title string, restart bool) error {
	pre := BuildServicePreflight(opts.ConfigPath, opts.StatePath)
	steps := preflightSteps(pre)
	if pre.HasErrors() {
		fmt.Fprint(outputWriter, preflightFailScreen(theme, steps, recentServiceLogLines(4)))
		return NewCommandError(title+" preflight failed", errors.New("The service was not touched."))
	}
	var svc []opStep
	if err := ResetServiceFailures(); err != nil {
		svc = append(svc, opStep{Mark: "warn", Name: "Failure state not reset", Detail: firstLineOf(err.Error())})
	} else {
		svc = append(svc, opStep{Mark: "ok", Name: "Failure state reset"})
	}
	began := time.Now()
	if restart {
		if err := RestartService(); err != nil {
			return serviceFailure(opts, svc, "Could not restart the service", err)
		}
		out := []opStep{{Mark: "ok", Name: "Stopped"}, {Mark: "ok", Name: "Started"}}
		healthy := opStep{Mark: "ok", Name: "Healthy", Detail: "reporting again"}
		if !waitForReport(opts, began, 30*time.Second) {
			healthy = opStep{Mark: "warn", Name: "Started, not reporting yet", Detail: "check: orion-agent status"}
		}
		healthy.TookMS = time.Since(began).Milliseconds()
		fmt.Fprint(outputWriter, "\n"+opLines(theme, append(out, healthy)))
		return nil
	}
	if err := StartService(); err != nil {
		return serviceFailure(opts, append(steps, svc...), "Could not start the service", err)
	}
	st, ok := waitForDaemon(opts, 8*time.Second)
	svc = append(svc, opStep{Mark: "ok", Name: "Started", Detail: serviceDetail(st, ok), TookMS: time.Since(began).Milliseconds()})
	fmt.Fprint(outputWriter, startScreen(theme, title, steps, svc, runningLine(st, ok), "orion-agent status"))
	return nil
}

func firstLineOf(s string) string { return strings.SplitN(s, "\n", 2)[0] }

func runStop(_ context.Context, opts *Options) error {
	if !opts.SetupYes && !opts.JSON && stdinIsTerminal() {
		host := firstNonEmpty(hostnameOrEmpty(), "this host")
		yes, err := confirmAction(theme, os.Stdin, outputWriter, "Stop the agent on "+host+"?", "Monitoring and log shipping pause until you start it again.", "Stop", "Cancel", "skip the prompt with --yes")
		if err != nil || !yes {
			return NewCommandError("Stop cancelled", errors.New("The agent is still running."))
		}
	}
	if err := StopService(); err != nil {
		return NewCommandError("Could not stop the service", err, "orion-agent logs", "orion-agent doctor")
	}
	fmt.Fprint(outputWriter, "\n"+opLines(theme, []opStep{{Mark: "ok", Name: "Stopped"}}))
	return nil
}

// confirmAction asks a yes/no question on a terminal. Left/right or tab move,
// y and n answer, enter confirms, esc cancels.
func confirmAction(th *ui.Theme, in *os.File, out io.Writer, question, detail, yes, no, hint string) (bool, error) {
	if !term.IsTerminal(in.Fd()) {
		return false, errors.New("stdin is not a terminal")
	}
	state, err := term.MakeRaw(in.Fd())
	if err != nil {
		return false, err
	}
	defer term.Restore(in.Fd(), state)
	focus := true
	lines := 0
	draw := func() {
		if lines > 0 {
			fmt.Fprintf(out, "\x1b[%dA\r\x1b[J", lines)
		}
		screen := strings.ReplaceAll(confirmView(th, question, detail, yes, no, focus, hint), "\n", "\r\n")
		fmt.Fprint(out, screen)
		lines = strings.Count(screen, "\n")
	}
	draw()
	buf := make([]byte, 16)
	for {
		n, err := in.Read(buf)
		if err != nil {
			return false, err
		}
		switch s := string(buf[:n]); {
		case s == "\r" || s == "\n":
			fmt.Fprint(out, "\r\n")
			return focus, nil
		case s == "y" || s == "Y":
			fmt.Fprint(out, "\r\n")
			return true, nil
		case s == "n" || s == "N" || s == "\x03" || s == "\x1b":
			fmt.Fprint(out, "\r\n")
			return false, nil
		case s == "\t" || s == "\x1b[C" || s == "\x1b[D" || s == "h" || s == "l":
			focus = !focus
			draw()
		}
	}
}
