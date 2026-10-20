package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"orion/agent/internal/ui"
)

func TestUpdateScreensMatchBoard(t *testing.T) {
	th := ui.WithMode(false)
	board := boardBlock(t, "cli-update.txt", "update")
	_, board, _ = strings.Cut(board, "\n")
	panelBoard, rest, _ := strings.Cut(board, "\n  …and when it finishes")
	steps := []updateStep{
		{Name: "Downloaded", Detail: "24.1 MB", Took: "2.8 s", State: "done"},
		{Name: "Checksum matches", State: "done"},
		{Name: "Service stopped", State: "done"},
		{Name: "Swapping binary", State: "running"},
		{Name: "Starting and checking health", State: "todo"},
	}
	for _, dark := range []bool{false, true} {
		got := stripANSI(updatePanel(ui.WithMode(dark), "0.9.2", "0.9.3", steps, 2))
		sameAsBoard(t, "update panel", panelBoard, got)
	}
	collapsed := stripANSI(updateDone(th, "0.9.3", 14*time.Second))
	want := strings.TrimSpace(rest[strings.Index(rest, "✓ Updated"):])
	if norm(collapsed) != norm(want) {
		t.Errorf("collapsed differs.\n--- board\n%s\n--- got\n%s", norm(want), norm(collapsed))
	}

	fail := boardBlock(t, "cli-update.txt", "update  ·  fails and rolls back")
	_, fail, _ = strings.Cut(fail, "\n")
	fail, _, _ = strings.Cut(fail, "\n$ echo $?")
	fail = strings.Replace(fail, "the host kept reporting the whole time", "the previous version is running again", 1)
	var out bytes.Buffer
	r := newUpdateReporter(th, &out, false, "0.9.2")
	r.begin("Swapping binary")
	r.finish("Swapping binary", "Swapped binary", "", false)
	out.WriteString("   " + stripANSI(th.FailMark()) + " New version is not healthy after 30 s\n     │ exit status 1: state database migration failed\n   ⠹ Rolling back…\n")
	out.WriteString(updateRolledBack(th, "0.9.2"))
	if norm(stripANSI(out.String())) != norm(fail) {
		t.Errorf("rollback differs.\n--- board\n%s\n--- got\n%s", norm(fail), norm(stripANSI(out.String())))
	}
}

func TestFailHeadlineNamesTheHealthWait(t *testing.T) {
	if got := failHeadline(errors.New("new version did not become healthy: boom"), 30*time.Second); got != "New version is not healthy after 30 s" {
		t.Errorf("headline = %q", got)
	}
}

func TestUpdateReporterNeverDrawsAfterClose(t *testing.T) {
	var out bytes.Buffer
	r := newUpdateReporter(ui.WithMode(false), &out, true, "1.0.0")
	r.close(true)
	before := out.Len()
	r.mu.Lock()
	r.draw()
	r.mu.Unlock()
	if out.Len() != before {
		t.Fatal("a closed reporter drew a frame")
	}
	r.close(true) // closing twice is fine
}
