package cli

import (
	"fmt"
	"strings"

	"orion/node/internal/ui"
)

// opStep is one finished step of start, stop, restart or run.
type opStep struct {
	Mark   string // "ok", "warn", "fail"
	Name   string
	Detail string
	TookMS int64
	// Took overrides TookMS when the caller wants its own wording ("1.4 s").
	Took string
	// Evidence and Fix explain a failed step.
	Evidence []string
	Fix      string
}

func (s opStep) tookText() string {
	switch {
	case s.Took != "":
		return s.Took
	case s.TookMS > 0:
		return fmt.Sprintf("%d s", (s.TookMS+500)/1000)
	}
	return ""
}

func opMark(th *ui.Theme, mark string) string {
	switch mark {
	case "warn":
		return th.WarnMark()
	case "fail":
		return th.FailMark()
	}
	return th.Pass()
}

// opRow is a step inside a panel: mark, name, detail, and the time on the right.
func opRow(th *ui.Theme, s opStep) string {
	took := s.Took
	if took == "" && s.TookMS > 0 {
		took = fmt.Sprintf("%.1f s", float64(s.TookMS)/1000)
	}
	return " " + opMark(th, s.Mark) + " " + padRight(s.Name, 30) + th.Muted(padRight(clip(s.Detail, 42), 43)) + th.Muted(fmt.Sprintf("%5s", took)) + "   "
}

// opPanel draws steps as a titled panel; right is the count shown on its edge.
func opPanel(th *ui.Theme, title, right string, steps []opStep) string {
	rows := make([]string, len(steps))
	for i, s := range steps {
		rows[i] = opRow(th, s)
	}
	return th.Panel(title, right, rows, panelWidth)
}

// opLines draws steps as plain indented lines (no panel), with evidence under a failure.
func opLines(th *ui.Theme, steps []opStep) string {
	var b strings.Builder
	bar := "     " + th.Faint("│") + " "
	for _, s := range steps {
		line := "   " + opMark(th, s.Mark) + " " + s.Name
		if s.Detail != "" {
			line = "   " + opMark(th, s.Mark) + " " + padRight(s.Name, 30) + th.Muted(s.Detail)
		}
		if t := s.tookText(); t != "" {
			line = strings.TrimRight(line, " ")
			line += strings.Repeat(" ", max(2, panelWidth-visibleWidth(line)-len(t))) + th.Muted(t)
		}
		b.WriteString(line + "\n")
		for _, ev := range s.Evidence {
			b.WriteString(bar + th.Muted(ev) + "\n")
		}
		if s.Fix != "" {
			b.WriteString(bar + fixLine(th, s.Fix) + "\n")
		}
	}
	return b.String()
}

func countOK(steps []opStep) int {
	n := 0
	for _, s := range steps {
		if s.Mark == "ok" {
			n++
		}
	}
	return n
}

// startScreen is the board's successful start.
func startScreen(th *ui.Theme, title string, pre, svc []opStep, running, next string) string {
	if th.Plain() {
		return opLines(th, append(append([]opStep{}, pre...), svc...)) + running + "\nnext: " + next + "\n"
	}
	blocks := []string{th.Accent("◆") + " " + th.Bold(title),
		opPanel(th, "Preflight", fmt.Sprintf("%d ✓", countOK(pre)), pre),
		opPanel(th, "Service", "", svc), "",
		th.Panel("", "", []string{" " + th.Tone("up", "●") + " " + running, "", " " + th.Accent("▸") + " " + th.Bold(next)}, panelWidth)}
	return "\n" + margin(th, blocks) + "\n"
}

// preflightFailScreen shows what was checked, the last service log lines, and
// leaves the headline to the error renderer.
func preflightFailScreen(th *ui.Theme, pre []opStep, logs []string) string {
	var b strings.Builder
	b.WriteString("\n " + th.Muted("PREFLIGHT") + "\n" + opLines(th, pre))
	if len(logs) > 0 {
		b.WriteString("\n " + th.Muted("RECENT SERVICE LOG") + "\n")
		for _, l := range logs {
			b.WriteString("   " + l + "\n")
		}
	}
	return b.String()
}

// confirmView is a yes/no question with the answer highlighted.
func confirmView(th *ui.Theme, question, detail, yes, no string, yesFocused bool, hint string) string {
	focus := 1
	if yesFocused {
		focus = 0
	}
	return choiceView(th, question, detail, []string{yes, no}, focus, hint)
}

// choiceView is a question with a row of answers; the focused one is lit.
func choiceView(th *ui.Theme, question, detail string, options []string, focus int, hint string) string {
	bar := " " + th.Accent("┃") + " "
	var cells []string
	for i, o := range options {
		if i == focus {
			cells = append(cells, th.Badge("up", o))
		} else {
			cells = append(cells, th.Muted(" "+o+" "))
		}
	}
	if th.Plain() {
		return question + " [" + strings.Join(options, "/") + "]\n"
	}
	out := "\n" + bar + th.Bold(question) + "\n"
	if detail != "" {
		out += bar + th.Muted(detail) + "\n"
	}
	out += "   " + strings.Join(cells, "  ")
	if hint != "" {
		out += "      " + th.Muted(hint)
	}
	return out + "\n"
}

// installHint prints the one-line installer, wrapped so it can be copied.
func installHint(th *ui.Theme, uninstall bool) string {
	arrow := th.Accent("▸")
	if th.Plain() {
		arrow = "$"
	}
	const url = "https://github.com/sunday-studio/orion/releases/latest/download/orion-node-installer.sh"
	if uninstall {
		return "\n   " + arrow + " " + th.Bold("curl -fsSL "+url+" | bash -s -- uninstall") + "\n " + th.Muted("Add --purge to remove config and state too.") + "\n"
	}
	return "\n " + "The installer sets everything up in one step:\n\n   " + arrow + " " + th.Bold("curl -fsSL "+url+" | bash -s -- \\") + "\n         " + th.Bold("--core-url <url> --enrollment-token <token>") +
		"\n\n " + th.Muted("Per-user install without root: add --user     Preview: add --dry-run") + "\n"
}

// runScreen is the foreground run's header and finished steps.
func runHeader(th *ui.Theme, what string) string {
	if th.Plain() {
		return "\nRun: " + what + "\n\n"
	}
	return "\n " + th.Accent("◆") + " " + th.Bold("Run") + "  " + th.Muted(what) + "\n\n"
}
