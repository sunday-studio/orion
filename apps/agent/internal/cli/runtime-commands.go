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
	PrintHeader("run")
	PrintInfo("config", opts.ConfigPath)
	PrintInfo("state", opts.StatePath)
	PrintInfo("once", opts.Once)
	if opts.Verbose {
		PrintInfo("verbose", true)
	}

	PrintStep("loading config")
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
	PrintOK(fmt.Sprintf("config loaded with %d monitor(s)", len(userConfig.Monitors)))
	if !opts.Once {
		PrintInfo("core_url", userConfig.CoreURL)
		PrintInfo("interval", userConfig.Interval)
		PrintInfo("log_file", userConfig.Logging.Path)
	}
	if len(userConfig.Monitors) == 0 {
		PrintSkip("no monitor checks configured; host metrics will still report")
	}

	PrintStep("opening state database")
	stateStore, err := agentstate.Open(opts.StatePath)
	if err != nil {
		return NewCommandError("could not open state database", err)
	}
	defer stateStore.Close()
	PrintOK("state database ready")

	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if !opts.Once {
		return runDaemon(ctx, opts, userConfig, stateStore)
	}

	PrintStep("registering agent and monitors")
	if err := registration.New(userConfig, opts.ConfigPath, stateStore).RegisterAgentIfNeeded(); err != nil {
		return NewCommandError("could not register agent and monitors", err)
	}
	PrintOK("registration complete")
	agentInstance, err := agent.NewWithStateStore(userConfig, stateStore)
	if err != nil {
		return NewCommandError("could not initialize agent", err)
	}
	PrintStep("running one collection cycle")
	if err := agentInstance.RunOnce(ctx); err != nil {
		logging.Errorf("Agent run failed: %v", err)
		if transport.IsAuthError(err) {
			PrintError("Core rejected Agent credentials; reporting stopped")
			PrintInfo("recovery", "apply an admin-issued replacement token with orion-agent token apply")
		}
		return NewCommandError("agent run failed", err)
	}
	PrintOK("one collection cycle complete")
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
	if !opts.JSON {
		PrintHeader("maintenance")
		if registered {
			PrintInfo("state", opts.StatePath)
			PrintInfo("action", action)
			if reason != "" {
				PrintInfo("reason", reason)
			}
			PrintStep("updating maintenance mode")
		}
	}

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
		if !registered {
			return nil
		}
		PrintOK("maintenance mode disabled")
		PrintOK("core maintenance mode updated")
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
		if !registered {
			return nil
		}
		PrintOK("maintenance mode enabled")
		PrintInfo("maintenance ends", until.UTC().Format(time.RFC3339))
		if reason != "" {
			PrintInfo("maintenance reason", reason)
		}
		PrintOK("core maintenance mode updated")
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

	PrintHeader("config validate")
	PrintInfo("config", opts.ConfigPath)
	PrintStep("loading config")
	PrintOK("config file is valid")
	PrintInfo("core_url", userConfig.CoreURL)
	PrintInfo("interval", userConfig.Interval)
	PrintInfo("monitors", len(userConfig.Monitors))
	if len(userConfig.Monitors) == 0 {
		PrintSkip("no monitor checks configured; host metrics will still report")
	}
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
	PrintHeader("state init")
	PrintOK("state database initialized")
	PrintInfo("file", stateStore.Path())
	return nil
}

func runTokenApply(_ context.Context, opts *Options) error {
	PrintHeader("token apply")
	PrintInfo("state", opts.StatePath)

	replacementToken, err := replacementTokenFromOptions(opts)
	if err != nil {
		return NewCommandError("could not apply replacement token", err)
	}
	if replacementToken == "" {
		return NewCommandError("could not apply replacement token", fmt.Errorf("replacement token is required"), "use: orion-agent token apply --token-file /path/to/token")
	}

	PrintStep("opening state database")
	stateStore, err := agentstate.Open(opts.StatePath)
	if err != nil {
		return NewCommandError("could not open state database", err)
	}
	defer stateStore.Close()
	PrintOK("state database ready")

	PrintStep("loading local agent state")
	before, err := stateStore.Get()
	if err != nil {
		return NewCommandError("could not load state", err)
	}
	if !before.IsRegistered() || before.AgentID == "" || before.CoreURL == "" {
		return NewCommandError("could not apply replacement token", fmt.Errorf("state is not registered"), "run: orion-agent reconfigure only if a new Agent identity is intended")
	}
	PrintInfo("agent_id", before.AgentID)
	PrintInfo("core_url", before.CoreURL)
	PrintInfo("monitor_mappings", len(before.Monitors))

	PrintStep("saving replacement token")
	if err := stateStore.ApplyReplacementToken(replacementToken); err != nil {
		return NewCommandError("could not apply replacement token", err)
	}
	PrintOK("replacement token applied")
	PrintInfo("identity", "preserved")
	PrintInfo("next", "orion-agent restart")
	if opts.JSON {
		return renderJSON(outputWriter, map[string]any{"ok": true, "identity": "preserved", "restart_required": true})
	}
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

	PrintHeader("reconfigure")
	PrintStep("checking service")
	wasRunning, status, err := GetServiceStatus()
	if err != nil {
		return NewCommandError("could not get service status", err)
	}
	PrintInfo("service_manager", DetectServiceManager())
	PrintInfo("agent_service", status)

	if wasRunning {
		PrintStep("stopping service")
		if err := StopService(); err != nil {
			return NewCommandError("could not stop service", err)
		}
		PrintOK("agent service stopped")
	}

	PrintStep("resetting local registration")
	PrintOK("local registration reset")

	PrintStep("registering agent and monitors")
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
			PrintStep("restoring service after failed reconfigure")
			if restartErr := StartService(); restartErr != nil {
				return NewCommandError("could not register agent or restore service", errors.Join(err, restartErr))
			}
			PrintOK("agent service restored")
		}
		return NewCommandError("could not register agent and monitors", err)
	}
	PrintOK("registration complete")

	if wasRunning {
		PrintStep("starting service")
		if err := StartService(); err != nil {
			return NewCommandError("could not start service", err)
		}
		PrintOK("agent service started")
	} else {
		PrintSkip("service was not running before reconfigure")
	}
	return nil
}
