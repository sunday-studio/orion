package cli

import (
	"fmt"
	"os"
	"strings"

	"orion/agent/internal/config"
	"orion/agent/internal/logging"
	agentstate "orion/agent/internal/state"
	"orion/agent/internal/transport"
)

func HandleMaintenance(userConfigPath, internalStatePath *string) {
	if len(os.Args) < 2 {
		fmt.Println("Usage: orion-agent maintenance <up|down> [reason] [-state path]")
		os.Exit(1)
	}

	action, reason, statePath, err := parseMaintenanceCommand(os.Args[1:], *internalStatePath)
	if err != nil {
		fmt.Println(err)
		fmt.Println("Usage: orion-agent maintenance <up|down> [reason] [-state path]")
		os.Exit(1)
	}
	*internalStatePath = statePath

	PrintHeader("maintenance")
	PrintInfo("state", *internalStatePath)
	PrintInfo("action", action)
	if reason != "" {
		PrintInfo("reason", reason)
	}

	PrintStep("opening state database")
	stateStore, err := agentstate.Open(*internalStatePath)
	if err != nil {
		logging.Fatalf("Failed to open state database: %v", err)
	}
	defer stateStore.Close()
	PrintOK("state database ready")

	PrintStep("loading local agent state")
	internalState, err := stateStore.Get()
	if err != nil {
		logging.Fatalf("Failed to load state: %v", err)
	}
	PrintOK("local agent state loaded")

	switch action {
	case "-up":
		PrintStep("updating maintenance mode")
		if err := updateCoreMaintenanceMode(internalState, false); err != nil {
			logging.Fatalf("Failed to update core maintenance mode: %v", err)
		}

		if err := stateStore.SetMaintenanceMode(false, nil); err != nil {
			logging.Fatalf("Failed to save state: %v", err)
		}
		PrintOK("maintenance mode disabled")

	case "-down":
		var reasonPtr *string
		if reason != "" {
			reasonPtr = &reason
		}

		PrintStep("updating maintenance mode")
		if err := updateCoreMaintenanceMode(internalState, true); err != nil {
			logging.Fatalf("Failed to update core maintenance mode: %v", err)
		}

		if err := stateStore.SetMaintenanceMode(true, reasonPtr); err != nil {
			logging.Fatalf("Failed to save state: %v", err)
		}
		PrintOK("maintenance mode enabled")
		if reason != "" {
			PrintInfo("maintenance reason", reason)
		}

	default:
		fmt.Printf("Unknown maintenance action: %s\n", action)
		fmt.Println("Usage: orion-agent maintenance <up|down> [reason] [-state path]")
		os.Exit(1)
	}
}

func updateCoreMaintenanceMode(internalState *config.InternalState, enabled bool) error {
	if internalState.AgentID == "" && internalState.Token == "" {
		PrintSkip("agent is not registered; updating local maintenance state only")
		return nil
	}
	if internalState.AgentID == "" || internalState.Token == "" {
		return fmt.Errorf("state is missing agent id or token")
	}
	if internalState.CoreURL == "" {
		return fmt.Errorf("state is missing core URL")
	}

	client := transport.NewClient(internalState.CoreURL, internalState.Token)
	PrintInfo("core_url", internalState.CoreURL)
	if err := client.SetMaintenanceMode(internalState.AgentID, enabled); err != nil {
		return err
	}

	PrintOK("core maintenance mode updated")
	return nil
}

func parseMaintenanceCommand(args []string, defaultStatePath string) (string, string, string, error) {
	action := ""
	statePath := defaultStatePath
	reasonParts := []string{}

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-state" || arg == "--state":
			if i+1 >= len(args) {
				return "", "", "", fmt.Errorf("%s requires a path", arg)
			}
			statePath = args[i+1]
			i++
		case strings.HasPrefix(arg, "-state="):
			statePath = strings.TrimPrefix(arg, "-state=")
		case strings.HasPrefix(arg, "--state="):
			statePath = strings.TrimPrefix(arg, "--state=")
		case strings.HasPrefix(arg, "-") && arg != "-up" && arg != "-down":
			return "", "", "", fmt.Errorf("unknown maintenance option: %s", arg)
		case action == "":
			action = normalizeMaintenanceAction(arg)
		default:
			reasonParts = append(reasonParts, arg)
		}
	}

	if action == "" {
		return "", "", "", fmt.Errorf("missing maintenance action")
	}
	return action, strings.Join(reasonParts, " "), statePath, nil
}

func normalizeMaintenanceAction(action string) string {
	switch strings.ToLower(action) {
	case "up", "-up":
		return "-up"
	case "down", "-down":
		return "-down"
	default:
		return action
	}
}

func HandleConfig(userConfigPath *string) {
	if len(os.Args) < 2 {
		fmt.Println("Usage: orion-agent config <validate|diff> [-config path]")
		os.Exit(1)
	}

	subcommand, configPath, err := parseConfigCommand(os.Args[1:], *userConfigPath)
	if err != nil {
		fmt.Println(err)
		fmt.Println("Usage: orion-agent config <validate|diff> [-config path]")
		os.Exit(1)
	}
	*userConfigPath = configPath

	PrintHeader("config " + subcommand)
	PrintInfo("config", *userConfigPath)

	switch subcommand {
	case "validate":
		PrintStep("loading config")
		userConfig, err := config.LoadUserConfig(*userConfigPath)
		if err != nil {
			PrintError("config validation failed")
			PrintInfo("file", *userConfigPath)
			PrintInfo("reason", err)
			os.Exit(1)
		}
		PrintOK("config file is valid")
		PrintInfo("core_url", userConfig.CoreURL)
		PrintInfo("interval", userConfig.Interval)
		PrintInfo("monitors", len(userConfig.Monitors))
		if len(userConfig.Monitors) == 0 {
			PrintSkip("no monitor checks configured; host metrics will still report")
		}

	case "diff":
		PrintStep("loading config")
		currentConfig, err := config.LoadUserConfig(*userConfigPath)
		if err != nil {
			logging.Fatalf("Failed to load current config: %v", err)
		}
		PrintOK("config loaded")

		fmt.Println("Current configuration:")
		fmt.Printf("  Core URL: %s\n", currentConfig.CoreURL)
		fmt.Printf("  Interval: %s\n", currentConfig.Interval)
		fmt.Printf("  Monitors: %d\n", len(currentConfig.Monitors))
		if len(currentConfig.Monitors) == 0 {
			fmt.Println("    - none configured")
		} else {
			for _, m := range currentConfig.Monitors {
				fmt.Printf("    - %s (%s every %s)\n", m.Name, m.Type, m.Interval)
			}
		}

	default:
		fmt.Printf("Unknown config command: %s\n", subcommand)
		fmt.Println("Usage: orion-agent config <validate|diff> [-config path]")
		os.Exit(1)
	}
}

func parseConfigCommand(args []string, defaultConfigPath string) (string, string, error) {
	subcommand := ""
	configPath := defaultConfigPath

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-config" || arg == "--config":
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("%s requires a path", arg)
			}
			configPath = args[i+1]
			i++
		case strings.HasPrefix(arg, "-config="):
			configPath = strings.TrimPrefix(arg, "-config=")
		case strings.HasPrefix(arg, "--config="):
			configPath = strings.TrimPrefix(arg, "--config=")
		case strings.HasPrefix(arg, "-"):
			return "", "", fmt.Errorf("unknown config option: %s", arg)
		case subcommand == "":
			subcommand = arg
		default:
			return "", "", fmt.Errorf("unexpected config argument: %s", arg)
		}
	}

	if subcommand == "" {
		return "", "", fmt.Errorf("missing config subcommand")
	}
	return subcommand, configPath, nil
}

func HandleState(internalStatePath *string) {
	if len(os.Args) < 2 {
		fmt.Println("Usage: orion-agent state <init> [-state path]")
		os.Exit(1)
	}

	subcommand, statePath, err := parseStateCommand(os.Args[1:], *internalStatePath)
	if err != nil {
		fmt.Println(err)
		fmt.Println("Usage: orion-agent state <init> [-state path]")
		os.Exit(1)
	}
	*internalStatePath = statePath

	PrintHeader("state " + subcommand)
	PrintInfo("state", *internalStatePath)

	switch subcommand {
	case "init":
		PrintStep("opening state database")
		stateStore, err := agentstate.Open(*internalStatePath)
		if err != nil {
			logging.Fatalf("Failed to open state database: %v", err)
		}
		defer stateStore.Close()
		PrintOK("state database ready")

		PrintStep("ensuring default agent state row")
		if _, err := stateStore.Get(); err != nil {
			logging.Fatalf("Failed to initialize state database: %v", err)
		}
		PrintOK("state database initialized")
		PrintInfo("file", stateStore.Path())

	default:
		fmt.Printf("Unknown state command: %s\n", subcommand)
		fmt.Println("Usage: orion-agent state <init> [-state path]")
		os.Exit(1)
	}
}

func parseStateCommand(args []string, defaultStatePath string) (string, string, error) {
	subcommand := ""
	statePath := defaultStatePath

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-state" || arg == "--state":
			if i+1 >= len(args) {
				return "", "", fmt.Errorf("%s requires a path", arg)
			}
			statePath = args[i+1]
			i++
		case strings.HasPrefix(arg, "-state="):
			statePath = strings.TrimPrefix(arg, "-state=")
		case strings.HasPrefix(arg, "--state="):
			statePath = strings.TrimPrefix(arg, "--state=")
		case strings.HasPrefix(arg, "-"):
			return "", "", fmt.Errorf("unknown state option: %s", arg)
		case subcommand == "":
			subcommand = arg
		default:
			return "", "", fmt.Errorf("unexpected state argument: %s", arg)
		}
	}

	if subcommand == "" {
		return "", "", fmt.Errorf("missing state subcommand")
	}
	return subcommand, statePath, nil
}
