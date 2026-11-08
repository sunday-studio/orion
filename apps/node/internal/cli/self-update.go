package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	node "orion/node/internal"
	"orion/node/internal/logging"
	"orion/node/internal/selfupdate"
	"orion/node/internal/transport"
)

const (
	// updateRestartExit is how the daemon leaves after swapping in a new binary. The service manager
	// restarts a daemon that exits with a failure, which is what loads the new version.
	updateRestartExit = 75
	// updateConfirmAfter is how long the new version must stay up before the update counts as done.
	updateConfirmAfter = 90 * time.Second
	// requestGiveUp is when an unreported result is dropped: Core expires requests after 24 hours.
	requestGiveUp = 25 * time.Hour
	releaseRepo   = "sunday-studio/orion"
	// Releases are signed by this workflow on this ref; a signature for anything else is refused.
	releaseWorkflow = "node-binaries.yml"
	releaseRef      = "refs/heads/main"
)

// errUpdateRestart tells the request applier that the binary was swapped and no result is reported
// yet: the new version reports it once it has stayed up.
var errUpdateRestart = errors.New("restarting to finish the update")

// These are the only places a test build changes how an update finds and trusts a release (see
// self-update_e2e.go, compiled only with the orion_e2e build tag). Release builds never reassign them.
var (
	releaseBase   = releaseBaseURL
	verifyRelease = func(sums, sig, cert []byte) error {
		return selfupdate.VerifySignature(sums, sig, cert, selfupdate.GitHubWorkflowIdentity(releaseRepo, releaseWorkflow, releaseRef))
	}
	afterUpdateBegin = func() {}
)

func selfUpdateEnv(statePath string) (*selfupdate.Env, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("find the running binary: %w", err)
	}
	if exe, err = filepath.EvalSymlinks(exe); err != nil {
		return nil, fmt.Errorf("resolve the running binary: %w", err)
	}
	return &selfupdate.Env{
		Executable: exe,
		StateDir:   filepath.Dir(statePath),
		Current:    node.Version,
		GOOS:       runtime.GOOS,
		GOARCH:     runtime.GOARCH,
		Fetch:      downloadFile,
		Version:    binaryVersionWithin(30 * time.Second),
		BaseURL:    func(tag string) string { return releaseBase(releaseRepo, tag) },
		Verify:     func(sums, sig, cert []byte) error { return verifyRelease(sums, sig, cert) },
	}, nil
}

// supervised is true when a service manager runs this process, so exiting makes it start again. A
// Node started by hand would just stop.
func supervised() bool {
	return os.Getenv("INVOCATION_ID") != "" || os.Getppid() == 1 || os.Getenv("XPC_SERVICE_NAME") == "com.orion.node"
}

// selfUpdateReady is true when this Node can take an update from the Console: it is not in a
// container and it can replace its own binary.
func selfUpdateReady() bool {
	if os.Getenv("ORION_CONTAINER") != "" || !supervised() {
		return false
	}
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return selfupdate.Ready(exe, node.Version)
}

// selfUpdater runs update_node requests inside the daemon.
type selfUpdater struct {
	env     *selfupdate.Env
	enabled func() bool
	restart func()

	mu      sync.Mutex
	pending *selfupdate.Outcome // an outcome waiting to be reported to Core
}

func newSelfUpdater(statePath string, enabled func() bool) (*selfUpdater, error) {
	env, err := selfUpdateEnv(statePath)
	if err != nil {
		return nil, err
	}
	return &selfUpdater{env: env, enabled: enabled}, nil
}

// begin looks at what an earlier run left behind. It runs before the config and state are loaded, so
// a new version that cannot get that far is still counted. It returns restart=true when the old
// binary was put back and the caller must exit so the service manager starts it.
func (u *selfUpdater) begin() (restart bool, err error) {
	out, restart, err := u.env.Startup()
	if err != nil || restart {
		return restart, err
	}
	if out != nil {
		u.setPending(out)
	}
	return false, nil
}

// watch confirms the update once the new version has stayed up.
func (u *selfUpdater) watch(ctx context.Context) {
	m, _ := u.env.ReadMarker()
	if m == nil || m.State != selfupdate.StateSwapped || m.Confirmed {
		return
	}
	go func() {
		select {
		case <-ctx.Done():
		case <-time.After(updateConfirmAfter):
			if confirmed, err := u.env.Confirm(); err == nil && confirmed != nil {
				u.setPending(confirmed)
			}
		}
	}()
}

func (u *selfUpdater) setPending(out *selfupdate.Outcome) {
	u.mu.Lock()
	u.pending = out
	u.mu.Unlock()
}

// flush reports a finished update to Core; the marker is removed only once Core has the answer.
func (u *selfUpdater) flush(report func(id, status, text string) error) {
	u.mu.Lock()
	out := u.pending
	u.mu.Unlock()
	if out == nil {
		return
	}
	if m, _ := u.env.ReadMarker(); m != nil && time.Since(m.At) > requestGiveUp {
		// Core drops a request after a day, so nobody is waiting for this answer any more.
		logging.Warnf("Giving up on reporting the update to %s: Core no longer holds the request", m.To)
		u.setPending(nil)
		_ = u.env.ClearMarker()
		return
	}
	if err := report(out.RequestID, out.Status, out.Text); err != nil {
		logging.Warnf("Could not report the update result yet: %v", err)
		return
	}
	u.setPending(nil)
	if err := u.env.ClearMarker(); err != nil {
		logging.Warnf("Could not clear the update marker: %v", err)
	}
}

// handle carries out one update_node request.
func (u *selfUpdater) handle(r transport.PulledRequest) (string, error) {
	if !u.enabled() {
		return "", errors.New("remote updates are turned off on this Node (remote_update: false in its config)")
	}
	if os.Getenv("ORION_CONTAINER") != "" {
		return "", errors.New("this Node runs in a container; replace the container with a newer image")
	}
	plan := selfupdate.Plan{
		RequestID: r.ID,
		Version:   strings.TrimSpace(stringField(r.Payload, "version")),
		Tag:       strings.TrimSpace(stringField(r.Payload, "tag")),
		SHA256:    strings.TrimSpace(stringField(r.Payload, "sha256")),
	}
	switch err := u.env.Apply(plan); {
	case errors.Is(err, selfupdate.ErrAlreadyCurrent):
		return "Already on " + plan.Version, nil
	case err != nil:
		return "", err
	}
	logging.Infof("Updated the binary from %s to %s; restarting", u.env.Current, plan.Version)
	u.restart()
	return "", errUpdateRestart
}

func stringField(payload map[string]any, key string) string {
	if v, ok := payload[key].(string); ok {
		return v
	}
	return ""
}

// beginSelfUpdate builds the updater for a daemon and looks at the last update. updater is nil when
// updates from the Console cannot work here.
func beginSelfUpdate(opts *Options) (updater *selfUpdater, restore bool, err error) {
	u, err := newSelfUpdater(opts.StatePath, func() bool { return userConfigRemoteUpdate(opts) })
	if err != nil {
		return nil, false, err
	}
	restore, err = u.begin()
	return u, restore, err
}
