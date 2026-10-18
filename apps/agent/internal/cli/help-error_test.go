package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"orion/agent/internal/config"
	"orion/agent/internal/ui"
)

// The artboard's help layout; the trailing "More:" line is an addition so
// commands outside the artboard (enroll, discover, usage, ...) stay discoverable.
const artboardHelpText = `
orion-agent  the Orion monitoring agent

USAGE
  orion-agent [command] [flags]

GET STARTED
  setup     Connect this host and choose what to monitor
  doctor    Check the agent and fix problems
  status    Show the agent and its Core connection

DAY TO DAY
  logs      View agent and shipped logs
  service   install, start, stop, restart, uninstall
  config    show, diff, validate, edit
  update    Upgrade safely, with rollback

FLAGS
  --json        Machine-readable output
  --no-color    Plain output
  -h, --help    Show help
`

func TestScreenHelpMatchesArtboard(t *testing.T) {
	for _, th := range []*ui.Theme{ui.WithMode(false), ui.WithMode(true), ui.Plain()} {
		got := stripANSI(rootHelp(th, 80))
		for _, l := range strings.Split(got, "\n") {
			if len([]rune(l)) > 80 {
				t.Errorf("help line wider than 80 columns: %q", l)
			}
		}
	}
	screenGolden(t, "help-plain-80", rootHelp(ui.Plain(), 80))
	screenGolden(t, "help-light-80", rootHelp(ui.WithMode(false), 80))
	// the artboard's own lines are all present, in order
	got := rootHelp(ui.Plain(), 80)
	if !strings.HasPrefix(stripANSI(got), artboardHelpText) {
		t.Fatalf("help differs from the artboard.\n--- artboard\n%s\n--- got\n%s", artboardHelpText, got)
	}
	last := -1
	for _, line := range []string{"GET STARTED", "setup", "doctor", "status", "DAY TO DAY", "logs", "service", "config", "update", "FLAGS", "--json", "--no-color", "-h, --help"} {
		i := strings.Index(got, line)
		if i < last || i < 0 {
			t.Fatalf("%q missing or out of order in:\n%s", line, got)
		}
		last = i
	}
}

func TestHelpAndNoArgsExitZero(t *testing.T) {
	for _, args := range [][]string{{}, {"--help"}, {"-h"}} {
		var out, errOut bytes.Buffer
		if code := Execute(context.Background(), args, &out, &errOut); code != 0 || !strings.Contains(out.String(), "GET STARTED") {
			t.Fatalf("args=%v code=%d out=%q", args, code, out.String())
		}
	}
}

func TestScreenConfigErrorMatchesArtboard(t *testing.T) {
	d := &config.Diagnostic{File: "/etc/orion/config.yaml", Line: 14, Col: 3, Key: "intervall", Message: "unknown key", Suggest: "interval", Source: "  intervall: 30s"}
	const want = `
  ERROR  Invalid config

  /etc/orion/config.yaml:14:3
  14 │   intervall: 30s
     │   ^^^^^^^^^ unknown key

  Did you mean interval?

  Fix: orion-agent config edit
`
	for _, dark := range []bool{false, true} {
		got := stripANSI(renderErrorScreen(ui.WithMode(dark), d, 80))
		if got != want {
			t.Fatalf("error screen differs (dark=%v).\n--- artboard\n%s\n--- got\n%s", dark, want, got)
		}
	}
	const wantPlain = `
[fail] /etc/orion/config.yaml:14:3 unknown key "intervall"
       did you mean "interval"?
`
	if got := renderErrorScreen(ui.Plain(), d, 80); got != wantPlain {
		t.Fatalf("plain error differs.\n--- artboard\n%s\n--- got\n%s", wantPlain, got)
	}
	screenGolden(t, "error-config-light", renderErrorScreen(ui.WithMode(false), d, 80))
}

func TestRealConfigTypoProducesTheArtboardError(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.yaml")
	os.WriteFile(cfg, []byte("core_url: https://core.example.com\nintervall: 30s\n"), 0o600)
	var out, errOut bytes.Buffer
	code := Execute(context.Background(), []string{"--config", cfg, "--no-color", "config", "validate"}, &out, &errOut)
	got := errOut.String()
	if code != ExitFailed || !strings.Contains(got, ":2:1 unknown key \"intervall\"") || !strings.Contains(got, `did you mean "interval"?`) {
		t.Fatalf("code=%d stderr=%q", code, got)
	}
}

func TestExitCodes(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := Execute(context.Background(), []string{"nonsense"}, &out, &errOut); code != ExitUsage || !strings.Contains(errOut.String(), "orion-agent --help") {
		t.Fatalf("unknown command: code=%d stderr=%q", code, errOut.String())
	}
	errOut.Reset()
	if code := Execute(context.Background(), []string{"status", "--bogus"}, &out, &errOut); code != ExitUsage {
		t.Fatalf("unknown flag: code=%d", code)
	}
	if commandExitCode(NewCommandError("could not elevate privileges", os.ErrPermission)) != ExitElevate {
		t.Fatal("elevation failures must exit 3")
	}
}

func TestHelpSubcommandStillReachesCommandHelp(t *testing.T) {
	if isRootHelp([]string{"help", "doctor"}) || isRootHelp([]string{"doctor", "--help"}) {
		t.Fatal("help for a command must not be replaced by the root help")
	}
	if !isRootHelp([]string{"--no-color", "--help"}) || !isRootHelp([]string{"help"}) {
		t.Fatal("root help forms not recognised")
	}
	if isUsageError(errors.New("write: invalid argument")) {
		t.Fatal("a syscall error is not a usage error")
	}
}
