package cli

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	agent "orion/agent/internal"
	"orion/agent/internal/config"
	"orion/agent/internal/logging"
	"orion/agent/internal/registration"
	agentstate "orion/agent/internal/state"

	"github.com/spf13/cobra"
)

func newStartCommand(ctx context.Context, opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the agent service",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := ensurePrivilegeFor(opts, "start"); err != nil {
				return NewCommandError("could not elevate privileges", err)
			}
			return runStart(ctx, opts)
		},
	}
	cmd.Flags().IntVar(&opts.LogLines, "lines", 80, "Number of service log lines to show")
	return cmd
}

func newStopCommand(ctx context.Context, opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the agent service",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := ensurePrivilegeFor(opts, "stop"); err != nil {
				return NewCommandError("could not elevate privileges", err)
			}
			return runStop(ctx, opts)
		},
	}
}

func newStatusCommand(ctx context.Context, opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show agent service status",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(ctx, opts)
		},
	}
}

func newRestartCommand(ctx context.Context, opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "restart",
		Short: "Restart the agent service",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := ensurePrivilegeFor(opts, "restart"); err != nil {
				return NewCommandError("could not elevate privileges", err)
			}
			return runRestart(ctx, opts)
		},
	}
	cmd.Flags().IntVar(&opts.LogLines, "lines", 80, "Number of service log lines to show")
	return cmd
}

func newUpdateCommand(ctx context.Context, opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Download and install a release binary",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := ensurePrivilegeFor(opts, "update"); err != nil {
				return NewCommandError("could not elevate privileges", err)
			}
			return runUpdate(ctx, opts)
		},
	}
	cmd.Flags().StringVar(&opts.UpdateVersion, "version", "latest", "Release version to install")
	cmd.Flags().StringVar(&opts.UpdateRepo, "repo", "sunday-studio/orion", "GitHub repository to use")
	cmd.Flags().IntVar(&opts.LogLines, "lines", 80, "Number of service log lines to show")
	return cmd
}

func newRunCommand(ctx context.Context, opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run the agent",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if commandNeedsElevation("run", commandArgs(opts, "run")) {
				if err := ensurePrivilegeFor(opts, "run"); err != nil {
					return NewCommandError("could not elevate privileges", err)
				}
			}
			return runAgent(ctx, opts)
		},
	}
	cmd.Flags().BoolVar(&opts.Once, "once", false, "Run once and exit")
	return cmd
}

func newMaintenanceCommand(ctx context.Context, opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "maintenance",
		Short: "Manage maintenance mode",
	}

	down := &cobra.Command{
		Use:   "down [reason]",
		Short: "Enter maintenance mode",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if commandNeedsElevation("maintenance", commandArgs(opts, "maintenance")) {
				if err := ensurePrivilegeFor(opts, "maintenance"); err != nil {
					return NewCommandError("could not elevate privileges", err)
				}
			}
			reason := ""
			if len(args) > 0 {
				reason = fmt.Sprint(args[0])
				for _, part := range args[1:] {
					reason += " " + part
				}
			}
			return runMaintenance(ctx, opts, "-down", reason)
		},
	}

	up := &cobra.Command{
		Use:   "up",
		Short: "Exit maintenance mode",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if commandNeedsElevation("maintenance", commandArgs(opts, "maintenance")) {
				if err := ensurePrivilegeFor(opts, "maintenance"); err != nil {
					return NewCommandError("could not elevate privileges", err)
				}
			}
			return runMaintenance(ctx, opts, "-up", "")
		},
	}

	cmd.AddCommand(down, up)
	return cmd
}

func newConfigCommand(ctx context.Context, opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration",
	}
	validate := &cobra.Command{
		Use:   "validate",
		Short: "Validate config file",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigValidate(ctx, opts)
		},
	}
	diff := &cobra.Command{
		Use:     "diff",
		Aliases: []string{"show"},
		Short:   "Show config summary",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigDiff(ctx, opts, cmd.CalledAs())
		},
	}
	cmd.AddCommand(validate, diff)
	return cmd
}

func newStateCommand(ctx context.Context, opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "state",
		Short: "Manage local state",
	}
	init := &cobra.Command{
		Use:   "init",
		Short: "Initialize state database",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if commandNeedsElevation("state", commandArgs(opts, "state")) {
				if err := ensurePrivilegeFor(opts, "state"); err != nil {
					return NewCommandError("could not elevate privileges", err)
				}
			}
			return runStateInit(ctx, opts)
		},
	}
	cmd.AddCommand(init)
	return cmd
}

func newReconfigureCommand(ctx context.Context, opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "reconfigure",
		Short: "Reset local registration and reconnect using installed config",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := ensurePrivilegeFor(opts, "reconfigure"); err != nil {
				return NewCommandError("could not elevate privileges", err)
			}
			return runReconfigure(ctx, opts)
		},
	}
}

func configureLogging(opts *Options) {
	if opts.Verbose {
		logging.SetLevel(logging.LevelDebug)
		logging.Debugf("verbose logging enabled")
	}
}

func configureRuntimeLogging(opts *Options, loggingConfig config.LoggingConfig) error {
	level, err := logging.ParseLevel(loggingConfig.Level)
	if err != nil {
		return err
	}
	if opts.Verbose {
		level = logging.LevelDebug
	}

	return logging.ConfigureFile(logging.FileConfig{
		Path:       loggingConfig.Path,
		Level:      level,
		MaxSizeMB:  loggingConfig.MaxSizeMB,
		MaxBackups: loggingConfig.MaxBackups,
		MaxAgeDays: loggingConfig.MaxAgeDays,
		Compress:   loggingConfig.CompressEnabled(),
	})
}

func runStart(_ context.Context, opts *Options) error {
	PrintHeader("start")
	preflight := BuildServicePreflight(opts.ConfigPath, opts.StatePath)
	PrintPreflightReport(preflight)
	if preflight.HasErrors() {
		PrintServiceDiagnostics(opts.LogLines)
		return NewCommandError("start preflight failed", fmt.Errorf("one or more required checks failed"), "run: orion-agent doctor", "run: orion-agent logs")
	}
	PrintStep("resetting service failure state")
	if err := ResetServiceFailures(); err != nil {
		PrintSkip(fmt.Sprintf("could not reset service failure state: %v", err))
	} else {
		PrintOK("service failure state reset")
	}
	PrintStep("starting service")
	if err := StartService(); err != nil {
		PrintServiceDiagnostics(opts.LogLines)
		return NewCommandError("could not start Orion Agent service", err, "run: orion-agent logs", "run: orion-agent status")
	}
	PrintOK("agent service started")
	printServiceStatus()
	return nil
}

func runStop(_ context.Context, _ *Options) error {
	manager := DetectServiceManager()
	PrintHeader("stop")
	PrintInfo("service_manager", manager)
	PrintStep("stopping service")
	if err := StopService(); err != nil {
		return NewCommandError("could not stop Orion Agent service", err)
	}
	PrintOK("agent service stopped")
	printServiceStatus()
	return nil
}

func runStatus(_ context.Context, opts *Options) error {
	PrintHeader("status")
	report := InspectAgentStatus(opts.StatePath)

	fmt.Fprintf(outputWriter, "  service_manager: %s\n", report.Service.Manager)
	fmt.Fprintf(outputWriter, "  agent_service: %s\n", report.Service.State)
	if report.Service.ServiceFile != "" {
		fmt.Fprintf(outputWriter, "  service_file: %s\n", report.Service.ServiceFile)
	}
	if report.Service.Error != nil {
		fmt.Fprintf(outputWriter, "  service_error: %v\n", report.Service.Error)
	}

	if report.StateCheck.Status != CheckOK {
		fmt.Fprintf(outputWriter, "  state_database: %s\n", opts.StatePath)
		if report.StateCheck.Error != nil {
			fmt.Fprintf(outputWriter, "  state: unavailable (%v)\n", report.StateCheck.Error)
		} else {
			fmt.Fprintf(outputWriter, "  state: unavailable (%s)\n", report.StateCheck.Detail)
		}
	} else if report.InternalState != nil {
		fmt.Fprintf(outputWriter, "  state_database: %s\n", opts.StatePath)
		fmt.Fprintf(outputWriter, "  state: %s\n", report.StateCheck.Detail)
		fmt.Fprintf(outputWriter, "  registered: %t\n", report.InternalState.IsRegistered())
		if report.InternalState.AgentID != "" {
			fmt.Fprintf(outputWriter, "  agent_id: %s\n", report.InternalState.AgentID)
		}
		if report.InternalState.CoreURL != "" {
			fmt.Fprintf(outputWriter, "  core_url: %s\n", report.InternalState.CoreURL)
		}
		fmt.Fprintf(outputWriter, "  maintenance: %t\n", report.InternalState.MaintenanceMode)
		if report.InternalState.MaintenanceReason != nil {
			fmt.Fprintf(outputWriter, "  maintenance_reason: %s\n", *report.InternalState.MaintenanceReason)
		}
	}

	if !report.Service.Running {
		PrintSkip("service is not running")
		return &CommandError{Summary: "service is not running", ExitCode: 1}
	}
	PrintOK("service is running")
	return nil
}

func runRestart(_ context.Context, opts *Options) error {
	PrintHeader("restart")
	preflight := BuildServicePreflight(opts.ConfigPath, opts.StatePath)
	PrintPreflightReport(preflight)
	if preflight.HasErrors() {
		PrintServiceDiagnostics(opts.LogLines)
		return NewCommandError("restart preflight failed", fmt.Errorf("one or more required checks failed"), "run: orion-agent doctor", "run: orion-agent logs")
	}
	PrintStep("resetting service failure state")
	if err := ResetServiceFailures(); err != nil {
		PrintSkip(fmt.Sprintf("could not reset service failure state: %v", err))
	} else {
		PrintOK("service failure state reset")
	}
	PrintStep("restarting service")
	if err := RestartService(); err != nil {
		PrintServiceDiagnostics(opts.LogLines)
		return NewCommandError("could not restart Orion Agent service", err, "run: orion-agent logs", "run: orion-agent status")
	}
	PrintOK("agent service restarted")
	PrintServiceDiagnostics(opts.LogLines)
	printServiceStatus()
	return nil
}

func runUpdate(_ context.Context, opts *Options) error {
	if err := UpdateAgent(UpdateOptions{
		Repo:           opts.UpdateRepo,
		Version:        opts.UpdateVersion,
		CurrentVersion: agent.Version,
		LogLines:       opts.LogLines,
	}); err != nil {
		return NewCommandError("could not update Orion Agent", err)
	}
	return nil
}

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
			return NewCommandError("agent run failed", err)
		}
		PrintOK("one collection cycle complete")
		return nil
	}

	PrintStep("starting continuous collection loop")
	if err := agentInstance.Run(ctx); err != nil {
		logging.Errorf("Agent stopped with error: %v", err)
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
	PrintHeader("config validate")
	PrintInfo("config", opts.ConfigPath)
	PrintStep("loading config")
	userConfig, err := config.LoadUserConfig(opts.ConfigPath)
	if err != nil {
		PrintError("config validation failed")
		PrintInfo("file", opts.ConfigPath)
		PrintInfo("reason", err)
		return NewCommandError("config validation failed", err)
	}
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
	PrintHeader("config " + calledAs)
	PrintInfo("config", opts.ConfigPath)
	PrintStep("loading config")
	currentConfig, err := config.LoadUserConfig(opts.ConfigPath)
	if err != nil {
		return NewCommandError("could not load current config", err)
	}
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

func printServiceStatus() {
	status := GetServiceStatusResult()
	PrintInfo("service_state", status.State)
	if status.Error != nil {
		PrintError(fmt.Sprintf("could not read service state: %v", status.Error))
	}
	if status.Running {
		PrintOK("service is running")
	} else {
		PrintSkip("service is not running")
	}
}
