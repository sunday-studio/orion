package cli

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"orion/agent/internal/control"
	"orion/agent/internal/ui"
)

var (
	barRun   = regexp.MustCompile(`[━╸─]{3,}`)
	sparkRun = regexp.MustCompile(`[▁▂▃▄▅▆▇█]{3,}`)
	spaces   = regexp.MustCompile(` +`)
)

// norm compares layout and wording while ignoring the exact length of bars and
// sparklines (the board draws those by hand, with slightly different lengths
// per row) and runs of spaces; panel widths are checked separately.
func norm(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for i, l := range lines {
		l = barRun.ReplaceAllString(l, "BAR")
		l = sparkRun.ReplaceAllString(l, "SPARK")
		lines[i] = strings.TrimRight(spaces.ReplaceAllString(l, " "), " ")
	}
	return " " + strings.Join(lines, "\n") + "\n"
}

func at(s string) time.Time {
	t, _ := time.ParseInLocation("15:04:05", s, time.Local)
	n := time.Now()
	return time.Date(n.Year(), n.Month(), n.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.Local)
}

func boardStatusInput() statusInput {
	now := at("14:32:10")
	st := control.Status{
		Name: "edge-05", Version: "0.9.2", Registered: true, CoreURL: "https://core.example.com", Mode: "system service, user orion",
		UptimeSeconds: int64((3*24 + 4) * 3600), LastReportAt: at("14:32:08"), ReportIntervalSeconds: 30, QueuePending: 0,
		Monitors: 5, Suggestions: 3, MissingLogKeys: 1, ReportMillis: []int{2, 3, 3, 5, 3, 2, 3, 3, 6, 3, 2, 3},
		Host: &control.HostStat{CPUPercent: 23, MemoryPercent: 61, DiskPath: "/", DiskPercent: 44, Load1: 0.42, NetRxBps: 1.2 * 1024 * 1024, NetTxBps: 340 * 1024},
		MonitorStates: []control.MonitorState{
			{Name: "nginx", Kind: "systemd-service", Health: "up", Since: now.Add(-12 * time.Second), At: now},
			{Name: "postgresql", Kind: "systemd-service", Health: "up", Since: now.Add(-12 * time.Second), At: now},
			{Name: "api", Kind: "docker-container", Health: "slow", Since: now.Add(-9 * time.Minute), At: now, Detail: "412 ms, limit 300"},
			{Name: "disk /var", Kind: "disk-usage", Health: "up", Since: now.Add(-2 * time.Hour), At: now, Value: "91%"},
			{Name: "redis", Kind: "tcp", Health: "down", Since: now.Add(-4 * time.Minute), At: now, Detail: "connection refused"},
		},
	}
	return statusInput{St: st, Now: now, Width: 120}
}

func TestScreenStatusConnectedMatchesBoard(t *testing.T) {
	want := norm(designBody(t, "status.txt", "orion-agent status"))
	for _, dark := range []bool{false, true} {
		got := stripANSI(statusScreen(ui.WithMode(dark), boardStatusInput()))
		if norm(got) != want {
			t.Fatalf("status differs from the board (dark=%v).\n--- board\n%s\n--- got\n%s", dark, designBody(t, "status.txt", "orion-agent status"), strings.TrimLeft(got, "\n"))
		}
		for _, l := range strings.Split(got, "\n") {
			if strings.ContainsAny(l, "╭╮╰╯") && len([]rune(l)) != 87 {
				t.Errorf("panel edge is %d wide, the board's is 87: %q", len([]rune(l)), l)
			}
			if strings.HasPrefix(l, " │") && len([]rune(l)) != 87 {
				t.Errorf("panel row is %d wide, the board's is 87: %q", len([]rune(l)), l)
			}
		}
	}
	screenGolden(t, "status-light", statusScreen(ui.WithMode(false), boardStatusInput()))
}

func TestScreenStatusSixStatesMatchBoard(t *testing.T) {
	now := at("14:32:10")
	base := control.Status{Name: "edge-05", Registered: true, ReportIntervalSeconds: 30}
	cases := []struct {
		name string
		in   statusInput
	}{
		{"not enrolled", statusInput{St: control.Status{Name: "edge-05"}, Now: now}},
		{"waiting", statusInput{St: base, Now: now}},
		{"no recent report", statusInput{St: func() control.Status { s := base; s.LastReportAt = at("14:21:08"); s.QueuePending = 312; return s }(), Now: now}},
		{"maintenance", statusInput{St: func() control.Status {
			s := base
			s.LastReportAt = now
			s.MaintenanceMode, s.MaintenanceReason = true, "kernel upgrade"
			u := at("16:42:00")
			s.MaintenanceUntil = &u
			return s
		}(), Now: at("14:50:00")}},
		{"not running", statusInput{St: control.Status{Name: "edge-05"}, Now: now, Stopped: true, Why: "the service is stopped", Hint: "orion-agent service start"}},
	}
	board := designBody(t, "status-states.txt", "orion-agent status")
	chunks := strings.Split(board, "──────────────────────────────────────────────────────────────────────────────────────\n")
	if len(chunks) != len(cases) {
		t.Fatalf("board has %d states, test has %d", len(chunks), len(cases))
	}
	for i, c := range cases {
		got := strings.TrimLeft(stripANSI(statusScreen(ui.WithMode(true), c.in)), "\n")
		if norm(got) != norm(chunks[i]) {
			t.Errorf("%s differs from the board.\n--- board\n%q\n--- got\n%q", c.name, norm(chunks[i]), norm(got))
		}
	}
}

func TestScreenStatusDiscoveryStatesMatchBoard(t *testing.T) {
	now := at("14:32:10")
	// first start: host card with the scan still running
	first := statusInput{Now: now, St: control.Status{Name: "edge-05", Registered: true, CoreURL: "https://core.example.com", Version: "0.9.2", Mode: "system service, user orion",
		LastReportAt: now, ReportIntervalSeconds: 30, Scanning: true, Host: &control.HostStat{CPUPercent: 23, MemoryPercent: 61, DiskPath: "/", DiskPercent: 44, NetRxBps: 1.2 * 1024 * 1024, NetTxBps: 340 * 1024}}}
	got := stripANSI(statusScreen(ui.WithMode(true), first))
	if !strings.Contains(got, "Looking at this server…   a few seconds, read only") || !strings.Contains(got, "Host") {
		t.Fatalf("first start:\n%s", got)
	}
	// after the scan: nothing monitored yet, things found
	after := first
	after.St.Scanning, after.St.Host, after.St.ThingsFound, after.St.Suggestions = false, nil, 7, 6
	got = stripANSI(statusScreen(ui.WithMode(true), after))
	for _, want := range []string{"● Watching 0 monitors   7 things found", "▸ orion-agent setup  6 suggestions waiting"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q:\n%s", want, got)
		}
	}
	board := designBody(t, "discovery-offline-new.txt", "orion-agent status")
	_ = board
	// offline with suggestions kept
	off := statusInput{Now: now, St: control.Status{Name: "edge-05", Registered: true, ReportIntervalSeconds: 30, LastReportAt: at("14:21:08"), Suggestions: 2}}
	got = stripANSI(statusScreen(ui.WithMode(true), off))
	for _, want := range []string{"No recent report", "agent offline since 14:21 · 2 suggestions kept", "they apply when the agent reconnects"} {
		if !strings.Contains(got, want) {
			t.Errorf("offline missing %q:\n%s", want, got)
		}
	}
	// new on this server
	newIn := first
	newIn.St.Scanning, newIn.St.Host, newIn.St.Suggestions = false, nil, 2
	newIn.St.NewItems = []control.DiscoveryItem{
		{ID: "docker:cache-warmer", Kind: "container", FirstSeen: at("14:40:00")},
		{ID: "port:9090", Kind: "port", Detail: "listening, prometheus"}}
	got = stripANSI(statusScreen(ui.WithMode(true), newIn))
	for _, want := range []string{"New on this server", "2 new", "! docker:cache-warmer", "container · started 14:40", "prometheus, listening", "▸ orion-agent setup  review them. Nothing is added silently"} {
		if !strings.Contains(got, want) {
			t.Errorf("new-on-server missing %q:\n%s", want, got)
		}
	}
}

func TestScreenStatusPlainMatchesOutputModesBoard(t *testing.T) {
	in := boardStatusInput()
	in.St.Suggestions, in.St.MissingLogKeys = 0, 0
	in.St.MonitorStates[4].Detail = "port 6379 refused"
	in.St.MonitorStates[2].Detail = "412 ms over limit"
	got := statusScreen(ui.Plain(), in)
	want := designBody(t, "output-plain.txt", "NO_COLOR=1 orion-agent status        (or any pipe)")
	want = strings.Split(want, "\n\n")[0] + "\n"
	// board lines: "[down]  redis       port 6379 refused, 4 m"
	g := norm(got)
	for _, line := range strings.Split(strings.TrimRight(norm(want), "\n"), "\n") {
		if strings.HasPrefix(line, "monitors") || strings.HasPrefix(line, "edge-05") || strings.HasPrefix(line, "last report") || strings.HasPrefix(line, "next:") {
			if !strings.Contains(g, line) {
				t.Errorf("plain output lacks the board line %q:\n%s", line, got)
			}
		}
	}
	if strings.ContainsAny(got, "╭╮╰╯│━╸─◆●◐■○▸") || strings.Contains(got, "\x1b") {
		t.Fatalf("plain must have no colour, box or glyphs:\n%s", got)
	}
	for _, l := range []string{"[down]", "[slow]", "redis", "api"} {
		if !strings.Contains(got, l) {
			t.Errorf("plain lacks %q", l)
		}
	}
	if strings.Contains(got, "nginx") {
		t.Error("healthy monitors are not listed in plain mode")
	}
	screenGolden(t, "status-plain", got)
}

func TestStatusShortIsOneLine(t *testing.T) {
	got := statusShort(boardStatusInput())
	if strings.Count(got, "\n") != 1 || !strings.Contains(got, "3 up 1 slow 1 down") || !strings.Contains(got, "edge-05") {
		t.Fatalf("short = %q", got)
	}
	off := statusShort(statusInput{St: control.Status{Name: "edge-05"}, Stopped: true})
	if off != "edge-05 not running\n" {
		t.Fatalf("short stopped = %q", off)
	}
}
