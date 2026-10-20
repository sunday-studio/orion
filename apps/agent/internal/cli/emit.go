package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"orion/agent/internal/config"
	"orion/agent/internal/logship"
	"orion/agent/internal/utils"

	"github.com/spf13/cobra"
)

// ingestTarget is where scripts send entries: the configured endpoint (or
// Core + /v1/ingest/logs) and an ingest key from a flag, the environment or a
// configured source. It works without the daemon.
type ingestTarget struct {
	Endpoint string
	Key      string
	App      string
	Source   string
}

func resolveIngestTarget(opts *Options, source, keyFile, endpoint, app string) (ingestTarget, error) {
	t := ingestTarget{Endpoint: endpoint, App: app, Source: source}
	cfg, _, _ := config.Load(opts.ConfigPath)
	if t.Endpoint == "" && cfg != nil {
		t.Endpoint = cfg.Logs.Endpoint
		if t.Endpoint == "" && cfg.CoreURL != "" {
			t.Endpoint = strings.TrimRight(cfg.CoreURL, "/") + "/v1/ingest/logs"
		}
	}
	if t.Endpoint == "" {
		if v, _ := config.Env(config.EnvCoreURL); v != "" {
			t.Endpoint = strings.TrimRight(v, "/") + "/v1/ingest/logs"
		}
	}
	switch {
	case keyFile != "":
		b, err := os.ReadFile(keyFile)
		if err != nil {
			return t, fmt.Errorf("read --key-file: %w", err)
		}
		t.Key = strings.TrimSpace(string(b))
	case os.Getenv("ORION_INGEST_KEY") != "":
		t.Key = strings.TrimSpace(os.Getenv("ORION_INGEST_KEY"))
	case cfg != nil:
		kr := &keyResolver{cfg: cfg.Logs.Sources}
		if source != "" {
			t.Key = kr.Key(source)
		} else if len(cfg.Logs.Sources) > 0 {
			t.Source = cfg.Logs.Sources[0].Name
			t.Key = kr.Key(t.Source)
		}
	}
	if t.App == "" {
		t.App = t.Source
	}
	switch {
	case t.Endpoint == "":
		return t, errors.New("no Core URL: set core_url in the config, ORION_CORE_URL, or pass --endpoint")
	case t.Key == "":
		return t, errors.New("no ingest key: pass --key-file, set ORION_INGEST_KEY, or use a configured logs source (Console, Settings, Apps, create a key)")
	}
	return t, nil
}

func (t ingestTarget) send(ctx context.Context, entries []logship.Entry) error {
	s := &logship.HTTPSender{URL: t.Endpoint, Key: func(string) string { return t.Key }, Client: &http.Client{Timeout: 15 * time.Second}}
	return s.Send(ctx, t.Source, entries)
}

func (t ingestTarget) resource() map[string]any {
	host, _ := os.Hostname()
	res := map[string]any{"host.name": host, "service.instance.id": host}
	if t.App != "" {
		res["service.name"] = t.App
	}
	return res
}

func newEmitCommand(ctx context.Context, opts *Options) *cobra.Command {
	var severity, event, source, keyFile, endpoint, app string
	var attrs []string
	cmd := &cobra.Command{
		Use:   "emit [flags] <message>",
		Short: "Send one log entry from a script or cron job",
		Long: "Sends a single entry to Core's log ingest with a write-only ingest key. Needs no running agent.\n\n" +
			"  orion-agent emit --severity error --event backup.failed --attr host=db1 \"backup failed\"",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sev := normalizeSeverity(severity)
			if sev == "" {
				return NewCommandError("invalid --severity", fmt.Errorf("%q is not one of trace, debug, info, warn, error, fatal", severity))
			}
			at := map[string]any{}
			for _, kv := range attrs {
				k, v, ok := strings.Cut(kv, "=")
				if !ok || k == "" {
					return NewCommandError("invalid --attr", fmt.Errorf("%q must look like key=value", kv))
				}
				at[k] = v
			}
			t, err := resolveIngestTarget(opts, source, keyFile, endpoint, app)
			if err != nil {
				return NewCommandError("cannot send the entry", err)
			}
			e := logship.Entry{ID: utils.NewReportID(), Timestamp: time.Now().UTC().Format(time.RFC3339Nano), Severity: sev,
				Body: strings.Join(args, " "), Event: event, Resource: t.resource()}
			if len(at) > 0 {
				e.Attributes = at
			}
			if err := t.send(ctx, []logship.Entry{e}); err != nil {
				return NewCommandError("could not send the entry", err)
			}
			if opts.JSON {
				return renderJSON(outputWriter, map[string]any{"sent": 1, "id": e.ID})
			}
			fmt.Fprint(outputWriter, credStep(theme, "Sent", "")+"\n")
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&severity, "severity", "info", "trace, debug, info, warn, error or fatal")
	f.StringVar(&event, "event", "", "short event name, e.g. backup.failed")
	f.StringArrayVar(&attrs, "attr", nil, "attribute key=value (repeatable)")
	f.StringVar(&source, "source", "", "configured logs source whose ingest key to use")
	f.StringVar(&keyFile, "key-file", "", "file holding the ingest key (or ORION_INGEST_KEY)")
	f.StringVar(&endpoint, "endpoint", "", "ingest URL (default: Core's /v1/ingest/logs)")
	f.StringVar(&app, "app", "", "service.name to attach")
	return cmd
}

// runJobResult is what run-job reports about the command it ran.
type runJobResult struct {
	ExitCode int
	Duration time.Duration
	Lines    []logship.Entry
}

func newRunJobCommand(ctx context.Context, opts *Options) *cobra.Command {
	var name, source, keyFile, endpoint, hbToken string
	var maxLines int
	cmd := &cobra.Command{
		Use:   "run-job --name <job> [flags] -- <command> [args...]",
		Short: "Run a command, ship its output and exit code, and ping a heartbeat",
		Long: "Runs the command, ships each output line and a final job.finished or job.failed entry, and pings the\n" +
			"heartbeat monitor so a failed or missing run alerts. The exit code is passed through.\n\n" +
			"  0 3 * * *  orion-agent run-job --name nightly-backup --heartbeat-token $T -- /usr/local/bin/backup.sh",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				return NewCommandError("--name is required", errors.New("name the job so its logs and heartbeat can be found"))
			}
			t, terr := resolveIngestTarget(opts, source, keyFile, endpoint, app(name))
			if hbToken == "" {
				hbToken = os.Getenv("ORION_HEARTBEAT_TOKEN")
			}
			res := runCommand(ctx, name, args, maxLines, cmd.OutOrStdout(), cmd.ErrOrStderr(), t.resource())
			warn := func(msg string, err error) { fmt.Fprintf(cmd.ErrOrStderr(), "orion-agent: %s: %v\n", msg, err) }
			shipped := len(res.Lines)
			if terr != nil {
				warn("job output not shipped", terr)
				shipped = -1
			} else if err := t.send(ctx, res.Lines); err != nil {
				warn("job output not shipped", err)
				shipped = -1
			}
			heartbeat := ""
			if hbToken != "" {
				if err := pingHeartbeat(ctx, opts, hbToken, res.ExitCode == 0); err != nil {
					warn("heartbeat not sent", err)
				} else if res.ExitCode == 0 {
					heartbeat = "ok"
				} else {
					heartbeat = "failed"
				}
			}
			fmt.Fprint(cmd.ErrOrStderr(), jobSummary(errorTheme(cmd.ErrOrStderr()), name, res.ExitCode, res.Duration, shipped, heartbeat))
			if res.ExitCode != 0 {
				return &CommandError{Summary: fmt.Sprintf("job %s failed with exit code %d", name, res.ExitCode), ExitCode: res.ExitCode, Silent: true}
			}
			return nil
		},
	}
	f := cmd.Flags()
	f.SetInterspersed(false) // everything after the first non-flag is the command
	f.StringVar(&name, "name", "", "job name (required)")
	f.StringVar(&source, "source", "", "configured logs source whose ingest key to use")
	f.StringVar(&keyFile, "key-file", "", "file holding the ingest key (or ORION_INGEST_KEY)")
	f.StringVar(&endpoint, "endpoint", "", "ingest URL (default: Core's /v1/ingest/logs)")
	f.StringVar(&hbToken, "heartbeat-token", "", "heartbeat monitor token (or ORION_HEARTBEAT_TOKEN)")
	f.IntVar(&maxLines, "max-lines", 2000, "most output lines to ship (the rest are counted)")
	return cmd
}

func app(name string) string { return name }

// runCommand runs argv, mirrors its output to the caller's stdout/stderr, and
// turns every line plus the outcome into entries.
func runCommand(ctx context.Context, job string, argv []string, maxLines int, stdout, stderr io.Writer, resource map[string]any) runJobResult {
	start := time.Now()
	c := exec.CommandContext(ctx, argv[0], argv[1:]...)
	outPipe, _ := c.StdoutPipe()
	errPipe, _ := c.StderrPipe()
	res := runJobResult{}
	res.Lines = nil
	if err := c.Start(); err != nil {
		res.ExitCode = 127
		res.Lines = append(res.Lines, jobEntry(job, "error", "job.failed", fmt.Sprintf("could not start %s: %v", argv[0], err), resource, map[string]any{"exit_code": 127}))
		res.Duration = time.Since(start)
		return res
	}
	type line struct{ text, sev string }
	lines := make(chan line, 256)
	done := make(chan struct{}, 2)
	pump := func(r io.Reader, mirror io.Writer, sev string) {
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 64*1024), logship.MaxLineBytes)
		for sc.Scan() {
			fmt.Fprintln(mirror, sc.Text())
			lines <- line{sc.Text(), sev}
		}
		done <- struct{}{}
	}
	go pump(outPipe, stdout, "info")
	go pump(errPipe, stderr, "warn")
	go func() { <-done; <-done; close(lines) }()
	dropped := 0
	for l := range lines {
		if len(res.Lines) >= maxLines {
			dropped++
			continue
		}
		res.Lines = append(res.Lines, jobEntry(job, l.sev, "job.output", l.text, resource, nil))
	}
	err := c.Wait()
	res.Duration = time.Since(start)
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			res.ExitCode = ee.ExitCode()
			if ws, ok := ee.Sys().(syscall.WaitStatus); ok && ws.Signaled() {
				res.ExitCode = 128 + int(ws.Signal())
			}
		} else {
			res.ExitCode = 1
		}
	}
	attrs := map[string]any{"exit_code": res.ExitCode, "duration_ms": res.Duration.Milliseconds()}
	if dropped > 0 {
		attrs["output_lines_dropped"] = dropped
	}
	event, sev, msg := "job.finished", "info", fmt.Sprintf("job %s finished in %s", job, res.Duration.Round(time.Millisecond))
	if res.ExitCode != 0 {
		event, sev, msg = "job.failed", "error", fmt.Sprintf("job %s failed with exit code %d after %s", job, res.ExitCode, res.Duration.Round(time.Millisecond))
	}
	res.Lines = append(res.Lines, jobEntry(job, sev, event, msg, resource, attrs))
	return res
}

func jobEntry(job, sev, event, body string, resource map[string]any, attrs map[string]any) logship.Entry {
	a := map[string]any{"job": job}
	for k, v := range attrs {
		a[k] = v
	}
	return logship.Entry{ID: utils.NewReportID(), Timestamp: time.Now().UTC().Format(time.RFC3339Nano), Severity: sev, Event: event, Body: body, Attributes: a, Resource: resource}
}

// pingHeartbeat reports the run to a Core heartbeat monitor
// (POST /v1/heartbeats/{token}/success|failure).
func pingHeartbeat(ctx context.Context, opts *Options, token string, ok bool) error {
	cfg, _, _ := config.Load(opts.ConfigPath)
	core := ""
	if cfg != nil {
		core = cfg.CoreURL
	}
	if core == "" {
		core, _ = config.Env(config.EnvCoreURL)
	}
	if core == "" {
		return errors.New("no Core URL to ping")
	}
	kind := "success"
	if !ok {
		kind = "failure"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(core, "/")+"/v1/heartbeats/"+token+"/"+kind, nil)
	if err != nil {
		return err
	}
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("Core answered HTTP %d", resp.StatusCode)
	}
	return nil
}
