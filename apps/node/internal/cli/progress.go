package cli

import (
	"fmt"
	"time"
)

// runWithProgress shows a spinner and the elapsed time while fn runs, so a slow stop, start or wait
// never looks like a hang. Where the output is not a terminal it prints one plain line instead,
// because redrawing in place would fill a log with carriage returns.
func runWithProgress(label string, fn func()) {
	if theme.Plain() || !stdoutIsTerminal() {
		fmt.Fprintln(outputWriter, "  "+label+"...")
		fn()
		return
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		fn()
	}()
	began := time.Now()
	tick := time.NewTicker(120 * time.Millisecond)
	defer tick.Stop()
	for frame := 0; ; frame++ {
		elapsed := int(time.Since(began).Seconds())
		fmt.Fprintf(outputWriter, "\r\x1b[2K %s %s", theme.Tone("degraded", theme.Spinner(frame)), theme.Muted(fmt.Sprintf("%s... %ds", label, elapsed)))
		select {
		case <-done:
			fmt.Fprint(outputWriter, "\r\x1b[2K")
			return
		case <-tick.C:
		}
	}
}
