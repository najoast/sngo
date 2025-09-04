package cluster

import (
	"context"
	"crypto/rand"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// localNode implements the Node interface for the local node
type localNode struct {
	id       NodeID
	address  net.Addr
	info     *NodeInfo
	mu       sync.RWMutex
	manager  *clusterManager
	lastPing int64 // atomic
}

// NewLocalNode creates a new local node
func NewLocalNode(id NodeID, address net.Addr, metadata map[string]string) Node {
	if id == "" {
		id = generateNodeID()
	}

	if metadata == nil {
		metadata = make(map[string]string)
	}

	now := time.Now()
	info := &NodeInfo{
		ID:          id,
		Address:     address.String(),
		Port:        extractPort(address),
		State:       NodeStateJoining,
		Metadata:    metadata,
		JoinedAt:    now,
		LastSeen:    now,
		StateChange: now,
		Load:        0.0,
		Version:     "1.0.0",
	}

	return &localNode{
		id:      id,
		address: address,
		info:    info,
	}
}

func (n *localNode) ID() NodeID {
	return n.id
}

func (n *localNode) Address() net.Addr {
	return n.address
}

func (n *localNode) Info() *NodeInfo {
	n.mu.RLock()
	defer n.mu.RUnlock()

	// Return a copy to prevent external modification
	info := *n.info
	return &info
}

func (n *localNode) IsLocal() bool {
	return true
}

func (n *localNode) IsActive() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.info.State == NodeStateActive
}

func (n *localNode) UpdateState(state NodeState) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	oldState := n.info.State
	n.info.State = state
	n.info.StateChange = time.Now()

	if n.manager != nil {
		event := ClusterEvent{
			Type:      getStateChangeEventType(oldState, state),
			NodeID:    n.id,
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"old_state": oldState.String(),
				"new_state": state.String(),
			},
		}
		n.manager.publishEvent(event)
	}

	return nil
}

func (n *localNode) UpdateLoad(load float64) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.info.Load = load
	n.info.LastSeen = time.Now()
	return nil
}

func (n *localNode) Ping(ctx context.Context) (time.Duration, error) {
	start := time.Now()
	atomic.StoreInt64(&n.lastPing, start.UnixNano())
	return time.Since(start), nil
}

// remoteNode implements the Node interface for remote nodes
type remoteNode struct {
	info     *NodeInfo
	mu       sync.RWMutex
	manager  *clusterManager
	lastPing int64 // atomic
}

// NewRemoteNode creates a new remote node
func NewRemoteNode(info *NodeInfo) Node {
	return &remoteNode{
		info: info,
	}
}

func (n *remoteNode) ID() NodeID {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.info.ID
}

func (n *remoteNode) Address() net.Addr {
	n.mu.RLock()
	defer n.mu.RUnlock()

	addr, _ := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d", n.info.Address, n.info.Port))
	return addr
}

func (n *remoteNode) Info() *NodeInfo {
	n.mu.RLock()
	defer n.mu.RUnlock()

	// Return a copy to prevent external modification
	info := *n.info
	return &info
}

func (n *remoteNode) IsLocal() bool {
	return false
}

func (n *remoteNode) IsActive() bool {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.info.State == NodeStateActive
}

func (n *remoteNode) UpdateState(state NodeState) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	oldState := n.info.State
	n.info.State = state
	n.info.StateChange = time.Now()

	if n.manager != nil {
		event := ClusterEvent{
			Type:      getStateChangeEventType(oldState, state),
			NodeID:    n.info.ID,
			Timestamp: time.Now(),
			Data: map[string]interface{}{
				"old_state": oldState.String(),
				"new_state": state.String(),
			},
		}
		n.manager.publishEvent(event)
	}

	return nil
}

func (n *remoteNode) UpdateLoad(load float64) error {
	n.mu.Lock()
	defer n.mu.Unlock()

	n.info.Load = load
	n.info.LastSeen = time.Now()
	return nil
}

func (n *remoteNode) Ping(ctx context.Context) (time.Duration, error) {
	start := time.Now()

	// TODO: Implement actual network ping
	// For now, simulate a ping
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case <-time.After(10 * time.Millisecond):
		duration := time.Since(start)
		atomic.StoreInt64(&n.lastPing, start.UnixNano())
		return duration, nil
	}
}

// clusterManager implements the ClusterManager interface
type clusterManager struct {
	config    *ClusterConfig
	localNode Node
	nodes     map[NodeID]Node
	nodesMu   sync.RWMutex

	transport MessageTransport
	service   RemoteService
	registry  ServiceRegistry

	events      chan ClusterEvent
	listeners   []func(ClusterEvent)
	listenersMu sync.RWMutex

	leader   NodeID
	leaderMu sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	started int32 // atomic
}

// NewClusterManager creates a new cluster manager
func NewClusterManager(config *ClusterConfig) ClusterManager {
	if config == nil {
		config = DefaultClusterConfig()
	}

	// Create bind address
	bindAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d", config.BindAddr, config.BindPort))
	if err != nil {
		panic(fmt.Sprintf("invalid bind address: %v", err))
	}

	// Create local node
	localNode := NewLocalNode(config.NodeID, bindAddr, config.Metadata)

	return &clusterManager{
		config:    config,
		localNode: localNode,
		nodes:     make(map[NodeID]Node),
		events:    make(chan ClusterEvent, 100),
		listeners: make([]func(ClusterEvent), 0),
	}
}

func (cm *clusterManager) Start(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&cm.started, 0, 1) {
		return fmt.Errorf("cluster manager already started")
	}

	cm.ctx, cm.cancel = context.WithCancel(ctx)

	// Initialize transport
	if cm.transport == nil {
		cm.transport = NewMessageTransport(cm.config)
	}

	// Initialize service
	if cm.service == nil {
		cm.service = NewRemoteService(cm)
	}

	// Initialize registry
	if cm.registry == nil {
		cm.registry = NewServiceRegistry(cm)
	}

	// Start transport
	if err := cm.transport.Start(cm.ctx); err != nil {
		return fmt.Errorf("failed to start transport: %w", err)
	}

	// Set message handler
	cm.transport.SetMessageHandler(cm)

	// Add local node to cluster
	cm.addNode(cm.localNode)

	// Start background goroutines
	cm.wg.Add(3)
	go cm.heartbeatLoop()
	go cm.failureDetectionLoop()
	go cm.eventProcessingLoop()

	// Update local node state
	if err := cm.localNode.UpdateState(NodeStateActive); err != nil {
		return fmt.Errorf("failed to update local node state: %w", err)
	}

	// For single node cluster, elect self as leader
	if len(cm.config.SeedNodes) == 0 {
		cm.electSelf()
	}

	return nil
}

func (cm *clusterManager) Stop(ctx context.Context) error {
	if !atomic.CompareAndSwapInt32(&cm.started, 1, 0) {
		return nil // Already stopped
	}

	// Update local node state
	cm.localNode.UpdateState(NodeStateLeaving)

	// Send leave message to cluster
	if err := cm.broadcastLeave(); err != nil {
		// Log error but don't fail the stop
		fmt.Printf("Error broadcasting leave: %v\n", err)
	}

	// Cancel context and wait for goroutines
	cm.cancel()
	cm.wg.Wait()

	// Stop transport
	if cm.transport != nil {
		if err := cm.transport.Stop(ctx); err != nil {
			return fmt.Errorf("failed to stop transport: %w", err)
		}
	}

	// Update local node state
	cm.localNode.UpdateState(NodeStateLeft)

	// Close events channel
	close(cm.events)

	return nil
}

func (cm *clusterManager) Join(ctx context.Context, seeds []string) error {
	if len(seeds) == 0 {
		// No seeds provided, start as single node cluster
		cm.electSelf()
		return nil
	}

	// Try to join via seed nodes
	for _, seed := range seeds {
		if err := cm.joinViaSeed(ctx, seed); err == nil {
			return nil
		}
	}

	return fmt.Errorf("failed to join cluster via any seed node")
}

func (cm *clusterManager) Leave(ctx context.Context) error {
	return cm.Stop(ctx)
}

func (cm *clusterManager) LocalNode() Node {
	return cm.localNode
}

func (cm *clusterManager) GetNode(nodeID NodeID) (Node, bool) {
	cm.nodesMu.RLock()
	defer cm.nodesMu.RUnlock()

	node, exists := cm.nodes[nodeID]
	return node, exists
}

func (cm *clusterManager) GetAllNodes() []Node {
	cm.nodesMu.RLock()
	defer cm.nodesMu.RUnlock()

	nodes := make([]Node, 0, len(cm.nodes))
	for _, node := range cm.nodes {
		nodes = append(nodes, node)
	}
	return nodes
}

func (cm *clusterManager) GetActiveNodes() []Node {
	cm.nodesMu.RLock()
	defer cm.nodesMu.RUnlock()

	nodes := make([]Node, 0, len(cm.nodes))
	for _, node := range cm.nodes {
		if node.IsActive() {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

func (cm *clusterManager) IsLeader() bool {
	cm.leaderMu.RLock()
	defer cm.leaderMu.RUnlock()

	return cm.leader == cm.localNode.ID()
}

func (cm *clusterManager) GetLeader() (Node, bool) {
	cm.leaderMu.RLock()
	leaderID := cm.leader
	cm.leaderMu.RUnlock()

	if leaderID == "" {
		return nil, false
	}

	return cm.GetNode(leaderID)
}

func (cm *clusterManager) Events() <-chan ClusterEvent {
	return cm.events
}

func (cm *clusterManager) AddEventListener(listener func(ClusterEvent)) {
	cm.listenersMu.Lock()
	defer cm.listenersMu.Unlock()

	cm.listeners = append(cm.listeners, listener)
}

func (cm *clusterManager) GetClusterSize() int {
	return len(cm.GetActiveNodes())
}

func (cm *clusterManager) GetClusterHealth() ClusterHealth {
	nodes := cm.GetAllNodes()
	active := 0
	suspected := 0
	failed := 0

	for _, node := range nodes {
		info := node.Info()
		switch info.State {
		case NodeStateActive:
			active++
		case NodeStateSuspected:
			suspected++
		case NodeStateFailed, NodeStateLeft:
			failed++
		}
	}

	leader, hasLeader := cm.GetLeader()
	var leaderID NodeID
	if hasLeader {
		leaderID = leader.ID()
	}

	// For single node cluster, it's healthy if node is active and has leader
	isHealthy := hasLeader && active > 0
	if len(nodes) > 1 {
		// For multi-node cluster, need majority of nodes active
		isHealthy = hasLeader && active > len(nodes)/2
	}

	return ClusterHealth{
		TotalNodes:     len(nodes),
		ActiveNodes:    active,
		SuspectedNodes: suspected,
		FailedNodes:    failed,
		HasLeader:      hasLeader,
		LeaderID:       leaderID,
		PartitionCount: 1, // TODO: Implement partition detection
		LastUpdate:     time.Now(),
		IsHealthy:      isHealthy,
	}
}

// Helper methods

func (cm *clusterManager) addNode(node Node) {
	cm.nodesMu.Lock()
	defer cm.nodesMu.Unlock()

	cm.nodes[node.ID()] = node

	// Set manager reference if it's a local or remote node
	if ln, ok := node.(*localNode); ok {
		ln.manager = cm
	} else if rn, ok := node.(*remoteNode); ok {
		rn.manager = cm
	}
}

func (cm *clusterManager) publishEvent(event ClusterEvent) {
	select {
	case cm.events <- event:
	default:
		// Channel full, drop event
	}

	cm.listenersMu.RLock()
	defer cm.listenersMu.RUnlock()

	for _, listener := range cm.listeners {
		go listener(event)
	}
}

func (cm *clusterManager) electSelf() {
	cm.leaderMu.Lock()
	defer cm.leaderMu.Unlock()

	cm.leader = cm.localNode.ID()

	event := ClusterEvent{
		Type:      EventLeaderElected,
		NodeID:    cm.localNode.ID(),
		Timestamp: time.Now(),
	}
	cm.publishEvent(event)
}

func (cm *clusterManager) joinViaSeed(ctx context.Context, seed string) error {
	// TODO: Implement seed node joining
	return fmt.Errorf("seed joining not implemented")
}

func (cm *clusterManager) broadcastLeave() error {
	// TODO: Implement leave broadcast
	return nil
}

// Background loops

func (cm *clusterManager) heartbeatLoop() {
	defer cm.wg.Done()

	ticker := time.NewTicker(cm.config.HeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-cm.ctx.Done():
			return
		case <-ticker.C:
			cm.sendHeartbeat()
		}
	}
}

func (cm *clusterManager) failureDetectionLoop() {
	defer cm.wg.Done()

	ticker := time.NewTicker(cm.config.SuspicionTimeout)
	defer ticker.Stop()

	for {
		select {
		case <-cm.ctx.Done():
			return
		case <-ticker.C:
			cm.detectFailures()
		}
	}
}

func (cm *clusterManager) eventProcessingLoop() {
	defer cm.wg.Done()

	for {
		select {
		case <-cm.ctx.Done():
			return
		case event := <-cm.events:
			cm.processEvent(event)
		}
	}
}

func (cm *clusterManager) sendHeartbeat() {
	// TODO: Implement heartbeat sending
}

func (cm *clusterManager) detectFailures() {
	// TODO: Implement failure detection
}

func (cm *clusterManager) processEvent(event ClusterEvent) {
	// TODO: Implement event processing
}

// MessageHandler implementation

func (cm *clusterManager) HandleMessage(ctx context.Context, from NodeID, message *ClusterMessage) error {
	// TODO: Implement message handling
	return nil
}

func (cm *clusterManager) HandleConnectionLost(nodeID NodeID, err error) {
	if node, exists := cm.GetNode(nodeID); exists {
		node.UpdateState(NodeStateSuspected)
	}
}

func (cm *clusterManager) HandleConnectionEstablished(nodeID NodeID) {
	if node, exists := cm.GetNode(nodeID); exists {
		node.UpdateState(NodeStateActive)
	}
}

// Utility functions

func generateNodeID() NodeID {
	b := make([]byte, 8)
	rand.Read(b)
	return NodeID(fmt.Sprintf("node-%x", b))
}

func extractPort(addr net.Addr) int {
	if tcpAddr, ok := addr.(*net.TCPAddr); ok {
		return tcpAddr.Port
	}
	return 0
}

func getStateChangeEventType(oldState, newState NodeState) ClusterEventType {
	switch newState {
	case NodeStateActive:
		if oldState == NodeStateJoining {
			return EventNodeJoined
		}
		return EventNodeRecovered
	case NodeStateLeft:
		return EventNodeLeft
	case NodeStateFailed:
		return EventNodeFailed
	default:
		return EventNodeJoined
	}
}
