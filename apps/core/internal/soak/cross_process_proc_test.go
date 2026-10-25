package soak

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// proc is one real Core process (API or Worker) sharing a data directory with its sibling.
type proc struct {
	name string
	cmd  *exec.Cmd
	log  string
	done chan error
}

func crossFreePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func buildBinary(t *testing.T, out, pkg string) {
	t.Helper()
	cmd := exec.Command("go", "build", "-o", out, pkg)
	cmd.Dir = "../.."
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build %s: %v\n%s", pkg, err, b)
	}
}

func startProc(t *testing.T, dir, name, bin string, env []string) *proc {
	t.Helper()
	logPath := filepath.Join(dir, name+".log")
	f, err := os.Create(logPath)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin)
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout, cmd.Stderr = f, f
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	p := &proc{name: name, cmd: cmd, log: logPath, done: make(chan error, 1)}
	go func() { p.done <- cmd.Wait(); f.Close() }()
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	return p
}

// stop sends SIGTERM and waits; it returns whether the process exited cleanly in time.
func (p *proc) stop(timeout time.Duration) error {
	_ = p.cmd.Process.Signal(syscall.SIGTERM)
	select {
	case err := <-p.done:
		return err
	case <-time.After(timeout):
		_ = p.cmd.Process.Kill()
		return fmt.Errorf("%s did not exit within %v after SIGTERM", p.name, timeout)
	}
}

func (p *proc) rssKB() int {
	out, err := exec.Command("ps", "-o", "rss=", "-p", strconv.Itoa(p.cmd.Process.Pid)).Output()
	if err != nil {
		return 0
	}
	n, _ := strconv.Atoi(strings.TrimSpace(string(out)))
	return n
}

func (p *proc) logContains(needles ...string) []string {
	data, _ := os.ReadFile(p.log)
	var hits []string
	for _, n := range needles {
		if bytes.Contains(bytes.ToLower(data), []byte(strings.ToLower(n))) {
			hits = append(hits, n)
		}
	}
	return hits
}

func crossFileSize(path string) int64 {
	if info, err := os.Stat(path); err == nil {
		return info.Size()
	}
	return 0
}

func crossWaitHealthy(t *testing.T, base string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if resp, err := http.Get(base + "/health"); err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				return
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("API never became healthy")
}

func crossPost(t *testing.T, base, path string, body any, out any) {
	t.Helper()
	raw, _ := json.Marshal(body)
	resp, err := http.Post(base+path, "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		t.Fatalf("POST %s = %d %s", path, resp.StatusCode, data)
	}
	if out != nil {
		if err := json.Unmarshal(data, out); err != nil {
			t.Fatal(err)
		}
	}
}
