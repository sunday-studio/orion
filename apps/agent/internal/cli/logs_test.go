package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestViewLogsFiltersAndFormatsJSONL(t *testing.T) {
	t.Parallel()

	path := writeTestLogFile(t, []string{
		`{"time":"2026-05-26T10:00:00Z","level":"INFO","component":"runtime","msg":"started"}`,
		`{"time":"2026-05-26T10:01:00Z","level":"ERROR","component":"registration","message":"register failed","monitor":"api","error":"401 unauthorized"}`,
		`{"time":"2026-05-26T10:02:00Z","level":"ERROR","component":"registration","message":"retry failed","monitor":"worker","error":"timeout"}`,
		`not-json`,
	})

	var out bytes.Buffer
	var errOut bytes.Buffer
	err := ViewLogs(LogViewOptions{
		File:      path,
		Source:    "file",
		Lines:     10,
		Level:     "error",
		Component: "registration",
		Monitor:   "api",
		Out:       &out,
		ErrOut:    &errOut,
	})
	if err != nil {
		t.Fatalf("ViewLogs() error = %v", err)
	}

	got := out.String()
	for _, want := range []string{
		"2026-05-26",
		"ERROR",
		"registration",
		"register failed",
		"monitor=api",
		`error="401 unauthorized"`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("ViewLogs() output = %q, want it to contain %q", got, want)
		}
	}
	if strings.Contains(got, "retry failed") {
		t.Fatalf("ViewLogs() output = %q, want monitor filter to exclude retry failed", got)
	}
	if !strings.Contains(errOut.String(), "skipped 1 malformed") {
		t.Fatalf("ViewLogs() stderr = %q, want malformed warning", errOut.String())
	}
}

func TestViewLogsJSONOutputPreservesRawRecords(t *testing.T) {
	t.Parallel()

	rawInfo := `{"time":"2026-05-26T10:00:00Z","level":"INFO","component":"runtime","msg":"started"}`
	rawError := `{"time":"2026-05-26T10:01:00Z","level":"ERROR","component":"registration","msg":"failed"}`
	path := writeTestLogFile(t, []string{rawInfo, rawError})

	var out bytes.Buffer
	err := ViewLogs(LogViewOptions{
		File:   path,
		Source: "file",
		Lines:  10,
		Level:  "error",
		JSON:   true,
		Out:    &out,
		ErrOut: &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("ViewLogs() error = %v", err)
	}

	got := strings.TrimSpace(out.String())
	if got != rawError {
		t.Fatalf("ViewLogs() JSON output = %q, want %q", got, rawError)
	}
}

func TestViewLogsLinesKeepsNewestMatches(t *testing.T) {
	t.Parallel()

	path := writeTestLogFile(t, []string{
		`{"time":"2026-05-26T10:00:00Z","level":"INFO","msg":"one"}`,
		`{"time":"2026-05-26T10:01:00Z","level":"INFO","msg":"two"}`,
		`{"time":"2026-05-26T10:02:00Z","level":"INFO","msg":"three"}`,
	})

	var out bytes.Buffer
	err := ViewLogs(LogViewOptions{
		File:   path,
		Source: "file",
		Lines:  2,
		Out:    &out,
		ErrOut: &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("ViewLogs() error = %v", err)
	}

	got := out.String()
	if strings.Contains(got, "one") || !strings.Contains(got, "two") || !strings.Contains(got, "three") {
		t.Fatalf("ViewLogs() output = %q, want newest two entries only", got)
	}
}

func TestViewLogsSinceFiltersByTimestamp(t *testing.T) {
	t.Parallel()

	path := writeTestLogFile(t, []string{
		`{"time":"2026-05-26T09:59:59Z","level":"INFO","msg":"old"}`,
		`{"time":"2026-05-26T10:00:00Z","level":"INFO","msg":"new"}`,
	})

	var out bytes.Buffer
	err := ViewLogs(LogViewOptions{
		File:   path,
		Source: "file",
		Lines:  10,
		Since:  "2026-05-26T10:00:00Z",
		Out:    &out,
		ErrOut: &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("ViewLogs() error = %v", err)
	}

	got := out.String()
	if strings.Contains(got, "old") || !strings.Contains(got, "new") {
		t.Fatalf("ViewLogs() output = %q, want entries at or after since", got)
	}
}

func TestViewLogsParsesRelativeSince(t *testing.T) {
	t.Parallel()

	path := writeTestLogFile(t, []string{
		`{"time":"2026-05-26T09:59:59Z","level":"INFO","msg":"old"}`,
		`{"time":"2026-05-26T10:30:00Z","level":"INFO","msg":"new"}`,
	})

	var out bytes.Buffer
	err := ViewLogs(LogViewOptions{
		File:   path,
		Source: "file",
		Lines:  10,
		Since:  "1h",
		Now: func() time.Time {
			return time.Date(2026, 5, 26, 11, 0, 0, 0, time.UTC)
		},
		Out:    &out,
		ErrOut: &bytes.Buffer{},
	})
	if err != nil {
		t.Fatalf("ViewLogs() error = %v", err)
	}

	got := out.String()
	if strings.Contains(got, "old") || !strings.Contains(got, "new") {
		t.Fatalf("ViewLogs() output = %q, want entries within the last hour", got)
	}
}

func TestViewLogsUsesFallbackWhenAutoSourceIsMissing(t *testing.T) {
	t.Parallel()

	called := false
	var errOut bytes.Buffer
	err := ViewLogs(LogViewOptions{
		File:   filepath.Join(t.TempDir(), "missing.log"),
		Source: "auto",
		Lines:  23,
		Out:    &bytes.Buffer{},
		ErrOut: &errOut,
		Fallback: func(lines int) {
			called = true
			if lines != 23 {
				t.Fatalf("fallback lines = %d, want 23", lines)
			}
		},
	})
	if err != nil {
		t.Fatalf("ViewLogs() error = %v", err)
	}
	if !called {
		t.Fatal("ViewLogs() did not call fallback")
	}
	if !strings.Contains(errOut.String(), "Orion log file not found") {
		t.Fatalf("ViewLogs() stderr = %q, want missing log warning", errOut.String())
	}
}

func TestViewLogsJSONMissingFileReturnsJSONLError(t *testing.T) {
	t.Parallel()

	err := ViewLogs(LogViewOptions{
		File:   filepath.Join(t.TempDir(), "missing.log"),
		Source: "auto",
		JSON:   true,
		Out:    &bytes.Buffer{},
		ErrOut: &bytes.Buffer{},
	})
	if err == nil {
		t.Fatal("ViewLogs() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "--json requires Orion JSONL logs") {
		t.Fatalf("ViewLogs() error = %q, want JSONL message", err.Error())
	}
}

func TestParseLogViewOptionsRejectsInvalidInputs(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{
		{"--level", "trace"},
		{"--since", "not-a-time"},
		{"--source", "weird"},
		{"--lines", "-1"},
	} {
		options, err := ParseLogViewOptions(args, 80)
		if err == nil {
			err = ViewLogs(LogViewOptions{
				File:   options.File,
				Source: options.Source,
				Lines:  options.Lines,
				Since:  options.Since,
				Level:  options.Level,
				Out:    &bytes.Buffer{},
				ErrOut: &bytes.Buffer{},
			})
		}
		if err == nil {
			t.Fatalf("Parse/View logs with args %v returned nil error", args)
		}
	}
}

func TestParseLogViewOptionsHelp(t *testing.T) {
	t.Parallel()

	_, err := ParseLogViewOptions([]string{"--help"}, 80)
	if !errors.Is(err, errLogsHelp) {
		t.Fatalf("ParseLogViewOptions(--help) error = %v, want errLogsHelp", err)
	}
}

func writeTestLogFile(t *testing.T, lines []string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "agent.log")
	data := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatalf("write test log file: %v", err)
	}
	return path
}
