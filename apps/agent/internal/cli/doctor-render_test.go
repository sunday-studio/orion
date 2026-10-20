package cli

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"orion/agent/internal/doctor"
	"orion/agent/internal/ui"
)

var updateScreens = flag.Bool("update-screens", false, "rewrite CLI screen goldens")

var ansi = regexp.MustCompile(`\x1b\[[0-9;?]*[a-zA-Z]`)

func stripANSI(s string) string { return ansi.ReplaceAllString(s, "") }

func screenGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", "screens", name+".txt")
	if *updateScreens {
		os.MkdirAll(filepath.Dir(path), 0o755)
		os.WriteFile(path, []byte(got), 0o644)
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("missing golden %s (run go test ./internal/cli -run Screen -update-screens): %v", path, err)
	}
	if string(want) != got {
		t.Fatalf("screen %s differs from its golden.\n--- want\n%s\n--- got\n%s", name, want, got)
	}
}

func dr(group, id, title string, st doctor.Status, msg string, took int64, fix string, evidence ...string) doctor.Result {
	return doctor.Result{ID: id, Group: group, Label: id, Title: title, Status: st, Message: msg, TookMS: took, Fix: fix, Evidence: evidence}
}

func skipped(n int) []doctor.Result {
	var out []doctor.Result
	for i := 0; i < n; i++ {
		out = append(out, doctor.Result{ID: "skipped", Status: doctor.Skip, Group: "System"})
	}
	return out
}

// runningReport is the "running" board: 16 of 18 checks done, one problem in
// Discovery and one in Logs, the spool check still running.
func runningReport() []doctor.Result {
	rs := []doctor.Result{
		dr("Agent", "config", "Config valid", doctor.OK, "/etc/orion/config.yaml", 4, ""),
		dr("Agent", "service", "Service running", doctor.OK, "systemd · pid 2114", 0, ""),
		dr("Agent", "state", "State database", doctor.OK, "", 0, ""),
		dr("Core", "core", "Reachable", doctor.OK, "core.example.com", 38, ""),
		dr("Core", "token", "Credentials accepted", doctor.OK, "", 0, ""),
		dr("Core", "clock", "Clock within 1 s", doctor.OK, "", 0, ""),
		dr("Discovery", "discovery", "Last scan", doctor.OK, "14:30 · found 7 things", 0, ""),
		dr("Discovery", "discovery-access", "Docker is not readable by the agent", doctor.Warn, "", 0,
			"sudo usermod -aG docker orion   (this access is powerful)", "Containers are not scanned, so none can be suggested."),
		dr("Logs", "log-keys", "Log key for nginx is missing", doctor.Warn, "", 0, "orion-agent setup --logs nginx"),
	}
	return append(rs, skipped(7)...)
}

func TestScreenDoctorRunningMatchesBoard(t *testing.T) {
	board := boardBlock(t, "cli-doctor.txt", "doctor  ·  running")
	_, board, _ = strings.Cut(board, "\n")
	spool := doctor.Check{ID: "spool", Group: "Logs", Label: "Spool writable"}
	for _, dark := range []bool{false, true} {
		got := stripANSI(doctorGrouped(ui.WithMode(dark), "edge-05", runningReport(), 18, &spool, 2))
		sameAsBoard(t, "doctor running", board, got)
	}
}

func TestScreenDoctorAllClearMatchesBoard(t *testing.T) {
	board := boardBlock(t, "cli-doctor.txt", "doctor  ·  all clear")
	_, board, _ = strings.Cut(board, "\n")
	board, _, _ = strings.Cut(board, "\n(when everything")
	got := stripANSI(doctorCollapsed(ui.WithMode(false), "edge-05", doctorCounts{pass: 16}, 900*time.Millisecond))
	if norm(got) != norm(board) {
		t.Errorf("all clear differs.\n--- board\n%s\n--- got\n%s", norm(board), norm(got))
	}
}

// failingReport has the two failures the board shows. The board lists one
// passing check and counts a warning it does not show; this screen lists every
// warning and no passes, so the expected text is adjusted below.
func failingReport() []doctor.Result {
	rs := []doctor.Result{
		dr("Core", "core", "Core is not reachable", doctor.Fail, "", 0, "check the address, then orion-agent config edit", "dial tcp 10.0.0.5:8999: connection refused"),
		dr("Core", "token", "Credentials rejected by Core", doctor.Fail, "", 0, "orion-agent token apply  with a replacement token from the Console", "Core answered 401 for this server's token"),
	}
	for i := 0; i < 14; i++ {
		rs = append(rs, doctor.Result{ID: "ok", Group: "System", Title: "x", Status: doctor.OK})
	}
	return rs
}

func TestScreenDoctorFailingMatchesBoard(t *testing.T) {
	board := boardBlock(t, "cli-doctor.txt", "doctor  ·  failing")
	_, board, _ = strings.Cut(board, "\n")
	board, _, _ = strings.Cut(board, "\n$ echo $?")
	board = strings.Replace(board, "   ✓ Config valid\n", "", 1)
	board = strings.Replace(board, "13 passed   ! 1 warning ", "14 passed   ! 0 warnings", 1)
	got := stripANSI(doctorFailing(ui.WithMode(false), failingReport()))
	sameAsBoardLines(t, "doctor failing", board, got)
}

// sameAsBoardLines compares text after collapsing runs of spaces.
func sameAsBoardLines(t *testing.T, name, board, got string) {
	t.Helper()
	if norm(got) != norm(board) {
		t.Errorf("%s differs from the board.\n--- board\n%s\n--- got\n%s", name, norm(board), norm(got))
	}
}

func TestScreenDoctorGoldensAndPlain(t *testing.T) {
	spool := doctor.Check{ID: "spool", Group: "Logs", Label: "Spool writable"}
	screenGolden(t, "doctor-running-light", doctorGrouped(ui.WithMode(false), "edge-05", runningReport(), 18, &spool, 2))
	screenGolden(t, "doctor-running-dark", doctorGrouped(ui.WithMode(true), "edge-05", runningReport(), 18, &spool, 2))
	screenGolden(t, "doctor-failing-light", doctorFailing(ui.WithMode(false), failingReport()))
	plain := doctorFailing(ui.Plain(), failingReport())
	if strings.Contains(plain, "\x1b") || strings.ContainsAny(plain, "✓✗╭│") || !strings.Contains(plain, "[fail]") || !strings.Contains(plain, "fix: ") {
		t.Fatalf("plain mode must use [ok]/[warn]/[fail] and no decoration:\n%s", plain)
	}
	if !strings.Contains(plain, "14 passed   0 warnings   2 failed   exit 1") {
		t.Fatalf("plain summary:\n%s", plain)
	}
	screenGolden(t, "doctor-plain", plain)
	clear := doctorGrouped(ui.Plain(), "edge-05", []doctor.Result{dr("Core", "dns", "DNS", doctor.OK, "x", 0, "")}, 1, nil, 0)
	if !strings.Contains(clear, "1 passed   0 warnings   0 failed   exit 0") {
		t.Fatalf("all-ok plain summary:\n%s", clear)
	}
}
