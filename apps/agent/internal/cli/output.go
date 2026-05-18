package cli

import "fmt"

const (
	colorReset  = "\033[0m"
	colorBold   = "\033[1m"
	colorDim    = "\033[2m"
	colorCyan   = "\033[36m"
	colorGreen  = "\033[32m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
)

func PrintHeader(title string) {
	fmt.Printf("%sOrion Agent:%s %s%s%s\n", colorBold, colorReset, colorCyan, title, colorReset)
}

func PrintInfo(label string, value interface{}) {
	fmt.Printf("  %s%s:%s %v\n", colorDim, label, colorReset, value)
}

func PrintStep(message string) {
	fmt.Printf("  %s->%s %s\n", colorCyan, colorReset, message)
}

func PrintOK(message string) {
	fmt.Printf("  %sok:%s %s\n", colorGreen, colorReset, message)
}

func PrintSkip(message string) {
	fmt.Printf("  %sskip:%s %s\n", colorYellow, colorReset, message)
}

func PrintError(message string) {
	fmt.Printf("  %serror:%s %s\n", colorRed, colorReset, message)
}
