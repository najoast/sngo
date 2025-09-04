package main

import (
	"log"
	"time"

	"github.com/najoast/sngo/examples/client"
)

func main() {
	log.Println("SNGO Integration Test")
	log.Println("====================")

	// Wait a moment for services to start
	time.Sleep(2 * time.Second)

	log.Println("Note: This test assumes the main SNGO application is running")
	log.Println("You should start it with: go run main.go")
	log.Println("")

	// For now, just run the client test
	// The client will try to connect to port 8888
	// If gate is not listening, it will fail gracefully
	client.RunTest()

	log.Println("Integration test completed.")
}
