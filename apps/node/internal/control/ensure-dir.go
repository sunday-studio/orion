package control

import (
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
)

const (
	darwinSocketPath   = "/usr/local/var/run/orion/node.sock"
	darwinServiceUser  = "_orion"
	darwinServiceGroup = "_orion"
)

// EnsureSocketDir makes sure the macOS socket folder exists and belongs to the service account.
// Nothing else creates it: launchd has no runtime directory, and the daemon runs as the unprivileged
// _orion user, which cannot create folders under the root-owned /usr/local/var/run. Without the folder
// the socket never starts, and status, doctor and restart cannot reach a node that is otherwise fine.
// Call it from commands that run as root (start, restart, update). It does nothing on other systems,
// for a custom socket path (never chown a folder the operator picked), or when not root.
func EnsureSocketDir(socketPath string) error {
	if runtime.GOOS != "darwin" || socketPath != darwinSocketPath || os.Geteuid() != 0 {
		return nil
	}
	dir := filepath.Dir(socketPath)
	if err := os.MkdirAll(dir, socketDirMode); err != nil {
		return err
	}
	u, err := user.Lookup(darwinServiceUser)
	if err != nil {
		return nil // no service account: the node runs as someone else and creates the folder itself
	}
	g, err := user.LookupGroup(darwinServiceGroup)
	if err != nil {
		return nil
	}
	uid, uidErr := strconv.Atoi(u.Uid)
	gid, gidErr := strconv.Atoi(g.Gid)
	if uidErr != nil || gidErr != nil {
		return nil
	}
	return os.Chown(dir, uid, gid)
}
