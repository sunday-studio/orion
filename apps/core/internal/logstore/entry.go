// Package logstore implements product logs: normalisation of schema-free entries,
// redaction, a bounded asynchronous writer into a separate logs.db, queries and
// retention. See docs/product-logs.md and docs/plans/product-logs-plan.md.
package logstore

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"
)

const (
	MaxEntryBytes     = 64 << 10
	MaxAttributeDepth = 8
	MaxAttributeKeys  = 128
	maxFutureSkew     = time.Hour
)

// Entry is a normalised log record, ready to store.
type Entry struct {
	ID             string
	AppID          string
	ClientID       string
	Timestamp      time.Time
	IngestedAt     time.Time
	Severity       string
	SeverityNumber int
	Event          string
	Body           string
	BodyJSON       string // raw JSON when the body was structured, else empty
	Attributes     map[string]any
	Resource       map[string]any
	TraceID        string
	SpanID         string
	RequestID      string
	Size           int
}

// EntryError explains why one entry was rejected.
type EntryError struct {
	Index   int    `json:"index"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *EntryError) Error() string { return e.Code + ": " + e.Message }

var severityByName = map[string]int{
	"trace": 1, "debug": 5, "info": 9, "warn": 13, "error": 17, "fatal": 21,
}

var severityAliases = map[string]string{
	"verbose": "trace", "silly": "trace", "finest": "trace", "debug2": "debug", "dbg": "debug",
	"information": "info", "informational": "info", "notice": "info", "log": "info", "ok": "info",
	"warning": "warn", "wrn": "warn", "err": "error", "severe": "error", "exception": "error",
	"critical": "fatal", "crit": "fatal", "alert": "fatal", "emerg": "fatal", "emergency": "fatal", "panic": "fatal",
}

// NormalizeSeverity maps text, OpenTelemetry numbers (1-24) and aliases to the
// canonical severity and its OTel base number. Unknown values become info.
func NormalizeSeverity(v any) (string, int) {
	switch value := v.(type) {
	case string:
		name := strings.ToLower(strings.TrimSpace(value))
		if alias, ok := severityAliases[name]; ok {
			name = alias
		}
		if number, ok := severityByName[name]; ok {
			return name, number
		}
		var asNumber float64
		if _, err := fmt.Sscanf(name, "%f", &asNumber); err == nil {
			return NormalizeSeverity(asNumber)
		}
	case int64:
		return NormalizeSeverity(float64(value))
	case float64:
		switch n := int(value); {
		case n >= 1 && n <= 4:
			return "trace", n
		case n >= 5 && n <= 8:
			return "debug", n
		case n >= 9 && n <= 12:
			return "info", n
		case n >= 13 && n <= 16:
			return "warn", n
		case n >= 17 && n <= 20:
			return "error", n
		case n >= 21 && n <= 24:
			return "fatal", n
		}
	case json.Number:
		if f, err := value.Float64(); err == nil {
			return NormalizeSeverity(f)
		}
	}
	return "info", severityByName["info"]
}

// SeverityNumbers returns the OTel base numbers for a comma-separated severity filter,
// or a minimum such as ">=warn".
func SeverityRange(filter string) (min int, max int, ok bool) {
	filter = strings.ToLower(strings.TrimSpace(filter))
	if strings.HasPrefix(filter, ">=") {
		name, _ := NormalizeSeverity(strings.TrimPrefix(filter, ">="))
		return severityByName[name], 24, true
	}
	return 0, 0, false
}

// parseTimestamp accepts RFC 3339 text or a Unix time as seconds, milliseconds,
// microseconds or nanoseconds (told apart by magnitude).
func parseTimestamp(v any) (time.Time, error) {
	switch value := v.(type) {
	case string:
		value = strings.TrimSpace(value)
		for _, layout := range []string{time.RFC3339Nano, "2006-01-02T15:04:05", "2006-01-02 15:04:05.999999999Z07:00", "2006-01-02 15:04:05"} {
			if t, err := time.Parse(layout, value); err == nil {
				return t.UTC(), nil
			}
		}
		var f float64
		if _, err := fmt.Sscanf(value, "%f", &f); err == nil && value != "" {
			return fromUnix(f), nil
		}
	case int64:
		return fromUnixInt(value), nil
	case float64:
		return fromUnix(value), nil
	case json.Number:
		if f, err := value.Float64(); err == nil {
			return fromUnix(f), nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognised timestamp %v", v)
}

// fromUnixInt keeps nanosecond precision, which float64 cannot for epoch-nanosecond values.
func fromUnixInt(i int64) time.Time {
	abs := i
	if abs < 0 {
		abs = -abs
	}
	switch {
	case abs < 1e11:
		return time.Unix(i, 0).UTC()
	case abs < 1e14:
		return time.UnixMilli(i).UTC()
	case abs < 1e17:
		return time.UnixMicro(i).UTC()
	default:
		return time.Unix(0, i).UTC()
	}
}

func fromUnix(f float64) time.Time {
	abs := math.Abs(f)
	switch {
	case abs < 1e11:
		return time.Unix(int64(f), int64((f-math.Trunc(f))*1e9)).UTC()
	case abs < 1e14:
		return time.UnixMilli(int64(f)).UTC()
	case abs < 1e17:
		return time.UnixMicro(int64(f)).UTC()
	default:
		return time.Unix(0, int64(f)).UTC()
	}
}

// Normalize turns one decoded JSON object into an Entry. raw is the entry's encoded
// size, used for the per-entry limit. Unknown top-level fields become attributes.
func Normalize(object map[string]any, rawSize int, now time.Time) (Entry, *EntryError) {
	if rawSize > MaxEntryBytes {
		return Entry{}, &EntryError{Code: "entry_too_large", Message: fmt.Sprintf("entry is %d bytes, the limit is %d", rawSize, MaxEntryBytes)}
	}
	entry := Entry{IngestedAt: now, Size: rawSize}
	rest := map[string]any{}
	attributes := map[string]any{}
	var severity any
	var hasBody, hasTimestamp bool
	var timestampValue any
	var bodyValue any

	for key, value := range object {
		switch key {
		case "id":
			entry.ClientID = scalarString(value)
		case "timestamp", "time", "ts", "@timestamp":
			if !hasTimestamp {
				timestampValue, hasTimestamp = value, true
			}
		case "severity", "level", "severity_text", "severity_number", "loglevel":
			if severity == nil {
				severity = value
			}
		case "body", "message", "msg":
			if !hasBody || key == "body" {
				bodyValue, hasBody = value, true
			}
		case "event":
			entry.Event = scalarString(value)
		case "attributes":
			if m, ok := value.(map[string]any); ok {
				attributes = m
			} else if value != nil {
				rest[key] = value
			}
		case "resource":
			if m, ok := value.(map[string]any); ok {
				entry.Resource = m
			}
		case "trace_id", "traceId", "trace.id":
			entry.TraceID = scalarString(value)
		case "span_id", "spanId", "span.id":
			entry.SpanID = scalarString(value)
		case "request_id", "requestId", "request.id":
			entry.RequestID = scalarString(value)
		default:
			rest[key] = value
		}
	}
	for key, value := range rest {
		if _, exists := attributes[key]; !exists {
			attributes[key] = value
		}
	}

	if hasBody && bodyValue != nil {
		switch body := bodyValue.(type) {
		case string:
			entry.Body = body
		default:
			encoded, err := json.Marshal(body)
			if err != nil {
				return Entry{}, &EntryError{Code: "invalid_json", Message: "body cannot be encoded"}
			}
			entry.Body, entry.BodyJSON = string(encoded), string(encoded)
		}
	}
	if strings.TrimSpace(entry.Body) == "" && strings.TrimSpace(entry.Event) == "" {
		return Entry{}, &EntryError{Code: "empty_entry", Message: "an entry needs a body or an event"}
	}

	entry.Timestamp = now
	if hasTimestamp && timestampValue != nil {
		parsed, err := parseTimestamp(timestampValue)
		if err != nil {
			return Entry{}, &EntryError{Code: "invalid_timestamp", Message: err.Error()}
		}
		entry.Timestamp = parsed
		if parsed.After(now.Add(maxFutureSkew)) { // a badly skewed clock must not park logs in the future
			entry.Timestamp = now
			attributes["orion.original_timestamp"] = parsed.Format(time.RFC3339Nano)
		}
	}
	entry.Severity, entry.SeverityNumber = NormalizeSeverity(severity)

	if depth, keys := measure(attributes, 1); depth > MaxAttributeDepth {
		return Entry{}, &EntryError{Code: "attributes_too_deep", Message: fmt.Sprintf("attributes are nested %d levels, the limit is %d", depth, MaxAttributeDepth)}
	} else if keys > MaxAttributeKeys {
		return Entry{}, &EntryError{Code: "too_many_attributes", Message: fmt.Sprintf("entry has %d attribute keys, the limit is %d", keys, MaxAttributeKeys)}
	}
	if len(attributes) > 0 {
		entry.Attributes = attributes
	}
	return entry, nil
}

func scalarString(v any) string {
	switch value := v.(type) {
	case string:
		return strings.TrimSpace(value)
	case nil:
		return ""
	case float64:
		return fmt.Sprintf("%.0f", value)
	default:
		return fmt.Sprint(value)
	}
}

// measure returns the deepest nesting level and the total number of keys.
func measure(v any, depth int) (int, int) {
	deepest, keys := depth, 0
	switch value := v.(type) {
	case map[string]any:
		for _, child := range value {
			keys++
			d, k := measure(child, depth+1)
			deepest, keys = max(deepest, d), keys+k
		}
	case []any:
		for _, child := range value {
			d, k := measure(child, depth+1)
			deepest, keys = max(deepest, d), keys+k
		}
	}
	return deepest, keys
}
