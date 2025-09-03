package core

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Handle represents a Skynet-compatible service handle.
// Handles can be used for both local and remote service addressing.
type Handle struct {
	// ID is the numeric handle ID (compatible with Skynet)
	ID uint32

	// ActorID is the internal Actor ID
	ActorID ActorID

	// Name is the service name (optional)
	Name string

	// Node identifies which node this handle belongs to
	Node uint32

	// IsLocal indicates if this is a local or remote handle
	IsLocal bool
}

// String returns a string representation of the handle.
func (h Handle) String() string {
	if h.Name != "" {
		return fmt.Sprintf(":%08x(%s)", h.ID, h.Name)
	}
	return fmt.Sprintf(":%08x", h.ID)
}

// ServiceAddress represents different ways to address a service.
type ServiceAddress struct {
	// Handle for numeric addressing
	Handle *Handle

	// Name for name-based addressing
	Name string

	// Pattern for pattern-based addressing (e.g., ".service")
	Pattern string
}

// IsValid checks if the service address is valid.
func (sa ServiceAddress) IsValid() bool {
	return sa.Handle != nil || sa.Name != "" || sa.Pattern != ""
}

// String returns a string representation of the service address.
func (sa ServiceAddress) String() string {
	if sa.Handle != nil {
		return sa.Handle.String()
	}
	if sa.Name != "" {
		return sa.Name
	}
	if sa.Pattern != "" {
		return sa.Pattern
	}
	return "<invalid>"
}

// HandleManager manages the mapping between Handles and Actors.
type HandleManager struct {
	mu sync.RWMutex

	// Maps handle ID to Handle
	handles map[uint32]*Handle

	// Maps actor ID to handle ID
	actorToHandle map[ActorID]uint32

	// Maps service name to handle ID
	nameToHandle map[string]uint32

	// Counter for generating unique handle IDs
	handleCounter uint32

	// Local node ID
	nodeID uint32
}

// NewHandleManager creates a new HandleManager.
func NewHandleManager(nodeID uint32) *HandleManager {
	return &HandleManager{
		handles:       make(map[uint32]*Handle),
		actorToHandle: make(map[ActorID]uint32),
		nameToHandle:  make(map[string]uint32),
		nodeID:        nodeID,
		handleCounter: nodeID<<24 + 1, // Encode node ID in high bits
	}
}

// AllocateHandle creates a new handle for an actor.
func (hm *HandleManager) AllocateHandle(actorID ActorID, name string) (*Handle, error) {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	// Check if actor already has a handle
	if existingHandleID, exists := hm.actorToHandle[actorID]; exists {
		return hm.handles[existingHandleID], nil
	}

	// Check if name is already taken
	if name != "" {
		if _, exists := hm.nameToHandle[name]; exists {
			return nil, fmt.Errorf("service name '%s' already exists", name)
		}
	}

	// Generate new handle ID
	handleID := hm.handleCounter
	hm.handleCounter++

	// Create handle
	handle := &Handle{
		ID:      handleID,
		ActorID: actorID,
		Name:    name,
		Node:    hm.nodeID,
		IsLocal: true,
	}

	// Store mappings
	hm.handles[handleID] = handle
	hm.actorToHandle[actorID] = handleID
	if name != "" {
		hm.nameToHandle[name] = handleID
	}

	return handle, nil
}

// GetHandle retrieves a handle by ID.
func (hm *HandleManager) GetHandle(handleID uint32) (*Handle, bool) {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	handle, exists := hm.handles[handleID]
	return handle, exists
}

// GetHandleByActor retrieves a handle by actor ID.
func (hm *HandleManager) GetHandleByActor(actorID ActorID) (*Handle, bool) {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	if handleID, exists := hm.actorToHandle[actorID]; exists {
		return hm.handles[handleID], true
	}
	return nil, false
}

// GetHandleByName retrieves a handle by service name.
func (hm *HandleManager) GetHandleByName(name string) (*Handle, bool) {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	if handleID, exists := hm.nameToHandle[name]; exists {
		return hm.handles[handleID], true
	}
	return nil, false
}

// ReleaseHandle removes a handle and its mappings.
func (hm *HandleManager) ReleaseHandle(handleID uint32) error {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	handle, exists := hm.handles[handleID]
	if !exists {
		return fmt.Errorf("handle %d not found", handleID)
	}

	// Remove all mappings
	delete(hm.handles, handleID)
	delete(hm.actorToHandle, handle.ActorID)
	if handle.Name != "" {
		delete(hm.nameToHandle, handle.Name)
	}

	return nil
}

// ListHandles returns all handles.
func (hm *HandleManager) ListHandles() []*Handle {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	handles := make([]*Handle, 0, len(hm.handles))
	for _, handle := range hm.handles {
		handles = append(handles, handle)
	}
	return handles
}

// ResolveAddress resolves a service address to a specific handle.
func (hm *HandleManager) ResolveAddress(addr ServiceAddress) (*Handle, error) {
	if addr.Handle != nil {
		return addr.Handle, nil
	}

	if addr.Name != "" {
		if handle, exists := hm.GetHandleByName(addr.Name); exists {
			return handle, nil
		}
		return nil, fmt.Errorf("service '%s' not found", addr.Name)
	}

	if addr.Pattern != "" {
		// Pattern matching logic (simplified for now)
		// TODO: Implement pattern matching
		return nil, fmt.Errorf("pattern matching not implemented: %s", addr.Pattern)
	}

	return nil, fmt.Errorf("invalid service address")
}

// MessageEnvelope wraps a message with routing information.
type MessageEnvelope struct {
	// Source service address
	Source ServiceAddress `json:"source"`

	// Target service address
	Target ServiceAddress `json:"target"`

	// Message content
	Message *Message `json:"message"`

	// Routing flags
	Flags MessageFlags `json:"flags,omitempty"`
}

// MessageFlags define special routing behavior.
type MessageFlags uint32

const (
	// FlagDontCopy indicates the message data should not be copied
	FlagDontCopy MessageFlags = 1 << iota

	// FlagAllocSession indicates a new session should be allocated
	FlagAllocSession

	// FlagResponse indicates this is a response message
	FlagResponse

	// FlagMulticast indicates this should be sent to multiple targets
	FlagMulticast
)

// MarshalJSON customizes JSON serialization for MessageEnvelope.
func (me *MessageEnvelope) MarshalJSON() ([]byte, error) {
	type Alias MessageEnvelope
	return json.Marshal(&struct {
		Timestamp string `json:"timestamp"`
		*Alias
	}{
		Timestamp: me.Message.Timestamp.Format(time.RFC3339Nano),
		Alias:     (*Alias)(me),
	})
}

// UnmarshalJSON customizes JSON deserialization for MessageEnvelope.
func (me *MessageEnvelope) UnmarshalJSON(data []byte) error {
	type Alias MessageEnvelope
	aux := &struct {
		Timestamp string `json:"timestamp"`
		*Alias
	}{
		Alias: (*Alias)(me),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	if aux.Timestamp != "" {
		timestamp, err := time.Parse(time.RFC3339Nano, aux.Timestamp)
		if err != nil {
			return err
		}
		me.Message.Timestamp = timestamp
	}

	return nil
}
