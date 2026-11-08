package selfupdate

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const newBinary = "#!/bin/sh\necho orion-node 0.9.3\n"

func sum(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }

// fixture lays out an installed binary, a state directory and a fake release.
type fixture struct {
	env   *Env
	exe   string
	asset string
	files map[string]string // url suffix -> content
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	dir := t.TempDir()
	exe := filepath.Join(dir, "bin", "orion-node")
	if err := os.MkdirAll(filepath.Dir(exe), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exe, []byte("old binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(dir, "state")
	if err := os.MkdirAll(state, 0o755); err != nil {
		t.Fatal(err)
	}
	f := &fixture{exe: exe, asset: "orion-node-linux-amd64", files: map[string]string{}}
	f.files["/SHA256SUMS"] = sum(newBinary) + "  " + f.asset + "\n"
	f.files["/"+f.asset] = newBinary
	f.env = &Env{
		Executable: exe, StateDir: state, Current: "0.9.2", GOOS: "linux", GOARCH: "amd64",
		BaseURL: func(tag string) string { return "https://releases.test/" + tag },
		Fetch: func(url, dest string) error {
			content, ok := f.files[strings.TrimPrefix(url, "https://releases.test/v0.9.3")]
			if !ok {
				return errors.New("404")
			}
			return os.WriteFile(dest, []byte(content), 0o755)
		},
		Version: func(path string) (string, error) {
			data, err := os.ReadFile(path)
			if err != nil {
				return "", err
			}
			if strings.Contains(string(data), "0.9.3") {
				return "0.9.3", nil
			}
			return "", errors.New("does not run")
		},
	}
	return f
}

func plan() Plan {
	return Plan{RequestID: "req-1", Version: "0.9.3", Tag: "v0.9.3", SHA256: sum(newBinary)}
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestApplySwapsAndKeepsTheOldBinary(t *testing.T) {
	f := newFixture(t)
	if err := f.env.Apply(plan()); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if read(t, f.exe) != newBinary || read(t, f.env.PrevPath()) != "old binary" {
		t.Fatal("expected the new binary in place and the old one kept")
	}
	m, _ := f.env.ReadMarker()
	if m == nil || m.State != StateSwapped || m.To != "0.9.3" || m.From != "0.9.2" || m.RequestID != "req-1" {
		t.Fatalf("marker = %+v", m)
	}
	if _, err := os.Stat(f.exe + ".new"); !os.IsNotExist(err) {
		t.Error("the staged file should be gone")
	}
}

func TestApplyRefusesAndLeavesTheBinaryAlone(t *testing.T) {
	cases := map[string]func(f *fixture, p *Plan){
		"core and release disagree": func(f *fixture, p *Plan) { p.SHA256 = sum("something else") },
		"download is tampered": func(f *fixture, p *Plan) {
			f.files["/"+f.asset] = "tampered"
		},
		"binary does not run": func(f *fixture, p *Plan) {
			f.files["/"+f.asset] = "not a binary"
			f.files["/SHA256SUMS"] = sum("not a binary") + "  " + f.asset + "\n"
			p.SHA256 = sum("not a binary")
		},
		"wrong version inside": func(f *fixture, p *Plan) {
			f.env.Version = func(string) (string, error) { return "0.9.9", nil }
		},
		"asset not listed": func(f *fixture, p *Plan) { f.files["/SHA256SUMS"] = sum(newBinary) + "  other-file\n" },
		"missing checksum": func(f *fixture, p *Plan) { p.SHA256 = "" },
		"tag mismatch":     func(f *fixture, p *Plan) { p.Tag = "v0.9.4" },
		"download fails":   func(f *fixture, p *Plan) { delete(f.files, "/"+f.asset) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			p := plan()
			mutate(f, &p)
			if err := f.env.Apply(p); err == nil {
				t.Fatal("want an error")
			}
			if read(t, f.exe) != "old binary" {
				t.Error("the installed binary changed")
			}
			if m, _ := f.env.ReadMarker(); m != nil {
				t.Errorf("no marker should be left, got %+v", m)
			}
		})
	}
}

func TestValidateVersions(t *testing.T) {
	f := newFixture(t)
	p := plan()
	p.Version, p.Tag = "0.9.2", "v0.9.2"
	if err := f.env.Validate(p); !errors.Is(err, ErrAlreadyCurrent) {
		t.Errorf("same version = %v", err)
	}
	p.Version, p.Tag = "0.9.1", "v0.9.1"
	if err := f.env.Validate(p); !errors.Is(err, ErrNotNewer) {
		t.Errorf("older version = %v", err)
	}
	f.env.Current = "dev"
	if err := f.env.Validate(plan()); err == nil {
		t.Error("a dev build must not update from the Console")
	}
	for _, bad := range []string{"", "v0.9.3", "0.9", "0.9.3; rm -rf /", "../../etc"} {
		p := plan()
		p.Version = bad
		if err := newFixture(t).env.Validate(p); err == nil {
			t.Errorf("version %q should be refused", bad)
		}
	}
}

func TestWritable(t *testing.T) {
	f := newFixture(t)
	if err := f.env.Writable(); err != nil {
		t.Fatalf("writable dir = %v", err)
	}
	if os.Geteuid() == 0 {
		t.Skip("root can write anywhere")
	}
	if err := os.Chmod(filepath.Dir(f.exe), 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Dir(f.exe), 0o755) })
	if err := f.env.Writable(); !errors.Is(err, ErrNotWritable) {
		t.Fatalf("read-only dir = %v", err)
	}
	if err := f.env.Apply(plan()); !errors.Is(err, ErrNotWritable) {
		t.Fatalf("Apply in a read-only dir = %v", err)
	}
}

// runAfterRestart models the daemon starting again as the new version.
func (f *fixture) restartAs(version string) *Env {
	next := *f.env
	next.Current = version
	return &next
}

func TestConfirmReportsAppliedOnceTheNewVersionStaysUp(t *testing.T) {
	f := newFixture(t)
	if err := f.env.Apply(plan()); err != nil {
		t.Fatal(err)
	}
	next := f.restartAs("0.9.3")
	out, restart, err := next.Startup()
	if err != nil || out != nil || restart {
		t.Fatalf("first start = %+v %v %v", out, restart, err)
	}
	got, err := next.Confirm()
	if err != nil || got == nil || got.Status != "applied" || got.RequestID != "req-1" || !strings.Contains(got.Text, "0.9.2") {
		t.Fatalf("Confirm = %+v %v", got, err)
	}
	if err := next.ClearMarker(); err != nil {
		t.Fatal(err)
	}
	if got, _ := next.Confirm(); got != nil {
		t.Error("nothing should be pending after the marker is cleared")
	}
}

func TestCrashLoopRestoresTheOldVersionAndReportsFailure(t *testing.T) {
	f := newFixture(t)
	if err := f.env.Apply(plan()); err != nil {
		t.Fatal(err)
	}
	next := f.restartAs("0.9.3")
	for i := 1; i <= MaxUnconfirmedStarts; i++ {
		if _, restart, err := next.Startup(); err != nil || restart {
			t.Fatalf("start %d: restart=%v err=%v", i, restart, err)
		}
	}
	_, restart, err := next.Startup()
	if err != nil || !restart {
		t.Fatalf("expected a restore after %d starts: restart=%v err=%v", MaxUnconfirmedStarts, restart, err)
	}
	if read(t, f.exe) != "old binary" {
		t.Fatal("the old binary should be back")
	}
	// The old binary starts and finds the rolled-back marker.
	old := f.restartAs("0.9.2")
	out, restart, err := old.Startup()
	if err != nil || restart || out == nil || out.Status != "failed" || out.RequestID != "req-1" || !strings.Contains(out.Text, "0.9.2") {
		t.Fatalf("old start = %+v %v %v", out, restart, err)
	}
	if err := old.ClearMarker(); err != nil {
		t.Fatal(err)
	}
	if m, _ := old.ReadMarker(); m != nil {
		t.Error("marker should be gone")
	}
}

func TestStartupIgnoresAMarkerForAnotherVersion(t *testing.T) {
	f := newFixture(t)
	if err := f.env.Apply(plan()); err != nil {
		t.Fatal(err)
	}
	// Someone put the old binary back by hand: the process is still 0.9.2.
	if out, restart, err := f.env.Startup(); err != nil || out != nil || restart {
		t.Fatalf("Startup = %+v %v %v", out, restart, err)
	}
	if m, _ := f.env.ReadMarker(); m != nil {
		t.Error("a stale marker should be cleared")
	}
}

func TestUnreadableMarkerIsDropped(t *testing.T) {
	f := newFixture(t)
	if err := os.WriteFile(f.env.markerPath(), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, restart, err := f.env.Startup(); err != nil || out != nil || restart {
		t.Fatalf("Startup = %+v %v %v", out, restart, err)
	}
	if _, err := os.Stat(f.env.markerPath()); !os.IsNotExist(err) {
		t.Error("the unreadable marker should be removed")
	}
}

func TestVersionOrder(t *testing.T) {
	cases := []struct {
		a, b string
		want int
		ok   bool
	}{
		{"0.9.2", "0.9.3", -1, true}, {"0.10.0", "0.9.9", 1, true}, {"0.9.3-rc.1", "0.9.3", -1, true},
		{"0.9.3", "0.9.3", 0, true}, {"dev", "0.9.3", 0, false},
	}
	for _, c := range cases {
		if got, ok := compare(c.a, c.b); got != c.want || ok != c.ok {
			t.Errorf("compare(%q,%q) = %d,%v; want %d,%v", c.a, c.b, got, ok, c.want, c.ok)
		}
	}
	if Tag("0.9.3") != "v0.9.3" || Tag("v0.9.3") != "v0.9.3" {
		t.Error("Tag")
	}
	if _, err := AssetName("windows", "amd64"); err == nil {
		t.Error("windows is not supported")
	}
}
