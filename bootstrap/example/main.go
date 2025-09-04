// Example demonstrating the SNGO bootstrap system
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/najoast/sngo/bootstrap"
)

// ExampleService demonstrates a custom service implementation
type ExampleService struct {
	name   string
	config map[string]interface{}
}

func (s *ExampleService) Name() string {
	return s.name
}

func (s *ExampleService) Start(ctx context.Context) error {
	fmt.Printf("Starting %s service...\n", s.name)
	// Simulate some initialization work
	time.Sleep(100 * time.Millisecond)
	fmt.Printf("%s service started successfully\n", s.name)
	return nil
}

func (s *ExampleService) Stop(ctx context.Context) error {
	fmt.Printf("Stopping %s service...\n", s.name)
	// Simulate cleanup work
	time.Sleep(50 * time.Millisecond)
	fmt.Printf("%s service stopped successfully\n", s.name)
	return nil
}

func (s *ExampleService) Health(ctx context.Context) (bootstrap.HealthStatus, error) {
	return bootstrap.HealthStatus{
		State:   bootstrap.HealthHealthy,
		Message: fmt.Sprintf("%s service is healthy", s.name),
		Data: map[string]interface{}{
			"uptime": "5m",
		},
	}, nil
}

func main() {
	fmt.Println("=== SNGO Bootstrap System Demo ===")

	// Create application with builder pattern
	app, err := bootstrap.NewApplicationBuilder().
		WithActorSystemConfig().
		WithNetworkConfig("localhost:8080").
		WithService("database", &ExampleService{
			name: "database",
			config: map[string]interface{}{
				"host": "localhost",
				"port": 5432,
			},
		}).
		WithService("api", &ExampleService{
			name: "api",
		}, "database", "actor-system"). // API depends on database and actor system
		WithServiceFactory("cache", func(c bootstrap.Container) (interface{}, error) {
			fmt.Println("Creating cache service via factory...")
			return &ExampleService{name: "cache"}, nil
		}).
		Build()

	if err != nil {
		log.Fatalf("Failed to build application: %v", err)
	}

	fmt.Printf("Application configured with %d services\n", len(app.LifecycleManager().Services()))

	// Set up lifecycle event listener
	app.LifecycleManager().AddListener(func(event bootstrap.LifecycleEvent) {
		fmt.Printf("Event: %s", event.Type)
		if event.Service != "" {
			fmt.Printf(" (service: %s)", event.Service)
		}
		fmt.Println()
	})

	// Create context with timeout for the demo
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Demonstrate dependency injection
	fmt.Println("\n=== Dependency Injection Demo ===")
	container := app.Container()

	// Resolve cache service
	cacheService, err := container.Resolve("cache")
	if err != nil {
		log.Printf("Failed to resolve cache service: %v", err)
	} else {
		fmt.Printf("Resolved cache service: %T\n", cacheService)
	}

	// Demonstrate service lifecycle
	fmt.Println("\n=== Service Lifecycle Demo ===")

	// Start all services
	fmt.Println("Starting all services...")
	if err := app.LifecycleManager().Start(ctx); err != nil {
		log.Fatalf("Failed to start services: %v", err)
	}

	// Check health status
	fmt.Println("\n=== Health Check Demo ===")
	health, err := app.LifecycleManager().Health(ctx)
	if err != nil {
		log.Printf("Failed to get health status: %v", err)
	} else {
		fmt.Println("Service Health Status:")
		for serviceName, status := range health {
			fmt.Printf("  %s: %s - %s\n", serviceName, status.State, status.Message)
		}
	}

	// Simulate running for a short time
	fmt.Println("\n=== Running Demo ===")
	fmt.Println("Application running... (simulating 2 seconds of work)")
	time.Sleep(2 * time.Second)

	// Graceful shutdown
	fmt.Println("\n=== Graceful Shutdown Demo ===")
	fmt.Println("Shutting down application...")
	if err := app.Shutdown(ctx); err != nil {
		log.Printf("Error during shutdown: %v", err)
	} else {
		fmt.Println("Application shut down successfully")
	}

	fmt.Println("\n=== Demo Complete ===")
}
