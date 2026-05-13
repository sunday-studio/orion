package cli

import (
	"fmt"
	"os"
	"strings"

	"orion/agent/internal/config"
	"orion/agent/internal/logging"
	"orion/agent/internal/transport"
)

func HandleMaintenance(userConfigPath, internalStatePath *string) {
	if len(os.Args) < 3 {
		fmt.Println("Usage: orion-agent maintenance <-up|-down> [reason]")
		os.Exit(1)
	}

	action := os.Args[2]

	internalState, err := config.LoadInternalState(*internalStatePath)
	if err != nil {
		logging.Fatalf("Failed to load internal state: %v", err)
	}

	switch action {
	case "-up":
		internalState.MaintenanceMode = false
		internalState.MaintenanceReason = nil
		if err := internalState.Save(*internalStatePath); err != nil {
			logging.Fatalf("Failed to save state: %v", err)
		}
		fmt.Println("Maintenance mode disabled")

		// Notify core via API
		if internalState.AgentID != "" && internalState.Token != "" {
			client := transport.NewClient(internalState.CoreURL, internalState.Token)
			if err := client.SetMaintenanceMode(internalState.AgentID, false); err != nil {
				logging.Warnf("Failed to update core maintenance mode: %v", err)
			} else {
				logging.Infof("Core maintenance mode updated")
			}
		}

	case "-down":
		reason := ""
		if len(os.Args) > 3 {
			reason = strings.Join(os.Args[3:], " ")
		}
		internalState.MaintenanceMode = true
		if reason != "" {
			internalState.MaintenanceReason = &reason
		}
		if err := internalState.Save(*internalStatePath); err != nil {
			logging.Fatalf("Failed to save state: %v", err)
		}
		fmt.Printf("Maintenance mode enabled")
		if reason != "" {
			fmt.Printf(" (reason: %s)", reason)
		}
		fmt.Println()

		// Notify core via API
		if internalState.AgentID != "" && internalState.Token != "" {
			client := transport.NewClient(internalState.CoreURL, internalState.Token)
			if err := client.SetMaintenanceMode(internalState.AgentID, true); err != nil {
				logging.Warnf("Failed to update core maintenance mode: %v", err)
			} else {
				logging.Infof("Core maintenance mode updated")
			}
		}

	default:
		fmt.Printf("Unknown maintenance action: %s\n", action)
		fmt.Println("Usage: orion-agent maintenance <-up|-down> [reason]")
		os.Exit(1)
	}
}

func HandleConfig(userConfigPath *string) {
	if len(os.Args) < 3 {
		fmt.Println("Usage: orion-agent config <validate|diff>")
		os.Exit(1)
	}

	subcommand := os.Args[2]

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
		fmt.Println("Usage: orion-agent config <validate|diff>")
		os.Exit(1)
	}
}
