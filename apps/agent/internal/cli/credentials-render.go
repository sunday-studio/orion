package cli

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/x/term"

	"orion/agent/internal/ui"
)

// credStep is one finished step: a check mark, a name, and an optional detail.
func credStep(th *ui.Theme, name, detail string) string {
	line := "   " + th.Pass() + " " + name
	if detail != "" {
		line += " " + th.Muted(detail)
	}
	return line
}

// credNote is a step that did not need to run, such as a service that was not running.
func credNote(th *ui.Theme, name, detail string) string {
	return "   " + th.SkipMark() + " " + name + " " + th.Muted(detail)
}

// nextCommand is the closing "what to run now" line.
func nextCommand(th *ui.Theme, cmd string) string {
	if th.Plain() {
		return "next: " + cmd
	}
	return "   " + th.Accent("▸") + " " + th.Bold(cmd)
}

// promptBlock is the three-line input block: label, masked value, hint.
func promptBlock(th *ui.Theme, label string, masked int, hint string) string {
	bar := th.Accent("┃")
	if th.Plain() {
		return label + ": " + strings.Repeat("*", masked)
	}
	return " " + bar + " " + th.Bold(label) + "\n" +
		" " + bar + " " + th.Accent("›") + " " + strings.Repeat("•", masked) + "\n" +
		" " + bar + " " + th.Muted(hint)
}

// credBlock joins lines with the blank lines the board leaves between groups.
func credBlock(groups ...[]string) string {
	var parts []string
	for _, g := range groups {
		parts = append(parts, strings.Join(g, "\n"))
	}
	return "\n" + strings.Join(parts, "\n\n") + "\n"
}

// readMasked reads a secret from a terminal, echoing a dot per character, and
// redraws the prompt block as it grows. It returns an error when stdin is not
// a terminal, so scripts must pass --token-file instead.
func readMasked(th *ui.Theme, in *os.File, out io.Writer, label, hint string) (string, error) {
	if !term.IsTerminal(in.Fd()) {
		return "", fmt.Errorf("stdin is not a terminal")
	}
	state, err := term.MakeRaw(in.Fd())
	if err != nil {
		return "", err
	}
	defer term.Restore(in.Fd(), state)
	var value []rune
	draw := func(first bool) {
		if !first {
			fmt.Fprint(out, "\x1b[3A\r")
		}
		for _, l := range strings.Split(promptBlock(th, label, len(value), hint), "\n") {
			fmt.Fprint(out, "\x1b[2K"+l+"\r\n")
		}
	}
	fmt.Fprint(out, "\r\n")
	draw(true)
	buf := make([]byte, 4)
	for {
		n, err := in.Read(buf)
		if err != nil {
			return "", err
		}
		for _, r := range []rune(string(buf[:n])) {
			switch r {
			case '\r', '\n':
				fmt.Fprint(out, "\r\n")
				return strings.TrimSpace(string(value)), nil
			case 3, 4, 27: // ctrl+c, ctrl+d, esc
				fmt.Fprint(out, "\r\n")
				return "", fmt.Errorf("cancelled")
			case 127, 8:
				if len(value) > 0 {
					value = value[:len(value)-1]
				}
			case 21: // ctrl+u
				value = nil
			default:
				if r >= 32 {
					value = append(value, r)
				}
			}
		}
		draw(false)
	}
}
