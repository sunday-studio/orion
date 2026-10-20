package cli

import (
	"strings"
	"testing"

	"orion/agent/internal/ui"
)

func TestCredentialScreensMatchBoard(t *testing.T) {
	th := ui.WithMode(false)
	enroll := credBlock(
		[]string{promptBlock(th, "One-time token", 24, "From Servers › Add server. Works once.")},
		[]string{credStep(th, "Reachable", ""), credStep(th, "Token accepted", ""), credStep(th, th.Bold("Enrolled as edge-05"), "")},
		[]string{nextCommand(th, "orion-agent service start")})
	apply := credBlock(
		[]string{promptBlock(th, "Replacement token", 40, "64 characters, from the Console. Your identity and monitors are kept.")},
		[]string{credStep(th, "Token saved", ""), credStep(th, "Identity preserved", "agt_3f9c1a · 5 monitors")},
		[]string{nextCommand(th, "orion-agent service restart")})
	reconf := credBlock(
		[]string{credStep(th, "Service stopped", ""), credStep(th, "Registration reset", ""), credStep(th, "Registered again", "5 monitors"), credStep(th, "Service started", "")})
	state := credBlock([]string{credStep(th, "State database ready", "/var/lib/orion/state.db")})
	for _, c := range []struct{ title, board, got string }{
		{"enroll", boardBlock(t, "cli-credentials.txt", "enroll"), enroll},
		{"token apply", boardBlock(t, "cli-credentials.txt", "token apply"), apply},
	} {
		_, board, _ := strings.Cut(c.board, "\n") // the "$ command" line is the shell's
		if norm(stripANSI(c.got)) != norm(board) {
			t.Errorf("%s differs.\n--- board\n%s\n--- got\n%s", c.title, norm(board), norm(stripANSI(c.got)))
		}
	}
	both := boardBlock(t, "cli-credentials.txt", "reconfigure and state init")
	if !strings.Contains(norm(both), strings.TrimSpace(norm(stripANSI(reconf)))) || !strings.Contains(norm(both), strings.TrimSpace(norm(stripANSI(state)))) {
		t.Errorf("reconfigure/state init differ.\n--- board\n%s\n--- got\n%s%s", norm(both), norm(stripANSI(reconf)), norm(stripANSI(state)))
	}
}

func TestCredentialPlainHasNoDecoration(t *testing.T) {
	th := ui.Plain()
	s := promptBlock(th, "One-time token", 5, "hint") + credStep(th, "Token saved", "") + nextCommand(th, "orion-agent service start")
	if strings.ContainsAny(s, "┃•▸✓") || ansi.MatchString(s) {
		t.Errorf("plain output has decoration: %q", s)
	}
}
