package cli

import (
	"fmt"
	"io"
	"os"
)

const (
	colorReset  = "\033[0m"
	colorBold   = "\033[1m"
	colorDim    = "\033[2m"
	colorCyan   = "\033[36m"
	colorGreen  = "\033[32m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
)

var (
	outputWriter io.Writer = os.Stdout
	colorEnabled           = true
)

func SetOutput(w io.Writer) {
	if w == nil {
		outputWriter = os.Stdout
		return
	}
	outputWriter = w
}

func SetColorEnabled(enabled bool) {
	colorEnabled = enabled
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

func color(code string) string {
	if !colorEnabled {
		return ""
	}
	return code
}

func PrintHeader(title string) {
	fmt.Fprintf(outputWriter, "%sOrion Agent:%s %s%s%s\n", color(colorBold), color(colorReset), color(colorCyan), title, color(colorReset))
}

func PrintInfo(label string, value interface{}) {
	fmt.Fprintf(outputWriter, "  %s%s:%s %v\n", color(colorDim), label, color(colorReset), value)
}

func PrintStep(message string) {
	fmt.Fprintf(outputWriter, "  %s->%s %s\n", color(colorCyan), color(colorReset), message)
}

func PrintOK(message string) {
	fmt.Fprintf(outputWriter, "  %sok:%s %s\n", color(colorGreen), color(colorReset), message)
}

func PrintSkip(message string) {
	fmt.Fprintf(outputWriter, "  %sskip:%s %s\n", color(colorYellow), color(colorReset), message)
}

func PrintError(message string) {
	fmt.Fprintf(outputWriter, "  %serror:%s %s\n", color(colorRed), color(colorReset), message)
}
