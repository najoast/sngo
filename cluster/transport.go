package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// messageTransport implements the MessageTransport interface
type messageTransport struct {
	config   *ClusterConfig
	listener net.Listener
	handler  MessageHandler

	connections map[NodeID]*connection
	connMu      sync.RWMutex

	stats TransportStatistics

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	started int32 // atomic
}

// connection represents a connection to a remote node
type connection struct {
	nodeID  NodeID
	conn    net.Conn
	encoder *json.Encoder
	decoder *json.Decoder

	sendChan chan *ClusterMessage

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	lastActivity int64 // atomic
}

// NewMessageTransport creates a new message transport
func NewMessageTransport(config *ClusterConfig) MessageTransport {
	return &messageTransport{
		config:      config,
		connections: make(map[NodeID]*connection),
	}
}

func (mt *messageTransport) Start(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&mt.started, 0, 1) {
		return fmt.Errorf("transport already started")
	}

	mt.ctx, mt.cancel = context.WithCancel(ctx)

	// Start listening
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", mt.config.BindAddr, mt.config.BindPort))
	if err != nil {
		atomic.StoreInt32(&mt.started, 0)
		return fmt.Errorf("failed to start listener: %w", err)
	}

	mt.listener = listener

	// Start accept loop
	mt.wg.Add(1)
	go mt.acceptLoop()

	return nil
}

func (mt *messageTransport) Stop(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&mt.started, 1, 0) {
		return nil // Already stopped
	}

	// Close listener
	if mt.listener != nil {
		mt.listener.Close()
	}

	// Close all connections
	mt.connMu.Lock()
	for _, conn := range mt.connections {
		conn.close()
	}
	mt.connections = make(map[NodeID]*connection)
	mt.connMu.Unlock()

	// Cancel context and wait
	mt.cancel()
	mt.wg.Wait()

	return nil
}

func (mt *messageTransport) Send(ctx context.Context, nodeID NodeID, message *ClusterMessage) error {
	conn, err := mt.getConnection(nodeID)
	if err != nil {
		return fmt.Errorf("failed to get connection to %s: %w", nodeID, err)
	}

	// Set source
	message.From = mt.config.NodeID
	message.To = nodeID
	message.Timestamp = time.Now()

	// Send message
	select {
	case conn.sendChan <- message:
		atomic.AddInt64(&mt.stats.MessagesSent, 1)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(mt.config.MessageTimeout):
		return fmt.Errorf("send timeout")
	}
}

func (mt *messageTransport) Broadcast(ctx context.Context, message *ClusterMessage) error {
	mt.connMu.RLock()
	connections := make([]*connection, 0, len(mt.connections))
	for _, conn := range mt.connections {
		connections = append(connections, conn)
	}
	mt.connMu.RUnlock()

	// Set source
	message.From = mt.config.NodeID
	message.To = "" // Broadcast
	message.Timestamp = time.Now()

	// Send to all connections
	var errors []error
	for _, conn := range connections {
		select {
		case conn.sendChan <- message:
			atomic.AddInt64(&mt.stats.MessagesSent, 1)
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(mt.config.MessageTimeout):
			errors = append(errors, fmt.Errorf("broadcast timeout to %s", conn.nodeID))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("broadcast failed to %d nodes", len(errors))
	}

	return nil
}

func (mt *messageTransport) SetMessageHandler(handler MessageHandler) {
	mt.handler = handler
}

func (mt *messageTransport) GetStatistics() TransportStatistics {
	mt.connMu.RLock()
	connCount := len(mt.connections)
	mt.connMu.RUnlock()

	return TransportStatistics{
		MessagesSent:     atomic.LoadInt64(&mt.stats.MessagesSent),
		MessagesReceived: atomic.LoadInt64(&mt.stats.MessagesReceived),
		BytesSent:        atomic.LoadInt64(&mt.stats.BytesSent),
		BytesReceived:    atomic.LoadInt64(&mt.stats.BytesReceived),
		ConnectionsOpen:  connCount,
		ErrorCount:       atomic.LoadInt64(&mt.stats.ErrorCount),
		AverageLatency:   mt.stats.AverageLatency,
	}
}

// Connection management

func (mt *messageTransport) getConnection(nodeID NodeID) (*connection, error) {
	mt.connMu.RLock()
	conn, exists := mt.connections[nodeID]
	mt.connMu.RUnlock()

	if exists && conn.isActive() {
		return conn, nil
	}

	// Need to create new connection
	return mt.createConnection(nodeID)
}

func (mt *messageTransport) createConnection(nodeID NodeID) (*connection, error) {
	mt.connMu.Lock()
	defer mt.connMu.Unlock()

	// Double-check after acquiring lock
	if conn, exists := mt.connections[nodeID]; exists && conn.isActive() {
		return conn, nil
	}

	// TODO: Get node address from cluster manager
	// For now, assume address format
	address := fmt.Sprintf("localhost:%d", mt.config.BindPort)

	netConn, err := net.DialTimeout("tcp", address, 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to dial %s: %w", address, err)
	}

	conn := &connection{
		nodeID:   nodeID,
		conn:     netConn,
		encoder:  json.NewEncoder(netConn),
		decoder:  json.NewDecoder(netConn),
		sendChan: make(chan *ClusterMessage, 100),
	}

	conn.ctx, conn.cancel = context.WithCancel(mt.ctx)

	// Start connection goroutines
	conn.wg.Add(2)
	go mt.handleConnection(conn)
	go mt.sendLoop(conn)

	mt.connections[nodeID] = conn

	// Notify handler
	if mt.handler != nil {
		mt.handler.HandleConnectionEstablished(nodeID)
	}

	return conn, nil
}

func (mt *messageTransport) removeConnection(nodeID NodeID) {
	mt.connMu.Lock()
	defer mt.connMu.Unlock()

	if conn, exists := mt.connections[nodeID]; exists {
		conn.close()
		delete(mt.connections, nodeID)
	}
}

// Network loops

func (mt *messageTransport) acceptLoop() {
	defer mt.wg.Done()

	for {
		netConn, err := mt.listener.Accept()
		if err != nil {
			select {
			case <-mt.ctx.Done():
				return
			default:
				atomic.AddInt64(&mt.stats.ErrorCount, 1)
				continue
			}
		}

		// Handle new connection
		go mt.handleIncomingConnection(netConn)
	}
}

func (mt *messageTransport) handleIncomingConnection(netConn net.Conn) {
	defer netConn.Close()

	// Set read timeout for handshake
	netConn.SetReadDeadline(time.Now().Add(10 * time.Second))

	decoder := json.NewDecoder(netConn)
	encoder := json.NewEncoder(netConn)

	// Read handshake message
	var handshake ClusterMessage
	if err := decoder.Decode(&handshake); err != nil {
		atomic.AddInt64(&mt.stats.ErrorCount, 1)
		return
	}

	if handshake.Type != MessageTypeJoin {
		atomic.AddInt64(&mt.stats.ErrorCount, 1)
		return
	}

	nodeID := handshake.From

	// Send handshake response
	response := &ClusterMessage{
		ID:        generateMessageID(),
		Type:      MessageTypeJoin,
		From:      mt.config.NodeID,
		To:        nodeID,
		Timestamp: time.Now(),
	}

	if err := encoder.Encode(response); err != nil {
		atomic.AddInt64(&mt.stats.ErrorCount, 1)
		return
	}

	// Create connection
	conn := &connection{
		nodeID:   nodeID,
		conn:     netConn,
		encoder:  encoder,
		decoder:  decoder,
		sendChan: make(chan *ClusterMessage, 100),
	}

	conn.ctx, conn.cancel = context.WithCancel(mt.ctx)

	// Add to connections
	mt.connMu.Lock()
	mt.connections[nodeID] = conn
	mt.connMu.Unlock()

	// Start connection goroutines
	conn.wg.Add(2)
	go mt.handleConnection(conn)
	go mt.sendLoop(conn)

	// Notify handler
	if mt.handler != nil {
		mt.handler.HandleConnectionEstablished(nodeID)
	}

	// Wait for connection to close
	conn.wg.Wait()
}

func (mt *messageTransport) handleConnection(conn *connection) {
	defer conn.wg.Done()
	defer func() {
		mt.removeConnection(conn.nodeID)
		if mt.handler != nil {
			mt.handler.HandleConnectionLost(conn.nodeID, fmt.Errorf("connection closed"))
		}
	}()

	for {
		select {
		case <-conn.ctx.Done():
			return
		default:
			// Set read timeout
			conn.conn.SetReadDeadline(time.Now().Add(30 * time.Second))

			var message ClusterMessage
			if err := conn.decoder.Decode(&message); err != nil {
				atomic.AddInt64(&mt.stats.ErrorCount, 1)
				return
			}

			atomic.StoreInt64(&conn.lastActivity, time.Now().UnixNano())
			atomic.AddInt64(&mt.stats.MessagesReceived, 1)

			// Handle message
			if mt.handler != nil {
				if err := mt.handler.HandleMessage(conn.ctx, conn.nodeID, &message); err != nil {
					atomic.AddInt64(&mt.stats.ErrorCount, 1)
				}
			}
		}
	}
}

func (mt *messageTransport) sendLoop(conn *connection) {
	defer conn.wg.Done()

	for {
		select {
		case <-conn.ctx.Done():
			return
		case message := <-conn.sendChan:
			if err := conn.encoder.Encode(message); err != nil {
				atomic.AddInt64(&mt.stats.ErrorCount, 1)
				return
			}

			atomic.StoreInt64(&conn.lastActivity, time.Now().UnixNano())
		}
	}
}

// Connection methods

func (c *connection) isActive() bool {
	lastActivity := atomic.LoadInt64(&c.lastActivity)
	return time.Since(time.Unix(0, lastActivity)) < 60*time.Second
}

func (c *connection) close() {
	c.cancel()
	c.conn.Close()
	close(c.sendChan)
}

// Utility functions

func generateMessageID() string {
	return fmt.Sprintf("msg-%d", time.Now().UnixNano())
}
