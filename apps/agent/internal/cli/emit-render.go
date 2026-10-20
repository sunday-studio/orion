package cli

import (
	"fmt"
	"strings"
	"time"

	"orion/agent/internal/ui"
)

// jobDuration writes a run time the short way: 12 ms, 41 s, 3 m 5 s.
func jobDuration(d time.Duration) string {
	switch {
	case d < time.Second:
		return fmt.Sprintf("%d ms", d.Milliseconds())
	case d < time.Minute:
		return fmt.Sprintf("%d s", int(d.Round(time.Second).Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%d m %d s", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%d h %d m", int(d.Hours()), int(d.Minutes())%60)
}

// jobSummary is the one line run-job prints to stderr after the command's own
// output: the exit code, how long it took, what was shipped, and the heartbeat.
// shipped is -1 when shipping failed (the warning says why); heartbeat is
// "ok", "failed" or "" when none was configured or it could not be sent.
func jobSummary(th *ui.Theme, name string, exitCode int, took time.Duration, shipped int, heartbeat string) string {
	parts := []string{fmt.Sprintf("exit %d", exitCode), jobDuration(took)}
	if shipped >= 0 {
		parts = append(parts, fmt.Sprintf("%d %s shipped", shipped, plural(shipped, "line", "lines")))
	}
	if heartbeat != "" {
		parts = append(parts, "heartbeat "+heartbeat)
	}
	detail := strings.Join(parts, " · ")
	if th.Plain() {
		tag := "[ok]"
		if exitCode != 0 {
			tag = "[fail]"
		}
		return tag + " " + name + " " + detail + "\n"
	}
	mark := th.Pass()
	if exitCode != 0 {
		mark = th.FailMark()
	}
	return "   " + mark + " " + th.Bold(name) + "  " + th.Muted(detail) + "\n"
}
