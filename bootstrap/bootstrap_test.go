// Package bootstrap provides tests for the bootstrap module
package bootstrap

import (
	"context"
	"testing"
	"time"
)

func TestContainer(t *testing.T) {
	container := NewContainer()

	// Test service registration
	err := container.Register("test-service", func(c Container) (interface{}, error) {
		return "test-instance", nil
	})
	if err != nil {
		t.Fatalf("Failed to register service: %v", err)
	}

	// Test service resolution
	instance, err := container.Resolve("test-service")
	if err != nil {
		t.Fatalf("Failed to resolve service: %v", err)
	}

	if instance != "test-instance" {
		t.Errorf("Expected 'test-instance', got %v", instance)
	}

	// Test service exists
	if !container.Has("test-service") {
		t.Error("Container should have test-service")
	}

	// Test service names
	names := container.Names()
	if len(names) != 1 || names[0] != "test-service" {
		t.Errorf("Expected ['test-service'], got %v", names)
	}
}

func TestLifecycleManager(t *testing.T) {
	container := NewContainer()
	lm := NewLifecycleManager(container)

	// Create a test service
	testService := &TestService{name: "test"}

	// Register service
	err := lm.Register("test", testService)
	if err != nil {
		t.Fatalf("Failed to register service: %v", err)
	}

	// Test start
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = lm.Start(ctx)
	if err != nil {
		t.Fatalf("Failed to start services: %v", err)
	}

	if !testService.started {
		t.Error("Test service should be started")
	}

	// Test health check
	health, err := lm.Health(ctx)
	if err != nil {
		t.Fatalf("Failed to get health status: %v", err)
	}

	if health["test"].State != HealthHealthy {
		t.Errorf("Expected healthy state, got %v", health["test"].State)
	}

	// Test stop
	err = lm.Stop(ctx)
	if err != nil {
		t.Fatalf("Failed to stop services: %v", err)
	}

	if !testService.stopped {
		t.Error("Test service should be stopped")
	}
}

func TestApplication(t *testing.T) {
	app := NewApplication()

	// Test configuration
	config := map[string]interface{}{
		"network": map[string]interface{}{
			"address": "localhost:9999",
		},
	}

	err := app.Configure(config)
	if err != nil {
		t.Fatalf("Failed to configure application: %v", err)
	}

	// Test container access
	container := app.Container()
	if container == nil {
		t.Error("Application should have a container")
	}

	// Test lifecycle manager access
	lm := app.LifecycleManager()
	if lm == nil {
		t.Error("Application should have a lifecycle manager")
	}

	// Test services are registered
	services := lm.Services()
	if len(services) == 0 {
		t.Error("Application should have core services registered")
	}
}

func TestApplicationBuilder(t *testing.T) {
	builder := NewApplicationBuilder()

	app, err := builder.
		WithActorSystemConfig().
		WithNetworkConfig("localhost:8888").
		WithServiceFactory("test-factory", func(c Container) (interface{}, error) {
			return "factory-instance", nil
		}).
		Build()

	if err != nil {
		t.Fatalf("Failed to build application: %v", err)
	}

	// Test that the application was configured
	container := app.Container()
	if !container.Has("test-factory") {
		t.Error("Application should have test-factory service")
	}
}

func TestScopedContainer(t *testing.T) {
	container := NewScopedContainer()

	// Test singleton scope
	err := container.RegisterScoped("singleton", func(c Container) (interface{}, error) {
		return &TestService{name: "singleton"}, nil
	}, ScopeSingleton)
	if err != nil {
		t.Fatalf("Failed to register singleton service: %v", err)
	}

	// Resolve twice and check it's the same instance
	instance1, err := container.Resolve("singleton")
	if err != nil {
		t.Fatalf("Failed to resolve singleton service: %v", err)
	}

	instance2, err := container.Resolve("singleton")
	if err != nil {
		t.Fatalf("Failed to resolve singleton service: %v", err)
	}

	if instance1 != instance2 {
		t.Error("Singleton service should return the same instance")
	}

	// Test transient scope
	err = container.RegisterScoped("transient", func(c Container) (interface{}, error) {
		return &TestService{name: "transient"}, nil
	}, ScopeTransient)
	if err != nil {
		t.Fatalf("Failed to register transient service: %v", err)
	}

	// Resolve twice and check they're different instances
	instance3, err := container.Resolve("transient")
	if err != nil {
		t.Fatalf("Failed to resolve transient service: %v", err)
	}

	instance4, err := container.Resolve("transient")
	if err != nil {
		t.Fatalf("Failed to resolve transient service: %v", err)
	}

	if instance3 == instance4 {
		t.Error("Transient service should return different instances")
	}
}

// TestService is a simple service implementation for testing
type TestService struct {
	name    string
	started bool
	stopped bool
}

func (s *TestService) Name() string {
	return s.name
}

func (s *TestService) Start(ctx context.Context) error {
	s.started = true
	return nil
}

func (s *TestService) Stop(ctx context.Context) error {
	s.stopped = true
	return nil
}

func (s *TestService) Health(ctx context.Context) (HealthStatus, error) {
	if s.started && !s.stopped {
		return HealthStatus{
			State:   HealthHealthy,
			Message: "Service is running",
		}, nil
	}
	return HealthStatus{
		State:   HealthUnhealthy,
		Message: "Service is not running",
	}, nil
}
