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
	form := huh.NewForm(huh.NewGroup(
		huh.NewInput().Title("Orion Core URL").Description("Where this host reports to, for example https://orion.example.com").
			Value(&coreURL).Validate(validateCoreURL),
		huh.NewInput().Title("Enrollment token (optional)").Description("From Console > Add server. Leave empty to enroll later.").
			EchoMode(huh.EchoModePassword).Value(&token),
	)).WithTheme(wizardTheme()).WithAccessible(accessible)
	if err := form.RunWithContext(ctx); err != nil {
		return plan, wizardErr(err)
	}
	plan.CoreURL = strings.TrimSpace(coreURL)
	if token = strings.TrimSpace(token); token != "" {
		f, err := os.CreateTemp("", "orion-enroll-*")
		if err != nil {
			return plan, NewCommandError("could not stage the token", err)
		}
		f.Chmod(0o600)
		f.WriteString(token)
		f.Close()
		plan.TokenFile = f.Name() // removed by applySetup once enrollment has run
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
		label := fmt.Sprintf("%-9s %-28s %s (%s)", c.Kind, c.Name, c.Detail, c.Confidence)
		opts2 = append(opts2, huh.NewOption(label, c.ID).Selected(c.Confidence == discover.High))
	}
	if err := huh.NewForm(huh.NewGroup(
		huh.NewMultiSelect[string]().Title("What should Orion monitor?").Description("space selects, enter confirms; high-confidence items are preselected").
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
	PrintHeader("config changes")
	if len(diff) == 0 {
		PrintOK("no changes to the config")
	}
	for _, l := range diff {
		fmt.Fprintln(outputWriter, "  "+l)
	}
	ok := true
	if err := huh.NewForm(huh.NewGroup(huh.NewConfirm().Title("Apply these changes?").Description("Applied without restarting the agent.").Affirmative("Apply").Negative("Cancel").Value(&ok))).
		WithTheme(wizardTheme()).WithAccessible(accessible).RunWithContext(ctx); err != nil {
		return plan, wizardErr(err)
	}
	if !ok {
		return plan, NewCommandError("setup cancelled", errors.New("nothing was changed"))
	}
	plan.Confirmed = true
	return plan, nil
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
