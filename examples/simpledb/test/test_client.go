package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/najoast/sngo/core"
	"github.com/najoast/sngo/examples/simpledb"
)

// TestClient tests the SimpleDB service
type TestClient struct{}

// HandleMessage implements the MessageHandler interface for test client
func (tc *TestClient) HandleMessage(ctx context.Context, msg *core.Message) error {
	log.Printf("Test client received message: %s", string(msg.Data))
	return nil
}

// testSimpleDB runs a series of tests against the SimpleDB service
func testSimpleDB(system core.ActorSystem) {
	// Create test client
	client := &TestClient{}
	clientActor, err := system.NewActor(client, core.DefaultActorOptions())
	if err != nil {
		log.Fatalf("Failed to create test client: %v", err)
	}

	// Get SimpleDB service
	dbHandle, exists := system.GetService("SIMPLEDB")
	if !exists {
		log.Fatalf("SimpleDB service not found")
	}

	log.Printf("Found SimpleDB service: %v", dbHandle)

	// Test SET command
	setReq := simpledb.DBRequest{
		Command: "SET",
		Args:    []interface{}{"test_key", "test_value"},
	}
	setData, _ := json.Marshal(setReq)

	err = system.Send(clientActor.ID(), dbHandle.ActorID, core.MessageTypeRequest, setData)
	if err != nil {
		log.Printf("Failed to send SET request: %v", err)
	} else {
		log.Printf("Sent SET request")
	}

	time.Sleep(100 * time.Millisecond)

	// Test GET command
	getReq := simpledb.DBRequest{
		Command: "GET",
		Args:    []interface{}{"test_key"},
	}
	getData, _ := json.Marshal(getReq)

	err = system.Send(clientActor.ID(), dbHandle.ActorID, core.MessageTypeRequest, getData)
	if err != nil {
		log.Printf("Failed to send GET request: %v", err)
	} else {
		log.Printf("Sent GET request")
	}

	time.Sleep(100 * time.Millisecond)

	// Test PING command
	pingReq := simpledb.DBRequest{
		Command: "PING",
		Args:    []interface{}{"hello"},
	}
	pingData, _ := json.Marshal(pingReq)

	err = system.Send(clientActor.ID(), dbHandle.ActorID, core.MessageTypeRequest, pingData)
	if err != nil {
		log.Printf("Failed to send PING request: %v", err)
	} else {
		log.Printf("Sent PING request")
	}

	time.Sleep(100 * time.Millisecond)

	// Test KEYS command
	keysReq := simpledb.DBRequest{
		Command: "KEYS",
		Args:    []interface{}{"*"},
	}
	keysData, _ := json.Marshal(keysReq)

	err = system.Send(clientActor.ID(), dbHandle.ActorID, core.MessageTypeRequest, keysData)
	if err != nil {
		log.Printf("Failed to send KEYS request: %v", err)
	} else {
		log.Printf("Sent KEYS request")
	}

	time.Sleep(100 * time.Millisecond)

	// Test SIZE command
	sizeReq := simpledb.DBRequest{
		Command: "SIZE",
	}
	sizeData, _ := json.Marshal(sizeReq)

	err = system.Send(clientActor.ID(), dbHandle.ActorID, core.MessageTypeRequest, sizeData)
	if err != nil {
		log.Printf("Failed to send SIZE request: %v", err)
	} else {
		log.Printf("Sent SIZE request")
	}

	log.Printf("All test requests sent")
}

// Create a separate main function for testing
func runTest() {
	fmt.Println("SimpleDB Test Client")
	fmt.Println("====================")

	// Create actor system
	system := core.NewActorSystem()

	// Create SimpleDB service directly
	db := simpledb.NewSimpleDB()
	handle, err := system.NewService("SIMPLEDB", db, core.DefaultActorOptions())
	if err != nil {
		log.Fatalf("Failed to create SimpleDB service: %v", err)
	}

	log.Printf("SimpleDB service created with handle: %v", handle)

	// Wait a moment for service to start
	time.Sleep(100 * time.Millisecond)

	// Run tests
	testSimpleDB(system)

	// Wait for tests to complete
	time.Sleep(1 * time.Second)

	// Shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := system.Shutdown(ctx); err != nil {
		log.Printf("Failed to shutdown system: %v", err)
	}

	fmt.Println("Test completed")
}
