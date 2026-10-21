package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"orion/agent/internal/logging"
)

type UpdateOptions struct {
	Repo           string
	Version        string
	CurrentVersion string
	LogLines       int
	BaseURL        string // overrides the GitHub release URL (mirrors, tests)
	Rollback       bool   // restore the previous binary
	Force          bool   // reinstall even when already on the target version
	SocketPath     string
}

// updateEnv is what the updater needs from the outside world, so rollback
// paths can be tested without a real service manager.
type updateEnv struct {
	executable string
	fetch      func(url, dest string) error
	version    func(path string) (string, error)
	service    serviceOps
	healthWait time.Duration
	healthPoll time.Duration
	healthy    func(wantVersion string) error // nil when the new daemon is serving
	rep        *updateReporter                // nil in tests
	updated    bool
	newVersion string
	restored   string // version the host is back on after a rollback
}

type serviceOps struct {
	running func() (bool, error)
	stop    func() error
	start   func() error
}

func UpdateAgent(opts UpdateOptions) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve current executable: %w", err)
	}
	if exe, err = filepath.EvalSymlinks(exe); err != nil {
		return fmt.Errorf("resolve executable symlink: %w", err)
	}
	env := &updateEnv{
		executable: exe,
		fetch:      downloadFile,
		version:    binaryVersion,
		service: serviceOps{
			running: func() (bool, error) { r, _, err := GetServiceStatus(); return r, err },
			stop:    StopService,
			start:   StartService,
		},
		healthWait: 45 * time.Second,
		healthPoll: time.Second,
	}
	env.healthy = func(want string) error { return daemonHealthy(opts, want) }
	started := time.Now()
	showReleaseNotes(opts)
	live := !theme.Plain() && stdoutIsTerminal() && terminalWidth() > panelWidth+1 && !opts.Rollback
	if !opts.Rollback {
		env.rep = newUpdateReporter(theme, outputWriter, live, opts.CurrentVersion)
	}
	err = runUpdate2(env, opts)
	switch {
	case opts.Rollback && err == nil:
		fmt.Fprint(outputWriter, "\n   "+theme.Pass()+" Rolled back to "+env.restored+"\n")
	case err == nil && env.updated:
		env.rep.close(live)
		fmt.Fprint(outputWriter, updateDone(theme, env.newVersion, time.Since(started)))
	case err != nil && env.restored != "":
		env.rep.close(false)
		fmt.Fprint(outputWriter, updateRolledBack(theme, env.restored))
		return &CommandError{Summary: "update failed and was rolled back", Cause: err, ExitCode: 1, Silent: true}
	default:
		env.rep.close(false)
	}
	return err
}

func daemonHealthy(opts UpdateOptions, want string) error {
	st, err := controlClient(&Options{SocketPath: opts.SocketPath}).Status()
	if err != nil {
		return err
	}
	if want != "" && st.Version != want {
		return fmt.Errorf("daemon reports version %q, want %q", st.Version, want)
	}
	return nil
}

func prevPath(exe string) string { return exe + ".prev" }

// runUpdate2 stages, verifies, swaps atomically, health-checks and rolls back.
func runUpdate2(env *updateEnv, opts UpdateOptions) error {
	if opts.Rollback {
		return rollbackBinary(env)
	}
	repo := firstNonEmpty(strings.TrimSpace(opts.Repo), "sunday-studio/orion")
	version := firstNonEmpty(strings.TrimSpace(opts.Version), "latest")
	asset, err := releaseAssetName()
	if err != nil {
		return err
	}
	base := strings.TrimRight(opts.BaseURL, "/")
	if base == "" {
		base = releaseBaseURL(repo, version)
	}

	work, err := os.MkdirTemp("", "orion-agent-update-*")
	if err != nil {
		return fmt.Errorf("create update workspace: %w", err)
	}
	defer os.RemoveAll(work)

	env.rep.begin("Downloading")
	sums, bin := filepath.Join(work, "SHA256SUMS"), filepath.Join(work, asset)
	if err := env.fetch(base+"/SHA256SUMS", sums); err != nil {
		return fmt.Errorf("download checksums: %w", err)
	}
	if err := env.fetch(base+"/"+asset, bin); err != nil {
		return err
	}
	if info, serr := os.Stat(bin); serr == nil {
		env.rep.finish("Downloading", "Downloaded", fmt.Sprintf("%.1f MB", float64(info.Size())/1048576), true)
	} else {
		env.rep.finish("Downloading", "Downloaded", "", true)
	}
	env.rep.begin("Verifying checksum")
	if err := verifyChecksum(sums, bin, asset); err != nil {
		return fmt.Errorf("%w; the installed binary was not touched", err)
	}
	env.rep.finish("Verifying checksum", "Checksum matches", "", false)
	if err := os.Chmod(bin, 0o755); err != nil {
		return err
	}
	newVersion, err := env.version(bin)
	if err != nil {
		return fmt.Errorf("downloaded binary does not run: %w; the installed binary was not touched", err)
	}
	env.newVersion = newVersion
	env.rep.setVersion(newVersion)
	if newVersion == opts.CurrentVersion && !opts.Force {
		env.rep.close(true)
		env.rep.say("\n   " + theme.Pass() + " " + padRight("Already on "+newVersion, 30) + theme.Muted("nothing to do · --force reinstalls") + "\n")
		return nil
	}

	// Stage next to the target so the final rename is atomic (same filesystem).
	staged := env.executable + ".new"
	if err := copyFile(bin, staged, 0o755); err != nil {
		return fmt.Errorf("stage new binary: %w", err)
	}
	defer os.Remove(staged)
	if err := copyFile(env.executable, prevPath(env.executable), 0o755); err != nil {
		return fmt.Errorf("keep previous binary for rollback: %w", err)
	}

	wasRunning, rerr := env.service.running()
	if rerr != nil {
		return fmt.Errorf("cannot tell whether the service is running (%v); nothing was changed", rerr)
	}
	if wasRunning {
		env.rep.begin("Stopping service")
		if err := env.service.stop(); err != nil {
			return fmt.Errorf("stop service: %w", err)
		}
		env.rep.finish("Stopping service", "Service stopped", "", false)
	} else {
		env.rep.skip("Stopping service")
		env.rep.skip("Starting and checking health")
	}
	env.rep.begin("Swapping binary")
	if err := os.Rename(staged, env.executable); err != nil {
		restartIf(env, wasRunning)
		return fmt.Errorf("swap binary: %w", err)
	}
	env.rep.finish("Swapping binary", "Swapped binary", "", false)
	if wasRunning {
		env.rep.begin("Starting and checking health")
		if err := env.service.start(); err != nil {
			return failAndRollback(env, wasRunning, fmt.Errorf("start new version: %w", err), opts.CurrentVersion)
		}
		if err := waitHealthy(env, newVersion); err != nil {
			return failAndRollback(env, wasRunning, err, opts.CurrentVersion)
		}
		env.rep.finish("Starting and checking health", "Healthy", "", false)
	}
	env.updated = true
	return nil
}

func restartIf(env *updateEnv, was bool) {
	if was {
		_ = env.service.start()
	}
}

func waitHealthy(env *updateEnv, want string) error {
	if env.healthy == nil {
		return nil
	}
	deadline := time.Now().Add(env.healthWait)
	var last error
	for {
		if running, _ := env.service.running(); running {
			if last = env.healthy(want); last == nil {
				return nil
			}
		} else {
			last = errors.New("service is not running")
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("new version did not become healthy: %w", last)
		}
		time.Sleep(env.healthPoll)
	}
}

func failAndRollback(env *updateEnv, wasRunning bool, cause error, from string) error {
	env.rep.close(true)
	env.rep.flatDone()
	env.rep.say("   " + theme.FailMark() + " " + failHeadline(cause, env.healthWait) + "\n     " + theme.Faint("│") + " " + theme.Muted(firstLineOf(cause.Error())) + "\n   " + theme.Tone("degraded", theme.Spinner(2)) + " Rolling back…\n")
	_ = env.service.stop()
	if err := installBinary(prevPath(env.executable), env.executable); err != nil {
		return fmt.Errorf("%w; ALSO could not restore the previous binary (%v): copy %s over %s manually", cause, err, prevPath(env.executable), env.executable)
	}
	restartIf(env, wasRunning)
	env.restored = from
	return fmt.Errorf("%w; rolled back to the previous version", cause)
}

func rollbackBinary(env *updateEnv) error {
	prev := prevPath(env.executable)
	if _, err := os.Stat(prev); err != nil {
		return fmt.Errorf("no previous binary at %s to roll back to", prev)
	}
	prevVersion, _ := env.version(prev)
	was, _ := env.service.running()
	fmt.Fprint(outputWriter, "\n")
	if was {
		_ = env.service.stop()
		fmt.Fprintln(outputWriter, "   "+theme.Pass()+" Stopped service")
	}
	if err := installBinary(prev, env.executable); err != nil {
		return err
	}
	fmt.Fprintln(outputWriter, "   "+theme.Pass()+" Restored "+prevVersion)
	restartIf(env, was)
	if was {
		fmt.Fprintln(outputWriter, "   "+theme.Pass()+" Started and healthy")
	}
	env.restored = prevVersion
	return nil
}

func verifyChecksum(sumsFile, file, asset string) error {
	data, err := os.ReadFile(sumsFile)
	if err != nil {
		return err
	}
	want := ""
	for _, line := range strings.Split(string(data), "\n") {
		f := strings.Fields(line)
		if len(f) == 2 && strings.TrimPrefix(f[1], "*") == asset {
			want = f[0]
		}
	}
	if want == "" {
		return fmt.Errorf("%s is not listed in SHA256SUMS", asset)
	}
	fh, err := os.Open(file)
	if err != nil {
		return err
	}
	defer fh.Close()
	h := sha256.New()
	if _, err := io.Copy(h, fh); err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != want {
		return fmt.Errorf("checksum mismatch for %s (expected %s, got %s)", asset, want, got)
	}
	return nil
}

func releaseAssetName() (string, error) {
	switch runtime.GOOS {
	case "darwin", "linux":
	default:
		return "", fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
	switch runtime.GOARCH {
	case "amd64", "arm64":
		return fmt.Sprintf("orion-agent-%s-%s", runtime.GOOS, runtime.GOARCH), nil
	case "arm":
		if runtime.GOOS == "linux" {
			return "orion-agent-linux-armv7", nil
		}
	}
	return "", fmt.Errorf("unsupported architecture: %s", runtime.GOARCH)
}

func releaseBaseURL(repo, version string) string {
	if version == "latest" {
		return fmt.Sprintf("https://github.com/%s/releases/latest/download", repo)
	}
	return fmt.Sprintf("https://github.com/%s/releases/download/%s", repo, version)
}

func releaseAssetURL(repo string, version string, assetName string) string {
	return releaseBaseURL(repo, version) + "/" + assetName
}

func downloadFile(url string, outputPath string) error {
	logging.Debugf("download update asset: url=%s output=%s", url, outputPath)
	if strings.HasPrefix(url, "file://") {
		return copyFile(strings.TrimPrefix(url, "file://"), outputPath, 0o755)
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create download request: %w", err)
	}
	// The token is only for GitHub; never send it to a mirror given via --base-url.
	if token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); token != "" && req.URL.Hostname() == "github.com" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := (&http.Client{Timeout: 5 * time.Minute}).Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: status %d", url, resp.StatusCode)
	}
	out, err := os.OpenFile(outputPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return fmt.Errorf("create download file: %w", err)
	}
	defer out.Close()
	if _, err := io.Copy(out, io.LimitReader(resp.Body, 256<<20)); err != nil {
		return fmt.Errorf("write download file: %w", err)
	}
	return nil
}

func binaryVersion(path string) (string, error) {
	output, err := exec.Command(path, "version").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("read binary version for %s: %w: %s", path, err, strings.TrimSpace(string(output)))
	}
	return strings.TrimPrefix(strings.TrimSpace(string(output)), "orion-agent "), nil
}

func copyFile(source string, destination string, mode os.FileMode) error {
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(destination, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Chmod(destination, mode)
}

func installBinary(source string, destination string) error {
	stagePath := destination + ".swap"
	if err := copyFile(source, stagePath, 0o755); err != nil {
		return err
	}
	if err := os.Rename(stagePath, destination); err != nil {
		_ = os.Remove(stagePath)
		return err
	}
	return os.Chmod(destination, 0o755)
}

// failHeadline says why an update was abandoned, in the board's words.
func failHeadline(cause error, wait time.Duration) string {
	if strings.Contains(cause.Error(), "did not become healthy") {
		return fmt.Sprintf("New version is not healthy after %d s", int(wait.Seconds()))
	}
	return "New version could not start"
}

// showReleaseNotes prints what is new before an update starts. Only a person at
// a terminal sees it, and a missing or slow answer is skipped silently.
func showReleaseNotes(opts UpdateOptions) {
	if opts.Rollback || opts.BaseURL != "" || theme.Plain() || !stdoutIsTerminal() {
		return
	}
	n, err := fetchReleaseNotes(firstNonEmpty(strings.TrimSpace(opts.Repo), "sunday-studio/orion"), firstNonEmpty(strings.TrimSpace(opts.Version), "latest"))
	if err != nil || strings.TrimPrefix(n.Tag, "v") == strings.TrimPrefix(opts.CurrentVersion, "v") {
		return
	}
	fmt.Fprint(outputWriter, releaseNotesView(n, opts.CurrentVersion, terminalWidth()))
}
