package cluster

import (
	"context"
	"fmt"
	"time"

	"github.com/najoast/sngo/bootstrap"
)

// ClusterService implements the bootstrap.Service interface
type ClusterService struct {
	manager ClusterManager
	config  *ClusterConfig
	started bool
}

// NewClusterService creates a new cluster service
func NewClusterService(cfg *ClusterConfig) *ClusterService {
	if cfg == nil {
		cfg = DefaultClusterConfig()
	}

	return &ClusterService{
		config: cfg,
	}
}

func (cs *ClusterService) Name() string {
	return "cluster"
}

func (cs *ClusterService) Start(ctx context.Context) error {
	if cs.started {
		return fmt.Errorf("cluster service already started")
	}

	// Create cluster manager
	cs.manager = NewClusterManager(cs.config)

	// Start cluster manager
	if err := cs.manager.Start(ctx); err != nil {
		return fmt.Errorf("failed to start cluster manager: %w", err)
	}

	// Join cluster if seed nodes are provided
	if len(cs.config.SeedNodes) > 0 {
		joinCtx, cancel := context.WithTimeout(ctx, cs.config.JoinTimeout)
		defer cancel()

		if err := cs.manager.Join(joinCtx, cs.config.SeedNodes); err != nil {
			return fmt.Errorf("failed to join cluster: %w", err)
		}
	}

	cs.started = true
	return nil
}

func (cs *ClusterService) Stop(ctx context.Context) error {
	if !cs.started || cs.manager == nil {
		return nil
	}

	// Gracefully leave cluster
	stopCtx, cancel := context.WithTimeout(ctx, cs.config.LeaveTimeout)
	defer cancel()

	err := cs.manager.Stop(stopCtx)
	cs.started = false
	return err
}

func (cs *ClusterService) Health(ctx context.Context) (bootstrap.HealthStatus, error) {
	if !cs.started || cs.manager == nil {
		return bootstrap.HealthStatus{
			State:   bootstrap.HealthStopped,
			Message: "cluster service not running",
		}, nil
	}

	health := cs.manager.GetClusterHealth()

	state := bootstrap.HealthHealthy
	message := fmt.Sprintf("cluster healthy: %d/%d nodes active", health.ActiveNodes, health.TotalNodes)

	if !health.IsHealthy {
		state = bootstrap.HealthUnhealthy
		message = fmt.Sprintf("cluster unhealthy: %d/%d nodes active", health.ActiveNodes, health.TotalNodes)
	}

	if !health.HasLeader {
		state = bootstrap.HealthCritical
		message = "no cluster leader"
	}

	return bootstrap.HealthStatus{
		State:     state,
		Message:   message,
		LastCheck: time.Now(),
		Data: map[string]interface{}{
			"total_nodes":     health.TotalNodes,
			"active_nodes":    health.ActiveNodes,
			"suspected_nodes": health.SuspectedNodes,
			"failed_nodes":    health.FailedNodes,
			"has_leader":      health.HasLeader,
			"leader_id":       health.LeaderID,
			"partition_count": health.PartitionCount,
		},
	}, nil
}

// GetManager returns the cluster manager instance
func (cs *ClusterService) GetManager() ClusterManager {
	return cs.manager
}

// GetRemoteService returns the remote service instance
func (cs *ClusterService) GetRemoteService() RemoteService {
	if cm, ok := cs.manager.(*clusterManager); ok {
		return cm.service
	}
	return nil
}

// GetServiceRegistry returns the service registry instance
func (cs *ClusterService) GetServiceRegistry() ServiceRegistry {
	if cm, ok := cs.manager.(*clusterManager); ok {
		return cm.registry
	}
	return nil
}

// CreateClusterServiceFactory creates a factory function for the cluster service
func CreateClusterServiceFactory(cfg *ClusterConfig) bootstrap.ServiceFactory {
	return func(container bootstrap.Container) (interface{}, error) {
		return NewClusterService(cfg), nil
	}
}

// ClusterApp provides a convenient way to create cluster-enabled applications
type ClusterApp struct {
	app     bootstrap.Application
	cluster *ClusterService
}

// NewClusterApp creates a new cluster-enabled application
func NewClusterApp(configPath string) (*ClusterApp, error) {
	// Create default application
	app := bootstrap.NewApplication()

	// Load configuration
	if configPath != "" {
		// TODO: Load cluster config from file
		// For now, use default config
	}

	// Create cluster service
	clusterService := NewClusterService(DefaultClusterConfig())

	// Get lifecycle manager and register cluster service
	lifecycleManager := app.LifecycleManager()
	if err := lifecycleManager.Register("cluster", clusterService); err != nil {
		return nil, fmt.Errorf("failed to register cluster service: %w", err)
	}

	// Register cluster service in container
	container := app.Container()
	if err := container.RegisterInstance("cluster_service", clusterService); err != nil {
		return nil, fmt.Errorf("failed to register cluster service in container: %w", err)
	}

	return &ClusterApp{
		app:     app,
		cluster: clusterService,
	}, nil
}

// Start starts the cluster application
func (ca *ClusterApp) Start(ctx context.Context) error {
	return ca.app.Run(ctx)
}

// Stop stops the cluster application
func (ca *ClusterApp) Stop(ctx context.Context) error {
	return ca.app.Shutdown(ctx)
}

// GetContainer returns the dependency injection container
func (ca *ClusterApp) GetContainer() bootstrap.Container {
	return ca.app.Container()
}

// GetClusterManager returns the cluster manager
func (ca *ClusterApp) GetClusterManager() ClusterManager {
	return ca.cluster.GetManager()
}

// GetRemoteService returns the remote service
func (ca *ClusterApp) GetRemoteService() RemoteService {
	return ca.cluster.GetRemoteService()
}

// GetServiceRegistry returns the service registry
func (ca *ClusterApp) GetServiceRegistry() ServiceRegistry {
	return ca.cluster.GetServiceRegistry()
}

// ExampleClusterApp shows how to create a cluster-enabled application
func ExampleClusterApp() error {
	// Create cluster application
	app, err := NewClusterApp("config.yaml")
	if err != nil {
		return fmt.Errorf("failed to create cluster app: %w", err)
	}

	// Get remote service and register a handler
	remoteService := app.GetRemoteService()
	if remoteService != nil {
		handler := &exampleHandler{}
		if err := remoteService.Register("example_service", handler); err != nil {
			return fmt.Errorf("failed to register service: %w", err)
		}
	}

	// Get cluster manager and listen for events
	manager := app.GetClusterManager()
	if manager != nil {
		go func() {
			for event := range manager.Events() {
				fmt.Printf("Cluster event: %s - Node: %s\n", event.Type, event.NodeID)
			}
		}()
	}

	// Start application
	ctx := context.Background()
	if err := app.Start(ctx); err != nil {
		return fmt.Errorf("failed to start application: %w", err)
	}

	return nil
}

// exampleHandler is a simple remote call handler
type exampleHandler struct{}

func (eh *exampleHandler) Handle(ctx context.Context, request interface{}) (interface{}, error) {
	return fmt.Sprintf("Hello from %v", request), nil
}
