package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	agent "orion/agent/internal"
	"orion/agent/internal/config"
	agentstate "orion/agent/internal/state"

	"github.com/spf13/cobra"
)

type Options struct {
	ConfigPath string
	StatePath  string
	Verbose    bool
	NoColor    bool
	JSON       bool

	Once          bool
	UpdateVersion string
	UpdateRepo    string
	LogLines      int

	normalizedArgs []string
}

type CommandError struct {
	Summary   string
	Cause     error
	NextSteps []string
	ExitCode  int
}

func (e *CommandError) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause == nil {
		return e.Summary
	}
	return fmt.Sprintf("%s: %v", e.Summary, e.Cause)
}

func (e *CommandError) code() int {
	if e == nil || e.ExitCode == 0 {
		return 1
	}
	return e.ExitCode
}

func NewCommandError(summary string, cause error, nextSteps ...string) *CommandError {
	return &CommandError{
		Summary:   summary,
		Cause:     cause,
		NextSteps: nextSteps,
		ExitCode:  1,
	}
}

func Execute(ctx context.Context, args []string, out, errOut io.Writer) int {
	opts := &Options{
		ConfigPath:    config.DefaultPath(),
		StatePath:     agentstate.DefaultPath(),
		UpdateVersion: "latest",
		UpdateRepo:    "sunday-studio/orion",
		LogLines:      80,
	}
	normalizedArgs := NormalizeLegacyArgs(args)
	opts.normalizedArgs = normalizedArgs

	cmd := NewRootCommand(ctx, opts, out, errOut)
	cmd.SetArgs(normalizedArgs)
	SetOutput(out)

	if len(normalizedArgs) == 0 {
		_ = cmd.Help()
		return 1
	}

	if err := cmd.Execute(); err != nil {
		renderCommandError(errOut, err)
		return commandExitCode(err)
	}
	return 0
}

func NewRootCommand(ctx context.Context, opts *Options, out, errOut io.Writer) *cobra.Command {
	root := &cobra.Command{
		Use:           "orion-agent",
		Short:         "Orion Agent CLI",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			SetOutput(out)
			SetColorEnabled(!opts.NoColor && writerSupportsColor(out))
			configureLogging(opts)
		},
	}
	root.SetOut(out)
	root.SetErr(errOut)

	root.PersistentFlags().StringVar(&opts.ConfigPath, "config", config.DefaultPath(), "Path to config file")
	root.PersistentFlags().StringVar(&opts.StatePath, "state", agentstate.DefaultPath(), "Path to SQLite state database")
	root.PersistentFlags().BoolVar(&opts.Verbose, "verbose", false, "Enable debug logging")
	root.PersistentFlags().BoolVar(&opts.NoColor, "no-color", false, "Disable ANSI color output")
	root.PersistentFlags().BoolVar(&opts.JSON, "json", false, "Use machine-readable output where supported")

	root.AddCommand(
		newVersionCommand(),
		newStartCommand(ctx, opts),
		newStopCommand(ctx, opts),
		newStatusCommand(ctx, opts),
		newRestartCommand(ctx, opts),
		newLogsCommand(ctx, opts),
		newUpdateCommand(ctx, opts),
		newRunCommand(ctx, opts),
		newMaintenanceCommand(ctx, opts),
		newConfigCommand(ctx, opts),
		newStateCommand(ctx, opts),
		newReconfigureCommand(ctx, opts),
	)

	return root
}

func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print agent version",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "orion-agent %s\n", agent.Version)
		},
	}
}

func commandArgs(opts *Options, command string) []string {
	for i, arg := range opts.normalizedArgs {
		if arg == command {
			commandArgs := make([]string, 0, len(opts.normalizedArgs)-1)
			commandArgs = append(commandArgs, opts.normalizedArgs[:i]...)
			commandArgs = append(commandArgs, opts.normalizedArgs[i+1:]...)
			return commandArgs
		}
	}
	return nil
}

func ensurePrivilegeFor(opts *Options, command string) error {
	return EnsureCommandPrivilege(command, commandArgs(opts, command))
}

func NormalizeLegacyArgs(args []string) []string {
	normalized := make([]string, 0, len(args))
	for i, arg := range args {
		if i == 0 && (arg == "-v" || arg == "--version") {
			normalized = append(normalized, "version")
			continue
		}
		if len(normalized) > 0 && normalized[0] == "maintenance" {
			switch arg {
			case "-up":
				normalized = append(normalized, "up")
				continue
			case "-down":
				normalized = append(normalized, "down")
				continue
			}
		}

		replacement := normalizeLegacyFlag(arg)
		normalized = append(normalized, replacement)
	}
	return normalized
}

func normalizeLegacyFlag(arg string) string {
	legacyFlags := []string{"config", "state", "once", "verbose", "version", "repo", "lines"}
	for _, name := range legacyFlags {
		short := "-" + name
		if arg == short {
			return "--" + name
		}
		if strings.HasPrefix(arg, short+"=") {
			return "--" + name + strings.TrimPrefix(arg, short)
		}
	}
	return arg
}

func renderCommandError(w io.Writer, err error) {
	if w == nil {
		w = os.Stderr
	}

	var commandErr *CommandError
	if errors.As(err, &commandErr) {
		fmt.Fprintf(w, "error: %s\n", commandErr.Summary)
		if commandErr.Cause != nil {
			fmt.Fprintf(w, "\ncause:\n  %v\n", commandErr.Cause)
		}
		if len(commandErr.NextSteps) > 0 {
			fmt.Fprintln(w, "\nnext steps:")
			for _, step := range commandErr.NextSteps {
				fmt.Fprintf(w, "  %s\n", step)
			}
		}
		return
	}

	fmt.Fprintf(w, "error: %v\n", err)
}

func commandExitCode(err error) int {
	var commandErr *CommandError
	if errors.As(err, &commandErr) {
		return commandErr.code()
	}
	return 1
}
