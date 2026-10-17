package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"sync"
	"syscall"

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
		logging.Warnf("Control socket unavailable: %v", err)
		PrintSkip(fmt.Sprintf("control socket unavailable: %v", err))
	} else {
		defer server.Close()
		PrintOK("control socket listening")
		PrintInfo("socket", server.Path())
	}

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

	for {
		PrintStep("registering agent and monitors")
		if err := registerWithRetry(ctx, registration.New(userConfig, opts.ConfigPath, store)); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			PrintError("registration failed; agent cannot continue")
			return NewCommandError("could not register agent and monitors", err)
		}
		PrintOK("registration complete")
		ctrl.update(userConfig)

		instance, err := agent.NewWithStateStore(userConfig, store)
		if err != nil {
			return NewCommandError("could not initialize agent", err)
		}
		PrintStep("starting continuous collection loop")

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
		runErr := instance.Run(runCtx)
		stopRun()

		mu.Lock()
		wasReload := reloaded
		mu.Unlock()
		if wasReload && ctx.Err() == nil && runErr == nil {
			next, _, err := config.Load(opts.ConfigPath)
			if err != nil {
				logging.Errorf("Reload failed, continuing with the previous config: %v", err)
			} else {
				userConfig = next
				PrintOK("configuration reloaded")
			}
			continue
		}
		if runErr != nil {
			logging.Errorf("Agent stopped with error: %v", runErr)
			if transport.IsAuthError(runErr) {
				PrintError("Core rejected Agent credentials; reporting stopped")
				PrintInfo("recovery", "apply an admin-issued replacement token with orion-agent token apply")
			}
			return NewCommandError("agent stopped with an error", runErr)
		}
		PrintOK("agent exited cleanly")
		return nil
	}
}

// registerWithRetry uses the registration service's capped backoff so a Core
// that boots after the agent does not leave it dead. Rejected credentials or
// enrollment tokens are permanent and returned at once.
func registerWithRetry(ctx context.Context, reg *registration.RegistrationService) error {
	return reg.RegisterWithRetry(ctx, registration.DefaultRegistrationBackoff)
}
