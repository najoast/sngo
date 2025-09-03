package core

import (
	"time"
)

// ActorID represents a unique identifier for an Actor.
type ActorID uint32

// MessageType defines the type of message being sent.
type MessageType uint8

// Message represents communication data between Actors.
type Message struct {
	// ID is a unique identifier for this message
	ID uint64

	// Type indicates the message category
	Type MessageType

	// Source is the ID of the sending Actor
	Source ActorID

	// Target is the ID of the receiving Actor
	Target ActorID

	// Session is used for request-response correlation
	Session uint32

	// Data contains the actual message payload
	Data []byte

	// Timestamp when the message was created
	Timestamp time.Time
}

// ActorState represents the current state of an Actor.
type ActorState uint8

const (
	// ActorStateIdle means the Actor is waiting for messages
	ActorStateIdle ActorState = iota

	// ActorStateRunning means the Actor is processing a message
	ActorStateRunning

	// ActorStateStopping means the Actor is shutting down
	ActorStateStopping

	// ActorStateStopped means the Actor has been stopped
	ActorStateStopped
)

// String returns the string representation of ActorState.
func (s ActorState) String() string {
	switch s {
	case ActorStateIdle:
		return "idle"
	case ActorStateRunning:
		return "running"
	case ActorStateStopping:
		return "stopping"
	case ActorStateStopped:
		return "stopped"
	default:
		return "unknown"
	}
}

// MessageTypes define various message categories similar to Skynet.
const (
	// MessageTypeText for plain text messages
	MessageTypeText MessageType = iota

	// MessageTypeResponse for response messages
	MessageTypeResponse

	// MessageTypeRequest for request messages
	MessageTypeRequest

	// MessageTypeSystem for system control messages
	MessageTypeSystem

	// MessageTypeError for error notifications
	MessageTypeError

	// MessageTypeMulticast for multicast messages
	MessageTypeMulticast
)

// String returns the string representation of MessageType.
func (t MessageType) String() string {
	switch t {
	case MessageTypeText:
		return "text"
	case MessageTypeResponse:
		return "response"
	case MessageTypeRequest:
		return "request"
	case MessageTypeSystem:
		return "system"
	case MessageTypeError:
		return "error"
	case MessageTypeMulticast:
		return "multicast"
	default:
		return "unknown"
	}
}

// ActorOptions contains configuration options for creating an Actor.
type ActorOptions struct {
	// MailboxSize sets the size of the Actor's message queue
	MailboxSize int

	// Name is a human-readable name for the Actor
	Name string

	// Timeout for message processing
	ProcessTimeout time.Duration
}

// DefaultActorOptions returns sensible default options.
func DefaultActorOptions() ActorOptions {
	return ActorOptions{
		MailboxSize:    1000,
		Name:           "",
		ProcessTimeout: 30 * time.Second,
	}
}

// ActorStats contains runtime statistics for an Actor.
type ActorStats struct {
	// ID of the Actor
	ID ActorID

	// Name of the Actor
	Name string

	// Current state
	State ActorState

	// Total messages processed
	MessagesProcessed uint64

	// Messages currently in mailbox
	MailboxSize int

	// Time when Actor was created
	CreatedAt time.Time

	// Last message processing time
	LastMessageAt time.Time
}
