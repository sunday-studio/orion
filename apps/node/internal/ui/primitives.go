package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

// DefaultWidth is the layout width when the terminal size is unknown. Every
// primitive must stay readable at 80 columns.
const DefaultWidth = 80

// Header is the title line shared by every command.
func (t *Theme) Header(title string) string {
	return t.Bold("Orion Node") + t.Faint(" / ") + t.Accent(title)
}

// KV renders aligned "key  value" lines.
func (t *Theme) KV(pairs [][2]string) string {
	width := 0
	for _, p := range pairs {
		width = max(width, lipgloss.Width(p[0]))
	}
	var b strings.Builder
	for _, p := range pairs {
		b.WriteString("  " + t.Muted(p[0]) + strings.Repeat(" ", width-lipgloss.Width(p[0])+2) + p[1] + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// Table renders rows under muted headers with hairline-free, column-aligned
// layout that wraps nothing and truncates the last column to fit width.
func (t *Theme) Table(headers []string, rows [][]string, width int) string {
	if width <= 0 {
		width = DefaultWidth
	}
	tb := table.New().
		Border(lipgloss.HiddenBorder()).
		BorderTop(false).BorderBottom(false).BorderLeft(false).BorderRight(false).
		BorderHeader(false).BorderColumn(false).
		Width(width).
		Headers(headers...).
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			s := t.renderer.NewStyle().PaddingRight(2)
			if row == table.HeaderRow {
				s = s.Bold(true)
				if t.color {
					s = s.Foreground(lipgloss.Color(t.p.Muted))
				}
			}
			return s
		})
	return tb.String()
}

// CalloutKind selects the label of a callout.
type CalloutKind string

const (
	Note    CalloutKind = "note"
	Warning CalloutKind = "warning"
	Failure CalloutKind = "error"
)

// Callout renders a labelled block; the label is text, so it survives
// monochrome output.
func (t *Theme) Callout(kind CalloutKind, text string) string {
	tone := map[CalloutKind]string{Note: "unknown", Warning: "degraded", Failure: "down"}[kind]
	label := strings.ToUpper(string(kind))
	return t.Tone(tone, "▌ "+label) + "  " + text
}

// StepState is the state of one line in a progress list.
type StepState string

const (
	StepRunning StepState = "running"
	StepOK      StepState = "ok"
	StepWarn    StepState = "warn"
	StepFail    StepState = "fail"
	StepSkip    StepState = "skip"
)

// Step renders one checklist line: glyph, state word, text.
func (t *Theme) Step(state StepState, text string) string {
	switch state {
	case StepOK:
		return fmt.Sprintf("  %s %s", t.Tone("up", "ok  "), text)
	case StepWarn:
		return fmt.Sprintf("  %s %s", t.Tone("degraded", "warn"), text)
	case StepFail:
		return fmt.Sprintf("  %s %s", t.Tone("down", "FAIL"), text)
	case StepSkip:
		return fmt.Sprintf("  %s %s", t.Faint("skip"), text)
	}
	return fmt.Sprintf("  %s %s", t.Accent("..  "), text)
}
