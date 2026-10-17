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
const SchemaVersion = 2

// Result is the outcome of one check.
type Result struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status Status `json:"status"`
	Detail string `json:"detail,omitempty"`
	// Fix is the exact command or step that resolves a warn/fail.
	Fix string `json:"fix,omitempty"`
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
}

// DaemonInfo is what the control socket reports, or why it could not.
type DaemonInfo struct {
	Reachable bool
	Err       error
	Version   string
	LastSync  time.Time
}

// Check is a single diagnostic.
type Check struct {
	ID    string
	Title string
	Run   func(ctx context.Context, e *Env) Result
}

// Run executes the checks in order, streaming each result to emit.
func Run(ctx context.Context, e *Env, checks []Check, emit func(Result)) Report {
	rep := Report{SchemaVersion: SchemaVersion, OK: true}
	for _, c := range checks {
		r := c.Run(ctx, e)
		r.ID, r.Title = c.ID, c.Title
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

func ok(detail string) Result        { return Result{Status: OK, Detail: detail} }
func skip(detail string) Result      { return Result{Status: Skip, Detail: detail} }
func warn(detail, fix string) Result { return Result{Status: Warn, Detail: detail, Fix: fix} }
func fail(detail, fix string) Result { return Result{Status: Fail, Detail: detail, Fix: fix} }
func (e *Env) now() time.Time {
	if e.Now != nil {
		return e.Now()
	}
	return time.Now()
}
