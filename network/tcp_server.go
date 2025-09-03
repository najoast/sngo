// Package network provides TCP server implementation
package network

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// tcpServer implements the Server interface for TCP
type tcpServer struct {
	config   *NetworkConfig
	listener net.Listener
	running  int32 // atomic flag

	// Event handlers
	connHandler ConnectionHandler
	msgHandler  MessageHandler

	// Connection management
	connections    map[string]Connection
	connectionsMu  sync.RWMutex
	connectionChan chan Connection

	// Synchronization
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Statistics
	totalConnections   int64
	currentConnections int64
	totalMessages      int64
	startTime          time.Time
}

// NewTCPServer creates a new TCP server
func NewTCPServer(config *NetworkConfig) (Server, error) {
	if config == nil {
		config = DefaultNetworkConfig()
	}

	// Validate configuration
	if config.Protocol != ProtocolTCP {
		return nil, fmt.Errorf("invalid protocol for TCP server: %s", config.Protocol)
	}

	ctx, cancel := context.WithCancel(context.Background())

	server := &tcpServer{
		config:         config,
		connections:    make(map[string]Connection),
		connectionChan: make(chan Connection, 100),
		ctx:            ctx,
		cancel:         cancel,
		startTime:      time.Now(),
	}

	return server, nil
}

// Start starts the TCP server
func (ts *tcpServer) Start() error {
	if !atomic.CompareAndSwapInt32(&ts.running, 0, 1) {
		return fmt.Errorf("server is already running")
	}

	// Create listener
	address := fmt.Sprintf("%s:%d", ts.config.Address, ts.config.Port)
	listener, err := net.Listen(string(ts.config.Protocol), address)
	if err != nil {
		atomic.StoreInt32(&ts.running, 0)
		return fmt.Errorf("failed to listen on %s: %w", address, err)
	}

	ts.listener = listener

	// Start accept goroutine
	ts.wg.Add(1)
	go ts.acceptLoop()

	// Start connection handler if set
	if ts.connHandler != nil {
		ts.wg.Add(1)
		go ts.connectionHandlerLoop()
	}

	fmt.Printf("TCP server started on %s\n", address)
	return nil
}

// Stop stops the TCP server gracefully
func (ts *tcpServer) Stop() error {
	if !atomic.CompareAndSwapInt32(&ts.running, 1, 0) {
		return nil // Already stopped
	}

	// Cancel context
	ts.cancel()

	// Close listener
	if ts.listener != nil {
		ts.listener.Close()
	}

	// Wait for goroutines to finish first
	ts.wg.Wait()

	// Then close connection channel
	close(ts.connectionChan)

	// Close all connections
	ts.connectionsMu.Lock()
	for _, conn := range ts.connections {
		conn.Close()
	}
	ts.connectionsMu.Unlock()

	fmt.Println("TCP server stopped")
	return nil
}

// Listen returns the listening address
func (ts *tcpServer) Listen() net.Addr {
	if ts.listener == nil {
		return nil
	}
	return ts.listener.Addr()
}

// AcceptConnection waits for and returns new connections
func (ts *tcpServer) AcceptConnection(ctx context.Context) (Connection, error) {
	select {
	case conn, ok := <-ts.connectionChan:
		if !ok {
			return nil, fmt.Errorf("server is shutting down")
		}
		return conn, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-ts.ctx.Done():
		return nil, fmt.Errorf("server is shutting down")
	}
}

// SetConnectionHandler sets the handler for new connections
func (ts *tcpServer) SetConnectionHandler(handler ConnectionHandler) {
	ts.connHandler = handler

	// Start connection handler loop if not already running
	if atomic.LoadInt32(&ts.running) == 1 && handler != nil {
		ts.wg.Add(1)
		go ts.connectionHandlerLoop()
	}
}

// SetMessageHandler sets the handler for incoming messages
func (ts *tcpServer) SetMessageHandler(handler MessageHandler) {
	ts.msgHandler = handler
}

// GetActiveConnections returns all active connections
func (ts *tcpServer) GetActiveConnections() []Connection {
	ts.connectionsMu.RLock()
	defer ts.connectionsMu.RUnlock()

	connections := make([]Connection, 0, len(ts.connections))
	for _, conn := range ts.connections {
		connections = append(connections, conn)
	}

	return connections
}

// GetConnectionCount returns the number of active connections
func (ts *tcpServer) GetConnectionCount() int {
	return int(atomic.LoadInt64(&ts.currentConnections))
}

// GetStatistics returns server statistics
func (ts *tcpServer) GetStatistics() ServerStatistics {
	return ServerStatistics{
		Address:            ts.Listen().String(),
		Protocol:           string(ts.config.Protocol),
		Running:            atomic.LoadInt32(&ts.running) == 1,
		StartTime:          ts.startTime,
		Uptime:             time.Since(ts.startTime),
		TotalConnections:   atomic.LoadInt64(&ts.totalConnections),
		CurrentConnections: atomic.LoadInt64(&ts.currentConnections),
		TotalMessages:      atomic.LoadInt64(&ts.totalMessages),
	}
}

// BroadcastMessage broadcasts a message to all connections
func (ts *tcpServer) BroadcastMessage(msg *Message) error {
	if msg == nil {
		return fmt.Errorf("message is nil")
	}

	connections := ts.GetActiveConnections()
	if len(connections) == 0 {
		return nil
	}

	var wg sync.WaitGroup
	errorChan := make(chan error, len(connections))

	for _, conn := range connections {
		wg.Add(1)
		go func(c Connection) {
			defer wg.Done()
			if err := c.SendMessage(msg); err != nil {
				errorChan <- fmt.Errorf("failed to send to %s: %w", c.ID(), err)
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
		return fmt.Errorf("broadcast failed for %d connections: %v", len(errors), errors)
	}

	return nil
}

// Private methods

// acceptLoop accepts incoming connections
func (ts *tcpServer) acceptLoop() {
	defer ts.wg.Done()

	for {
		// Check if server is shutting down
		select {
		case <-ts.ctx.Done():
			return
		default:
		}

		// Accept connection
		conn, err := ts.listener.Accept()
		if err != nil {
			// Check if this is due to server shutdown
			select {
			case <-ts.ctx.Done():
				return
			default:
				fmt.Printf("Failed to accept connection: %v\n", err)
				continue
			}
		}

		// Check connection limit
		if ts.config.MaxConnections > 0 {
			currentCount := atomic.LoadInt64(&ts.currentConnections)
			if currentCount >= int64(ts.config.MaxConnections) {
				fmt.Printf("Connection limit reached (%d), rejecting new connection from %s\n",
					ts.config.MaxConnections, conn.RemoteAddr())
				conn.Close()
				continue
			}
		}

		// Configure TCP connection
		if tcpConn, ok := conn.(*net.TCPConn); ok {
			if ts.config.KeepAlive {
				tcpConn.SetKeepAlive(true)
				tcpConn.SetKeepAlivePeriod(ts.config.KeepAliveInterval)
			}
		}

		// Create connection wrapper
		connection := NewTCPConnection(conn)

		// Configure timeouts
		connection.SetReadTimeout(ts.config.ReadTimeout)
		connection.SetWriteTimeout(ts.config.WriteTimeout)

		// Add to connections map
		ts.addConnection(connection)

		// Start message handler for this connection
		if ts.msgHandler != nil {
			ts.wg.Add(1)
			go ts.handleConnection(connection)
		}

		// Send to connection channel for external processing
		// Check context again before sending
		select {
		case <-ts.ctx.Done():
			connection.Close()
			return
		default:
		}

		select {
		case ts.connectionChan <- connection:
		case <-ts.ctx.Done():
			connection.Close()
			return
		default:
			// Channel is full, handle directly if possible
			if ts.connHandler != nil {
				go ts.connHandler.OnConnect(connection)
			}
		}

		// Update statistics
		atomic.AddInt64(&ts.totalConnections, 1)
	}
}

// connectionHandlerLoop processes connections from the channel
func (ts *tcpServer) connectionHandlerLoop() {
	defer ts.wg.Done()

	for {
		select {
		case conn, ok := <-ts.connectionChan:
			if !ok {
				return // Channel closed
			}
			if ts.connHandler != nil {
				ts.connHandler.OnConnect(conn)
			}
		case <-ts.ctx.Done():
			return
		}
	}
}

// handleConnection handles messages for a single connection
func (ts *tcpServer) handleConnection(conn Connection) {
	defer ts.wg.Done()
	defer ts.removeConnection(conn.ID())

	// Notify connection handler
	if ts.connHandler != nil {
		defer func() {
			ts.connHandler.OnDisconnect(conn, nil)
		}()
	}

	for {
		// Check if server is shutting down
		select {
		case <-ts.ctx.Done():
			return
		default:
		}

		// Read message
		msg, err := conn.ReadMessage()
		if err != nil {
			// Connection error
			if ts.connHandler != nil {
				ts.connHandler.OnError(conn, err)
			}
			return
		}

		// Process message
		if ts.msgHandler != nil {
			ts.msgHandler.OnMessage(conn, msg)
		}

		// Update statistics
		atomic.AddInt64(&ts.totalMessages, 1)
	}
}

// addConnection adds a connection to the server
func (ts *tcpServer) addConnection(conn Connection) {
	ts.connectionsMu.Lock()
	defer ts.connectionsMu.Unlock()

	ts.connections[conn.ID()] = conn
	atomic.AddInt64(&ts.currentConnections, 1)
}

// removeConnection removes a connection from the server
func (ts *tcpServer) removeConnection(connID string) {
	ts.connectionsMu.Lock()
	defer ts.connectionsMu.Unlock()

	if _, exists := ts.connections[connID]; exists {
		delete(ts.connections, connID)
		atomic.AddInt64(&ts.currentConnections, -1)
	}
}

// ServerStatistics holds statistics for a server
type ServerStatistics struct {
	Address            string        `json:"address"`
	Protocol           string        `json:"protocol"`
	Running            bool          `json:"running"`
	StartTime          time.Time     `json:"start_time"`
	Uptime             time.Duration `json:"uptime"`
	TotalConnections   int64         `json:"total_connections"`
	CurrentConnections int64         `json:"current_connections"`
	TotalMessages      int64         `json:"total_messages"`
}

// String returns the string representation of server statistics
func (ss ServerStatistics) String() string {
	return fmt.Sprintf("Server[%s] Protocol=%s Running=%t Uptime=%s Connections=%d/%d Messages=%d",
		ss.Address, ss.Protocol, ss.Running, ss.Uptime.Truncate(time.Second),
		ss.CurrentConnections, ss.TotalConnections, ss.TotalMessages)
}
