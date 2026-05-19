package config

import (
	"runtime"
	"testing"
)

func TestDefaultPathUsesPlatformInstallPath(t *testing.T) {
	t.Parallel()

	got := DefaultPath()
	switch runtime.GOOS {
	case "linux":
		if got != "/etc/orion/config.yaml" {
			t.Fatalf("DefaultPath() = %q, want Linux install path", got)
		}
	case "darwin":
		if got != "/usr/local/etc/orion/config.yaml" {
			t.Fatalf("DefaultPath() = %q, want macOS install path", got)
		}
	default:
		if got != "config.yaml" {
			t.Fatalf("DefaultPath() = %q, want local fallback", got)
		}
	}
}
