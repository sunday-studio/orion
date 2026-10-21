package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sync"
	"syscall"
	"time"

	agent "orion/agent/internal"
	"orion/agent/internal/config"
	"orion/agent/internal/control"
	"orion/agent/internal/logging"
	"orion/agent/internal/registration"
	agentstate "orion/agent/internal/state"
	"orion/agent/internal/transport"
)

// runDaemon is the long-running mode: register (retrying while Core is down),
// run the collection loop, and rebuild it in place on SIGHUP or a control
// socket reload instead of exiting.
func runDaemon(ctx context.Context, opts *Options, userConfig *config.UserConfig, store *agentstate.Store) error {
	// A monitoring daemon should stay small: collect garbage earlier and cap the
	// heap softly (budget: RSS under 30 MB with 20 monitors).
	if os.Getenv("GOMAXPROCS") == "" {
		runtime.GOMAXPROCS(min(runtime.NumCPU(), 2)) // per-P caches cost RSS; the agent is I/O bound
	}
	debug.SetGCPercent(40)
	debug.SetMemoryLimit(48 << 20)

	ctrl := newDaemonControl(opts, userConfig, store)
	server := control.NewServer(opts.SocketPath, ctrl)
	if err := server.Start(); err != nil {
		// The socket is a convenience for operators; never block reporting on it.
		fmt.Fprint(outputWriter, opLines(theme, []opStep{{Mark: "warn", Name: "Control socket unavailable", Detail: firstLineOf(err.Error())}}))
	} else {
		defer server.Close()
		fmt.Fprint(outputWriter, opLines(theme, []opStep{{Mark: "ok", Name: "Control socket listening", Detail: server.Path()}}))
	}

	ctrl.scan.onScan = newDiscoveryReporter(store, ctrl.config).Report
	go ctrl.scan.loop(ctx)
	go ctrl.host.Run(ctx, 5*time.Second)

	reloadCh := make(chan struct{}, 1)
	requestReload := func() error {
		// Validate first so a bad edit is rejected, not half-applied.
		if _, _, err := config.Load(opts.ConfigPath); err != nil {
			return fmt.Errorf("new config is invalid, keeping the running one: %w", err)
		}
		select {
		case reloadCh <- struct{}{}:
		default:
		}
		return nil
	}
	fn := requestReload
	ctrl.reload.Store(&fn)

	go newRequestApplier(store, opts.ConfigPath, ctrl.scan, requestReload).run(ctx, consoleRequestPollInterval)

	hup := make(chan os.Signal, 1)
	signal.Notify(hup, syscall.SIGHUP)
	defer signal.Stop(hup)
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-hup:
				logging.Infof("SIGHUP received; reloading configuration")
				if err := requestReload(); err != nil {
					logging.Errorf("Reload rejected: %v", err)
				}
			}
		}
	}()

	fileConfig := userConfig // from disk; Console checks are layered on top each cycle
	remote := &remoteMonitors{store: store, path: filepath.Join(filepath.Dir(opts.StatePath), "remote-monitors.json")}
	remote.load()
	// An enrolled agent can sync before registering, so the first registration
	// already includes Console checks and never unregisters them. If Core is
	// unreachable the persisted set from the last run stands in.
	if _, err := remote.sync(); err != nil {
		logging.Warnf("Monitor definition sync failed, using the last known set: %v", err)
	}
	go remote.poll(ctx, remoteMonitorPollInterval, func() { _ = requestReloadNoValidate(reloadCh) })

	announced := false
	for {
		userConfig = effective(fileConfig, remote.current())
		if err := registerWithRetry(ctx, registration.New(userConfig, opts.ConfigPath, store)); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return NewCommandError("could not register agent and monitors", err)
		}
		// Now that the agent has an identity, pick up checks configured in the Console.
		if changed, err := remote.sync(); err != nil {
			logging.Warnf("Monitor definition sync failed: %v", err)
		} else if changed {
			userConfig = effective(fileConfig, remote.current())
			if err := registerWithRetry(ctx, registration.New(userConfig, opts.ConfigPath, store)); err != nil && ctx.Err() == nil {
				// Not fatal: the poller retries and the next reload registers them.
				logging.Warnf("Could not register Console monitors yet: %v", err)
			}
		}
		fmt.Fprint(outputWriter, opLines(theme, []opStep{{Mark: "ok", Name: "Registered", Detail: ""}}))
		if !announced {
			// A foreground agent is silent from here on; say so, or it looks hung.
			announced = true
			fmt.Fprint(outputWriter, "\n   "+theme.Muted("Running and reporting. Ctrl+C stops it. In another terminal: orion-agent status")+"\n")
		}
		ctrl.update(userConfig)

		instance, err := agent.NewWithStateStore(userConfig, store)
		if err != nil {
			return NewCommandError("could not initialize agent", err)
		}
		instance.SetHooks(ctrl.hooks())

		runCtx, stopRun := context.WithCancel(ctx)
		var reloaded bool
		var mu sync.Mutex
		go func() {
			select {
			case <-reloadCh:
				mu.Lock()
				reloaded = true
				mu.Unlock()
				stopRun()
			case <-runCtx.Done():
			}
		}()
		waitLogs := startLogShipper(runCtx, opts, userConfig, store, ctrl)
		runErr := instance.Run(runCtx)
		stopRun()
		waitLogs()

		mu.Lock()
		wasReload := reloaded
		mu.Unlock()
		if wasReload && ctx.Err() == nil && runErr == nil {
			next, _, err := config.Load(opts.ConfigPath)
			if err != nil {
				logging.Errorf("Reload failed, continuing with the previous config: %v", err)
			} else {
				fileConfig = next
				fmt.Fprint(outputWriter, opLines(theme, []opStep{{Mark: "ok", Name: "Configuration reloaded", Detail: ""}}))
			}
			continue
		}
		if runErr != nil {
			logging.Errorf("Agent stopped with error: %v", runErr)
			if transport.IsAuthError(runErr) {
				return credentialsRejected()
			}
			return NewCommandError("agent stopped with an error", runErr)
		}
		fmt.Fprint(outputWriter, opLines(theme, []opStep{{Mark: "ok", Name: "Agent stopped", Detail: "exited cleanly"}}))
		return nil
	}
}

// registerWithRetry uses the registration service's capped backoff so a Core
// that boots after the agent does not leave it dead. Rejected credentials or
// enrollment tokens are permanent and returned at once.
func registerWithRetry(ctx context.Context, reg *registration.RegistrationService) error {
	return reg.RegisterWithRetry(ctx, registration.DefaultRegistrationBackoff)
}

// requestReloadNoValidate signals the loop to rebuild (used when Console checks change).
func requestReloadNoValidate(ch chan struct{}) error {
	select {
	case ch <- struct{}{}:
	default:
	}
	return nil
}
