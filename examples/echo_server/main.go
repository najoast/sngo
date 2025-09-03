// Package main provides a simple echo server example using SNGO network layer
package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sngo/sngo/network"
)

func main() {
	// Create server configuration
	config := network.DefaultNetworkConfig()
	config.Port = 8080
	config.MaxConnections = 100

	// Create TCP server
	server, err := network.CreateTCPServer(config)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Create connection manager
	connManager := network.CreateConnectionManager()

	// Set up connection handler
	server.SetConnectionHandler(&EchoConnectionHandler{
		manager: connManager,
	})

	// Set up message handler
	server.SetMessageHandler(&EchoMessageHandler{})

	// Start server
	fmt.Printf("Starting echo server on port %d...\n", config.Port)
	err = server.Start()
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}

	// Start heartbeat for connection management
	err = connManager.StartHeartbeat(30 * time.Second)
	if err != nil {
		log.Printf("Failed to start heartbeat: %v", err)
	}

	// Start statistics reporting
	go reportStatistics(server, connManager)

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("Echo server is running. Press Ctrl+C to stop.")
	<-sigChan

	// Graceful shutdown
	fmt.Println("\nShutting down server...")

	connManager.StopHeartbeat()
	connManager.CloseAllConnections()
	server.Stop()

	fmt.Println("Server stopped.")
}

// EchoConnectionHandler handles new connections
type EchoConnectionHandler struct {
	manager network.ConnectionManager
}

func (h *EchoConnectionHandler) OnConnect(conn network.Connection) {
	fmt.Printf("New connection: %s from %s\n", conn.ID(), conn.RemoteAddr())

	// Add to connection manager
	err := h.manager.AddConnection(conn)
	if err != nil {
		log.Printf("Failed to add connection to manager: %v", err)
	}

	// Send welcome message
	welcome := network.NewMessage(network.MessageTypeData, []byte("Welcome to SNGO Echo Server!"))
	conn.SendMessage(welcome)
}

func (h *EchoConnectionHandler) OnDisconnect(conn network.Connection, err error) {
	if err != nil {
		fmt.Printf("Connection %s disconnected with error: %v\n", conn.ID(), err)
	} else {
		fmt.Printf("Connection %s disconnected gracefully\n", conn.ID())
	}

	// Remove from connection manager
	h.manager.RemoveConnection(conn.ID())
}

func (h *EchoConnectionHandler) OnError(conn network.Connection, err error) {
	fmt.Printf("Connection %s error: %v\n", conn.ID(), err)
}

// EchoMessageHandler handles incoming messages
type EchoMessageHandler struct{}

func (h *EchoMessageHandler) OnMessage(conn network.Connection, msg *network.Message) {
	switch msg.Type {
	case network.MessageTypeHeartbeat:
		// Respond to heartbeat
		ack := network.NewAckMessage(msg.Sequence)
		conn.SendMessage(ack)

	case network.MessageTypeData:
		// Echo the message back with prefix
		echoData := fmt.Sprintf("Echo: %s", string(msg.Data))
		response := network.NewMessage(network.MessageTypeData, []byte(echoData))
		conn.SendMessage(response)

		fmt.Printf("Echoed to %s: %s\n", conn.ID(), string(msg.Data))

	case network.MessageTypeRPC:
		// Handle RPC calls
		h.handleRPC(conn, msg)

	default:
		fmt.Printf("Unknown message type from %s: %v\n", conn.ID(), msg.Type)
	}
}

func (h *EchoMessageHandler) OnError(conn network.Connection, err error) {
	fmt.Printf("Message handling error for %s: %v\n", conn.ID(), err)
}

func (h *EchoMessageHandler) handleRPC(conn network.Connection, msg *network.Message) {
	rpcCall := string(msg.Data)
	var response []byte

	switch rpcCall {
	case "ping":
		response = []byte("pong")
	case "time":
		response = []byte(time.Now().Format(time.RFC3339))
	case "stats":
		stats := conn.GetStatistics()
		response = []byte(fmt.Sprintf("Bytes: R/W=%d/%d, Messages: R/S=%d/%d",
			stats.BytesRead, stats.BytesWritten, stats.MessagesRead, stats.MessagesSent))
	default:
		response = []byte(fmt.Sprintf("Unknown RPC call: %s", rpcCall))
	}

	rpcResponse := network.NewRPCMessage(msg.Destination, msg.Source, response)
	conn.SendMessage(rpcResponse)
}

// reportStatistics periodically reports server statistics
func reportStatistics(server network.Server, manager network.ConnectionManager) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		serverStats := server.GetStatistics()
		managerStats := manager.GetStatistics()

		fmt.Printf("\n=== Server Statistics ===\n")
		fmt.Printf("Uptime: %v\n", serverStats.Uptime.Truncate(time.Second))
		fmt.Printf("Total Connections: %d\n", serverStats.TotalConnections)
		fmt.Printf("Current Connections: %d\n", serverStats.CurrentConnections)
		fmt.Printf("Total Messages: %d\n", serverStats.TotalMessages)
		fmt.Printf("Total Bytes: %d\n", managerStats.TotalBytes)
		fmt.Printf("========================\n\n")
	}
}
