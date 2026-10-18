package cli

import (
	"strings"
	"testing"
	"time"

	"orion/agent/internal/control"
	"orion/agent/internal/ui"
)

func artboardStatus() (control.Status, time.Time) {
	now := time.Date(2026, 10, 18, 14, 32, 10, 0, time.Local)
	return control.Status{
		Name: "edge-05", Version: "0.9.2", Registered: true, CoreURL: "https://core.example.com",
		UptimeSeconds: int64((3*24*time.Hour + 4*time.Hour).Seconds()), LastReportAt: time.Date(2026, 10, 18, 14, 32, 8, 0, time.Local),
		QueuePending: 0, Mode: "system service, user orion", Monitors: 5, LogSources: 2,
		MonitorStates: []control.MonitorState{
			{Name: "nginx", Kind: "systemd-service", Health: "up", At: now.Add(-12 * time.Second)},
			{Name: "postgresql", Kind: "systemd-service", Health: "up", At: now.Add(-12 * time.Second)},
			{Name: "api", Kind: "docker-container", Health: "slow", At: now.Add(-12 * time.Second)},
			{Name: "disk /var", Kind: "disk-usage", Health: "degraded", At: now.Add(-30 * time.Second)},
			{Name: "redis", Kind: "tcp", Health: "down", At: now.Add(-30 * time.Second)},
		},
		TopApps: []control.AppUsage{{Name: "nginx", CPUPercent: 4.1}, {Name: "postgresql", CPUPercent: 2.8}, {Name: "api", CPUPercent: 1.2}},
	}, now
}

// The cli-status artboard, character for character. Two deliberate differences:
// the agent cannot know the latest release, so "(latest)" after the version is
// not printed, and a monitor that is up/slow/down shows that word (the artboard
// shows "91%" for the disk row, which needs the metric value in the status).
const artboardStatusText = `
╭─ edge-05 ────────────────────────────────────────────────────╮
│ ● connected  to https://core.example.com
│ 
│ Version     0.9.2
│ Uptime      3d 4h
│ Last report 14:32:08 (2 s ago)
│ Queue       0 pending
│ Mode        system service, user orion
╰──────────────────────────────────────────────────────────────╯

Watching  5 monitors, 2 log sources

  NAME            KIND        STATUS     LAST
  ────────────────────────────────────────────────────
  nginx           systemd     ● up       12 s
  postgresql      systemd     ● up       12 s
  api             container   ◐ slow     12 s
  disk /var       disk        ◐ slow     30 s
  redis           port        ■ down     30 s

Using now  top apps by CPU
  nginx 4.1%   postgresql 2.8%   api 1.2%
`

func TestScreenStatusMatchesArtboard(t *testing.T) {
	st, now := artboardStatus()
	for _, dark := range []bool{false, true} {
		got := stripANSI(statusScreen(ui.WithMode(dark), st, now, 120))
		if got != artboardStatusText {
			t.Fatalf("status differs from the artboard (dark=%v).\n--- artboard\n%s\n--- got\n%s", dark, artboardStatusText, got)
		}
	}
	screenGolden(t, "status-light-120", statusScreen(ui.WithMode(false), st, now, 120))
	screenGolden(t, "status-dark-120", statusScreen(ui.WithMode(true), st, now, 120))
}

func TestScreenStatusPlainNarrowAndStates(t *testing.T) {
	st, now := artboardStatus()
	plain := statusScreen(ui.Plain(), st, now, 80)
	if strings.Contains(plain, "\x1b") || !strings.Contains(plain, "[ok]") || !strings.Contains(plain, "[down]") || !strings.Contains(plain, "[slow]") || strings.Contains(plain, "[slow] slow") {
		t.Fatalf("plain status must spell states in brackets:\n%s", plain)
	}
	for _, l := range strings.Split(plain, "\n") {
		if len([]rune(l)) > 80 {
			t.Errorf("wider than 80 columns (%d): %q", len([]rune(l)), l)
		}
	}
	screenGolden(t, "status-plain-80", plain)

	st.Registered, st.CoreURL = false, ""
	st.MonitorStates, st.TopApps = nil, nil
	screenGolden(t, "status-notenrolled-80", statusScreen(ui.Plain(), st, now, 80))

	st.Registered, st.MaintenanceMode = true, true
	if !strings.Contains(stripANSI(statusScreen(ui.WithMode(false), st, now, 80)), "‖ maintenance") {
		t.Fatal("maintenance must show the pause glyph and the word")
	}
	stopped := statusStopped(ui.Plain(), "edge-05", "service stopped", "orion-agent service start", 80)
	screenGolden(t, "status-stopped-80", stopped)
}
