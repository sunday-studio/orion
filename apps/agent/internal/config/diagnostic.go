package config

import (
	"fmt"
	"os"
	"strings"
)

// Diagnostic is a config problem with its position, so the CLI can show the
// file, line and column, the offending source line with a caret, and a fix.
type Diagnostic struct {
	File    string
	Line    int
	Col     int    // 1-based; 0 when the whole line is at fault
	Key     string // the offending key, for the caret width
	Message string // "unknown key" or the validation text
	Suggest string // closest valid key, if any
	Source  string // the text of the line, for the excerpt
	Before  string // the line above it, for context
	Hint    string // "fix: add `timeout: 5s`"
}

func (d *Diagnostic) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s:%d", d.File, d.Line)
	if d.Col > 0 {
		fmt.Fprintf(&b, ":%d", d.Col)
	}
	b.WriteString(": " + d.Message)
	if d.Key != "" {
		fmt.Fprintf(&b, " %q", d.Key)
	}
	if d.Suggest != "" {
		fmt.Fprintf(&b, " (did you mean %q?)", d.Suggest)
	}
	if d.Hint != "" {
		b.WriteString("\n  " + d.Hint)
	}
	return b.String()
}

// sourceLine reads one 1-based line of a file ("" if unreadable).
func sourceLine(file string, line int) string {
	data, err := os.ReadFile(file)
	if err != nil || line < 1 {
		return ""
	}
	lines := strings.Split(string(data), "\n")
	if line > len(lines) {
		return ""
	}
	return strings.TrimRight(lines[line-1], "\r")
}

// keyColumn finds the 1-based column of "key:" in a line.
func keyColumn(line, key string) int {
	if i := strings.Index(line, key+":"); i >= 0 {
		return i + 1
	}
	return 0
}

// lineBefore is the text of the line above, or "" at the top of the file.
func lineBefore(file string, line int) string {
	if line < 2 {
		return ""
	}
	return sourceLine(file, line-1)
}
