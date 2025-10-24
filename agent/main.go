package main

import (
    "context"
    "flag"
    "os"
    "os/signal"
    "syscall"
    "time"

    agent "orion/agent/internal"
    "orion/agent/internal/config"
    "orion/agent/internal/logging"
    "orion/agent/internal/registration"
)

var (
    configPath = flag.String("config", "/etc/orion/config.yaml", "config.yaml")
    once       = flag.Bool("once", false, "Run once and exit (for debugging)")
)

func main() {
    flag.Parse()
    logging.Infof("Starting Orion Agent...")

    // Load configuration
    cfg, err := config.Load(*configPath)
    if err != nil {
        logging.Fatalf("Failed to load config: %v", err)
    }

    // Handle agent registration if needed
    regService := registration.New(cfg, *configPath)
    if err := regService.RegisterIfNeeded(); err != nil {
        logging.Fatalf("Failed to register agent: %v", err)
    }

    // Reload config to get updated agent_id and token
    cfg, err = config.Load(*configPath)
    if err != nil {
        logging.Fatalf("Failed to reload config after registration: %v", err)
    }

    // Create agent instance
    a := agent.New(cfg)

    // Context for graceful shutdown
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Capture OS signals
    sigs := make(chan os.Signal, 1)
    signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

    // Run in a goroutine so we can catch signals
    go func() {
        if *once {
            if err := a.RunOnce(ctx); err != nil {
                logging.Errorf("Agent run failed: %v", err)
            }
            cancel()
            return
        }

        if err := a.Run(ctx); err != nil {
            logging.Errorf("Agent stopped with error: %v", err)
        }
        cancel()
    }()

    // Wait for shutdown signal
    <-sigs
    logging.Infof("Received shutdown signal, stopping agent...")
    cancel()
    time.Sleep(1 * time.Second)
    logging.Infof("Orion Agent exited cleanly")
}
