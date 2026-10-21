package state

import (
	"runtime"
	"testing"
)

func TestDefaultPathUsesPlatformInstallPath(t *testing.T) {
	t.Parallel()

	got := DefaultPath()
	switch runtime.GOOS {
	case "linux":
		if got != "/var/lib/orion/state.db" {
			t.Fatalf("DefaultPath() = %q, want Linux install path", got)
		}
	case "darwin":
		if got != "/usr/local/var/lib/orion/state.db" {
			t.Fatalf("DefaultPath() = %q, want macOS install path", got)
		}
	default:
		if got != "state.db" {
			t.Fatalf("DefaultPath() = %q, want local fallback", got)
		}
	}
}
