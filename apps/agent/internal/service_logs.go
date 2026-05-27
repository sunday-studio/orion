package agent

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"orion/agent/internal/config"
	"orion/agent/internal/transport"
)

const serviceLogBatchLimit = 100

func (a *Agent) shipServiceLogs() error {
	if strings.TrimSpace(a.userConfig.Logging.Path) == "" {
		return nil
	}
	entries, err := collectServiceLogEntries(a.userConfig.Logging.Path, a.internalState.Monitors, serviceLogBatchLimit)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if len(entries) == 0 {
		return nil
	}
	return a.transport.SendServiceLogs(transport.ServiceLogBatch{Entries: entries}, a.internalState.AgentID)
}

func collectServiceLogEntries(path string, monitors []config.InternalStateMonitor, limit int) ([]transport.ServiceLogEntry, error) {
	if limit <= 0 {
		return nil, nil
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read service log file %s: %w", path, err)
	}
	defer file.Close()

	monitorIDs := map[string]string{}
	for _, monitor := range monitors {
		if monitor.Name != "" && monitor.ID != "" {
			monitorIDs[monitor.Name] = monitor.ID
		}
	}

	entries := make([]transport.ServiceLogEntry, 0, limit)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		entry, ok := serviceLogEntryFromLine(line, monitorIDs)
		if !ok {
			continue
		}
		entries = append(entries, entry)
		if len(entries) > limit {
			copy(entries, entries[len(entries)-limit:])
			entries = entries[:limit]
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan service log file %s: %w", path, err)
	}
	return entries, nil
}

func serviceLogEntryFromLine(line string, monitorIDs map[string]string) (transport.ServiceLogEntry, bool) {
	var values map[string]any
	if err := json.Unmarshal([]byte(line), &values); err != nil {
		return transport.ServiceLogEntry{}, false
	}

	timestamp, _ := stringServiceLogValue(values, "time")
	if timestamp == "" {
		timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	}

	level, _ := stringServiceLogValue(values, "level")
	component, _ := stringServiceLogValue(values, "component")
	message, _ := firstStringServiceLogValue(values, "message", "msg")
	monitorName, _ := firstStringServiceLogValue(values, "monitor_name", "monitor")

	return transport.ServiceLogEntry{
		Timestamp:   timestamp,
		Source:      "agent",
		Stream:      "jsonl",
		Level:       strings.ToUpper(level),
		Component:   component,
		MonitorName: monitorName,
		MonitorID:   monitorIDs[monitorName],
		Message:     redactServiceLogText(message),
		Fields:      serviceLogFields(values),
		Fingerprint: serviceLogFingerprint(line),
	}, true
}

func serviceLogFields(values map[string]any) map[string]any {
	hidden := map[string]bool{
		"time":         true,
		"level":        true,
		"component":    true,
		"message":      true,
		"msg":          true,
		"monitor":      true,
		"monitor_name": true,
	}
	fields := map[string]any{}
	for key, value := range values {
		if hidden[key] {
			continue
		}
		if isSensitiveServiceLogField(key) {
			fields[key] = "[redacted]"
			continue
		}
		fields[key] = value
	}
	if len(fields) == 0 {
		return nil
	}
	return fields
}

func serviceLogFingerprint(line string) string {
	sum := sha256.Sum256([]byte(line))
	return hex.EncodeToString(sum[:])
}

func firstStringServiceLogValue(values map[string]any, keys ...string) (string, bool) {
	for _, key := range keys {
		if value, ok := stringServiceLogValue(values, key); ok {
			return value, true
		}
	}
	return "", false
}

func stringServiceLogValue(values map[string]any, key string) (string, bool) {
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

func isSensitiveServiceLogField(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(key, "_", ""))
	for _, marker := range []string{"token", "secret", "password", "apikey", "authorization"} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

var serviceLogSecretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(token|secret|password|api[_-]?key|authorization)(\s*[:=]\s*)([^\s,;]+)`),
	regexp.MustCompile(`(?i)(bearer\s+)([A-Za-z0-9._~+/=-]+)`),
}

func redactServiceLogText(value string) string {
	redacted := value
	for _, pattern := range serviceLogSecretPatterns {
		redacted = pattern.ReplaceAllString(redacted, `${1}${2}[redacted]`)
	}
	return redacted
}
