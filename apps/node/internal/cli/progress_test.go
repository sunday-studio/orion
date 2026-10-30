package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunWithProgressRunsAndLabelsOffTerminal(t *testing.T) {
	var buf bytes.Buffer
	old := outputWriter
	outputWriter = &buf
	t.Cleanup(func() { outputWriter = old })

	ran := false
	runWithProgress("Restarting the service", func() { ran = true })

	if !ran {
		t.Fatal("the wrapped work did not run")
	}
	if got := buf.String(); !strings.Contains(got, "Restarting the service...") || strings.Contains(got, "\r") {
		t.Fatalf("want one plain label line without carriage returns, got %q", got)
	}
}
