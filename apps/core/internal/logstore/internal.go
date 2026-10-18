package logstore

import (
	"encoding/json"
	"io"
	"strings"
	"time"
)

// InternalAppID is the reserved app id under which Core stores its own log lines. It is not a row in
// apps: product searches skip it unless it is asked for by id, and the Internal logs screen asks for it.
const InternalAppID = "orion-internal"

// Components shown on the Internal logs screen.
const (
	ComponentAPI       = "core api"
	ComponentWorker    = "worker"
	ComponentScheduler = "scheduler"
	ComponentGateway   = "agent gateway"
)

// coreLogSink turns Core's JSON log lines into log entries. It implements io.Writer so a Logger can tee into it.
type coreLogSink struct {
	store   *Store
	process string // api | worker
	redact  *Redactor
}

// NewCoreLogSink records the process's own log lines in the log store (Internal logs). Writes never
// block: when the queue is full the line is dropped.
func NewCoreLogSink(store *Store, process string) io.Writer {
	return &coreLogSink{store: store, process: process, redact: NewRedactor(nil, false)}
}

func (c *coreLogSink) Write(p []byte) (int, error) {
	if entry, ok := c.entry(p); ok {
		_ = c.store.Enqueue([]Entry{entry})
	}
	return len(p), nil
}

func (c *coreLogSink) entry(line []byte) (Entry, bool) {
	var raw map[string]any
	if json.Unmarshal(line, &raw) != nil {
		return Entry{}, false
	}
	msg, _ := raw["msg"].(string)
	level, _ := raw["level"].(string)
	if msg == "" || msg == "Dropped a batch of log entries" { // the store reporting its own trouble must not feed itself
		return Entry{}, false
	}
	attrs := map[string]any{}
	for k, v := range raw {
		switch k {
		case "time", "msg", "level":
		default:
			attrs[k] = v
		}
	}
	if msg == "HTTP request" && !noteworthyRequest(attrs) {
		return Entry{}, false
	}
	now := time.Now().UTC()
	at := now
	if text, _ := raw["time"].(string); text != "" {
		if t, err := time.Parse(time.RFC3339Nano, text); err == nil {
			at = t.UTC()
		}
	}
	severity, number := NormalizeSeverity(strings.ToLower(level))
	if _, hasComponent := attrs["component"]; !hasComponent {
		attrs["component"] = classifyComponent(c.process, msg, attrs)
	}
	attrs["process"] = c.process
	entry := Entry{ID: NewEntryID(now), AppID: InternalAppID, Timestamp: at, IngestedAt: now, Severity: severity, SeverityNumber: number, Body: msg, Attributes: attrs, Size: len(line)}
	c.redact.Apply(&entry)
	return entry, true
}

// noteworthyRequest keeps failed or slow HTTP requests; routine successes would drown everything else
// (and every read of this screen would log itself).
func noteworthyRequest(attrs map[string]any) bool {
	if status, ok := attrs["status"].(float64); ok && status >= 400 {
		return true
	}
	if ms, ok := attrs["duration_ms"].(float64); ok && ms >= 1000 {
		return true
	}
	return false
}

func classifyComponent(process, msg string, attrs map[string]any) string {
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "scheduler") || strings.Contains(lower, "lifecycle") || strings.Contains(lower, "retention") || strings.Contains(lower, "rollup") || strings.Contains(lower, "archive"):
		return ComponentScheduler
	case process == "worker" || strings.Contains(lower, "alert delivery") || strings.Contains(lower, "core worker") || strings.Contains(lower, "dispatch"):
		return ComponentWorker
	}
	if route, _ := attrs["route"].(string); strings.HasPrefix(route, "/v1/agents") || strings.HasPrefix(route, "/v1/ingest") || strings.HasPrefix(route, "/v1/register") {
		return ComponentGateway
	}
	if strings.Contains(lower, "agent") && (strings.Contains(lower, "report") || strings.Contains(lower, "token") || strings.Contains(lower, "register")) {
		return ComponentGateway
	}
	return ComponentAPI
}
