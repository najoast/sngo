// Package main demonstrates how to use the SNGO configuration system
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/najoast/sngo/config"
)

func main() {
	fmt.Println("SNGO Configuration System Demo")
	fmt.Println("==============================")

	// Create a configuration loader
	loader := config.NewLoader()

	// Try to auto-load configuration
	cfg, err := loader.AutoLoad()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Display loaded configuration
	displayConfig(cfg)

	// Demonstrate configuration watching
	demonstrateWatching(cfg, loader)
}

func displayConfig(cfg *config.Config) {
	fmt.Printf("\nLoaded Configuration:\n")
	fmt.Printf("  App Name: %s\n", cfg.App.Name)
	fmt.Printf("  Version: %s\n", cfg.App.Version)
	fmt.Printf("  Environment: %s\n", cfg.App.Environment)
	fmt.Printf("  Debug Mode: %t\n", cfg.IsDebugEnabled())

	fmt.Printf("\nNetwork Settings:\n")
	fmt.Printf("  TCP Address: %s:%d\n", cfg.Network.TCP.Address, cfg.Network.TCP.Port)
	fmt.Printf("  Max Connections: %d\n", cfg.Network.Limits.MaxConnections)

	fmt.Printf("\nLogging Settings:\n")
	fmt.Printf("  Level: %s\n", cfg.Log.Level)
	fmt.Printf("  Format: %s\n", cfg.Log.Format)
	fmt.Printf("  Output: %s\n", cfg.Log.Output)

	fmt.Printf("\nActor System:\n")
	fmt.Printf("  Max Actors: %d\n", cfg.Actor.MaxActors)
	fmt.Printf("  Default Mailbox Size: %d\n", cfg.Actor.DefaultMailboxSize)

	fmt.Printf("\nService Discovery:\n")
	fmt.Printf("  Enabled: %t\n", cfg.Discovery.Enabled)
	fmt.Printf("  Type: %s\n", cfg.Discovery.Type)
	fmt.Printf("  Service Name: %s\n", cfg.GetServiceName())

	if len(cfg.Custom) > 0 {
		fmt.Printf("\nCustom Configuration:\n")
		for key, value := range cfg.Custom {
			fmt.Printf("  %s: %v\n", key, value)
		}
	}
}

func demonstrateWatching(cfg *config.Config, loader *config.Loader) {
	// Check if there's a config file to watch
	configFiles := []string{"config.yaml", "config.yml", "config.json"}
	var configFile string

	for _, file := range configFiles {
		if _, err := os.Stat(file); err == nil {
			configFile = file
			break
		}
	}

	if configFile == "" {
		fmt.Printf("\nNo configuration file found for watching demo.\n")
		fmt.Printf("To see configuration hot-reload in action:\n")
		fmt.Printf("1. Create a config.yaml file in the current directory\n")
		fmt.Printf("2. Run this program again\n")
		fmt.Printf("3. Modify the config file while the program is running\n")
		return
	}

	fmt.Printf("\nStarting configuration watcher for: %s\n", configFile)
	fmt.Printf("Modify the config file to see hot-reload in action...\n")
	fmt.Printf("Press Ctrl+C to exit\n\n")

	// Create file provider with watching capability
	provider, err := config.NewFileProvider(configFile)
	if err != nil {
		log.Printf("Failed to create file provider: %v", err)
		return
	}
	defer provider.Close()

	// Set up context for cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start watching for configuration changes
	go func() {
		err := provider.Watch(ctx, func(oldConfig, newConfig *config.Config) {
			fmt.Printf("[%s] Configuration changed!\n", time.Now().Format("15:04:05"))

			// Show what changed
			if oldConfig.App.Name != newConfig.App.Name {
				fmt.Printf("  App Name: %s -> %s\n", oldConfig.App.Name, newConfig.App.Name)
			}
			if oldConfig.Network.TCP.Port != newConfig.Network.TCP.Port {
				fmt.Printf("  TCP Port: %d -> %d\n", oldConfig.Network.TCP.Port, newConfig.Network.TCP.Port)
			}
			if oldConfig.Log.Level != newConfig.Log.Level {
				fmt.Printf("  Log Level: %s -> %s\n", oldConfig.Log.Level, newConfig.Log.Level)
			}
			if oldConfig.Actor.MaxActors != newConfig.Actor.MaxActors {
				fmt.Printf("  Max Actors: %d -> %d\n", oldConfig.Actor.MaxActors, newConfig.Actor.MaxActors)
			}

			fmt.Printf("  Configuration reloaded successfully!\n\n")
		})

		if err != nil {
			log.Printf("Configuration watching failed: %v", err)
		}
	}()

	// Wait for signal
	<-sigChan
	fmt.Printf("\nShutting down configuration watcher...\n")
}
