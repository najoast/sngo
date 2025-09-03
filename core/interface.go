package core

import (
	"context"
)

// MessageHandler processes incoming messages for an Actor.
type MessageHandler interface {
	// HandleMessage processes a single message.
	// It should return an error if processing fails.
	HandleMessage(ctx context.Context, msg *Message) error
}

// Actor represents a computational unit that processes messages sequentially.
// Each Actor runs in its own goroutine and communicates through channels.
type Actor interface {
	// ID returns the unique identifier of this Actor.
	ID() ActorID

	// Start begins the Actor's message processing loop.
	// It should be called only once per Actor instance.
	Start(ctx context.Context) error

	// Stop gracefully shuts down the Actor.
	// It will finish processing the current message before stopping.
	Stop() error

	// Send sends a message to this Actor's mailbox.
	// It returns an error if the Actor is stopped or mailbox is full.
	Send(msg *Message) error

	// Call sends a message and waits for a response.
	// It blocks until a response is received or timeout occurs.
	Call(ctx context.Context, msg *Message) (*Message, error)

	// Stats returns current runtime statistics for this Actor.
	Stats() ActorStats
}

// Router manages message routing between Actors.
type Router interface {
	// Register adds an Actor to the routing table.
	Register(actor Actor) error

	// Unregister removes an Actor from the routing table.
	Unregister(id ActorID) error

	// Route sends a message to the target Actor.
	Route(msg *Message) error

	// Lookup finds an Actor by its ID.
	Lookup(id ActorID) (Actor, bool)

	// List returns all registered Actor IDs.
	List() []ActorID
}

// ActorSystem manages the lifecycle of all Actors in the system.
type ActorSystem interface {
	// NewActor creates and registers a new Actor.
	NewActor(handler MessageHandler, opts ActorOptions) (Actor, error)

	// NewService creates and registers a named service.
	NewService(name string, handler MessageHandler, opts ActorOptions) (*Handle, error)

	// GetActor retrieves an Actor by its ID.
	GetActor(id ActorID) (Actor, bool)

	// GetService retrieves a service by name.
	GetService(name string) (*Handle, bool)

	// Send sends a message from one Actor to another.
	Send(from, to ActorID, msgType MessageType, data []byte) error

	// SendByName sends a message using service names.
	SendByName(from, to string, msgType MessageType, data []byte) error

	// Call makes a synchronous call from one Actor to another.
	Call(ctx context.Context, from, to ActorID, msgType MessageType, data []byte) ([]byte, error)

	// CallByName makes a synchronous call using service names.
	CallByName(ctx context.Context, from, to string, msgType MessageType, data []byte) ([]byte, error)

	// Shutdown gracefully stops all Actors in the system.
	Shutdown(ctx context.Context) error

	// Stats returns statistics for all Actors.
	Stats() []ActorStats

	// ListServices returns all registered services.
	ListServices() []*Handle

	// DiscoverService finds the best instance of a service
	DiscoverService(name string) (*ServiceInfo, error)

	// DiscoverServices finds all services matching criteria
	DiscoverServices(query ServiceQuery) ([]*ServiceInfo, error)

	// UpdateServiceHealth updates service health status
	UpdateServiceHealth(name string, status ServiceStatus) error

	// SetLoadBalanceStrategy sets the load balancing strategy
	SetLoadBalanceStrategy(strategy LoadBalanceStrategy) error
}

// Supervisor monitors Actor health and handles failures.
type Supervisor interface {
	// Watch starts monitoring an Actor.
	Watch(actor Actor) error

	// Unwatch stops monitoring an Actor.
	Unwatch(id ActorID) error

	// Restart attempts to restart a failed Actor.
	Restart(id ActorID) error
}
