package core

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// AdvancedRouter extends the basic Router with Handle support and distributed routing.
type AdvancedRouter interface {
	Router

	// RouteByHandle routes a message using handle-based addressing
	RouteByHandle(envelope *MessageEnvelope) error

	// RouteByName routes a message using name-based addressing
	RouteByName(source, target string, msg *Message) error

	// RegisterService registers a service with a name
	RegisterService(actor Actor, name string) (*Handle, error)

	// UnregisterService unregisters a service by name
	UnregisterService(name string) error

	// LookupService finds a service by name
	LookupService(name string) (*Handle, bool)

	// GetHandleManager returns the underlying handle manager
	GetHandleManager() *HandleManager
}

// advancedRouter implements the AdvancedRouter interface.
type advancedRouter struct {
	*router // Embed basic router

	handleManager *HandleManager
	nodeID        uint32

	// Remote node connections (for future cluster support)
	remoteNodes sync.Map // map[uint32]RemoteNode
}

// NewAdvancedRouter creates a new AdvancedRouter.
func NewAdvancedRouter(nodeID uint32) AdvancedRouter {
	return &advancedRouter{
		router:        NewRouter().(*router),
		handleManager: NewHandleManager(nodeID),
		nodeID:        nodeID,
	}
}

// Register adds an Actor to the routing table and allocates a handle.
func (ar *advancedRouter) Register(actor Actor) error {
	// Register with basic router first
	if err := ar.router.Register(actor); err != nil {
		return err
	}

	// Allocate a handle
	_, err := ar.handleManager.AllocateHandle(actor.ID(), "")
	return err
}

// RegisterService registers a service with a name.
func (ar *advancedRouter) RegisterService(actor Actor, name string) (*Handle, error) {
	// Register with basic router first
	if err := ar.router.Register(actor); err != nil {
		return nil, err
	}

	// Allocate a named handle
	return ar.handleManager.AllocateHandle(actor.ID(), name)
}

// Unregister removes an Actor from the routing table.
func (ar *advancedRouter) Unregister(id ActorID) error {
	// Get handle before unregistering
	if handle, exists := ar.handleManager.GetHandleByActor(id); exists {
		ar.handleManager.ReleaseHandle(handle.ID)
	}

	return ar.router.Unregister(id)
}

// UnregisterService unregisters a service by name.
func (ar *advancedRouter) UnregisterService(name string) error {
	handle, exists := ar.handleManager.GetHandleByName(name)
	if !exists {
		return fmt.Errorf("service '%s' not found", name)
	}

	// Unregister the actor
	if err := ar.router.Unregister(handle.ActorID); err != nil {
		return err
	}

	// Release the handle
	return ar.handleManager.ReleaseHandle(handle.ID)
}

// RouteByHandle routes a message using handle-based addressing.
func (ar *advancedRouter) RouteByHandle(envelope *MessageEnvelope) error {
	if envelope == nil || envelope.Message == nil {
		return fmt.Errorf("invalid message envelope")
	}

	// Resolve target address
	targetHandle, err := ar.handleManager.ResolveAddress(envelope.Target)
	if err != nil {
		return fmt.Errorf("failed to resolve target: %w", err)
	}

	// Check if target is local
	if targetHandle.IsLocal {
		// Route to local actor
		envelope.Message.Target = targetHandle.ActorID
		return ar.router.Route(envelope.Message)
	}

	// Route to remote node (TODO: implement cluster routing)
	return ar.routeToRemoteNode(targetHandle, envelope)
}

// RouteByName routes a message using name-based addressing.
func (ar *advancedRouter) RouteByName(source, target string, msg *Message) error {
	// Resolve source if provided
	var sourceHandle *Handle
	if source != "" {
		var exists bool
		sourceHandle, exists = ar.handleManager.GetHandleByName(source)
		if !exists {
			return fmt.Errorf("source service '%s' not found", source)
		}
		msg.Source = sourceHandle.ActorID
	}

	// Resolve target
	targetHandle, exists := ar.handleManager.GetHandleByName(target)
	if !exists {
		return fmt.Errorf("target service '%s' not found", target)
	}

	// Set target and route
	msg.Target = targetHandle.ActorID
	return ar.router.Route(msg)
}

// LookupService finds a service by name.
func (ar *advancedRouter) LookupService(name string) (*Handle, bool) {
	return ar.handleManager.GetHandleByName(name)
}

// GetHandleManager returns the underlying handle manager.
func (ar *advancedRouter) GetHandleManager() *HandleManager {
	return ar.handleManager
}

// routeToRemoteNode handles routing to remote nodes (placeholder for cluster support).
func (ar *advancedRouter) routeToRemoteNode(targetHandle *Handle, envelope *MessageEnvelope) error {
	// TODO: Implement cluster routing
	return fmt.Errorf("remote routing not implemented (target node: %d)", targetHandle.Node)
}

// MessageSession manages request-response sessions with timeouts.
type MessageSession struct {
	ID        uint32
	Source    ActorID
	Target    ActorID
	CreatedAt time.Time
	Timeout   time.Duration
	Response  chan *Message
}

// SessionManager manages ongoing message sessions.
type SessionManager struct {
	mu       sync.RWMutex
	sessions map[uint32]*MessageSession
	counter  uint32
}

// NewSessionManager creates a new SessionManager.
func NewSessionManager() *SessionManager {
	sm := &SessionManager{
		sessions: make(map[uint32]*MessageSession),
	}

	// Start cleanup goroutine
	go sm.cleanupExpiredSessions()

	return sm
}

// CreateSession creates a new message session.
func (sm *SessionManager) CreateSession(source, target ActorID, timeout time.Duration) (*MessageSession, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.counter++
	sessionID := sm.counter

	session := &MessageSession{
		ID:        sessionID,
		Source:    source,
		Target:    target,
		CreatedAt: time.Now(),
		Timeout:   timeout,
		Response:  make(chan *Message, 1),
	}

	sm.sessions[sessionID] = session
	return session, nil
}

// GetSession retrieves a session by ID.
func (sm *SessionManager) GetSession(sessionID uint32) (*MessageSession, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	session, exists := sm.sessions[sessionID]
	return session, exists
}

// CompleteSession completes a session with a response.
func (sm *SessionManager) CompleteSession(sessionID uint32, response *Message) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[sessionID]
	if !exists {
		return fmt.Errorf("session %d not found", sessionID)
	}

	select {
	case session.Response <- response:
		delete(sm.sessions, sessionID)
		return nil
	default:
		// Channel is full or closed
		delete(sm.sessions, sessionID)
		return fmt.Errorf("session %d response channel is full", sessionID)
	}
}

// CleanupSession removes a session without sending a response.
func (sm *SessionManager) CleanupSession(sessionID uint32) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if session, exists := sm.sessions[sessionID]; exists {
		close(session.Response)
		delete(sm.sessions, sessionID)
	}
}

// cleanupExpiredSessions periodically removes expired sessions.
func (sm *SessionManager) cleanupExpiredSessions() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		sm.mu.Lock()

		for sessionID, session := range sm.sessions {
			if now.Sub(session.CreatedAt) > session.Timeout {
				close(session.Response)
				delete(sm.sessions, sessionID)
			}
		}

		sm.mu.Unlock()
	}
}

// WaitForResponse waits for a session response with timeout.
func (session *MessageSession) WaitForResponse(ctx context.Context) (*Message, error) {
	select {
	case response := <-session.Response:
		if response == nil {
			return nil, fmt.Errorf("session %d was closed", session.ID)
		}
		return response, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(session.Timeout):
		return nil, fmt.Errorf("session %d timeout", session.ID)
	}
}
