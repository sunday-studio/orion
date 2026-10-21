package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"orion/node/internal/config"
	"orion/node/internal/ui"
)

func TestScreenHelpMatchesBoard(t *testing.T) {
	board := boardBlock(t, "cli-help-error.txt", "--help")
	_, board, _ = strings.Cut(board, "\n")
	// The board's "More" line lists seven commands; this help lists the rest
	// too so that no command is hidden.
	moreLine := " More: enroll · discover · usage · maintenance · token · version · run"
	want, _, _ := strings.Cut(board, moreLine)
	for _, th := range []*ui.Theme{ui.WithMode(false), ui.WithMode(true)} {
		got := stripANSI(rootHelp(th, 80))
		for _, l := range strings.Split(got, "\n") {
			if len([]rune(l)) > 80 {
				t.Errorf("help line wider than 80 columns: %q", l)
			}
		}
		head, tail, ok := strings.Cut(got, moreLine)
		if !ok || strings.TrimSpace(head) != strings.TrimSpace(want) {
			t.Fatalf("help differs from the board.\n--- board\n%s\n--- got\n%s", want, got)
		}
		for _, c := range []string{"start", "stop", "restart", "reconfigure", "state", "emit", "run-job"} {
			if !strings.Contains(tail, c) {
				t.Errorf("command %q is hidden from the help", c)
			}
		}
	}
	plain := rootHelp(ui.Plain(), 80)
	if strings.ContainsAny(plain, "◆") || strings.Contains(plain, "\x1b") {
		t.Fatalf("plain help has decoration:\n%s", plain)
	}
	screenGolden(t, "help-plain-80", plain)
	screenGolden(t, "help-light-80", rootHelp(ui.WithMode(false), 80))
}

func TestHelpAndNoArgsExitZero(t *testing.T) {
	for _, args := range [][]string{{}, {"--help"}, {"-h"}} {
		var out, errOut bytes.Buffer
		if code := Execute(context.Background(), args, &out, &errOut); code != 0 || !strings.Contains(out.String(), "START HERE") {
			t.Fatalf("args=%v code=%d out=%q", args, code, out.String())
		}
	}
}

func TestScreenErrorsMatchBoard(t *testing.T) {
	board := boardBlock(t, "cli-help-error.txt", "errors")
	parts := strings.Split(board, " ──────────────────────────────────────────────────────────────────────────────────────\n")
	body := func(chunk string) string { _, rest, _ := strings.Cut(chunk, "\n"); return rest } // drop "$ command"
	names := []string{"setup", "doctor", "status", "logs", "config", "update", "service"}
	commandNames.Store(&names)
	d := &config.Diagnostic{File: "/etc/orion/config.yaml", Line: 14, Col: 3, Key: "intervall", Message: "unknown key", Suggest: "interval",
		Before: "  interval: 30s", Source: "  intervall: 30s"}
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"config", d, body(parts[0])},
		{"unknown command", errors.New(`unknown command "statsu" for "orion-node"`), body(strings.TrimPrefix(parts[1], "\n"))},
		{"state not writable", NewCommandError("Cannot start: state database is not writable", errors.New("/var/lib/orion/state.db is owned by root"), "sudo orion-node start", "orion-node doctor"), body(strings.TrimPrefix(parts[2], "\n"))},
	}
	for _, c := range cases {
		for _, dark := range []bool{false, true} {
			err := c.err
			if c.name == "unknown command" {
				err = &CommandError{Summary: err.Error(), ExitCode: ExitUsage}
			}
			got := stripANSI(renderErrorScreen(ui.WithMode(dark), err, 80))
			if norm(got) != norm(c.want) {
				t.Errorf("%s differs from the board (dark=%v).\n--- board\n%s\n--- got\n%s", c.name, dark, norm(c.want), norm(got))
			}
		}
	}
	const wantPlain = `
[fail] /etc/orion/config.yaml:14:3 unknown key "intervall"
       did you mean "interval"?
`
	if got := renderErrorScreen(ui.Plain(), d, 80); got != wantPlain {
		t.Fatalf("plain error differs.\n--- board\n%s\n--- got\n%s", wantPlain, got)
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
	if code := Execute(context.Background(), []string{"nonsense"}, &out, &errOut); code != ExitUsage || !strings.Contains(errOut.String(), "orion-node --help") {
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
