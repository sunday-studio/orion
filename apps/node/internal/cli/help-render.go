package cli

import (
	"fmt"
	"strings"

	"orion/node/internal/ui"
)

type helpEntry struct{ name, desc string }

type helpGroup struct {
	title   string
	entries []helpEntry
}

// rootHelpGroups mirrors the cli-help-error board: commands grouped by
// intent, not alphabetically.
var rootHelpGroups = []helpGroup{
	{"START HERE", []helpEntry{
		{"setup", "connect this host and choose what to monitor"},
		{"doctor", "check everything and fix what is wrong"},
		{"status", "see what the node is doing right now"},
	}},
	{"DAY TO DAY", []helpEntry{
		{"logs", "read node and shipped logs"},
		{"config", "show, diff, validate and edit settings"},
		{"update", "upgrade safely, with rollback"},
		{"service", "start, stop, restart, install"},
	}},
}

var helpExamples = []string{"orion-node setup", "orion-node logs --follow --level warn", "orion-node status --short"}

// helpMore lists the commands that are not in a group, so none is hidden.
// The first seven are the board's line; the rest are the other commands.
var helpMore = []string{"enroll", "discover", "usage", "maintenance", "token", "version", "run", "start", "stop", "restart", "reconfigure", "state", "emit", "run-job"}

// rootHelp renders `orion-node --help`. It fits 80 columns.
func rootHelp(th *ui.Theme, width int) string {
	var b strings.Builder
	if th.Plain() {
		b.WriteString("\norion-node  monitor this host and report to Orion Core\n\nUSAGE   orion-node <command> [flags]\n")
	} else {
		b.WriteString("\n " + th.Accent("◆") + " " + th.Bold("orion-node") + th.Muted("  monitor this host and report to Orion Core") + "\n\n")
		b.WriteString(" " + th.Muted("USAGE") + "   orion-node " + th.Muted("<command> [flags]") + "\n")
	}
	for _, g := range rootHelpGroups {
		b.WriteString("\n " + th.Muted(g.title) + "\n")
		for _, e := range g.entries {
			b.WriteString("   " + th.Accent(fmt.Sprintf("%-9s", e.name)) + " " + e.desc + "\n")
		}
	}
	b.WriteString("\n " + th.Muted("EXAMPLES") + "\n")
	for _, ex := range helpExamples {
		b.WriteString("   " + th.Muted("$") + " " + ex + "\n")
	}
	more := strings.Join(helpMore, " · ")
	lines := ui.Wrap(more, width-8, 0)
	for i, l := range lines {
		if i == 0 {
			b.WriteString("\n " + th.Muted("More:") + " " + l + "\n")
		} else {
			b.WriteString("       " + l + "\n")
		}
	}
	return b.String()
}
