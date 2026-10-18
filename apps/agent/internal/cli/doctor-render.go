package cli

import (
	"fmt"
	"strings"

	"orion/agent/internal/doctor"
	"orion/agent/internal/ui"
)

const doctorLabelWidth = 17

// doctorRow renders one finished check exactly like the cli-doctor artboard:
// glyph, padded label, message, and the fix indented underneath.
func doctorRow(th *ui.Theme, r doctor.Result, width int) string {
	var mark string
	switch r.Status {
	case doctor.OK:
		mark = th.Pass()
	case doctor.Warn:
		mark = th.WarnMark()
	case doctor.Fail:
		mark = th.FailMark()
	default:
		mark = th.SkipMark()
	}
	if th.Plain() {
		mark = fmt.Sprintf("%-6s", strings.TrimSpace(mark))
	}
	prefixWidth := 2 + visibleWidth(mark) + 1 + doctorLabelWidth
	label := fmt.Sprintf("%-*s", doctorLabelWidth, r.Label)
	lines := ui.Wrap(r.Message, width-prefixWidth, 0)
	if len(lines) == 0 {
		lines = []string{""}
	}
	var b strings.Builder
	b.WriteString("  " + mark + " " + label + th.Text(lines[0]))
	for _, l := range lines[1:] {
		b.WriteString("\n" + strings.Repeat(" ", prefixWidth) + l)
	}
	if r.Fix != "" && (r.Status == doctor.Warn || r.Status == doctor.Fail) {
		const indent = 6
		fixLines := ui.Wrap("fix: "+r.Fix, width-indent, 5)
		for i, l := range fixLines {
			if i == 0 {
				l = th.Muted("fix:") + strings.TrimPrefix(l, "fix:")
			}
			b.WriteString("\n" + strings.Repeat(" ", indent) + l)
		}
	}
	return b.String()
}

func visibleWidth(s string) int {
	// glyphs are single cells; plain marks are ASCII
	n := 0
	inEsc := false
	for _, r := range s {
		switch {
		case r == 0x1b:
			inEsc = true
		case inEsc && r == 'm':
			inEsc = false
		case !inEsc:
			n++
		}
	}
	return n
}

// doctorSummary is the last line: counts then the exit code.
func doctorSummary(th *ui.Theme, rep doctor.Report) (string, int) {
	var pass, warn, fail int
	for _, c := range rep.Checks {
		switch c.Status {
		case doctor.OK:
			pass++
		case doctor.Warn:
			warn++
		case doctor.Fail:
			fail++
		}
	}
	exit := 0
	if fail > 0 {
		exit = 1
	}
	wl := "warnings"
	if warn == 1 {
		wl = "warning"
	}
	line := "  " + th.Tone("up", fmt.Sprintf("%d passed", pass)) + "   " +
		th.Tone("degraded", fmt.Sprintf("%d %s", warn, wl)) + "   " +
		th.Tone("down", fmt.Sprintf("%d failed", fail)) + "   " +
		th.Muted(fmt.Sprintf("exit %d", exit))
	return line, exit
}

// renderDoctor renders a whole report (used by tests and non-streaming output).
func renderDoctor(th *ui.Theme, rep doctor.Report, width int) (string, int) {
	var b strings.Builder
	b.WriteString("\n")
	for _, c := range rep.Checks {
		b.WriteString(doctorRow(th, c, width) + "\n")
	}
	sum, exit := doctorSummary(th, rep)
	b.WriteString("\n" + sum + "\n")
	return b.String(), exit
}
