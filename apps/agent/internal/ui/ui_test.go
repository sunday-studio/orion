package ui

import (
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

var update = flag.Bool("update", false, "rewrite snapshots")

func snapshot(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name+".txt")
	if *update {
		os.MkdirAll("testdata", 0o755)
		os.WriteFile(path, []byte(got), 0o644)
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("missing snapshot %s (run with -update): %v", path, err)
	}
	if string(want) != got {
		t.Fatalf("snapshot %s differs.\n--- want\n%s\n--- got\n%s", name, want, got)
	}
}

func screen(th *Theme, width int) string {
	var b strings.Builder
	b.WriteString(th.Header("status") + "\n")
	b.WriteString(th.KV([][2]string{{"service", th.Status("up")}, {"core", "https://core.example.com"}, {"reporting", th.Status("maintenance") + " deploying"}}) + "\n\n")
	b.WriteString(th.Table([]string{"Check", "Result", "Detail"}, [][]string{
		{"config", "ok", "/etc/orion/config.yaml, 3 monitor(s)"},
		{"core", "FAIL", "connection refused: check the network path to Core and the core_url value"},
	}, width) + "\n\n")
	b.WriteString(th.Callout(Warning, "the clock differs from Core by 2m") + "\n")
	for _, s := range []StepState{StepOK, StepWarn, StepFail, StepSkip, StepRunning} {
		b.WriteString(th.Step(s, "check "+string(s)) + "\n")
	}
	return b.String()
}

func TestSnapshotsPlain80And120(t *testing.T) {
	for _, w := range []int{80, 120} {
		snapshot(t, "plain-"+itoa(w), screen(Plain(), w))
	}
}

func TestSnapshotsColourLightAndDark(t *testing.T) {
	snapshot(t, "colour-light", screen(WithMode(false), 80))
	snapshot(t, "colour-dark", screen(WithMode(true), 80))
}

func itoa(i int) string { return strconv.Itoa(i) }

func TestFits80Columns(t *testing.T) {
	for _, line := range strings.Split(screen(Plain(), 80), "\n") {
		if lipgloss.Width(line) > 80 {
			t.Errorf("line wider than 80 columns (%d): %q", lipgloss.Width(line), line)
		}
	}
}

// Meaning never rides on colour alone: plain output still names every status.
func TestStatusNeverColourOnly(t *testing.T) {
	th := Plain()
	for _, name := range StatusNames() {
		out := th.Status(name)
		if !strings.Contains(out, name) || len([]rune(out)) <= len([]rune(name)) {
			t.Errorf("status %q renders %q without a glyph and name", name, out)
		}
		if strings.Contains(out, "\x1b") {
			t.Errorf("plain theme emitted ANSI for %q", name)
		}
	}
	// Shapes differ for the statuses that matter most.
	if th.StatusGlyph("up") == th.StatusGlyph("down") || th.StatusGlyph("up") == th.StatusGlyph("degraded") {
		t.Error("up, down and degraded must have distinct shapes")
	}
}

func TestNoColorEnvDisablesColour(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if Detect(os.Stdout).ColorEnabled() {
		t.Fatal("NO_COLOR must disable colour")
	}
}

func TestNonTTYWriterIsPlain(t *testing.T) {
	var sb strings.Builder
	if Detect(&sb).ColorEnabled() {
		t.Fatal("a non-terminal writer must get plain output")
	}
}

// The generated theme must match docs/design/tokens.json (CI also diffs it).
func TestGeneratedThemeIsCurrent(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not on PATH")
	}
	tmp := filepath.Join(t.TempDir(), "theme_gen.go")
	cmd := exec.Command("go", "run", "./gen-theme", "-in", "../../../../docs/design/tokens.json", "-out", tmp)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generator failed: %v\n%s", err, out)
	}
	want, _ := os.ReadFile(tmp)
	got, _ := os.ReadFile("theme_gen.go")
	if string(want) != string(got) {
		t.Fatal("theme_gen.go is stale: run `make agent-theme` after changing docs/design/tokens.json")
	}
}
