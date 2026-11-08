package cli

import (
	"os"
	"path/filepath"
	"testing"
)

type opLog struct {
	chowns  []string
	stopped int
	started int
	reload  int
}

func layoutFixture(t *testing.T, uid int) (layoutPlan, layoutOps, *opLog) {
	t.Helper()
	root := t.TempDir()
	log := &opLog{}
	p := layoutPlan{
		Link:     filepath.Join(root, "usr-local-bin", "orion-node"),
		Dir:      filepath.Join(root, "opt-orion"),
		StateDir: filepath.Join(root, "var-lib-orion"),
		DropIn:   filepath.Join(root, "systemd", "orion-node.service.d", "self-update.conf"),
	}
	for _, d := range []string{filepath.Dir(p.Link), p.StateDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(p.Link, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(prevPath(p.Link), []byte("previous"), 0o755); err != nil {
		t.Fatal(err)
	}
	ops := layoutOps{
		owner:         func(string) (int, int, error) { return uid, uid, nil },
		chown:         func(path string, _, _ int) error { log.chowns = append(log.chowns, filepath.Base(path)); return nil },
		stop:          func() error { log.stopped++; return nil },
		start:         func() error { log.started++; return nil },
		reloadSystemd: func() error { log.reload++; return nil },
	}
	return p, ops, log
}

func TestMigrateLayoutMovesTheBinaryAndLeavesALink(t *testing.T) {
	p, ops, log := layoutFixture(t, 999)
	moved, err := migrateLayout(p, ops)
	if err != nil || !moved {
		t.Fatalf("migrate = %v %v", moved, err)
	}
	target := filepath.Join(p.Dir, "orion-node")
	if got, _ := os.ReadFile(target); string(got) != "binary" {
		t.Error("the binary should be in the service user's folder")
	}
	if got, _ := os.ReadFile(prevPath(target)); string(got) != "previous" {
		t.Error("the previous binary should move too")
	}
	if dest, err := os.Readlink(p.Link); err != nil || dest != target {
		t.Errorf("link = %q %v, want %q", dest, err, target)
	}
	if body, _ := os.ReadFile(p.DropIn); string(body) != "[Service]\nReadWritePaths="+p.Dir+"\n" {
		t.Errorf("drop-in = %q", body)
	}
	if log.stopped != 1 || log.started != 1 || log.reload != 1 {
		t.Errorf("stop/start/reload = %d/%d/%d, want 1/1/1", log.stopped, log.started, log.reload)
	}
	if len(log.chowns) < 2 {
		t.Errorf("chowns = %v", log.chowns)
	}
}

func TestMigrateLayoutIsRepeatableAndSkipsWhatItShouldNot(t *testing.T) {
	p, ops, log := layoutFixture(t, 999)
	if moved, err := migrateLayout(p, ops); err != nil || !moved {
		t.Fatalf("first run = %v %v", moved, err)
	}
	log.stopped, log.started = 0, 0
	if moved, err := migrateLayout(p, ops); err != nil || moved || log.stopped != 0 {
		t.Errorf("second run = %v %v stopped=%d: an installed layout must be left alone", moved, err, log.stopped)
	}
	rootOwned, rootOps, _ := layoutFixture(t, 0)
	if moved, err := migrateLayout(rootOwned, rootOps); err != nil || moved {
		t.Errorf("a state folder owned by root has no service user: %v %v", moved, err)
	}
	missing, missingOps, _ := layoutFixture(t, 999)
	_ = os.Remove(missing.Link)
	if moved, err := migrateLayout(missing, missingOps); err != nil || moved {
		t.Errorf("no binary: %v %v", moved, err)
	}
}
