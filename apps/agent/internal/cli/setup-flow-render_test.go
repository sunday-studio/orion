package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"orion/agent/internal/control"
	"orion/agent/internal/ui"
)

// boardBlock returns the terminal text of one block of a board dump in
// testdata/boards (blocks start with "=== title").
func boardBlock(t *testing.T, file, title string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "boards", file))
	if err != nil {
		t.Fatal(err)
	}
	for _, chunk := range strings.Split(string(b), "=== ")[1:] {
		head, body, _ := strings.Cut(chunk, "\n")
		if head == title {
			body, _, _ = strings.Cut(body, "\n--- notes")
			return body
		}
	}
	t.Fatalf("%s has no block %q", file, title)
	return ""
}

func sameAsBoard(t *testing.T, name, board, got string) {
	t.Helper()
	if norm(got) != norm(board) {
		t.Errorf("%s differs from the board.\n--- board\n%s\n--- got\n%s", name, norm(board), norm(got))
	}
	for _, l := range strings.Split(got, "\n") {
		if strings.ContainsAny(l, "╭╮╰╯") && len([]rune(l)) != 87 {
			t.Errorf("%s: panel edge is %d wide, the board's is 87: %q", name, len([]rune(l)), l)
		}
		if strings.HasPrefix(l, " │") && len([]rune(l)) != 87 {
			t.Errorf("%s: panel row is %d wide, the board's is 87: %q", name, len([]rune(l)), l)
		}
	}
}

func setupItems() []control.DiscoveryItem {
	return []control.DiscoveryItem{
		{ID: "service:nginx", Kind: "service", Name: "nginx", Confidence: "high", Status: "suggested"},
		{ID: "service:postgresql", Kind: "service", Name: "postgresql", Confidence: "high", Status: "suggested"},
		{ID: "service:redis", Kind: "service", Name: "redis", Confidence: "high", Status: "monitored", ManagedIn: "file"},
		{ID: "container:api", Kind: "container", Name: "api", Confidence: "high", Status: "suggested"},
		{ID: "port:6379", Kind: "port", Name: ":6379", Detail: "listening, redis", Confidence: "medium", Status: "suggested"},
		{ID: "disk:/var", Kind: "disk", Name: "/var", Confidence: "high", Status: "suggested"},
		{ID: "log:/var/log/nginx/error.log", Kind: "log", Name: "/var/log/nginx/error.log", Confidence: "medium", Status: "suggested"},
	}
}

func TestSetupConnectMatchesBoard(t *testing.T) {
	th := ui.WithMode(false)
	got := stripANSI(connectScreen(th, connectView{URL: "https://core.example.com", Token: strings.Repeat("a", 20), Check: "ok", Latency: 38 * time.Millisecond, Version: "1.4.0"}))
	sameAsBoard(t, "connect", boardBlock(t, "cli-setup.txt", "setup  ·  connect"), got)
}

func TestSetupChooseMatchesBoard(t *testing.T) {
	th := ui.WithMode(false)
	ticked := map[string]bool{"service:nginx": true, "service:postgresql": true, "container:api": true, "disk:/var": true}
	got := stripANSI(chooseScreen(th, chooseView{Items: setupItems(), Ticked: ticked, Cursor: -1, Enrolled: true}))
	sameAsBoard(t, "choose", boardBlock(t, "cli-setup.txt", "setup  ·  choose, already enrolled"), got)
}

func TestSetupScanSettingsMatchesBoard(t *testing.T) {
	th := ui.WithMode(false)
	got := stripANSI(scanSettingsScreen(th, scanSettingsView{Enabled: true, Enrolled: true, Cursor: -1,
		Kinds: map[string]bool{"services": true, "containers": true, "ports": true, "disks": true, "logs": true},
		Never: []neverItem{{ID: "docker:legacy-cron", When: "dismissed 12 Oct"}}}))
	sameAsBoard(t, "scan settings", boardBlock(t, "cli-setup.txt", "setup  ·  scan settings"), got)
}

func TestSetupReviewMatchesBoard(t *testing.T) {
	th := ui.WithMode(false)
	got := stripANSI(reviewScreen(th, reviewView{Path: "/etc/orion/config.yaml", Apply: true, Enrolled: true,
		Rows: []string{"+ monitors:",
			"+   - name: nginx        type: systemd-service",
			"+   - name: postgresql   type: systemd-service",
			"+   - name: api          type: docker-container",
			"+   - name: /var         type: disk-usage  threshold: 90%"},
		NotWrite: []string{"nginx error.log needs a log key. Create one in the Console."}}))
	sameAsBoard(t, "review", boardBlock(t, "cli-setup.txt", "setup  ·  review"), got)
}

func TestSetupAppliedMatchesBoard(t *testing.T) {
	th := ui.WithMode(false)
	got := stripANSI(appliedScreen(th, appliedView{Path: "/etc/orion/config.yaml", Reloaded: true, Monitors: []appliedMonitor{
		{Name: "nginx"}, {Name: "postgresql"}, {Name: "api"}, {Name: "/var", Ready: true, Since: 4 * time.Second}}, Frame: 2}))
	sameAsBoard(t, "applied", boardBlock(t, "cli-setup.txt", "setup  ·  applied"), got)
}

func TestSetupNonInteractiveMatchesBoard(t *testing.T) {
	th := ui.WithMode(false)
	got := stripANSI(setupSummary(th, []setupStep{
		{Mark: "ok", Name: "Scanned", Detail: "found 7 things", Took: 3100 * time.Millisecond},
		{Mark: "ok", Name: "Accepted", Detail: "4 high-confidence · 1 already monitored"},
		{Mark: "warn", Name: "Skipped", Detail: "nginx error.log needs a log key"},
		{Mark: "ok", Name: "Applied", Detail: "no restart"},
	}, false))
	board := boardBlock(t, "cli-setup.txt", "non-interactive")
	_, board, _ = strings.Cut(board, "\n")
	if norm(got) != norm(board) {
		t.Errorf("non-interactive differs.\n--- board\n%s\n--- got\n%s", norm(board), norm(got))
	}
}

func TestSetupPlainScreensHaveNoBoxes(t *testing.T) {
	th := ui.Plain()
	for _, s := range []string{
		connectScreen(th, connectView{URL: "https://c", Token: "x"}),
		chooseScreen(th, chooseView{Items: setupItems(), Cursor: -1}),
		reviewScreen(th, reviewView{Path: "/etc/orion/config.yaml", Rows: []string{"+ monitors:"}}),
		appliedScreen(th, appliedView{Path: "/etc/orion/config.yaml", Reloaded: true}),
	} {
		if strings.ContainsAny(s, "╭│╰◉◯") || ansi.MatchString(s) {
			t.Errorf("plain output has decoration:\n%s", s)
		}
	}
}

func TestChooseScrollsALongListAroundTheCursor(t *testing.T) {
	th := ui.WithMode(false)
	var items []control.DiscoveryItem
	for i := 0; i < 30; i++ {
		items = append(items, control.DiscoveryItem{ID: fmt.Sprintf("port:%d", 9000+i), Kind: "port", Name: fmt.Sprintf(":%d", 9000+i), Confidence: "medium", Status: "suggested"})
	}
	for _, cursor := range []int{0, 15, 29} {
		out := stripANSI(chooseScreen(th, chooseView{Items: items, Ticked: map[string]bool{}, Cursor: cursor, Enrolled: true, Height: 30}))
		lines := strings.Count(out, "\n")
		if lines > 30 {
			t.Fatalf("cursor %d: %d lines do not fit 30 rows:\n%s", cursor, lines, out)
		}
		if !strings.Contains(out, fmt.Sprintf(":%d ", 9000+cursor)) {
			t.Errorf("cursor %d: the selected row is not in view:\n%s", cursor, out)
		}
		if !strings.Contains(out, "↓") && cursor == 0 || !strings.Contains(out, "↑") && cursor == 29 {
			t.Errorf("cursor %d: no hint that more rows exist:\n%s", cursor, out)
		}
		if !strings.Contains(out, "Set up this host") && !strings.Contains(out, "Connect") {
			t.Errorf("the header must stay in view:\n%s", out)
		}
	}
}
