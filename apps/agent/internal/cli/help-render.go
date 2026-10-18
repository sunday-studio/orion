package cli

import (
	"fmt"
	"strings"

	"orion/agent/internal/ui"
)

type helpEntry struct{ name, desc string }

type helpGroup struct {
	title   string
	entries []helpEntry
}

// rootHelpGroups mirrors the cli-help-error artboard: commands grouped by
// intent, not alphabetically.
var rootHelpGroups = []helpGroup{
	{"GET STARTED", []helpEntry{
		{"setup", "Connect this host and choose what to monitor"},
		{"doctor", "Check the agent and fix problems"},
		{"status", "Show the agent and its Core connection"},
	}},
	{"DAY TO DAY", []helpEntry{
		{"logs", "View agent and shipped logs"},
		{"service", "install, start, stop, restart, uninstall"},
		{"config", "show, diff, validate, edit"},
		{"update", "Upgrade safely, with rollback"},
	}},
}

// rootHelp renders `orion-agent --help`. It fits 80 columns.
func rootHelp(th *ui.Theme, width int) string {
	var b strings.Builder
	b.WriteString("\n" + th.Bold("orion-agent") + th.Muted("  the Orion monitoring agent") + "\n\n")
	b.WriteString(th.Bold("USAGE") + "\n  orion-agent " + th.Muted("[command] [flags]") + "\n")
	for _, g := range rootHelpGroups {
		b.WriteString("\n" + th.Bold(g.title) + "\n")
		for _, e := range g.entries {
			b.WriteString("  " + th.Accent(fmt.Sprintf("%-10s", e.name)) + e.desc + "\n")
		}
	}
	b.WriteString("\n" + th.Bold("FLAGS") + "\n")
	for _, f := range [][2]string{{"--json", "Machine-readable output"}, {"--no-color", "Plain output"}, {"-h, --help", "Show help"}} {
		b.WriteString(fmt.Sprintf("  %-14s%s\n", f[0], f[1]))
	}
	more := "More: enroll, discover, usage, token, maintenance, version, run   (orion-agent <command> --help)"
	for _, l := range ui.Wrap(more, width-2, 6) {
		b.WriteString("\n" + "  " + th.Muted(l))
	}
	b.WriteString("\n")
	return b.String()
}
