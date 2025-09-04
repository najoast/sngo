// Package bootstrap provides service lifecycle management
package bootstrap

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// DefaultLifecycleManager implements the LifecycleManager interface
type DefaultLifecycleManager struct {
	// services holds all registered services
	services map[string]Service

	// dependencies tracks service dependencies
	dependencies map[string][]string

	// startOrder tracks the order services were started
	startOrder []string

	// container for dependency injection
	container Container

	// mutex protects concurrent access
	mutex sync.RWMutex

	// started indicates if the lifecycle manager has been started
	started bool

	// stopping indicates if the lifecycle manager is shutting down
	stopping bool

	// eventChan for broadcasting lifecycle events
	eventChan chan LifecycleEvent

	// listeners for lifecycle events
	listeners []func(LifecycleEvent)

	// timeout for service operations
	timeout time.Duration
}

// NewLifecycleManager creates a new lifecycle manager
func NewLifecycleManager(container Container) LifecycleManager {
	return &DefaultLifecycleManager{
		services:     make(map[string]Service),
		dependencies: make(map[string][]string),
		container:    container,
		eventChan:    make(chan LifecycleEvent, 100),
		timeout:      30 * time.Second,
	}
}

// Register registers a service with the lifecycle manager
func (lm *DefaultLifecycleManager) Register(name string, service Service, deps ...string) error {
	if name == "" {
		return fmt.Errorf("service name cannot be empty")
	}
	if service == nil {
		return fmt.Errorf("service cannot be nil")
	}

	lm.mutex.Lock()
	defer lm.mutex.Unlock()

	if lm.started {
		return fmt.Errorf("cannot register service %s: lifecycle manager already started", name)
	}

	if _, exists := lm.services[name]; exists {
		return fmt.Errorf("service %s is already registered", name)
	}

	lm.services[name] = service
	lm.dependencies[name] = deps

	lm.broadcastEvent(LifecycleEvent{
		Type:      "service.registered",
		Service:   name,
		Timestamp: time.Now(),
		Data:      map[string]interface{}{"dependencies": deps},
	})

	return nil
}

// Start starts all services in dependency order
func (lm *DefaultLifecycleManager) Start(ctx context.Context) error {
	lm.mutex.Lock()
	defer lm.mutex.Unlock()

	if lm.started {
		return fmt.Errorf("lifecycle manager already started")
	}

	// Calculate startup order
	startOrder, err := lm.calculateStartOrder()
	if err != nil {
		return fmt.Errorf("failed to calculate start order: %w", err)
	}

	lm.broadcastEvent(LifecycleEvent{
		Type:      "lifecycle.starting",
		Timestamp: time.Now(),
		Data:      map[string]interface{}{"order": startOrder},
	})

	// Start services in order
	for _, serviceName := range startOrder {
		service := lm.services[serviceName]

		lm.broadcastEvent(LifecycleEvent{
			Type:      "service.starting",
			Service:   serviceName,
			Timestamp: time.Now(),
		})

		// Create context with timeout
		startCtx, cancel := context.WithTimeout(ctx, lm.timeout)

		err := service.Start(startCtx)
		cancel()

		if err != nil {
			lm.broadcastEvent(LifecycleEvent{
				Type:      "service.start_failed",
				Service:   serviceName,
				Timestamp: time.Now(),
				Error:     err,
			})
			return fmt.Errorf("failed to start service %s: %w", serviceName, err)
		}

		lm.startOrder = append(lm.startOrder, serviceName)

		lm.broadcastEvent(LifecycleEvent{
			Type:      "service.started",
			Service:   serviceName,
			Timestamp: time.Now(),
		})
	}

	lm.started = true

	lm.broadcastEvent(LifecycleEvent{
		Type:      "lifecycle.started",
		Timestamp: time.Now(),
	})

	return nil
}

// Stop stops all services in reverse dependency order
func (lm *DefaultLifecycleManager) Stop(ctx context.Context) error {
	lm.mutex.Lock()
	defer lm.mutex.Unlock()

	if !lm.started {
		return nil // Already stopped
	}

	if lm.stopping {
		return fmt.Errorf("lifecycle manager already stopping")
	}

	lm.stopping = true

	lm.broadcastEvent(LifecycleEvent{
		Type:      "lifecycle.stopping",
		Timestamp: time.Now(),
	})

	// Stop services in reverse order
	stopOrder := make([]string, len(lm.startOrder))
	copy(stopOrder, lm.startOrder)

	// Reverse the slice
	for i := len(stopOrder)/2 - 1; i >= 0; i-- {
		opp := len(stopOrder) - 1 - i
		stopOrder[i], stopOrder[opp] = stopOrder[opp], stopOrder[i]
	}

	var lastError error

	for _, serviceName := range stopOrder {
		service := lm.services[serviceName]

		lm.broadcastEvent(LifecycleEvent{
			Type:      "service.stopping",
			Service:   serviceName,
			Timestamp: time.Now(),
		})

		// Create context with timeout
		stopCtx, cancel := context.WithTimeout(ctx, lm.timeout)

		err := service.Stop(stopCtx)
		cancel()

		if err != nil {
			lastError = err
			lm.broadcastEvent(LifecycleEvent{
				Type:      "service.stop_failed",
				Service:   serviceName,
				Timestamp: time.Now(),
				Error:     err,
			})
		} else {
			lm.broadcastEvent(LifecycleEvent{
				Type:      "service.stopped",
				Service:   serviceName,
				Timestamp: time.Now(),
			})
		}
	}

	lm.started = false
	lm.stopping = false
	lm.startOrder = nil

	lm.broadcastEvent(LifecycleEvent{
		Type:      "lifecycle.stopped",
		Timestamp: time.Now(),
	})

	return lastError
}

// Health returns the health status of all services
func (lm *DefaultLifecycleManager) Health(ctx context.Context) (map[string]HealthStatus, error) {
	lm.mutex.RLock()
	defer lm.mutex.RUnlock()

	health := make(map[string]HealthStatus)

	for name, service := range lm.services {
		// Create context with timeout
		healthCtx, cancel := context.WithTimeout(ctx, 5*time.Second)

		status, err := service.Health(healthCtx)
		cancel()

		if err != nil {
			health[name] = HealthStatus{
				State:   HealthUnhealthy,
				Message: err.Error(),
			}
		} else {
			health[name] = status
		}
	}

	return health, nil
}

// Services returns all registered service names
func (lm *DefaultLifecycleManager) Services() []string {
	lm.mutex.RLock()
	defer lm.mutex.RUnlock()

	names := make([]string, 0, len(lm.services))
	for name := range lm.services {
		names = append(names, name)
	}

	sort.Strings(names)
	return names
}

// Events returns a channel for lifecycle events
func (lm *DefaultLifecycleManager) Events() <-chan LifecycleEvent {
	return lm.eventChan
}

// AddListener adds a lifecycle event listener
func (lm *DefaultLifecycleManager) AddListener(listener func(LifecycleEvent)) {
	lm.mutex.Lock()
	defer lm.mutex.Unlock()

	lm.listeners = append(lm.listeners, listener)
}

// calculateStartOrder calculates the order to start services based on dependencies
func (lm *DefaultLifecycleManager) calculateStartOrder() ([]string, error) {
	// Topological sort using Kahn's algorithm
	inDegree := make(map[string]int)
	graph := make(map[string][]string)

	// Initialize in-degree for all services
	for service := range lm.services {
		inDegree[service] = 0
		graph[service] = []string{}
	}

	// Build the dependency graph
	for service, deps := range lm.dependencies {
		for _, dep := range deps {
			if _, exists := lm.services[dep]; !exists {
				return nil, fmt.Errorf("dependency %s of service %s is not registered", dep, service)
			}
			graph[dep] = append(graph[dep], service)
			inDegree[service]++
		}
	}

	// Find services with no dependencies
	queue := []string{}
	for service, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, service)
		}
	}

	result := []string{}

	for len(queue) > 0 {
		// Remove a service from queue
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)

		// Reduce in-degree for dependent services
		for _, dependent := range graph[current] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	// Check for circular dependencies
	if len(result) != len(lm.services) {
		return nil, fmt.Errorf("circular dependency detected")
	}

	return result, nil
}

// broadcastEvent broadcasts a lifecycle event to all listeners
func (lm *DefaultLifecycleManager) broadcastEvent(event LifecycleEvent) {
	// Send to channel (non-blocking)
	select {
	case lm.eventChan <- event:
	default:
		// Channel is full, skip this event
	}

	// Send to all listeners
	for _, listener := range lm.listeners {
		go func(l func(LifecycleEvent)) {
			defer func() {
				if r := recover(); r != nil {
					// Ignore panics in listeners
				}
			}()
			l(event)
		}(listener)
	}
}

// SetTimeout sets the timeout for service operations
func (lm *DefaultLifecycleManager) SetTimeout(timeout time.Duration) {
	lm.mutex.Lock()
	defer lm.mutex.Unlock()

	lm.timeout = timeout
}

// IsStarted returns true if the lifecycle manager has been started
func (lm *DefaultLifecycleManager) IsStarted() bool {
	lm.mutex.RLock()
	defer lm.mutex.RUnlock()

	return lm.started
}

// IsStopping returns true if the lifecycle manager is currently stopping
func (lm *DefaultLifecycleManager) IsStopping() bool {
	lm.mutex.RLock()
	defer lm.mutex.RUnlock()

	return lm.stopping
}

// GetService returns a registered service by name
func (lm *DefaultLifecycleManager) GetService(name string) (Service, bool) {
	lm.mutex.RLock()
	defer lm.mutex.RUnlock()

	service, exists := lm.services[name]
	return service, exists
}

// GetDependencies returns the dependencies for a service
func (lm *DefaultLifecycleManager) GetDependencies(name string) ([]string, bool) {
	lm.mutex.RLock()
	defer lm.mutex.RUnlock()

	deps, exists := lm.dependencies[name]
	if !exists {
		return nil, false
	}

	// Return a copy to prevent modification
	result := make([]string, len(deps))
	copy(result, deps)
	return result, true
}
