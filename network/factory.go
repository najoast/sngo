// Package network provides factory implementation for creating network components
package network

import (
	"fmt"
)

// networkFactory implements the NetworkFactory interface
type networkFactory struct{}

// NewNetworkFactory creates a new network factory
func NewNetworkFactory() NetworkFactory {
	return &networkFactory{}
}

// CreateServer creates a new server
func (nf *networkFactory) CreateServer(config *NetworkConfig) (Server, error) {
	if config == nil {
		return nil, fmt.Errorf("config is nil")
	}

	switch config.Protocol {
	case ProtocolTCP:
		return NewTCPServer(config)
	case ProtocolUDP:
		// TODO: Implement UDP server
		return nil, fmt.Errorf("UDP server not implemented yet")
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", config.Protocol)
	}
}

// CreateClient creates a new client
func (nf *networkFactory) CreateClient(config *NetworkConfig) (Client, error) {
	if config == nil {
		return nil, fmt.Errorf("config is nil")
	}

	switch config.Protocol {
	case ProtocolTCP:
		return NewTCPClient(config)
	case ProtocolUDP:
		// TODO: Implement UDP client
		return nil, fmt.Errorf("UDP client not implemented yet")
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", config.Protocol)
	}
}

// CreateConnectionManager creates a new connection manager
func (nf *networkFactory) CreateConnectionManager() ConnectionManager {
	return NewConnectionManager()
}

// Global factory instance
var DefaultFactory = NewNetworkFactory()

// Convenience functions using the default factory

// CreateTCPServer creates a TCP server with the given config
func CreateTCPServer(config *NetworkConfig) (Server, error) {
	if config == nil {
		config = DefaultNetworkConfig()
	}
	config.Protocol = ProtocolTCP
	return DefaultFactory.CreateServer(config)
}

// CreateTCPClient creates a TCP client with the given config
func CreateTCPClient(config *NetworkConfig) (Client, error) {
	if config == nil {
		config = DefaultNetworkConfig()
	}
	config.Protocol = ProtocolTCP
	return DefaultFactory.CreateClient(config)
}

// CreateConnectionManager creates a connection manager
func CreateConnectionManager() ConnectionManager {
	return DefaultFactory.CreateConnectionManager()
}

// Helper functions for common configurations

// CreateSimpleTCPServer creates a simple TCP server on the given port
func CreateSimpleTCPServer(port int) (Server, error) {
	config := DefaultNetworkConfig()
	config.Port = port
	return CreateTCPServer(config)
}

// CreateSimpleTCPClient creates a simple TCP client
func CreateSimpleTCPClient() (Client, error) {
	return CreateTCPClient(nil)
}

// Network component builder for fluent API

// ServerBuilder provides a fluent API for building servers
type ServerBuilder struct {
	config *NetworkConfig
}

// NewServerBuilder creates a new server builder
func NewServerBuilder() *ServerBuilder {
	return &ServerBuilder{
		config: DefaultNetworkConfig(),
	}
}

// Protocol sets the protocol
func (sb *ServerBuilder) Protocol(protocol Protocol) *ServerBuilder {
	sb.config.Protocol = protocol
	return sb
}

// Address sets the listening address
func (sb *ServerBuilder) Address(address string) *ServerBuilder {
	sb.config.Address = address
	return sb
}

// Port sets the listening port
func (sb *ServerBuilder) Port(port int) *ServerBuilder {
	sb.config.Port = port
	return sb
}

// MaxConnections sets the maximum number of connections
func (sb *ServerBuilder) MaxConnections(max int) *ServerBuilder {
	sb.config.MaxConnections = max
	return sb
}

// BufferSize sets the buffer size
func (sb *ServerBuilder) BufferSize(size int) *ServerBuilder {
	sb.config.BufferSize = size
	return sb
}

// KeepAlive enables TCP keep-alive
func (sb *ServerBuilder) KeepAlive(enabled bool) *ServerBuilder {
	sb.config.KeepAlive = enabled
	return sb
}

// Build creates the server
func (sb *ServerBuilder) Build() (Server, error) {
	return DefaultFactory.CreateServer(sb.config)
}

// ClientBuilder provides a fluent API for building clients
type ClientBuilder struct {
	config *NetworkConfig
}

// NewClientBuilder creates a new client builder
func NewClientBuilder() *ClientBuilder {
	return &ClientBuilder{
		config: DefaultNetworkConfig(),
	}
}

// Protocol sets the protocol
func (cb *ClientBuilder) Protocol(protocol Protocol) *ClientBuilder {
	cb.config.Protocol = protocol
	return cb
}

// AutoReconnect sets auto-reconnect options
func (cb *ClientBuilder) AutoReconnect(maxAttempts int) *ClientBuilder {
	cb.config.MaxReconnectAttempts = maxAttempts
	return cb
}

// BufferSize sets the buffer size
func (cb *ClientBuilder) BufferSize(size int) *ClientBuilder {
	cb.config.BufferSize = size
	return cb
}

// KeepAlive enables TCP keep-alive
func (cb *ClientBuilder) KeepAlive(enabled bool) *ClientBuilder {
	cb.config.KeepAlive = enabled
	return cb
}

// Build creates the client
func (cb *ClientBuilder) Build() (Client, error) {
	return DefaultFactory.CreateClient(cb.config)
}
