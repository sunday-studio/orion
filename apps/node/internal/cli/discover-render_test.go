package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"orion/node/internal/control"
	"orion/node/internal/ui"
)

// designBody returns what the design board shows after its "$ command" line,
// with trailing blanks trimmed per line. The files are extracted from the CLI
// board of the design canvas (testdata/design).
func designBody(t *testing.T, name, command string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "design", name))
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	i := strings.Index(text, "$ "+command+"\n")
	if i < 0 {
		t.Fatalf("%s has no command %q", name, command)
	}
	rest := text[i+len("$ "+command+"\n"):]
	if j := strings.Index(rest, "\n$ "); j >= 0 {
		rest = rest[:j]
	}
	return trimLines(rest)
}

func trimLines(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " ")
	}
	return strings.Trim(strings.Join(lines, "\n"), "\n") + "\n"
}

func boardDiscovery() control.Discovery {
	now := time.Now()
	return control.Discovery{Enabled: true, ScannedAt: now,
		Kinds: []control.KindState{{Kind: "services", State: "scanned"}, {Kind: "containers", State: "scanned"}, {Kind: "ports", State: "scanned"}, {Kind: "disks", State: "scanned"}, {Kind: "logs", State: "scanned"}},
		Items: []control.DiscoveryItem{
			{ID: "service:nginx", Kind: "service", Name: "nginx", Confidence: "high", Status: "suggested", New: true},
			{ID: "service:postgresql", Kind: "service", Name: "postgresql", Confidence: "high", Status: "suggested"},
			{ID: "service:redis", Kind: "service", Name: "redis", Confidence: "high", Status: "monitored", ManagedIn: "file"},
			{ID: "container:api", Kind: "container", Name: "api", Confidence: "high", Status: "suggested"},
			{ID: "port:6379", Kind: "port", Name: ":6379", Detail: "listening, redis", Confidence: "medium", Status: "suggested"},
			{ID: "disk:/var", Kind: "disk", Name: "/var", Confidence: "high", Status: "suggested"},
			{ID: "log:/var/log/nginx/error.log", Kind: "log", Name: "/var/log/nginx/error.log", Confidence: "medium", Status: "suggested"},
		}}
}

func render(th *ui.Theme, d control.Discovery) string {
	return trimLines(stripANSI(discoverScreen(th, d, "edge-05", 120)))
}

func TestScreenDiscoverMatchesBoard(t *testing.T) {
	want := designBody(t, "discover.txt", "orion-node discover")
	for _, dark := range []bool{false, true} {
		if got := render(ui.WithMode(dark), boardDiscovery()); got != want {
			t.Fatalf("discover differs from the board (dark=%v).\n--- board\n%s\n--- got\n%s", dark, want, got)
		}
	}
	screenGolden(t, "discover-light", discoverScreen(ui.WithMode(false), boardDiscovery(), "edge-05", 120))
}

func TestScreenDiscoverPlainHasNoBoxes(t *testing.T) {
	out := discoverScreen(ui.Plain(), boardDiscovery(), "edge-05", 80)
	if strings.ContainsAny(out, "╭╮╰╯│─◉◯✓◆") || strings.Contains(out, "\x1b") {
		t.Fatalf("plain must have no box or glyphs:\n%s", out)
	}
	for _, want := range []string{"Services (3)", "[x] nginx", "[ ] :6379 redis", "[=] redis", "already monitored (managed in file)", "high", "medium"} {
		if !strings.Contains(out, want) {
			t.Errorf("plain output missing %q:\n%s", want, out)
		}
	}
	screenGolden(t, "discover-plain", out)
}

func TestScreenScanBlockedNothingFoundAndOff(t *testing.T) {
	blocked := control.Discovery{Enabled: true, Kinds: []control.KindState{
		{Kind: "services", State: "scanned"}, {Kind: "containers", State: "blocked", Message: "Could not look at Docker: the node has no access.",
			Fix: "sudo usermod -aG docker orion  then restart the node", Why: "Docker access is powerful: members can control this host."},
		{Kind: "ports", State: "scanned"}, {Kind: "disks", State: "scanned"}, {Kind: "logs", State: "scanned"}}}
	// the board shows services, containers, ports, disks as one scanned group: reorder is by kind order, so
	// the blocked kind splits the group exactly as the board draws it only when it comes after the others.
	blocked.Kinds = []control.KindState{{Kind: "services", State: "scanned"}, {Kind: "containers", State: "blocked", Message: blocked.Kinds[1].Message, Fix: blocked.Kinds[1].Fix, Why: blocked.Kinds[1].Why}, {Kind: "ports", State: "scanned"}, {Kind: "disks", State: "scanned"}, {Kind: "logs", State: "scanned"}}
	got := trimLines(stripANSI(discoverScreen(ui.WithMode(false), blocked, "edge-05", 120)))
	if !strings.Contains(got, "! Containers") || !strings.Contains(got, "│ Could not look at Docker: the node has no access.") ||
		!strings.Contains(got, "Fix  sudo usermod -aG docker orion  then restart the node") || !strings.Contains(got, "Docker access is powerful") || !strings.Contains(got, " 1 ! ╮") {
		t.Fatalf("blocked scan panel:\n%s", got)
	}

	nothing := control.Discovery{Enabled: true}
	if got, want := trimLines(stripANSI(discoverScreen(ui.WithMode(false), nothing, "edge-05", 120))), designBody(t, "discovery-scan-blocked-and-nothing-found.txt", "orion-node discover"); got != want {
		t.Fatalf("nothing-found differs from the board.\n--- board\n%s\n--- got\n%s", want, got)
	}
	off := control.Discovery{Enabled: false}
	if got, want := trimLines(stripANSI(discoverScreen(ui.WithMode(false), off, "edge-05", 120))), designBody(t, "discovery-scanning-off-and-a-kind-off.txt", "orion-node discover"); got != want {
		t.Fatalf("scanning-off differs from the board.\n--- board\n%s\n--- got\n%s", want, got)
	}
}

func TestScreenKindOffMatchesBoardBesidesHeader(t *testing.T) {
	d := control.Discovery{Enabled: true, Kinds: []control.KindState{{Kind: "services", State: "scanned"}, {Kind: "containers", State: "off"}},
		Items: []control.DiscoveryItem{
			{ID: "service:nginx", Kind: "service", Name: "nginx", Confidence: "high", Status: "suggested"},
			{ID: "service:postgresql", Kind: "service", Name: "postgresql", Confidence: "high", Status: "suggested"}}}
	got := stripANSI(discoverScreen(ui.WithMode(false), d, "edge-05", 120))
	lines := strings.Split(trimLines(got), "\n")
	// The board omits the "Discover" header and the footer on this state; we keep both
	// because they carry the count and the next step.
	body := trimLines(strings.Join(lines[1:len(lines)-2], "\n"))
	board := trimLines(strings.SplitN(string(mustRead(t, "discovery-scanning-off-and-a-kind-off.txt")), "$ orion-node discover\n", 3)[2])
	if body != board {
		t.Fatalf("kind-off differs from the board.\n--- board\n%s\n--- got\n%s", board, body)
	}
}

func mustRead(t *testing.T, name string) []byte {
	b, err := os.ReadFile(filepath.Join("testdata", "design", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestScreenDismissedMatchesBoard(t *testing.T) {
	d := control.Discovery{Enabled: true, Items: []control.DiscoveryItem{
		{ID: "docker:legacy-cron", Dismissed: true, DismissedAt: time.Date(2026, 10, 12, 9, 0, 0, 0, time.Local)},
		{ID: "port:8081", Dismissed: true, DismissedAt: time.Date(2026, 10, 9, 9, 0, 0, 0, time.Local)}}}
	got := trimLines(stripANSI(dismissedScreen(ui.WithMode(false), d, 120)))
	want := designBody(t, "discovery-dismissed-ignore-and-restore.txt", "orion-node discover --dismissed")
	if got != want {
		t.Fatalf("dismissed differs from the board.\n--- board\n%s\n--- got\n%s", want, got)
	}
}
