// Package doctor runs the agent's self-diagnostics. Every check reports a
// status and, when it fails, the exact command that fixes it. Results stream
// through a callback so the CLI can print them as they complete.
package doctor

import (
	"context"
	"time"

	"orion/agent/internal/config"
)

// Status of one check.
type Status string

const (
	OK   Status = "ok"
	Warn Status = "warn"
	Fail Status = "fail"
	Skip Status = "skip"
)

// SchemaVersion is the version of the --json output.
const SchemaVersion = 4

// Result is the outcome of one check.
type Result struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Status Status `json:"status"`
	// Message is the one-line result shown after the label, e.g. "142 ms".
	Message string `json:"message,omitempty"`
	// Fix is the exact command or step that resolves a warn/fail. A command
	// followed by two spaces and a note is shown with the note dimmed.
	Fix string `json:"fix,omitempty"`
	// Group is the panel the check belongs to (Agent, Core, Discovery, Logs, System).
	Group string `json:"group,omitempty"`
	// Title is the headline: "Config valid" when it passes, "Core is not
	// reachable" when it does not. Label is the stable short name.
	Title string `json:"title,omitempty"`
	// Evidence explains a warn/fail in a few short lines.
	Evidence []string `json:"evidence,omitempty"`
	// TookMS is how long a network check took, when that is worth showing.
	TookMS int64 `json:"took_ms,omitempty"`
}

// Report is the full doctor output.
type Report struct {
	SchemaVersion int      `json:"schema_version"`
	OK            bool     `json:"ok"`
	Checks        []Result `json:"checks"`
}

// Env is everything a check may look at. Tests replace the function fields.
type Env struct {
	ConfigPath string
	StatePath  string
	SocketPath string
	Version    string
	GOOS       string
	Hostname   string

	Config     *config.UserConfig
	ConfigErr  error
	Registered bool
	// TokenProbe asks Core, with the agent token, for a cheap authenticated
	// resource and returns the HTTP status. nil when the token is unavailable.
	TokenProbe func(ctx context.Context) (int, error)

	Service func() (installed, running bool, state string)

	LookupHost func(ctx context.Context, host string) ([]string, error)
	HTTP       HTTPDoer
	DialTLS    func(ctx context.Context, addr, serverName string) (notAfter time.Time, err error)
	Now        func() time.Time
	DiskFreeMB func(path string) (uint64, error)
	CanRead    func(path string) error
	Daemon     DaemonInfo
	// Queue returns the number of items waiting to be sent, and false when unknown.
	Queue func() (int, bool)
	// StateOpen opens the state database read-only and reports why it cannot.
	StateOpen func() error
	// Discovery is the daemon's latest scan, false when the daemon is not reachable.
	Discovery func() (DiscoveryInfo, bool)
	// ReadKey returns the contents of an ingest key file.
	ReadKey func(path string) (string, error)
}

// DiscoveryInfo is the part of the scan Doctor reports on.
type DiscoveryInfo struct {
	ScannedAt time.Time
	Things    int
	Blocked   []BlockedKind
}

// BlockedKind is a kind of thing the agent looked for but could not read.
type BlockedKind struct{ Kind, Message, Fix, Why string }

// DaemonInfo is what the control socket reports, or why it could not.
type DaemonInfo struct {
	Reachable bool
	Err       error
	Version   string
	LastSync  time.Time
	Uptime    time.Duration
	PID       int
}

// Check is a single diagnostic.
type Check struct {
	ID    string
	Group string
	Label string // short row label, e.g. "Core reachable"
	Run   func(ctx context.Context, e *Env) Result
}

// Run executes the checks in order, streaming each result to emit.
func Run(ctx context.Context, e *Env, checks []Check, emit func(Result)) Report {
	return RunWithStart(ctx, e, checks, nil, emit)
}

// RunWithStart is Run plus a hook called before each check starts, so a TTY
// can show a spinner on the one that is running.
func RunWithStart(ctx context.Context, e *Env, checks []Check, start func(Check), emit func(Result)) Report {
	rep := Report{SchemaVersion: SchemaVersion, OK: true}
	for _, c := range checks {
		if start != nil {
			start(c)
		}
		r := c.Run(ctx, e)
		r.ID, r.Label, r.Group = c.ID, c.Label, c.Group
		if r.Title == "" {
			r.Title = c.Label
		}
		if r.Status == Fail {
			rep.OK = false
		}
		rep.Checks = append(rep.Checks, r)
		if emit != nil {
			emit(r)
		}
	}
	return rep
}

func ok(msg string) Result        { return Result{Status: OK, Message: msg} }
func skip(msg string) Result      { return Result{Status: Skip, Message: msg} }
func warn(msg, fix string) Result { return Result{Status: Warn, Message: msg, Fix: fix} }
func fail(msg, fix string) Result { return Result{Status: Fail, Message: msg, Fix: fix} }

// titled names a result's headline and, for a problem, the evidence under it.
func (r Result) titled(title string, evidence ...string) Result {
	r.Title, r.Evidence = title, evidence
	return r
}
func (e *Env) now() time.Time {
	if e.Now != nil {
		return e.Now()
	}
	return time.Now()
}
