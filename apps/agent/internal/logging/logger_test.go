package logging

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigureFileWritesJSONLines(t *testing.T) {
	t.Cleanup(func() {
		_ = Close()
		ConfigureText(io.Discard)
		SetLevel(LevelInfo)
	})

	logPath := filepath.Join(t.TempDir(), "orion", "agent.log")
	if err := ConfigureFile(FileConfig{
		Path:       logPath,
		Level:      LevelDebug,
		MaxSizeMB:  25,
		MaxBackups: 5,
		MaxAgeDays: 14,
		Compress:   true,
	}); err != nil {
		t.Fatalf("ConfigureFile() error = %v", err)
	}

	Debugf("runtime started: monitor=%s", "api")
	if err := Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	var record map[string]any
	if err := json.Unmarshal(data, &record); err != nil {
		t.Fatalf("log line is not JSON: %v\n%s", err, data)
	}
	if record["level"] != "DEBUG" {
		t.Fatalf("level = %v, want DEBUG", record["level"])
	}
	if record["msg"] != "runtime started: monitor=api" {
		t.Fatalf("msg = %v, want formatted message", record["msg"])
	}
	if record["component"] != "agent" {
		t.Fatalf("component = %v, want agent", record["component"])
	}

	info, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if got := info.Mode().Perm(); got != 0o640 {
		t.Fatalf("log mode = %o, want 0640", got)
	}
}

func TestConfigureFileHonorsLevel(t *testing.T) {
	t.Cleanup(func() {
		_ = Close()
		ConfigureText(io.Discard)
		SetLevel(LevelInfo)
	})

	logPath := filepath.Join(t.TempDir(), "agent.log")
	if err := ConfigureFile(FileConfig{
		Path:      logPath,
		Level:     LevelWarn,
		MaxSizeMB: 25,
	}); err != nil {
		t.Fatalf("ConfigureFile() error = %v", err)
	}

	Infof("hidden")
	Warnf("visible")
	if err := Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if strings.Contains(string(data), "hidden") {
		t.Fatalf("log contains filtered info entry: %s", data)
	}
	if !strings.Contains(string(data), "visible") {
		t.Fatalf("log does not contain warning entry: %s", data)
	}
}

func TestParseLevel(t *testing.T) {
	tests := map[string]Level{
		"debug":   LevelDebug,
		"INFO":    LevelInfo,
		"warning": LevelWarn,
		"error":   LevelError,
		"":        LevelInfo,
	}

	for input, want := range tests {
		got, err := ParseLevel(input)
		if err != nil {
			t.Fatalf("ParseLevel(%q) error = %v", input, err)
		}
		if got != want {
			t.Fatalf("ParseLevel(%q) = %v, want %v", input, got, want)
		}
	}

	if _, err := ParseLevel("trace"); err == nil {
		t.Fatal("ParseLevel(trace) error = nil, want error")
	}
}
