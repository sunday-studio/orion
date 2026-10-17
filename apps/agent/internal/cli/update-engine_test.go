package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type updateFixture struct {
	dir, exe, base string
	env            *updateEnv
	running        bool
	starts, stops  int
	healthErr      error
}

func scriptBin(version string) string { return "#!/bin/sh\necho 'orion-agent " + version + "'\n" }

func newFixture(t *testing.T, newBody string) *updateFixture {
	t.Helper()
	dir := t.TempDir()
	f := &updateFixture{dir: dir, exe: filepath.Join(dir, "orion-agent"), base: filepath.Join(dir, "release"), running: true}
	os.MkdirAll(f.base, 0o755)
	os.WriteFile(f.exe, []byte(scriptBin("1.0.0")), 0o755)
	asset, _ := releaseAssetName()
	os.WriteFile(filepath.Join(f.base, asset), []byte(newBody), 0o755)
	sum := sha256.Sum256([]byte(newBody))
	os.WriteFile(filepath.Join(f.base, "SHA256SUMS"), []byte(hex.EncodeToString(sum[:])+"  "+asset+"\n"), 0o644)
	f.env = &updateEnv{
		executable: f.exe, fetch: downloadFile, version: binaryVersion,
		healthWait: 20 * time.Millisecond, healthPoll: 5 * time.Millisecond,
		service: serviceOps{
			running: func() (bool, error) { return f.running, nil },
			stop:    func() error { f.stops++; f.running = false; return nil },
			start:   func() error { f.starts++; f.running = true; return nil },
		},
	}
	f.env.healthy = func(string) error { return f.healthErr }
	return f
}

func (f *updateFixture) opts() UpdateOptions {
	return UpdateOptions{CurrentVersion: "1.0.0", BaseURL: "file://" + f.base}
}

func (f *updateFixture) exeBody(t *testing.T) string {
	b, err := os.ReadFile(f.exe)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestUpdateSuccessKeepsPrevious(t *testing.T) {
	f := newFixture(t, scriptBin("2.0.0"))
	if err := runUpdate2(f.env, f.opts()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.exeBody(t), "2.0.0") || !f.running {
		t.Fatalf("not updated; body=%q running=%v", f.exeBody(t), f.running)
	}
	prev, _ := os.ReadFile(prevPath(f.exe))
	if !strings.Contains(string(prev), "1.0.0") {
		t.Fatal("previous binary not kept")
	}
	if err := runUpdate2(f.env, UpdateOptions{Rollback: true}); err != nil || !strings.Contains(f.exeBody(t), "1.0.0") {
		t.Fatalf("rollback failed: %v", err)
	}
}

func TestUpdateChecksumMismatchLeavesOldVersionRunning(t *testing.T) {
	f := newFixture(t, scriptBin("2.0.0"))
	asset, _ := releaseAssetName()
	os.WriteFile(filepath.Join(f.base, asset), []byte(scriptBin("evil")), 0o755) // tamper after hashing
	err := runUpdate2(f.env, f.opts())
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(f.exeBody(t), "1.0.0") || f.stops != 0 || !f.running {
		t.Fatal("old version must be untouched and still running")
	}
}

func TestUpdateUnhealthyRollsBack(t *testing.T) {
	f := newFixture(t, scriptBin("2.0.0"))
	f.healthErr = errors.New("daemon crashed")
	err := runUpdate2(f.env, f.opts())
	if err == nil || !strings.Contains(err.Error(), "rolled back") {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(f.exeBody(t), "1.0.0") {
		t.Fatalf("old binary not restored: %q", f.exeBody(t))
	}
	if !f.running {
		t.Fatal("old version must be running again after rollback")
	}
}

func TestUpdateNewBinaryThatDoesNotRunIsRejectedEarly(t *testing.T) {
	f := newFixture(t, "#!/bin/sh\nexit 3\n")
	err := runUpdate2(f.env, f.opts())
	if err == nil || !strings.Contains(err.Error(), "installed binary was not touched") || f.stops != 0 {
		t.Fatalf("err = %v stops=%d", err, f.stops)
	}
}

func TestUpdateAlreadyUpToDateIsNoop(t *testing.T) {
	f := newFixture(t, scriptBin("1.0.0"))
	if err := runUpdate2(f.env, f.opts()); err != nil || f.stops != 0 {
		t.Fatalf("err=%v stops=%d", err, f.stops)
	}
}
