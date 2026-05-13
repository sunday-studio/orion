package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	agent "orion/agent/internal"
	"orion/agent/internal/cli"
	"orion/agent/internal/config"
	"orion/agent/internal/logging"
	"orion/agent/internal/registration"
)

var (
	userConfigPath    = flag.String("config", "config.yaml", config.DefaultPath())
	internalStatePath = flag.String("state", "state.yaml", config.DefaultPath())
	once              = flag.Bool("once", false, "Run once and exit (for debugging)")
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	os.Args = os.Args[1:] // Remove command from args for flag parsing

	switch command {
	case "start":
		handleStart()
	case "stop":
		handleStop()
	case "status":
		handleStatus()
	case "restart":
		handleRestart()
	case "run":
		handleRun()
	case "maintenance":
		cli.HandleMaintenance(userConfigPath, internalStatePath)
	case "config":
		cli.HandleConfig(userConfigPath)
	default:
		fmt.Printf("Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Orion Agent CLI")
	fmt.Println("Usage: orion-agent <command> [options]")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  start         Start the agent service")
	fmt.Println("  stop          Stop the agent service")
	fmt.Println("  status        Show agent service status")
	fmt.Println("  restart       Restart the agent service")
	fmt.Println("  run           Run the agent (for systemd/launchd)")
	fmt.Println("  maintenance   Manage maintenance mode")
	fmt.Println("                -up      Exit maintenance mode")
	fmt.Println("                -down    Enter maintenance mode [reason]")
	fmt.Println("  config        Manage configuration")
	fmt.Println("                validate  Validate config file")
	fmt.Println("                diff     Show config diff")
	fmt.Println("")
	fmt.Println("Options:")
	fmt.Println("  -config   Path to config file (default: config.yaml)")
	fmt.Println("  -state    Path to state file (default: state.yaml)")
	fmt.Println("  -once     Run once and exit (for debugging)")
}

func handleStart() {
	if err := cli.StartService(); err != nil {
		logging.Fatalf("Failed to start service: %v", err)
	}
	fmt.Println("Agent service started")
}

func handleStop() {
	if err := cli.StopService(); err != nil {
		logging.Fatalf("Failed to stop service: %v", err)
	}
	fmt.Println("Agent service stopped")
}

func handleStatus() {
	flag.Parse()
	running, status, err := cli.GetServiceStatus()
	if err != nil {
		logging.Fatalf("Failed to get service status: %v", err)
	}

	fmt.Printf("Service manager: %s\n", cli.DetectServiceManager())
	if running {
		fmt.Printf("Agent service: %s\n", status)
	} else {
		fmt.Printf("Agent service: %s\n", status)
	}

	internalState, err := config.LoadInternalState(*internalStatePath)
	if err != nil {
		fmt.Printf("State file: %s\n", *internalStatePath)
		fmt.Printf("State: unavailable (%v)\n", err)
	} else {
		fmt.Printf("State file: %s\n", *internalStatePath)
		fmt.Printf("Registered: %t\n", internalState.IsRegistered())
		if internalState.AgentID != "" {
			fmt.Printf("Agent ID: %s\n", internalState.AgentID)
		}
		if internalState.CoreURL != "" {
			fmt.Printf("Core URL: %s\n", internalState.CoreURL)
		}
		fmt.Printf("Maintenance: %t\n", internalState.MaintenanceMode)
		if internalState.MaintenanceReason != nil {
			fmt.Printf("Maintenance reason: %s\n", *internalState.MaintenanceReason)
		}
	}

	if !running {
		os.Exit(1)
	}
}

func handleRestart() {
	if err := cli.RestartService(); err != nil {
		logging.Fatalf("Failed to restart service: %v", err)
	}
	fmt.Println("Agent service restarted")
}

func handleRun() {
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

	agentInstance := agent.NewWithStatePath(userConfig, internalState, *internalStatePath)

	go func() {
		<-sigs
		logging.Infof("Received shutdown signal, stopping agent...")
		cancel()
	}()

	if *once {
		// Run once for testing
		if err := agentInstance.RunOnce(ctx); err != nil {
			logging.Errorf("Agent run failed: %v", err)
			os.Exit(1)
		}
		return
	}

	if err := agentInstance.Run(ctx); err != nil {
		logging.Errorf("Agent stopped with error: %v", err)
		os.Exit(1)
	}

	logging.Infof("Orion Agent exited cleanly")
}
