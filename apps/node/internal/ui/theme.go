// Package ui is the CLI's visual layer. Colours and status shapes come from the
// shared design tokens (docs/design/tokens.json) through theme_gen.go; nothing
// here hard-codes a colour. Meaning is never carried by colour alone: every
// status renders a glyph and its name.
package ui

//go:generate go run ./gen-theme -in ../../../../docs/design/tokens.json -out theme_gen.go

import (
	"io"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// Palette is the foreground palette for one colour mode.
type Palette struct{ Text, Muted, Faint, Accent, Border string }

// StatusToken is one status: a shape glyph plus a colour per mode.
type StatusToken struct{ Shape, Glyph, Light, Dark, BadgeBg, BadgeFg string }

// Theme renders styled text for one output destination.
type Theme struct {
	p        Palette
	dark     bool
	color    bool
	renderer *lipgloss.Renderer
}

// Detect builds the theme for w. Colour is disabled for NO_COLOR, TERM=dumb,
// and any writer that is not a terminal, so piped output stays plain.
func Detect(w io.Writer) *Theme {
	r := lipgloss.NewRenderer(w)
	profile := r.ColorProfile()
	color := profile != termenv.Ascii && os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb"
	dark := r.HasDarkBackground()
	return newTheme(r, dark, color)
}

// Plain returns a theme with colour disabled (tests, --no-color).
func Plain() *Theme { return newTheme(lipgloss.NewRenderer(io.Discard), false, false) }

// WithMode forces light or dark with colour on; used by screenshot tests.
func WithMode(dark bool) *Theme {
	r := lipgloss.NewRenderer(io.Discard)
	r.SetColorProfile(termenv.TrueColor)
	return newTheme(r, dark, true)
}

func newTheme(r *lipgloss.Renderer, dark, color bool) *Theme {
	p := paletteLight
	if dark {
		p = paletteDark
	}
	return &Theme{p: p, dark: dark, color: color, renderer: r}
}

// ColorEnabled reports whether this theme emits ANSI colour.
func (t *Theme) ColorEnabled() bool { return t.color }

func (t *Theme) style(hex string) lipgloss.Style {
	s := t.renderer.NewStyle()
	if t.color {
		s = s.Foreground(lipgloss.Color(hex))
	}
	return s
}

// Text, Muted, Faint, Accent style a string with the matching token.
func (t *Theme) Text(s string) string   { return t.style(t.p.Text).Render(s) }
func (t *Theme) Muted(s string) string  { return t.style(t.p.Muted).Render(s) }
func (t *Theme) Faint(s string) string  { return t.style(t.p.Faint).Render(s) }
func (t *Theme) Accent(s string) string { return t.style(t.p.Accent).Render(s) }

// Bold renders emphasised text (works without colour).
func (t *Theme) Bold(s string) string { return t.renderer.NewStyle().Bold(true).Render(s) }

// Status renders "<glyph> <name>" for a known status; unknown names fall back
// to the "unknown" shape so a typo never hides state.
func (t *Theme) Status(name string) string {
	tok, ok := statusTokens[name]
	if !ok {
		tok = statusTokens["unknown"]
	}
	hex := tok.Light
	if t.dark {
		hex = tok.Dark
	}
	return t.style(hex).Render(tok.Glyph + " " + name)
}

// StatusGlyph returns just the shape glyph (colour included).
func (t *Theme) StatusGlyph(name string) string {
	tok, ok := statusTokens[name]
	if !ok {
		tok = statusTokens["unknown"]
	}
	hex := tok.Light
	if t.dark {
		hex = tok.Dark
	}
	return t.style(hex).Render(tok.Glyph)
}

// StatusNames lists the known statuses (sorted by the generator).
func StatusNames() []string {
	out := make([]string, 0, len(statusTokens))
	for n := range statusTokens {
		out = append(out, n)
	}
	return out
}

// Tone styles text with a status colour but no glyph (for "ok:" / "error:" labels).
func (t *Theme) Tone(name, s string) string {
	tok, ok := statusTokens[name]
	if !ok {
		tok = statusTokens["unknown"]
	}
	hex := tok.Light
	if t.dark {
		hex = tok.Dark
	}
	return t.style(hex).Render(s)
}

// Palettes returns the light and dark palettes (for adapters such as fang).
func Palettes() (light, dark Palette) { return paletteLight, paletteDark }

// StatusColors returns a status colour in both modes.
func StatusColors(name string) (light, dark string) {
	tok, ok := statusTokens[name]
	if !ok {
		tok = statusTokens["unknown"]
	}
	return tok.Light, tok.Dark
}
