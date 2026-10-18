package cli

import (
	"os"

	"orion/agent/internal/ui"

	"github.com/charmbracelet/x/term"
)

// terminalWidth is the layout width of stdout (or $COLUMNS, else 80).
func terminalWidth() int { return ui.Width(os.Stdout) }

func stdoutIsTerminal() bool { return term.IsTerminal(os.Stdout.Fd()) }

func hostnameOrEmpty() string {
	h, _ := os.Hostname()
	return h
}
