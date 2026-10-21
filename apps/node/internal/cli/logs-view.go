package cli

import (
	"strings"
	"time"

	"orion/node/internal/control"
	"orion/node/internal/logship"
)

// logsState is everything the shipped-logs screen shows. The same value drives
// the interactive follower and the static render, so both look identical.
type logsState struct {
	Entries   []logship.Entry
	Level     string // minimum level: trace|debug|info|warn|error|fatal
	Source    string // "" means all sources
	Search    string
	Paused    bool
	Following bool
	Arrived   int // lines that arrived while paused
	Note      string
	Runtime   runtimeState
	Host      string
	// SelectedID is the entry under the cursor ("" means the newest one) and
	// Drawer says whether its details are open.
	SelectedID string
	Drawer     bool
	// Interactive is set by the full-screen viewer, which shows a cursor.
	Interactive bool
}

type runtimeState struct {
	Label string
	Tone  string
}

var levelOrder = []string{"trace", "debug", "info", "warn", "error", "fatal"}

func levelRank(l string) int {
	for i, v := range levelOrder {
		if v == l {
			return i
		}
	}
	return 2 // unknown severities count as info
}

func entrySource(e logship.Entry) string {
	if s, ok := e.Resource["log.source"].(string); ok && s != "" {
		return s
	}
	if s, ok := e.Resource["service.name"].(string); ok {
		return s
	}
	return e.Source
}

func (s logsState) match(e logship.Entry) bool {
	if levelRank(e.Severity) < levelRank(s.Level) {
		return false
	}
	if s.Source != "" && entrySource(e) != s.Source {
		return false
	}
	if s.Search != "" && !strings.Contains(strings.ToLower(e.Body+" "+e.Event), strings.ToLower(s.Search)) {
		return false
	}
	return true
}

// visible returns the matching entries, newest first, at most n.
func (s logsState) visible(n int) []logship.Entry {
	var out []logship.Entry
	for i := len(s.Entries) - 1; i >= 0 && len(out) < n; i-- {
		if s.match(s.Entries[i]) {
			out = append(out, s.Entries[i])
		}
	}
	return out
}

func clock(ts string) string {
	if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
		return t.Local().Format("15:04:05")
	}
	return "--:--:--"
}

func levelTone(sev string) string {
	switch sev {
	case "error", "fatal":
		return "down"
	case "warn":
		return "degraded"
	case "info":
		return "up"
	}
	return "unknown"
}

func logRuntimeState(st control.Status, err error, now time.Time) runtimeState {
	if err != nil {
		return runtimeState{Label: "No reply", Tone: "degraded"}
	}
	if !st.Registered {
		return runtimeState{Label: "Not enrolled", Tone: "unknown"}
	}
	if st.MaintenanceMode {
		return runtimeState{Label: "Maintenance", Tone: "maintenance"}
	}
	if st.LastReportAt.IsZero() {
		return runtimeState{Label: "Waiting", Tone: "unknown"}
	}
	if !st.LastReportAt.IsZero() && now.Sub(st.LastReportAt) > 5*time.Minute {
		return runtimeState{Label: "No recent report", Tone: "degraded"}
	}
	return runtimeState{Label: "Connected", Tone: "up"}
}
