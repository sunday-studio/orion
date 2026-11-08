package selfupdate

import "fmt"

// MaxUnconfirmedStarts is how often the new version may start without staying up before the old one
// is restored. The service manager gives up after five quick restarts, so this must stay below that.
const MaxUnconfirmedStarts = 3

// Outcome is the result of an update that finished on a previous run and still has to be reported.
type Outcome struct {
	RequestID string
	// Status is "applied" or "failed", the words Core uses for a request.
	Status string
	Text   string
}

// Startup runs first thing when the daemon starts. It returns an outcome to report when an update
// just finished, and restart=true when the new version could not start and the old one was restored,
// so the caller must exit and let the service manager start the old binary.
func (e *Env) Startup() (out *Outcome, restart bool, err error) {
	m, err := e.ReadMarker()
	if err != nil || m == nil {
		return nil, false, err
	}
	switch m.State {
	case StateRolledBack:
		// This is the restored binary, back on its old version: say so.
		return &Outcome{RequestID: m.RequestID, Status: "failed", Text: m.Reason}, false, nil
	case StateSwapped:
		if e.Current != m.To {
			// The binary on disk is not the one that was swapped in (someone put the old one back).
			return nil, false, e.ClearMarker()
		}
		m.Starts++
		if m.Starts > MaxUnconfirmedStarts {
			if err := e.Restore(); err != nil {
				return nil, false, fmt.Errorf("the new version cannot start and the old one could not be restored: %w", err)
			}
			m.State = StateRolledBack
			m.Reason = fmt.Sprintf("%s could not start, so %s was restored", m.To, m.From)
			if err := e.writeMarker(*m); err != nil {
				return nil, false, err
			}
			return nil, true, nil
		}
		return nil, false, e.writeMarker(*m)
	}
	return nil, false, e.ClearMarker()
}

// Confirm is called once the new version has stayed up. It returns the outcome to report, or nil when
// there is nothing pending.
func (e *Env) Confirm() (*Outcome, error) {
	m, err := e.ReadMarker()
	if err != nil || m == nil || m.State != StateSwapped || e.Current != m.To {
		return nil, err
	}
	return &Outcome{RequestID: m.RequestID, Status: "applied", Text: fmt.Sprintf("Updated from %s to %s", m.From, m.To)}, nil
}
