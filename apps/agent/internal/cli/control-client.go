package cli

import (
	"errors"

	"orion/agent/internal/config"
	"orion/agent/internal/control"
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
		AgentID:         st.AgentID,
		Registered:      st.Registered,
		CoreURL:         st.CoreURL,
		LastSync:        st.LastSync,
		MaintenanceMode: st.MaintenanceMode,
	}
	if st.MaintenanceReason != "" {
		reason := st.MaintenanceReason
		state.MaintenanceReason = &reason
	}
	return state
}

// tryControlMaintenance toggles maintenance via the daemon. handled is false
// when the socket is not usable and the caller should fall back.
func tryControlMaintenance(opts *Options, enabled bool, reason string) (handled bool, err error) {
	err = controlClient(opts).SetMaintenance(enabled, reason)
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

func maintenanceViaDaemon(opts *Options, enabled bool, reason string) (bool, error) {
	handled, err := tryControlMaintenance(opts, enabled, reason)
	if !handled || err != nil {
		return handled, err
	}
	PrintHeader("maintenance")
	if enabled {
		PrintOK("maintenance mode enabled (via daemon)")
		if reason != "" {
			PrintInfo("maintenance reason", reason)
		}
	} else {
		PrintOK("maintenance mode disabled (via daemon)")
	}
	return true, nil
}

func applyTokenViaDaemon(opts *Options) (bool, error) {
	token, err := replacementTokenFromOptions(opts)
	if err != nil || token == "" {
		return false, nil // let the direct path report the problem
	}
	handled, err := tryControlToken(opts, token)
	if !handled || err != nil {
		return handled, err
	}
	PrintHeader("token apply")
	PrintOK("replacement token applied (via daemon)")
	PrintInfo("identity", "preserved")
	PrintInfo("next", "orion-agent restart")
	return true, nil
}
