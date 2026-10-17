package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"time"

	agent "orion/agent/internal"
	"orion/agent/internal/config"
	"orion/agent/internal/control"
	agentstate "orion/agent/internal/state"
	"orion/agent/internal/transport"
)

// daemonControl serves the local control socket from inside the daemon.
type daemonControl struct {
	startedAt  time.Time
	configPath string
	logPath    string
	monitors   atomic.Int64
	store      *agentstate.Store
	reload     atomic.Pointer[func() error]
}

func newDaemonControl(opts *Options, userConfig *config.UserConfig, store *agentstate.Store) *daemonControl {
	logPath := ""
	if userConfig.Logging.Path != "" {
		logPath = userConfig.Logging.Path
	}
	d := &daemonControl{
		startedAt:  time.Now(),
		configPath: opts.ConfigPath,
		logPath:    logPath,
		store:      store,
	}
	d.update(userConfig)
	return d
}

// update refreshes the values the daemon reports after a config reload.
func (d *daemonControl) update(userConfig *config.UserConfig) {
	d.monitors.Store(int64(len(userConfig.Monitors)))
}

func (d *daemonControl) Status() (control.Status, error) {
	st, err := d.store.Get()
	if err != nil {
		return control.Status{}, err
	}
	out := control.Status{
		Version:         agent.Version,
		PID:             os.Getpid(),
		StartedAt:       d.startedAt,
		UptimeSeconds:   int64(time.Since(d.startedAt).Seconds()),
		Registered:      st.IsRegistered(),
		AgentID:         st.AgentID,
		CoreURL:         st.CoreURL,
		LastSync:        st.LastSync,
		MaintenanceMode: st.MaintenanceMode,
		Monitors:        int(d.monitors.Load()),
		ConfigPath:      d.configPath,
	}
	if st.MaintenanceReason != nil {
		out.MaintenanceReason = *st.MaintenanceReason
	}
	return out, nil
}

func (d *daemonControl) SetMaintenance(req control.MaintenanceRequest) error {
	st, err := d.store.Get()
	if err != nil {
		return err
	}
	if st.AgentID != "" && st.Token != "" && st.CoreURL != "" {
		client := transport.NewClient(st.CoreURL, st.Token)
		if err := client.SetMaintenanceMode(st.AgentID, req.Enabled); err != nil {
			return fmt.Errorf("update core maintenance mode: %w", err)
		}
	}
	var reason *string
	if req.Enabled && req.Reason != "" {
		reason = &req.Reason
	}
	return d.store.SetMaintenanceMode(req.Enabled, reason)
}

func (d *daemonControl) ApplyToken(token string) error {
	st, err := d.store.Get()
	if err != nil {
		return err
	}
	if !st.IsRegistered() || st.AgentID == "" || st.CoreURL == "" {
		return errors.New("agent is not registered; run setup instead")
	}
	return d.store.ApplyReplacementToken(token)
}

func (d *daemonControl) LogPath() string { return d.logPath }

func (d *daemonControl) Reload() error {
	if fn := d.reload.Load(); fn != nil {
		return (*fn)()
	}
	return errors.New("hot config reload is not available yet; restart the service")
}

// TokenStatus probes Core with the agent token; the token stays in the daemon.
func (d *daemonControl) TokenStatus() (int, error) {
	st, err := d.store.Get()
	if err != nil {
		return 0, err
	}
	return probeToken(context.Background(), st.CoreURL, st.AgentID, st.Token)
}
