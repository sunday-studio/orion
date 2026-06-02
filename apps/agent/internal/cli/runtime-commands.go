package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	agent "orion/agent/internal"
	"orion/agent/internal/config"
	"orion/agent/internal/logging"
	"orion/agent/internal/registration"
	agentstate "orion/agent/internal/state"
	"orion/agent/internal/transport"
)

func runAgent(ctx context.Context, opts *Options) error {
	PrintHeader("run")
	PrintInfo("config", opts.ConfigPath)
	PrintInfo("state", opts.StatePath)
	PrintInfo("once", opts.Once)
	PrintInfo("verbose", opts.Verbose)

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
	PrintInfo("core_url", userConfig.CoreURL)
	PrintInfo("interval", userConfig.Interval)
	if !opts.Once {
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

	PrintStep("registering agent and monitors")
	registrationService := registration.New(userConfig, opts.ConfigPath, stateStore)
	if err := registrationService.RegisterAgentIfNeeded(); err != nil {
		PrintError("registration failed after retry attempts; agent cannot continue")
		return NewCommandError("could not register agent and monitors", err)
	}
	PrintOK("registration complete")
	if internalState, err := stateStore.Get(); err == nil {
		PrintInfo("registered", internalState.IsRegistered())
		if internalState.AgentID != "" {
			PrintInfo("agent_id", internalState.AgentID)
		}
		PrintInfo("monitor_mappings", len(internalState.Monitors))
	}

	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	agentInstance, err := agent.NewWithStateStore(userConfig, stateStore)
	if err != nil {
		return NewCommandError("could not initialize agent", err)
	}
	PrintOK("agent initialized")

	if opts.Once {
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

	PrintStep("starting continuous collection loop")
	if err := agentInstance.Run(ctx); err != nil {
		logging.Errorf("Agent stopped with error: %v", err)
		if transport.IsAuthError(err) {
			PrintError("Core rejected Agent credentials; reporting stopped")
			PrintInfo("recovery", "apply an admin-issued replacement token with orion-agent token apply")
		}
		return NewCommandError("agent stopped with an error", err)
	}

	PrintOK("agent exited cleanly")
	return nil
}

func runMaintenance(_ context.Context, opts *Options, action string, reason string) error {
	PrintHeader("maintenance")
	PrintInfo("state", opts.StatePath)
	PrintInfo("action", action)
	if reason != "" {
		PrintInfo("reason", reason)
	}

	PrintStep("opening state database")
	stateStore, err := agentstate.Open(opts.StatePath)
	if err != nil {
		return NewCommandError("could not open state database", err)
	}
	defer stateStore.Close()
	PrintOK("state database ready")

	PrintStep("loading local agent state")
	internalState, err := stateStore.Get()
	if err != nil {
		return NewCommandError("could not load state", err)
	}
	PrintOK("local agent state loaded")

	switch action {
	case "-up":
		PrintStep("updating maintenance mode")
		if err := updateCoreMaintenanceMode(internalState, false); err != nil {
			return NewCommandError("could not update core maintenance mode", err)
		}
		if err := stateStore.SetMaintenanceMode(false, nil); err != nil {
			return NewCommandError("could not save state", err)
		}
		PrintOK("maintenance mode disabled")
	case "-down":
		var reasonPtr *string
		if reason != "" {
			reasonPtr = &reason
		}
		PrintStep("updating maintenance mode")
		if err := updateCoreMaintenanceMode(internalState, true); err != nil {
			return NewCommandError("could not update core maintenance mode", err)
		}
		if err := stateStore.SetMaintenanceMode(true, reasonPtr); err != nil {
			return NewCommandError("could not save state", err)
		}
		PrintOK("maintenance mode enabled")
		if reason != "" {
			PrintInfo("maintenance reason", reason)
		}
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

func runConfigDiff(_ context.Context, opts *Options, calledAs string) error {
	if calledAs == "" {
		calledAs = "diff"
	}
	currentConfig, err := config.LoadUserConfig(opts.ConfigPath)
	if err != nil {
		return NewCommandError("could not load current config", err)
	}
	if opts.JSON {
		return renderJSON(outputWriter, configSummaryJSON(opts.ConfigPath, currentConfig))
	}

	PrintHeader("config " + calledAs)
	PrintInfo("config", opts.ConfigPath)
	PrintStep("loading config")
	PrintOK("config loaded")

	fmt.Fprintln(outputWriter, "Current configuration:")
	fmt.Fprintf(outputWriter, "  Core URL: %s\n", currentConfig.CoreURL)
	fmt.Fprintf(outputWriter, "  Interval: %s\n", currentConfig.Interval)
	fmt.Fprintf(outputWriter, "  Monitors: %d\n", len(currentConfig.Monitors))
	if len(currentConfig.Monitors) == 0 {
		fmt.Fprintln(outputWriter, "    - none configured")
	} else {
		for _, m := range currentConfig.Monitors {
			fmt.Fprintf(outputWriter, "    - %s (%s every %s)\n", m.Name, m.Type, m.Interval)
		}
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
	PrintHeader("state init")
	PrintInfo("state", opts.StatePath)
	PrintStep("opening state database")
	stateStore, err := agentstate.Open(opts.StatePath)
	if err != nil {
		return NewCommandError("could not open state database", err)
	}
	defer stateStore.Close()
	PrintOK("state database ready")

	PrintStep("ensuring default agent state row")
	if _, err := stateStore.Get(); err != nil {
		return NewCommandError("could not initialize state database", err)
	}
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

func runReconfigure(_ context.Context, opts *Options) error {
	PrintHeader("reconfigure")
	PrintInfo("config", opts.ConfigPath)
	PrintInfo("state", opts.StatePath)

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

	PrintStep("loading config")
	userConfig, err := config.LoadUserConfig(opts.ConfigPath)
	if err != nil {
		return NewCommandError("could not load user config", err)
	}
	PrintOK(fmt.Sprintf("config loaded with %d monitor(s)", len(userConfig.Monitors)))
	PrintInfo("core_url", userConfig.CoreURL)

	PrintStep("opening state database")
	stateStore, err := agentstate.Open(opts.StatePath)
	if err != nil {
		return NewCommandError("could not open state database", err)
	}
	defer stateStore.Close()
	PrintOK("state database ready")

	PrintStep("resetting local registration")
	if err := stateStore.ResetRegistration(); err != nil {
		return NewCommandError("could not reset registration state", err)
	}
	PrintOK("local registration reset")

	PrintStep("registering agent and monitors")
	registrationService := registration.New(userConfig, opts.ConfigPath, stateStore)
	if err := registrationService.RegisterAgentIfNeeded(); err != nil {
		return NewCommandError("could not register agent and monitors", err)
	}
	PrintOK("registration complete")
	if internalState, err := stateStore.Get(); err == nil {
		PrintInfo("registered", internalState.IsRegistered())
		if internalState.AgentID != "" {
			PrintInfo("agent_id", internalState.AgentID)
		}
		PrintInfo("monitor_mappings", len(internalState.Monitors))
	}

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
