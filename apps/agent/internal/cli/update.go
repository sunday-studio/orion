package cli

import (
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
}

func UpdateAgent(opts UpdateOptions) error {
	repo := strings.TrimSpace(opts.Repo)
	if repo == "" {
		repo = "sunday-studio/orion"
	}
	version := strings.TrimSpace(opts.Version)
	if version == "" {
		version = "latest"
	}

	PrintHeader("update")
	PrintInfo("current_version", opts.CurrentVersion)
	PrintInfo("target_version", version)
	PrintInfo("repo", repo)

	executablePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve current executable: %w", err)
	}
	executablePath, err = filepath.EvalSymlinks(executablePath)
	if err != nil {
		return fmt.Errorf("resolve executable symlink: %w", err)
	}
	PrintInfo("binary", executablePath)

	assetName, err := releaseAssetName()
	if err != nil {
		return err
	}
	downloadURL := releaseAssetURL(repo, version, assetName)
	PrintInfo("asset", assetName)

	wasRunning, serviceStatus, err := GetServiceStatus()
	if err != nil {
		return fmt.Errorf("read service status: %w", err)
	}
	PrintInfo("service_state", serviceStatus)

	workDir, err := os.MkdirTemp("", "orion-agent-update-*")
	if err != nil {
		return fmt.Errorf("create update workspace: %w", err)
	}
	defer os.RemoveAll(workDir)

	downloadPath := filepath.Join(workDir, assetName)
	PrintStep("downloading release binary")
	if err := downloadFile(downloadURL, downloadPath); err != nil {
		return err
	}
	if err := os.Chmod(downloadPath, 0o755); err != nil {
		return fmt.Errorf("mark downloaded binary executable: %w", err)
	}
	PrintOK("release binary downloaded")

	PrintStep("checking downloaded version")
	downloadedVersion, err := binaryVersion(downloadPath)
	if err != nil {
		return err
	}
	PrintInfo("downloaded_version", downloadedVersion)

	if wasRunning {
		PrintStep("stopping service")
		if err := StopService(); err != nil {
			return fmt.Errorf("stop service before update: %w", err)
		}
		PrintOK("agent service stopped")
	}

	backupPath := executablePath + ".bak-" + time.Now().UTC().Format("20060102150405")
	PrintStep("backing up current binary")
	if err := copyFile(executablePath, backupPath, 0o755); err != nil {
		if wasRunning {
			_ = StartService()
		}
		return fmt.Errorf("backup current binary: %w", err)
	}
	PrintInfo("backup", backupPath)

	PrintStep("installing updated binary")
	if err := installBinary(downloadPath, executablePath); err != nil {
		_ = installBinary(backupPath, executablePath)
		if wasRunning {
			_ = StartService()
		}
		return fmt.Errorf("install updated binary: %w", err)
	}
	PrintOK("binary updated")

	installedVersion, err := binaryVersion(executablePath)
	if err != nil {
		_ = installBinary(backupPath, executablePath)
		if wasRunning {
			_ = StartService()
		}
		return err
	}
	PrintInfo("installed_version", installedVersion)

	if wasRunning {
		PrintStep("starting service")
		if err := StartService(); err != nil {
			return fmt.Errorf("start service after update: %w", err)
		}
		PrintOK("agent service started")
	} else {
		PrintSkip("service was not running before update")
	}

	PrintOK("update complete")
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
	default:
		return "", fmt.Errorf("unsupported architecture: %s", runtime.GOARCH)
	}

	return fmt.Sprintf("orion-agent-%s-%s", runtime.GOOS, runtime.GOARCH), nil
}

func releaseAssetURL(repo string, version string, assetName string) string {
	if version == "latest" {
		return fmt.Sprintf("https://github.com/%s/releases/latest/download/%s", repo, assetName)
	}
	return fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", repo, version, assetName)
}

func downloadFile(url string, outputPath string) error {
	logging.Debugf("download update asset: url=%s output=%s", url, outputPath)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create download request: %w", err)
	}
	if token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := http.DefaultClient.Do(req)
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

	if _, err := io.Copy(out, resp.Body); err != nil {
		return fmt.Errorf("write download file: %w", err)
	}
	return nil
}

func binaryVersion(path string) (string, error) {
	output, err := exec.Command(path, "version").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("read binary version for %s: %w: %s", path, err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
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
	stagePath := destination + ".new-" + time.Now().UTC().Format("20060102150405")
	if err := copyFile(source, stagePath, 0o755); err != nil {
		return err
	}
	if err := os.Rename(stagePath, destination); err != nil {
		_ = os.Remove(stagePath)
		return err
	}
	return os.Chmod(destination, 0o755)
}
