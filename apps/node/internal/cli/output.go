package cli

import (
	"fmt"
	"io"
	"os"

	"orion/node/internal/ui"
)

var (
	outputWriter io.Writer = os.Stdout
	colorEnabled           = true
	theme                  = ui.Detect(os.Stdout)
)

func SetOutput(w io.Writer) {
	if w == nil {
		w = os.Stdout
	}
	outputWriter = w
	refreshTheme()
}

func SetColorEnabled(enabled bool) {
	colorEnabled = enabled
	refreshTheme()
}

// refreshTheme picks plain output for --no-color / NO_COLOR / non-terminals and
// the design-token theme otherwise.
func refreshTheme() {
	if !colorEnabled {
		theme = ui.Plain()
		return
	}
	theme = ui.Detect(outputWriter)
}

func writerSupportsColor(w io.Writer) bool {
	file, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func PrintHeader(title string) {
	fmt.Fprintln(outputWriter, theme.Header(title))
}

func PrintOrionBanner(subtitle string) {
	fmt.Fprintln(outputWriter, theme.Accent(theme.Bold("   ____       _             \n  / __ \\_____(_)___  ____   \n / / / / ___/ / __ \\/ __ \\  \n/ /_/ / /  / / /_/ / / / /  \n\\____/_/  /_/\\____/_/ /_/   ")))
	if subtitle != "" {
		fmt.Fprintln(outputWriter, theme.Muted(subtitle))
	}
}

func PrintInfo(label string, value interface{}) {
	fmt.Fprintf(outputWriter, "  %s %v\n", theme.Muted(label+":"), value)
}

func PrintCommandHint(command string, description string) {
	fmt.Fprintf(outputWriter, "  %s %s\n", theme.Accent(fmt.Sprintf("%-36s", command)), description)
}

func PrintPostUpdateGuide() {
	fmt.Fprintln(outputWriter, "\nNext commands")
	PrintCommandHint("orion-node status", "confirm the service and local state")
	PrintCommandHint("orion-node logs --lines 80", "review recent Node logs")
	PrintCommandHint("orion-node config show", "confirm the installed configuration")
}

func PrintStep(message string) {
	fmt.Fprintf(outputWriter, "  %s %s\n", theme.Accent("->"), message)
}

func PrintOK(message string) {
	fmt.Fprintf(outputWriter, "  %s %s\n", theme.Tone("up", "ok:"), message)
}

func PrintSkip(message string) {
	fmt.Fprintf(outputWriter, "  %s %s\n", theme.Tone("degraded", "skip:"), message)
}

func PrintError(message string) {
	fmt.Fprintf(outputWriter, "  %s %s\n", theme.Tone("down", "error:"), message)
}
