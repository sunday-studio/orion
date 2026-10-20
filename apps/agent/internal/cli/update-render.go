package cli

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"orion/agent/internal/ui"
)

// updateStep is one line of the update checklist.
type updateStep struct {
	Name   string
	Detail string
	Took   string
	State  string // done, running, todo, fail
}

func stepMark(th *ui.Theme, s updateStep, frame int) string {
	switch s.State {
	case "done":
		return th.Pass()
	case "fail":
		return th.FailMark()
	case "running":
		return th.Tone("degraded", th.Spinner(frame))
	}
	return th.Muted("○")
}

// updatePanel is the live checklist: steps in a panel, then the progress bar.
func updatePanel(th *ui.Theme, from, to string, steps []updateStep, frame int) string {
	done := 0
	rows := make([]string, len(steps))
	for i, s := range steps {
		if s.State == "done" {
			done++
		}
		took := ""
		if s.Took != "" {
			took = s.Took
		}
		rows[i] = " " + stepMark(th, s, frame) + " " + padRight(s.Name, 30) + th.Muted(padRight(clip(s.Detail, 42), 43)) + th.Muted(fmt.Sprintf("%5s", took)) + "   "
	}
	if th.Plain() {
		var b strings.Builder
		for _, s := range steps {
			if s.State == "done" {
				b.WriteString("[ok] " + s.Name + " " + s.Detail + "\n")
			}
		}
		return b.String()
	}
	left := " " + th.Accent("◆") + " " + th.Bold("Update")
	right := from + " → " + to
	header := left + strings.Repeat(" ", max(2, panelWidth-visibleWidth(left)-len([]rune(right)))) + th.Muted(right)
	pct := 0
	if len(steps) > 0 {
		pct = done * 100 / len(steps)
	}
	bar := "   " + progressBar(th, done, len(steps), 40) + "  " + th.Muted(fmt.Sprintf("%d%%", pct))
	return "\n" + header + "\n" + margin(th, []string{th.Panel("Steps", fmt.Sprintf("%d of %d", done, len(steps)), rows, panelWidth)}) + "\n" + bar + "\n"
}

// updateDone is the collapsed finish.
func updateDone(th *ui.Theme, version string, took time.Duration) string {
	return "\n   " + th.Pass() + " " + fmt.Sprintf("Updated to %s in %d s", version, int(took.Round(time.Second).Seconds())) + "\n     " +
		th.Muted("previous version kept · undo with orion-agent update --rollback") + "\n"
}

// updateRolledBack is the calm ending of a failed update.
func updateRolledBack(th *ui.Theme, restored string) string {
	return "\n   " + th.Pass() + " " + "Rolled back to " + restored + "\n     " + th.Muted("the previous version is running again · logs: orion-agent logs --lines 80") + "\n"
}

// updateReporter draws the update as it happens. On a terminal it redraws the
// panel in place; otherwise it prints a line per finished step. A nil
// reporter does nothing, so tests run silent.
type updateReporter struct {
	mu       sync.Mutex
	th       *ui.Theme
	w        io.Writer
	live     bool
	from, to string
	steps    []updateStep
	drawn    int
	frame    int
	stop     chan struct{}
	started  time.Time
	stepTime time.Time
}

func newUpdateReporter(th *ui.Theme, w io.Writer, live bool, from string) *updateReporter {
	r := &updateReporter{th: th, w: w, live: live, from: from, to: "latest", started: time.Now(), stop: make(chan struct{})}
	for _, n := range []string{"Downloading", "Verifying checksum", "Stopping service", "Swapping binary", "Starting and checking health"} {
		r.steps = append(r.steps, updateStep{Name: n, State: "todo"})
	}
	if live {
		r.draw()
		go func() {
			t := time.NewTicker(120 * time.Millisecond)
			defer t.Stop()
			for {
				select {
				case <-r.stop:
					return
				case <-t.C:
					r.mu.Lock()
					r.frame++
					r.draw()
					r.mu.Unlock()
				}
			}
		}()
	}
	return r
}

func (r *updateReporter) draw() {
	if !r.live {
		return
	}
	if r.drawn > 0 {
		fmt.Fprintf(r.w, "\x1b[%dA\r\x1b[J", r.drawn)
	}
	screen := updatePanel(r.th, r.from, r.to, r.steps, r.frame)
	fmt.Fprint(r.w, screen)
	r.drawn = strings.Count(screen, "\n")
}

func (r *updateReporter) find(name string) int {
	for i, s := range r.steps {
		if s.Name == name {
			return i
		}
	}
	return -1
}

// begin marks a step as running.
func (r *updateReporter) begin(name string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if i := r.find(name); i >= 0 {
		r.steps[i].State = "running"
	}
	r.stepTime = time.Now()
	r.draw()
}

// finish completes a step under a new name, with an optional detail and time.
func (r *updateReporter) finish(name, doneName, detail string, timed bool) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	i := r.find(name)
	if i < 0 {
		return
	}
	r.steps[i].Name, r.steps[i].Detail, r.steps[i].State = doneName, detail, "done"
	if timed {
		r.steps[i].Took = fmt.Sprintf("%.1f s", time.Since(r.stepTime).Seconds())
	}
	if !r.live {
		fmt.Fprint(r.w, "   "+r.th.Pass()+" "+padRight(doneName, 30)+r.th.Muted(detail)+"\n")
	}
	r.draw()
}

// skip removes a step that does not apply (the service was not running).
func (r *updateReporter) skip(name string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if i := r.find(name); i >= 0 {
		r.steps = append(r.steps[:i], r.steps[i+1:]...)
	}
	r.draw()
}

func (r *updateReporter) setVersion(v string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.to = v
	r.draw()
	r.mu.Unlock()
}

// close stops the redraw and leaves the cursor below the screen.
func (r *updateReporter) close(clear bool) {
	if r == nil {
		return
	}
	select {
	case <-r.stop:
	default:
		close(r.stop)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.live && clear && r.drawn > 0 {
		fmt.Fprintf(r.w, "\x1b[%dA\r\x1b[J", r.drawn)
		r.drawn = 0
	}
}

// say prints a line below the checklist (a failure, a note, the finish).
func (r *updateReporter) say(s string) {
	if r == nil {
		return
	}
	fmt.Fprint(r.w, s)
}

// flatDone replaces the live panel by one plain line per finished step, so a
// failure reads as a short story.
func (r *updateReporter) flatDone() {
	if r == nil || !r.live {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range r.steps {
		if s.State == "done" {
			fmt.Fprint(r.w, "   "+r.th.Pass()+" "+padRight(s.Name, 30)+r.th.Muted(s.Detail)+"\n")
		}
	}
}
