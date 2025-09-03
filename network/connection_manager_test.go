// Package network provides tests for connection manager
package network

import (
	"fmt"
	"net"
	"sync"
	"testing"
	"time"
)

func TestConnectionManager(t *testing.T) {
	manager := NewConnectionManager()

	t.Run("AddAndGetConnection", func(t *testing.T) {
		// Create a mock connection
		conn := &mockConnection{
			id:    "test-conn-1",
			state: ConnectionStateConnected,
		}

		// Add connection
		err := manager.AddConnection(conn)
		if err != nil {
			t.Fatalf("Failed to add connection: %v", err)
		}

		// Get connection
		retrieved, exists := manager.GetConnection("test-conn-1")
		if !exists {
			t.Fatal("Connection should exist")
		}
		if retrieved.ID() != "test-conn-1" {
			t.Errorf("Expected connection ID 'test-conn-1', got %s", retrieved.ID())
		}

		// Check connection count
		if manager.GetConnectionCount() != 1 {
			t.Errorf("Expected 1 connection, got %d", manager.GetConnectionCount())
		}
	})

	t.Run("RemoveConnection", func(t *testing.T) {
		manager := NewConnectionManager()
		conn := &mockConnection{
			id:    "test-conn-2",
			state: ConnectionStateConnected,
		}

		manager.AddConnection(conn)

		// Remove connection
		err := manager.RemoveConnection("test-conn-2")
		if err != nil {
			t.Fatalf("Failed to remove connection: %v", err)
		}

		// Check that connection is gone
		_, exists := manager.GetConnection("test-conn-2")
		if exists {
			t.Error("Connection should not exist after removal")
		}

		if manager.GetConnectionCount() != 0 {
			t.Errorf("Expected 0 connections, got %d", manager.GetConnectionCount())
		}

		// Check that connection was closed
		if !conn.closed {
			t.Error("Connection should be closed after removal")
		}
	})

	t.Run("GetAllConnections", func(t *testing.T) {
		manager := NewConnectionManager()

		// Add multiple connections
		connections := []*mockConnection{
			{id: "conn-1", state: ConnectionStateConnected},
			{id: "conn-2", state: ConnectionStateConnected},
			{id: "conn-3", state: ConnectionStateConnected},
		}

		for _, conn := range connections {
			manager.AddConnection(conn)
		}

		// Get all connections
		allConns := manager.GetAllConnections()
		if len(allConns) != len(connections) {
			t.Errorf("Expected %d connections, got %d", len(connections), len(allConns))
		}

		// Check that all connections are present
		connMap := make(map[string]bool)
		for _, conn := range allConns {
			connMap[conn.ID()] = true
		}

		for _, expected := range connections {
			if !connMap[expected.id] {
				t.Errorf("Missing connection %s", expected.id)
			}
		}
	})

	t.Run("BroadcastMessage", func(t *testing.T) {
		manager := NewConnectionManager()

		// Add connections
		connections := []*mockConnection{
			{id: "conn-1", state: ConnectionStateConnected},
			{id: "conn-2", state: ConnectionStateConnected},
			{id: "conn-3", state: ConnectionStateConnected},
		}

		for _, conn := range connections {
			manager.AddConnection(conn)
		}

		// Broadcast message
		msg := NewMessage(MessageTypeBroadcast, []byte("broadcast test"))
		err := manager.BroadcastMessage(msg)
		if err != nil {
			t.Fatalf("Failed to broadcast message: %v", err)
		}

		// Check that all connections received the message
		for _, conn := range connections {
			if len(conn.sentMessages) != 1 {
				t.Errorf("Connection %s expected 1 message, got %d", conn.id, len(conn.sentMessages))
			} else if string(conn.sentMessages[0].Data) != "broadcast test" {
				t.Errorf("Connection %s expected 'broadcast test', got %s",
					conn.id, string(conn.sentMessages[0].Data))
			}
		}
	})

	t.Run("BroadcastData", func(t *testing.T) {
		manager := NewConnectionManager()

		// Add connections
		connections := []*mockConnection{
			{id: "conn-1", state: ConnectionStateConnected},
			{id: "conn-2", state: ConnectionStateConnected},
		}

		for _, conn := range connections {
			manager.AddConnection(conn)
		}

		// Broadcast data
		testData := []byte("raw data broadcast")
		err := manager.BroadcastData(testData)
		if err != nil {
			t.Fatalf("Failed to broadcast data: %v", err)
		}

		// Check that all connections received the data
		for _, conn := range connections {
			if len(conn.sentData) != 1 {
				t.Errorf("Connection %s expected 1 data send, got %d", conn.id, len(conn.sentData))
			} else if string(conn.sentData[0]) != string(testData) {
				t.Errorf("Connection %s expected %s, got %s",
					conn.id, string(testData), string(conn.sentData[0]))
			}
		}
	})

	t.Run("Cleanup", func(t *testing.T) {
		manager := NewConnectionManager()

		// Add connections with different last activity times
		now := time.Now()
		connections := []*mockConnection{
			{
				id:           "active-conn",
				state:        ConnectionStateConnected,
				lastActivity: now, // Recent activity
			},
			{
				id:           "inactive-conn",
				state:        ConnectionStateConnected,
				lastActivity: now.Add(-10 * time.Minute), // Old activity
			},
			{
				id:           "closed-conn",
				state:        ConnectionStateClosed,
				lastActivity: now,
			},
		}

		for _, conn := range connections {
			manager.AddConnection(conn)
		}

		// Cleanup with 5 minute timeout
		removedCount := manager.Cleanup(5 * time.Minute)

		// Should remove inactive and closed connections
		if removedCount != 2 {
			t.Errorf("Expected 2 connections removed, got %d", removedCount)
		}

		// Check that only active connection remains
		if manager.GetConnectionCount() != 1 {
			t.Errorf("Expected 1 connection remaining, got %d", manager.GetConnectionCount())
		}

		remaining, exists := manager.GetConnection("active-conn")
		if !exists {
			t.Error("Active connection should still exist")
		}
		if remaining.ID() != "active-conn" {
			t.Errorf("Expected remaining connection to be 'active-conn', got %s", remaining.ID())
		}
	})
}

func TestConnectionManagerHeartbeat(t *testing.T) {
	manager := NewConnectionManager()

	// Add a connection
	conn := &mockConnection{
		id:    "heartbeat-conn",
		state: ConnectionStateConnected,
	}
	manager.AddConnection(conn)

	// Start heartbeat
	err := manager.StartHeartbeat(100 * time.Millisecond)
	if err != nil {
		t.Fatalf("Failed to start heartbeat: %v", err)
	}

	// Wait for some heartbeats
	time.Sleep(350 * time.Millisecond)

	// Stop heartbeat
	err = manager.StopHeartbeat()
	if err != nil {
		t.Fatalf("Failed to stop heartbeat: %v", err)
	}

	// Check that heartbeat messages were sent
	// Should have received at least 2-3 heartbeats in 350ms with 100ms interval
	if len(conn.sentMessages) < 2 {
		t.Errorf("Expected at least 2 heartbeat messages, got %d", len(conn.sentMessages))
	}

	// Check that messages are heartbeat type
	for i, msg := range conn.sentMessages {
		if msg.Type != MessageTypeHeartbeat {
			t.Errorf("Message %d expected heartbeat type, got %v", i, msg.Type)
		}
	}
}

func TestConnectionManagerConcurrency(t *testing.T) {
	manager := NewConnectionManager()

	// Concurrent operations
	var wg sync.WaitGroup
	numGoroutines := 10
	connectionsPerGoroutine := 5

	// Add connections concurrently
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(routineID int) {
			defer wg.Done()
			for j := 0; j < connectionsPerGoroutine; j++ {
				conn := &mockConnection{
					id:    fmt.Sprintf("conn-%d-%d", routineID, j),
					state: ConnectionStateConnected,
				}
				manager.AddConnection(conn)
			}
		}(i)
	}

	wg.Wait()

	// Check total count
	expectedTotal := numGoroutines * connectionsPerGoroutine
	if manager.GetConnectionCount() != expectedTotal {
		t.Errorf("Expected %d connections, got %d", expectedTotal, manager.GetConnectionCount())
	}

	// Broadcast concurrently
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(routineID int) {
			defer wg.Done()
			msg := NewMessage(MessageTypeData, []byte(fmt.Sprintf("msg-%d", routineID)))
			manager.BroadcastMessage(msg)
		}(i)
	}

	wg.Wait()

	// Remove connections concurrently
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(routineID int) {
			defer wg.Done()
			for j := 0; j < connectionsPerGoroutine; j++ {
				connID := fmt.Sprintf("conn-%d-%d", routineID, j)
				manager.RemoveConnection(connID)
			}
		}(i)
	}

	wg.Wait()

	// Check that all connections are removed
	if manager.GetConnectionCount() != 0 {
		t.Errorf("Expected 0 connections after removal, got %d", manager.GetConnectionCount())
	}
}

// Mock connection for testing
type mockConnection struct {
	id           string
	state        ConnectionState
	userData     interface{}
	lastActivity time.Time
	closed       bool
	sentMessages []*Message
	sentData     [][]byte
	mu           sync.Mutex
}

func (mc *mockConnection) ID() string {
	return mc.id
}

func (mc *mockConnection) RemoteAddr() net.Addr {
	return &mockAddr{address: "127.0.0.1:8080"}
}

func (mc *mockConnection) LocalAddr() net.Addr {
	return &mockAddr{address: "127.0.0.1:12345"}
}

func (mc *mockConnection) Send(data []byte) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if mc.closed {
		return fmt.Errorf("connection closed")
	}

	// Copy data to avoid race conditions
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)
	mc.sentData = append(mc.sentData, dataCopy)
	mc.lastActivity = time.Now()

	return nil
}

func (mc *mockConnection) SendMessage(msg *Message) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	if mc.closed {
		return fmt.Errorf("connection closed")
	}

	mc.sentMessages = append(mc.sentMessages, msg.Clone())
	mc.lastActivity = time.Now()

	return nil
}

func (mc *mockConnection) Close() error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.closed = true
	mc.state = ConnectionStateClosed
	return nil
}

func (mc *mockConnection) State() ConnectionState {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	return mc.state
}

func (mc *mockConnection) SetReadTimeout(timeout time.Duration) {}

func (mc *mockConnection) SetWriteTimeout(timeout time.Duration) {}

func (mc *mockConnection) GetLastActivity() time.Time {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	return mc.lastActivity
}

func (mc *mockConnection) GetUserData() interface{} {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	return mc.userData
}

func (mc *mockConnection) SetUserData(data interface{}) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.userData = data
}

func (mc *mockConnection) ReadMessage() (*Message, error) {
	return nil, fmt.Errorf("not implemented")
}

func (mc *mockConnection) GetStatistics() ConnectionStatistics {
	return ConnectionStatistics{
		ConnectionID: mc.id,
		State:        mc.state,
		LastActivity: mc.lastActivity,
	}
}

// Mock network address for testing
type mockAddr struct {
	address string
}

func (ma *mockAddr) Network() string {
	return "tcp"
}

func (ma *mockAddr) String() string {
	return ma.address
}
