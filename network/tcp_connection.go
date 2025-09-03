// Package network provides TCP connection implementation
package network

import (
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// tcpConnection implements the Connection interface for TCP connections
type tcpConnection struct {
	id           string
	conn         net.Conn
	state        int32 // ConnectionState as atomic int32
	userData     interface{}
	readTimeout  time.Duration
	writeTimeout time.Duration
	lastActivity int64 // Unix timestamp as atomic int64
	codec        MessageCodec

	// Synchronization
	mu       sync.RWMutex
	closed   int32 // atomic flag
	sendChan chan []byte

	// Statistics
	bytesRead    int64
	bytesWritten int64
	messagesRead int64
	messagesSent int64
}

// connectionIDCounter generates unique connection IDs
var connectionIDCounter int64

// NewTCPConnection creates a new TCP connection wrapper
func NewTCPConnection(conn net.Conn) Connection {
	id := fmt.Sprintf("tcp-%d", atomic.AddInt64(&connectionIDCounter, 1))

	tcpConn := &tcpConnection{
		id:           id,
		conn:         conn,
		state:        int32(ConnectionStateConnected),
		readTimeout:  30 * time.Second,
		writeTimeout: 30 * time.Second,
		lastActivity: time.Now().Unix(),
		codec:        NewBinaryMessageCodec(),
		sendChan:     make(chan []byte, 256), // Buffered channel for async sends
	}

	// Start the send goroutine
	go tcpConn.sendLoop()

	return tcpConn
}

// ID returns the connection ID
func (tc *tcpConnection) ID() string {
	return tc.id
}

// RemoteAddr returns the remote address
func (tc *tcpConnection) RemoteAddr() net.Addr {
	if tc.conn == nil {
		return nil
	}
	return tc.conn.RemoteAddr()
}

// LocalAddr returns the local address
func (tc *tcpConnection) LocalAddr() net.Addr {
	if tc.conn == nil {
		return nil
	}
	return tc.conn.LocalAddr()
}

// Send sends raw data through the connection
func (tc *tcpConnection) Send(data []byte) error {
	if tc.isClosed() {
		return fmt.Errorf("connection %s is closed", tc.id)
	}

	if len(data) == 0 {
		return nil
	}

	// Try to send through buffered channel (non-blocking)
	select {
	case tc.sendChan <- data:
		return nil
	default:
		// Channel is full, send directly (blocking)
		return tc.sendDirect(data)
	}
}

// SendMessage sends a structured message
func (tc *tcpConnection) SendMessage(msg *Message) error {
	if tc.isClosed() {
		return fmt.Errorf("connection %s is closed", tc.id)
	}

	if msg == nil {
		return fmt.Errorf("message is nil")
	}

	// Set connection ID
	msg.ConnectionID = tc.id

	// Encode message
	data, err := tc.codec.Encode(msg)
	if err != nil {
		return fmt.Errorf("failed to encode message: %w", err)
	}

	// Send encoded data
	err = tc.Send(data)
	if err == nil {
		atomic.AddInt64(&tc.messagesSent, 1)
	}

	return err
}

// Close closes the connection
func (tc *tcpConnection) Close() error {
	if !atomic.CompareAndSwapInt32(&tc.closed, 0, 1) {
		return nil // Already closed
	}

	// Update state
	atomic.StoreInt32(&tc.state, int32(ConnectionStateClosed))

	// Close send channel
	close(tc.sendChan)

	// Close underlying connection
	if tc.conn != nil {
		return tc.conn.Close()
	}

	return nil
}

// State returns the current connection state
func (tc *tcpConnection) State() ConnectionState {
	return ConnectionState(atomic.LoadInt32(&tc.state))
}

// SetReadTimeout sets the read timeout
func (tc *tcpConnection) SetReadTimeout(timeout time.Duration) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.readTimeout = timeout
}

// SetWriteTimeout sets the write timeout
func (tc *tcpConnection) SetWriteTimeout(timeout time.Duration) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.writeTimeout = timeout
}

// GetLastActivity returns the last activity timestamp
func (tc *tcpConnection) GetLastActivity() time.Time {
	timestamp := atomic.LoadInt64(&tc.lastActivity)
	return time.Unix(timestamp, 0)
}

// GetUserData returns user-defined data
func (tc *tcpConnection) GetUserData() interface{} {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return tc.userData
}

// SetUserData sets user-defined data
func (tc *tcpConnection) SetUserData(data interface{}) {
	tc.mu.Lock()
	defer tc.mu.Unlock()
	tc.userData = data
}

// ReadMessage reads a message from the connection
func (tc *tcpConnection) ReadMessage() (*Message, error) {
	if tc.isClosed() {
		return nil, fmt.Errorf("connection %s is closed", tc.id)
	}

	// Set read deadline
	tc.mu.RLock()
	readTimeout := tc.readTimeout
	tc.mu.RUnlock()

	if readTimeout > 0 {
		err := tc.conn.SetReadDeadline(time.Now().Add(readTimeout))
		if err != nil {
			return nil, fmt.Errorf("failed to set read deadline: %w", err)
		}
	}

	// Read message header first
	headerBuf := make([]byte, MessageHeaderSize)
	_, err := tc.readFull(headerBuf)
	if err != nil {
		return nil, fmt.Errorf("failed to read message header: %w", err)
	}

	// Decode header to get data length
	header, err := tc.codec.DecodeHeader(headerBuf)
	if err != nil {
		return nil, fmt.Errorf("failed to decode message header: %w", err)
	}

	// Read message data if any
	if cap(header.Data) > 0 {
		dataBuf := make([]byte, cap(header.Data))
		_, err = tc.readFull(dataBuf)
		if err != nil {
			return nil, fmt.Errorf("failed to read message data: %w", err)
		}
		header.Data = dataBuf
	}

	// Update statistics and activity
	atomic.AddInt64(&tc.messagesRead, 1)
	tc.updateActivity()

	// Set connection ID
	header.ConnectionID = tc.id

	return header, nil
}

// GetStatistics returns connection statistics
func (tc *tcpConnection) GetStatistics() ConnectionStatistics {
	return ConnectionStatistics{
		ConnectionID: tc.id,
		State:        tc.State(),
		BytesRead:    atomic.LoadInt64(&tc.bytesRead),
		BytesWritten: atomic.LoadInt64(&tc.bytesWritten),
		MessagesRead: atomic.LoadInt64(&tc.messagesRead),
		MessagesSent: atomic.LoadInt64(&tc.messagesSent),
		LastActivity: tc.GetLastActivity(),
		RemoteAddr:   tc.RemoteAddr().String(),
		LocalAddr:    tc.LocalAddr().String(),
	}
}

// Private methods

// isClosed checks if the connection is closed
func (tc *tcpConnection) isClosed() bool {
	return atomic.LoadInt32(&tc.closed) != 0
}

// sendLoop handles asynchronous sending
func (tc *tcpConnection) sendLoop() {
	defer func() {
		if r := recover(); r != nil {
			// Log the panic but don't crash the program
			fmt.Printf("sendLoop panic in connection %s: %v\n", tc.id, r)
		}
	}()

	for data := range tc.sendChan {
		if tc.isClosed() {
			break
		}

		err := tc.sendDirect(data)
		if err != nil {
			// Connection error, close the connection
			tc.Close()
			break
		}
	}
}

// sendDirect sends data directly through the connection
func (tc *tcpConnection) sendDirect(data []byte) error {
	if tc.conn == nil {
		return fmt.Errorf("connection is nil")
	}

	// Set write deadline
	tc.mu.RLock()
	writeTimeout := tc.writeTimeout
	tc.mu.RUnlock()

	if writeTimeout > 0 {
		err := tc.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
		if err != nil {
			return fmt.Errorf("failed to set write deadline: %w", err)
		}
	}

	// Write data
	n, err := tc.conn.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write data: %w", err)
	}

	// Update statistics and activity
	atomic.AddInt64(&tc.bytesWritten, int64(n))
	tc.updateActivity()

	return nil
}

// readFull reads exactly len(buf) bytes
func (tc *tcpConnection) readFull(buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := tc.conn.Read(buf[total:])
		if err != nil {
			return total, err
		}
		total += n
		atomic.AddInt64(&tc.bytesRead, int64(n))
	}
	return total, nil
}

// updateActivity updates the last activity timestamp
func (tc *tcpConnection) updateActivity() {
	atomic.StoreInt64(&tc.lastActivity, time.Now().Unix())
}

// ConnectionStatistics holds statistics for a connection
type ConnectionStatistics struct {
	ConnectionID string          `json:"connection_id"`
	State        ConnectionState `json:"state"`
	BytesRead    int64           `json:"bytes_read"`
	BytesWritten int64           `json:"bytes_written"`
	MessagesRead int64           `json:"messages_read"`
	MessagesSent int64           `json:"messages_sent"`
	LastActivity time.Time       `json:"last_activity"`
	RemoteAddr   string          `json:"remote_addr"`
	LocalAddr    string          `json:"local_addr"`
}

// String returns the string representation of connection statistics
func (cs ConnectionStatistics) String() string {
	return fmt.Sprintf("Connection[%s] State=%s BytesR/W=%d/%d MsgsR/S=%d/%d LastActivity=%s Remote=%s",
		cs.ConnectionID, cs.State, cs.BytesRead, cs.BytesWritten,
		cs.MessagesRead, cs.MessagesSent, cs.LastActivity.Format(time.RFC3339),
		cs.RemoteAddr)
}
