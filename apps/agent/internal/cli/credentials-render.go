package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

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

// maskedInput is the editing state of a secret prompt, kept apart from the
// terminal so it can be tested.
type maskedInput struct {
	value   []rune
	pending []byte // an incomplete UTF-8 sequence from the previous read
}

// feed applies one read from the terminal. A lone escape cancels; an escape
// sequence (arrow keys and the like) is ignored.
func (m *maskedInput) feed(b []byte) (done, cancelled bool) {
	if len(b) > 1 && b[0] == 27 {
		return false, false
	}
	data := append(m.pending, b...)
	m.pending = nil
	for len(data) > 0 {
		r, size := utf8.DecodeRune(data)
		if r == utf8.RuneError && size <= 1 && !utf8.FullRune(data) {
			m.pending = append([]byte(nil), data...)
			break
		}
		data = data[size:]
		switch r {
		case '\r', '\n':
			return true, false
		case 3, 4, 27: // ctrl+c, ctrl+d, esc
			return false, true
		case 127, 8:
			if len(m.value) > 0 {
				m.value = m.value[:len(m.value)-1]
			}
		case 21: // ctrl+u
			m.value = nil
		default:
			if r >= 32 {
				m.value = append(m.value, r)
			}
		}
	}
	return false, false
}

// errCancelled is returned by readMasked when the person backs out.
var errCancelled = fmt.Errorf("cancelled")

// canPromptSecret: only a person at a terminal is asked for a secret, and
// never when output is machine-readable or quiet.
func canPromptSecret(opts *Options) bool {
	return !opts.JSON && !opts.Quiet && stdinIsTerminal()
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
	var m maskedInput
	lines := len(strings.Split(promptBlock(th, label, 0, hint), "\n"))
	draw := func(first bool) {
		if !first {
			fmt.Fprintf(out, "\x1b[%dA\r", lines)
		}
		for _, l := range strings.Split(promptBlock(th, label, len(m.value), hint), "\n") {
			fmt.Fprint(out, "\x1b[2K"+l+"\r\n")
		}
	}
	fmt.Fprint(out, "\r\n")
	draw(true)
	buf := make([]byte, 256)
	for {
		n, err := in.Read(buf)
		if err != nil {
			return "", err
		}
		done, cancelled := m.feed(buf[:n])
		switch {
		case cancelled:
			fmt.Fprint(out, "\r\n")
			return "", errCancelled
		case done:
			fmt.Fprint(out, "\r\n")
			return strings.TrimSpace(string(m.value)), nil
		}
		draw(false)
	}
}
