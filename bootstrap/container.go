// Package bootstrap provides dependency injection container implementation
package bootstrap

import (
	"fmt"
	"reflect"
	"sync"
)

// DefaultContainer provides a simple dependency injection container
type DefaultContainer struct {
	// services holds registered service factories
	services map[string]ServiceFactory

	// instances holds created service instances
	instances map[string]interface{}

	// mutex protects concurrent access
	mutex sync.RWMutex
}

// NewContainer creates a new dependency injection container
func NewContainer() Container {
	return &DefaultContainer{
		services:  make(map[string]ServiceFactory),
		instances: make(map[string]interface{}),
	}
}

// Register registers a service factory with the container
func (c *DefaultContainer) Register(name string, factory ServiceFactory) error {
	if name == "" {
		return fmt.Errorf("service name cannot be empty")
	}
	if factory == nil {
		return fmt.Errorf("service factory cannot be nil")
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	if _, exists := c.services[name]; exists {
		return fmt.Errorf("service %s is already registered", name)
	}

	c.services[name] = factory
	return nil
}

// RegisterInstance registers a service instance with the container
func (c *DefaultContainer) RegisterInstance(name string, instance interface{}) error {
	if name == "" {
		return fmt.Errorf("service name cannot be empty")
	}
	if instance == nil {
		return fmt.Errorf("service instance cannot be nil")
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	if _, exists := c.instances[name]; exists {
		return fmt.Errorf("service instance %s is already registered", name)
	}

	c.instances[name] = instance
	return nil
}

// Resolve resolves a service by name
func (c *DefaultContainer) Resolve(name string) (interface{}, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Check if we already have an instance
	if instance, exists := c.instances[name]; exists {
		return instance, nil
	}

	// Check if we have a factory
	factory, exists := c.services[name]
	if !exists {
		return nil, fmt.Errorf("service %s is not registered", name)
	}

	// Create the instance
	instance, err := factory(c)
	if err != nil {
		return nil, fmt.Errorf("failed to create service %s: %w", name, err)
	}

	// Cache the instance
	c.instances[name] = instance
	return instance, nil
}

// ResolveAs resolves a service and casts it to the specified type
func (c *DefaultContainer) ResolveAs(name string, target interface{}) error {
	instance, err := c.Resolve(name)
	if err != nil {
		return err
	}

	// Use reflection to set the target
	targetValue := reflect.ValueOf(target)
	if targetValue.Kind() != reflect.Ptr {
		return fmt.Errorf("target must be a pointer")
	}

	instanceValue := reflect.ValueOf(instance)
	targetType := targetValue.Elem().Type()

	if !instanceValue.Type().AssignableTo(targetType) {
		return fmt.Errorf("service %s of type %s is not assignable to %s",
			name, instanceValue.Type(), targetType)
	}

	targetValue.Elem().Set(instanceValue)
	return nil
}

// Has checks if a service is registered
func (c *DefaultContainer) Has(name string) bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	_, hasFactory := c.services[name]
	_, hasInstance := c.instances[name]
	return hasFactory || hasInstance
}

// Names returns all registered service names
func (c *DefaultContainer) Names() []string {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	nameSet := make(map[string]bool)

	// Add factory names
	for name := range c.services {
		nameSet[name] = true
	}

	// Add instance names
	for name := range c.instances {
		nameSet[name] = true
	}

	// Convert to slice
	names := make([]string, 0, len(nameSet))
	for name := range nameSet {
		names = append(names, name)
	}

	return names
}

// Clear removes all services and instances from the container
func (c *DefaultContainer) Clear() {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.services = make(map[string]ServiceFactory)
	c.instances = make(map[string]interface{})
}

// GetInstance returns a cached instance if it exists
func (c *DefaultContainer) GetInstance(name string) (interface{}, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	instance, exists := c.instances[name]
	return instance, exists
}

// RemoveInstance removes a cached instance
func (c *DefaultContainer) RemoveInstance(name string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	delete(c.instances, name)
}

// ContainerBuilder helps build and configure containers
type ContainerBuilder struct {
	container *DefaultContainer
}

// NewContainerBuilder creates a new container builder
func NewContainerBuilder() *ContainerBuilder {
	return &ContainerBuilder{
		container: &DefaultContainer{
			services:  make(map[string]ServiceFactory),
			instances: make(map[string]interface{}),
		},
	}
}

// WithService registers a service factory
func (b *ContainerBuilder) WithService(name string, factory ServiceFactory) *ContainerBuilder {
	b.container.Register(name, factory)
	return b
}

// WithInstance registers a service instance
func (b *ContainerBuilder) WithInstance(name string, instance interface{}) *ContainerBuilder {
	b.container.RegisterInstance(name, instance)
	return b
}

// Build returns the configured container
func (b *ContainerBuilder) Build() Container {
	return b.container
}

// ServiceScope represents the scope of a service
type ServiceScope int

const (
	// ScopeSingleton indicates the service should be created once and reused
	ScopeSingleton ServiceScope = iota

	// ScopeTransient indicates a new instance should be created each time
	ScopeTransient

	// ScopeScoped indicates the service should be scoped to a specific context
	ScopeScoped
)

// ScopedContainer extends the basic container with scoping capabilities
type ScopedContainer struct {
	*DefaultContainer
	scopes map[string]ServiceScope
}

// NewScopedContainer creates a new scoped container
func NewScopedContainer() *ScopedContainer {
	return &ScopedContainer{
		DefaultContainer: &DefaultContainer{
			services:  make(map[string]ServiceFactory),
			instances: make(map[string]interface{}),
		},
		scopes: make(map[string]ServiceScope),
	}
}

// RegisterScoped registers a service with a specific scope
func (c *ScopedContainer) RegisterScoped(name string, factory ServiceFactory, scope ServiceScope) error {
	err := c.Register(name, factory)
	if err != nil {
		return err
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.scopes[name] = scope
	return nil
}

// Resolve resolves a service respecting its scope
func (c *ScopedContainer) Resolve(name string) (interface{}, error) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	scope, hasScope := c.scopes[name]
	if !hasScope {
		// Default behavior for non-scoped services
		return c.DefaultContainer.Resolve(name)
	}

	switch scope {
	case ScopeSingleton:
		// Check if we already have an instance
		if instance, exists := c.instances[name]; exists {
			return instance, nil
		}

		// Create and cache the instance
		factory, exists := c.services[name]
		if !exists {
			return nil, fmt.Errorf("service %s is not registered", name)
		}

		instance, err := factory(c)
		if err != nil {
			return nil, fmt.Errorf("failed to create service %s: %w", name, err)
		}

		c.instances[name] = instance
		return instance, nil

	case ScopeTransient:
		// Always create a new instance
		factory, exists := c.services[name]
		if !exists {
			return nil, fmt.Errorf("service %s is not registered", name)
		}

		return factory(c)

	case ScopeScoped:
		// For now, treat scoped the same as singleton
		// In a real implementation, this would check the current scope context
		return c.Resolve(name)

	default:
		return nil, fmt.Errorf("unknown service scope: %v", scope)
	}
}
