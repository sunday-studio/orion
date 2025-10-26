package agent

import (
	"context"
	"log"
	"os/signal"
	"syscall"
	"time"

	"orion/agent/internal/collector"
	"orion/agent/internal/config"
	"orion/agent/internal/transport"
)

type Agent struct {
	userConfig    *config.UserConfig
	internalState *config.InternalState
	transport     *transport.Client
}

func New(userConfig *config.UserConfig, internalState *config.InternalState) *Agent {
	return &Agent{
		userConfig:    userConfig,
		transport:     transport.NewClient(userConfig.CoreURL, internalState.Token),
		internalState: internalState,
	}
}

// Run starts the agent event loop. It periodically collects metrics and sends them.
func (a *Agent) Run(ctx context.Context) error {
	interval, err := time.ParseDuration(a.userConfig.Interval)
	if err != nil {
		return err
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Printf("Agent started with interval: %s", a.userConfig.Interval)

	done := ctx.Done()
	for {
		select {
		case <-done:
			log.Println("Shutting down agent gracefully...")
			return nil
		default:
			log.Println("Collecting system metrics...")
			data, err := collector.Collect()
			if err != nil {
				log.Printf("Error collecting data: %v", err)
			} else {
				report := &transport.SystemReport{
					Hostname:   data.Hostname,
					OS:         data.OS,
					CPUUsage:   data.CPUUsage,
					MemoryUsed: uint64(data.MemUsage * 100),
					Timestamp:  time.Now(),
				}

				if err := a.transport.SendReport(*report, a.internalState.AgentID); err != nil {
					log.Printf("Error sending data: %v", err)
				}
			}
			select {
			case <-done:
				log.Println("Shutting down agent gracefully...")
				return nil
			case <-ticker.C:
				// Continue loop
			}
		}
	}
}

// RunOnce runs the agent once and exits (useful for debugging).
func (a *Agent) RunOnce(ctx context.Context) error {
	data, err := collector.Collect()
	if err != nil {
		return err
	}

	// Convert SystemMetrics to SystemReport
	report := &transport.SystemReport{
		Hostname:   data.Hostname,
		OS:         data.OS,
		CPUUsage:   data.CPUUsage,
		MemoryUsed: uint64(data.MemUsage * 100),
		Timestamp:  time.Now(),
	}

	return a.transport.SendReport(*report, a.internalState.AgentID)
}

// RunDefault sets up signal handling and runs the agent with a default config.
func RunDefault() error {
	userConfig, err := config.LoadUserConfig(config.DefaultPath())
	internalState, err := config.LoadInternalState(config.DefaultPath())
	if err != nil {
		return err
	}

	agent := New(userConfig, internalState)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	return agent.Run(ctx)
}
