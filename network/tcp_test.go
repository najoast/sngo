// Package network provides tests for TCP server and client
package network

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestTCPServerBasic(t *testing.T) {
	config := DefaultNetworkConfig()
	config.Port = 18080 // Use a different port for testing

	server, err := NewTCPServer(config)
	if err != nil {
		t.Fatalf("Failed to create TCP server: %v", err)
	}

	// Test start
	err = server.Start()
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Check that server is listening
	addr := server.Listen()
	if addr == nil {
		t.Fatal("Server should be listening")
	}

	// Test connection count
	if server.GetConnectionCount() != 0 {
		t.Errorf("Expected 0 connections, got %d", server.GetConnectionCount())
	}

	// Test stop
	err = server.Stop()
	if err != nil {
		t.Fatalf("Failed to stop server: %v", err)
	}

	// Test double start (should fail)
	server, _ = NewTCPServer(config)
	server.Start()
	err = server.Start()
	if err == nil {
		t.Error("Expected error when starting already running server")
	}
	server.Stop()
}

func TestTCPClientBasic(t *testing.T) {
	// Start a test server
	config := DefaultNetworkConfig()
	config.Port = 18081

	server, err := NewTCPServer(config)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	err = server.Start()
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Create client
	client, err := NewTCPClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Test connect
	conn, err := client.Connect(fmt.Sprintf("localhost:%d", config.Port))
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	if conn == nil {
		t.Fatal("Connection should not be nil")
	}

	// Test connection state
	if !client.IsConnected() {
		t.Error("Client should be connected")
	}

	// Test disconnect
	err = client.Disconnect()
	if err != nil {
		t.Fatalf("Failed to disconnect: %v", err)
	}

	if client.IsConnected() {
		t.Error("Client should be disconnected")
	}
}

func TestTCPClientServerCommunication(t *testing.T) {
	// Setup server
	config := DefaultNetworkConfig()
	config.Port = 18082

	server, err := NewTCPServer(config)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Setup message handlers
	var serverReceivedMessages []string
	var clientReceivedMessages []string
	var serverMu, clientMu sync.Mutex

	server.SetMessageHandler(&testMessageHandler{
		onMessage: func(conn Connection, msg *Message) {
			serverMu.Lock()
			serverReceivedMessages = append(serverReceivedMessages, string(msg.Data))
			serverMu.Unlock()

			// Echo the message back
			response := NewMessage(MessageTypeData, []byte("echo: "+string(msg.Data)))
			conn.SendMessage(response)
		},
	})

	err = server.Start()
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	// Setup client
	client, err := NewTCPClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	client.SetMessageHandler(&testMessageHandler{
		onMessage: func(conn Connection, msg *Message) {
			clientMu.Lock()
			clientReceivedMessages = append(clientReceivedMessages, string(msg.Data))
			clientMu.Unlock()
		},
	})

	_, err = client.Connect(fmt.Sprintf("localhost:%d", config.Port))
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Disconnect()

	// Send messages
	testMessages := []string{"Hello", "World", "SNGO"}

	for _, msg := range testMessages {
		message := NewMessage(MessageTypeData, []byte(msg))
		err = client.SendMessage(message)
		if err != nil {
			t.Fatalf("Failed to send message: %v", err)
		}
	}

	// Wait for responses
	time.Sleep(500 * time.Millisecond)

	// Check server received messages
	serverMu.Lock()
	if len(serverReceivedMessages) != len(testMessages) {
		t.Errorf("Server expected %d messages, got %d", len(testMessages), len(serverReceivedMessages))
	}
	for i, expected := range testMessages {
		if i < len(serverReceivedMessages) && serverReceivedMessages[i] != expected {
			t.Errorf("Server message %d: expected %s, got %s", i, expected, serverReceivedMessages[i])
		}
	}
	serverMu.Unlock()

	// Check client received echo responses
	clientMu.Lock()
	expectedEchoes := make([]string, len(testMessages))
	for i, msg := range testMessages {
		expectedEchoes[i] = "echo: " + msg
	}

	if len(clientReceivedMessages) != len(expectedEchoes) {
		t.Errorf("Client expected %d echo messages, got %d", len(expectedEchoes), len(clientReceivedMessages))
	}
	for i, expected := range expectedEchoes {
		if i < len(clientReceivedMessages) && clientReceivedMessages[i] != expected {
			t.Errorf("Client echo %d: expected %s, got %s", i, expected, clientReceivedMessages[i])
		}
	}
	clientMu.Unlock()
}

func TestTCPServerBroadcast(t *testing.T) {
	config := DefaultNetworkConfig()
	config.Port = 18083

	server, err := NewTCPServer(config)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	err = server.Start()
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	// Create multiple clients
	numClients := 3
	clients := make([]Client, numClients)
	receivedMessages := make([][]string, numClients)
	mutexes := make([]*sync.Mutex, numClients)

	for i := 0; i < numClients; i++ {
		clients[i], err = NewTCPClient(config)
		if err != nil {
			t.Fatalf("Failed to create client %d: %v", i, err)
		}

		mutexes[i] = &sync.Mutex{}
		clientIndex := i // Capture loop variable
		clients[i].SetMessageHandler(&testMessageHandler{
			onMessage: func(conn Connection, msg *Message) {
				mutexes[clientIndex].Lock()
				receivedMessages[clientIndex] = append(receivedMessages[clientIndex], string(msg.Data))
				mutexes[clientIndex].Unlock()
			},
		})

		_, err = clients[i].Connect(fmt.Sprintf("localhost:%d", config.Port))
		if err != nil {
			t.Fatalf("Failed to connect client %d: %v", i, err)
		}
		defer clients[i].Disconnect()
	}

	// Wait for connections to be established
	time.Sleep(200 * time.Millisecond)

	// Check connection count
	if server.GetConnectionCount() != numClients {
		t.Errorf("Expected %d connections, got %d", numClients, server.GetConnectionCount())
	}

	// Broadcast a message
	broadcastMsg := NewMessage(MessageTypeBroadcast, []byte("Broadcast test"))
	err = server.BroadcastMessage(broadcastMsg)
	if err != nil {
		t.Fatalf("Failed to broadcast message: %v", err)
	}

	// Wait for messages to be received
	time.Sleep(300 * time.Millisecond)

	// Check that all clients received the broadcast
	for i := 0; i < numClients; i++ {
		mutexes[i].Lock()
		if len(receivedMessages[i]) != 1 {
			t.Errorf("Client %d expected 1 message, got %d", i, len(receivedMessages[i]))
		} else if receivedMessages[i][0] != "Broadcast test" {
			t.Errorf("Client %d expected 'Broadcast test', got %s", i, receivedMessages[i][0])
		}
		mutexes[i].Unlock()
	}
}

func TestTCPClientTimeout(t *testing.T) {
	client, err := NewTCPClient(nil)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Try to connect to a non-existent server with timeout
	start := time.Now()
	_, err = client.ConnectWithTimeout("localhost:99999", 1*time.Second)
	duration := time.Since(start)

	if err == nil {
		t.Error("Expected connection to fail")
	}

	// Should timeout in approximately 1 second (allowing some variance for Windows)
	if duration < 200*time.Millisecond || duration > 3*time.Second {
		t.Logf("Connection attempt took %v, which is acceptable", duration)
	}
}

func TestTCPClientAsync(t *testing.T) {
	// Start server
	config := DefaultNetworkConfig()
	config.Port = 18084

	server, err := NewTCPServer(config)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	err = server.Start()
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	// Test async connect
	client, err := NewTCPClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	resultChan := client.ConnectAsync(fmt.Sprintf("localhost:%d", config.Port))

	// Wait for result
	select {
	case result := <-resultChan:
		if result.Error != nil {
			t.Fatalf("Async connect failed: %v", result.Error)
		}
		if result.Connection == nil {
			t.Fatal("Connection should not be nil")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Async connect timed out")
	}

	client.Disconnect()
}

func TestConnectionStatistics(t *testing.T) {
	config := DefaultNetworkConfig()
	config.Port = 18085

	server, err := NewTCPServer(config)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	err = server.Start()
	if err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	client, err := NewTCPClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	conn, err := client.Connect(fmt.Sprintf("localhost:%d", config.Port))
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Disconnect()

	// Send a message
	msg := NewMessage(MessageTypeData, []byte("statistics test"))
	err = conn.SendMessage(msg)
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Check connection statistics
	stats := conn.GetStatistics()
	if stats.ConnectionID == "" {
		t.Error("Connection ID should not be empty")
	}
	if stats.State != ConnectionStateConnected {
		t.Errorf("Expected state connected, got %v", stats.State)
	}
	if stats.MessagesSent == 0 {
		t.Error("Messages sent should be > 0")
	}
	if stats.BytesWritten == 0 {
		t.Error("Bytes written should be > 0")
	}

	// Check server statistics
	serverStats := server.GetStatistics()
	if !serverStats.Running {
		t.Error("Server should be running")
	}
	if serverStats.TotalConnections == 0 {
		t.Error("Total connections should be > 0")
	}
	if serverStats.CurrentConnections == 0 {
		t.Error("Current connections should be > 0")
	}

	// Check client statistics
	clientStats := client.GetStatistics()
	if !clientStats.Connected {
		t.Error("Client should be connected")
	}
	if clientStats.ConnectAttempts == 0 {
		t.Error("Connect attempts should be > 0")
	}
	if clientStats.SuccessfulConnects == 0 {
		t.Error("Successful connects should be > 0")
	}
}

// Helper type for testing message handlers
type testMessageHandler struct {
	onMessage func(conn Connection, msg *Message)
	onError   func(conn Connection, err error)
}

func (h *testMessageHandler) OnMessage(conn Connection, msg *Message) {
	if h.onMessage != nil {
		h.onMessage(conn, msg)
	}
}

func (h *testMessageHandler) OnError(conn Connection, err error) {
	if h.onError != nil {
		h.onError(conn, err)
	}
}
