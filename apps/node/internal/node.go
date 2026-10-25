package node

import (
	"context"
	"sync"
	"time"

	"orion/node/internal/appres"
	"orion/node/internal/config"
	"orion/node/internal/logging"
	"orion/node/internal/state"
	"orion/node/internal/transport"
)

type Node struct {
	userConfig    *config.UserConfig
	internalState *config.InternalState
	stateStore    *state.Store
	stateMu       sync.Mutex
	transport     *transport.Client
	retryQueue    *RetryQueue
	sampler       *appres.Sampler
	hooks         Hooks
}

// Hooks lets the daemon observe results without the node knowing about the
// control socket: the status screen shows monitor states and top apps from them.
type Hooks struct {
	// MonitorResult reports a check outcome: detail is the failure reason or
	// error, value a short reading such as "91%" when the check has one.
	MonitorResult func(name, kind, target, health, detail, value string, at time.Time)
	// SystemReport reports a sent system report and how long sending took.
	SystemReport func(at time.Time, apps *appres.Report, took time.Duration)
}

// SetHooks installs result observers; call before Run.
func (a *Node) SetHooks(h Hooks) { a.hooks = h }

func New(userConfig *config.UserConfig, stateStore *state.Store, internalState *config.InternalState) *Node {
	client := transport.NewClient(userConfig.CoreURL, internalState.Token)
	client.SetCompression(userConfig.Transport.Compression)
	return &Node{
		userConfig:    userConfig,
		transport:     client,
		internalState: internalState,
		stateStore:    stateStore,
		retryQueue:    NewRetryQueue(100),
		sampler:       newSampler(userConfig),
	}
}

func NewWithStateStore(userConfig *config.UserConfig, stateStore *state.Store) (*Node, error) {
	internalState, err := stateStore.Get()
	if err != nil {
		return nil, err
	}
	logging.Debugf("node state loaded: node_id=%s registered=%t monitors=%d maintenance=%t", internalState.NodeID, internalState.IsRegistered(), len(internalState.Monitors), internalState.MaintenanceMode)
	return New(userConfig, stateStore, internalState), nil
}

func (a *Node) Run(ctx context.Context) error {
	logging.Debugf("starting node runtime: interval=%s monitors=%d geo_location=%t", a.userConfig.Interval, len(a.userConfig.Monitors), a.userConfig.GeoLocation)
	runtimeCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	fatalErrs := make(chan error, 1)

	// Check if node is in maintenance mode
	if inMaintenance, maintenanceReason := a.maintenanceStatus(); inMaintenance {
		reason := "No reason provided"
		if maintenanceReason != "" {
			reason = maintenanceReason
		}
		logging.Infof("Node is in maintenance mode. Reporting workers will pause until maintenance clears. Reason: %s", reason)
	}
	if err := a.flushDurableSpool(runtimeCtx); err != nil {
		if transport.IsAuthError(err) {
			return err
		}
		logging.Warnf("Initial durable report spool flush failed: %v", err)
	}

	var workers sync.WaitGroup

	// Start system metrics worker
	workers.Add(1)
	go func() {
		defer workers.Done()
		a.startSystemMetricsWorker(runtimeCtx, fatalErrs)
	}()
	workers.Add(1)
	go func() {
		defer workers.Done()
		a.startRetryQueueWorker(runtimeCtx, fatalErrs)
	}()

	// start one worker per monitor
	for _, monitor := range a.userConfig.Monitors {
		logging.Debugf("starting worker goroutine: monitor=%s type=%s interval=%s", monitor.Name, monitor.Type, monitor.Interval)
		workers.Add(1)
		go func(monitor config.UserMonitor) {
			defer workers.Done()
			a.startMonitorWorker(runtimeCtx, monitor, fatalErrs)
		}(monitor)
	}
	var runErr error
	select {
	case <-ctx.Done():
	case runErr = <-fatalErrs:
		cancel()
	}
	workers.Wait()
	if transport.IsAuthError(runErr) {
		logging.Errorf("Core rejected Node credentials; stopping reporting until a replacement token is applied")
	} else {
		// Bounded final flush: shutdown must not block on a slow Core.
		flushCtx, flushCancel := context.WithTimeout(context.Background(), shutdownFlushTimeout)
		defer flushCancel()
		if err := a.flushDurableSpool(flushCtx); err != nil {
			logging.Warnf("Durable report spool flush during shutdown failed: %v", err)
		}
		if err := a.retryQueue.Flush(flushCtx); err != nil {
			logging.Warnf("Retry queue flush during shutdown failed: %v", err)
		}
	}

	logging.Infof("Node runtime stopped")
	return runErr
}

// RunOnce runs the node once (collects and sends all metrics, then exits)
func (a *Node) RunOnce(ctx context.Context) error {
	logging.Infof("Running node once (single collection cycle)")
	logging.Debugf("run once started: monitors=%d", len(a.userConfig.Monitors))
	if err := a.flushDurableSpool(ctx); err != nil {
		if transport.IsAuthError(err) {
			return err
		}
		logging.Warnf("Durable report spool flush failed before run once: %v", err)
	}

	// Run system metrics once
	if err := a.runSystemMetrics(ctx); err != nil {
		logging.Errorf("System metrics error: %v", err)
		if transport.IsAuthError(err) {
			return err
		}
	}

	// Run all monitors once
	for _, monitor := range a.userConfig.Monitors {
		logging.Debugf("run once collecting monitor: name=%s type=%s", monitor.Name, monitor.Type)
		internalMonitor, err := a.stateStore.GetMonitorByName(monitor.Name)
		if err != nil {
			return err
		}
		if internalMonitor == nil {
			logging.Warnf("Monitor not found in internal state: %s", monitor.Name)
			continue
		}

		if err := a.runMonitorMetrics(ctx, *internalMonitor, monitor); err != nil {
			logging.Errorf("Monitor metrics error for %s: %v", monitor.Name, err)
			if transport.IsAuthError(err) {
				return err
			}
		}
	}

	logging.Infof("Single run completed")
	return nil
}

// System Metrics Worker
func (a *Node) startSystemMetricsWorker(ctx context.Context, fatalErrs chan<- error) {
	interval := intervalOrDefault("system", a.userConfig.Interval)

	if err := a.runSystemMetrics(ctx); err != nil {
		logging.Errorf("System metrics error: %v", err)
		if transport.IsAuthError(err) {
			reportFatalError(fatalErrs, err)
			return
		}
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	logging.Debugf("system metrics worker interval set: interval=%s", interval)

	for {
		select {
		case <-ctx.Done():
			logging.Infof("System metrics worker stopped")
			return
		case <-ticker.C:
			if err := a.runSystemMetrics(ctx); err != nil {
				logging.Errorf("System metrics error: %v", err)
				if transport.IsAuthError(err) {
					reportFatalError(fatalErrs, err)
					return
				}
			}
		}
	}
}

func (a *Node) startRetryQueueWorker(ctx context.Context, fatalErrs chan<- error) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	logging.Debugf("retry queue worker started: interval=30s")

	for {
		select {
		case <-ctx.Done():
			logging.Infof("Retry queue worker stopped")
			return
		case <-ticker.C:
			if err := a.flushDurableSpool(ctx); err != nil {
				logging.Errorf("Durable report spool flush failed: %v", err)
				if transport.IsAuthError(err) {
					reportFatalError(fatalErrs, err)
					return
				}
			}
			if a.retryQueue.Len() > 0 {
				logging.Infof("Flushing retry queue (%d pending)", a.retryQueue.Len())
				if err := a.retryQueue.Flush(ctx); err != nil {
					logging.Errorf("Retry queue flush failed: %v", err)
					if transport.IsAuthError(err) {
						reportFatalError(fatalErrs, err)
						return
					}
				}
			}
		}
	}
}

// Start Monitor Worker
func (a *Node) startMonitorWorker(ctx context.Context, monitor config.UserMonitor, fatalErrs chan<- error) {
	logging.Infof("Starting monitor worker for %s...", monitor.Name)

	internalMonitor := a.waitForMonitorState(ctx, monitor.Name)
	if internalMonitor == nil {
		return // shutting down
	}
	interval := intervalOrDefault(monitor.Name, monitor.Interval)

	if err := a.runMonitorMetrics(ctx, *internalMonitor, monitor); err != nil {
		logging.Errorf("Monitor metrics error: %v", err)
		if transport.IsAuthError(err) {
			reportFatalError(fatalErrs, err)
			return
		}
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	logging.Debugf("monitor worker interval set: monitor=%s interval=%s", monitor.Name, interval)

	for {
		select {
		case <-ctx.Done():
			logging.Infof("Monitor worker stopped")
			return
		case <-ticker.C:
			if err := a.runMonitorMetrics(ctx, *internalMonitor, monitor); err != nil {
				logging.Errorf("Monitor metrics error: %v", err)
				if transport.IsAuthError(err) {
					reportFatalError(fatalErrs, err)
					return
				}
			}

		}
	}
}

const (
	defaultWorkerInterval   = 60 * time.Second
	monitorStateRetryPeriod = 30 * time.Second
)

// intervalOrDefault parses a reporting interval. Config validation rejects bad
// intervals at load, so a failure here is unexpected; reporting at the default
// rate is better than a worker that silently never reports.
func intervalOrDefault(name string, value string) time.Duration {
	interval, err := time.ParseDuration(value)
	if err != nil || interval <= 0 {
		logging.Errorf("Invalid interval %q for %s (%v); using %s", value, name, err, defaultWorkerInterval)
		return defaultWorkerInterval
	}
	return interval
}

// waitForMonitorState returns the monitor's registered state, retrying until it
// exists. A monitor that is not (yet) registered used to make its worker exit
// silently, leaving Core to find out through staleness; now it keeps trying
// and logs the problem loudly. It returns nil only when ctx is cancelled.
func (a *Node) waitForMonitorState(ctx context.Context, name string) *config.InternalStateMonitor {
	for {
		internalMonitor, err := a.stateStore.GetMonitorByName(name)
		switch {
		case err != nil:
			logging.Errorf("Failed to load monitor state for %s: %v (retrying in %s)", name, err, monitorStateRetryPeriod)
		case internalMonitor == nil:
			logging.Errorf("Monitor %s is not registered with Core yet (retrying in %s); restart the node if this persists", name, monitorStateRetryPeriod)
		default:
			return internalMonitor
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(monitorStateRetryPeriod):
		}
	}
}

func reportFatalError(fatalErrs chan<- error, err error) {
	select {
	case fatalErrs <- err:
	default:
	}
}

// nodeID returns the current node identity under the state lock.
func (a *Node) nodeID() string {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	return a.internalState.NodeID
}

func (a *Node) monitorsSnapshot() []config.InternalStateMonitor {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	return append([]config.InternalStateMonitor(nil), a.internalState.Monitors...)
}

func (a *Node) maintenanceReason() string {
	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	if a.internalState.MaintenanceReason != nil {
		return *a.internalState.MaintenanceReason
	}
	return "No reason provided"
}

func (a *Node) isInMaintenanceMode() bool {
	inMaintenance, _ := a.maintenanceStatus()
	return inMaintenance
}

func (a *Node) maintenanceStatus() (bool, string) {
	a.refreshInternalState()

	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	reason := ""
	if a.internalState.MaintenanceReason != nil {
		reason = *a.internalState.MaintenanceReason
	}
	return a.internalState.MaintenanceMode, reason
}

func (a *Node) refreshInternalState() {
	latestState, err := a.stateStore.Get()
	if err != nil {
		logging.Warnf("Failed to refresh internal state: %v", err)
		return
	}

	a.stateMu.Lock()
	defer a.stateMu.Unlock()
	a.internalState = latestState
	a.transport.SetAuthToken(latestState.Token)
	logging.Debugf("internal state refreshed: registered=%t node_id=%s monitors=%d maintenance=%t", latestState.IsRegistered(), latestState.NodeID, len(latestState.Monitors), latestState.MaintenanceMode)
}

const shutdownFlushTimeout = 2 * time.Second

func newSampler(cfg *config.UserConfig) *appres.Sampler {
	if !cfg.AppResources.IsEnabled() {
		return nil
	}
	return appres.NewSampler(appres.Options{TopN: cfg.AppResources.TopN, Paths: cfg.AppResources.Paths, DockerSock: cfg.AppResources.DockerSocket, NoDocker: !cfg.AppResources.DockerEnabled()})
}
