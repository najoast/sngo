package core

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// router implements the Router interface.
type router struct {
	// Map of Actor ID to Actor instance
	actors sync.Map // map[ActorID]Actor

	// Counter for generating unique Actor IDs
	idCounter uint32
}

// NewRouter creates a new Router instance.
func NewRouter() Router {
	return &router{}
}

// Register adds an Actor to the routing table.
func (r *router) Register(actor Actor) error {
	if actor == nil {
		return fmt.Errorf("cannot register nil actor")
	}

	id := actor.ID()
	if _, exists := r.actors.LoadOrStore(id, actor); exists {
		return fmt.Errorf("actor with ID %d already registered", id)
	}

	return nil
}

// Unregister removes an Actor from the routing table.
func (r *router) Unregister(id ActorID) error {
	if _, exists := r.actors.LoadAndDelete(id); !exists {
		return fmt.Errorf("actor with ID %d not found", id)
	}

	return nil
}

// Route sends a message to the target Actor.
func (r *router) Route(msg *Message) error {
	if msg == nil {
		return fmt.Errorf("cannot route nil message")
	}

	actor, exists := r.actors.Load(msg.Target)
	if !exists {
		return fmt.Errorf("target actor %d not found", msg.Target)
	}

	return actor.(Actor).Send(msg)
}

// Lookup finds an Actor by its ID.
func (r *router) Lookup(id ActorID) (Actor, bool) {
	if actor, exists := r.actors.Load(id); exists {
		return actor.(Actor), true
	}
	return nil, false
}

// List returns all registered Actor IDs.
func (r *router) List() []ActorID {
	var ids []ActorID

	r.actors.Range(func(key, value interface{}) bool {
		ids = append(ids, key.(ActorID))
		return true
	})

	return ids
}

// NextID generates the next available Actor ID.
func (r *router) NextID() ActorID {
	return ActorID(atomic.AddUint32(&r.idCounter, 1))
}
