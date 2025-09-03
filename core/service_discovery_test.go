package core

import (
	"context"
	"testing"
	"time"
)

func TestServiceRegistry(t *testing.T) {
	registry := NewServiceRegistry()

	// Create test service info
	handle := &Handle{
		ID:      1001,
		ActorID: 100,
		Name:    "test-service",
		Node:    1,
		IsLocal: true,
	}

	serviceInfo := &ServiceInfo{
		Handle:              handle,
		Description:         "Test service",
		Version:             "1.0.0",
		Tags:                []string{"test", "example"},
		Status:              ServiceStatusHealthy,
		Metadata:            map[string]string{"env": "test"},
		HealthCheckInterval: 30 * time.Second,
	}

	// Test registration
	err := registry.Register(serviceInfo)
	if err != nil {
		t.Fatalf("Failed to register service: %v", err)
	}

	// Test get service
	found, err := registry.Get("test-service")
	if err != nil {
		t.Fatalf("Failed to get service: %v", err)
	}

	if found.Handle.Name != "test-service" {
		t.Errorf("Expected service name 'test-service', got '%s'", found.Handle.Name)
	}

	// Test discovery with exact name
	services, err := registry.Discover(ServiceQuery{Name: "test-service"})
	if err != nil {
		t.Fatalf("Failed to discover service: %v", err)
	}

	if len(services) != 1 {
		t.Errorf("Expected 1 service, got %d", len(services))
	}

	// Test discovery with tags
	services, err = registry.Discover(ServiceQuery{Tags: []string{"test"}})
	if err != nil {
		t.Fatalf("Failed to discover service by tags: %v", err)
	}

	if len(services) != 1 {
		t.Errorf("Expected 1 service with tag 'test', got %d", len(services))
	}

	// Test discovery with metadata
	services, err = registry.Discover(ServiceQuery{
		Metadata: map[string]string{"env": "test"},
	})
	if err != nil {
		t.Fatalf("Failed to discover service by metadata: %v", err)
	}

	if len(services) != 1 {
		t.Errorf("Expected 1 service with metadata 'env=test', got %d", len(services))
	}

	// Test status update
	err = registry.UpdateStatus("test-service", ServiceStatusUnhealthy)
	if err != nil {
		t.Fatalf("Failed to update service status: %v", err)
	}

	found, _ = registry.Get("test-service")
	if found.Status != ServiceStatusUnhealthy {
		t.Errorf("Expected status %s, got %s", ServiceStatusUnhealthy, found.Status)
	}

	// Test metadata update
	newMetadata := map[string]string{"version": "2.0"}
	err = registry.UpdateMetadata("test-service", newMetadata)
	if err != nil {
		t.Fatalf("Failed to update service metadata: %v", err)
	}

	found, _ = registry.Get("test-service")
	if found.Metadata["version"] != "2.0" {
		t.Errorf("Expected metadata version '2.0', got '%s'", found.Metadata["version"])
	}

	// Test unregistration
	err = registry.Unregister("test-service")
	if err != nil {
		t.Fatalf("Failed to unregister service: %v", err)
	}

	_, err = registry.Get("test-service")
	if err == nil {
		t.Error("Expected error getting unregistered service")
	}
}

func TestServiceRegistryWatch(t *testing.T) {
	registry := NewServiceRegistry()

	// Start watching
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	eventChan, err := registry.Watch(ctx)
	if err != nil {
		t.Fatalf("Failed to start watching: %v", err)
	}

	// Register a service
	handle := &Handle{
		ID:      1002,
		ActorID: 200,
		Name:    "watch-test-service",
		Node:    1,
		IsLocal: true,
	}

	serviceInfo := &ServiceInfo{
		Handle:              handle,
		Status:              ServiceStatusHealthy,
		HealthCheckInterval: 30 * time.Second,
	}

	go func() {
		time.Sleep(100 * time.Millisecond)
		registry.Register(serviceInfo)
	}()

	// Wait for event
	select {
	case event := <-eventChan:
		if event.Type != ServiceEventRegister {
			t.Errorf("Expected register event, got %s", event.Type)
		}
		if event.Service.Handle.Name != "watch-test-service" {
			t.Errorf("Expected service name 'watch-test-service', got '%s'", event.Service.Handle.Name)
		}
	case <-time.After(2 * time.Second):
		t.Error("Timeout waiting for register event")
	}
}

func TestLoadBalancer(t *testing.T) {
	// Test round robin
	lb := NewLoadBalancer(StrategyRoundRobin)

	services := []*ServiceInfo{
		{Handle: &Handle{Name: "service1"}, Status: ServiceStatusHealthy},
		{Handle: &Handle{Name: "service2"}, Status: ServiceStatusHealthy},
		{Handle: &Handle{Name: "service3"}, Status: ServiceStatusHealthy},
	}

	// Test multiple selections
	selections := make(map[string]int)
	for i := 0; i < 6; i++ {
		selected, err := lb.Select(services)
		if err != nil {
			t.Fatalf("Failed to select service: %v", err)
		}
		selections[selected.Handle.Name]++
	}

	// Each service should be selected twice (6 total / 3 services)
	for name, count := range selections {
		if count != 2 {
			t.Errorf("Service %s selected %d times, expected 2", name, count)
		}
	}

	// Test random selection
	lbRandom := NewLoadBalancer(StrategyRandom)

	// Should work without error
	_, err := lbRandom.Select(services)
	if err != nil {
		t.Fatalf("Failed random selection: %v", err)
	}

	// Test least connections
	lbLC := NewLoadBalancer(StrategyLeastConnections)

	// Update metrics for services
	lbLC.UpdateMetrics("service1", ServiceMetrics{ActiveConnections: 5})
	lbLC.UpdateMetrics("service2", ServiceMetrics{ActiveConnections: 2})
	lbLC.UpdateMetrics("service3", ServiceMetrics{ActiveConnections: 8})

	selected, err := lbLC.Select(services)
	if err != nil {
		t.Fatalf("Failed least connections selection: %v", err)
	}

	// Should select service2 (least connections)
	if selected.Handle.Name != "service2" {
		t.Errorf("Expected service2 (least connections), got %s", selected.Handle.Name)
	}

	// Test with unhealthy services
	services[1].Status = ServiceStatusUnhealthy

	// Should only select from healthy services
	for i := 0; i < 10; i++ {
		selected, err := lb.Select(services)
		if err != nil {
			t.Fatalf("Failed to select from healthy services: %v", err)
		}
		if selected.Handle.Name == "service2" {
			t.Error("Selected unhealthy service")
		}
	}

	// Test with no healthy services
	for _, service := range services {
		service.Status = ServiceStatusUnhealthy
	}

	_, err = lb.Select(services)
	if err == nil {
		t.Error("Expected error when no healthy services available")
	}
}

func TestServiceDiscovery(t *testing.T) {
	sd := NewServiceDiscovery()

	// Create test handle
	handle := &Handle{
		ID:      1003,
		ActorID: 300,
		Name:    "discovery-test-service",
		Node:    1,
		IsLocal: true,
	}

	// Register service
	regInfo := ServiceRegistrationInfo{
		Description:         "Discovery test service",
		Version:             "1.0.0",
		Tags:                []string{"test", "discovery"},
		Metadata:            map[string]string{"type": "test"},
		HealthCheckInterval: 30 * time.Second,
	}

	err := sd.RegisterService(handle, regInfo)
	if err != nil {
		t.Fatalf("Failed to register service: %v", err)
	}

	// Discover service
	discovered, err := sd.DiscoverService("discovery-test-service")
	if err != nil {
		t.Fatalf("Failed to discover service: %v", err)
	}

	if discovered.Handle.Name != "discovery-test-service" {
		t.Errorf("Expected service name 'discovery-test-service', got '%s'", discovered.Handle.Name)
	}

	// Test service discovery with query
	services, err := sd.DiscoverServices(ServiceQuery{
		Tags: []string{"test"},
	})
	if err != nil {
		t.Fatalf("Failed to discover services: %v", err)
	}

	if len(services) != 1 {
		t.Errorf("Expected 1 service, got %d", len(services))
	}

	// Test health update
	err = sd.UpdateServiceHealth("discovery-test-service", ServiceStatusUnhealthy)
	if err != nil {
		t.Fatalf("Failed to update service health: %v", err)
	}

	// Service should not be discoverable when unhealthy (depending on load balancer)
	// This depends on the load balancer filtering healthy services

	// Test load balance strategy change
	err = sd.SetLoadBalanceStrategy(StrategyRandom)
	if err != nil {
		t.Fatalf("Failed to set load balance strategy: %v", err)
	}

	// Unregister service
	err = sd.UnregisterService("discovery-test-service")
	if err != nil {
		t.Fatalf("Failed to unregister service: %v", err)
	}

	// Service should not be discoverable after unregistration
	_, err = sd.DiscoverService("discovery-test-service")
	if err == nil {
		t.Error("Expected error discovering unregistered service")
	}
}

func TestServiceMetrics(t *testing.T) {
	metrics := ServiceMetrics{
		TotalRequests:       100,
		FailedRequests:      5,
		AverageResponseTime: 50 * time.Millisecond,
		CPUUsage:            75.5,
		MemoryUsage:         60.2,
	}

	successRate := metrics.SuccessRate()
	expectedRate := 0.95 // 95/100

	if successRate != expectedRate {
		t.Errorf("Expected success rate %f, got %f", expectedRate, successRate)
	}

	// Test with zero requests
	zeroMetrics := ServiceMetrics{}
	if zeroMetrics.SuccessRate() != 1.0 {
		t.Error("Expected success rate 1.0 for zero requests")
	}
}

func TestIntegratedServiceDiscovery(t *testing.T) {
	system := NewActorSystemWithNodeID(1)

	handler := &echoHandler{}
	opts := DefaultActorOptions()

	// Create multiple instances of the same service
	_, err := system.NewService("echo-service", handler, opts)
	if err != nil {
		t.Fatalf("Failed to create first service instance: %v", err)
	}

	// Test discovery
	serviceInfo, err := system.DiscoverService("echo-service")
	if err != nil {
		t.Fatalf("Failed to discover service: %v", err)
	}

	if serviceInfo.Handle.Name != "echo-service" {
		t.Errorf("Expected service name 'echo-service', got '%s'", serviceInfo.Handle.Name)
	}

	// Test discovery query
	services, err := system.DiscoverServices(ServiceQuery{
		Tags: []string{"sngo-service"},
	})
	if err != nil {
		t.Fatalf("Failed to discover services: %v", err)
	}

	if len(services) < 1 {
		t.Error("Expected at least 1 service with tag 'sngo-service'")
	}

	// Test health update
	err = system.UpdateServiceHealth("echo-service", ServiceStatusMaintenance)
	if err != nil {
		t.Fatalf("Failed to update service health: %v", err)
	}

	// Test load balance strategy
	err = system.SetLoadBalanceStrategy(StrategyLeastConnections)
	if err != nil {
		t.Fatalf("Failed to set load balance strategy: %v", err)
	}

	// Cleanup
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = system.Shutdown(ctx)
	if err != nil {
		t.Fatalf("Failed to shutdown system: %v", err)
	}
}
