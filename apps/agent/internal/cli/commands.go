package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

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
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Is this host reporting? One glance, one next step",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(ctx, opts)
		},
	}
	cmd.Flags().BoolVar(&opts.StatusShort, "short", false, "One line, for prompts and cron")
	cmd.Flags().BoolVar(&opts.StatusWatch, "watch", false, "Redraw live in place (q quits)")
	return cmd
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
	cmd.Flags().StringVar(&opts.UpdateBaseURL, "base-url", "", "Release asset base URL (mirrors and tests)")
	cmd.Flags().BoolVar(&opts.UpdateRollback, "rollback", false, "Restore the binary that was installed before the last update")
	cmd.Flags().BoolVar(&opts.UpdateForce, "force", false, "Reinstall even when already on the target version")
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
		Short: "Connect this host and choose what Orion should monitor",
		Long: "Interactive on a terminal: asks for the Core URL and Add server token, discovers services, " +
			"containers and ports on this host, lets you pick what to monitor, shows the config diff and applies it " +
			"without a restart. Non-interactive with --yes (or when stdin is not a terminal): pass --core-url, " +
			"--token and --accept-discovered, or use --token-file and --monitor selectors; `orion-agent discover` lists the ids.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if commandNeedsElevation("setup", commandArgs(opts, "setup")) {
				if err := ensurePrivilegeFor(opts, "setup"); err != nil {
					return NewCommandError("could not elevate privileges", err)
				}
			}
			return runSetup(ctx, opts)
		},
	}
	f := cmd.Flags()
	f.StringVar(&opts.SetupCoreURL, "core-url", "", "Core URL (or "+"ORION_CORE_URL"+")")
	f.BoolVar(&opts.SetupForce, "force", false, "Start from a blank config instead of adding to the existing one")
	f.BoolVar(&opts.SetupInitState, "init-state", false, "Initialize the state database after writing config")
	f.BoolVar(&opts.SetupYes, "yes", false, "Never prompt; apply the flags as given")
	f.BoolVar(&opts.SetupDryRun, "dry-run", false, "Show the config diff and change nothing")
	f.StringVar(&opts.SetupToken, "token", "", "One-time Add server token; prefer --token-file for unattended use")
	f.StringVar(&opts.SetupTokenFile, "token-file", "", "File with a one-time Add server token; connects this host")
	f.BoolVar(&opts.SetupAcceptDiscovered, "accept-discovered", false, "Add all high-confidence discovered monitors")
	f.StringSliceVar(&opts.SetupMonitors, "monitor", nil, "Discovered ids to monitor (e.g. service:nginx), or high, medium, all; repeatable")
	cmd.MarkFlagsMutuallyExclusive("token", "token-file")
	cmd.MarkFlagsMutuallyExclusive("accept-discovered", "monitor")
	return cmd
}

func newMaintenanceCommand(ctx context.Context, opts *Options) *cobra.Command {
	cmd := newGroupCommand("maintenance", "Manage maintenance mode")

	var maintenanceFor time.Duration
	down := &cobra.Command{
		Use:   "down [reason]",
		Short: "Enter maintenance mode (ends automatically after --for, default 24h)",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if maintenanceFor <= 0 || maintenanceFor > maxMaintenanceDuration {
				return NewCommandError("invalid --for duration", fmt.Errorf("must be greater than zero and at most 720h"))
			}
			reason := ""
			if len(args) > 0 {
				reason = fmt.Sprint(args[0])
				for _, part := range args[1:] {
					reason += " " + part
				}
			}
			if handled, err := maintenanceViaDaemon(opts, true, reason, maintenanceFor); handled {
				return err
			}
			if commandNeedsElevation("maintenance", commandArgs(opts, "maintenance")) {
				if err := ensurePrivilegeFor(opts, "maintenance"); err != nil {
					return NewCommandError("could not elevate privileges", err)
				}
			}
			return runMaintenance(ctx, opts, "down", reason, maintenanceFor)
		},
	}

	up := &cobra.Command{
		Use:   "up",
		Short: "Exit maintenance mode",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if handled, err := maintenanceViaDaemon(opts, false, "", 0); handled {
				return err
			}
			if commandNeedsElevation("maintenance", commandArgs(opts, "maintenance")) {
				if err := ensurePrivilegeFor(opts, "maintenance"); err != nil {
					return NewCommandError("could not elevate privileges", err)
				}
			}
			return runMaintenance(ctx, opts, "up", "", 0)
		},
	}

	down.Flags().DurationVar(&maintenanceFor, "for", defaultMaintenanceDuration, "how long maintenance lasts before alerting resumes (max 720h)")
	cmd.AddCommand(down, up)
	return cmd
}

func newConfigCommand(ctx context.Context, opts *Options) *cobra.Command {
	cmd := newGroupCommand("config", "Manage configuration")
	validate := &cobra.Command{
		Use:   "validate",
		Short: "Validate config file",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigValidate(ctx, opts)
		},
	}
	diff := &cobra.Command{
		Use:   "diff [other-config]",
		Short: "Diff the effective config against another file (default: the base file alone)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			other := ""
			if len(args) > 0 {
				other = args[0]
			}
			return runConfigDiffAgainst(ctx, opts, other)
		},
	}
	show := &cobra.Command{
		Use:   "show",
		Short: "Show the effective config and where it came from",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigShow(ctx, opts)
		},
	}
	cmd.AddCommand(validate, diff, show, newConfigEditCommand(ctx, opts))
	return cmd
}

func newStateCommand(ctx context.Context, opts *Options) *cobra.Command {
	cmd := newGroupCommand("state", "Manage local state")
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
	cmd := newGroupCommand("token", "Manage local Agent credentials")
	apply := &cobra.Command{
		Use:   "apply [replacement-token]",
		Short: "Apply a replacement token without changing Agent identity",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.TokenApply = args[0]
			}
			if tok, terr := replacementTokenFromOptions(opts); terr == nil && tok == "" {
				return NewCommandError("could not apply replacement token", fmt.Errorf("replacement token is required"), "use: orion-agent token apply --token-file /path/to/token")
			}
			if handled, err := applyTokenViaDaemon(opts); handled {
				return err
			}
			if commandNeedsElevation("token", commandArgs(opts, "token")) {
				if err := ensurePrivilegeFor(opts, "token"); err != nil {
					return NewCommandError("could not elevate privileges", err)
				}
			}
			return runTokenApply(ctx, opts)
		},
	}
	apply.Flags().StringVar(&opts.TokenFile, "token-file", "", "Path to a file containing the replacement token")
	cmd.AddCommand(apply)
	return cmd
}

func newGroupCommand(use, short string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
}

func newReconfigureCommand(ctx context.Context, opts *Options) *cobra.Command {
	var token, tokenFile string
	cmd := &cobra.Command{
		Use:   "reconfigure",
		Short: "Reset local registration and reconnect using installed config",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if commandNeedsElevation("reconfigure", commandArgs(opts, "reconfigure")) {
				if err := ensurePrivilegeFor(opts, "reconfigure"); err != nil {
					return NewCommandError("could not elevate privileges", err)
				}
			}
			return runReconfigure(ctx, opts, token, tokenFile)
		},
	}
	cmd.Flags().StringVar(&token, "token", "", "Fresh one-time Add server token (prefer --token-file)")
	cmd.Flags().StringVar(&tokenFile, "token-file", "", "File containing a fresh one-time Add server token")
	cmd.MarkFlagsMutuallyExclusive("token", "token-file")
	return cmd
}

func configureLogging(opts *Options) {
	// Machine-readable output owns stdout; logs go to stderr.
	var logOut io.Writer = outputWriter
	if opts.JSON {
		logOut = os.Stderr
	}
	logging.ConfigureText(logOut)
	logging.SetTextColorEnabled(!opts.NoColor && writerSupportsColor(logOut))
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

	switch strings.ToLower(strings.TrimSpace(loggingConfig.Path)) {
	case "stdout", "stderr", "-", "none":
		// Containers and journald: keep console logging only.
		logging.SetLevel(level)
		return nil
	}

	err = logging.ConfigureFile(logging.FileConfig{
		Path:       loggingConfig.Path,
		Level:      level,
		MaxSizeMB:  loggingConfig.MaxSizeMB,
		MaxBackups: loggingConfig.MaxBackups,
		MaxAgeDays: loggingConfig.MaxAgeDays,
		Compress:   loggingConfig.CompressEnabled(),
	})
	if err != nil && errors.Is(err, os.ErrPermission) {
		// Zero-config runs as a normal user cannot write /var/log/orion; keep going on the console.
		logging.SetLevel(level)
		logging.Warnf("Cannot write log file %s (%v); logging to console only. Set logging.path or ORION_LOG_PATH.", loggingConfig.Path, err)
		return nil
	}
	return err
}
