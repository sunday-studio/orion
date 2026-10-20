package cli

import (
	"fmt"
	"strings"
	"time"

	"orion/agent/internal/doctor"
	"orion/agent/internal/ui"
)

var doctorGroups = []string{"Agent", "Core", "Discovery", "Logs", "System"}

// Columns inside a group panel: mark, title, detail, then the time, right aligned.
const (
	doctorTitleW  = 30
	doctorDetailW = 43
	doctorTimeW   = 5
)

type doctorCounts struct{ pass, warn, fail, skip int }

func countDoctor(results []doctor.Result) (c doctorCounts) {
	for _, r := range results {
		switch r.Status {
		case doctor.OK:
			c.pass++
		case doctor.Warn:
			c.warn++
		case doctor.Fail:
			c.fail++
		default:
			c.skip++
		}
	}
	return
}

func doctorMark(th *ui.Theme, s doctor.Status) string {
	switch s {
	case doctor.OK:
		return th.Pass()
	case doctor.Warn:
		return th.WarnMark()
	case doctor.Fail:
		return th.FailMark()
	}
	return th.SkipMark()
}

// fixLine renders "Fix  command  note": the command bold, the note dim.
func fixLine(th *ui.Theme, fix string) string {
	cmd, note, _ := strings.Cut(fix, "  ")
	line := th.Muted("Fix") + "  " + th.Bold(cmd)
	if n := strings.TrimSpace(note); n != "" {
		line += "  " + th.Muted(n)
	}
	return line
}

// doctorPanelRows renders one group's checks as the rows of a panel. Skipped
// checks are left out. running names the check being run, shown with a spinner.
func doctorPanelRows(th *ui.Theme, results []doctor.Result, running string, frame int) []string {
	var rows []string
	bar := "      " + th.Faint("│") + " "
	for _, r := range results {
		switch r.Status {
		case doctor.Skip:
			continue
		case doctor.OK:
			took := ""
			if r.TookMS > 0 {
				took = fmt.Sprintf("%d ms", r.TookMS)
			}
			detail := r.Message
			if r.ID == "token" || r.ID == "clock" {
				detail = "" // the headline already says it ("Credentials accepted", "Clock within 1 s")
			}
			rows = append(rows, " "+doctorMark(th, r.Status)+" "+padRight(r.Title, doctorTitleW)+th.Muted(padRight(clip(detail, doctorDetailW-1), doctorDetailW))+th.Muted(fmt.Sprintf("%*s", doctorTimeW, took)))
		default:
			rows = append(rows, " "+doctorMark(th, r.Status)+" "+r.Title)
			for _, ev := range r.Evidence {
				rows = append(rows, bar+th.Muted(ev))
			}
			if r.Fix != "" {
				rows = append(rows, bar+fixLine(th, r.Fix))
			}
		}
	}
	if running != "" {
		rows = append(rows, " "+th.Tone("degraded", th.Spinner(frame))+" "+running+th.Muted("…"))
	}
	return rows
}

func panelRight(c doctorCounts) string {
	switch {
	case c.fail > 0 && c.warn > 0:
		return fmt.Sprintf("%d ✗ %d !", c.fail, c.warn)
	case c.fail > 0:
		return fmt.Sprintf("%d ✗", c.fail)
	case c.warn > 0:
		return fmt.Sprintf("%d !", c.warn)
	}
	return fmt.Sprintf("%d ✓", c.pass)
}

// doctorHeader is the title line with a short note on the right.
func doctorHeader(th *ui.Theme, host, right string) string {
	left := " " + th.Accent("◆") + " " + th.Bold("Doctor")
	if host != "" {
		left += "  " + th.Muted(host)
	}
	if right == "" {
		return left
	}
	return left + strings.Repeat(" ", max(2, panelWidth-visibleWidth(left)-len([]rune(right)))) + th.Muted(right)
}

// progressBar is the thin bar under the checklist.
func progressBar(th *ui.Theme, done, total, width int) string {
	if total <= 0 {
		return ""
	}
	filled := done * width / total
	if filled >= width {
		return th.Tone("up", strings.Repeat("━", width))
	}
	return th.Tone("up", strings.Repeat("━", filled)+"╸") + th.Faint(strings.Repeat("─", width-filled-1))
}

// doctorGrouped is the checklist as panels per group. total is the number of
// checks that will run; running and frame draw the live spinner row.
func doctorGrouped(th *ui.Theme, host string, results []doctor.Result, total int, running *doctor.Check, frame int) string {
	if th.Plain() {
		return plainDoctor(results)
	}
	var blocks []string
	blocks = append(blocks, doctorHeader(th, host, fmt.Sprintf("%d checks", total)))
	for _, g := range doctorGroups {
		var in []doctor.Result
		for _, r := range results {
			if r.Group == g {
				in = append(in, r)
			}
		}
		run := ""
		if running != nil && running.Group == g {
			run = running.Label
		}
		rows := doctorPanelRows(th, in, run, frame)
		if len(rows) == 0 {
			continue
		}
		blocks = append(blocks, th.Panel(g, panelRight(countDoctor(in)), rows, panelWidth))
	}
	if running != nil {
		done := len(results)
		blocks = append(blocks, "", "   "+progressBar(th, done, total, 40)+"  "+th.Muted(fmt.Sprintf("%d of %d", done, total)))
	}
	return margin(th, blocks) + "\n"
}

// doctorCollapsed is the all-clear screen: one line, the list a flag away.
func doctorCollapsed(th *ui.Theme, host string, c doctorCounts, took time.Duration) string {
	if th.Plain() {
		return fmt.Sprintf("[ok] %d checks passed (%.1f s)\n", c.pass, took.Seconds())
	}
	t := fmt.Sprintf("%.1f s", took.Seconds())
	left := "   " + th.Pass() + " " + fmt.Sprintf("%d checks passed", c.pass)
	line := left + strings.Repeat(" ", max(2, panelWidth-visibleWidth(left)-len(t)-1)) + th.Muted(t)
	return "\n" + doctorHeader(th, host, "") + "\n\n" + line + "\n"
}

// doctorFailing is the problem list with a summary box, used when a check failed.
func doctorFailing(th *ui.Theme, results []doctor.Result) string {
	if th.Plain() {
		return plainDoctor(results)
	}
	var b strings.Builder
	b.WriteString("\n")
	bar := "     " + th.Faint("│") + " "
	for _, r := range results {
		if r.Status != doctor.Warn && r.Status != doctor.Fail {
			continue
		}
		b.WriteString("   " + doctorMark(th, r.Status) + " " + r.Title + "\n")
		for _, ev := range r.Evidence {
			b.WriteString(bar + th.Muted(ev) + "\n")
		}
		if r.Fix != "" {
			b.WriteString(bar + fixLine(th, r.Fix) + "\n")
		}
	}
	c := countDoctor(results)
	wl := plural(c.warn, "warning", "warnings")
	sum := " " + th.Pass() + " " + fmt.Sprintf("%d passed", c.pass) + "   " + th.WarnMark() + " " + fmt.Sprintf("%d %s", c.warn, wl) + "   " + th.FailMark() + " " + fmt.Sprintf("%d failed", c.fail)
	b.WriteString("\n" + margin(th, []string{th.Panel("", "", []string{sum}, 52)}) + "\n")
	return b.String()
}

// plainDoctor is the no-colour, no-box layout: one fact per line.
func plainDoctor(results []doctor.Result) string {
	th := ui.Plain()
	var b strings.Builder
	b.WriteString("\n")
	for _, r := range results {
		mark := fmt.Sprintf("%-6s", strings.TrimSpace(doctorMark(th, r.Status)))
		if r.Status == doctor.OK || r.Status == doctor.Skip {
			msg := r.Message
			if r.TookMS > 0 {
				msg += fmt.Sprintf(" %d ms", r.TookMS)
			}
			b.WriteString("  " + mark + " " + padRight(r.Label, 17) + msg + "\n")
			continue
		}
		b.WriteString("  " + mark + " " + r.Title + "\n")
		for _, ev := range r.Evidence {
			b.WriteString("         " + ev + "\n")
		}
		if r.Fix != "" {
			b.WriteString("         fix: " + r.Fix + "\n")
		}
	}
	c := countDoctor(results)
	exit := 0
	if c.fail > 0 {
		exit = 1
	}
	fmt.Fprintf(&b, "\n  %d passed   %d %s   %d failed   exit %d\n", c.pass, c.warn, plural(c.warn, "warning", "warnings"), c.fail, exit)
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
