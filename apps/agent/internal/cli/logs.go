package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"orion/agent/internal/config"

	"github.com/spf13/cobra"
)

const defaultLogLines = 80

type LogViewOptions struct {
	File      string
	Source    string
	Lines     int
	Since     string
	Level     string
	Component string
	Monitor   string
	JSON      bool
	Out       io.Writer
	ErrOut    io.Writer
	Fallback  func(lines int)
	Now       func() time.Time
}

type logFilters struct {
	since     *time.Time
	level     string
	component string
	monitor   string
	limit     int
}

type logEntry struct {
	raw    string
	values map[string]any
	when   *time.Time
}

func DefaultAgentLogPath() string {
	return config.DefaultLogPath()
}

func newLogsCommand(ctx context.Context, opts *Options) *cobra.Command {
	var logFile string
	var source string
	var since string
	var level string
	var component string
	var monitor string

	command := &cobra.Command{
		Use:   "logs",
		Short: "Show Orion Agent logs",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = ctx
			return ViewLogs(LogViewOptions{
				File:      logFile,
				Source:    source,
				Lines:     opts.LogLines,
				Since:     since,
				Level:     level,
				Component: component,
				Monitor:   monitor,
				JSON:      opts.JSON,
				Out:       cmd.OutOrStdout(),
				ErrOut:    cmd.ErrOrStderr(),
				Fallback:  PrintServiceDiagnostics,
			})
		},
	}

	command.Flags().IntVar(&opts.LogLines, "lines", defaultLogLines, "Number of log lines to show")
	command.Flags().StringVar(&since, "since", "", "Only show logs after a duration or timestamp")
	command.Flags().StringVar(&level, "level", "", "Only show logs at a level")
	command.Flags().StringVar(&component, "component", "", "Only show logs for a component")
	command.Flags().StringVar(&monitor, "monitor", "", "Only show logs for a monitor")
	command.Flags().StringVar(&logFile, "file", DefaultAgentLogPath(), "Path to Orion JSONL log file")
	command.Flags().StringVar(&source, "source", "auto", "Log source: auto, file, or system")

	return command
}

func HandleLogs(args []string, defaultLinesCount int) {
	options, err := ParseLogViewOptions(args, defaultLinesCount)
	if err != nil {
		if errors.Is(err, errLogsHelp) {
			printLogsUsage()
			os.Exit(0)
		}
		fmt.Println(err)
		printLogsUsage()
		os.Exit(1)
	}
	options.Out = os.Stdout
	options.ErrOut = os.Stderr
	options.Fallback = PrintServiceDiagnostics

	if err := ViewLogs(options); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func ParseLogViewOptions(args []string, defaultLinesCount int) (LogViewOptions, error) {
	if defaultLinesCount <= 0 {
		defaultLinesCount = defaultLogLines
	}

	options := LogViewOptions{
		File:   DefaultAgentLogPath(),
		Source: "auto",
		Lines:  defaultLinesCount,
	}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-lines" || arg == "--lines":
			value, ok := nextLogArg(args, &i)
			if !ok {
				return options, fmt.Errorf("%s requires a number", arg)
			}
			lines, err := strconv.Atoi(value)
			if err != nil || lines < 0 {
				return options, fmt.Errorf("%s must be a non-negative number", arg)
			}
			options.Lines = lines
		case strings.HasPrefix(arg, "-lines="):
			lines, err := strconv.Atoi(strings.TrimPrefix(arg, "-lines="))
			if err != nil || lines < 0 {
				return options, fmt.Errorf("-lines must be a non-negative number")
			}
			options.Lines = lines
		case strings.HasPrefix(arg, "--lines="):
			lines, err := strconv.Atoi(strings.TrimPrefix(arg, "--lines="))
			if err != nil || lines < 0 {
				return options, fmt.Errorf("--lines must be a non-negative number")
			}
			options.Lines = lines
		case arg == "-since" || arg == "--since":
			value, ok := nextLogArg(args, &i)
			if !ok {
				return options, fmt.Errorf("%s requires a duration or timestamp", arg)
			}
			options.Since = value
		case strings.HasPrefix(arg, "-since="):
			options.Since = strings.TrimPrefix(arg, "-since=")
		case strings.HasPrefix(arg, "--since="):
			options.Since = strings.TrimPrefix(arg, "--since=")
		case arg == "-level" || arg == "--level":
			value, ok := nextLogArg(args, &i)
			if !ok {
				return options, fmt.Errorf("%s requires a level", arg)
			}
			options.Level = value
		case strings.HasPrefix(arg, "-level="):
			options.Level = strings.TrimPrefix(arg, "-level=")
		case strings.HasPrefix(arg, "--level="):
			options.Level = strings.TrimPrefix(arg, "--level=")
		case arg == "-component" || arg == "--component":
			value, ok := nextLogArg(args, &i)
			if !ok {
				return options, fmt.Errorf("%s requires a component", arg)
			}
			options.Component = value
		case strings.HasPrefix(arg, "-component="):
			options.Component = strings.TrimPrefix(arg, "-component=")
		case strings.HasPrefix(arg, "--component="):
			options.Component = strings.TrimPrefix(arg, "--component=")
		case arg == "-monitor" || arg == "--monitor":
			value, ok := nextLogArg(args, &i)
			if !ok {
				return options, fmt.Errorf("%s requires a monitor", arg)
			}
			options.Monitor = value
		case strings.HasPrefix(arg, "-monitor="):
			options.Monitor = strings.TrimPrefix(arg, "-monitor=")
		case strings.HasPrefix(arg, "--monitor="):
			options.Monitor = strings.TrimPrefix(arg, "--monitor=")
		case arg == "-file" || arg == "--file":
			value, ok := nextLogArg(args, &i)
			if !ok {
				return options, fmt.Errorf("%s requires a path", arg)
			}
			options.File = value
		case strings.HasPrefix(arg, "-file="):
			options.File = strings.TrimPrefix(arg, "-file=")
		case strings.HasPrefix(arg, "--file="):
			options.File = strings.TrimPrefix(arg, "--file=")
		case arg == "-source" || arg == "--source":
			value, ok := nextLogArg(args, &i)
			if !ok {
				return options, fmt.Errorf("%s requires auto, file, or system", arg)
			}
			options.Source = value
		case strings.HasPrefix(arg, "-source="):
			options.Source = strings.TrimPrefix(arg, "-source=")
		case strings.HasPrefix(arg, "--source="):
			options.Source = strings.TrimPrefix(arg, "--source=")
		case arg == "-json" || arg == "--json":
			options.JSON = true
		case arg == "-h" || arg == "--help":
			return options, errLogsHelp
		default:
			return options, fmt.Errorf("unknown logs option: %s", arg)
		}
	}

	return options, nil
}

func ViewLogs(options LogViewOptions) error {
	if options.Out == nil {
		options.Out = io.Discard
	}
	if options.ErrOut == nil {
		options.ErrOut = io.Discard
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	if options.Source == "" {
		options.Source = "auto"
	}
	if options.File == "" {
		options.File = DefaultAgentLogPath()
	}
	if options.Lines < 0 {
		return fmt.Errorf("lines must be non-negative")
	}
	if err := validateLogSource(options.Source); err != nil {
		return err
	}
	if options.Source == "system" {
		if options.JSON {
			return fmt.Errorf("--json requires Orion JSONL logs; system log fallback is not JSONL")
		}
		if options.Fallback == nil {
			return fmt.Errorf("system log fallback is unavailable")
		}
		options.Fallback(options.Lines)
		return nil
	}

	filters, err := buildLogFilters(options)
	if err != nil {
		return err
	}

	entries, skipped, err := readJSONLLogFile(options.File, filters)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && options.Source == "auto" && !options.JSON {
			fmt.Fprintf(options.ErrOut, "warning: Orion log file not found at %s\n", options.File)
			if options.Fallback != nil {
				fmt.Fprintln(options.ErrOut, "showing service-manager log fallback")
				options.Fallback(options.Lines)
				return nil
			}
		}
		if errors.Is(err, os.ErrNotExist) && options.JSON {
			return fmt.Errorf("--json requires Orion JSONL logs; no JSONL log file found at %s", options.File)
		}
		return err
	}
	if skipped > 0 {
		fmt.Fprintf(options.ErrOut, "warning: skipped %d malformed log line(s)\n", skipped)
	}

	if len(entries) == 0 {
		if !options.JSON {
			fmt.Fprintln(options.Out, "no matching log entries")
		}
		return nil
	}

	for _, entry := range entries {
		if options.JSON {
			fmt.Fprintln(options.Out, entry.raw)
			continue
		}
		fmt.Fprintln(options.Out, formatLogEntry(entry))
	}

	return nil
}

var errLogsHelp = errors.New("logs help requested")

func printLogsUsage() {
	fmt.Println("Usage: orion-agent logs [-lines N] [--since DURATION|TIMESTAMP] [--level LEVEL] [--component NAME] [--monitor NAME] [--json] [--file PATH] [--source auto|file|system]")
}

func nextLogArg(args []string, index *int) (string, bool) {
	next := *index + 1
	if next >= len(args) {
		return "", false
	}
	*index = next
	return args[next], true
}

func validateLogSource(source string) error {
	switch source {
	case "auto", "file", "system":
		return nil
	default:
		return fmt.Errorf("unknown log source %q; expected auto, file, or system", source)
	}
}

func buildLogFilters(options LogViewOptions) (logFilters, error) {
	filters := logFilters{
		component: options.Component,
		monitor:   options.Monitor,
		limit:     options.Lines,
	}

	if options.Level != "" {
		level, err := normalizeLogLevel(options.Level)
		if err != nil {
			return filters, err
		}
		filters.level = level
	}
	if options.Since != "" {
		since, err := parseLogSince(options.Since, options.Now)
		if err != nil {
			return filters, err
		}
		filters.since = &since
	}
	return filters, nil
}
