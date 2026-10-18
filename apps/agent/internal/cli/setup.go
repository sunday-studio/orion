package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"orion/agent/internal/config"
	"orion/agent/internal/discover"
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
}

func stdinIsTerminal() bool { return term.IsTerminal(os.Stdin.Fd()) && term.IsTerminal(os.Stdout.Fd()) }

func runSetup(ctx context.Context, opts *Options) error {
	interactive := !opts.SetupYes && !opts.JSON && stdinIsTerminal()
	var plan setupPlan
	var err error
	if interactive {
		plan, err = runWizard(ctx, opts)
	} else {
		plan, err = planFromFlags(ctx, opts)
	}
	if err != nil {
		return err
	}
	return applySetup(ctx, opts, plan)
}

// discoverFn is replaced in tests with a fixture host.
var discoverFn = func(ctx context.Context) discover.Result {
	return discover.Run(ctx, discover.HostEnv(runtime.GOOS))
}

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
	if len(opts.SetupMonitors) > 0 || opts.SetupAcceptDiscovered {
		found := discoverHost(ctx)
		selectors := opts.SetupMonitors
		if opts.SetupAcceptDiscovered {
			selectors = []string{"high"}
		}
		sel, err := discover.Select(found.Candidates, selectors)
		if err != nil {
			return plan, NewCommandError("could not select monitors", err)
		}
		plan.Selected = sel
	}
	return plan, nil
}

// buildSetupConfig merges the plan into the existing config (or a blank one).
func buildSetupConfig(opts *Options, plan setupPlan) (next config.UserConfig, before []byte, err error) {
	base := config.UserConfig{Interval: config.DefaultInterval, Monitors: []config.UserMonitor{}}
	if data, rerr := os.ReadFile(opts.ConfigPath); rerr == nil {
		before = data
		if !plan.Force {
			existing, _, lerr := config.LoadRaw(opts.ConfigPath)
			if lerr != nil {
				return next, before, fmt.Errorf("the existing config is invalid, fix it or use --force to start over: %w", lerr)
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

func applySetup(ctx context.Context, opts *Options, plan setupPlan) error {
	if plan.RemoveTokenFile {
		defer os.Remove(plan.TokenFile)
	}
	next, before, err := buildSetupConfig(opts, plan)
	if err != nil {
		return NewCommandError("config validation failed", err)
	}
	data, err := yaml.Marshal(next)
	if err != nil {
		return NewCommandError("could not render config", err)
	}
	diff := lineDiff(splitLines(string(before)), splitLines(string(data)))

	if opts.JSON {
		_ = renderJSON(outputWriter, map[string]any{"config": opts.ConfigPath, "changes": diff, "applied": !opts.SetupDryRun, "monitors": len(next.Monitors)})
	} else if !plan.Confirmed {
		PrintHeader("setup")
		PrintInfo("config", opts.ConfigPath)
		if len(diff) == 0 {
			PrintOK("config already up to date")
		}
		for _, l := range diff {
			fmt.Fprintln(outputWriter, "  "+l)
		}
	}
	if opts.SetupDryRun {
		PrintSkip("dry run: nothing was written")
		return nil
	}

	if len(diff) > 0 || len(before) == 0 {
		if err := writeConfigAtomic(opts.ConfigPath, data); err != nil {
			return NewCommandError("could not write config", err, "re-run with sufficient permissions (sudo orion-agent setup)")
		}
		handOverToServiceUser(opts.ConfigPath)
		PrintOK("config written")
	}

	switch {
	case plan.TokenFile != "":
		if err := runEnroll(ctx, &Options{ConfigPath: opts.ConfigPath, StatePath: opts.StatePath, SocketPath: opts.SocketPath, JSON: false}, plan.CoreURL, "", plan.TokenFile); err != nil {
			return err
		}
	case opts.SetupInitState:
		store, err := agentstate.Open(opts.StatePath)
		if err != nil {
			return NewCommandError("could not initialize state database", err)
		}
		_, _ = store.Get()
		_ = store.Close()
		handOverToServiceUser(opts.StatePath)
		PrintOK("state database initialized")
	}

	if err := controlClient(opts).Reload(); err == nil {
		PrintOK("running daemon picked up the new config (no restart)")
	} else {
		PrintInfo("next", "orion-agent start   (or: orion-agent run --once to try it)")
	}
	return nil
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
