// Package discover finds things on this host worth monitoring (services,
// containers, listening ports, log files) and proposes monitors and log
// sources with a confidence label. It only reads; it never changes the host.
package discover

import (
	"context"
	"fmt"
	"os/exec"
	"sort"
	"strings"
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
	KindDisk      Kind = "disk"
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
	// Status is "suggested" or "monitored"; ManagedIn says where a monitored
	// match lives ("file", "scan", "console").
	Status    string `json:"status"`
	ManagedIn string `json:"managed_in,omitempty"`
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
	// Kinds limits what is scanned (nil: everything). Ignore lists candidate ids
	// never to suggest. Existing is the configured monitors, for "already monitored".
	Kinds    map[string]bool
	Ignore   []string
	Existing []config.UserMonitor
	Tracked  []string // app_resources.paths already configured
	// Mounts lists filesystems (replaced in tests).
	Mounts func() ([]Mount, error)
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
		Mounts:    hostMounts,
	}
}

// Config kinds, as written in discovery.kinds.
const (
	ConfigServices   = "services"
	ConfigContainers = "containers"
	ConfigPM2        = "pm2"
	ConfigPorts      = "ports"
	ConfigDisks      = "disks"
	ConfigLogs       = "logs"
	ConfigStorage    = "storage"
)

// AllKinds is the default for discovery.kinds, in display order.
var AllKinds = []string{ConfigServices, ConfigContainers, ConfigPM2, ConfigPorts, ConfigDisks, ConfigLogs, ConfigStorage}

// ConfigKind maps a candidate to the discovery.kinds name that controls it.
func ConfigKind(k Kind) string {
	switch k {
	case KindService:
		return ConfigServices
	case KindContainer:
		return ConfigContainers
	case KindPM2:
		return ConfigPM2
	case KindPort:
		return ConfigPorts
	case KindDisk:
		return ConfigDisks
	case KindLog:
		return ConfigLogs
	case KindStorage:
		return ConfigStorage
	}
	return string(k)
}

// Scan states of one kind.
const (
	ScanOK      = "scanned"
	ScanOff     = "off"     // turned off in discovery.kinds
	ScanBlocked = "blocked" // looked, but the agent lacks access
	ScanNone    = "absent"  // the thing is not on this host (no Docker, no pm2): silent
)

// KindStatus says what happened for one kind in a scan.
type KindStatus struct {
	Kind    string `json:"kind"`
	State   string `json:"state"`
	Message string `json:"message,omitempty"` // "Could not look at Docker: the agent has no access."
	Fix     string `json:"fix,omitempty"`     // the exact command
	Why     string `json:"why,omitempty"`     // the trade-off in one line
}

// Result is everything found, sorted for stable output.
type Result struct {
	Candidates []Candidate  `json:"candidates"`
	Kinds      []KindStatus `json:"kinds"`
	Notes      []string     `json:"notes,omitempty"`
}

// Status of a candidate after comparing it with what is configured.
const (
	StatusSuggested = "suggested"
	StatusMonitored = "monitored" // a matching monitor already exists
)

type source struct {
	kind string
	run  func(ctx context.Context) ([]Candidate, error)
}

// Run executes every enabled source. A failing source becomes a KindStatus
// (blocked, with the fix) and never aborts the scan; kinds that are off are
// reported as such and never touched.
func Run(ctx context.Context, e Env) Result {
	if e.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, e.Timeout)
		defer cancel()
	}
	var res Result
	on := func(kind string) bool { return e.Kinds == nil || e.Kinds[kind] }
	has := func(bin string) bool { return e.LookupBin == nil || e.LookupBin(bin) }

	var sources []source
	switch e.GOOS {
	case "linux":
		if has("systemctl") {
			sources = append(sources, source{ConfigServices, func(c context.Context) ([]Candidate, error) { return systemdUnits(c, e) }})
		}
	case "darwin":
		sources = append(sources, source{ConfigServices, func(c context.Context) ([]Candidate, error) { return launchdServices(c, e) }})
	}
	if has("docker") {
		sources = append(sources, source{ConfigContainers, func(c context.Context) ([]Candidate, error) { return dockerContainers(c, e) }})
	}
	if has("pm2") {
		sources = append(sources, source{ConfigPM2, func(c context.Context) ([]Candidate, error) { return pm2Apps(c, e) }})
	}
	sources = append(sources,
		source{ConfigPorts, func(c context.Context) ([]Candidate, error) { return listeningPorts(c, e) }},
		source{ConfigDisks, func(c context.Context) ([]Candidate, error) { return diskMounts(e) }},
		source{ConfigLogs, func(context.Context) ([]Candidate, error) { return logFiles(e), nil }},
	)
	present := map[string]bool{}
	for _, src := range sources {
		present[src.kind] = true
		if !on(src.kind) {
			continue
		}
		cs, err := src.run(ctx)
		res.Candidates = append(res.Candidates, cs...)
		if err != nil {
			res.Notes = append(res.Notes, fmt.Sprintf("%s: %v", src.kind, err))
		}
		res.Kinds = append(res.Kinds, kindStatus(src.kind, err))
	}
	if on(ConfigStorage) {
		cs, notes := storagePaths(ctx, e, res.Candidates)
		res.Candidates = append(res.Candidates, cs...)
		res.Notes = append(res.Notes, notes...)
		res.Kinds = append(res.Kinds, KindStatus{Kind: ConfigStorage, State: ScanOK})
	}
	for _, k := range AllKinds {
		if !on(k) {
			res.Kinds = append(res.Kinds, KindStatus{Kind: k, State: ScanOff})
		} else if !present[k] && k != ConfigStorage {
			res.Kinds = append(res.Kinds, KindStatus{Kind: k, State: ScanNone})
		}
	}

	res.Candidates = markExisting(dedupe(dropIgnored(res.Candidates, e.Ignore)), e.Existing, e.Tracked)
	sort.SliceStable(res.Candidates, func(i, j int) bool {
		a, b := res.Candidates[i], res.Candidates[j]
		if (a.Status == StatusMonitored) != (b.Status == StatusMonitored) {
			return b.Status == StatusMonitored
		}
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

// kindStatus classifies a source error. Permission problems are "blocked" with
// the fix and the trade-off; anything else is a plain failure message.
func kindStatus(kind string, err error) KindStatus {
	st := KindStatus{Kind: kind, State: ScanOK}
	if err == nil {
		return st
	}
	msg := err.Error()
	st.State = ScanBlocked
	switch {
	case kind == ConfigContainers && (strings.Contains(msg, "permission denied") || strings.Contains(msg, "docker group")):
		st.Message = "Could not look at Docker: the agent has no access."
		st.Fix = "sudo usermod -aG docker orion  then restart the agent"
		st.Why = "Docker access is powerful: members can control this host."
	default:
		st.Message = "Could not scan " + kind + ": " + strings.SplitN(msg, "\n", 2)[0]
	}
	return st
}

func dropIgnored(in []Candidate, ignore []string) []Candidate {
	if len(ignore) == 0 {
		return in
	}
	skip := map[string]bool{}
	for _, id := range ignore {
		skip[id] = true
	}
	out := in[:0]
	for _, c := range in {
		if !skip[c.ID] {
			out = append(out, c)
		}
	}
	return out
}

// markExisting flags suggestions that duplicate a configured monitor (or an
// already tracked path). A hand-written entry always wins: it is shown as
// monitored, never suggested a second time.
func markExisting(in []Candidate, existing []config.UserMonitor, tracked []string) []Candidate {
	for i := range in {
		in[i].Status = StatusSuggested
		for _, p := range tracked {
			if in[i].StoragePath != "" && in[i].StoragePath == p {
				in[i].Status, in[i].ManagedIn = StatusMonitored, "file"
			}
		}
		if in[i].Monitor == nil {
			continue
		}
		for _, m := range existing {
			if sameCheck(*in[i].Monitor, m) {
				in[i].Status = StatusMonitored
				in[i].ManagedIn = managedIn(m)
				break
			}
		}
	}
	return in
}

func managedIn(m config.UserMonitor) string {
	switch m.Meta["source"] {
	case "scan":
		return "scan"
	case "console":
		return "console"
	}
	return "file"
}

// sameCheck reports whether two monitors watch the same thing.
func sameCheck(a, b config.UserMonitor) bool {
	if a.Type != b.Type {
		return false
	}
	switch a.Type {
	case config.UserMonitorTypeSystemd:
		return a.Systemd != nil && b.Systemd != nil && a.Systemd.Name == b.Systemd.Name
	case config.UserMonitorTypeDocker:
		return a.Docker != nil && b.Docker != nil && a.Docker.Name == b.Docker.Name
	case config.UserMonitorTypePM2:
		return a.PM2 != nil && b.PM2 != nil && a.PM2.AppName == b.PM2.AppName
	case config.UserMonitorTypeTCP:
		return a.TCP != nil && b.TCP != nil && a.TCP.Port == b.TCP.Port
	case config.UserMonitorTypeDiskUsage:
		return a.DiskUsage != nil && b.DiskUsage != nil && a.DiskUsage.Path == b.DiskUsage.Path
	}
	return a.Name == b.Name
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
