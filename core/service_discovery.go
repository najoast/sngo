package core

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// LoadBalancer selects the best service instance from multiple available instances.
type LoadBalancer interface {
	// Select chooses the best service instance based on the load balancing strategy
	Select(services []*ServiceInfo) (*ServiceInfo, error)

	// UpdateMetrics updates the metrics for a service instance
	UpdateMetrics(serviceID string, metrics ServiceMetrics) error

	// GetStrategy returns the current load balancing strategy
	GetStrategy() LoadBalanceStrategy
}

// LoadBalanceStrategy defines different load balancing algorithms.
type LoadBalanceStrategy uint8

const (
	// StrategyRoundRobin cycles through services in order
	StrategyRoundRobin LoadBalanceStrategy = iota

	// StrategyRandom selects services randomly
	StrategyRandom

	// StrategyLeastConnections selects the service with fewest active connections
	StrategyLeastConnections

	// StrategyWeightedRoundRobin uses service weights for selection
	StrategyWeightedRoundRobin

	// StrategyConsistentHash uses consistent hashing for selection
	StrategyConsistentHash
)

// String returns the string representation of LoadBalanceStrategy.
func (s LoadBalanceStrategy) String() string {
	switch s {
	case StrategyRoundRobin:
		return "round_robin"
	case StrategyRandom:
		return "random"
	case StrategyLeastConnections:
		return "least_connections"
	case StrategyWeightedRoundRobin:
		return "weighted_round_robin"
	case StrategyConsistentHash:
		return "consistent_hash"
	default:
		return "unknown"
	}
}

// ServiceMetrics contains performance metrics for a service instance.
type ServiceMetrics struct {
	// ActiveConnections is the number of active connections
	ActiveConnections int64

	// TotalRequests is the total number of requests served
	TotalRequests int64

	// FailedRequests is the number of failed requests
	FailedRequests int64

	// AverageResponseTime is the average response time in milliseconds
	AverageResponseTime time.Duration

	// CPU usage percentage (0-100)
	CPUUsage float64

	// Memory usage percentage (0-100)
	MemoryUsage float64

	// Custom metrics
	CustomMetrics map[string]float64

	// Last update time
	LastUpdated time.Time
}

// SuccessRate calculates the success rate of the service.
func (m ServiceMetrics) SuccessRate() float64 {
	if m.TotalRequests == 0 {
		return 1.0
	}
	successful := m.TotalRequests - m.FailedRequests
	return float64(successful) / float64(m.TotalRequests)
}

// loadBalancer implements the LoadBalancer interface.
type loadBalancer struct {
	strategy LoadBalanceStrategy
	mu       sync.RWMutex

	// Round robin state
	roundRobinIndex int

	// Service metrics
	metrics map[string]*ServiceMetrics // key: service name

	// Weighted round robin state
	weightedServices []weightedService
	weightedIndex    int

	// Random generator
	rand *rand.Rand
}

// weightedService holds service info with its weight.
type weightedService struct {
	Service *ServiceInfo
	Weight  int
}

// NewLoadBalancer creates a new LoadBalancer with the specified strategy.
func NewLoadBalancer(strategy LoadBalanceStrategy) LoadBalancer {
	return &loadBalancer{
		strategy: strategy,
		metrics:  make(map[string]*ServiceMetrics),
		rand:     rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Select chooses the best service instance based on the load balancing strategy.
func (lb *loadBalancer) Select(services []*ServiceInfo) (*ServiceInfo, error) {
	if len(services) == 0 {
		return nil, errors.New("no services available")
	}

	// Filter out unhealthy services
	healthyServices := lb.filterHealthyServices(services)
	if len(healthyServices) == 0 {
		return nil, errors.New("no healthy services available")
	}

	lb.mu.Lock()
	defer lb.mu.Unlock()

	switch lb.strategy {
	case StrategyRoundRobin:
		return lb.selectRoundRobin(healthyServices), nil

	case StrategyRandom:
		return lb.selectRandom(healthyServices), nil

	case StrategyLeastConnections:
		return lb.selectLeastConnections(healthyServices), nil

	case StrategyWeightedRoundRobin:
		return lb.selectWeightedRoundRobin(healthyServices), nil

	case StrategyConsistentHash:
		// TODO: Implement consistent hashing
		return lb.selectRandom(healthyServices), nil

	default:
		return healthyServices[0], nil
	}
}

// UpdateMetrics updates the metrics for a service instance.
func (lb *loadBalancer) UpdateMetrics(serviceID string, metrics ServiceMetrics) error {
	lb.mu.Lock()
	defer lb.mu.Unlock()

	metrics.LastUpdated = time.Now()
	lb.metrics[serviceID] = &metrics
	return nil
}

// GetStrategy returns the current load balancing strategy.
func (lb *loadBalancer) GetStrategy() LoadBalanceStrategy {
	return lb.strategy
}

// filterHealthyServices returns only healthy services.
func (lb *loadBalancer) filterHealthyServices(services []*ServiceInfo) []*ServiceInfo {
	var healthy []*ServiceInfo
	for _, service := range services {
		if service.Status == ServiceStatusHealthy {
			healthy = append(healthy, service)
		}
	}
	return healthy
}

// selectRoundRobin implements round robin selection.
func (lb *loadBalancer) selectRoundRobin(services []*ServiceInfo) *ServiceInfo {
	if len(services) == 0 {
		return nil
	}

	service := services[lb.roundRobinIndex%len(services)]
	lb.roundRobinIndex++
	return service
}

// selectRandom implements random selection.
func (lb *loadBalancer) selectRandom(services []*ServiceInfo) *ServiceInfo {
	if len(services) == 0 {
		return nil
	}

	index := lb.rand.Intn(len(services))
	return services[index]
}

// selectLeastConnections implements least connections selection.
func (lb *loadBalancer) selectLeastConnections(services []*ServiceInfo) *ServiceInfo {
	if len(services) == 0 {
		return nil
	}

	bestService := services[0]
	bestConnections := int64(^uint64(0) >> 1) // Max int64

	for _, service := range services {
		metrics := lb.metrics[service.Handle.Name]
		connections := int64(0)
		if metrics != nil {
			connections = metrics.ActiveConnections
		}

		if connections < bestConnections {
			bestConnections = connections
			bestService = service
		}
	}

	return bestService
}

// selectWeightedRoundRobin implements weighted round robin selection.
func (lb *loadBalancer) selectWeightedRoundRobin(services []*ServiceInfo) *ServiceInfo {
	if len(services) == 0 {
		return nil
	}

	// Build weighted services list if needed
	if len(lb.weightedServices) != len(services) {
		lb.buildWeightedServices(services)
	}

	if len(lb.weightedServices) == 0 {
		return services[0]
	}

	service := lb.weightedServices[lb.weightedIndex%len(lb.weightedServices)]
	lb.weightedIndex++
	return service.Service
}

// buildWeightedServices builds the weighted services list.
func (lb *loadBalancer) buildWeightedServices(services []*ServiceInfo) {
	lb.weightedServices = nil

	for _, service := range services {
		weight := lb.getServiceWeight(service)
		for i := 0; i < weight; i++ {
			lb.weightedServices = append(lb.weightedServices, weightedService{
				Service: service,
				Weight:  weight,
			})
		}
	}
}

// getServiceWeight calculates the weight of a service based on its metrics.
func (lb *loadBalancer) getServiceWeight(service *ServiceInfo) int {
	metrics := lb.metrics[service.Handle.Name]
	if metrics == nil {
		return 1 // Default weight
	}

	// Calculate weight based on success rate and response time
	successRate := metrics.SuccessRate()
	responseTimeFactor := 1.0
	if metrics.AverageResponseTime > 0 {
		// Lower response time = higher weight
		responseTimeFactor = 1000.0 / float64(metrics.AverageResponseTime.Milliseconds())
	}

	weight := int(successRate * responseTimeFactor * 10)
	if weight < 1 {
		weight = 1
	}
	if weight > 100 {
		weight = 100
	}

	return weight
}

// ServiceDiscovery combines service registry and load balancing for complete service discovery.
type ServiceDiscovery interface {
	// RegisterService registers a service with optional metadata
	RegisterService(handle *Handle, info ServiceRegistrationInfo) error

	// UnregisterService unregisters a service
	UnregisterService(name string) error

	// DiscoverService finds and selects the best service instance
	DiscoverService(name string) (*ServiceInfo, error)

	// DiscoverServices finds all matching services
	DiscoverServices(query ServiceQuery) ([]*ServiceInfo, error)

	// UpdateServiceHealth updates the health status of a service
	UpdateServiceHealth(name string, status ServiceStatus) error

	// UpdateServiceMetrics updates the performance metrics of a service
	UpdateServiceMetrics(name string, metrics ServiceMetrics) error

	// SetLoadBalanceStrategy sets the load balancing strategy
	SetLoadBalanceStrategy(strategy LoadBalanceStrategy) error
}

// ServiceRegistrationInfo contains information for registering a service.
type ServiceRegistrationInfo struct {
	Description         string
	Version             string
	Tags                []string
	Metadata            map[string]string
	HealthCheckInterval time.Duration
}

// serviceDiscovery implements the ServiceDiscovery interface.
type serviceDiscovery struct {
	registry     ServiceRegistry
	loadBalancer LoadBalancer
}

// NewServiceDiscovery creates a new ServiceDiscovery instance.
func NewServiceDiscovery() ServiceDiscovery {
	return &serviceDiscovery{
		registry:     NewServiceRegistry(),
		loadBalancer: NewLoadBalancer(StrategyRoundRobin),
	}
}

// RegisterService registers a service with optional metadata.
func (sd *serviceDiscovery) RegisterService(handle *Handle, info ServiceRegistrationInfo) error {
	serviceInfo := &ServiceInfo{
		Handle:              handle,
		Description:         info.Description,
		Version:             info.Version,
		Tags:                info.Tags,
		Status:              ServiceStatusHealthy,
		Metadata:            info.Metadata,
		RegisteredAt:        time.Now(),
		HealthCheckInterval: info.HealthCheckInterval,
	}

	if serviceInfo.HealthCheckInterval == 0 {
		serviceInfo.HealthCheckInterval = 30 * time.Second
	}

	return sd.registry.Register(serviceInfo)
}

// UnregisterService unregisters a service.
func (sd *serviceDiscovery) UnregisterService(name string) error {
	return sd.registry.Unregister(name)
}

// DiscoverService finds and selects the best service instance.
func (sd *serviceDiscovery) DiscoverService(name string) (*ServiceInfo, error) {
	// Find all instances of the service
	services, err := sd.registry.Discover(ServiceQuery{Name: name})
	if err != nil {
		return nil, err
	}

	if len(services) == 0 {
		return nil, fmt.Errorf("service '%s' not found", name)
	}

	// Use load balancer to select the best instance
	return sd.loadBalancer.Select(services)
}

// DiscoverServices finds all matching services.
func (sd *serviceDiscovery) DiscoverServices(query ServiceQuery) ([]*ServiceInfo, error) {
	return sd.registry.Discover(query)
}

// UpdateServiceHealth updates the health status of a service.
func (sd *serviceDiscovery) UpdateServiceHealth(name string, status ServiceStatus) error {
	return sd.registry.UpdateStatus(name, status)
}

// UpdateServiceMetrics updates the performance metrics of a service.
func (sd *serviceDiscovery) UpdateServiceMetrics(name string, metrics ServiceMetrics) error {
	return sd.loadBalancer.UpdateMetrics(name, metrics)
}

// SetLoadBalanceStrategy sets the load balancing strategy.
func (sd *serviceDiscovery) SetLoadBalanceStrategy(strategy LoadBalanceStrategy) error {
	// Create new load balancer with the specified strategy
	sd.loadBalancer = NewLoadBalancer(strategy)
	return nil
}
