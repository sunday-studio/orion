package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
)

// A system install keeps the binary in a folder the service user owns, so the Node can replace
// itself when Core asks. The old place (/usr/local/bin) stays as a link to it, so the service unit,
// the plist and everything people type keep working.

// layoutPlan says where things are on a machine.
type layoutPlan struct {
	Link     string // what the service and people run, for example /usr/local/bin/orion-node
	Dir      string // the folder the service user owns
	StateDir string // owned by the service user; tells us who that is
	// DropIn is a systemd drop-in that lets the sandboxed service write Dir; empty where there is no
	// sandbox (macOS).
	DropIn string
}

// layoutOps are the parts that need root or a service manager, so tests can run without them.
type layoutOps struct {
	owner         func(path string) (uid, gid int, err error)
	chown         func(path string, uid, gid int) error
	stop, start   func() error
	reloadSystemd func() error
}

func defaultLayoutPlan(statePath string) layoutPlan {
	p := layoutPlan{Link: "/usr/local/bin/orion-node", StateDir: filepath.Dir(statePath)}
	if runtime.GOOS == "darwin" {
		p.Dir = "/usr/local/lib/orion"
		return p
	}
	p.Dir = "/opt/orion"
	p.DropIn = "/etc/systemd/system/orion-node.service.d/self-update.conf"
	return p
}

func systemLayoutOps() layoutOps {
	return layoutOps{
		owner: func(path string) (int, int, error) {
			info, err := os.Stat(path)
			if err != nil {
				return 0, 0, err
			}
			st, ok := info.Sys().(*syscall.Stat_t)
			if !ok {
				return 0, 0, errors.New("cannot read the owner of " + path)
			}
			return int(st.Uid), int(st.Gid), nil
		},
		chown: os.Chown,
		stop:  StopService,
		start: StartService,
		reloadSystemd: func() error {
			if runtime.GOOS != "linux" {
				return nil
			}
			return exec.Command("systemctl", "daemon-reload").Run()
		},
	}
}

// migrateLayout moves a system install's binary into the service user's folder and leaves a link in
// its place. It reports false when there is nothing to do: the install is already laid out this way,
// the Node is not a system install, or it was not started as root.
func migrateLayout(p layoutPlan, ops layoutOps) (moved bool, err error) {
	info, err := os.Lstat(p.Link)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return false, nil // missing, or already a link
	}
	uid, gid, err := ops.owner(p.StateDir)
	if err != nil || uid == 0 {
		return false, nil // no service user to hand the binary to
	}
	if err := os.MkdirAll(p.Dir, 0o755); err != nil {
		return false, fmt.Errorf("create %s: %w", p.Dir, err)
	}
	if err := ops.chown(p.Dir, uid, gid); err != nil {
		return false, fmt.Errorf("give %s to the service user: %w", p.Dir, err)
	}
	target := filepath.Join(p.Dir, filepath.Base(p.Link))
	if err := copyFile(p.Link, target, 0o755); err != nil {
		return false, fmt.Errorf("copy the binary to %s: %w", target, err)
	}
	if _, err := os.Stat(prevPath(p.Link)); err == nil {
		if err := copyFile(prevPath(p.Link), prevPath(target), 0o755); err != nil {
			return false, fmt.Errorf("copy the previous binary: %w", err)
		}
		_ = ops.chown(prevPath(target), uid, gid)
	}
	if err := ops.chown(target, uid, gid); err != nil {
		return false, fmt.Errorf("give %s to the service user: %w", target, err)
	}
	if p.DropIn != "" {
		if err := os.MkdirAll(filepath.Dir(p.DropIn), 0o755); err != nil {
			return false, err
		}
		body := "[Service]\nReadWritePaths=" + p.Dir + "\n"
		if err := os.WriteFile(p.DropIn, []byte(body), 0o644); err != nil {
			return false, fmt.Errorf("let the service write %s: %w", p.Dir, err)
		}
		if err := ops.reloadSystemd(); err != nil {
			return false, fmt.Errorf("reload systemd: %w", err)
		}
	}

	_ = ops.stop()
	tmp := p.Link + ".link"
	_ = os.Remove(tmp)
	if err := os.Symlink(target, tmp); err != nil {
		_ = ops.start()
		return false, fmt.Errorf("link %s: %w", p.Link, err)
	}
	if err := os.Rename(tmp, p.Link); err != nil {
		_ = os.Remove(tmp)
		_ = ops.start()
		return false, fmt.Errorf("replace %s with a link: %w", p.Link, err)
	}
	_ = os.Remove(prevPath(p.Link))
	return true, ops.start()
}

// migrateAfterUpdate runs after a successful `orion-node update` as root on a system install.
func migrateAfterUpdate(opts *Options) {
	if os.Geteuid() != 0 || preferUserInstall() || os.Getenv("ORION_CONTAINER") != "" {
		return
	}
	moved, err := migrateLayout(defaultLayoutPlan(opts.StatePath), systemLayoutOps())
	switch {
	case err != nil:
		fmt.Fprintln(outputWriter, "   "+theme.Muted("could not move the binary so the Node can update itself: "+err.Error()))
	case moved:
		fmt.Fprintln(outputWriter, "   "+theme.Pass()+" "+theme.Muted("the binary now lives where the Node can update itself from the Console"))
	}
}
