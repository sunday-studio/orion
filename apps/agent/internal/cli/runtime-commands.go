package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	agent "orion/agent/internal"
	"orion/agent/internal/config"
	"orion/agent/internal/logging"
	"orion/agent/internal/registration"
	agentstate "orion/agent/internal/state"
	"orion/agent/internal/transport"
)

func runAgent(ctx context.Context, opts *Options) error {
	if opts.Once && !opts.Verbose {
		logging.SetLevel(logging.LevelFatal)
		defer logging.SetLevel(logging.LevelInfo)
	}
	what := "foreground"
	if opts.Once {
		what = "foreground, one cycle"
	}
	fmt.Fprint(outputWriter, runHeader(theme, what))
	userConfig, err := config.LoadUserConfig(opts.ConfigPath)
	if err != nil {
		return NewCommandError("could not load user config", err)
	}
	if !opts.Once {
		if err := configureRuntimeLogging(opts, userConfig.Logging); err != nil {
			return NewCommandError("could not configure runtime logging", err)
		}
		defer logging.Close()
	}
	monitors := len(userConfig.Monitors)
	fmt.Fprint(outputWriter, opLines(theme, []opStep{{Mark: "ok", Name: "Config loaded", Detail: fmt.Sprintf("%d %s", monitors, plural(monitors, "monitor", "monitors"))}}))
	if monitors == 0 {
		fmt.Fprint(outputWriter, opLines(theme, []opStep{{Mark: "warn", Name: "No monitors configured", Detail: "host metrics still report"}}))
	}

	stateStore, err := agentstate.Open(opts.StatePath)
	if err != nil {
		return NewCommandError("could not open state database", err)
	}
	defer stateStore.Close()
	fmt.Fprint(outputWriter, opLines(theme, []opStep{{Mark: "ok", Name: "State database ready"}}))

	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if !opts.Once {
		return runDaemon(ctx, opts, userConfig, stateStore)
	}

	if err := registration.New(userConfig, opts.ConfigPath, stateStore).RegisterAgentIfNeeded(); err != nil {
		return NewCommandError("could not register agent and monitors", err)
	}
	fmt.Fprint(outputWriter, opLines(theme, []opStep{{Mark: "ok", Name: "Registered"}}))
	agentInstance, err := agent.NewWithStateStore(userConfig, stateStore)
	if err != nil {
		return NewCommandError("could not initialize agent", err)
	}
	cycleStart := time.Now()
	if err := agentInstance.RunOnce(ctx); err != nil {
		logging.Errorf("Agent run failed: %v", err)
		if transport.IsAuthError(err) {
			return credentialsRejected()
		}
		return NewCommandError("agent run failed", err)
	}
	fmt.Fprint(outputWriter, opLines(theme, []opStep{{Mark: "ok", Name: "One collection cycle complete", Took: fmt.Sprintf("%.1f s", time.Since(cycleStart).Seconds())}}))
	return nil
}

func runMaintenance(_ context.Context, opts *Options, action string, reason string, duration time.Duration) error {
	action = strings.TrimPrefix(action, "-")
	stateStore, err := agentstate.Open(opts.StatePath)
	if err != nil {
		return NewCommandError("could not open state database", err)
	}
	defer stateStore.Close()
	internalState, err := stateStore.Get()
	if err != nil {
		return NewCommandError("could not load state", err)
	}
	registered := internalState.IsRegistered()

	switch action {
	case "up":
		if err := updateCoreMaintenanceMode(internalState, false, 0); err != nil {
			return NewCommandError("could not update core maintenance mode", err)
		}
		if err := stateStore.SetMaintenanceMode(false, nil); err != nil {
			return NewCommandError("could not save state", err)
		}
		if opts.JSON {
			return renderJSON(outputWriter, maintenanceResult{OK: true, Enabled: false, Via: "state"})
		}
		fmt.Fprint(outputWriter, maintenanceOutcome(theme, false, time.Time{}, time.Now(), registered))
	case "down":
		var reasonPtr *string
		if reason != "" {
			reasonPtr = &reason
		}
		if duration <= 0 {
			duration = defaultMaintenanceDuration
		}
		if err := updateCoreMaintenanceMode(internalState, true, duration); err != nil {
			return NewCommandError("could not update core maintenance mode", err)
		}
		until := time.Now().Add(duration)
		if err := stateStore.SetMaintenanceWindow(true, reasonPtr, &until); err != nil {
			return NewCommandError("could not save state", err)
		}
		if opts.JSON {
			return renderJSON(outputWriter, maintenanceResult{OK: true, Enabled: true, Reason: reason, Via: "state"})
		}
		printMaintenanceDown(opts, until, reason, duration, registered)
	}
	return nil
}

func runConfigValidate(_ context.Context, opts *Options) error {
	userConfig, err := config.LoadUserConfig(opts.ConfigPath)
	if err != nil {
		return NewCommandError("config validation failed", err)
	}
	if opts.JSON {
		return renderJSON(outputWriter, configSummaryJSON(opts.ConfigPath, userConfig))
	}

	fmt.Fprint(outputWriter, configValidScreen(theme, opts.ConfigPath, len(userConfig.Monitors)))
	return nil
}

type configSummaryPayload struct {
	Path     string                 `json:"path"`
	CoreURL  string                 `json:"core_url"`
	Interval string                 `json:"interval"`
	Logging  config.LoggingConfig   `json:"logging"`
	Monitors []configMonitorSummary `json:"monitors"`
}

type configMonitorSummary struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Interval string `json:"interval"`
}

func configSummaryJSON(path string, userConfig *config.UserConfig) configSummaryPayload {
	monitors := make([]configMonitorSummary, 0, len(userConfig.Monitors))
	for _, monitor := range userConfig.Monitors {
		monitors = append(monitors, configMonitorSummary{
			Name:     monitor.Name,
			Type:     string(monitor.Type),
			Interval: monitor.Interval,
		})
	}
	return configSummaryPayload{
		Path:     path,
		CoreURL:  userConfig.CoreURL,
		Interval: userConfig.Interval,
		Logging:  userConfig.Logging,
		Monitors: monitors,
	}
}

func runStateInit(_ context.Context, opts *Options) error {
	stateStore, err := agentstate.Open(opts.StatePath)
	if err != nil {
		return NewCommandError("could not open state database", err)
	}
	defer stateStore.Close()
	if _, err := stateStore.Get(); err != nil {
		return NewCommandError("could not initialize state database", err)
	}
	if opts.JSON {
		return renderJSON(outputWriter, map[string]any{"ok": true, "file": stateStore.Path()})
	}
	fmt.Fprint(outputWriter, credBlock([]string{credStep(theme, "State database ready", stateStore.Path())}))
	return nil
}

func runTokenApply(_ context.Context, opts *Options) error {
	replacementToken, err := replacementTokenFromOptions(opts)
	if err != nil {
		return NewCommandError("could not apply replacement token", err)
	}
	if replacementToken == "" {
		return NewCommandError("could not apply replacement token", fmt.Errorf("replacement token is required"), "use: orion-agent token apply --token-file /path/to/token")
	}
	stateStore, err := agentstate.Open(opts.StatePath)
	if err != nil {
		return NewCommandError("could not open state database", err)
	}
	defer stateStore.Close()
	before, err := stateStore.Get()
	if err != nil {
		return NewCommandError("could not load state", err)
	}
	if !before.IsRegistered() || before.AgentID == "" || before.CoreURL == "" {
		return NewCommandError("could not apply replacement token", fmt.Errorf("state is not registered"), "run: orion-agent reconfigure only if a new Agent identity is intended")
	}
	if err := stateStore.ApplyReplacementToken(replacementToken); err != nil {
		return NewCommandError("could not apply replacement token", err)
	}
	if opts.JSON {
		return renderJSON(outputWriter, map[string]any{"ok": true, "identity": "preserved", "restart_required": true})
	}
	fmt.Fprint(outputWriter, tokenAppliedOutput(before.AgentID, len(before.Monitors)))
	return nil
}

// tokenAppliedOutput is the finish of `token apply`, whichever path saved it.
func tokenAppliedOutput(agentID string, monitors int) string {
	return credBlock(
		[]string{credStep(theme, "Token saved", ""), credStep(theme, "Identity preserved", fmt.Sprintf("%s · %d %s", clip(agentID, 14), monitors, plural(monitors, "monitor", "monitors")))},
		[]string{nextCommand(theme, "orion-agent service restart")})
}

// promptReplacementToken asks for the token on a terminal when none was given.
func promptReplacementToken(opts *Options) error {
	if tok, err := replacementTokenFromOptions(opts); err != nil || tok != "" || !canPromptSecret(opts) {
		return nil
	}
	t, err := readMasked(theme, os.Stdin, outputWriter, "Replacement token", "64 characters, from the Console. Your identity and monitors are kept.")
	if err != nil {
		if errors.Is(err, errCancelled) {
			return NewCommandError("token apply cancelled", errors.New("nothing was changed"))
		}
		return nil
	}
	opts.TokenApply = t
	return nil
}

func replacementTokenFromOptions(opts *Options) (string, error) {
	inlineToken := strings.TrimSpace(opts.TokenApply)
	tokenFile := strings.TrimSpace(opts.TokenFile)
	if inlineToken != "" && tokenFile != "" {
		return "", fmt.Errorf("provide either a replacement token argument or --token-file, not both")
	}
	if tokenFile == "" {
		return inlineToken, nil
	}
	data, err := os.ReadFile(tokenFile)
	if err != nil {
		return "", fmt.Errorf("read token file: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

func runReconfigure(ctx context.Context, opts *Options, token, tokenFile string) error {
	if tokenFile != "" {
		data, err := os.ReadFile(tokenFile)
		if err != nil {
			return NewCommandError("could not read token file", err)
		}
		token = strings.TrimSpace(string(data))
	}
	if token != "" && looksLikeAgentToken(token) {
		return NewCommandError("this is a Server replacement token, not an Add server token", errors.New("reconfigure needs a fresh Add server token"), "run: orion-agent token apply --token-file <file>")
	}

	userConfig, err := config.LoadUserConfig(opts.ConfigPath)
	if err != nil {
		return NewCommandError("could not load user config", err)
	}
	if token == "" {
		token, err = config.Env(config.EnvEnrollment)
		if err != nil {
			return NewCommandError("could not read enrollment token", err)
		}
	}
	if token == "" {
		token = strings.TrimSpace(userConfig.EnrollmentToken)
	}
	if token == "" && canPromptSecret(opts) {
		t, perr := readMasked(theme, os.Stdin, outputWriter, "One-time token", "From Servers › Add server. Works once.")
		if errors.Is(perr, errCancelled) {
			return NewCommandError("reconfigure cancelled", errors.New("nothing was changed"))
		}
		if perr == nil {
			token = t
		}
	}
	if token == "" {
		return NewCommandError("a fresh Add server token is required", errors.New("reconfigure must be authorized by Core"), "create a token in Console under Servers, Add server", "run: orion-agent reconfigure --token-file <file>")
	}
	stateStore, err := agentstate.Open(opts.StatePath)
	if err != nil {
		return NewCommandError("could not open state database", err)
	}
	defer stateStore.Close()
	currentState, err := stateStore.Get()
	if err != nil {
		return NewCommandError("could not read local registration", err)
	}
	if currentState.IsRegistered() && strings.TrimRight(currentState.CoreURL, "/") != strings.TrimRight(userConfig.CoreURL, "/") {
		return NewCommandError(
			"reconfigure cannot move an enrolled host to another Core",
			fmt.Errorf("local state belongs to %s", currentState.CoreURL),
			"restore that Core URL, or install as a new host with a separate state directory",
		)
	}
	if _, err := checkSetupConnection(ctx, userConfig.CoreURL, token); err != nil {
		return err
	}

	wasRunning, _, err := GetServiceStatus()
	if err != nil {
		return NewCommandError("could not get service status", err)
	}
	var steps []string
	if wasRunning {
		if err := StopService(); err != nil {
			return NewCommandError("could not stop service", err)
		}
		steps = append(steps, credStep(theme, "Service stopped", ""))
	}
	steps = append(steps, credStep(theme, "Registration reset", ""))

	previous, hadPrevious := os.LookupEnv(config.EnvEnrollment)
	os.Setenv(config.EnvEnrollment, token)
	defer func() {
		if hadPrevious {
			os.Setenv(config.EnvEnrollment, previous)
		} else {
			os.Unsetenv(config.EnvEnrollment)
		}
	}()
	registrationService := registration.New(userConfig, opts.ConfigPath, stateStore)
	if err := registrationService.ReconnectAgent(); err != nil {
		if wasRunning {
			if restartErr := StartService(); restartErr != nil {
				return NewCommandError("could not register agent or restore service", errors.Join(err, restartErr))
			}
		}
		return NewCommandError("could not register agent and monitors", err)
	}
	monitors := 0
	if st, gerr := stateStore.Get(); gerr == nil {
		monitors = len(st.Monitors)
	}
	steps = append(steps, credStep(theme, "Registered again", fmt.Sprintf("%d %s", monitors, plural(monitors, "monitor", "monitors"))))
	if wasRunning {
		if err := StartService(); err != nil {
			return NewCommandError("could not start service", err)
		}
		steps = append(steps, credStep(theme, "Service started", ""))
	} else {
		steps = append(steps, credNote(theme, "Service was not running", "left as it was"))
	}
	fmt.Fprint(outputWriter, credBlock(steps))
	return nil
}

// credentialsRejected is the run-time error for a token Core no longer accepts.
func credentialsRejected() error {
	return NewCommandError("Core rejected this server's credentials",
		errors.New("Reporting stopped. Identity and monitors are kept."),
		"orion-agent token apply  with a replacement token from the Console")
}

// printMaintenanceDown shows the result of `maintenance down`: the form's
// panel when the form was used, otherwise the short step lines.
func printMaintenanceDown(opts *Options, until time.Time, reason string, d time.Duration, coreTold bool) {
	if opts.MaintenanceForm && coreTold {
		fmt.Fprint(outputWriter, maintenanceResultPanel(theme, until, time.Now(), reason, d))
		return
	}
	fmt.Fprint(outputWriter, maintenanceOutcome(theme, true, until, time.Now(), coreTold))
}
