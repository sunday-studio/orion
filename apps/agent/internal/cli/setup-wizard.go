package cli

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"orion/agent/internal/config"
	"orion/agent/internal/discover"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/huh/spinner"
	"gopkg.in/yaml.v3"
)

// runWizard is the interactive setup: Core URL and token, discovery, a
// checklist of what to monitor, then a confirmation after the diff is shown.
func runWizard(ctx context.Context, opts *Options) (setupPlan, error) {
	plan := setupPlan{Force: opts.SetupForce}
	accessible := os.Getenv("ORION_ACCESSIBLE") != ""

	coreURL := firstNonEmpty(opts.SetupCoreURL, envOrEmpty(config.EnvCoreURL))
	if coreURL == "" {
		if existing, _, err := config.Load(opts.ConfigPath); err == nil {
			coreURL = existing.CoreURL
		}
	}
	token := ""
	setSetupTerminalTitle(1)
	form := huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Core URL").Description("Where is your Orion Core?").Prompt(" ").
			Value(&coreURL).Validate(validateCoreURL),
		huh.NewInput().Title("Enrollment token").Description("From Console, Servers, Add server").Prompt(" ").
			EchoMode(huh.EchoModePassword).Value(&token),
	)).WithTheme(wizardTheme()).WithAccessible(accessible)
	if err := form.RunWithContext(ctx); err != nil {
		return plan, wizardErr(err)
	}
	plan.CoreURL = strings.TrimSpace(coreURL)
	if token = strings.TrimSpace(token); token != "" {
		path, err := stageEnrollmentToken(token)
		if err != nil {
			return plan, err
		}
		plan.TokenFile, plan.RemoveTokenFile = path, true
	}

	var found discover.Result
	_ = spinner.New().Title("Looking for services, containers and ports on this host...").Context(ctx).
		Action(func() { found = discoverHost(ctx) }).Run()
	for _, n := range found.Notes {
		PrintSkip(n)
	}
	picks := pickableCandidates(found.Candidates)
	if len(picks) == 0 {
		PrintSkip("nothing to suggest; host metrics will still report")
		return plan, nil
	}
	var chosen []string
	opts2 := make([]huh.Option[string], 0, len(picks))
	for _, c := range picks {
		label := fmt.Sprintf("%-18s %-16s %s", c.Name, c.Kind, c.Confidence)
		if c.Detail != "" {
			label = fmt.Sprintf("%-18s %-16s %s  %s", c.Name, c.Detail, c.Kind, c.Confidence)
		}
		opts2 = append(opts2, huh.NewOption(label, c.ID).Selected(c.Confidence == discover.High))
	}
	setSetupTerminalTitle(2)
	if err := huh.NewForm(huh.NewGroup(
		huh.NewMultiSelect[string]().Title("What should Orion watch?").Description("Found by discovery. Space to toggle, a for all.").
			Options(opts2...).Value(&chosen),
	)).WithTheme(wizardTheme()).WithAccessible(accessible).RunWithContext(ctx); err != nil {
		return plan, wizardErr(err)
	}
	sel, err := discover.Select(found.Candidates, chosen)
	if err != nil {
		return plan, NewCommandError("could not select monitors", err)
	}
	plan.Selected = sel

	next, before, err := buildSetupConfig(opts, plan)
	if err != nil {
		return plan, NewCommandError("config validation failed", err)
	}
	data, _ := yaml.Marshal(next)
	diff := lineDiff(splitLines(string(before)), splitLines(string(data)))
	fmt.Fprintln(outputWriter)
	setSetupTerminalTitle(3)
	PrintHeader("Review changes")
	PrintInfo("config", opts.ConfigPath)
	if len(diff) == 0 {
		PrintOK("no changes to the config")
	}
	for _, l := range diff {
		fmt.Fprintln(outputWriter, "  "+l)
	}
	ok := true
	if err := huh.NewForm(huh.NewGroup(huh.NewConfirm().Title("Apply these changes?").Description("Applies without restarting the agent.").Affirmative("Yes").Negative("No").Value(&ok))).
		WithTheme(wizardTheme()).WithAccessible(accessible).RunWithContext(ctx); err != nil {
		return plan, wizardErr(err)
	}
	if !ok {
		return plan, NewCommandError("setup cancelled", errors.New("nothing was changed"))
	}
	plan.Confirmed = true
	return plan, nil
}

// setSetupTerminalTitle supplies the three-step counter in terminals that
// support OSC titles. It is only called in the interactive path.
func setSetupTerminalTitle(step int) {
	fmt.Fprintf(outputWriter, "\x1b]0;orion-agent setup, %d of 3\a", step)
}

func stageEnrollmentToken(token string) (string, error) {
	f, err := os.CreateTemp("", "orion-enroll-*")
	if err != nil {
		return "", NewCommandError("could not stage the token", err)
	}
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", NewCommandError("could not protect the staged token", err)
	}
	if _, err := f.WriteString(strings.TrimSpace(token)); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", NewCommandError("could not stage the token", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return "", NewCommandError("could not stage the token", err)
	}
	return f.Name(), nil
}

// pickableCandidates drops items that cannot become monitors and low-confidence noise.
func pickableCandidates(all []discover.Candidate) []discover.Candidate {
	var out []discover.Candidate
	for _, c := range all {
		if c.Monitor != nil && c.Confidence != discover.Low {
			out = append(out, c)
		}
	}
	return out
}

func validateCoreURL(s string) error {
	u, err := url.Parse(strings.TrimSpace(s))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return errors.New("enter a full URL such as https://orion.example.com")
	}
	return nil
}

func envOrEmpty(name string) string { v, _ := config.Env(name); return v }

func wizardErr(err error) error {
	if errors.Is(err, huh.ErrUserAborted) {
		return NewCommandError("setup cancelled", errors.New("nothing was changed"))
	}
	return NewCommandError("setup wizard failed", err)
}
