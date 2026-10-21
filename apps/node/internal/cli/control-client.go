package cli

import (
	"errors"
	"fmt"
	"time"

	"orion/node/internal/config"
	"orion/node/internal/control"
)

// controlClient returns a client for the daemon socket.
func controlClient(opts *Options) *control.Client {
	return control.NewClient(opts.SocketPath)
}

// socketUsable reports whether an error means "fall back to the direct path"
// rather than a real failure from a reachable daemon.
func socketUsable(err error) bool {
	return !errors.Is(err, control.ErrUnavailable) && !errors.Is(err, control.ErrPermission)
}

// statusFromSocket converts daemon status into the CLI's state view. The token
// is never available through the socket.
func statusFromSocket(st control.Status) *config.InternalState {
	state := &config.InternalState{
		NodeID:           st.NodeID,
		Registered:       st.Registered,
		CoreURL:          st.CoreURL,
		LastSync:         st.LastSync,
		MaintenanceMode:  st.MaintenanceMode,
		MaintenanceUntil: st.MaintenanceUntil,
	}
	if st.MaintenanceReason != "" {
		reason := st.MaintenanceReason
		state.MaintenanceReason = &reason
	}
	return state
}

// tryControlMaintenance toggles maintenance via the daemon. handled is false
// when the socket is not usable and the caller should fall back.
func tryControlMaintenance(opts *Options, enabled bool, reason string, d time.Duration) (handled bool, err error) {
	err = controlClient(opts).SetMaintenance(enabled, reason, d)
	if err == nil {
		return true, nil
	}
	if !socketUsable(err) {
		return false, nil
	}
	return true, NewCommandError("daemon rejected maintenance change", err)
}

// tryControlToken applies a replacement token via the daemon.
func tryControlToken(opts *Options, token string) (handled bool, err error) {
	err = controlClient(opts).ApplyToken(token)
	if err == nil {
		return true, nil
	}
	if !socketUsable(err) {
		return false, nil
	}
	return true, NewCommandError("daemon rejected replacement token", err)
}

func maintenanceViaDaemon(opts *Options, enabled bool, reason string, d time.Duration) (bool, error) {
	status, statusErr := controlClient(opts).Status()
	if statusErr != nil {
		if !socketUsable(statusErr) {
			return false, nil
		}
		return true, NewCommandError("could not read daemon status", statusErr)
	}
	handled, err := tryControlMaintenance(opts, enabled, reason, d)
	if !handled || err != nil {
		return handled, err
	}
	if opts.JSON {
		return true, renderJSON(outputWriter, maintenanceResult{OK: true, Enabled: enabled, Reason: reason, Via: "daemon"})
	}
	if enabled {
		if d <= 0 {
			d = defaultMaintenanceDuration
		}
		printMaintenanceDown(opts, time.Now().Add(d), reason, d, status.Registered)
	} else {
		fmt.Fprint(outputWriter, maintenanceOutcome(theme, false, time.Time{}, time.Now(), status.Registered))
	}
	return true, nil
}

func applyTokenViaDaemon(opts *Options) (bool, error) {
	token, err := replacementTokenFromOptions(opts)
	if err != nil || token == "" {
		return false, nil // let the direct path report the problem
	}
	status, statusErr := controlClient(opts).Status()
	if statusErr != nil {
		if !socketUsable(statusErr) {
			return false, nil
		}
		return true, NewCommandError("could not read daemon status", statusErr)
	}
	handled, err := tryControlToken(opts, token)
	if !handled || err != nil {
		return handled, err
	}
	if opts.JSON {
		return true, renderJSON(outputWriter, map[string]any{"ok": true, "identity": "preserved", "restart_required": true, "via": "daemon"})
	}
	fmt.Fprint(outputWriter, tokenAppliedOutput(status.NodeID, status.Monitors))
	return true, nil
}

type maintenanceResult struct {
	OK      bool   `json:"ok"`
	Enabled bool   `json:"enabled"`
	Reason  string `json:"reason,omitempty"`
	Via     string `json:"via"`
}
