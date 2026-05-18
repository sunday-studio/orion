package cli

import "fmt"

func PrintHeader(title string) {
	fmt.Printf("Orion Agent: %s\n", title)
}

func PrintInfo(label string, value interface{}) {
	fmt.Printf("  %s: %v\n", label, value)
}

func PrintStep(message string) {
	fmt.Printf("  -> %s\n", message)
}

func PrintOK(message string) {
	fmt.Printf("  ok: %s\n", message)
}

func PrintSkip(message string) {
	fmt.Printf("  skip: %s\n", message)
}

func PrintError(message string) {
	fmt.Printf("  error: %s\n", message)
}
