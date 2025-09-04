package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/najoast/sngo/cluster"
)

// EchoService is a simple service that echoes messages
type EchoService struct {
	nodeID string
}

func (es *EchoService) Handle(ctx context.Context, request interface{}) (interface{}, error) {
	message, ok := request.(string)
	if !ok {
		return nil, fmt.Errorf("invalid request type")
	}

	response := fmt.Sprintf("Echo from node %s: %s", es.nodeID, message)
	log.Printf("Handling request: %s -> %s", message, response)
	return response, nil
}

// CounterService maintains a distributed counter
type CounterService struct {
	nodeID string
	count  int
}

func (cs *CounterService) Handle(ctx context.Context, request interface{}) (interface{}, error) {
	cmd, ok := request.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid request format")
	}

	action, exists := cmd["action"]
	if !exists {
		return nil, fmt.Errorf("missing action")
	}

	switch action {
	case "increment":
		cs.count++
		response := map[string]interface{}{
			"node":  cs.nodeID,
			"count": cs.count,
		}
		log.Printf("Counter incremented on node %s: %d", cs.nodeID, cs.count)
		return response, nil

	case "get":
		response := map[string]interface{}{
			"node":  cs.nodeID,
			"count": cs.count,
		}
		return response, nil

	default:
		return nil, fmt.Errorf("unknown action: %v", action)
	}
}

func main() {
	// Parse command line arguments
	if len(os.Args) < 2 {
		log.Fatal("Usage: cluster_example <node_id> [seed_nodes...]")
	}

	nodeID := os.Args[1]
	seedNodes := os.Args[2:]

	// Parse port from environment or use default
	port := 7946
	if portStr := os.Getenv("CLUSTER_PORT"); portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			port = p
		}
	}

	// Create cluster configuration
	config := cluster.DefaultClusterConfig()
	config.NodeID = cluster.NodeID(nodeID)
	config.BindPort = port
	config.SeedNodes = seedNodes
	config.ClusterName = "sngo-example-cluster"

	// Add some metadata
	config.Metadata = map[string]string{
		"version": "1.0.0",
		"role":    "worker",
		"region":  "local",
	}

	log.Printf("Starting cluster node %s on port %d", nodeID, port)
	if len(seedNodes) > 0 {
		log.Printf("Seed nodes: %v", seedNodes)
	}

	// Create cluster application
	app, err := cluster.NewClusterApp("")
	if err != nil {
		log.Fatalf("Failed to create cluster app: %v", err)
	}

	// Get services
	remoteService := app.GetRemoteService()
	if remoteService == nil {
		log.Fatal("Remote service not available")
	}

	manager := app.GetClusterManager()
	if manager == nil {
		log.Fatal("Cluster manager not available")
	}

	// Register services
	echoService := &EchoService{nodeID: nodeID}
	if err := remoteService.Register("echo", echoService); err != nil {
		log.Fatalf("Failed to register echo service: %v", err)
	}

	counterService := &CounterService{nodeID: nodeID}
	if err := remoteService.Register("counter", counterService); err != nil {
		log.Fatalf("Failed to register counter service: %v", err)
	}

	log.Printf("Registered services: echo, counter")

	// Listen for cluster events
	go func() {
		for event := range manager.Events() {
			log.Printf("Cluster event: %s - Node: %s at %s",
				event.Type, event.NodeID, event.Timestamp.Format(time.RFC3339))
		}
	}()

	// Print cluster status periodically
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				printClusterStatus(manager)
			}
		}
	}()

	// Demo remote calls (only after some time to allow cluster to form)
	go func() {
		time.Sleep(5 * time.Second)
		demonstrateRemoteCalls(remoteService, manager)
	}()

	// Start the application
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutdown signal received, stopping application...")
		cancel()
	}()

	// Start application
	log.Println("Starting cluster application...")
	if err := app.Start(ctx); err != nil {
		log.Fatalf("Application failed: %v", err)
	}

	log.Println("Application stopped")
}

func printClusterStatus(manager cluster.ClusterManager) {
	health := manager.GetClusterHealth()

	log.Printf("=== Cluster Status ===")
	log.Printf("Healthy: %t", health.IsHealthy)
	log.Printf("Nodes: %d total, %d active, %d suspected, %d failed",
		health.TotalNodes, health.ActiveNodes, health.SuspectedNodes, health.FailedNodes)
	log.Printf("Leader: %t (ID: %s)", health.HasLeader, health.LeaderID)
	log.Printf("Partitions: %d", health.PartitionCount)

	// List all nodes
	nodes := manager.GetAllNodes()
	log.Printf("All nodes:")
	for _, node := range nodes {
		info := node.Info()
		log.Printf("  - %s: %s (%s) - Load: %.2f",
			info.ID, info.Address, info.State, info.Load)
	}
	log.Printf("=====================")
}

func demonstrateRemoteCalls(remoteService cluster.RemoteService, manager cluster.ClusterManager) {
	log.Println("=== Demonstrating Remote Calls ===")

	// Get all nodes except local
	localNodeID := manager.LocalNode().ID()
	nodes := manager.GetActiveNodes()

	for _, node := range nodes {
		if node.ID() == localNodeID {
			continue // Skip local node
		}

		// Demonstrate echo service call
		echoRef := cluster.RemoteActorRef{
			NodeID:  node.ID(),
			ActorID: "echo",
			Address: node.Address().String(),
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		result, err := remoteService.Call(ctx, echoRef, "Hello from "+string(localNodeID))
		cancel()

		if err != nil {
			log.Printf("Echo call to %s failed: %v", node.ID(), err)
		} else {
			log.Printf("Echo response from %s: %v", node.ID(), result)
		}

		// Demonstrate counter service call
		counterRef := cluster.RemoteActorRef{
			NodeID:  node.ID(),
			ActorID: "counter",
			Address: node.Address().String(),
		}

		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		result, err = remoteService.Call(ctx, counterRef, map[string]interface{}{
			"action": "increment",
		})
		cancel()

		if err != nil {
			log.Printf("Counter call to %s failed: %v", node.ID(), err)
		} else {
			log.Printf("Counter response from %s: %v", node.ID(), result)
		}

		// Fire and forget call
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		err = remoteService.Send(ctx, echoRef, "Fire and forget message")
		cancel()

		if err != nil {
			log.Printf("Fire and forget to %s failed: %v", node.ID(), err)
		} else {
			log.Printf("Fire and forget sent to %s", node.ID())
		}
	}

	log.Println("=== Remote Calls Demo Complete ===")
}
