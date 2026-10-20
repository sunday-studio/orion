package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"orion/agent/internal/config"
	"orion/agent/internal/control"
	"orion/agent/internal/discover"
	"orion/agent/internal/logging"
	agentstate "orion/agent/internal/state"

	"github.com/charmbracelet/x/term"
	"gopkg.in/yaml.v3"
)

// setupPlan is everything setup decided, whether it came from flags or the wizard.
type setupPlan struct {
	CoreURL   string
	TokenFile string // empty: do not enroll now
	// RemoveTokenFile only applies to a temporary file created from --token or
	// the interactive password field. A caller-owned --token-file is preserved.
	RemoveTokenFile bool
	Selected        []discover.Candidate
	Force           bool
	Confirmed       bool // the wizard already showed the diff
	// Found and ScanTook describe the scan that produced Selected, for the summary.
	Found    *discover.Result
	ScanTook time.Duration
	// Label names the kind of selection in the summary ("high-confidence").
	Label string
}

func stdinIsTerminal() bool { return term.IsTerminal(os.Stdin.Fd()) && term.IsTerminal(os.Stdout.Fd()) }

func runSetup(ctx context.Context, opts *Options) error {
	discoverConfigPath = opts.ConfigPath
	if !opts.Verbose {
		logging.SetLevel(logging.LevelError)
		defer logging.SetLevel(logging.LevelInfo)
	}
	interactive := !opts.SetupYes && !opts.JSON && stdinIsTerminal()
	if interactive {
		return runFlow(ctx, opts)
	}
	plan, err := planFromFlags(ctx, opts)
	if err != nil {
		if plan.RemoveTokenFile {
			_ = os.Remove(plan.TokenFile)
		}
		return err
	}
	return applySetup(ctx, opts, plan)
}

// discoverFn is replaced in tests with a fixture host.
var discoverFn = func(ctx context.Context) discover.Result {
	cfg, _, err := config.LoadRaw(discoverConfigPath)
	if err != nil || cfg == nil {
		cfg = &config.UserConfig{}
	}
	return discover.Run(ctx, hostDiscoverEnv(cfg))
}

// discoverConfigPath is the config whose discovery.* settings setup honours.
var discoverConfigPath string

func discoverHost(ctx context.Context) discover.Result { return discoverFn(ctx) }

// planFromFlags builds the plan for automation (cloud-init, Ansible, Docker).
func planFromFlags(ctx context.Context, opts *Options) (setupPlan, error) {
	plan := setupPlan{CoreURL: strings.TrimSpace(opts.SetupCoreURL), TokenFile: opts.SetupTokenFile, Force: opts.SetupForce}
	if opts.SetupToken != "" {
		path, err := stageEnrollmentToken(opts.SetupToken)
		if err != nil {
			return plan, err
		}
		plan.TokenFile, plan.RemoveTokenFile = path, true
	}
	if plan.CoreURL == "" {
		plan.CoreURL, _ = config.Env(config.EnvCoreURL)
	}
	if plan.CoreURL == "" {
		if existing, _, err := config.Load(opts.ConfigPath); err == nil {
			plan.CoreURL = existing.CoreURL
		}
	}
	if plan.CoreURL == "" {
		return plan, NewCommandError("could not create config", errors.New("--core-url is required"), "pass --core-url https://your-core, or set "+config.EnvCoreURL)
	}
	if !opts.SetupDryRun {
		connection, err := preflightSetupPlan(ctx, opts, plan)
		if err != nil {
			return plan, err
		}
		if connection != nil {
			PrintOK(setupConnectionSummary(*connection))
		}
	}
	if len(opts.SetupMonitors) > 0 || opts.SetupAcceptDiscovered {
		scanStart := time.Now()
		found := discoverHost(ctx)
		plan.Found, plan.ScanTook = &found, time.Since(scanStart)
		selectors := opts.SetupMonitors
		if opts.SetupAcceptDiscovered {
			selectors = []string{"high"}
			plan.Label = "high-confidence"
		}
		sel, err := discover.Select(found.Candidates, selectors)
		if err != nil {
			return plan, NewCommandError("could not select monitors", err)
		}
		plan.Selected = sel
	}
	return plan, nil
}

func preflightSetupPlan(ctx context.Context, opts *Options, plan setupPlan) (*setupConnection, error) {
	if plan.TokenFile != "" {
		data, err := os.ReadFile(plan.TokenFile)
		if err != nil {
			return nil, NewCommandError("could not read the Add server token", err)
		}
		result, err := checkSetupConnection(ctx, plan.CoreURL, strings.TrimSpace(string(data)))
		return &result, err
	}
	if current := currentSetupState(opts.StatePath); current != nil {
		result, err := checkCurrentSetupConnection(ctx, current, plan.CoreURL)
		return &result, err
	}
	return nil, nil
}

// buildSetupConfig merges the plan into the existing config (or a blank one).
func buildSetupConfig(opts *Options, plan setupPlan) (next config.UserConfig, before []byte, err error) {
	base := config.UserConfig{Interval: config.DefaultInterval, Monitors: []config.UserMonitor{}}
	if data, rerr := os.ReadFile(opts.ConfigPath); rerr == nil {
		before = data
		if !plan.Force {
			existing, _, lerr := config.LoadRaw(opts.ConfigPath)
			if lerr != nil {
				// A file that only holds discovery settings has no core_url yet;
				// setup is what adds it, so read it without that rule.
				var partial config.UserConfig
				if yerr := yaml.Unmarshal(before, &partial); yerr != nil || partial.CoreURL != "" {
					return next, before, fmt.Errorf("the existing config is invalid, fix it or use --force to start over: %w", lerr)
				}
				if partial.Interval == "" {
					partial.Interval = config.DefaultInterval
				}
				existing = &partial
			}
			base = *existing
		}
	}
	base.CoreURL = plan.CoreURL
	base = discover.BuildConfig(base, plan.Selected)
	checked := base // defaults are applied for validation only, never written
	checked.ApplyDefaults()
	if err := checked.Validate(); err != nil {
		return base, before, err
	}
	return base, before, nil
}

// commitResult is what committing a setup plan did, for either front end.
type commitResult struct {
	ConfigPath string
	Diff       []string
	Monitors   int
	Written    bool
	Enrolled   bool
	StateInit  bool
	Reloaded   bool
}

// commitSetup validates the plan, writes the config, enrolls and reloads the
// daemon. It prints nothing; the caller decides how to show the result.
func commitSetup(ctx context.Context, opts *Options, plan setupPlan) (commitResult, error) {
	res := commitResult{ConfigPath: opts.ConfigPath}
	next, before, err := buildSetupConfig(opts, plan)
	if err != nil {
		return res, NewCommandError("config validation failed", err)
	}
	data, err := yaml.Marshal(next)
	if err != nil {
		return res, NewCommandError("could not render config", err)
	}
	res.Diff = lineDiff(splitLines(string(before)), splitLines(string(data)))
	res.Monitors = len(next.Monitors)
	if opts.SetupDryRun {
		return res, nil
	}
	if len(res.Diff) > 0 || len(before) == 0 {
		if err := writeConfigAtomic(opts.ConfigPath, data); err != nil {
			return res, NewCommandError("could not write config", err, "re-run with sufficient permissions (sudo orion-agent setup)")
		}
		handOverToServiceUser(opts.ConfigPath)
		res.Written = true
	}
	switch {
	case plan.TokenFile != "":
		if err := runEnroll(ctx, &Options{ConfigPath: opts.ConfigPath, StatePath: opts.StatePath, SocketPath: opts.SocketPath, Quiet: true}, plan.CoreURL, "", plan.TokenFile); err != nil {
			return res, err
		}
		res.Enrolled = true
	case opts.SetupInitState:
		store, err := agentstate.Open(opts.StatePath)
		if err != nil {
			return res, NewCommandError("could not initialize state database", err)
		}
		_, _ = store.Get()
		_ = store.Close()
		handOverToServiceUser(opts.StatePath)
		res.StateInit = true
	}
	res.Reloaded = controlClient(opts).Reload() == nil
	return res, nil
}

// applySetup is the non-interactive finish: it commits and prints one line per step.
func applySetup(ctx context.Context, opts *Options, plan setupPlan) error {
	if plan.RemoveTokenFile {
		defer os.Remove(plan.TokenFile)
	}
	res, err := commitSetup(ctx, opts, plan)
	if err != nil {
		return err
	}
	if opts.JSON {
		return renderJSON(outputWriter, map[string]any{"config": opts.ConfigPath, "changes": res.Diff, "applied": !opts.SetupDryRun, "monitors": res.Monitors})
	}
	steps := plan.summarySteps()
	if opts.SetupDryRun {
		fmt.Fprint(outputWriter, "\n")
		for _, l := range res.Diff {
			fmt.Fprintln(outputWriter, "  "+l)
		}
		if len(res.Diff) == 0 {
			fmt.Fprintln(outputWriter, "  "+theme.Muted("no changes to the config"))
		}
		fmt.Fprint(outputWriter, setupSummary(theme, steps, true))
		return nil
	}
	if res.Written {
		steps = append(steps, setupStep{Mark: "ok", Name: "Config written", Detail: opts.ConfigPath})
	} else {
		steps = append(steps, setupStep{Mark: "ok", Name: "Config", Detail: "already up to date"})
	}
	if res.Enrolled {
		steps = append(steps, setupStep{Mark: "ok", Name: "Enrolled", Detail: "host enrolled"})
	}
	if res.StateInit {
		steps = append(steps, setupStep{Mark: "ok", Name: "State", Detail: "database initialized"})
	}
	if res.Reloaded {
		steps = append(steps, setupStep{Mark: "ok", Name: "Applied", Detail: "no restart"})
	} else {
		steps = append(steps, setupStep{Mark: "ok", Name: "Next", Detail: "orion-agent run --once"})
	}
	fmt.Fprint(outputWriter, setupSummary(theme, steps, false))
	return nil
}

// summarySteps are the scan and selection lines of the non-interactive output.
func (p setupPlan) summarySteps() []setupStep {
	if p.Found == nil {
		return nil
	}
	steps := []setupStep{{Mark: "ok", Name: "Scanned", Detail: fmt.Sprintf("found %d %s", len(p.Found.Candidates), plural(len(p.Found.Candidates), "thing", "things")), Took: p.ScanTook}}
	monitored := 0
	for _, c := range p.Found.Candidates {
		if c.Status == discover.StatusMonitored {
			monitored++
		}
	}
	picked := 0
	var logs []string
	for _, c := range p.Selected {
		if c.Monitor != nil || c.StoragePath != "" {
			picked++
		}
	}
	for _, c := range p.Found.Candidates {
		if c.Kind == discover.KindLog && c.Status != discover.StatusMonitored {
			logs = append(logs, displayName(control.DiscoveryItem{Kind: "log", Name: c.Name}))
		}
	}
	label := p.Label
	if label == "" {
		label = "selected"
	}
	detail := fmt.Sprintf("%d %s", picked, label)
	if monitored > 0 {
		detail += fmt.Sprintf(" · %d already monitored", monitored)
	}
	steps = append(steps, setupStep{Mark: "ok", Name: "Accepted", Detail: detail})
	for _, l := range logs {
		steps = append(steps, setupStep{Mark: "warn", Name: "Skipped", Detail: l + " needs a log key"})
	}
	return steps
}

func splitLines(s string) []string {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// writeConfigAtomic writes next to the target and renames, so a crash never
// leaves a half-written config.
func writeConfigAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config-*.yaml")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o640); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
