// Package discover finds things on this host worth monitoring (services,
// containers, listening ports, log files) and proposes monitors and log
// sources with a confidence label. It only reads; it never changes the host.
package discover

import (
	"context"
	"fmt"
	"os/exec"
	"sort"
	"time"

	"orion/agent/internal/config"
)

// Kind of discovered thing.
type Kind string

const (
	KindService   Kind = "service"   // systemd unit or launchd label
	KindContainer Kind = "container" // docker container
	KindPM2       Kind = "pm2"
	KindPort      Kind = "port"
	KindLog       Kind = "log"
)

// Confidence says how sure we are that the user wants this monitored.
type Confidence string

const (
	High   Confidence = "high"   // a well-known app or an explicitly managed workload
	Medium Confidence = "medium" // plausible
	Low    Confidence = "low"    // generic or infrastructure noise
)

// Candidate is one proposal. Exactly one of Monitor or LogPath is set.
type Candidate struct {
	ID         string              `json:"id"` // stable selector, e.g. service:nginx
	Kind       Kind                `json:"kind"`
	Name       string              `json:"name"`
	Detail     string              `json:"detail,omitempty"`
	Confidence Confidence          `json:"confidence"`
	Monitor    *config.UserMonitor `json:"monitor,omitempty"`
	LogPath    string              `json:"log_path,omitempty"`
	// StoragePath is a directory to size under app_resources.paths.
	StoragePath string `json:"storage_path,omitempty"`
}

// Runner executes a command and returns stdout.
type Runner func(ctx context.Context, name string, args ...string) ([]byte, error)

// ListeningPort is one TCP listener.
type ListeningPort struct {
	Port    int
	Process string
}

// Env abstracts the host so discovery can run against fixtures.
type Env struct {
	GOOS      string
	Run       Runner
	Ports     func(ctx context.Context) ([]ListeningPort, error)
	Glob      func(pattern string) ([]string, error)
	Exists    func(dir string) bool // directory exists
	Readable  func(dir string) bool // the agent user can list it
	Interval  string                // monitor interval written into proposals
	Timeout   time.Duration
	LookupBin func(name string) bool
}

// HostEnv returns the real-host environment.
func HostEnv(goos string) Env {
	return Env{
		GOOS: goos,
		Run: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return exec.CommandContext(ctx, name, args...).Output()
		},
		Ports:     hostPorts,
		Glob:      globFiles,
		Exists:    dirExists,
		Readable:  dirReadable,
		Interval:  "60s",
		Timeout:   4 * time.Second,
		LookupBin: func(n string) bool { _, err := exec.LookPath(n); return err == nil },
	}
}

// Result is everything found, sorted for stable output, plus per-source notes.
type Result struct {
	Candidates []Candidate `json:"candidates"`
	Notes      []string    `json:"notes,omitempty"`
}

// Run executes every source; a failing source adds a note and never aborts.
func Run(ctx context.Context, e Env) Result {
	if e.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, e.Timeout)
		defer cancel()
	}
	var res Result
	add := func(cs []Candidate, err error, src string) {
		if err != nil {
			res.Notes = append(res.Notes, fmt.Sprintf("%s: %v", src, err))
		}
		res.Candidates = append(res.Candidates, cs...)
	}
	switch e.GOOS {
	case "linux":
		if e.LookupBin == nil || e.LookupBin("systemctl") {
			cs, err := systemdUnits(ctx, e)
			add(cs, err, "systemd")
		}
	case "darwin":
		cs, err := launchdServices(ctx, e)
		add(cs, err, "launchd")
	}
	if e.LookupBin == nil || e.LookupBin("docker") {
		cs, err := dockerContainers(ctx, e)
		add(cs, err, "docker")
	}
	if e.LookupBin == nil || e.LookupBin("pm2") {
		cs, err := pm2Apps(ctx, e)
		add(cs, err, "pm2")
	}
	cs, err := listeningPorts(ctx, e)
	add(cs, err, "ports")
	add(logFiles(e), nil, "logs")
	{
		cs, notes := storagePaths(ctx, e, res.Candidates)
		res.Candidates = append(res.Candidates, cs...)
		res.Notes = append(res.Notes, notes...)
	}

	res.Candidates = dedupe(res.Candidates)
	sort.SliceStable(res.Candidates, func(i, j int) bool {
		a, b := res.Candidates[i], res.Candidates[j]
		if rank(a.Confidence) != rank(b.Confidence) {
			return rank(a.Confidence) < rank(b.Confidence)
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return a.Name < b.Name
	})
	return res
}

func rank(c Confidence) int {
	switch c {
	case High:
		return 0
	case Medium:
		return 1
	}
	return 2
}

func dedupe(in []Candidate) []Candidate {
	seen := map[string]bool{}
	out := in[:0]
	for _, c := range in {
		if seen[c.ID] {
			continue
		}
		seen[c.ID] = true
		out = append(out, c)
	}
	return out
}

func (e Env) interval() string {
	if e.Interval == "" {
		return "60s"
	}
	return e.Interval
}
