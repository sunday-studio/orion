// Package selfupdate replaces the running Node binary with a newer release without a terminal and
// without root. It needs the binary to live in a directory the service user can write; see
// docs/plans/node-update-from-console.md.
package selfupdate

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	// ErrAlreadyCurrent means the Node already runs the requested version.
	ErrAlreadyCurrent = errors.New("already on that version")
	// ErrNotNewer means the request would not move the Node forward.
	ErrNotNewer = errors.New("that version is not newer than the one running")
	// ErrNotWritable means the binary sits where the service user cannot replace it.
	ErrNotWritable = errors.New("the Node cannot replace its own binary where it is installed")
)

var shaPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// Plan is what Core asked for.
type Plan struct {
	RequestID string
	Version   string
	// Tag is the GitHub release tag exactly as GitHub lists it: the version, with or without a "v".
	Tag string
	// SHA256 is the checksum Core read for this machine's file. It has to match the release's own list.
	SHA256 string
}

// Env is everything the updater needs from the machine, so tests need no network or service manager.
type Env struct {
	// Executable is the resolved path of the running binary.
	Executable string
	// StateDir holds the marker and the download workspace; the service user owns it.
	StateDir string
	// Current is the running version.
	Current string
	GOOS    string
	GOARCH  string
	// Fetch downloads url to dest.
	Fetch func(url, dest string) error
	// Version runs `<path> version` and returns the version it prints.
	Version func(path string) (string, error)
	// BaseURL is where the files of a release live.
	BaseURL func(tag string) string
	// Verify checks the release's signature over SHA256SUMS. It is required: an update never installs
	// a release whose signature has not been checked.
	Verify func(sums, sig, cert []byte) error
	Now    func() time.Time
}

func (e *Env) now() time.Time {
	if e.Now != nil {
		return e.Now()
	}
	return time.Now()
}

// PrevPath is where the old binary is kept while an update is unconfirmed.
func (e *Env) PrevPath() string { return e.Executable + ".prev" }

// Writable says whether the Node can replace its own binary, which decides whether it offers updates
// from the Console at all.
func (e *Env) Writable() error {
	f, err := os.CreateTemp(filepath.Dir(e.Executable), ".orion-write-test-*")
	if err != nil {
		return fmt.Errorf("%w (%v)", ErrNotWritable, err)
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return nil
}

// Validate checks a request before anything is downloaded.
func (e *Env) Validate(p Plan) error {
	if _, ok := parseVersion(p.Version); !ok || strings.HasPrefix(p.Version, "v") {
		return fmt.Errorf("%q is not a release version", p.Version)
	}
	if p.Tag != p.Version && p.Tag != "v"+p.Version {
		return fmt.Errorf("release tag %q does not match version %q", p.Tag, p.Version)
	}
	if !shaPattern.MatchString(strings.ToLower(p.SHA256)) {
		return errors.New("the request carries no checksum")
	}
	cmp, ok := compare(e.Current, p.Version)
	if !ok {
		return fmt.Errorf("this build (%q) is not a release, so it cannot be updated from the Console", e.Current)
	}
	switch {
	case cmp == 0:
		return ErrAlreadyCurrent
	case cmp > 0:
		return ErrNotNewer
	}
	return nil
}

// Apply downloads, checks and swaps in the requested release. On success the new binary is in place
// and the old one is kept; the caller must now exit so the service manager starts the new one. The
// installed binary is not touched unless every check passed.
func (e *Env) Apply(p Plan) error {
	if err := e.Validate(p); err != nil {
		return err
	}
	if err := e.Writable(); err != nil {
		return err
	}
	asset, err := AssetName(e.GOOS, e.GOARCH)
	if err != nil {
		return err
	}
	work, err := os.MkdirTemp(e.StateDir, "update-*")
	if err != nil {
		return fmt.Errorf("create the update workspace: %w", err)
	}
	defer os.RemoveAll(work)

	base := strings.TrimRight(e.BaseURL(p.Tag), "/")
	sums, bin := filepath.Join(work, "SHA256SUMS"), filepath.Join(work, asset)
	if err := e.Fetch(base+"/SHA256SUMS", sums); err != nil {
		return fmt.Errorf("download the checksums: %w", err)
	}
	if err := e.Fetch(base+"/"+asset, bin); err != nil {
		return fmt.Errorf("download %s: %w", asset, err)
	}
	if err := e.verifySignature(base, work, sums); err != nil {
		return err
	}
	listed, err := checksumFor(sums, asset)
	if err != nil {
		return err
	}
	if !strings.EqualFold(listed, p.SHA256) {
		return fmt.Errorf("Core and the release disagree about the checksum of %s; nothing was installed", asset)
	}
	got, err := fileSHA256(bin)
	if err != nil {
		return err
	}
	if got != listed {
		return fmt.Errorf("the download of %s does not match its checksum; nothing was installed", asset)
	}
	if err := os.Chmod(bin, 0o755); err != nil {
		return err
	}
	version, err := e.Version(bin)
	if err != nil {
		return fmt.Errorf("the downloaded binary does not run: %w", err)
	}
	if strings.TrimPrefix(version, "v") != p.Version {
		return fmt.Errorf("the downloaded binary is version %q, not %q; nothing was installed", version, p.Version)
	}

	staged := e.Executable + ".new"
	if err := copyFile(bin, staged, 0o755); err != nil {
		return fmt.Errorf("stage the new binary: %w", err)
	}
	defer os.Remove(staged)
	if err := copyFile(e.Executable, e.PrevPath(), 0o755); err != nil {
		return fmt.Errorf("keep the old binary for a rollback: %w", err)
	}
	if err := e.writeMarker(Marker{RequestID: p.RequestID, From: e.Current, To: p.Version, At: e.now().UTC(), State: StateSwapped}); err != nil {
		return fmt.Errorf("record the update: %w", err)
	}
	if err := os.Rename(staged, e.Executable); err != nil {
		_ = e.ClearMarker()
		return fmt.Errorf("swap in the new binary: %w", err)
	}
	return nil
}

// verifySignature fetches the release's signature and certificate and checks them against the
// checksums that were just downloaded.
func (e *Env) verifySignature(base, work, sums string) error {
	if e.Verify == nil {
		return errors.New("no signature check is configured; nothing was installed")
	}
	sigFile, certFile := filepath.Join(work, "SHA256SUMS.sig"), filepath.Join(work, "SHA256SUMS.pem")
	if err := e.Fetch(base+"/SHA256SUMS.sig", sigFile); err != nil {
		return fmt.Errorf("the release has no signature (%v); nothing was installed", err)
	}
	if err := e.Fetch(base+"/SHA256SUMS.pem", certFile); err != nil {
		return fmt.Errorf("the release has no signing certificate (%v); nothing was installed", err)
	}
	data, sig, cert := mustRead(sums), mustRead(sigFile), mustRead(certFile)
	if err := e.Verify(data, sig, cert); err != nil {
		return fmt.Errorf("the release signature does not verify: %w; nothing was installed", err)
	}
	return nil
}

func mustRead(path string) []byte {
	data, _ := os.ReadFile(path)
	return data
}

func checksumFor(sumsFile, asset string) (string, error) {
	data, err := os.ReadFile(sumsFile)
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		f := strings.Fields(line)
		if len(f) == 2 && strings.TrimPrefix(f[1], "*") == asset {
			return strings.ToLower(f[0]), nil
		}
	}
	return "", fmt.Errorf("%s is not listed in the release's checksums", asset)
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// copyFile writes destination by creating a fresh file next to it and renaming it into place. It
// never opens an existing destination, so a file left behind by another user, a read-only file or a
// link cannot get in the way or be written through.
func copyFile(source, destination string, mode os.FileMode) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := fmt.Sprintf("%s.tmp-%d-%d", destination, os.Getpid(), time.Now().UnixNano())
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := out.Sync(); err != nil {
		_ = out.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Chmod(tmp, mode); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, destination); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// Restore puts the kept binary back in place of the current one.
// It refuses a kept binary that does not run, so a truncated copy never replaces a binary that starts.
func (e *Env) Restore(want string) error {
	if e.Version != nil {
		got, err := e.Version(e.PrevPath())
		if err != nil || strings.TrimPrefix(got, "v") != strings.TrimPrefix(want, "v") {
			return fmt.Errorf("the kept binary %s is not version %s (%v)", e.PrevPath(), want, err)
		}
	}
	return copyFile(e.PrevPath(), e.Executable, 0o755)
}

// Ready says whether a binary can be updated this way at all: it is a release build, and the folder
// it sits in is one the service user can write to.
func Ready(executable, current string) bool {
	if _, ok := parseVersion(current); !ok {
		return false
	}
	return (&Env{Executable: executable}).Writable() == nil
}
