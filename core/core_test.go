package core

import (
	"context"
	"testing"
	"time"
)

// echoHandler is a simple message handler for testing.
type echoHandler struct{}

func (h *echoHandler) HandleMessage(ctx context.Context, msg *Message) error {
	// Echo messages just return the same data
	return nil
}

func TestNewActor(t *testing.T) {
	handler := &echoHandler{}
	opts := DefaultActorOptions()
	opts.Name = "test-actor"

	actor := NewActor(1, handler, opts)

	if actor.ID() != 1 {
		t.Errorf("Expected actor ID 1, got %d", actor.ID())
	}

	stats := actor.Stats()
	if stats.Name != "test-actor" {
		t.Errorf("Expected actor name 'test-actor', got '%s'", stats.Name)
	}

	if stats.State != ActorStateIdle {
		t.Errorf("Expected initial state %s, got %s", ActorStateIdle, stats.State)
	}
}

func TestActorStartStop(t *testing.T) {
	handler := &echoHandler{}
	opts := DefaultActorOptions()

	actor := NewActor(2, handler, opts)

	// Test start
	ctx := context.Background()
	err := actor.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start actor: %v", err)
	}

	// Give it a moment to start
	time.Sleep(10 * time.Millisecond)

	// Test stop
	err = actor.Stop()
	if err != nil {
		t.Fatalf("Failed to stop actor: %v", err)
	}

	stats := actor.Stats()
	if stats.State != ActorStateStopped {
		t.Errorf("Expected final state %s, got %s", ActorStateStopped, stats.State)
	}
}

func TestActorSend(t *testing.T) {
	handler := &echoHandler{}
	opts := DefaultActorOptions()

	actor := NewActor(3, handler, opts)

	ctx := context.Background()
	err := actor.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start actor: %v", err)
	}
	defer actor.Stop()

	// Send a message
	msg := &Message{
		Type:      MessageTypeText,
		Source:    0,
		Target:    3,
		Data:      []byte("hello"),
		Timestamp: time.Now(),
	}

	err = actor.Send(msg)
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	// Give it time to process
	time.Sleep(10 * time.Millisecond)

	stats := actor.Stats()
	if stats.MessagesProcessed != 1 {
		t.Errorf("Expected 1 processed message, got %d", stats.MessagesProcessed)
	}
}

func TestRouter(t *testing.T) {
	router := NewRouter()

	handler := &echoHandler{}
	opts := DefaultActorOptions()

	actor1 := NewActor(10, handler, opts)
	actor2 := NewActor(20, handler, opts)

	// Test register
	err := router.Register(actor1)
	if err != nil {
		t.Fatalf("Failed to register actor1: %v", err)
	}

	err = router.Register(actor2)
	if err != nil {
		t.Fatalf("Failed to register actor2: %v", err)
	}

	// Test lookup
	found, exists := router.Lookup(10)
	if !exists {
		t.Fatal("Actor 10 not found")
	}
	if found.ID() != 10 {
		t.Errorf("Expected actor ID 10, got %d", found.ID())
	}

	// Test list
	ids := router.List()
	if len(ids) != 2 {
		t.Errorf("Expected 2 actors, got %d", len(ids))
	}

	// Test unregister
	err = router.Unregister(10)
	if err != nil {
		t.Fatalf("Failed to unregister actor: %v", err)
	}

	_, exists = router.Lookup(10)
	if exists {
		t.Error("Actor 10 should not exist after unregister")
	}
}

func TestActorSystem(t *testing.T) {
	system := NewActorSystem()

	handler := &echoHandler{}
	opts := DefaultActorOptions()
	opts.Name = "test-system-actor"

	// Create actor
	actor, err := system.NewActor(handler, opts)
	if err != nil {
		t.Fatalf("Failed to create actor: %v", err)
	}

	// Check if we can get it back
	found, exists := system.GetActor(actor.ID())
	if !exists {
		t.Fatal("Created actor not found in system")
	}

	if found.ID() != actor.ID() {
		t.Errorf("Expected actor ID %d, got %d", actor.ID(), found.ID())
	}

	// Test stats
	stats := system.Stats()
	if len(stats) != 1 {
		t.Errorf("Expected 1 actor in stats, got %d", len(stats))
	}

	// Test shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = system.Shutdown(ctx)
	if err != nil {
		t.Fatalf("Failed to shutdown system: %v", err)
	}
}
