// Package network provides TCP/UDP networking capabilities for SNGO framework
package network

import (
	"context"
	"net"
	"time"
)

// Protocol defines the network protocol type
type Protocol string

const (
	ProtocolTCP Protocol = "tcp"
	ProtocolUDP Protocol = "udp"
)

// ConnectionState represents the state of a network connection
type ConnectionState int

const (
	ConnectionStateConnected ConnectionState = iota
	ConnectionStateDisconnected
	ConnectionStateReconnecting
	ConnectionStateClosed
)

// String returns the string representation of ConnectionState
func (cs ConnectionState) String() string {
	switch cs {
	case ConnectionStateConnected:
		return "connected"
	case ConnectionStateDisconnected:
		return "disconnected"
	case ConnectionStateReconnecting:
		return "reconnecting"
	case ConnectionStateClosed:
		return "closed"
	default:
		return "unknown"
	}
}

// Connection represents a network connection with extended functionality
type Connection interface {
	// ID returns the unique identifier for this connection
	ID() string

	// RemoteAddr returns the remote network address
	RemoteAddr() net.Addr

	// LocalAddr returns the local network address
	LocalAddr() net.Addr

	// Send sends data to the connection
	Send(data []byte) error

	// SendMessage sends a structured message
	SendMessage(msg *Message) error

	// Close closes the connection
	Close() error

	// State returns the current connection state
	State() ConnectionState

	// SetReadTimeout sets the read timeout
	SetReadTimeout(timeout time.Duration)

	// SetWriteTimeout sets the write timeout
	SetWriteTimeout(timeout time.Duration)

	// GetLastActivity returns the timestamp of last activity
	GetLastActivity() time.Time

	// GetUserData returns user-defined data associated with this connection
	GetUserData() interface{}

	// SetUserData sets user-defined data for this connection
	SetUserData(data interface{})

	// ReadMessage reads a message from the connection
	ReadMessage() (*Message, error)

	// GetStatistics returns connection statistics
	GetStatistics() ConnectionStatistics
}

// Server represents a network server
type Server interface {
	// Start starts the server
	Start() error

	// Stop stops the server gracefully
	Stop() error

	// Listen returns the listening address
	Listen() net.Addr

	// AcceptConnection waits for and returns new connections
	AcceptConnection(ctx context.Context) (Connection, error)

	// SetConnectionHandler sets the handler for new connections
	SetConnectionHandler(handler ConnectionHandler)

	// SetMessageHandler sets the handler for incoming messages
	SetMessageHandler(handler MessageHandler)

	// GetActiveConnections returns all active connections
	GetActiveConnections() []Connection

	// GetConnectionCount returns the number of active connections
	GetConnectionCount() int

	// GetStatistics returns server statistics
	GetStatistics() ServerStatistics

	// BroadcastMessage broadcasts a message to all connections
	BroadcastMessage(msg *Message) error
}

// Client represents a network client
type Client interface {
	// Connect connects to the remote server
	Connect(address string) (Connection, error)

	// ConnectWithTimeout connects with a timeout
	ConnectWithTimeout(address string, timeout time.Duration) (Connection, error)

	// ConnectAsync connects asynchronously
	ConnectAsync(address string) <-chan ConnectionResult

	// Disconnect disconnects from the server
	Disconnect() error

	// GetConnection returns the current connection
	GetConnection() Connection

	// SetAutoReconnect enables/disables auto reconnection
	SetAutoReconnect(enabled bool, interval time.Duration)

	// SetMessageHandler sets the handler for incoming messages
	SetMessageHandler(handler MessageHandler)

	// IsConnected returns true if the client is connected
	IsConnected() bool

	// GetStatistics returns client statistics
	GetStatistics() ClientStatistics

	// SendMessage sends a message through the client connection
	SendMessage(msg *Message) error
}

// ConnectionResult represents the result of an async connection
type ConnectionResult struct {
	Connection Connection
	Error      error
}

// ConnectionHandler handles new connections
type ConnectionHandler interface {
	// OnConnect is called when a new connection is established
	OnConnect(conn Connection)

	// OnDisconnect is called when a connection is closed
	OnDisconnect(conn Connection, err error)

	// OnError is called when a connection error occurs
	OnError(conn Connection, err error)
}

// MessageHandler handles incoming messages
type MessageHandler interface {
	// OnMessage is called when a message is received
	OnMessage(conn Connection, msg *Message)

	// OnError is called when a message processing error occurs
	OnError(conn Connection, err error)
}

// ConnectionManager manages multiple connections
type ConnectionManager interface {
	// AddConnection adds a connection to the manager
	AddConnection(conn Connection) error

	// RemoveConnection removes a connection from the manager
	RemoveConnection(connID string) error

	// GetConnection gets a connection by ID
	GetConnection(connID string) (Connection, bool)

	// GetAllConnections returns all managed connections
	GetAllConnections() []Connection

	// BroadcastMessage broadcasts a message to all connections
	BroadcastMessage(msg *Message) error

	// BroadcastData broadcasts raw data to all connections
	BroadcastData(data []byte) error

	// GetConnectionCount returns the number of managed connections
	GetConnectionCount() int

	// StartHeartbeat starts heartbeat for all connections
	StartHeartbeat(interval time.Duration) error

	// StopHeartbeat stops heartbeat
	StopHeartbeat() error

	// Cleanup removes inactive connections
	Cleanup(timeout time.Duration) int

	// GetStatistics returns connection manager statistics
	GetStatistics() ConnectionManagerStatistics

	// SendMessageToConnection sends a message to a specific connection
	SendMessageToConnection(connID string, msg *Message) error

	// GetConnectionsByState returns connections filtered by state
	GetConnectionsByState(state ConnectionState) []Connection

	// CloseAllConnections closes all managed connections
	CloseAllConnections() error
}

// NetworkConfig represents network configuration
type NetworkConfig struct {
	// Protocol is the network protocol (tcp, udp)
	Protocol Protocol

	// Address is the listening address
	Address string

	// Port is the listening port
	Port int

	// ReadTimeout is the read timeout duration
	ReadTimeout time.Duration

	// WriteTimeout is the write timeout duration
	WriteTimeout time.Duration

	// KeepAlive enables TCP keep-alive
	KeepAlive bool

	// KeepAliveInterval is the keep-alive interval
	KeepAliveInterval time.Duration

	// MaxConnections is the maximum number of concurrent connections
	MaxConnections int

	// BufferSize is the buffer size for reading/writing
	BufferSize int

	// HeartbeatInterval is the heartbeat interval
	HeartbeatInterval time.Duration

	// ReconnectInterval is the auto-reconnect interval
	ReconnectInterval time.Duration

	// MaxReconnectAttempts is the maximum number of reconnect attempts
	MaxReconnectAttempts int
}

// DefaultNetworkConfig returns a default network configuration
func DefaultNetworkConfig() *NetworkConfig {
	return &NetworkConfig{
		Protocol:             ProtocolTCP,
		Address:              "0.0.0.0",
		Port:                 8080,
		ReadTimeout:          30 * time.Second,
		WriteTimeout:         30 * time.Second,
		KeepAlive:            true,
		KeepAliveInterval:    60 * time.Second,
		MaxConnections:       1000,
		BufferSize:           4096,
		HeartbeatInterval:    30 * time.Second,
		ReconnectInterval:    5 * time.Second,
		MaxReconnectAttempts: 3,
	}
}

// NetworkFactory creates network components
type NetworkFactory interface {
	// CreateServer creates a new server
	CreateServer(config *NetworkConfig) (Server, error)

	// CreateClient creates a new client
	CreateClient(config *NetworkConfig) (Client, error)

	// CreateConnectionManager creates a new connection manager
	CreateConnectionManager() ConnectionManager
}
