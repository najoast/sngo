package core

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// system implements the ActorSystem interface.
type system struct {
	router           AdvancedRouter
	sessionManager   *SessionManager
	serviceDiscovery ServiceDiscovery
	mu               sync.RWMutex
	nodeID           uint32

	// System shutdown context
	ctx    context.Context
	cancel context.CancelFunc

	// Wait group for all actors
	wg sync.WaitGroup
}

// NewActorSystem creates a new ActorSystem instance.
func NewActorSystem() ActorSystem {
	return NewActorSystemWithNodeID(1) // Default node ID
}

// NewActorSystemWithNodeID creates a new ActorSystem with a specific node ID.
func NewActorSystemWithNodeID(nodeID uint32) ActorSystem {
	ctx, cancel := context.WithCancel(context.Background())

	return &system{
		router:           NewAdvancedRouter(nodeID),
		sessionManager:   NewSessionManager(),
		serviceDiscovery: NewServiceDiscovery(),
		nodeID:           nodeID,
		ctx:              ctx,
		cancel:           cancel,
	}
}

// NewActor creates and registers a new Actor.
func (s *system) NewActor(handler MessageHandler, opts ActorOptions) (Actor, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if system is shutting down
	select {
	case <-s.ctx.Done():
		return nil, fmt.Errorf("actor system is shutting down")
	default:
	}

	// Generate unique ID
	id := s.router.(*advancedRouter).router.NextID()

	// Apply default options if needed
	if opts.MailboxSize == 0 {
		opts = DefaultActorOptions()
	}

	// Create actor
	actor := NewActor(id, handler, opts)

	// Register with router
	if err := s.router.Register(actor); err != nil {
		return nil, fmt.Errorf("failed to register actor: %w", err)
	}

	// Start the actor
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := actor.Start(s.ctx); err != nil {
			// Log error but don't panic
			// TODO: Add proper logging
		}
	}()

	return actor, nil
}

// NewService creates and registers a named service.
func (s *system) NewService(name string, handler MessageHandler, opts ActorOptions) (*Handle, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if system is shutting down
	select {
	case <-s.ctx.Done():
		return nil, fmt.Errorf("actor system is shutting down")
	default:
	}

	// Generate unique ID
	id := s.router.(*advancedRouter).router.NextID()

	// Apply default options if needed
	if opts.MailboxSize == 0 {
		opts = DefaultActorOptions()
	}
	if opts.Name == "" {
		opts.Name = name
	}

	// Create actor
	actor := NewActor(id, handler, opts)

	// Register as named service
	handle, err := s.router.RegisterService(actor, name)
	if err != nil {
		return nil, fmt.Errorf("failed to register service: %w", err)
	}

	// Register with service discovery
	regInfo := ServiceRegistrationInfo{
		Description:         fmt.Sprintf("Service %s", name),
		Version:             "1.0.0",
		Tags:                []string{"sngo-service"},
		Metadata:            make(map[string]string),
		HealthCheckInterval: 30 * time.Second,
	}

	if err := s.serviceDiscovery.RegisterService(handle, regInfo); err != nil {
		// Rollback router registration
		s.router.UnregisterService(name)
		return nil, fmt.Errorf("failed to register with service discovery: %w", err)
	}

	// Start the actor
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := actor.Start(s.ctx); err != nil {
			// Log error but don't panic
			// TODO: Add proper logging
		}
	}()

	return handle, nil
}

// GetActor retrieves an Actor by its ID.
func (s *system) GetActor(id ActorID) (Actor, bool) {
	return s.router.Lookup(id)
}

// GetService retrieves a service by name.
func (s *system) GetService(name string) (*Handle, bool) {
	return s.router.LookupService(name)
}

// Send sends a message from one Actor to another.
func (s *system) Send(from, to ActorID, msgType MessageType, data []byte) error {
	msg := &Message{
		Type:      msgType,
		Source:    from,
		Target:    to,
		Data:      data,
		Timestamp: time.Now(),
	}

	return s.router.Route(msg)
}

// SendByName sends a message using service names.
func (s *system) SendByName(from, to string, msgType MessageType, data []byte) error {
	msg := &Message{
		Type:      msgType,
		Data:      data,
		Timestamp: time.Now(),
	}

	return s.router.RouteByName(from, to, msg)
}

// Call makes a synchronous call from one Actor to another.
func (s *system) Call(ctx context.Context, from, to ActorID, msgType MessageType, data []byte) ([]byte, error) {
	// Get source actor for making the call
	sourceActor, exists := s.router.Lookup(from)
	if !exists {
		return nil, fmt.Errorf("source actor %d not found", from)
	}

	msg := &Message{
		Type:      msgType,
		Source:    from,
		Target:    to,
		Data:      data,
		Timestamp: time.Now(),
	}

	resp, err := sourceActor.Call(ctx, msg)
	if err != nil {
		return nil, err
	}

	if resp.Type == MessageTypeError {
		return nil, fmt.Errorf("remote error: %s", string(resp.Data))
	}

	return resp.Data, nil
}

// CallByName makes a synchronous call using service names.
func (s *system) CallByName(ctx context.Context, from, to string, msgType MessageType, data []byte) ([]byte, error) {
	// Resolve source service
	sourceHandle, exists := s.router.LookupService(from)
	if !exists {
		return nil, fmt.Errorf("source service '%s' not found", from)
	}

	// Resolve target service
	targetHandle, exists := s.router.LookupService(to)
	if !exists {
		return nil, fmt.Errorf("target service '%s' not found", to)
	}

	// Make the call using actor IDs
	return s.Call(ctx, sourceHandle.ActorID, targetHandle.ActorID, msgType, data)
}

// Shutdown gracefully stops all Actors in the system.
func (s *system) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Signal shutdown
	s.cancel()

	// Stop all actors
	actorIDs := s.router.List()
	for _, id := range actorIDs {
		if actor, exists := s.router.Lookup(id); exists {
			if err := actor.Stop(); err != nil {
				// Log error but continue shutdown
				// TODO: Add proper logging
			}
		}
	}

	// Wait for all actors to finish with timeout
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Stats returns statistics for all Actors.
func (s *system) Stats() []ActorStats {
	var stats []ActorStats

	actorIDs := s.router.List()
	for _, id := range actorIDs {
		if actor, exists := s.router.Lookup(id); exists {
			stats = append(stats, actor.Stats())
		}
	}

	return stats
}

// ListServices returns all registered services.
func (s *system) ListServices() []*Handle {
	return s.router.GetHandleManager().ListHandles()
}

// DiscoverService finds the best instance of a service.
func (s *system) DiscoverService(name string) (*ServiceInfo, error) {
	return s.serviceDiscovery.DiscoverService(name)
}

// DiscoverServices finds all services matching criteria.
func (s *system) DiscoverServices(query ServiceQuery) ([]*ServiceInfo, error) {
	return s.serviceDiscovery.DiscoverServices(query)
}

// UpdateServiceHealth updates service health status.
func (s *system) UpdateServiceHealth(name string, status ServiceStatus) error {
	return s.serviceDiscovery.UpdateServiceHealth(name, status)
}

// SetLoadBalanceStrategy sets the load balancing strategy.
func (s *system) SetLoadBalanceStrategy(strategy LoadBalanceStrategy) error {
	return s.serviceDiscovery.SetLoadBalanceStrategy(strategy)
}
