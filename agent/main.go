package main

import (
	"encoding/json"
	"flag"
	"fmt"

	"orion/agent/internal/collector"
	"orion/agent/internal/logging"
)

// var (
// 	userConfigPath    = flag.String("config", "/etc/orion/config.yaml", "config.yaml")
// 	internalStatePath = flag.String("state", "/etc/orion/state.yaml", "state.yaml")
// 	once              = flag.Bool("once", false, "Run once and exit (for debugging)")
// )

func main() {
	flag.Parse()
	logging.Infof("Starting Orion Agent...")

	// userConfig, err := config.LoadUserConfig(*userConfigPath)
	// internalState, err := config.LoadInternalState(*internalStatePath)

	// if err != nil {
	// 	logging.Fatalf("Failed to load config: %v", err)
	// }

	systemMetrics, err := collector.Collect()
	if err != nil {
		logging.Fatalf("Failed to collect system metrics: %v", err)
	}

	prettyPrint(systemMetrics)

	// return

	// returnedName := systemMetrics.Hostname

	// regService := registration.New(userConfig, *userConfigPath, internalState, *internalStatePath)
	// if err := regService.RegisterAgentIfNeeded(); err != nil {
	// 	logging.Fatalf("Failed to register agent: %v", err)
	// }

	// userConfig, err = config.LoadUserConfig(*userConfigPath)
	// if err != nil {
	// 	logging.Fatalf("Failed to reload config after registration: %v", err)
	// }

	// a := agent.New(userConfig, internalState)

	// ctx, cancel := context.WithCancel(context.Background())
	// defer cancel()

	// sigs := make(chan os.Signal, 1)
	// signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	// // Run in a goroutine so we can catch signals
	// go func() {
	// 	if *once {
	// 		if err := a.RunOnce(ctx); err != nil {
	// 			logging.Errorf("Agent run failed: %v", err)
	// 		}
	// 		cancel()
	// 		return
	// 	}

	// 	if err := a.Run(ctx); err != nil {
	// 		logging.Errorf("Agent stopped with error: %v", err)
	// 	}
	// 	cancel()
	// }()

	// <-sigs
	// logging.Infof("Received shutdown signal, stopping agent...")
	// cancel()
	// time.Sleep(1 * time.Second)
	// logging.Infof("Orion Agent exited cleanly")
}

func prettyPrint(v interface{}) {
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(b))
}
