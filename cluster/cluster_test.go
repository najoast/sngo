package cluster

import (
	"context"
	"testing"
	"time"
)

// TestClusterManager tests basic cluster manager functionality
func TestClusterManager(t *testing.T) {
	config := DefaultClusterConfig()
	config.NodeID = "test-node-1"
	config.BindPort = 0 // Use random port for testing

	manager := NewClusterManager(config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Test start
	if err := manager.Start(ctx); err != nil {
		t.Fatalf("Failed to start cluster manager: %v", err)
	}

	// Test local node
	localNode := manager.LocalNode()
	if localNode == nil {
		t.Fatal("Local node is nil")
	}

	if localNode.ID() != config.NodeID {
		t.Errorf("Expected node ID %s, got %s", config.NodeID, localNode.ID())
	}

	if !localNode.IsLocal() {
		t.Error("Local node should return true for IsLocal()")
	}

	// Test cluster health
	health := manager.GetClusterHealth()
	if health.TotalNodes != 1 {
		t.Errorf("Expected 1 total node, got %d", health.TotalNodes)
	}

	if health.ActiveNodes != 1 {
		t.Errorf("Expected 1 active node, got %d", health.ActiveNodes)
	}

	if !health.HasLeader {
		t.Error("Single node cluster should have a leader")
	}

	// Test stop
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()

	if err := manager.Stop(stopCtx); err != nil {
		t.Fatalf("Failed to stop cluster manager: %v", err)
	}
}

// TestMessageTransport tests basic message transport functionality
func TestMessageTransport(t *testing.T) {
	config := DefaultClusterConfig()
	config.BindPort = 0 // Use random port for testing

	transport := NewMessageTransport(config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Test start
	if err := transport.Start(ctx); err != nil {
		t.Fatalf("Failed to start transport: %v", err)
	}

	// Test statistics
	stats := transport.GetStatistics()
	if stats.ConnectionsOpen != 0 {
		t.Errorf("Expected 0 connections, got %d", stats.ConnectionsOpen)
	}

	// Test stop
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()

	if err := transport.Stop(stopCtx); err != nil {
		t.Fatalf("Failed to stop transport: %v", err)
	}
}

// TestRemoteService tests basic remote service functionality
func TestRemoteService(t *testing.T) {
	// Create a mock cluster manager
	config := DefaultClusterConfig()
	config.NodeID = "test-node"

	manager := NewClusterManager(config)
	service := NewRemoteService(manager)

	// Test service registration
	handler := &testHandler{}
	if err := service.Register("test-service", handler); err != nil {
		t.Fatalf("Failed to register service: %v", err)
	}

	// Test service unregistration
	if err := service.Unregister("test-service"); err != nil {
		t.Fatalf("Failed to unregister service: %v", err)
	}
}

// TestServiceRegistry tests basic service registry functionality
func TestServiceRegistry(t *testing.T) {
	// Create a mock cluster manager
	config := DefaultClusterConfig()
	config.NodeID = "test-node"

	manager := NewClusterManager(config)
	registry := NewServiceRegistry(manager)

	ctx := context.Background()

	// Test service registration
	metadata := map[string]string{"type": "test"}
	if err := registry.RegisterService(ctx, "test-service", metadata); err != nil {
		t.Fatalf("Failed to register service: %v", err)
	}

	// Test service discovery
	instances, err := registry.DiscoverService(ctx, "test-service")
	if err != nil {
		t.Fatalf("Failed to discover service: %v", err)
	}

	if len(instances) != 1 {
		t.Errorf("Expected 1 service instance, got %d", len(instances))
	}

	if instances[0].ServiceID != "test-service" {
		t.Errorf("Expected service ID 'test-service', got %s", instances[0].ServiceID)
	}

	// Test service unregistration
	if err := registry.UnregisterService(ctx, "test-service"); err != nil {
		t.Fatalf("Failed to unregister service: %v", err)
	}

	// Verify service is removed
	instances, err = registry.DiscoverService(ctx, "test-service")
	if err != nil {
		t.Fatalf("Failed to discover service after unregistration: %v", err)
	}

	if len(instances) != 0 {
		t.Errorf("Expected 0 service instances after unregistration, got %d", len(instances))
	}
}

// TestClusterService tests the bootstrap integration
func TestClusterService(t *testing.T) {
	config := DefaultClusterConfig()
	config.BindPort = 0 // Use random port for testing

	service := NewClusterService(config)

	if service.Name() != "cluster" {
		t.Errorf("Expected service name 'cluster', got %s", service.Name())
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Test start
	if err := service.Start(ctx); err != nil {
		t.Fatalf("Failed to start cluster service: %v", err)
	}

	// Test health check
	health, err := service.Health(ctx)
	if err != nil {
		t.Fatalf("Health check failed: %v", err)
	}

	if health.State != "healthy" && health.State != "unhealthy" {
		t.Errorf("Unexpected health state: %s", health.State)
	}

	// Test manager access
	manager := service.GetManager()
	if manager == nil {
		t.Error("Cluster manager should not be nil")
	}

	// Test stop
	stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer stopCancel()

	if err := service.Stop(stopCtx); err != nil {
		t.Fatalf("Failed to stop cluster service: %v", err)
	}
}

// testHandler is a test implementation of RemoteCallHandler
type testHandler struct{}

func (th *testHandler) Handle(ctx context.Context, request interface{}) (interface{}, error) {
	return "test-response", nil
}

// BenchmarkClusterManager benchmarks cluster manager operations
func BenchmarkClusterManager(b *testing.B) {
	config := DefaultClusterConfig()
	config.NodeID = "bench-node"
	config.BindPort = 0

	manager := NewClusterManager(config)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := manager.Start(ctx); err != nil {
		b.Fatalf("Failed to start cluster manager: %v", err)
	}
	defer manager.Stop(ctx)

	b.ResetTimer()

	b.Run("GetClusterHealth", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = manager.GetClusterHealth()
		}
	})

	b.Run("GetActiveNodes", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = manager.GetActiveNodes()
		}
	})

	b.Run("IsLeader", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = manager.IsLeader()
		}
	})
}
