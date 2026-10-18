package cli

import (
	"fmt"
	"strings"
	"time"

	"orion/agent/internal/logship"
	"orion/agent/internal/ui"

	"github.com/charmbracelet/lipgloss"
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

// logsHeader is the status line: follow state chip, level, source, search.
func logsHeader(th *ui.Theme, s logsState) string {
	chip := " snapshot "
	switch {
	case s.Paused:
		chip = fmt.Sprintf(" paused +%d ", s.Arrived)
	case s.Following:
		chip = " following "
	}
	var chipOut string
	if th.Plain() {
		chipOut = "[" + strings.TrimSpace(strings.TrimPrefix(chip, " ")) + "]"
	} else {
		tone := "up"
		if s.Paused {
			tone = "degraded"
		} else if !s.Following {
			tone = "unknown"
		}
		chipOut = th.Chip(tone, chip)
	}
	src := "all"
	if s.Source != "" {
		src = s.Source
	}
	search := "none"
	if s.Search != "" {
		search = s.Search
	}
	return chipOut + "  level " + th.Bold(">= "+s.Level) + "   source " + th.Bold(src) + "   " + th.Muted("search: "+search)
}

// logsScreen renders the header, a rule and up to height rows (newest first),
// each `HH:MM:SS level  source  body` with the body clipped to the width.
func logsScreen(th *ui.Theme, s logsState, width, height int) string {
	var b strings.Builder
	b.WriteString("\n" + logsHeader(th, s) + "\n")
	b.WriteString(th.Rule(min(width, 70)) + "\n")
	rows := s.visible(height)
	if len(rows) == 0 {
		b.WriteString(th.Muted("no log entries match") + "\n")
	}
	for _, e := range rows {
		sev := e.Severity
		if sev == "" {
			sev = "info"
		}
		src := clip(entrySource(e), 7)
		prefix := th.Muted(clock(e.Timestamp)) + " " + th.Tone(levelTone(sev), fmt.Sprintf("%-5s", sev)) + " " + th.Muted(fmt.Sprintf("%-7s", src)) + " "
		room := width - (8 + 1 + 5 + 1 + 7 + 1)
		body := strings.ReplaceAll(strings.TrimSpace(strings.SplitN(e.Body, "\n", 2)[0]), "\t", " ")
		if lipgloss.Width(body) > room && room > 1 {
			body = string([]rune(body)[:room-1]) + "…"
		}
		b.WriteString(prefix + body + "\n")
	}
	return b.String()
}

// logsFooter lists the keys of the interactive viewer.
func logsFooter(th *ui.Theme) string {
	keys := " q quit   f filter   l level   p pause   / search   c clear "
	if th.Plain() {
		return keys
	}
	return th.Chip("unknown", keys)
}
