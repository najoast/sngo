// Package network provides connection management implementation
package network

import (
	"fmt"
	"sync"
	"time"
)

// connectionManager implements the ConnectionManager interface
type connectionManager struct {
	connections map[string]Connection
	mu          sync.RWMutex

	// Heartbeat management
	heartbeatEnabled  bool
	heartbeatInterval time.Duration
	heartbeatTicker   *time.Ticker
	heartbeatStopChan chan struct{}
	heartbeatWg       sync.WaitGroup

	// Statistics
	totalConnections int64
	totalMessages    int64
	startTime        time.Time
}

// NewConnectionManager creates a new connection manager
func NewConnectionManager() ConnectionManager {
	return &connectionManager{
		connections: make(map[string]Connection),
		startTime:   time.Now(),
	}
}

// AddConnection adds a connection to the manager
func (cm *connectionManager) AddConnection(conn Connection) error {
	if conn == nil {
		return fmt.Errorf("connection is nil")
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	connID := conn.ID()
	if _, exists := cm.connections[connID]; exists {
		return fmt.Errorf("connection %s already exists", connID)
	}

	cm.connections[connID] = conn
	cm.totalConnections++

	return nil
}

// RemoveConnection removes a connection from the manager
func (cm *connectionManager) RemoveConnection(connID string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	conn, exists := cm.connections[connID]
	if !exists {
		return fmt.Errorf("connection %s not found", connID)
	}

	// Close the connection
	conn.Close()

	// Remove from map
	delete(cm.connections, connID)

	return nil
}

// GetConnection gets a connection by ID
func (cm *connectionManager) GetConnection(connID string) (Connection, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	conn, exists := cm.connections[connID]
	return conn, exists
}

// GetAllConnections returns all managed connections
func (cm *connectionManager) GetAllConnections() []Connection {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	connections := make([]Connection, 0, len(cm.connections))
	for _, conn := range cm.connections {
		connections = append(connections, conn)
	}

	return connections
}

// BroadcastMessage broadcasts a message to all connections
func (cm *connectionManager) BroadcastMessage(msg *Message) error {
	if msg == nil {
		return fmt.Errorf("message is nil")
	}

	connections := cm.GetAllConnections()
	if len(connections) == 0 {
		return nil
	}

	var wg sync.WaitGroup
	errorChan := make(chan error, len(connections))
	successCount := 0

	for _, conn := range connections {
		wg.Add(1)
		go func(c Connection) {
			defer wg.Done()
			if err := c.SendMessage(msg); err != nil {
				errorChan <- fmt.Errorf("failed to send to %s: %w", c.ID(), err)
			} else {
				successCount++
			}
		}(conn)
	}

	wg.Wait()
	close(errorChan)

	// Collect errors
	var errors []error
	for err := range errorChan {
		errors = append(errors, err)
	}

	cm.totalMessages += int64(successCount)

	if len(errors) > 0 {
		return fmt.Errorf("broadcast failed for %d/%d connections: %v",
			len(errors), len(connections), errors)
	}

	return nil
}

// BroadcastData broadcasts raw data to all connections
func (cm *connectionManager) BroadcastData(data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("data is empty")
	}

	connections := cm.GetAllConnections()
	if len(connections) == 0 {
		return nil
	}

	var wg sync.WaitGroup
	errorChan := make(chan error, len(connections))
	successCount := 0

	for _, conn := range connections {
		wg.Add(1)
		go func(c Connection) {
			defer wg.Done()
			if err := c.Send(data); err != nil {
				errorChan <- fmt.Errorf("failed to send to %s: %w", c.ID(), err)
			} else {
				successCount++
			}
		}(conn)
	}

	wg.Wait()
	close(errorChan)

	// Collect errors
	var errors []error
	for err := range errorChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("broadcast failed for %d/%d connections: %v",
			len(errors), len(connections), errors)
	}

	return nil
}

// GetConnectionCount returns the number of managed connections
func (cm *connectionManager) GetConnectionCount() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	return len(cm.connections)
}

// StartHeartbeat starts heartbeat for all connections
func (cm *connectionManager) StartHeartbeat(interval time.Duration) error {
	if interval <= 0 {
		return fmt.Errorf("invalid heartbeat interval: %v", interval)
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.heartbeatEnabled {
		return fmt.Errorf("heartbeat is already running")
	}

	cm.heartbeatEnabled = true
	cm.heartbeatInterval = interval
	cm.heartbeatTicker = time.NewTicker(interval)
	cm.heartbeatStopChan = make(chan struct{})

	cm.heartbeatWg.Add(1)
	go cm.heartbeatLoop()

	return nil
}

// StopHeartbeat stops heartbeat
func (cm *connectionManager) StopHeartbeat() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if !cm.heartbeatEnabled {
		return nil // Already stopped
	}

	cm.heartbeatEnabled = false

	// Stop ticker
	if cm.heartbeatTicker != nil {
		cm.heartbeatTicker.Stop()
		cm.heartbeatTicker = nil
	}

	// Stop heartbeat loop
	if cm.heartbeatStopChan != nil {
		close(cm.heartbeatStopChan)
		cm.heartbeatStopChan = nil
	}

	// Wait for heartbeat loop to finish
	cm.heartbeatWg.Wait()

	return nil
}

// Cleanup removes inactive connections
func (cm *connectionManager) Cleanup(timeout time.Duration) int {
	if timeout <= 0 {
		return 0
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	cutoffTime := time.Now().Add(-timeout)
	removedCount := 0

	for connID, conn := range cm.connections {
		lastActivity := conn.GetLastActivity()
		if lastActivity.Before(cutoffTime) || conn.State() == ConnectionStateClosed {
			// Connection is inactive or closed, remove it
			conn.Close()
			delete(cm.connections, connID)
			removedCount++
		}
	}

	return removedCount
}

// GetStatistics returns connection manager statistics
func (cm *connectionManager) GetStatistics() ConnectionManagerStatistics {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// Count connections by state
	stateCount := make(map[ConnectionState]int)
	totalBytes := int64(0)
	totalMessages := int64(0)

	for _, conn := range cm.connections {
		state := conn.State()
		stateCount[state]++

		stats := conn.GetStatistics()
		totalBytes += stats.BytesRead + stats.BytesWritten
		totalMessages += stats.MessagesRead + stats.MessagesSent
	}

	return ConnectionManagerStatistics{
		TotalConnections:   cm.totalConnections,
		ActiveConnections:  int64(len(cm.connections)),
		ConnectionsByState: stateCount,
		TotalBytes:         totalBytes,
		TotalMessages:      totalMessages,
		HeartbeatEnabled:   cm.heartbeatEnabled,
		HeartbeatInterval:  cm.heartbeatInterval,
		StartTime:          cm.startTime,
		Uptime:             time.Since(cm.startTime),
	}
}

// SendMessageToConnection sends a message to a specific connection
func (cm *connectionManager) SendMessageToConnection(connID string, msg *Message) error {
	conn, exists := cm.GetConnection(connID)
	if !exists {
		return fmt.Errorf("connection %s not found", connID)
	}

	err := conn.SendMessage(msg)
	if err == nil {
		cm.mu.Lock()
		cm.totalMessages++
		cm.mu.Unlock()
	}

	return err
}

// GetConnectionsByState returns connections filtered by state
func (cm *connectionManager) GetConnectionsByState(state ConnectionState) []Connection {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	var connections []Connection
	for _, conn := range cm.connections {
		if conn.State() == state {
			connections = append(connections, conn)
		}
	}

	return connections
}

// CloseAllConnections closes all managed connections
func (cm *connectionManager) CloseAllConnections() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	var errors []error
	for connID, conn := range cm.connections {
		if err := conn.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close connection %s: %w", connID, err))
		}
	}

	// Clear the connections map
	cm.connections = make(map[string]Connection)

	// Stop heartbeat if running
	if cm.heartbeatEnabled {
		cm.heartbeatEnabled = false
		if cm.heartbeatTicker != nil {
			cm.heartbeatTicker.Stop()
		}
		if cm.heartbeatStopChan != nil {
			close(cm.heartbeatStopChan)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors occurred while closing connections: %v", errors)
	}

	return nil
}

// Private methods

// heartbeatLoop sends periodic heartbeat messages
func (cm *connectionManager) heartbeatLoop() {
	defer cm.heartbeatWg.Done()

	heartbeatMsg := NewHeartbeatMessage()

	for {
		select {
		case <-cm.heartbeatTicker.C:
			cm.sendHeartbeatToAll(heartbeatMsg)
		case <-cm.heartbeatStopChan:
			return
		}
	}
}

// sendHeartbeatToAll sends heartbeat to all active connections
func (cm *connectionManager) sendHeartbeatToAll(msg *Message) {
	connections := cm.GetConnectionsByState(ConnectionStateConnected)

	for _, conn := range connections {
		go func(c Connection) {
			if err := c.SendMessage(msg); err != nil {
				// Connection error, it will be cleaned up in the next cleanup cycle
			}
		}(conn)
	}
}

// ConnectionManagerStatistics holds statistics for the connection manager
type ConnectionManagerStatistics struct {
	TotalConnections   int64                   `json:"total_connections"`
	ActiveConnections  int64                   `json:"active_connections"`
	ConnectionsByState map[ConnectionState]int `json:"connections_by_state"`
	TotalBytes         int64                   `json:"total_bytes"`
	TotalMessages      int64                   `json:"total_messages"`
	HeartbeatEnabled   bool                    `json:"heartbeat_enabled"`
	HeartbeatInterval  time.Duration           `json:"heartbeat_interval"`
	StartTime          time.Time               `json:"start_time"`
	Uptime             time.Duration           `json:"uptime"`
}

// String returns the string representation of connection manager statistics
func (cms ConnectionManagerStatistics) String() string {
	return fmt.Sprintf("ConnectionManager Total=%d Active=%d Bytes=%d Messages=%d Heartbeat=%t Uptime=%s",
		cms.TotalConnections, cms.ActiveConnections, cms.TotalBytes, cms.TotalMessages,
		cms.HeartbeatEnabled, cms.Uptime.Truncate(time.Second))
}
