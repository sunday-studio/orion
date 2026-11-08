package cli

import (
	"os"
	"path/filepath"
	"strings"
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
		Dir:      filepath.Join(root, "opt-orion-node"),
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
	if body, _ := os.ReadFile(p.DropIn); string(body) != "[Service]\nReadWritePaths=-"+p.Dir+"\n" {
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

func TestMigrateLayoutRefusesALinkWhereTheFolderShouldBe(t *testing.T) {
	p, ops, log := layoutFixture(t, 999)
	elsewhere := t.TempDir()
	if err := os.Symlink(elsewhere, p.Dir); err != nil {
		t.Fatal(err)
	}
	moved, err := migrateLayout(p, ops)
	if err == nil || moved || log.stopped != 0 {
		t.Fatalf("migrate = %v %v stopped=%d: a link at the folder must stop the move", moved, err, log.stopped)
	}
	if entries, _ := os.ReadDir(elsewhere); len(entries) != 0 {
		t.Error("nothing may be written through the link")
	}
}

func TestMigrateLayoutSaysWhenOnlyTheStartFailed(t *testing.T) {
	p, ops, _ := layoutFixture(t, 999)
	ops.start = func() error { return os.ErrPermission }
	moved, err := migrateLayout(p, ops)
	if !moved || err == nil || !strings.Contains(err.Error(), "did not start") {
		t.Fatalf("migrate = %v %v: the move happened and the start failed, and the message must say so", moved, err)
	}
}

func TestMigrateLayoutRefusesAFolderHoldingSomeoneElsesFiles(t *testing.T) {
	p, ops, log := layoutFixture(t, 999)
	if err := os.MkdirAll(p.Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p.Dir, "docker-compose.yml"), []byte("core"), 0o644); err != nil {
		t.Fatal(err)
	}
	moved, err := migrateLayout(p, ops)
	if moved || err == nil || !strings.Contains(err.Error(), "docker-compose.yml") {
		t.Fatalf("migrate = %v %v, want a refusal naming the foreign file", moved, err)
	}
	if len(log.chowns) != 0 || log.stopped != 0 {
		t.Errorf("nothing should change hands or restart: chowns=%v stopped=%d", log.chowns, log.stopped)
	}
	if info, _ := os.Lstat(p.Link); info == nil || info.Mode()&os.ModeSymlink != 0 {
		t.Error("the binary must stay where it was")
	}
}
