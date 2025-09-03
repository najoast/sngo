package core

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ServiceInfo contains detailed information about a registered service.
type ServiceInfo struct {
	// Handle is the service handle
	Handle *Handle

	// Description is a human-readable description
	Description string

	// Version is the service version
	Version string

	// Tags are service labels for categorization
	Tags []string

	// Health status
	Status ServiceStatus

	// Metadata contains additional service information
	Metadata map[string]string

	// Registration time
	RegisteredAt time.Time

	// Last health check time
	LastHealthCheck time.Time

	// Health check interval
	HealthCheckInterval time.Duration
}

// ServiceStatus represents the health status of a service.
type ServiceStatus uint8

const (
	// ServiceStatusUnknown means the status is not yet determined
	ServiceStatusUnknown ServiceStatus = iota

	// ServiceStatusHealthy means the service is healthy and available
	ServiceStatusHealthy

	// ServiceStatusUnhealthy means the service is unhealthy but still registered
	ServiceStatusUnhealthy

	// ServiceStatusMaintenance means the service is under maintenance
	ServiceStatusMaintenance

	// ServiceStatusDraining means the service is being drained
	ServiceStatusDraining
)

// String returns the string representation of ServiceStatus.
func (s ServiceStatus) String() string {
	switch s {
	case ServiceStatusUnknown:
		return "unknown"
	case ServiceStatusHealthy:
		return "healthy"
	case ServiceStatusUnhealthy:
		return "unhealthy"
	case ServiceStatusMaintenance:
		return "maintenance"
	case ServiceStatusDraining:
		return "draining"
	default:
		return "invalid"
	}
}

// ServiceQuery represents criteria for service discovery.
type ServiceQuery struct {
	// Name is the exact service name to match
	Name string

	// Pattern is a glob pattern to match service names
	Pattern string

	// Tags are required tags that services must have
	Tags []string

	// Status filters services by health status
	Status []ServiceStatus

	// Metadata filters services by metadata key-value pairs
	Metadata map[string]string

	// Node filters services by node ID
	Node uint32

	// Limit limits the number of results
	Limit int
}

// ServiceRegistry manages service registration and discovery.
type ServiceRegistry interface {
	// Register registers a service with the registry
	Register(info *ServiceInfo) error

	// Unregister removes a service from the registry
	Unregister(name string) error

	// Discover finds services matching the query criteria
	Discover(query ServiceQuery) ([]*ServiceInfo, error)

	// Get retrieves a specific service by name
	Get(name string) (*ServiceInfo, error)

	// List returns all registered services
	List() ([]*ServiceInfo, error)

	// UpdateStatus updates the health status of a service
	UpdateStatus(name string, status ServiceStatus) error

	// UpdateMetadata updates the metadata of a service
	UpdateMetadata(name string, metadata map[string]string) error

	// Watch starts watching for service changes
	Watch(ctx context.Context) (<-chan ServiceEvent, error)
}

// ServiceEvent represents a change in service registry.
type ServiceEvent struct {
	// Type of the event
	Type ServiceEventType

	// Service information
	Service *ServiceInfo

	// Timestamp when the event occurred
	Timestamp time.Time
}

// ServiceEventType represents different types of service events.
type ServiceEventType uint8

const (
	// ServiceEventRegister indicates a service was registered
	ServiceEventRegister ServiceEventType = iota

	// ServiceEventUnregister indicates a service was unregistered
	ServiceEventUnregister

	// ServiceEventStatusChange indicates a service status changed
	ServiceEventStatusChange

	// ServiceEventMetadataChange indicates service metadata changed
	ServiceEventMetadataChange
)

// String returns the string representation of ServiceEventType.
func (t ServiceEventType) String() string {
	switch t {
	case ServiceEventRegister:
		return "register"
	case ServiceEventUnregister:
		return "unregister"
	case ServiceEventStatusChange:
		return "status_change"
	case ServiceEventMetadataChange:
		return "metadata_change"
	default:
		return "unknown"
	}
}

// localServiceRegistry implements ServiceRegistry for local services.
type localServiceRegistry struct {
	mu       sync.RWMutex
	services map[string]*ServiceInfo

	// Watchers for service events
	watchers     map[uint64]chan ServiceEvent
	watcherID    uint64
	watcherMutex sync.RWMutex
}

// NewServiceRegistry creates a new local service registry.
func NewServiceRegistry() ServiceRegistry {
	registry := &localServiceRegistry{
		services: make(map[string]*ServiceInfo),
		watchers: make(map[uint64]chan ServiceEvent),
	}

	// Start health check routine
	go registry.healthCheckRoutine()

	return registry
}

// Register registers a service with the registry.
func (r *localServiceRegistry) Register(info *ServiceInfo) error {
	if info == nil || info.Handle == nil {
		return fmt.Errorf("invalid service info")
	}

	if info.Handle.Name == "" {
		return fmt.Errorf("service name cannot be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if service already exists
	if _, exists := r.services[info.Handle.Name]; exists {
		return fmt.Errorf("service '%s' already registered", info.Handle.Name)
	}

	// Set default values
	if info.RegisteredAt.IsZero() {
		info.RegisteredAt = time.Now()
	}
	if info.Status == ServiceStatusUnknown {
		info.Status = ServiceStatusHealthy
	}
	if info.HealthCheckInterval == 0 {
		info.HealthCheckInterval = 30 * time.Second
	}
	if info.Metadata == nil {
		info.Metadata = make(map[string]string)
	}

	// Register the service
	r.services[info.Handle.Name] = info

	// Notify watchers
	r.notifyWatchers(ServiceEvent{
		Type:      ServiceEventRegister,
		Service:   info,
		Timestamp: time.Now(),
	})

	return nil
}

// Unregister removes a service from the registry.
func (r *localServiceRegistry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	service, exists := r.services[name]
	if !exists {
		return fmt.Errorf("service '%s' not found", name)
	}

	delete(r.services, name)

	// Notify watchers
	r.notifyWatchers(ServiceEvent{
		Type:      ServiceEventUnregister,
		Service:   service,
		Timestamp: time.Now(),
	})

	return nil
}

// Discover finds services matching the query criteria.
func (r *localServiceRegistry) Discover(query ServiceQuery) ([]*ServiceInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []*ServiceInfo

	for _, service := range r.services {
		if r.matchesQuery(service, query) {
			results = append(results, service)
		}

		// Apply limit
		if query.Limit > 0 && len(results) >= query.Limit {
			break
		}
	}

	return results, nil
}

// Get retrieves a specific service by name.
func (r *localServiceRegistry) Get(name string) (*ServiceInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	service, exists := r.services[name]
	if !exists {
		return nil, fmt.Errorf("service '%s' not found", name)
	}

	return service, nil
}

// List returns all registered services.
func (r *localServiceRegistry) List() ([]*ServiceInfo, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	services := make([]*ServiceInfo, 0, len(r.services))
	for _, service := range r.services {
		services = append(services, service)
	}

	return services, nil
}

// UpdateStatus updates the health status of a service.
func (r *localServiceRegistry) UpdateStatus(name string, status ServiceStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	service, exists := r.services[name]
	if !exists {
		return fmt.Errorf("service '%s' not found", name)
	}

	oldStatus := service.Status
	service.Status = status
	service.LastHealthCheck = time.Now()

	// Notify watchers if status changed
	if oldStatus != status {
		r.notifyWatchers(ServiceEvent{
			Type:      ServiceEventStatusChange,
			Service:   service,
			Timestamp: time.Now(),
		})
	}

	return nil
}

// UpdateMetadata updates the metadata of a service.
func (r *localServiceRegistry) UpdateMetadata(name string, metadata map[string]string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	service, exists := r.services[name]
	if !exists {
		return fmt.Errorf("service '%s' not found", name)
	}

	// Update metadata
	if service.Metadata == nil {
		service.Metadata = make(map[string]string)
	}
	for k, v := range metadata {
		service.Metadata[k] = v
	}

	// Notify watchers
	r.notifyWatchers(ServiceEvent{
		Type:      ServiceEventMetadataChange,
		Service:   service,
		Timestamp: time.Now(),
	})

	return nil
}

// Watch starts watching for service changes.
func (r *localServiceRegistry) Watch(ctx context.Context) (<-chan ServiceEvent, error) {
	r.watcherMutex.Lock()
	defer r.watcherMutex.Unlock()

	r.watcherID++
	watcherID := r.watcherID

	eventChan := make(chan ServiceEvent, 100)
	r.watchers[watcherID] = eventChan

	// Start a goroutine to clean up when context is done
	go func() {
		<-ctx.Done()
		r.watcherMutex.Lock()
		delete(r.watchers, watcherID)
		close(eventChan)
		r.watcherMutex.Unlock()
	}()

	return eventChan, nil
}

// matchesQuery checks if a service matches the query criteria.
func (r *localServiceRegistry) matchesQuery(service *ServiceInfo, query ServiceQuery) bool {
	// Check name exact match
	if query.Name != "" && service.Handle.Name != query.Name {
		return false
	}

	// Check pattern match (simplified glob matching)
	if query.Pattern != "" {
		// TODO: Implement proper glob matching
		if service.Handle.Name != query.Pattern {
			return false
		}
	}

	// Check status
	if len(query.Status) > 0 {
		found := false
		for _, status := range query.Status {
			if service.Status == status {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check tags
	if len(query.Tags) > 0 {
		for _, requiredTag := range query.Tags {
			found := false
			for _, serviceTag := range service.Tags {
				if serviceTag == requiredTag {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}

	// Check metadata
	if len(query.Metadata) > 0 {
		for key, value := range query.Metadata {
			if service.Metadata[key] != value {
				return false
			}
		}
	}

	// Check node
	if query.Node != 0 && service.Handle.Node != query.Node {
		return false
	}

	return true
}

// notifyWatchers sends an event to all registered watchers.
func (r *localServiceRegistry) notifyWatchers(event ServiceEvent) {
	r.watcherMutex.RLock()
	defer r.watcherMutex.RUnlock()

	for _, watcher := range r.watchers {
		select {
		case watcher <- event:
		default:
			// Channel is full, skip this watcher
		}
	}
}

// healthCheckRoutine periodically checks the health of registered services.
func (r *localServiceRegistry) healthCheckRoutine() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		r.performHealthChecks()
	}
}

// performHealthChecks checks the health of all registered services.
func (r *localServiceRegistry) performHealthChecks() {
	r.mu.RLock()
	services := make([]*ServiceInfo, 0, len(r.services))
	for _, service := range r.services {
		services = append(services, service)
	}
	r.mu.RUnlock()

	for _, service := range services {
		// Check if health check is needed
		if time.Since(service.LastHealthCheck) > service.HealthCheckInterval {
			// TODO: Implement actual health check logic
			// For now, just update the timestamp
			r.UpdateStatus(service.Handle.Name, service.Status)
		}
	}
}
