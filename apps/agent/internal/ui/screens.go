package ui

import (
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"
)

// Width returns the layout width for w: the real terminal width, else $COLUMNS,
// else 80. It is never below 40.
func Width(f *os.File) int {
	if f != nil {
		if w, _, err := term.GetSize(f.Fd()); err == nil && w > 0 {
			return max(w, 40)
		}
	}
	if c, err := strconv.Atoi(os.Getenv("COLUMNS")); err == nil && c > 0 {
		return max(c, 40)
	}
	return DefaultWidth
}

// Plain reports whether this theme prints plain text ([ok], [warn], [fail]).
func (t *Theme) Plain() bool { return !t.color }

// Pass, WarnMark and FailMark are the check glyphs: always followed by a word
// by the caller, and spelled [ok] / [warn] / [fail] in plain mode.
func (t *Theme) Pass() string {
	if t.Plain() {
		return "[ok]"
	}
	return t.Tone("up", "✓")
}

func (t *Theme) WarnMark() string {
	if t.Plain() {
		return "[warn]"
	}
	return t.Tone("degraded", "!")
}

func (t *Theme) FailMark() string {
	if t.Plain() {
		return "[fail]"
	}
	return t.Tone("down", "✗")
}

// SkipMark marks a check that could not run.
func (t *Theme) SkipMark() string {
	if t.Plain() {
		return "[skip]"
	}
	return t.Faint("○")
}

// Spinner is one frame of the progress glyph.
func (t *Theme) Spinner(frame int) string {
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	if t.Plain() {
		return "..."
	}
	return t.Tone("degraded", frames[frame%len(frames)])
}

// Rule is a thin horizontal rule.
func (t *Theme) Rule(width int) string { return t.Faint(strings.Repeat("─", width)) }

// Box draws the artboard's rounded box: a titled top edge, a left bar on every
// row and a closing bottom edge. Rows are not padded to a right border, so
// long values never break the frame.
func (t *Theme) Box(title string, rows []string, width int) string {
	if width < 20 {
		width = 20
	}
	top := "╭─ " + title + " "
	pad := width - lipgloss.Width(top) - 1
	if pad < 1 {
		pad = 1
	}
	var b strings.Builder
	b.WriteString(t.Faint("╭─ ") + t.Bold(title) + t.Faint(" "+strings.Repeat("─", pad)+"╮") + "\n")
	for _, r := range rows {
		b.WriteString(t.Faint("│") + " " + r + "\n")
	}
	b.WriteString(t.Faint("╰" + strings.Repeat("─", width-2) + "╯"))
	return b.String()
}

// Wrap breaks text at word boundaries into lines of at most width cells, the
// continuation lines indented by hang spaces. A word longer than a line is
// kept whole rather than cut, so commands stay copyable.
func Wrap(text string, width, hang int) []string {
	if width <= hang+10 {
		width = hang + 10
	}
	var lines []string
	cur := ""
	for _, w := range strings.Fields(text) {
		switch {
		case cur == "":
			cur = w
		case lipgloss.Width(cur)+1+lipgloss.Width(w) <= width:
			cur += " " + w
		default:
			lines = append(lines, cur)
			cur = strings.Repeat(" ", hang) + w
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}
