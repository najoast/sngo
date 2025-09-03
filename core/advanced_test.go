package core

import (
	"context"
	"testing"
	"time"
)

func TestHandleManager(t *testing.T) {
	hm := NewHandleManager(1)

	// Test allocate handle
	handle1, err := hm.AllocateHandle(100, "test-service")
	if err != nil {
		t.Fatalf("Failed to allocate handle: %v", err)
	}

	if handle1.ActorID != 100 {
		t.Errorf("Expected actor ID 100, got %d", handle1.ActorID)
	}

	if handle1.Name != "test-service" {
		t.Errorf("Expected service name 'test-service', got '%s'", handle1.Name)
	}

	// Test get handle by ID
	found, exists := hm.GetHandle(handle1.ID)
	if !exists {
		t.Fatal("Handle not found by ID")
	}
	if found.ActorID != handle1.ActorID {
		t.Errorf("Expected actor ID %d, got %d", handle1.ActorID, found.ActorID)
	}

	// Test get handle by actor
	foundByActor, exists := hm.GetHandleByActor(100)
	if !exists {
		t.Fatal("Handle not found by actor ID")
	}
	if foundByActor.ID != handle1.ID {
		t.Errorf("Expected handle ID %d, got %d", handle1.ID, foundByActor.ID)
	}

	// Test get handle by name
	foundByName, exists := hm.GetHandleByName("test-service")
	if !exists {
		t.Fatal("Handle not found by name")
	}
	if foundByName.ID != handle1.ID {
		t.Errorf("Expected handle ID %d, got %d", handle1.ID, foundByName.ID)
	}

	// Test duplicate name
	_, err = hm.AllocateHandle(200, "test-service")
	if err == nil {
		t.Error("Expected error for duplicate service name")
	}

	// Test release handle
	err = hm.ReleaseHandle(handle1.ID)
	if err != nil {
		t.Fatalf("Failed to release handle: %v", err)
	}

	// Verify handle is gone
	_, exists = hm.GetHandle(handle1.ID)
	if exists {
		t.Error("Handle should not exist after release")
	}
}

func TestAdvancedRouter(t *testing.T) {
	router := NewAdvancedRouter(1)

	handler := &echoHandler{}
	opts := DefaultActorOptions()
	opts.Name = "test-actor"

	actor1 := NewActor(10, handler, opts)

	// Test register service
	handle, err := router.RegisterService(actor1, "echo-service")
	if err != nil {
		t.Fatalf("Failed to register service: %v", err)
	}

	if handle.Name != "echo-service" {
		t.Errorf("Expected service name 'echo-service', got '%s'", handle.Name)
	}

	// Test lookup service
	foundHandle, exists := router.LookupService("echo-service")
	if !exists {
		t.Fatal("Service not found")
	}
	if foundHandle.ID != handle.ID {
		t.Errorf("Expected handle ID %d, got %d", handle.ID, foundHandle.ID)
	}

	// Test route by name
	ctx := context.Background()
	err = actor1.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start actor: %v", err)
	}
	defer actor1.Stop()

	msg := &Message{
		Type:      MessageTypeText,
		Data:      []byte("hello"),
		Timestamp: time.Now(),
	}

	err = router.RouteByName("", "echo-service", msg)
	if err != nil {
		t.Fatalf("Failed to route by name: %v", err)
	}

	// Give it time to process
	time.Sleep(10 * time.Millisecond)

	stats := actor1.Stats()
	if stats.MessagesProcessed != 1 {
		t.Errorf("Expected 1 processed message, got %d", stats.MessagesProcessed)
	}
}

func TestAdvancedActorSystem(t *testing.T) {
	system := NewActorSystemWithNodeID(2)

	handler := &echoHandler{}
	opts := DefaultActorOptions()

	// Test create service
	handle, err := system.NewService("math-service", handler, opts)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	if handle.Name != "math-service" {
		t.Errorf("Expected service name 'math-service', got '%s'", handle.Name)
	}

	// Test get service
	foundHandle, exists := system.GetService("math-service")
	if !exists {
		t.Fatal("Service not found")
	}
	if foundHandle.ID != handle.ID {
		t.Errorf("Expected handle ID %d, got %d", handle.ID, foundHandle.ID)
	}

	// Test send by name
	err = system.SendByName("", "math-service", MessageTypeText, []byte("test"))
	if err != nil {
		t.Fatalf("Failed to send by name: %v", err)
	}

	// Give it time to process
	time.Sleep(10 * time.Millisecond)

	// Test list services
	services := system.ListServices()
	if len(services) != 1 {
		t.Errorf("Expected 1 service, got %d", len(services))
	}

	if services[0].Name != "math-service" {
		t.Errorf("Expected service name 'math-service', got '%s'", services[0].Name)
	}

	// Test shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = system.Shutdown(ctx)
	if err != nil {
		t.Fatalf("Failed to shutdown system: %v", err)
	}
}

func TestMessageEnvelope(t *testing.T) {
	handle1 := &Handle{
		ID:      1001,
		ActorID: 10,
		Name:    "service1",
		Node:    1,
		IsLocal: true,
	}

	handle2 := &Handle{
		ID:      1002,
		ActorID: 20,
		Name:    "service2",
		Node:    1,
		IsLocal: true,
	}

	msg := &Message{
		Type:      MessageTypeRequest,
		Source:    10,
		Target:    20,
		Data:      []byte("test message"),
		Timestamp: time.Now(),
	}

	envelope := &MessageEnvelope{
		Source:  ServiceAddress{Handle: handle1},
		Target:  ServiceAddress{Handle: handle2},
		Message: msg,
		Flags:   FlagAllocSession,
	}

	// Test JSON serialization
	data, err := envelope.MarshalJSON()
	if err != nil {
		t.Fatalf("Failed to marshal envelope: %v", err)
	}

	// Test JSON deserialization
	var newEnvelope MessageEnvelope
	err = newEnvelope.UnmarshalJSON(data)
	if err != nil {
		t.Fatalf("Failed to unmarshal envelope: %v", err)
	}

	if newEnvelope.Source.Handle.Name != "service1" {
		t.Errorf("Expected source service 'service1', got '%s'", newEnvelope.Source.Handle.Name)
	}

	if newEnvelope.Target.Handle.Name != "service2" {
		t.Errorf("Expected target service 'service2', got '%s'", newEnvelope.Target.Handle.Name)
	}

	if string(newEnvelope.Message.Data) != "test message" {
		t.Errorf("Expected message 'test message', got '%s'", string(newEnvelope.Message.Data))
	}
}

func TestSessionManager(t *testing.T) {
	sm := NewSessionManager()

	// Test create session
	session, err := sm.CreateSession(10, 20, 5*time.Second)
	if err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	if session.Source != 10 {
		t.Errorf("Expected source 10, got %d", session.Source)
	}

	if session.Target != 20 {
		t.Errorf("Expected target 20, got %d", session.Target)
	}

	// Test get session
	foundSession, exists := sm.GetSession(session.ID)
	if !exists {
		t.Fatal("Session not found")
	}
	if foundSession.ID != session.ID {
		t.Errorf("Expected session ID %d, got %d", session.ID, foundSession.ID)
	}

	// Test complete session
	response := &Message{
		Type:      MessageTypeResponse,
		Source:    20,
		Target:    10,
		Session:   session.ID,
		Data:      []byte("response"),
		Timestamp: time.Now(),
	}

	err = sm.CompleteSession(session.ID, response)
	if err != nil {
		t.Fatalf("Failed to complete session: %v", err)
	}

	// Test wait for response
	ctx := context.Background()
	resp, err := session.WaitForResponse(ctx)
	if err != nil {
		t.Fatalf("Failed to wait for response: %v", err)
	}

	if string(resp.Data) != "response" {
		t.Errorf("Expected response 'response', got '%s'", string(resp.Data))
	}

	// Verify session is cleaned up
	_, exists = sm.GetSession(session.ID)
	if exists {
		t.Error("Session should not exist after completion")
	}
}
