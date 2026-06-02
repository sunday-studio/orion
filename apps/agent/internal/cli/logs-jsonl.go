package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

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
