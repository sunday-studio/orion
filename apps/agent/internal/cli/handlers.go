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
		fmt.Println("Usage: orion-agent maintenance <-up|-down> [reason] [-state path]")
		os.Exit(1)
	}

	action, reason, statePath, err := parseMaintenanceCommand(os.Args[1:], *internalStatePath)
	if err != nil {
		fmt.Println(err)
		fmt.Println("Usage: orion-agent maintenance <-up|-down> [reason] [-state path]")
		os.Exit(1)
	}
	*internalStatePath = statePath

	stateStore, err := agentstate.Open(*internalStatePath)
	if err != nil {
		logging.Fatalf("Failed to open state database: %v", err)
	}
	defer stateStore.Close()

	internalState, err := stateStore.Get()
	if err != nil {
		logging.Fatalf("Failed to load state: %v", err)
	}

	switch action {
	case "-up":
		if err := updateCoreMaintenanceMode(internalState, false); err != nil {
			logging.Fatalf("Failed to update core maintenance mode: %v", err)
		}

		if err := stateStore.SetMaintenanceMode(false, nil); err != nil {
			logging.Fatalf("Failed to save state: %v", err)
		}
		fmt.Println("Maintenance mode disabled")

	case "-down":
		var reasonPtr *string
		if reason != "" {
			reasonPtr = &reason
		}

		if err := updateCoreMaintenanceMode(internalState, true); err != nil {
			logging.Fatalf("Failed to update core maintenance mode: %v", err)
		}

		if err := stateStore.SetMaintenanceMode(true, reasonPtr); err != nil {
			logging.Fatalf("Failed to save state: %v", err)
		}
		fmt.Printf("Maintenance mode enabled")
		if reason != "" {
			fmt.Printf(" (reason: %s)", reason)
		}
		fmt.Println()

	default:
		fmt.Printf("Unknown maintenance action: %s\n", action)
		fmt.Println("Usage: orion-agent maintenance <-up|-down> [reason] [-state path]")
		os.Exit(1)
	}
}

func updateCoreMaintenanceMode(internalState *config.InternalState, enabled bool) error {
	if internalState.AgentID == "" && internalState.Token == "" {
		logging.Warnf("Agent is not registered; updating local maintenance state only")
		return nil
	}
	if internalState.AgentID == "" || internalState.Token == "" {
		return fmt.Errorf("state is missing agent id or token")
	}
	if internalState.CoreURL == "" {
		return fmt.Errorf("state is missing core URL")
	}

	client := transport.NewClient(internalState.CoreURL, internalState.Token)
	if err := client.SetMaintenanceMode(internalState.AgentID, enabled); err != nil {
		return err
	}

	logging.Infof("Core maintenance mode updated")
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
			action = arg
		default:
			reasonParts = append(reasonParts, arg)
		}
	}

	if action == "" {
		return "", "", "", fmt.Errorf("missing maintenance action")
	}
	return action, strings.Join(reasonParts, " "), statePath, nil
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

	switch subcommand {
	case "validate":
		userConfig, err := config.LoadUserConfig(*userConfigPath)
		if err != nil {
			fmt.Println("Config validation failed")
			fmt.Printf("  file: %s\n", *userConfigPath)
			fmt.Printf("  error: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Config file is valid")
		fmt.Printf("  file: %s\n", *userConfigPath)
		fmt.Printf("  core_url: %s\n", userConfig.CoreURL)
		fmt.Printf("  interval: %s\n", userConfig.Interval)
		fmt.Printf("  monitors: %d\n", len(userConfig.Monitors))

	case "diff":
		// Load current config
		currentConfig, err := config.LoadUserConfig(*userConfigPath)
		if err != nil {
			logging.Fatalf("Failed to load current config: %v", err)
		}

		// For now, just show the config (diff would compare with a reference)
		fmt.Println("Current configuration:")
		fmt.Printf("  Core URL: %s\n", currentConfig.CoreURL)
		fmt.Printf("  Interval: %s\n", currentConfig.Interval)
		fmt.Printf("  Monitors: %d\n", len(currentConfig.Monitors))
		for _, m := range currentConfig.Monitors {
			fmt.Printf("    - %s (%s)\n", m.Name, m.Type)
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
