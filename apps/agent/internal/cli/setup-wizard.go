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
	current := currentSetupState(opts.StatePath)

	coreURL := firstNonEmpty(opts.SetupCoreURL, envOrEmpty(config.EnvCoreURL))
	if coreURL == "" {
		if existing, _, err := config.Load(opts.ConfigPath); err == nil {
			coreURL = existing.CoreURL
		}
	}
	token := ""
	setSetupTerminalTitle(1)
	fields := []huh.Field{
		huh.NewInput().Title("Core URL").Description("Where is your Orion Core?").Prompt(" ").
			Value(&coreURL).Validate(validateCoreURL),
	}
	if current == nil {
		fields = append(fields, huh.NewInput().Title("Add server token").
			Description("Create one in Console, Servers, Add server").Prompt(" ").
			EchoMode(huh.EchoModePassword).Value(&token))
	}
	connect := huh.NewGroup(fields...).Title("Connect this host").Description(strings.Repeat("─", 62))
	form := huh.NewForm(connect).WithTheme(wizardTheme()).WithAccessible(accessible)
	if err := form.RunWithContext(ctx); err != nil {
		return plan, wizardErr(err)
	}
	plan.CoreURL = strings.TrimSpace(coreURL)
	var connection setupConnection
	var connectionErr error
	checkTitle := "Checking Core and Add server token..."
	if current != nil {
		checkTitle = "Checking the existing Core connection..."
	}
	_ = spinner.New().Title(checkTitle).Context(ctx).Action(func() {
		if current != nil {
			connection, connectionErr = checkCurrentSetupConnection(ctx, current, plan.CoreURL)
			return
		}
		connection, connectionErr = checkSetupConnection(ctx, plan.CoreURL, strings.TrimSpace(token))
	}).Run()
	if connectionErr != nil {
		return plan, connectionErr
	}
	fmt.Fprintf(outputWriter, "%s Core reachable %s   %s %s\n",
		theme.Pass(), setupConnectionLatency(connection), theme.Pass(), setupConnectionTokenLabel(connection))
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
	picks, omitted := pickableCandidates(found.Candidates)
	setSetupTerminalTitle(2)
	if len(picks) == 0 {
		PrintHeader("What should Orion watch?")
		PrintSkip("nothing suggested; system metrics will still report")
	} else {
		var chosen []string
		opts2 := make([]huh.Option[string], 0, len(picks))
		for _, c := range picks {
			opts2 = append(opts2, huh.NewOption(candidateOptionLabel(c), c.ID).Selected(c.Confidence == discover.High))
		}
		description := "Found by discovery. Space to toggle, a for all."
		if omitted > 0 {
			description = fmt.Sprintf("Top suggestions shown; %d more available with orion-agent discover.", omitted)
		}
		selectField := huh.NewMultiSelect[string]().Title("What should Orion watch?").Description(description).
			Options(opts2...).Value(&chosen).Height(min(len(opts2), maxWizardCandidates) + 2).Filterable(true)
		if err := huh.NewForm(huh.NewGroup(selectField).Description(strings.Repeat("─", 62))).
			WithTheme(wizardTheme()).WithAccessible(accessible).RunWithContext(ctx); err != nil {
			return plan, wizardErr(err)
		}
		sel, err := discover.Select(found.Candidates, chosen)
		if err != nil {
			return plan, NewCommandError("could not select monitors", err)
		}
		plan.Selected = sel
	}

	next, before, err := buildSetupConfig(opts, plan)
	if err != nil {
		return plan, NewCommandError("config validation failed", err)
	}
	data, _ := yaml.Marshal(next)
	diff := lineDiff(splitLines(string(before)), splitLines(string(data)))
	fmt.Fprintln(outputWriter)
	setSetupTerminalTitle(3)
	PrintHeader("Review changes")
	fmt.Fprintln(outputWriter, theme.Muted(opts.ConfigPath))
	fmt.Fprintln(outputWriter)
	if len(diff) == 0 {
		PrintOK("no changes to the config")
	}
	for _, l := range diff {
		switch {
		case strings.HasPrefix(l, "+ "):
			fmt.Fprintln(outputWriter, theme.Tone("up", "+ "+strings.TrimPrefix(l, "+ ")))
		case strings.HasPrefix(l, "- "):
			fmt.Fprintln(outputWriter, theme.Tone("down", "- "+strings.TrimPrefix(l, "- ")))
		default:
			fmt.Fprintln(outputWriter, l)
		}
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
const maxWizardCandidates = 6

func pickableCandidates(all []discover.Candidate) ([]discover.Candidate, int) {
	var out []discover.Candidate
	for _, c := range all {
		if c.Monitor != nil && c.Confidence != discover.Low {
			out = append(out, c)
		}
	}
	if len(out) <= maxWizardCandidates {
		return out, 0
	}
	return out[:maxWizardCandidates], len(out) - maxWizardCandidates
}

func candidateOptionLabel(candidate discover.Candidate) string {
	return fmt.Sprintf("%-24s %-12s %s", truncateLabel(candidate.Name, 24), candidateKindLabel(candidate), candidate.Confidence)
}

func candidateKindLabel(candidate discover.Candidate) string {
	switch candidate.Kind {
	case discover.KindService:
		return "system service"
	case discover.KindContainer:
		return "docker"
	case discover.KindPM2:
		return "pm2"
	case discover.KindPort:
		return "port"
	default:
		return string(candidate.Kind)
	}
}

func truncateLabel(value string, width int) string {
	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	if width <= 1 {
		return string(runes[:width])
	}
	return string(runes[:width-1]) + "…"
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
