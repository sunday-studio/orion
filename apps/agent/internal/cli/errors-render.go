package cli

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

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

// renderErrorScreen renders an error like the cli-help-error artboard.
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
	var b strings.Builder
	b.WriteString("\n" + errorHeader(th, capitalize(summary)) + "\n")
	pad := "  "
	if th.Plain() {
		pad = "       "
	}
	if detail != "" {
		b.WriteString("\n")
		for _, l := range ui.Wrap(detail, width-len(pad), 0) {
			b.WriteString(pad + l + "\n")
		}
	}
	if len(fixes) > 0 {
		b.WriteString("\n")
		for _, f := range fixes {
			b.WriteString(pad + th.Muted("Fix:") + " " + strings.TrimPrefix(f, "run: ") + "\n")
		}
	}
	return b.String()
}

func errorHeader(th *ui.Theme, summary string) string {
	if th.Plain() {
		return "[fail] " + summary
	}
	return " " + th.Chip("down", " ERROR ") + " " + th.Bold(summary)
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
	b.WriteString("  " + th.Accent(where) + "\n")
	num := strconv.Itoa(d.Line)
	if d.Source != "" {
		b.WriteString("  " + th.Muted(num+" │") + " " + d.Source + "\n")
		if d.Col > 0 {
			carets := strings.Repeat("^", max(len([]rune(d.Key)), 1))
			b.WriteString("  " + th.Muted(strings.Repeat(" ", len(num))+" │") + " " + strings.Repeat(" ", d.Col-1) + th.Tone("down", carets+" "+d.Message) + "\n")
		} else {
			b.WriteString("  " + th.Muted(strings.Repeat(" ", len(num))+" │") + " " + th.Tone("down", d.Message) + "\n")
		}
	} else {
		b.WriteString("  " + d.Message + "\n")
	}
	if d.Suggest != "" {
		b.WriteString("\n  Did you mean " + th.Tone("up", d.Suggest) + "?\n")
	}
	b.WriteString("\n  " + th.Muted("Fix:") + " ")
	if d.Hint != "" {
		b.WriteString(strings.TrimPrefix(d.Hint, "fix: ") + "\n")
	} else {
		b.WriteString("orion-agent config edit\n")
	}
	return b.String()
}
