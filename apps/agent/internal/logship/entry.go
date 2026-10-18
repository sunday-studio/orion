// Package logship tails log files, journald and containers on this host and
// ships them to Core as product-log entries (docs/plans/product-logs-plan.md).
// Reading is decoupled from sending by an on-disk spool, so a Core outage
// never stalls or loses reading, and the CLI can show recent logs offline.
package logship

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// MaxLineBytes caps one log entry body; the rest of a longer line is dropped.
const MaxLineBytes = 64 * 1024

// Entry is the product-log envelope (OpenTelemetry log data model, subset).
type Entry struct {
	ID         string         `json:"id"`
	Timestamp  string         `json:"timestamp,omitempty"`
	Severity   string         `json:"severity,omitempty"`
	Body       string         `json:"body"`
	Event      string         `json:"event,omitempty"`
	Attributes map[string]any `json:"attributes,omitempty"`
	Resource   map[string]any `json:"resource,omitempty"`
	// Source names the configured source; kept in the spool to pick the ingest
	// key, and removed before sending.
	Source string `json:"_source,omitempty"`
}

// entryID is a stable idempotency key: the same bytes at the same position of
// the same file always get the same id, so replays are harmless.
func entryID(source, fileID string, offset int64) string {
	sum := sha1.Sum([]byte(fmt.Sprintf("%s|%s|%d", source, fileID, offset)))
	return hex.EncodeToString(sum[:12])
}

var severityWord = regexp.MustCompile(`(?i)\b(trace|debug|info|notice|warn|warning|error|err|fatal|crit|critical|panic)\b`)

func normalizeSeverity(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "trace":
		return "trace"
	case "debug":
		return "debug"
	case "info", "notice", "information":
		return "info"
	case "warn", "warning":
		return "warn"
	case "error", "err":
		return "error"
	case "fatal", "crit", "critical", "panic", "emerg", "alert":
		return "fatal"
	}
	return ""
}

// parseLine turns one (already multiline-joined) log line into an entry body,
// severity and attributes. JSON lines keep their structure; plain text is
// scanned for a severity word near the start.
func parseLine(line string) (body, severity string, attrs map[string]any, ts string, event string) {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
		var m map[string]any
		if json.Unmarshal([]byte(trimmed), &m) == nil {
			take := func(keys ...string) string {
				for _, k := range keys {
					if v, ok := m[k]; ok {
						delete(m, k)
						if s, ok := v.(string); ok {
							return s
						}
						return fmt.Sprint(v)
					}
				}
				return ""
			}
			body = take("msg", "message", "body")
			severity = normalizeSeverity(take("level", "severity", "lvl"))
			ts = take("time", "timestamp", "ts", "@timestamp")
			event = take("event", "event_name")
			if body == "" {
				body = trimmed
			}
			if len(m) > 0 {
				attrs = m
			}
			return
		}
	}
	body = line
	head := line
	if len(head) > 80 {
		head = head[:80]
	}
	if m := severityWord.FindString(head); m != "" {
		severity = normalizeSeverity(m)
	}
	return
}

// Redactor masks secrets at the edge, before anything touches the disk spool.
type Redactor struct{ patterns []*regexp.Regexp }

var defaultRedactions = []string{
	`(?i)((?:password|passwd|secret|token|api[_-]?key|authorization|cookie)["']?\s*[:=]\s*["']?)[^"'\s,;}&]+`,
	`(?i)(bearer\s+)[A-Za-z0-9._~+/=-]+`,
}

// NewRedactor compiles the defaults plus any custom regexes. A custom regex
// with a capture group keeps group 1 and masks the rest; without one it masks
// the whole match.
func NewRedactor(custom []string) (*Redactor, error) {
	r := &Redactor{}
	for _, p := range append(append([]string{}, defaultRedactions...), custom...) {
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, fmt.Errorf("invalid redaction pattern %q: %w", p, err)
		}
		r.patterns = append(r.patterns, re)
	}
	return r, nil
}

// Apply masks secrets in s.
func (r *Redactor) Apply(s string) string {
	if r == nil {
		return s
	}
	for _, re := range r.patterns {
		if re.NumSubexp() >= 1 {
			s = re.ReplaceAllString(s, "${1}[redacted]")
		} else {
			s = re.ReplaceAllString(s, "[redacted]")
		}
	}
	return s
}

var sensitiveKey = regexp.MustCompile(`(?i)(password|passwd|secret|token|api[_-]?key|authorization|cookie)`)

// redactAttrs masks values of sensitive keys, recursively.
func redactAttrs(m map[string]any, r *Redactor) {
	for k, v := range m {
		switch t := v.(type) {
		case string:
			if sensitiveKey.MatchString(k) {
				m[k] = "[redacted]"
			} else {
				m[k] = r.Apply(t)
			}
		case map[string]any:
			redactAttrs(t, r)
		}
	}
}

func nowRFC3339(now func() time.Time) string { return now().UTC().Format(time.RFC3339Nano) }
