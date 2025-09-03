// Package network provides TCP client implementation
package network

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// tcpClient implements the Client interface for TCP
type tcpClient struct {
	config *NetworkConfig
	conn   Connection

	// Event handlers
	msgHandler MessageHandler

	// Auto-reconnect
	autoReconnect        bool
	reconnectInterval    time.Duration
	maxReconnectAttempts int
	currentAttempt       int

	// State management
	connecting   int32 // atomic flag
	connected    int32 // atomic flag
	reconnecting int32 // atomic flag

	// Synchronization
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.RWMutex

	// Target address
	targetAddress string

	// Statistics
	connectAttempts    int64
	successfulConnects int64
	totalMessages      int64
	startTime          time.Time
}

// NewTCPClient creates a new TCP client
func NewTCPClient(config *NetworkConfig) (Client, error) {
	if config == nil {
		config = DefaultNetworkConfig()
	}

	// Validate configuration
	if config.Protocol != ProtocolTCP {
		return nil, fmt.Errorf("invalid protocol for TCP client: %s", config.Protocol)
	}

	ctx, cancel := context.WithCancel(context.Background())

	client := &tcpClient{
		config:               config,
		ctx:                  ctx,
		cancel:               cancel,
		reconnectInterval:    config.ReconnectInterval,
		maxReconnectAttempts: config.MaxReconnectAttempts,
		startTime:            time.Now(),
	}

	return client, nil
}

// Connect connects to the remote server
func (tc *tcpClient) Connect(address string) (Connection, error) {
	return tc.ConnectWithTimeout(address, 30*time.Second)
}

// ConnectWithTimeout connects with a timeout
func (tc *tcpClient) ConnectWithTimeout(address string, timeout time.Duration) (Connection, error) {
	if !atomic.CompareAndSwapInt32(&tc.connecting, 0, 1) {
		return nil, fmt.Errorf("connection already in progress")
	}
	defer atomic.StoreInt32(&tc.connecting, 0)

	tc.mu.Lock()
	tc.targetAddress = address
	tc.mu.Unlock()

	// Increment attempt counter
	atomic.AddInt64(&tc.connectAttempts, 1)

	// Create dialer with timeout
	dialer := &net.Dialer{
		Timeout: timeout,
	}

	// Connect to remote server
	conn, err := dialer.Dial(string(tc.config.Protocol), address)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", address, err)
	}

	// Configure TCP connection
	if tcpConn, ok := conn.(*net.TCPConn); ok {
		if tc.config.KeepAlive {
			tcpConn.SetKeepAlive(true)
			tcpConn.SetKeepAlivePeriod(tc.config.KeepAliveInterval)
		}
	}

	// Create connection wrapper
	connection := NewTCPConnection(conn)

	// Configure timeouts
	connection.SetReadTimeout(tc.config.ReadTimeout)
	connection.SetWriteTimeout(tc.config.WriteTimeout)

	// Update state
	tc.mu.Lock()
	tc.conn = connection
	tc.mu.Unlock()

	atomic.StoreInt32(&tc.connected, 1)
	atomic.AddInt64(&tc.successfulConnects, 1)

	// Start message handler
	if tc.msgHandler != nil {
		tc.wg.Add(1)
		go tc.messageLoop()
	}

	// Start auto-reconnect monitoring
	if tc.autoReconnect {
		tc.wg.Add(1)
		go tc.reconnectLoop()
	}

	fmt.Printf("TCP client connected to %s\n", address)
	return connection, nil
}

// ConnectAsync connects asynchronously
func (tc *tcpClient) ConnectAsync(address string) <-chan ConnectionResult {
	resultChan := make(chan ConnectionResult, 1)

	go func() {
		conn, err := tc.Connect(address)
		resultChan <- ConnectionResult{
			Connection: conn,
			Error:      err,
		}
		close(resultChan)
	}()

	return resultChan
}

// Disconnect disconnects from the server
func (tc *tcpClient) Disconnect() error {
	// Cancel context to stop all goroutines
	tc.cancel()

	// Close connection
	tc.mu.RLock()
	conn := tc.conn
	tc.mu.RUnlock()

	if conn != nil {
		err := conn.Close()
		atomic.StoreInt32(&tc.connected, 0)

		tc.mu.Lock()
		tc.conn = nil
		tc.mu.Unlock()

		// Wait for goroutines to finish
		tc.wg.Wait()

		fmt.Println("TCP client disconnected")
		return err
	}

	return nil
}

// GetConnection returns the current connection
func (tc *tcpClient) GetConnection() Connection {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return tc.conn
}

// SetAutoReconnect enables/disables auto reconnection
func (tc *tcpClient) SetAutoReconnect(enabled bool, interval time.Duration) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	tc.autoReconnect = enabled
	if interval > 0 {
		tc.reconnectInterval = interval
	}
}

// SetMessageHandler sets the handler for incoming messages
func (tc *tcpClient) SetMessageHandler(handler MessageHandler) {
	tc.msgHandler = handler

	// Start message loop if connected and not already running
	if atomic.LoadInt32(&tc.connected) == 1 && handler != nil {
		tc.wg.Add(1)
		go tc.messageLoop()
	}
}

// IsConnected returns true if the client is connected
func (tc *tcpClient) IsConnected() bool {
	return atomic.LoadInt32(&tc.connected) == 1
}

// GetStatistics returns client statistics
func (tc *tcpClient) GetStatistics() ClientStatistics {
	tc.mu.RLock()
	targetAddr := tc.targetAddress
	tc.mu.RUnlock()

	var connStats ConnectionStatistics
	conn := tc.GetConnection()
	if conn != nil {
		connStats = conn.GetStatistics()
	}

	return ClientStatistics{
		TargetAddress:      targetAddr,
		Protocol:           string(tc.config.Protocol),
		Connected:          tc.IsConnected(),
		StartTime:          tc.startTime,
		Uptime:             time.Since(tc.startTime),
		ConnectAttempts:    atomic.LoadInt64(&tc.connectAttempts),
		SuccessfulConnects: atomic.LoadInt64(&tc.successfulConnects),
		TotalMessages:      atomic.LoadInt64(&tc.totalMessages),
		AutoReconnect:      tc.autoReconnect,
		ReconnectInterval:  tc.reconnectInterval,
		ConnectionStats:    connStats,
	}
}

// SendMessage sends a message through the client connection
func (tc *tcpClient) SendMessage(msg *Message) error {
	conn := tc.GetConnection()
	if conn == nil {
		return fmt.Errorf("client is not connected")
	}

	err := conn.SendMessage(msg)
	if err == nil {
		atomic.AddInt64(&tc.totalMessages, 1)
	}

	return err
}

// Private methods

// messageLoop handles incoming messages
func (tc *tcpClient) messageLoop() {
	defer tc.wg.Done()

	conn := tc.GetConnection()
	if conn == nil {
		return
	}

	for {
		// Check if client is shutting down
		select {
		case <-tc.ctx.Done():
			return
		default:
		}

		// Check if still connected
		if !tc.IsConnected() {
			return
		}

		// Read message
		msg, err := conn.ReadMessage()
		if err != nil {
			// Connection error
			if tc.msgHandler != nil {
				tc.msgHandler.OnError(conn, err)
			}

			// Mark as disconnected
			atomic.StoreInt32(&tc.connected, 0)
			return
		}

		// Process message
		if tc.msgHandler != nil {
			tc.msgHandler.OnMessage(conn, msg)
		}

		// Update statistics
		atomic.AddInt64(&tc.totalMessages, 1)
	}
}

// reconnectLoop handles auto-reconnection
func (tc *tcpClient) reconnectLoop() {
	defer tc.wg.Done()

	ticker := time.NewTicker(tc.reconnectInterval)
	defer ticker.Stop()

	for {
		select {
		case <-tc.ctx.Done():
			return
		case <-ticker.C:
			// Check if we need to reconnect
			if !tc.IsConnected() && atomic.LoadInt32(&tc.reconnecting) == 0 {
				tc.attemptReconnect()
			}
		}
	}
}

// attemptReconnect attempts to reconnect to the server
func (tc *tcpClient) attemptReconnect() {
	if !atomic.CompareAndSwapInt32(&tc.reconnecting, 0, 1) {
		return // Already reconnecting
	}
	defer atomic.StoreInt32(&tc.reconnecting, 0)

	tc.mu.RLock()
	targetAddr := tc.targetAddress
	tc.mu.RUnlock()

	if targetAddr == "" {
		return // No target address set
	}

	// Check reconnect attempts limit
	if tc.maxReconnectAttempts > 0 && tc.currentAttempt >= tc.maxReconnectAttempts {
		fmt.Printf("Max reconnect attempts (%d) reached for %s\n", tc.maxReconnectAttempts, targetAddr)
		return
	}

	tc.currentAttempt++
	fmt.Printf("Attempting to reconnect to %s (attempt %d)\n", targetAddr, tc.currentAttempt)

	_, err := tc.ConnectWithTimeout(targetAddr, 10*time.Second)
	if err != nil {
		fmt.Printf("Reconnect attempt %d failed: %v\n", tc.currentAttempt, err)
	} else {
		fmt.Printf("Reconnected successfully to %s\n", targetAddr)
		tc.currentAttempt = 0 // Reset attempt counter on success
	}
}

// ClientStatistics holds statistics for a client
type ClientStatistics struct {
	TargetAddress      string               `json:"target_address"`
	Protocol           string               `json:"protocol"`
	Connected          bool                 `json:"connected"`
	StartTime          time.Time            `json:"start_time"`
	Uptime             time.Duration        `json:"uptime"`
	ConnectAttempts    int64                `json:"connect_attempts"`
	SuccessfulConnects int64                `json:"successful_connects"`
	TotalMessages      int64                `json:"total_messages"`
	AutoReconnect      bool                 `json:"auto_reconnect"`
	ReconnectInterval  time.Duration        `json:"reconnect_interval"`
	ConnectionStats    ConnectionStatistics `json:"connection_stats"`
}

// String returns the string representation of client statistics
func (cs ClientStatistics) String() string {
	return fmt.Sprintf("Client[%s] Protocol=%s Connected=%t Uptime=%s Attempts=%d/%d Messages=%d AutoReconnect=%t",
		cs.TargetAddress, cs.Protocol, cs.Connected, cs.Uptime.Truncate(time.Second),
		cs.SuccessfulConnects, cs.ConnectAttempts, cs.TotalMessages, cs.AutoReconnect)
}
