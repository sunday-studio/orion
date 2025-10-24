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

// Agent is the core orchestrator that ties together config, collector, and transport.
type Agent struct {
	cfg       *config.Config
	transport *transport.Client
}

// New initializes a new Agent with all dependencies wired.
func New(cfg *config.Config) *Agent {
	return &Agent{
		cfg:       cfg,
		transport: transport.NewClient(cfg.CoreURL, cfg.Token),
	}
}

// Run starts the agent event loop. It periodically collects metrics and sends them.
func (a *Agent) Run(ctx context.Context) error {
	interval, err := time.ParseDuration(a.cfg.Interval)
	if err != nil {
		return err
	}
	
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	log.Printf("Agent started with interval: %s", a.cfg.Interval)

	done := ctx.Done()
	for {
		select {
		case <-done:
			log.Println("Shutting down agent gracefully...")
			return nil
		default:
			log.Println("Collecting system metrics...")
			data, err := collector.Collect()
			log.Printf("Data collected: %+v", data)
			if err != nil {
				log.Printf("Error collecting data: %v", err)
			} else {
				// report := &transport.SystemReport{
				// 	Hostname:  data.Hostname,
				// 	OS:        data.OS,
				// 	CPUUsage:  data.CPUUsage,
				// 	MemoryUsed: uint64(data.MemUsage * 100), 
				// 	Timestamp: time.Now(),
				// }

				if err := a.transport.SendReport(data); err != nil {
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
		Hostname:  data.Hostname,
		OS:        data.OS,
		CPUUsage:  data.CPUUsage,
		MemoryUsed: uint64(data.MemUsage * 100), // Convert percentage to bytes (simplified)
		Timestamp: time.Now(),
	}

	return a.transport.SendReport(*report)
}

// RunDefault sets up signal handling and runs the agent with a default config.
func RunDefault() error {
	cfg, err := config.Load(config.DefaultPath())
	if err != nil {
		return err
	}

	agent := New(cfg)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	return agent.Run(ctx)
}
