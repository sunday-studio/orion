package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	agent "orion/agent/internal"
	"orion/agent/internal/config"
	"orion/agent/internal/logging"
	"orion/agent/internal/registration"
)

var (
	// userConfigPath    = flag.String("config", "/etc/orion/config.yaml", "config.yaml")
	// internalStatePath = flag.String("state", "/etc/orion/state.yaml", "state.yaml")
	userConfigPath    = flag.String("config", "config.yaml", config.DefaultPath())
	internalStatePath = flag.String("state", "state.yaml", config.DefaultPath())
	once              = flag.Bool("once", false, "Run once and exit (for debugging)")
)

func main() {
	flag.Parse()
	logging.Infof("Starting Orion Agent...")

	userConfig, err := config.LoadUserConfig(*userConfigPath)
	if err != nil {
		logging.Fatalf("Failed to load user config: %v", err)
	}

	internalState, err := config.LoadInternalState(*internalStatePath)
	if err != nil {
		logging.Fatalf("Failed to load internal state: %v", err)
	}

	registrationService := registration.New(userConfig, *userConfigPath, internalState, *internalStatePath)
	if err := registrationService.RegisterAgentIfNeeded(); err != nil {
		logging.Fatalf("Failed to register agent & monitors: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	agent := agent.New(userConfig, internalState)

	go func() {
		<-sigs
		logging.Infof("Received shutdown signal, stopping agent...")
		cancel()
	}()

	if err := agent.Run(ctx); err != nil {
		logging.Errorf("Agent stopped with error: %v", err)
	}

	logging.Infof("Orion Agent exited cleanly")
}
