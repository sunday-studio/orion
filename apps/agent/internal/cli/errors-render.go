package cli

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"

	"orion/agent/internal/config"
	"orion/agent/internal/ui"
)

// Exit codes (cli-help-error artboard): 0 ok, 1 failed checks or errors,
// 2 usage, 3 needs elevation.
const (
	ExitFailed  = 1
	ExitUsage   = 2
	ExitElevate = 3
)

// diagnosticOf finds a config.Diagnostic directly or as a command's cause.
func diagnosticOf(err error) *config.Diagnostic {
	var d *config.Diagnostic
	if errors.As(err, &d) {
		return d
	}
	var ce *CommandError
	if errors.As(err, &ce) && ce.Cause != nil {
		return diagnosticOf(ce.Cause)
	}
	return nil
}

// docsConfigURL is where the config reference lives.
const docsConfigURL = "https://orion.thesunday.studio/docs/config"

// commandNames are the root commands, for "did you mean" on a typo.
var commandNames atomic.Pointer[[]string]

// renderErrorScreen renders an error like the cli-help-error board: a chip
// and a headline, the evidence indented under it, then the fix.
func renderErrorScreen(th *ui.Theme, err error, width int) string {
	if d := diagnosticOf(err); d != nil {
		return renderDiagnostic(th, d)
	}
	summary, detail, fixes := err.Error(), "", []string(nil)
	var ce *CommandError
	if errors.As(err, &ce) {
		summary, fixes = ce.Summary, ce.NextSteps
		if ce.Cause != nil {
			detail = ce.Cause.Error()
		}
	}
	extra := ""
	if name, ok := unknownCommandName(summary); ok {
		summary = fmt.Sprintf("Unknown command %q", name)
		detail = ""
		if near := closestWord(name, knownCommandNames()); near != "" {
			extra = "Did you mean " + th.Tone("up", near) + "?  "
		}
		extra += th.Muted("Run orion-agent --help for all commands.")
		fixes = nil
		if th.Plain() {
			extra = "Run orion-agent --help for all commands."
			if near := closestWord(name, knownCommandNames()); near != "" {
				extra = fmt.Sprintf("did you mean %q? ", near) + extra
			}
		}
	}
	var b strings.Builder
	b.WriteString("\n" + errorHeader(th, capitalize(summary)) + "\n")
	pad := "   "
	if th.Plain() {
		pad = "       "
	}
	if extra != "" {
		b.WriteString(pad + extra + "\n")
	}
	if detail != "" {
		for _, l := range ui.Wrap(detail, width-len(pad), 0) {
			b.WriteString(pad + th.Muted(l) + "\n")
		}
	}
	if len(fixes) > 0 {
		b.WriteString(pad + fixLineAlternatives(th, fixes) + "\n")
	}
	return b.String()
}

// unknownCommandName pulls the name out of cobra's "unknown command" message.
func unknownCommandName(msg string) (string, bool) {
	rest, ok := strings.CutPrefix(msg, "unknown command \"")
	if !ok {
		return "", false
	}
	name, _, ok := strings.Cut(rest, "\"")
	return name, ok
}

// fixLineAlternatives is "Fix   a   or   b": the first step bold, others dim.
func fixLineAlternatives(th *ui.Theme, fixes []string) string {
	if th.Plain() {
		return "fix: " + strings.Join(fixes, "  or  ")
	}
	parts := make([]string, len(fixes))
	for i, f := range fixes {
		cmd, note, _ := strings.Cut(strings.TrimPrefix(f, "run: "), "  ")
		parts[i] = th.Bold(cmd)
		if n := strings.TrimSpace(note); n != "" {
			parts[i] += "   " + th.Muted(n)
		}
	}
	return th.Muted("Fix") + "   " + strings.Join(parts, "   "+th.Muted("or")+"   ")
}

func errorHeader(th *ui.Theme, summary string) string {
	if th.Plain() {
		return "[fail] " + summary
	}
	return th.Chip("down", " ERROR ") + " " + th.Bold(summary)
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func renderDiagnostic(th *ui.Theme, d *config.Diagnostic) string {
	where := fmt.Sprintf("%s:%d", d.File, d.Line)
	if d.Col > 0 {
		where += ":" + strconv.Itoa(d.Col)
	}
	if th.Plain() {
		var b strings.Builder
		msg := d.Message
		if d.Key != "" {
			msg += fmt.Sprintf(" %q", d.Key)
		}
		b.WriteString("\n[fail] " + where + " " + msg + "\n")
		if d.Suggest != "" {
			b.WriteString(fmt.Sprintf("       did you mean %q?\n", d.Suggest))
		}
		if d.Hint != "" {
			b.WriteString("       " + d.Hint + "\n")
		}
		return b.String()
	}
	var b strings.Builder
	b.WriteString("\n" + errorHeader(th, "Invalid config") + "\n\n")
	b.WriteString("   " + th.Accent(where) + "\n")
	num := strconv.Itoa(d.Line)
	gutter := func(n string) string { return th.Muted(fmt.Sprintf("%*s │", len(num), n)) }
	if d.Source != "" {
		if d.Before != "" {
			b.WriteString("   " + gutter(strconv.Itoa(d.Line-1)) + " " + d.Before + "\n")
		}
		b.WriteString("   " + gutter(num) + " " + d.Source + "\n")
		if d.Col > 0 {
			carets := strings.Repeat("^", max(len([]rune(d.Key)), 1))
			b.WriteString("   " + gutter("") + " " + strings.Repeat(" ", d.Col-1) + th.Tone("down", carets+" "+d.Message) + "\n")
		} else {
			b.WriteString("   " + gutter("") + " " + th.Tone("down", d.Message) + "\n")
		}
	} else {
		b.WriteString("   " + d.Message + "\n")
	}
	if d.Suggest != "" {
		b.WriteString("\n   Did you mean " + th.Tone("up", d.Suggest) + "?\n")
	}
	fix := "orion-agent config edit"
	if d.Hint != "" {
		fix = strings.TrimPrefix(d.Hint, "fix: ")
	}
	b.WriteString("\n   " + th.Muted("Fix") + "   " + th.Bold(fix) + "\n")
	b.WriteString("   " + th.Muted("Docs") + "  " + docsConfigURL + "\n")
	return b.String()
}

func knownCommandNames() []string {
	if p := commandNames.Load(); p != nil {
		return *p
	}
	return nil
}
