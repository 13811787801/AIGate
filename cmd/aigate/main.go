package main

import (
	"fmt"
	"os"

	"aigate/internal/config"
	"aigate/internal/server"
)

func main() {
	cfgPath := "configs/config.yaml"
	if v := os.Getenv("AIGATE_CONFIG"); v != "" {
		cfgPath = v
	}
	// 允许：aigate.exe -c path
	for i, a := range os.Args {
		if (a == "-c" || a == "--config") && i+1 < len(os.Args) {
			cfgPath = os.Args[i+1]
		}
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config failed: %v\n", err)
		fmt.Fprintf(os.Stderr, "hint: copy configs/config.example.yaml to configs/config.yaml\n")
		os.Exit(1)
	}

	if err := server.Run(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "server stopped: %v\n", err)
		os.Exit(1)
	}
}
