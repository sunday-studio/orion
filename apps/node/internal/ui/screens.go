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

// StatusMark is a status glyph; plain mode spells the state in brackets so
// colour (and even the glyph) is never the only signal.
func (t *Theme) StatusMark(name string) string {
	if t.Plain() {
		switch name {
		case "up":
			return "[ok]"
		case "down":
			return "[down]"
		case "degraded":
			return "[slow]"
		case "maintenance":
			return "[maint]"
		}
		return "[?]"
	}
	return t.StatusGlyph(name)
}

// Chip is a filled badge ("ERROR"): coloured background in colour mode, and
// the bracketed word in plain mode.
func (t *Theme) Chip(status, text string) string {
	if t.Plain() {
		return "[" + strings.ToLower(strings.TrimSpace(text)) + "]"
	}
	tok, ok := statusTokens[status]
	if !ok {
		tok = statusTokens["unknown"]
	}
	bg := tok.Light
	if t.dark {
		bg = tok.Dark
	}
	return t.renderer.NewStyle().Bold(true).Background(lipgloss.Color(bg)).Foreground(lipgloss.Color("#ffffff")).Render(text)
}

// Panel draws the discover/usage style box: rounded corners on all four sides,
// the title on the top edge on the left and a count or label on the right.
// Rows are padded to the inner width (84 for the standard 86-wide panel);
// rows wider than that are clipped.
func (t *Theme) Panel(title, right string, rows []string, width int) string {
	if t.Plain() { // plain is a text layout, not a stripped box: a heading, then indented lines
		var b strings.Builder
		head := title
		if right != "" {
			head += " (" + right + ")"
		}
		b.WriteString(head + "\n")
		for _, r := range rows {
			if strings.TrimSpace(r) != "" {
				b.WriteString("  " + strings.TrimLeft(r, " ") + "\n")
			}
		}
		return strings.TrimRight(b.String(), "\n")
	}
	if width < 24 {
		width = 24
	}
	inner := width - 2 // between the side borders; rows carry their own leading space
	left := ""
	if title != "" {
		left = " " + title + " "
	}
	rt := ""
	if right != "" {
		rt = " " + right + " "
	}
	dash := width - 2 - lipgloss.Width(left) - lipgloss.Width(rt)
	if dash < 1 {
		dash = 1
	}
	var b strings.Builder
	b.WriteString(t.Faint("╭") + t.Muted(left) + t.Faint(strings.Repeat("─", dash)) + t.Muted(rt) + t.Faint("╮") + "\n")
	for _, r := range rows {
		w := lipgloss.Width(r)
		if w > inner {
			r = lipgloss.NewStyle().MaxWidth(inner).Render(r)
			w = lipgloss.Width(r)
		}
		b.WriteString(t.Faint("│") + r + strings.Repeat(" ", inner-w) + t.Faint("│") + "\n")
	}
	b.WriteString(t.Faint("╰" + strings.Repeat("─", width-2) + "╯"))
	return b.String()
}

// Badge is a filled status chip with the design's badge tints (" ● Connected ").
// Plain mode prints the bracketed word. Terminals use one palette, so the dark
// badge colours apply in light and dark.
func (t *Theme) Badge(status, text string) string {
	if t.Plain() {
		return "[" + strings.ToLower(strings.TrimSpace(text)) + "]"
	}
	tok, ok := statusTokens[status]
	if !ok {
		tok = statusTokens["unknown"]
	}
	return t.renderer.NewStyle().Bold(true).Background(lipgloss.Color(tok.BadgeBg)).Foreground(lipgloss.Color(tok.BadgeFg)).Render(" " + text + " ")
}

// Selected highlights a line: the whole width gets the selection background,
// including after the colour resets inside it. Plain mode marks it with ">".
func (t *Theme) Selected(line string, width int) string {
	if t.Plain() {
		return ">" + strings.TrimPrefix(line, " ")
	}
	tok := statusTokens["unknown"]
	bg := tok.BadgeBg
	seq := t.renderer.ColorProfile().Color(bg).Sequence(true)
	if seq == "" {
		return line
	}
	on := "\x1b[" + seq + "m"
	pad := ""
	if w := lipgloss.Width(line); w < width {
		pad = strings.Repeat(" ", width-w)
	}
	return on + strings.ReplaceAll(line, "\x1b[0m", "\x1b[0m"+on) + pad + "\x1b[0m"
}
