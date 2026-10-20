package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"orion/agent/internal/appres"
	"orion/agent/internal/hoststats"
	"orion/agent/internal/logship"
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
	startedAt    time.Time
	configPath   string
	logPath      string
	monitors     atomic.Int64
	logSources   atomic.Int64
	spoolMu      sync.Mutex
	cfgMu        sync.Mutex
	cfg          *config.UserConfig
	scan         *scanner
	spool        *logship.Spool
	obsMu        sync.Mutex
	states       map[string]control.MonitorState
	lastReport   time.Time
	topApps      []control.AppUsage
	reportMillis []int
	host         *hoststats.Sampler
	store        *agentstate.Store
	reload       atomic.Pointer[func() error]
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
	d.cfg = userConfig
	d.host = hoststats.New(hoststats.HostSource("/"), "/")
	d.scan = newScanner(store, d.config)
	d.update(userConfig)
	return d
}

func (d *daemonControl) config() *config.UserConfig {
	d.cfgMu.Lock()
	defer d.cfgMu.Unlock()
	return d.cfg
}

// hooks feeds monitor results and system reports into the status view.
func (d *daemonControl) hooks() agent.Hooks {
	return agent.Hooks{
		MonitorResult: func(name, kind, target, health, detail, value string, at time.Time) {
			d.obsMu.Lock()
			defer d.obsMu.Unlock()
			if d.states == nil {
				d.states = map[string]control.MonitorState{}
			}
			since := at
			if prev, ok := d.states[name]; ok && prev.Health == health {
				since = prev.Since
			}
			d.states[name] = control.MonitorState{Name: name, Kind: kind, Target: target, Health: health, At: at, Since: since, Detail: detail, Value: value}
		},
		SystemReport: func(at time.Time, apps *appres.Report, took time.Duration) {
			d.obsMu.Lock()
			defer d.obsMu.Unlock()
			d.lastReport = at
			d.topApps = topApps(apps, 3)
			d.reportMillis = append(d.reportMillis, int(took.Milliseconds()))
			if len(d.reportMillis) > 12 {
				d.reportMillis = d.reportMillis[len(d.reportMillis)-12:]
			}
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
	d.cfgMu.Lock()
	d.cfg = userConfig
	d.cfgMu.Unlock()
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
	out.ReportMillis = append([]int(nil), d.reportMillis...)
	for _, s := range d.states {
		out.MonitorStates = append(out.MonitorStates, s)
	}
	d.obsMu.Unlock()
	if view, err := d.scan.View(); err == nil {
		out.Suggestions, out.NewItems = pendingFromView(view)
		out.Scanning = view.Scanning
		for _, it := range view.Items {
			if !it.Dismissed {
				out.ThingsFound++
			}
		}
	}
	if snap := d.host.Snapshot(); snap.Ready {
		out.Host = &control.HostStat{CPUPercent: snap.CPUPercent, MemoryPercent: snap.MemoryPercent, DiskPath: snap.DiskPath, DiskPercent: snap.DiskPercent,
			Load1: snap.Load1, NetRxBps: snap.NetRxBps, NetTxBps: snap.NetTxBps}
	}
	if cfg := d.config(); cfg != nil {
		if secs, err := time.ParseDuration(cfg.Interval); err == nil {
			out.ReportIntervalSeconds = int(secs.Seconds())
		}
		kr := &keyResolver{cfg: cfg.Logs.Sources}
		for _, src := range cfg.Logs.Sources {
			if kr.Key(src.Name) == "" {
				out.MissingLogKeys++
			}
		}
	}

	sort.Slice(out.MonitorStates, func(i, j int) bool { return out.MonitorStates[i].Name < out.MonitorStates[j].Name })
	if st.MaintenanceReason != nil {
		out.MaintenanceReason = *st.MaintenanceReason
	}
	out.MaintenanceUntil = st.MaintenanceUntil
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

func (d *daemonControl) setSpool(s *logship.Spool) {
	d.spoolMu.Lock()
	defer d.spoolMu.Unlock()
	d.spool = s
}

// SpoolRecent returns the newest shipped-log entries as NDJSON, so an
// unprivileged `orion-agent logs` works offline through the socket.
func (d *daemonControl) SpoolRecent(n int) ([]byte, error) {
	d.spoolMu.Lock()
	sp := d.spool
	d.spoolMu.Unlock()
	if sp == nil {
		return nil, errors.New("log shipping is not configured (add logs.sources to the config)")
	}
	entries, err := sp.Recent(n)
	if err != nil {
		return nil, err
	}
	var out []byte
	for _, e := range entries {
		b, _ := json.Marshal(e)
		out = append(append(out, b...), '\n')
	}
	return out, nil
}

func (d *daemonControl) Discovery() (control.Discovery, error) { return d.scan.View() }

func (d *daemonControl) Rescan() (control.Discovery, error) {
	return d.scan.Rescan(context.Background())
}

func (d *daemonControl) DiscoveryAction(a control.DiscoveryAction) error { return d.scan.Action(a) }
