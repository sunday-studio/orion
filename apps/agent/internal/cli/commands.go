package cli

import (
	"context"
	"fmt"

	"orion/agent/internal/config"
	"orion/agent/internal/logging"

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

func newDoctorCommand(ctx context.Context, opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Run Agent diagnostics",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runDoctor(ctx, opts)
		},
	}
}

func newSetupCommand(ctx context.Context, opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Create a starter Agent config",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runSetup(ctx, opts)
		},
	}
	cmd.Flags().StringVar(&opts.SetupCoreURL, "core-url", "", "Core URL to write to the config")
	cmd.Flags().BoolVar(&opts.SetupForce, "force", false, "Overwrite an existing config file")
	cmd.Flags().BoolVar(&opts.SetupInitState, "init-state", false, "Initialize the state database after writing config")
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
		Use:   "diff",
		Short: "Show config summary",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigDiff(ctx, opts, cmd.CalledAs())
		},
	}
	show := &cobra.Command{
		Use:   "show",
		Short: "Show config summary",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigDiff(ctx, opts, cmd.CalledAs())
		},
	}
	cmd.AddCommand(validate, diff, show)
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

func newTokenCommand(ctx context.Context, opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "token",
		Short: "Manage local Agent credentials",
	}
	apply := &cobra.Command{
		Use:   "apply [replacement-token]",
		Short: "Apply a replacement token without changing Agent identity",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if commandNeedsElevation("token", commandArgs(opts, "token")) {
				if err := ensurePrivilegeFor(opts, "token"); err != nil {
					return NewCommandError("could not elevate privileges", err)
				}
			}
			if len(args) > 0 {
				opts.TokenApply = args[0]
			}
			return runTokenApply(ctx, opts)
		},
	}
	apply.Flags().StringVar(&opts.TokenFile, "token-file", "", "Path to a file containing the replacement token")
	cmd.AddCommand(apply)
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
	logging.ConfigureText(outputWriter)
	logging.SetTextColorEnabled(!opts.NoColor && writerSupportsColor(outputWriter))
	if opts.Verbose {
		logging.SetLevel(logging.LevelDebug)
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
