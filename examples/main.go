package main

import (
	"context"
	"log"

	"github.com/najoast/sngo/bootstrap"
	"github.com/najoast/sngo/examples/gate"
	"github.com/najoast/sngo/examples/simpledb"
	"github.com/najoast/sngo/examples/watchdog"
)

func main() {
	// Create application
	app := bootstrap.NewApplication()

	// Register services
	lifecycleManager := app.LifecycleManager()

	// Register SimpleDB service
	dbService := simpledb.NewSimpleDBService()
	if err := lifecycleManager.Register("simpledb", dbService); err != nil {
		log.Fatalf("Failed to register SimpleDB service: %v", err)
	}

	// Register Watchdog service
	watchdogService := watchdog.NewWatchdogService()
	if err := lifecycleManager.Register("watchdog", watchdogService); err != nil {
		log.Fatalf("Failed to register Watchdog service: %v", err)
	}

	// Register Gate service
	gateService := gate.NewGateService()
	if err := lifecycleManager.Register("gate", gateService); err != nil {
		log.Fatalf("Failed to register Gate service: %v", err)
	}

	log.Println("Starting SNGO Examples Application")
	log.Println("Services registered:")
	log.Println("- SimpleDB: In-memory key-value database")
	log.Println("- Watchdog: Connection manager and agent creator")
	log.Println("- Gate: Network gateway and message forwarder")

	// Start application
	ctx := context.Background()
	if err := app.Run(ctx); err != nil {
		log.Fatalf("Application failed: %v", err)
	}
}
