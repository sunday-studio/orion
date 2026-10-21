package cli

import (
	"strings"

	"github.com/charmbracelet/glamour"

	"orion/agent/internal/ui"
)

// sanitizeMarkdown drops control characters other than newline and tab, so text
// from the network can never move the cursor or rewrite the screen.
func sanitizeMarkdown(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		if r < 0x20 || r == 0x7f || (r >= 0x80 && r < 0xa0) {
			return -1
		}
		return r
	}, s)
}

// renderMarkdown formats md for the terminal. Without color (plain output,
// pipes, accessible mode) the markdown is printed as written, which is
// readable on its own.
func renderMarkdown(th *ui.Theme, md string, width int) string {
	md = sanitizeMarkdown(md)
	if th.Plain() {
		return strings.TrimRight(md, "\n") + "\n"
	}
	if width < 40 || width > 100 {
		width = 80
	}
	style := "light"
	if th.Dark() {
		style = "dark"
	}
	r, err := glamour.NewTermRenderer(glamour.WithStandardStyle(style), glamour.WithWordWrap(width-4))
	if err != nil {
		return strings.TrimRight(md, "\n") + "\n"
	}
	out, err := r.Render(md)
	if err != nil {
		return strings.TrimRight(md, "\n") + "\n"
	}
	return strings.Trim(out, "\n") + "\n"
}
