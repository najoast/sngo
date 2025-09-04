// Package bootstrap provides application lifecycle management and dependency injection for SNGO
package bootstrap

import (
	"context"
	"fmt"
	"time"
)

// Service represents a service that can be managed by the lifecycle manager
type Service interface {
	// Start starts the service
	Start(ctx context.Context) error

	// Stop stops the service
	Stop(ctx context.Context) error

	// Health returns the health status of the service
	Health(ctx context.Context) (HealthStatus, error)

	// Name returns the service name
	Name() string
}

// HealthStatus represents the health status of a service
type HealthStatus struct {
	// State indicates whether the service is healthy
	State HealthState `json:"state"`

	// Message provides additional information about the health status
	Message string `json:"message,omitempty"`

	// LastCheck is the timestamp of the last health check
	LastCheck time.Time `json:"last_check,omitempty"`

	// Data contains additional health information
	Data map[string]interface{} `json:"data,omitempty"`
}

// HealthState represents the health state of a service
type HealthState string

const (
	// HealthUnknown indicates the health status is unknown
	HealthUnknown HealthState = "unknown"

	// HealthStarting indicates the service is starting up
	HealthStarting HealthState = "starting"

	// HealthHealthy indicates the service is healthy and operational
	HealthHealthy HealthState = "healthy"

	// HealthUnhealthy indicates the service is unhealthy but may recover
	HealthUnhealthy HealthState = "unhealthy"

	// HealthCritical indicates the service is in a critical state
	HealthCritical HealthState = "critical"

	// HealthStopping indicates the service is shutting down
	HealthStopping HealthState = "stopping"

	// HealthStopped indicates the service has stopped
	HealthStopped HealthState = "stopped"
)

// Container provides dependency injection capabilities
type Container interface {
	// Register registers a service with the container
	Register(name string, factory ServiceFactory) error

	// RegisterInstance registers a service instance with the container
	RegisterInstance(name string, instance interface{}) error

	// Resolve resolves a service by name
	Resolve(name string) (interface{}, error)

	// ResolveAs resolves a service and casts it to the specified type
	ResolveAs(name string, target interface{}) error

	// Has checks if a service is registered
	Has(name string) bool

	// Names returns all registered service names
	Names() []string
}

// ServiceFactory is a function that creates a service instance
type ServiceFactory func(container Container) (interface{}, error)

// LifecycleManager manages the lifecycle of services
type LifecycleManager interface {
	// Register registers a service with optional dependencies
	Register(name string, service Service, deps ...string) error

	// Start starts all services in dependency order
	Start(ctx context.Context) error

	// Stop stops all services in reverse dependency order
	Stop(ctx context.Context) error

	// Health returns the health status of all services
	Health(ctx context.Context) (map[string]HealthStatus, error)

	// Services returns all registered service names
	Services() []string

	// Events returns a channel for lifecycle events
	Events() <-chan LifecycleEvent

	// AddListener adds a lifecycle event listener
	AddListener(listener func(LifecycleEvent))
}

// Application represents the main SNGO application
type Application interface {
	// Configure configures the application with a configuration
	Configure(config interface{}) error

	// Run runs the application
	Run(ctx context.Context) error

	// Shutdown shuts down the application gracefully
	Shutdown(ctx context.Context) error

	// Container returns the dependency injection container
	Container() Container

	// LifecycleManager returns the lifecycle manager
	LifecycleManager() LifecycleManager
}

// LifecycleEvent represents an event in the service lifecycle
type LifecycleEvent struct {
	Type      string                 `json:"type"`
	Service   string                 `json:"service,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Error     error                  `json:"error,omitempty"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

// ApplicationError represents an error that occurred during application lifecycle
type ApplicationError struct {
	Operation string
	Service   string
	Err       error
}

func (e *ApplicationError) Error() string {
	if e.Service != "" {
		return fmt.Sprintf("%s failed for service %s: %v", e.Operation, e.Service, e.Err)
	}
	return fmt.Sprintf("%s failed: %v", e.Operation, e.Err)
}

func (e *ApplicationError) Unwrap() error {
	return e.Err
}
