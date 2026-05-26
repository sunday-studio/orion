package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
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

func normalizeLogLevel(level string) (string, error) {
	normalized := strings.ToUpper(strings.TrimSpace(level))
	switch normalized {
	case "DEBUG", "INFO", "WARN", "WARNING", "ERROR":
		if normalized == "WARNING" {
			return "WARN", nil
		}
		return normalized, nil
	default:
		return "", fmt.Errorf("unknown log level %q; expected debug, info, warn, or error", level)
	}
}

func parseLogSince(value string, now func() time.Time) (time.Time, error) {
	if duration, err := time.ParseDuration(value); err == nil {
		return now().Add(-duration), nil
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
	} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("invalid --since value %q; use a duration like 1h or an RFC3339 timestamp", value)
}

func readJSONLLogFile(path string, filters logFilters) ([]logEntry, int, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, 0, fmt.Errorf("read Orion log file %s: %w", path, err)
	}
	defer file.Close()

	var entries []logEntry
	skipped := 0
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		entry, err := parseLogEntry(line)
		if err != nil {
			skipped++
			continue
		}
		if !logEntryMatches(entry, filters) {
			continue
		}
		entries = append(entries, entry)
		if filters.limit > 0 && len(entries) > filters.limit {
			copy(entries, entries[len(entries)-filters.limit:])
			entries = entries[:filters.limit]
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, skipped, fmt.Errorf("scan Orion log file %s: %w", path, err)
	}
	return entries, skipped, nil
}

func parseLogEntry(line string) (logEntry, error) {
	var values map[string]any
	if err := json.Unmarshal([]byte(line), &values); err != nil {
		return logEntry{}, err
	}

	var when *time.Time
	if value, ok := stringLogValue(values, "time"); ok {
		if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
			when = &parsed
		}
	}

	return logEntry{raw: line, values: values, when: when}, nil
}

func logEntryMatches(entry logEntry, filters logFilters) bool {
	if filters.since != nil {
		if entry.when == nil || entry.when.Before(*filters.since) {
			return false
		}
	}
	if filters.level != "" {
		level, ok := stringLogValue(entry.values, "level")
		if !ok || strings.ToUpper(level) != filters.level {
			return false
		}
	}
	if filters.component != "" {
		component, ok := stringLogValue(entry.values, "component")
		if !ok || component != filters.component {
			return false
		}
	}
	if filters.monitor != "" {
		monitor, ok := firstStringLogValue(entry.values, "monitor", "monitor_name")
		if !ok || monitor != filters.monitor {
			return false
		}
	}
	return true
}

func formatLogEntry(entry logEntry) string {
	timestamp := "-"
	if entry.when != nil {
		timestamp = entry.when.Local().Format("2006-01-02 15:04:05")
	}
	level, _ := stringLogValue(entry.values, "level")
	if level == "" {
		level = "INFO"
	}
	level = strings.ToUpper(level)
	component, _ := stringLogValue(entry.values, "component")
	message, _ := firstStringLogValue(entry.values, "message", "msg")

	parts := []string{timestamp, level}
	if component != "" {
		parts = append(parts, component)
	}
	if message != "" {
		parts = append(parts, message)
	}

	fields := formatLogFields(entry.values)
	if fields != "" {
		parts = append(parts, fields)
	}
	return strings.Join(parts, " ")
}

func formatLogFields(values map[string]any) string {
	hidden := map[string]bool{
		"time":      true,
		"level":     true,
		"component": true,
		"message":   true,
		"msg":       true,
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		if !hidden[key] {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)

	fields := make([]string, 0, len(keys))
	for _, key := range keys {
		fields = append(fields, fmt.Sprintf("%s=%s", key, formatLogValue(values[key])))
	}
	return strings.Join(fields, " ")
}

func formatLogValue(value any) string {
	switch v := value.(type) {
	case string:
		if strings.ContainsAny(v, " \t\n\"") {
			return strconv.Quote(v)
		}
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	case nil:
		return "null"
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprint(v)
		}
		return string(data)
	}
}

func firstStringLogValue(values map[string]any, keys ...string) (string, bool) {
	for _, key := range keys {
		if value, ok := stringLogValue(values, key); ok {
			return value, true
		}
	}
	return "", false
}

func stringLogValue(values map[string]any, key string) (string, bool) {
	value, ok := values[key]
	if !ok {
		return "", false
	}
	switch v := value.(type) {
	case string:
		return v, true
	default:
		return fmt.Sprint(v), true
	}
}
