package cli

import (
	"fmt"
	"strings"
	"time"

	"orion/node/internal/ui"
)

// maintenanceDurations are the choices of the form, in the board's order.
var maintenanceDurations = []struct {
	Label string
	D     time.Duration
}{{"2 hours", 2 * time.Hour}, {"1 hour", time.Hour}, {"4 hours", 4 * time.Hour}, {"24 hours", 24 * time.Hour}}

// maintFormState is what the maintenance form shows.
type maintFormState struct {
	Reason   string
	Focus    int // 0 reason, 1 duration, 2 buttons
	Duration int // index into maintenanceDurations; len(...) means custom
	Custom   string
	Start    bool // the Start button is the highlighted one
}

func maintenanceForm(th *ui.Theme, f maintFormState) string {
	bar := func(on bool, rows ...string) []string {
		out := make([]string, len(rows))
		for i, r := range rows {
			if on {
				out[i] = " " + th.Accent("┃") + " " + r
			} else {
				out[i] = " " + r
			}
		}
		return out
	}
	why := bar(f.Focus != 1, th.Accent("›")+" "+f.Reason)
	why = append(why, " "+th.Muted("Optional. Shown on the incident and in status."))
	var opts []string
	for i, d := range maintenanceDurations {
		if i == f.Duration {
			opts = append(opts, th.Badge("up", d.Label))
		} else {
			opts = append(opts, th.Muted(d.Label))
		}
	}
	custom := "custom…"
	if f.Duration == len(maintenanceDurations) {
		if f.Custom != "" {
			custom = f.Custom
		}
		opts = append(opts, th.Badge("up", custom))
	} else {
		opts = append(opts, th.Muted(custom))
	}
	start, cancel := th.Muted(" Start "), th.Muted(" Cancel ")
	if f.Focus == 2 && f.Start {
		start = th.Badge("up", "Start")
	} else if f.Focus == 2 {
		cancel = th.Badge("up", "Cancel")
	}
	blocks := []string{th.Accent("◆") + " " + th.Bold("Maintenance"),
		th.Panel("Why?", "", why, panelWidth),
		th.Panel("How long?", "", []string{"  " + strings.Join(opts, "  ")}, panelWidth), "",
		"   " + start + "   " + cancel}
	return "\n" + margin(th, blocks) + "\n"
}

// untilText says when maintenance ends: "16:42", "14:32 tomorrow", or a date.
func untilText(until, now time.Time) string {
	until, now = until.Local(), now.Local()
	clock := until.Format("15:04")
	switch {
	case sameDay(until, now):
		return clock
	case sameDay(until, now.AddDate(0, 0, 1)):
		return clock + " tomorrow"
	}
	return until.Format("2 Jan ") + clock
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

// maintenanceResult renders what was done, for down and up, with or without Core.
func maintenanceOutcome(th *ui.Theme, enabled bool, until time.Time, now time.Time, coreTold bool) string {
	var steps []opStep
	switch {
	case enabled && coreTold:
		steps = append(steps, opStep{Mark: "ok", Name: "Maintenance on", Detail: "alerts muted until " + untilText(until, now)})
	case enabled:
		steps = append(steps, opStep{Mark: "ok", Name: "Maintenance on", Detail: "until " + untilText(until, now)})
	default:
		steps = append(steps, opStep{Mark: "ok", Name: "Maintenance off", Detail: "alerts resume"})
	}
	if coreTold {
		steps = append(steps, opStep{Mark: "ok", Name: "Core told"})
		return "\n" + opLines(th, steps)
	}
	steps = append(steps, opStep{Mark: "warn", Name: "Local only", Evidence: []string{"This host is not enrolled, so Core was not told.", "Connect it first:  orion-node setup"}})
	return "\n" + opLines(th, steps)
}

// maintenanceResultPanel is the form's finish: a panel and the short form to repeat.
func maintenanceResultPanel(th *ui.Theme, until, now time.Time, reason string, d time.Duration) string {
	steps := []opStep{{Mark: "ok", Name: "Maintenance on", Detail: "alerts muted until " + untilText(until, now)}, {Mark: "ok", Name: "Core told"}}
	short := "orion-node maintenance down"
	if reason != "" {
		short += fmt.Sprintf(" %q", reason)
	}
	short += " --for " + shortDuration(d)
	return "\n" + margin(th, []string{opPanel(th, "Result", "", steps)}) + "\n\n  " + th.Muted("Short form:") + "  " + th.Bold(short) + "\n"
}

// shortDuration writes 2h, 90m or 45s the way --for accepts it.
func shortDuration(d time.Duration) string {
	switch {
	case d%time.Hour == 0:
		return fmt.Sprintf("%dh", d/time.Hour)
	case d%time.Minute == 0:
		return fmt.Sprintf("%dm", d/time.Minute)
	}
	return d.String()
}
