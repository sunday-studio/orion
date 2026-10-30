package control

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureSocketDirLeavesCustomPathsAlone(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "custom", "run")
	if err := EnsureSocketDir(filepath.Join(dir, "node.sock")); err != nil {
		t.Fatalf("EnsureSocketDir on a custom path = %v, want nil", err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("a custom socket folder must not be created or chowned, stat err = %v", err)
	}
}

func TestEnsureSocketDirDoesNothingWithoutRoot(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root")
	}
	if err := EnsureSocketDir(darwinSocketPath); err != nil {
		t.Fatalf("EnsureSocketDir as a normal user = %v, want nil", err)
	}
}
