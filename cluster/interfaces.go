// Package cluster provides distributed cluster support for SNGO framework
package cluster

import (
	"context"
	"fmt"
	"net"
	"time"
)

// NodeID represents a unique identifier for a cluster node
type NodeID string

// NodeState represents the state of a cluster node
type NodeState int

const (
	NodeStateUnknown NodeState = iota
	NodeStateJoining
	NodeStateActive
	NodeStateSuspected
	NodeStateLeaving
	NodeStateLeft
	NodeStateFailed
)

// String returns the string representation of NodeState
func (ns NodeState) String() string {
	switch ns {
	case NodeStateUnknown:
		return "unknown"
	case NodeStateJoining:
		return "joining"
	case NodeStateActive:
		return "active"
	case NodeStateSuspected:
		return "suspected"
	case NodeStateLeaving:
		return "leaving"
	case NodeStateLeft:
		return "left"
	case NodeStateFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// NodeInfo contains information about a cluster node
type NodeInfo struct {
	ID       NodeID            `json:"id"`
	Address  string            `json:"address"`
	Port     int               `json:"port"`
	State    NodeState         `json:"state"`
	Metadata map[string]string `json:"metadata,omitempty"`

	// Timestamps
	JoinedAt    time.Time `json:"joined_at"`
	LastSeen    time.Time `json:"last_seen"`
	StateChange time.Time `json:"state_change"`

	// Health info
	Load    float64 `json:"load"`
	Version string  `json:"version"`
}

// Node represents a cluster node
type Node interface {
	// ID returns the unique identifier of this node
	ID() NodeID

	// Address returns the network address of this node
	Address() net.Addr

	// Info returns detailed information about this node
	Info() *NodeInfo

	// IsLocal returns true if this is the local node
	IsLocal() bool

	// IsActive returns true if the node is active
	IsActive() bool

	// UpdateState updates the node state
	UpdateState(state NodeState) error

	// UpdateLoad updates the node load information
	UpdateLoad(load float64) error

	// Ping sends a ping to the node and returns response time
	Ping(ctx context.Context) (time.Duration, error)
}

// ClusterEvent represents an event in the cluster
type ClusterEvent struct {
	Type      ClusterEventType       `json:"type"`
	NodeID    NodeID                 `json:"node_id"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

// ClusterEventType represents the type of cluster event
type ClusterEventType string

const (
	EventNodeJoined    ClusterEventType = "node_joined"
	EventNodeLeft      ClusterEventType = "node_left"
	EventNodeFailed    ClusterEventType = "node_failed"
	EventNodeRecovered ClusterEventType = "node_recovered"
	EventLeaderElected ClusterEventType = "leader_elected"
	EventPartition     ClusterEventType = "partition_detected"
	EventMerge         ClusterEventType = "partition_healed"
)

// ClusterManager manages the cluster membership and state
type ClusterManager interface {
	// Start starts the cluster manager
	Start(ctx context.Context) error

	// Stop stops the cluster manager
	Stop(ctx context.Context) error

	// Join joins the cluster
	Join(ctx context.Context, seeds []string) error

	// Leave gracefully leaves the cluster
	Leave(ctx context.Context) error

	// LocalNode returns the local node
	LocalNode() Node

	// GetNode returns a node by ID
	GetNode(nodeID NodeID) (Node, bool)

	// GetAllNodes returns all known nodes
	GetAllNodes() []Node

	// GetActiveNodes returns all active nodes
	GetActiveNodes() []Node

	// IsLeader returns true if this node is the cluster leader
	IsLeader() bool

	// GetLeader returns the current cluster leader
	GetLeader() (Node, bool)

	// Events returns a channel for cluster events
	Events() <-chan ClusterEvent

	// AddEventListener adds an event listener
	AddEventListener(listener func(ClusterEvent))

	// GetClusterSize returns the number of active nodes
	GetClusterSize() int

	// GetClusterHealth returns overall cluster health
	GetClusterHealth() ClusterHealth
}

// ClusterHealth represents the health status of the cluster
type ClusterHealth struct {
	TotalNodes     int       `json:"total_nodes"`
	ActiveNodes    int       `json:"active_nodes"`
	SuspectedNodes int       `json:"suspected_nodes"`
	FailedNodes    int       `json:"failed_nodes"`
	HasLeader      bool      `json:"has_leader"`
	LeaderID       NodeID    `json:"leader_id,omitempty"`
	PartitionCount int       `json:"partition_count"`
	LastUpdate     time.Time `json:"last_update"`
	IsHealthy      bool      `json:"is_healthy"`
}

// MessageType represents the type of cluster message
type MessageType string

const (
	MessageTypeJoin       MessageType = "join"
	MessageTypeLeave      MessageType = "leave"
	MessageTypeHeartbeat  MessageType = "heartbeat"
	MessageTypeElection   MessageType = "election"
	MessageTypeActorCall  MessageType = "actor_call"
	MessageTypeActorReply MessageType = "actor_reply"
	MessageTypeSync       MessageType = "sync"
	MessageTypeBroadcast  MessageType = "broadcast"
)

// ClusterMessage represents a message sent between cluster nodes
type ClusterMessage struct {
	ID       string                 `json:"id"`
	Type     MessageType            `json:"type"`
	From     NodeID                 `json:"from"`
	To       NodeID                 `json:"to,omitempty"` // empty for broadcast
	Payload  []byte                 `json:"payload"`
	Headers  map[string]string      `json:"headers,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// Timing
	Timestamp time.Time     `json:"timestamp"`
	TTL       time.Duration `json:"ttl,omitempty"`

	// Routing
	Hops int      `json:"hops"`
	Path []NodeID `json:"path,omitempty"`
}

// MessageTransport handles message transmission between cluster nodes
type MessageTransport interface {
	// Start starts the message transport
	Start(ctx context.Context) error

	// Stop stops the message transport
	Stop(ctx context.Context) error

	// Send sends a message to a specific node
	Send(ctx context.Context, nodeID NodeID, message *ClusterMessage) error

	// Broadcast sends a message to all nodes
	Broadcast(ctx context.Context, message *ClusterMessage) error

	// SetMessageHandler sets the handler for incoming messages
	SetMessageHandler(handler MessageHandler)

	// GetStatistics returns transport statistics
	GetStatistics() TransportStatistics
}

// MessageHandler handles incoming cluster messages
type MessageHandler interface {
	// HandleMessage processes an incoming cluster message
	HandleMessage(ctx context.Context, from NodeID, message *ClusterMessage) error

	// HandleConnectionLost handles connection loss with a node
	HandleConnectionLost(nodeID NodeID, err error)

	// HandleConnectionEstablished handles new connection with a node
	HandleConnectionEstablished(nodeID NodeID)
}

// TransportStatistics contains transport layer statistics
type TransportStatistics struct {
	MessagesSent     int64         `json:"messages_sent"`
	MessagesReceived int64         `json:"messages_received"`
	BytesSent        int64         `json:"bytes_sent"`
	BytesReceived    int64         `json:"bytes_received"`
	ConnectionsOpen  int           `json:"connections_open"`
	ErrorCount       int64         `json:"error_count"`
	AverageLatency   time.Duration `json:"average_latency"`
}

// RemoteActorRef represents a reference to an actor on another node
type RemoteActorRef struct {
	NodeID  NodeID `json:"node_id"`
	ActorID string `json:"actor_id"`
	Address string `json:"address"`
}

// RemoteService provides remote service call capabilities
type RemoteService interface {
	// Call makes a remote call to an actor on another node
	Call(ctx context.Context, ref RemoteActorRef, message interface{}) (interface{}, error)

	// Send sends a message to a remote actor (fire and forget)
	Send(ctx context.Context, ref RemoteActorRef, message interface{}) error

	// Register registers a local service for remote access
	Register(serviceID string, handler RemoteCallHandler) error

	// Unregister unregisters a local service
	Unregister(serviceID string) error

	// Resolve resolves a service ID to actor references across the cluster
	Resolve(ctx context.Context, serviceID string) ([]RemoteActorRef, error)

	// GetServiceRegistry returns the service registry
	GetServiceRegistry() ServiceRegistry
}

// RemoteCallHandler handles remote service calls
type RemoteCallHandler interface {
	// Handle processes a remote call and returns a response
	Handle(ctx context.Context, request interface{}) (interface{}, error)
}

// ServiceRegistry manages service registration and discovery across the cluster
type ServiceRegistry interface {
	// RegisterService registers a service on this node
	RegisterService(ctx context.Context, serviceID string, metadata map[string]string) error

	// UnregisterService unregisters a service from this node
	UnregisterService(ctx context.Context, serviceID string) error

	// DiscoverService discovers all instances of a service across the cluster
	DiscoverService(ctx context.Context, serviceID string) ([]ServiceInstance, error)

	// Watch watches for changes to a service
	Watch(ctx context.Context, serviceID string) (<-chan ServiceEvent, error)

	// GetAllServices returns all registered services
	GetAllServices() map[string][]ServiceInstance
}

// ServiceInstance represents an instance of a service
type ServiceInstance struct {
	ServiceID string            `json:"service_id"`
	NodeID    NodeID            `json:"node_id"`
	Address   string            `json:"address"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Health    ServiceHealth     `json:"health"`

	RegisteredAt time.Time `json:"registered_at"`
	LastSeen     time.Time `json:"last_seen"`
}

// ServiceHealth represents the health of a service instance
type ServiceHealth string

const (
	ServiceHealthHealthy   ServiceHealth = "healthy"
	ServiceHealthUnhealthy ServiceHealth = "unhealthy"
	ServiceHealthUnknown   ServiceHealth = "unknown"
)

// ServiceEvent represents a service registry event
type ServiceEvent struct {
	Type      ServiceEventType `json:"type"`
	ServiceID string           `json:"service_id"`
	Instance  ServiceInstance  `json:"instance"`
	Timestamp time.Time        `json:"timestamp"`
}

// ServiceEventType represents the type of service event
type ServiceEventType string

const (
	ServiceEventRegistered   ServiceEventType = "registered"
	ServiceEventUnregistered ServiceEventType = "unregistered"
	ServiceEventHealthy      ServiceEventType = "healthy"
	ServiceEventUnhealthy    ServiceEventType = "unhealthy"
)

// ClusterConfig contains cluster configuration
type ClusterConfig struct {
	// Node configuration
	NodeID   NodeID `yaml:"node_id" json:"node_id"`
	BindAddr string `yaml:"bind_addr" json:"bind_addr"`
	BindPort int    `yaml:"bind_port" json:"bind_port"`

	// Cluster settings
	ClusterName string   `yaml:"cluster_name" json:"cluster_name"`
	SeedNodes   []string `yaml:"seed_nodes" json:"seed_nodes"`

	// Timing settings
	HeartbeatInterval time.Duration `yaml:"heartbeat_interval" json:"heartbeat_interval"`
	ElectionTimeout   time.Duration `yaml:"election_timeout" json:"election_timeout"`
	JoinTimeout       time.Duration `yaml:"join_timeout" json:"join_timeout"`
	LeaveTimeout      time.Duration `yaml:"leave_timeout" json:"leave_timeout"`

	// Failure detection
	SuspicionTimeout    time.Duration `yaml:"suspicion_timeout" json:"suspicion_timeout"`
	SuspicionMultiplier int           `yaml:"suspicion_multiplier" json:"suspicion_multiplier"`

	// Transport settings
	MessageTimeout     time.Duration `yaml:"message_timeout" json:"message_timeout"`
	MaxMessageSize     int           `yaml:"max_message_size" json:"max_message_size"`
	CompressionEnabled bool          `yaml:"compression_enabled" json:"compression_enabled"`
	EncryptionEnabled  bool          `yaml:"encryption_enabled" json:"encryption_enabled"`

	// Advanced settings
	GossipFanout     int           `yaml:"gossip_fanout" json:"gossip_fanout"`
	GossipInterval   time.Duration `yaml:"gossip_interval" json:"gossip_interval"`
	PushPullInterval time.Duration `yaml:"push_pull_interval" json:"push_pull_interval"`

	// Metadata
	Metadata map[string]string `yaml:"metadata,omitempty" json:"metadata,omitempty"`
}

// DefaultClusterConfig returns a default cluster configuration
func DefaultClusterConfig() *ClusterConfig {
	return &ClusterConfig{
		NodeID:   "", // Will be generated
		BindAddr: "0.0.0.0",
		BindPort: 7946,

		ClusterName: "sngo-cluster",
		SeedNodes:   []string{},

		HeartbeatInterval: 1 * time.Second,
		ElectionTimeout:   10 * time.Second,
		JoinTimeout:       30 * time.Second,
		LeaveTimeout:      10 * time.Second,

		SuspicionTimeout:    5 * time.Second,
		SuspicionMultiplier: 3,

		MessageTimeout:     10 * time.Second,
		MaxMessageSize:     1024 * 1024, // 1MB
		CompressionEnabled: true,
		EncryptionEnabled:  false,

		GossipFanout:     3,
		GossipInterval:   200 * time.Millisecond,
		PushPullInterval: 30 * time.Second,

		Metadata: make(map[string]string),
	}
}

// ClusterError represents an error that occurred in cluster operations
type ClusterError struct {
	Operation string
	NodeID    NodeID
	Err       error
}

func (e *ClusterError) Error() string {
	if e.NodeID != "" {
		return fmt.Sprintf("cluster %s failed for node %s: %v", e.Operation, e.NodeID, e.Err)
	}
	return fmt.Sprintf("cluster %s failed: %v", e.Operation, e.Err)
}

func (e *ClusterError) Unwrap() error {
	return e.Err
}
