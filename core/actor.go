package core

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// actor implements the Actor interface.
type actor struct {
	id      ActorID
	name    string
	handler MessageHandler

	// Channel for receiving messages
	mailbox chan *Message

	// Context for controlling the Actor lifecycle
	ctx    context.Context
	cancel context.CancelFunc

	// Wait group for graceful shutdown
	wg sync.WaitGroup

	// Atomic counters for statistics
	state             int32 // ActorState
	messagesProcessed uint64
	createdAt         time.Time
	lastMessageAt     int64 // Unix timestamp

	// Pending calls for synchronous communication
	pendingCalls   sync.Map // map[uint32]chan *Message
	sessionCounter uint32

	// Actor options
	opts ActorOptions
}

// NewActor creates a new Actor instance.
func NewActor(id ActorID, handler MessageHandler, opts ActorOptions) Actor {
	ctx, cancel := context.WithCancel(context.Background())

	a := &actor{
		id:        id,
		name:      opts.Name,
		handler:   handler,
		mailbox:   make(chan *Message, opts.MailboxSize),
		ctx:       ctx,
		cancel:    cancel,
		createdAt: time.Now(),
		opts:      opts,
	}

	// Set initial state
	atomic.StoreInt32(&a.state, int32(ActorStateIdle))

	return a
}

// ID returns the unique identifier of this Actor.
func (a *actor) ID() ActorID {
	return a.id
}

// Start begins the Actor's message processing loop.
func (a *actor) Start(ctx context.Context) error {
	currentState := ActorState(atomic.LoadInt32(&a.state))
	if currentState != ActorStateIdle {
		return fmt.Errorf("actor %d is already started (state: %s)", a.id, currentState)
	}

	a.wg.Add(1)
	go a.messageLoop()

	return nil
}

// Stop gracefully shuts down the Actor.
func (a *actor) Stop() error {
	// Set state to stopping
	if !atomic.CompareAndSwapInt32(&a.state, int32(ActorStateIdle), int32(ActorStateStopping)) &&
		!atomic.CompareAndSwapInt32(&a.state, int32(ActorStateRunning), int32(ActorStateStopping)) {
		return fmt.Errorf("actor %d cannot be stopped from state %s",
			a.id, ActorState(atomic.LoadInt32(&a.state)))
	}

	// Cancel context to signal shutdown
	a.cancel()

	// Wait for message loop to finish
	a.wg.Wait()

	// Set final state
	atomic.StoreInt32(&a.state, int32(ActorStateStopped))

	return nil
}

// Send sends a message to this Actor's mailbox.
func (a *actor) Send(msg *Message) error {
	currentState := ActorState(atomic.LoadInt32(&a.state))
	if currentState == ActorStateStopped || currentState == ActorStateStopping {
		return fmt.Errorf("actor %d is not running (state: %s)", a.id, currentState)
	}

	select {
	case a.mailbox <- msg:
		return nil
	case <-a.ctx.Done():
		return fmt.Errorf("actor %d is shutting down", a.id)
	default:
		return fmt.Errorf("actor %d mailbox is full", a.id)
	}
}

// Call sends a message and waits for a response.
func (a *actor) Call(ctx context.Context, msg *Message) (*Message, error) {
	// Generate a unique session ID
	session := atomic.AddUint32(&a.sessionCounter, 1)
	msg.Session = session

	// Create response channel
	respChan := make(chan *Message, 1)
	a.pendingCalls.Store(session, respChan)
	defer a.pendingCalls.Delete(session)

	// Send the message
	if err := a.Send(msg); err != nil {
		return nil, err
	}

	// Wait for response or timeout
	select {
	case resp := <-respChan:
		return resp, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-a.ctx.Done():
		return nil, fmt.Errorf("actor %d is shutting down", a.id)
	}
}

// Stats returns current runtime statistics for this Actor.
func (a *actor) Stats() ActorStats {
	lastMsg := atomic.LoadInt64(&a.lastMessageAt)
	var lastMessageAt time.Time
	if lastMsg > 0 {
		lastMessageAt = time.Unix(lastMsg, 0)
	}

	return ActorStats{
		ID:                a.id,
		Name:              a.name,
		State:             ActorState(atomic.LoadInt32(&a.state)),
		MessagesProcessed: atomic.LoadUint64(&a.messagesProcessed),
		MailboxSize:       len(a.mailbox),
		CreatedAt:         a.createdAt,
		LastMessageAt:     lastMessageAt,
	}
}

// messageLoop is the main processing loop for the Actor.
func (a *actor) messageLoop() {
	defer a.wg.Done()
	defer close(a.mailbox)

	for {
		select {
		case msg := <-a.mailbox:
			if msg == nil {
				continue
			}
			a.processMessage(msg)

		case <-a.ctx.Done():
			// Process remaining messages before shutting down
			a.drainMailbox()
			return
		}
	}
}

// processMessage handles a single message.
func (a *actor) processMessage(msg *Message) {
	// Set state to running
	atomic.StoreInt32(&a.state, int32(ActorStateRunning))
	defer atomic.StoreInt32(&a.state, int32(ActorStateIdle))

	// Update statistics
	atomic.AddUint64(&a.messagesProcessed, 1)
	atomic.StoreInt64(&a.lastMessageAt, time.Now().Unix())

	// Create context with timeout
	ctx, cancel := context.WithTimeout(a.ctx, a.opts.ProcessTimeout)
	defer cancel()

	// Handle the message
	err := a.handler.HandleMessage(ctx, msg)

	// If this was a call (has session), send response
	if msg.Session != 0 {
		a.sendResponse(msg, err)
	}
}

// sendResponse sends a response message for a call.
func (a *actor) sendResponse(originalMsg *Message, err error) {
	if respChan, ok := a.pendingCalls.Load(originalMsg.Session); ok {
		ch := respChan.(chan *Message)

		resp := &Message{
			Type:      MessageTypeResponse,
			Source:    a.id,
			Target:    originalMsg.Source,
			Session:   originalMsg.Session,
			Timestamp: time.Now(),
		}

		if err != nil {
			resp.Type = MessageTypeError
			resp.Data = []byte(err.Error())
		}

		select {
		case ch <- resp:
		default:
			// Response channel is full or closed, ignore
		}
	}
}

// drainMailbox processes remaining messages during shutdown.
func (a *actor) drainMailbox() {
	for {
		select {
		case msg := <-a.mailbox:
			if msg == nil {
				return
			}
			// Send error response for any pending calls
			if msg.Session != 0 {
				a.sendResponse(msg, fmt.Errorf("actor %d is shutting down", a.id))
			}
		default:
			return
		}
	}
}
