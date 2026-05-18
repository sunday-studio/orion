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
	agentstate "orion/agent/internal/state"
)

var (
	userConfigPath    = flag.String("config", "config.yaml", config.DefaultPath())
	internalStatePath = flag.String("state", agentstate.DefaultPath(), "Path to SQLite state database")
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
	case "help", "-h", "--help":
		printUsage()
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
	case "state":
		cli.HandleState(internalStatePath)
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
	fmt.Println("  state         Manage local state")
	fmt.Println("                init     Initialize state database")
	fmt.Println("")
	fmt.Println("Options:")
	fmt.Println("  -config   Path to config file (default: config.yaml)")
	fmt.Println("  -state    Path to SQLite state database (default: state.db)")
	fmt.Println("  -once     Run once and exit (for debugging)")
	fmt.Println("")
	fmt.Println("Common checks:")
	fmt.Println("  orion-agent config validate -config /etc/orion/config.yaml")
	fmt.Println("  orion-agent state init -state /var/lib/orion/state.db")
	fmt.Println("  orion-agent status -state /var/lib/orion/state.db")
	fmt.Println("  orion-agent run -config /etc/orion/config.yaml -state /var/lib/orion/state.db -once")
}

func handleStart() {
	manager := cli.DetectServiceManager()
	cli.PrintHeader("start")
	cli.PrintInfo("service_manager", manager)
	cli.PrintStep("starting service")
	if err := cli.StartService(); err != nil {
		logging.Fatalf("Failed to start service: %v", err)
	}
	cli.PrintOK("agent service started")
	printServiceStatus()
}

func handleStop() {
	manager := cli.DetectServiceManager()
	cli.PrintHeader("stop")
	cli.PrintInfo("service_manager", manager)
	cli.PrintStep("stopping service")
	if err := cli.StopService(); err != nil {
		logging.Fatalf("Failed to stop service: %v", err)
	}
	cli.PrintOK("agent service stopped")
	printServiceStatus()
}

func handleStatus() {
	flag.Parse()
	cli.PrintHeader("status")
	cli.PrintInfo("state", *internalStatePath)
	cli.PrintStep("checking service")
	running, status, err := cli.GetServiceStatus()
	if err != nil {
		logging.Fatalf("Failed to get service status: %v", err)
	}

	fmt.Printf("  service_manager: %s\n", cli.DetectServiceManager())
	if running {
		fmt.Printf("  agent_service: %s\n", status)
	} else {
		fmt.Printf("  agent_service: %s\n", status)
	}

	cli.PrintStep("opening state database")
	stateStore, err := agentstate.Open(*internalStatePath)
	if err != nil {
		fmt.Printf("  state_database: %s\n", *internalStatePath)
		fmt.Printf("  state: unavailable (%v)\n", err)
	} else {
		defer stateStore.Close()
		internalState, err := stateStore.Get()
		if err != nil {
			fmt.Printf("  state_database: %s\n", *internalStatePath)
			fmt.Printf("  state: unavailable (%v)\n", err)
		} else {
			fmt.Printf("  state_database: %s\n", stateStore.Path())
			fmt.Printf("  registered: %t\n", internalState.IsRegistered())
			if internalState.AgentID != "" {
				fmt.Printf("  agent_id: %s\n", internalState.AgentID)
			}
			if internalState.CoreURL != "" {
				fmt.Printf("  core_url: %s\n", internalState.CoreURL)
			}
			fmt.Printf("  maintenance: %t\n", internalState.MaintenanceMode)
			if internalState.MaintenanceReason != nil {
				fmt.Printf("  maintenance_reason: %s\n", *internalState.MaintenanceReason)
			}
		}
	}

	if !running {
		cli.PrintSkip("service is not running")
		os.Exit(1)
	}
	cli.PrintOK("service is running")
}

func handleRestart() {
	manager := cli.DetectServiceManager()
	cli.PrintHeader("restart")
	cli.PrintInfo("service_manager", manager)
	cli.PrintStep("restarting service")
	if err := cli.RestartService(); err != nil {
		logging.Fatalf("Failed to restart service: %v", err)
	}
	cli.PrintOK("agent service restarted")
	printServiceStatus()
}

func handleRun() {
	flag.Parse()
	cli.PrintHeader("run")
	cli.PrintInfo("config", *userConfigPath)
	cli.PrintInfo("state", *internalStatePath)
	cli.PrintInfo("once", *once)

	cli.PrintStep("loading config")
	userConfig, err := config.LoadUserConfig(*userConfigPath)
	if err != nil {
		logging.Fatalf("Failed to load user config: %v", err)
	}
	cli.PrintOK(fmt.Sprintf("config loaded with %d monitor(s)", len(userConfig.Monitors)))
	cli.PrintInfo("core_url", userConfig.CoreURL)
	cli.PrintInfo("interval", userConfig.Interval)
	if len(userConfig.Monitors) == 0 {
		cli.PrintSkip("no monitor checks configured; host metrics will still report")
	}

	cli.PrintStep("opening state database")
	stateStore, err := agentstate.Open(*internalStatePath)
	if err != nil {
		logging.Fatalf("Failed to open state database: %v", err)
	}
	defer stateStore.Close()
	cli.PrintOK("state database ready")

	cli.PrintStep("registering agent and monitors")
	registrationService := registration.New(userConfig, *userConfigPath, stateStore)
	if err := registrationService.RegisterAgentIfNeeded(); err != nil {
		cli.PrintError("registration failed after retry attempts; agent cannot continue")
		logging.Fatalf("Failed to register agent & monitors: %v", err)
	}
	cli.PrintOK("registration complete")
	if internalState, err := stateStore.Get(); err == nil {
		cli.PrintInfo("registered", internalState.IsRegistered())
		if internalState.AgentID != "" {
			cli.PrintInfo("agent_id", internalState.AgentID)
		}
		cli.PrintInfo("monitor_mappings", len(internalState.Monitors))
	}

	ctx, cancel := context.WithCancel(context.Background())

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	agentInstance, err := agent.NewWithStateStore(userConfig, stateStore)
	if err != nil {
		logging.Fatalf("Failed to initialize agent: %v", err)
	}
	cli.PrintOK("agent initialized")

	go func() {
		<-sigs
		cli.PrintStep("received shutdown signal")
		cancel()
	}()

	if *once {
		cli.PrintStep("running one collection cycle")
		if err := agentInstance.RunOnce(ctx); err != nil {
			logging.Errorf("Agent run failed: %v", err)
			os.Exit(1)
		}
		cli.PrintOK("one collection cycle complete")
		return
	}

	cli.PrintStep("starting continuous collection loop")
	if err := agentInstance.Run(ctx); err != nil {
		logging.Errorf("Agent stopped with error: %v", err)
		os.Exit(1)
	}

	cli.PrintOK("agent exited cleanly")
}

func printServiceStatus() {
	running, status, err := cli.GetServiceStatus()
	if err != nil {
		cli.PrintError(fmt.Sprintf("could not read service state: %v", err))
		return
	}
	cli.PrintInfo("service_state", status)
	if running {
		cli.PrintOK("service is running")
	} else {
		cli.PrintSkip("service is not running")
	}
}
