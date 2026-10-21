package cli

import (
	"os"
	"strings"
	"testing"

	"orion/node/internal/ui"
)

func TestOperateScreensMatchBoard(t *testing.T) {
	pre := []opStep{{Mark: "ok", Name: "Config valid"}, {Mark: "ok", Name: "State database"}, {Mark: "ok", Name: "Permissions"}}
	svc := []opStep{{Mark: "ok", Name: "Failure state reset"}, {Mark: "ok", Name: "Started", Detail: "systemd · pid 2114", TookMS: 1200}}
	board := boardBlock(t, "cli-service.txt", "start")
	_, board, _ = strings.Cut(board, "\n")
	for _, dark := range []bool{false, true} {
		got := stripANSI(startScreen(ui.WithMode(dark), "Start", pre, svc, "running   first report due in 30 s", "orion-node status"))
		sameAsBoard(t, "start", board, got)
	}

	fail := []opStep{{Mark: "ok", Name: "Config valid"},
		{Mark: "fail", Name: "State database is not writable", Evidence: []string{"/var/lib/orion/state.db is owned by root"}, Fix: "sudo chown -R orion /var/lib/orion"},
		{Mark: "ok", Name: "Permissions"}}
	got := stripANSI(preflightFailScreen(ui.WithMode(false), fail, []string{"14:21:08 error state: open /var/lib/orion/state.db: permission denied", "14:21:08 error node: exiting"}))
	board = boardBlock(t, "cli-service.txt", "start  ·  preflight fails")
	_, board, _ = strings.Cut(board, "\n")
	board, _, _ = strings.Cut(board, "\n ERROR  Start preflight failed")
	if norm(got) != norm(board) {
		t.Errorf("preflight failure differs.\n--- board\n%s\n--- got\n%s", norm(board), norm(got))
	}
}

func TestOperateConfirmAndInstallHints(t *testing.T) {
	th := ui.WithMode(false)
	got := stripANSI(confirmView(th, "Stop the node on edge-05?", "Monitoring and log shipping pause until you start it again.", "Stop", "Cancel", true, "skip the prompt with --yes"))
	for _, want := range []string{"┃ Stop the node on edge-05?", "┃ Monitoring and log shipping pause until you start it again.", "Stop", "Cancel", "skip the prompt with --yes"} {
		if !strings.Contains(got, want) {
			t.Errorf("confirm lacks %q:\n%s", want, got)
		}
	}
	in := stripANSI(installHint(th, false))
	if !strings.Contains(in, "The installer sets everything up in one step:") || !strings.Contains(in, "--core-url <url> --enrollment-token <token>") || !strings.Contains(in, "add --user") {
		t.Errorf("install hint:\n%s", in)
	}
	un := stripANSI(installHint(th, true))
	if !strings.Contains(un, "| bash -s -- uninstall") || !strings.Contains(un, "Add --purge to remove config and state too.") {
		t.Errorf("uninstall hint:\n%s", un)
	}
	if p := installHint(ui.Plain(), false); strings.ContainsAny(p, "▸") || strings.Contains(p, "\x1b") {
		t.Errorf("plain hint has decoration: %q", p)
	}
}

func TestRunStopRestartAndCredentialScreensMatchBoard(t *testing.T) {
	th := ui.WithMode(false)
	run := boardBlock(t, "cli-service.txt", "run")
	parts := strings.Split(run, "\n$ orion-node run\n")
	once := strings.TrimPrefix(parts[0], "$ orion-node run --once\n")
	gotOnce := stripANSI(runHeader(th, "foreground, one cycle") + opLines(th, []opStep{
		{Mark: "ok", Name: "Config loaded", Detail: "5 monitors"}, {Mark: "ok", Name: "State database ready"}, {Mark: "ok", Name: "Registered"},
		{Mark: "ok", Name: "One collection cycle complete", Took: "1.4 s"}}))
	if norm(gotOnce) != norm(once) {
		t.Errorf("run --once differs.\n--- board\n%s\n--- got\n%s", norm(once), norm(gotOnce))
	}
	errBoard := parts[1]
	gotErr := stripANSI(renderErrorScreen(th, credentialsRejected(), 80))
	if norm(gotErr) != norm(errBoard) {
		t.Errorf("credentials error differs.\n--- board\n%s\n--- got\n%s", norm(errBoard), norm(gotErr))
	}

	sr := boardBlock(t, "cli-service.txt", "stop and restart")
	restart := sr[strings.Index(sr, "$ orion-node restart\n")+len("$ orion-node restart\n"):]
	gotRestart := stripANSI(opLines(th, []opStep{{Mark: "ok", Name: "Stopped"}, {Mark: "ok", Name: "Started"}, {Mark: "ok", Name: "Healthy", Detail: "reporting again", TookMS: 6000}}))
	if norm(gotRestart) != norm(restart) {
		t.Errorf("restart differs.\n--- board\n%s\n--- got\n%s", norm(restart), norm(gotRestart))
	}
}

func TestPreflightStepsExplainAFailureWithTheOwnerAndTheFix(t *testing.T) {
	dir := t.TempDir()
	r := PreflightReport{Checks: []DiagnosticCheck{
		{Name: "service", Status: CheckOK}, {Name: "config", Status: CheckOK},
		{Name: "state", Status: CheckError, Path: dir + "/state.db", Error: os.ErrPermission},
		{Name: "logs", Status: CheckOK},
	}}
	steps := preflightSteps(r)
	if steps[1].Mark != "fail" || steps[1].Name != "State database is not writable" || !strings.Contains(steps[1].Fix, "sudo chown -R orion "+dir) {
		t.Fatalf("steps = %+v", steps)
	}
	if steps[0].Mark != "ok" || steps[2].Mark != "ok" {
		t.Fatalf("only the state check failed: %+v", steps)
	}
}
