package cli

import (
	"image/color"
	"io"

	"orion/agent/internal/ui"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/fang"
)

// fangScheme maps the design tokens onto fang's help and error styling so
// `--help` shares the look of every other command.
func fangScheme(c lipgloss.LightDarkFunc) fang.ColorScheme {
	light, dark := ui.Palettes()
	pick := func(l, d string) color.Color { return c(lipgloss.Color(l), lipgloss.Color(d)) }
	terminalDefault := lipgloss.NoColor{}
	downL, downD := ui.StatusColors("down")
	return fang.ColorScheme{
		// Fang falls back to light mode when a terminal does not answer its
		// background-colour query. Keep neutral text and the usage block on the
		// terminal defaults so that fallback cannot produce dark-on-dark text or
		// a light rectangle in a dark terminal.
		Base:           terminalDefault,
		Title:          pick(light.Accent, dark.Accent),
		Description:    pick(light.Muted, dark.Muted),
		Codeblock:      terminalDefault,
		Program:        pick(light.Accent, dark.Accent),
		DimmedArgument: pick(light.Faint, dark.Faint),
		Comment:        pick(light.Faint, dark.Faint),
		Flag:           pick(light.Accent, dark.Accent),
		FlagDefault:    pick(light.Faint, dark.Faint),
		Command:        terminalDefault,
		QuotedString:   pick(light.Muted, dark.Muted),
		Argument:       terminalDefault,
		Help:           pick(light.Muted, dark.Muted),
		Dash:           pick(light.Faint, dark.Faint),
		ErrorHeader:    [2]color.Color{lipgloss.Color("#ffffff"), pick(downL, downD)},
		ErrorDetails:   pick(downL, downD),
	}
}

// fangErrorHandler keeps our own error rendering (summary, cause, next steps).
func fangErrorHandler(errOut io.Writer) fang.ErrorHandler {
	return func(_ io.Writer, _ fang.Styles, err error) { renderCommandError(errOut, err) }
}
