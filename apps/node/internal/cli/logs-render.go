package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"orion/node/internal/logship"
	"orion/node/internal/ui"
)

const (
	logsRuleW   = 86
	logsHeaderW = 85
	logsDrawerW = 84
)

// followChip is the log stream's state: following, paused with a count, or a snapshot.
func followChip(th *ui.Theme, s logsState) string {
	switch {
	case s.Paused:
		return th.Badge("degraded", fmt.Sprintf("◐ Paused +%d", s.Arrived))
	case s.Following:
		return th.Badge("up", "● Following")
	}
	return th.Badge("unknown", "○ Snapshot")
}

func runtimeChip(th *ui.Theme, r runtimeState) string {
	if r.Label == "" {
		return ""
	}
	glyph := map[string]string{"up": "●", "degraded": "◐", "maintenance": "‖"}[r.Tone]
	if glyph == "" {
		glyph = "○"
	}
	return th.Badge(r.Tone, glyph+" "+r.Label)
}

// logsHeader is the title line with the stream and daemon chips on the right,
// then the rule and the filters.
func logsHeader(th *ui.Theme, s logsState) string {
	src := "all"
	if s.Source != "" {
		src = s.Source
	}
	if th.Plain() {
		line := followChip(th, s)
		if r := runtimeChip(th, s.Runtime); r != "" {
			line += " " + r
		}
		return line + "  level >= " + s.Level + "   source " + src + "   search: " + orDefault(s.Search, "none")
	}
	left := " " + th.Accent("◆") + " " + th.Bold("Logs")
	if s.Host != "" {
		left += "  " + th.Muted(s.Host)
	}
	right := followChip(th, s)
	if r := runtimeChip(th, s.Runtime); r != "" {
		right += "  " + r
	}
	title := left + strings.Repeat(" ", max(2, logsHeaderW-visibleWidth(left)-visibleWidth(right)-1)) + right
	search := th.Faint("press /")
	if s.Search != "" {
		search = th.Bold(s.Search)
	}
	filters := " " + th.Muted("level") + "  " + th.Bold(s.Level+"+") + "   " + th.Muted("source") + "  " + th.Bold(src) + "   " + th.Muted("search") + "  " + search
	return title + "\n " + th.Faint(strings.Repeat("─", logsRuleW)) + "\n" + filters
}

func orDefault(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

// logRow is one entry: time, level, source, then the first line of the body.
func logRow(th *ui.Theme, e logship.Entry, width int, selected bool) string {
	sev := orDefault(e.Severity, "info")
	level := th.Tone(levelTone(sev), fmt.Sprintf("%-5s", sev))
	if sev == "trace" || sev == "debug" {
		level = th.Faint(fmt.Sprintf("%-5s", sev))
	}
	src := clip(entrySource(e), 7)
	prefix := th.Muted(clock(e.Timestamp)) + " " + level + "  " + th.Muted(fmt.Sprintf("%-7s", src)) + " "
	room := width - (1 + 8 + 1 + 5 + 2 + 7 + 1)
	body := strings.ReplaceAll(strings.TrimSpace(strings.SplitN(e.Body, "\n", 2)[0]), "\t", " ")
	if lipgloss.Width(body) > room && room > 1 {
		body = string([]rune(body)[:room-1]) + "…"
	}
	row := " " + prefix + body
	if selected {
		return th.Selected(row, logsHeaderW)
	}
	return row
}

// selectedIndex is where the cursor stands among the visible rows.
func (s logsState) selectedIndex(rows []logship.Entry) int {
	for i, e := range rows {
		if e.ID == s.SelectedID {
			return i
		}
	}
	return 0
}

// logDetails is the drawer for the selected entry.
func logDetails(th *ui.Theme, e logship.Entry) string {
	source := entrySource(e)
	if p, ok := e.Resource["log.file.path"].(string); ok && p != "" {
		source += " · " + p
	}
	rows := []string{" " + th.Muted(padRight("source", 8)) + " " + source}
	var keys []string
	for k := range e.Attributes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var cells []string
	for _, k := range keys {
		cells = append(cells, th.Muted(padRight(clip(k, 8), 8))+" "+clip(fmt.Sprint(e.Attributes[k]), 28))
	}
	for i := 0; i < len(cells) && i < 6; i += 2 {
		line := " " + cells[i]
		if i+1 < len(cells) {
			line = " " + padRight(cells[i], 8+1+28+4) + cells[i+1]
		}
		rows = append(rows, line)
	}
	state := th.Muted("○ not sent yet")
	switch {
	case e.Shipped != nil && *e.Shipped:
		state = th.Tone("up", "✓ sent to Core")
	case e.Shipped == nil:
		state = th.Muted("unknown")
	}
	rows = append(rows, " "+th.Muted(padRight("shipped", 8))+" "+state)
	return th.Panel("details", "", rows, logsDrawerW)
}

// logsScreen renders the header, up to height rows (newest first) and, when
// open, the details drawer of the selected entry.
func logsScreen(th *ui.Theme, s logsState, width, height int) string {
	var b strings.Builder
	b.WriteString("\n" + logsHeader(th, s) + "\n\n")
	rows := s.visible(height)
	if len(rows) == 0 {
		b.WriteString(" " + th.Muted("no log entries match") + "  " + th.Bold(fmt.Sprintf("level ≥ %s", s.Level)))
		if s.Search != "" {
			b.WriteString(th.Bold(fmt.Sprintf(" · search %q", s.Search)))
		}
		b.WriteString("  " + th.Muted("press esc to clear") + "\n")
		return b.String()
	}
	sel := s.selectedIndex(rows)
	for i, e := range rows {
		b.WriteString(logRow(th, e, width, !th.Plain() && s.cursorOn() && i == sel) + "\n")
	}
	if s.Drawer && !th.Plain() {
		b.WriteString("\n " + strings.ReplaceAll(logDetails(th, rows[sel]), "\n", "\n ") + "\n")
	}
	return b.String()
}

// cursorOn: the static snapshot has no cursor; the interactive view does.
func (s logsState) cursorOn() bool { return s.SelectedID != "" || s.Drawer || s.Interactive }

// logsFooter lists the keys of the interactive viewer.
func logsFooter(th *ui.Theme) string {
	pairs := [][2]string{{"↑↓", "move"}, {"enter", "details"}, {"f", "follow"}, {"p", "pause"}, {"l", "level"}, {"/", "search"}, {"q", "quit"}}
	if th.Plain() {
		var parts []string
		for _, p := range pairs {
			parts = append(parts, p[0]+" "+p[1])
		}
		return " " + strings.Join(parts, "   ") + " "
	}
	var parts []string
	for _, p := range pairs {
		parts = append(parts, th.Bold(p[0])+"  "+th.Muted(p[1]))
	}
	return "  " + strings.Join(parts, "  ")
}
