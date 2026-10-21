package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"orion/core/internal/db"
	"strings"
)

// describeMonitorReport turns a stored check payload into the drawer's one-liner, e.g.
// "Timeout, 10,000 ms" or "200 OK, 201 ms", plus duration and who reported it.
func describeMonitorReport(report db.MonitorReport) (summary string, durationMs *int64, reportedBy string) {
	var payload map[string]any
	if json.Unmarshal([]byte(report.Payload), &payload) != nil {
		return "", nil, ""
	}
	if v, ok := payload["duration_ms"].(float64); ok {
		d := int64(v)
		durationMs = &d
	}
	switch runner, _ := payload["runner"].(string); runner {
	case "core":
		reportedBy = "Core worker"
	case "heartbeat":
		reportedBy = "Heartbeat ping"
	default:
		reportedBy = "Node"
	}
	var parts []string
	if msg, ok := payload["error"].(string); ok && strings.TrimSpace(msg) != "" {
		parts = append(parts, shortError(msg))
	} else if code, ok := payload["status_code"].(float64); ok && code > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", int(code), http.StatusText(int(code))))
	} else if report.Health == "up" {
		parts = append(parts, "OK")
	} else {
		parts = append(parts, strings.ToUpper(report.Health[:1])+report.Health[1:])
	}
	if durationMs != nil {
		parts = append(parts, formatThousands(*durationMs)+" ms")
	}
	return strings.Join(parts, ", "), durationMs, reportedBy
}

// shortError reduces a Go-style error to a label: "context deadline exceeded" and i/o timeouts become "Timeout".
func shortError(msg string) string {
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "deadline exceeded") || strings.Contains(lower, "timeout") || strings.Contains(lower, "timed out"):
		return "Timeout"
	case strings.Contains(lower, "connection refused"):
		return "Connection refused"
	case strings.Contains(lower, "no such host"):
		return "DNS lookup failed"
	case strings.Contains(lower, "certificate"):
		return "TLS certificate error"
	}
	if len(msg) > 80 {
		return msg[:80]
	}
	return msg
}

func formatThousands(n int64) string {
	s := fmt.Sprintf("%d", n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}
