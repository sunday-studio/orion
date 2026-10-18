package cli

import (
	"context"
	"errors"
	"fmt"
	"orion/agent/internal/appres"
	"os"
	"os/user"
	"sort"
	"sync"
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
	logSources atomic.Int64
	obsMu      sync.Mutex
	states     map[string]control.MonitorState
	lastReport time.Time
	topApps    []control.AppUsage
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

// hooks feeds monitor results and system reports into the status view.
func (d *daemonControl) hooks() agent.Hooks {
	return agent.Hooks{
		MonitorResult: func(name, kind, health string, at time.Time) {
			d.obsMu.Lock()
			defer d.obsMu.Unlock()
			if d.states == nil {
				d.states = map[string]control.MonitorState{}
			}
			d.states[name] = control.MonitorState{Name: name, Kind: kind, Health: health, At: at}
		},
		SystemReport: func(at time.Time, apps *appres.Report) {
			d.obsMu.Lock()
			defer d.obsMu.Unlock()
			d.lastReport = at
			d.topApps = topApps(apps, 3)
		},
	}
}

func topApps(r *appres.Report, n int) []control.AppUsage {
	if r == nil {
		return nil
	}
	var out []control.AppUsage
	for _, s := range r.Series {
		if s.CPUPercent != nil && s.Kind != appres.KindPath {
			out = append(out, control.AppUsage{Name: s.Name, CPUPercent: *s.CPUPercent})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CPUPercent > out[j].CPUPercent })
	if len(out) > n {
		out = out[:n]
	}
	return out
}

// update refreshes the values the daemon reports after a config reload.
func (d *daemonControl) update(userConfig *config.UserConfig) {
	d.monitors.Store(int64(len(userConfig.Monitors)))
	keep := map[string]bool{}
	for _, m := range userConfig.Monitors {
		keep[m.Name] = true
	}
	d.obsMu.Lock()
	for name := range d.states {
		if !keep[name] { // removed from config or Console
			delete(d.states, name)
		}
	}
	d.obsMu.Unlock()
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
		QueuePending:    d.store.PendingReports(),
		ConfigPath:      d.configPath,
		Name:            hostnameOrEmpty(),
		Mode:            runMode(),
		LogSources:      int(d.logSources.Load()),
	}
	d.obsMu.Lock()
	out.LastReportAt = d.lastReport
	out.TopApps = append([]control.AppUsage(nil), d.topApps...)
	for _, s := range d.states {
		out.MonitorStates = append(out.MonitorStates, s)
	}
	d.obsMu.Unlock()
	sort.Slice(out.MonitorStates, func(i, j int) bool { return out.MonitorStates[i].Name < out.MonitorStates[j].Name })
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
	dur := time.Duration(req.DurationSeconds) * time.Second
	if req.Enabled && dur <= 0 {
		dur = defaultMaintenanceDuration
	}
	if err := updateCoreMaintenanceModeQuiet(st, req.Enabled, dur); err != nil {
		return err
	}
	var reason *string
	if req.Enabled && req.Reason != "" {
		reason = &req.Reason
	}
	if !req.Enabled {
		return d.store.SetMaintenanceMode(false, nil)
	}
	until := time.Now().Add(dur)
	return d.store.SetMaintenanceWindow(true, reason, &until)
}

func updateCoreMaintenanceModeQuiet(st *config.InternalState, enabled bool, d time.Duration) error {
	if st.AgentID == "" || st.Token == "" || st.CoreURL == "" {
		return nil
	}
	if err := transport.NewClient(st.CoreURL, st.Token).SetMaintenanceMode(st.AgentID, enabled, d); err != nil {
		return fmt.Errorf("update core maintenance mode: %w", err)
	}
	return nil
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

// runMode says how this daemon was started, for the status screen.
func runMode() string {
	name := "unknown"
	if u, err := user.Current(); err == nil {
		name = u.Username
	}
	// A service manager (systemd, launchd) is the parent of a managed daemon.
	// XPC_SERVICE_NAME alone is set for every macOS process, so it is not enough.
	if os.Getppid() == 1 || os.Getenv("XPC_SERVICE_NAME") == "com.orion.agent" {
		return "system service, user " + name
	}
	return "manual run, user " + name
}
