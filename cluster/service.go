package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// remoteService implements the RemoteService interface
type remoteService struct {
	manager   ClusterManager
	transport MessageTransport
	registry  ServiceRegistry

	handlers   map[string]RemoteCallHandler
	handlersMu sync.RWMutex

	pendingCalls map[string]*pendingCall
	callsMu      sync.RWMutex

	callCounter int64 // atomic
}

// pendingCall represents a pending remote call
type pendingCall struct {
	id      string
	result  chan interface{}
	error   chan error
	timeout time.Time
}

// RemoteCallRequest represents a remote call request
type RemoteCallRequest struct {
	CallID    string      `json:"call_id"`
	ServiceID string      `json:"service_id"`
	Method    string      `json:"method"`
	Args      interface{} `json:"args"`
}

// RemoteCallResponse represents a remote call response
type RemoteCallResponse struct {
	CallID string      `json:"call_id"`
	Result interface{} `json:"result,omitempty"`
	Error  string      `json:"error,omitempty"`
}

// NewRemoteService creates a new remote service
func NewRemoteService(manager ClusterManager) RemoteService {
	rs := &remoteService{
		manager:      manager,
		handlers:     make(map[string]RemoteCallHandler),
		pendingCalls: make(map[string]*pendingCall),
	}

	// TODO: Get transport from manager
	// rs.transport = manager.GetTransport()

	return rs
}

func (rs *remoteService) Call(ctx context.Context, ref RemoteActorRef, message interface{}) (interface{}, error) {
	// Generate call ID
	callID := rs.generateCallID()

	// Create request
	request := RemoteCallRequest{
		CallID:    callID,
		ServiceID: ref.ActorID,
		Method:    "handle", // Default method
		Args:      message,
	}

	// Serialize request
	payload, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize request: %w", err)
	}

	// Create cluster message
	clusterMsg := &ClusterMessage{
		ID:        generateMessageID(),
		Type:      MessageTypeActorCall,
		From:      rs.manager.LocalNode().ID(),
		To:        ref.NodeID,
		Payload:   payload,
		Timestamp: time.Now(),
		TTL:       30 * time.Second,
	}

	// Create pending call
	pending := &pendingCall{
		id:      callID,
		result:  make(chan interface{}, 1),
		error:   make(chan error, 1),
		timeout: time.Now().Add(30 * time.Second),
	}

	rs.callsMu.Lock()
	rs.pendingCalls[callID] = pending
	rs.callsMu.Unlock()

	// Cleanup pending call
	defer func() {
		rs.callsMu.Lock()
		delete(rs.pendingCalls, callID)
		rs.callsMu.Unlock()
	}()

	// Send message
	if err := rs.transport.Send(ctx, ref.NodeID, clusterMsg); err != nil {
		return nil, fmt.Errorf("failed to send remote call: %w", err)
	}

	// Wait for response
	select {
	case result := <-pending.result:
		return result, nil
	case err := <-pending.error:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("remote call timeout")
	}
}

func (rs *remoteService) Send(ctx context.Context, ref RemoteActorRef, message interface{}) error {
	// Serialize message
	payload, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to serialize message: %w", err)
	}

	// Create cluster message
	clusterMsg := &ClusterMessage{
		ID:        generateMessageID(),
		Type:      MessageTypeActorCall,
		From:      rs.manager.LocalNode().ID(),
		To:        ref.NodeID,
		Payload:   payload,
		Timestamp: time.Now(),
		Headers: map[string]string{
			"target_actor": ref.ActorID,
			"fire_forget":  "true",
		},
	}

	// Send message
	return rs.transport.Send(ctx, ref.NodeID, clusterMsg)
}

func (rs *remoteService) Register(serviceID string, handler RemoteCallHandler) error {
	rs.handlersMu.Lock()
	defer rs.handlersMu.Unlock()

	if _, exists := rs.handlers[serviceID]; exists {
		return fmt.Errorf("service %s already registered", serviceID)
	}

	rs.handlers[serviceID] = handler

	// Register with service registry
	if rs.registry != nil {
		metadata := map[string]string{
			"type": "remote_service",
		}
		return rs.registry.RegisterService(context.Background(), serviceID, metadata)
	}

	return nil
}

func (rs *remoteService) Unregister(serviceID string) error {
	rs.handlersMu.Lock()
	defer rs.handlersMu.Unlock()

	delete(rs.handlers, serviceID)

	// Unregister from service registry
	if rs.registry != nil {
		return rs.registry.UnregisterService(context.Background(), serviceID)
	}

	return nil
}

func (rs *remoteService) Resolve(ctx context.Context, serviceID string) ([]RemoteActorRef, error) {
	if rs.registry == nil {
		return nil, fmt.Errorf("service registry not available")
	}

	instances, err := rs.registry.DiscoverService(ctx, serviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to discover service: %w", err)
	}

	refs := make([]RemoteActorRef, 0, len(instances))
	for _, instance := range instances {
		ref := RemoteActorRef{
			NodeID:  instance.NodeID,
			ActorID: serviceID,
			Address: instance.Address,
		}
		refs = append(refs, ref)
	}

	return refs, nil
}

func (rs *remoteService) GetServiceRegistry() ServiceRegistry {
	return rs.registry
}

// MessageHandler interface implementation

func (rs *remoteService) HandleMessage(ctx context.Context, from NodeID, message *ClusterMessage) error {
	switch message.Type {
	case MessageTypeActorCall:
		return rs.handleRemoteCall(ctx, from, message)
	case MessageTypeActorReply:
		return rs.handleRemoteReply(ctx, from, message)
	default:
		return nil // Not our message type
	}
}

func (rs *remoteService) handleRemoteCall(ctx context.Context, from NodeID, message *ClusterMessage) error {
	// Check if it's fire and forget
	if message.Headers["fire_forget"] == "true" {
		return rs.handleFireAndForget(ctx, from, message)
	}

	// Parse request
	var request RemoteCallRequest
	if err := json.Unmarshal(message.Payload, &request); err != nil {
		return fmt.Errorf("failed to parse remote call request: %w", err)
	}

	// Get handler
	rs.handlersMu.RLock()
	handler, exists := rs.handlers[request.ServiceID]
	rs.handlersMu.RUnlock()

	if !exists {
		// Send error response
		return rs.sendErrorResponse(ctx, from, request.CallID, fmt.Errorf("service not found: %s", request.ServiceID))
	}

	// Handle call
	result, err := handler.Handle(ctx, request.Args)

	// Send response
	if err != nil {
		return rs.sendErrorResponse(ctx, from, request.CallID, err)
	}

	return rs.sendSuccessResponse(ctx, from, request.CallID, result)
}

func (rs *remoteService) handleFireAndForget(ctx context.Context, from NodeID, message *ClusterMessage) error {
	targetActor := message.Headers["target_actor"]
	if targetActor == "" {
		return fmt.Errorf("target_actor header missing")
	}

	// Get handler
	rs.handlersMu.RLock()
	handler, exists := rs.handlers[targetActor]
	rs.handlersMu.RUnlock()

	if !exists {
		return fmt.Errorf("actor not found: %s", targetActor)
	}

	// Parse message
	var args interface{}
	if err := json.Unmarshal(message.Payload, &args); err != nil {
		return fmt.Errorf("failed to parse message: %w", err)
	}

	// Handle message (fire and forget)
	go func() {
		if _, err := handler.Handle(context.Background(), args); err != nil {
			// Log error but don't return it
			fmt.Printf("Fire and forget call failed: %v\n", err)
		}
	}()

	return nil
}

func (rs *remoteService) handleRemoteReply(ctx context.Context, from NodeID, message *ClusterMessage) error {
	// Parse response
	var response RemoteCallResponse
	if err := json.Unmarshal(message.Payload, &response); err != nil {
		return fmt.Errorf("failed to parse remote call response: %w", err)
	}

	// Find pending call
	rs.callsMu.RLock()
	pending, exists := rs.pendingCalls[response.CallID]
	rs.callsMu.RUnlock()

	if !exists {
		// Call may have timed out
		return nil
	}

	// Send result
	if response.Error != "" {
		select {
		case pending.error <- fmt.Errorf(response.Error):
		default:
		}
	} else {
		select {
		case pending.result <- response.Result:
		default:
		}
	}

	return nil
}

func (rs *remoteService) sendSuccessResponse(ctx context.Context, to NodeID, callID string, result interface{}) error {
	response := RemoteCallResponse{
		CallID: callID,
		Result: result,
	}

	payload, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to serialize response: %w", err)
	}

	clusterMsg := &ClusterMessage{
		ID:        generateMessageID(),
		Type:      MessageTypeActorReply,
		From:      rs.manager.LocalNode().ID(),
		To:        to,
		Payload:   payload,
		Timestamp: time.Now(),
	}

	return rs.transport.Send(ctx, to, clusterMsg)
}

func (rs *remoteService) sendErrorResponse(ctx context.Context, to NodeID, callID string, err error) error {
	response := RemoteCallResponse{
		CallID: callID,
		Error:  err.Error(),
	}

	payload, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to serialize error response: %w", err)
	}

	clusterMsg := &ClusterMessage{
		ID:        generateMessageID(),
		Type:      MessageTypeActorReply,
		From:      rs.manager.LocalNode().ID(),
		To:        to,
		Payload:   payload,
		Timestamp: time.Now(),
	}

	return rs.transport.Send(ctx, to, clusterMsg)
}

func (rs *remoteService) generateCallID() string {
	counter := atomic.AddInt64(&rs.callCounter, 1)
	return fmt.Sprintf("call-%s-%d", rs.manager.LocalNode().ID(), counter)
}

// serviceRegistry implements the ServiceRegistry interface
type serviceRegistry struct {
	manager   ClusterManager
	transport MessageTransport

	services   map[string][]ServiceInstance
	servicesMu sync.RWMutex

	watchers   map[string][]chan ServiceEvent
	watchersMu sync.RWMutex
}

// NewServiceRegistry creates a new service registry
func NewServiceRegistry(manager ClusterManager) ServiceRegistry {
	return &serviceRegistry{
		manager:  manager,
		services: make(map[string][]ServiceInstance),
		watchers: make(map[string][]chan ServiceEvent),
	}
}

func (sr *serviceRegistry) RegisterService(ctx context.Context, serviceID string, metadata map[string]string) error {
	localNode := sr.manager.LocalNode()

	instance := ServiceInstance{
		ServiceID:    serviceID,
		NodeID:       localNode.ID(),
		Address:      localNode.Address().String(),
		Metadata:     metadata,
		Health:       ServiceHealthHealthy,
		RegisteredAt: time.Now(),
		LastSeen:     time.Now(),
	}

	sr.servicesMu.Lock()
	if instances, exists := sr.services[serviceID]; exists {
		// Check if already registered
		for i, existing := range instances {
			if existing.NodeID == localNode.ID() {
				// Update existing
				sr.services[serviceID][i] = instance
				sr.servicesMu.Unlock()
				sr.notifyWatchers(serviceID, ServiceEvent{
					Type:      ServiceEventRegistered,
					ServiceID: serviceID,
					Instance:  instance,
					Timestamp: time.Now(),
				})
				return nil
			}
		}
		// Add new
		sr.services[serviceID] = append(instances, instance)
	} else {
		sr.services[serviceID] = []ServiceInstance{instance}
	}
	sr.servicesMu.Unlock()

	// Notify watchers
	sr.notifyWatchers(serviceID, ServiceEvent{
		Type:      ServiceEventRegistered,
		ServiceID: serviceID,
		Instance:  instance,
		Timestamp: time.Now(),
	})

	// TODO: Broadcast registration to cluster

	return nil
}

func (sr *serviceRegistry) UnregisterService(ctx context.Context, serviceID string) error {
	localNode := sr.manager.LocalNode()

	sr.servicesMu.Lock()
	defer sr.servicesMu.Unlock()

	instances, exists := sr.services[serviceID]
	if !exists {
		return nil
	}

	// Remove local instance
	var removedInstance ServiceInstance
	newInstances := make([]ServiceInstance, 0, len(instances))
	for _, instance := range instances {
		if instance.NodeID != localNode.ID() {
			newInstances = append(newInstances, instance)
		} else {
			removedInstance = instance
		}
	}

	if len(newInstances) == 0 {
		delete(sr.services, serviceID)
	} else {
		sr.services[serviceID] = newInstances
	}

	// Notify watchers
	if removedInstance.ServiceID != "" {
		sr.notifyWatchers(serviceID, ServiceEvent{
			Type:      ServiceEventUnregistered,
			ServiceID: serviceID,
			Instance:  removedInstance,
			Timestamp: time.Now(),
		})
	}

	// TODO: Broadcast unregistration to cluster

	return nil
}

func (sr *serviceRegistry) DiscoverService(ctx context.Context, serviceID string) ([]ServiceInstance, error) {
	sr.servicesMu.RLock()
	defer sr.servicesMu.RUnlock()

	instances, exists := sr.services[serviceID]
	if !exists {
		return []ServiceInstance{}, nil
	}

	// Return a copy
	result := make([]ServiceInstance, len(instances))
	copy(result, instances)
	return result, nil
}

func (sr *serviceRegistry) Watch(ctx context.Context, serviceID string) (<-chan ServiceEvent, error) {
	ch := make(chan ServiceEvent, 100)

	sr.watchersMu.Lock()
	defer sr.watchersMu.Unlock()

	if watchers, exists := sr.watchers[serviceID]; exists {
		sr.watchers[serviceID] = append(watchers, ch)
	} else {
		sr.watchers[serviceID] = []chan ServiceEvent{ch}
	}

	// Cleanup when context is done
	go func() {
		<-ctx.Done()
		sr.removeWatcher(serviceID, ch)
		close(ch)
	}()

	return ch, nil
}

func (sr *serviceRegistry) GetAllServices() map[string][]ServiceInstance {
	sr.servicesMu.RLock()
	defer sr.servicesMu.RUnlock()

	result := make(map[string][]ServiceInstance)
	for serviceID, instances := range sr.services {
		result[serviceID] = make([]ServiceInstance, len(instances))
		copy(result[serviceID], instances)
	}

	return result
}

func (sr *serviceRegistry) notifyWatchers(serviceID string, event ServiceEvent) {
	sr.watchersMu.RLock()
	defer sr.watchersMu.RUnlock()

	watchers, exists := sr.watchers[serviceID]
	if !exists {
		return
	}

	for _, ch := range watchers {
		select {
		case ch <- event:
		default:
			// Channel full, skip
		}
	}
}

func (sr *serviceRegistry) removeWatcher(serviceID string, ch chan ServiceEvent) {
	sr.watchersMu.Lock()
	defer sr.watchersMu.Unlock()

	watchers, exists := sr.watchers[serviceID]
	if !exists {
		return
	}

	newWatchers := make([]chan ServiceEvent, 0, len(watchers))
	for _, watcher := range watchers {
		if watcher != ch {
			newWatchers = append(newWatchers, watcher)
		}
	}

	if len(newWatchers) == 0 {
		delete(sr.watchers, serviceID)
	} else {
		sr.watchers[serviceID] = newWatchers
	}
}
