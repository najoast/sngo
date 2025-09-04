// Package bootstrap provides application implementation
package bootstrap

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/najoast/sngo/config"
	"github.com/najoast/sngo/core"
	"github.com/najoast/sngo/network"
)

// DefaultApplication implements the Application interface
type DefaultApplication struct {
	// config holds the application configuration
	config interface{}

	// container provides dependency injection
	container Container

	// lifecycleManager manages service lifecycles
	lifecycleManager LifecycleManager

	// configLoader manages configuration loading
	configLoader *config.Loader

	// actor system for message passing
	actorSystem core.ActorSystem

	// network server for TCP connections
	networkServer network.Server

	// mutex protects concurrent access
	mutex sync.RWMutex

	// running indicates if the application is running
	running bool

	// shutdownChan for graceful shutdown
	shutdownChan chan os.Signal
}

// NewApplication creates a new SNGO application
func NewApplication() Application {
	container := NewContainer()
	lifecycleManager := NewLifecycleManager(container)

	app := &DefaultApplication{
		container:        container,
		lifecycleManager: lifecycleManager,
		shutdownChan:     make(chan os.Signal, 1),
		configLoader:     config.NewLoader(),
	}

	// Register core services
	app.registerCoreServices()

	return app
}

// Configure configures the application with the provided configuration
func (app *DefaultApplication) Configure(cfg interface{}) error {
	app.mutex.Lock()
	defer app.mutex.Unlock()

	if app.running {
		return fmt.Errorf("cannot configure application while running")
	}

	app.config = cfg
	return app.configureCoreServices(cfg)
}

// Run runs the application until shutdown
func (app *DefaultApplication) Run(ctx context.Context) error {
	app.mutex.Lock()
	if app.running {
		app.mutex.Unlock()
		return fmt.Errorf("application is already running")
	}
	app.running = true
	app.mutex.Unlock()

	// Setup signal handling for graceful shutdown
	signal.Notify(app.shutdownChan, os.Interrupt, syscall.SIGTERM)

	// Start all services
	if err := app.lifecycleManager.Start(ctx); err != nil {
		app.mutex.Lock()
		app.running = false
		app.mutex.Unlock()
		return fmt.Errorf("failed to start services: %w", err)
	}

	// Wait for shutdown signal or context cancellation
	select {
	case <-app.shutdownChan:
		fmt.Println("Received shutdown signal, starting graceful shutdown...")
	case <-ctx.Done():
		fmt.Println("Context cancelled, starting graceful shutdown...")
	}

	// Shutdown gracefully
	return app.Shutdown(context.Background())
}

// Shutdown shuts down the application gracefully
func (app *DefaultApplication) Shutdown(ctx context.Context) error {
	app.mutex.Lock()
	if !app.running {
		app.mutex.Unlock()
		return nil // Already shut down
	}
	app.running = false
	app.mutex.Unlock()

	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Stop all services
	if err := app.lifecycleManager.Stop(shutdownCtx); err != nil {
		return fmt.Errorf("failed to stop services: %w", err)
	}

	return nil
}

// Container returns the dependency injection container
func (app *DefaultApplication) Container() Container {
	return app.container
}

// LifecycleManager returns the lifecycle manager
func (app *DefaultApplication) LifecycleManager() LifecycleManager {
	return app.lifecycleManager
}

// registerCoreServices registers core SNGO services
func (app *DefaultApplication) registerCoreServices() {
	// Register actor system service
	app.lifecycleManager.Register("actor-system", &ActorSystemService{app: app})

	// Register network server service
	app.lifecycleManager.Register("network-server", &NetworkServerService{app: app}, "actor-system")
}

// configureCoreServices configures core services with the provided configuration
func (app *DefaultApplication) configureCoreServices(cfg interface{}) error {
	// Initialize actor system
	actorSystem := core.NewActorSystem()
	app.actorSystem = actorSystem

	// Register actor system in container
	app.container.RegisterInstance("actor-system", actorSystem)

	// Initialize network server if configuration is provided
	if configMap, ok := cfg.(map[string]interface{}); ok {
		if networkConfig, exists := configMap["network"]; exists {
			if netCfg, ok := networkConfig.(map[string]interface{}); ok {
				networkConfig := &network.NetworkConfig{
					Protocol: network.ProtocolTCP,
					Address:  "localhost",
					Port:     8080,
				}

				// Configure server if address is provided
				if addr, exists := netCfg["address"]; exists {
					if addrStr, ok := addr.(string); ok {
						// Parse address:port format
						parts := strings.Split(addrStr, ":")
						if len(parts) == 2 {
							networkConfig.Address = parts[0]
							if port, err := strconv.Atoi(parts[1]); err == nil {
								networkConfig.Port = port
							}
						} else {
							networkConfig.Address = addrStr
						}
					}
				}

				server, err := network.NewTCPServer(networkConfig)
				if err != nil {
					return fmt.Errorf("failed to create network server: %w", err)
				}

				app.networkServer = server
				app.container.RegisterInstance("network-server", server)
			}
		}
	}

	return nil
}

// ActorSystemService wraps the actor system as a managed service
type ActorSystemService struct {
	app *DefaultApplication
}

func (s *ActorSystemService) Name() string {
	return "actor-system"
}

func (s *ActorSystemService) Start(ctx context.Context) error {
	if s.app.actorSystem == nil {
		s.app.actorSystem = core.NewActorSystem()
		s.app.container.RegisterInstance("actor-system", s.app.actorSystem)
	}
	return nil
}

func (s *ActorSystemService) Stop(ctx context.Context) error {
	if s.app.actorSystem != nil {
		return s.app.actorSystem.Shutdown(ctx)
	}
	return nil
}

func (s *ActorSystemService) Health(ctx context.Context) (HealthStatus, error) {
	if s.app.actorSystem == nil {
		return HealthStatus{
			State:   HealthUnhealthy,
			Message: "Actor system not initialized",
		}, nil
	}

	return HealthStatus{
		State:   HealthHealthy,
		Message: "Actor system running",
	}, nil
}

// NetworkServerService wraps the network server as a managed service
type NetworkServerService struct {
	app *DefaultApplication
}

func (s *NetworkServerService) Name() string {
	return "network-server"
}

func (s *NetworkServerService) Start(ctx context.Context) error {
	if s.app.networkServer == nil {
		return nil // No network server configured
	}

	return s.app.networkServer.Start()
}

func (s *NetworkServerService) Stop(ctx context.Context) error {
	if s.app.networkServer == nil {
		return nil
	}

	return s.app.networkServer.Stop()
}

func (s *NetworkServerService) Health(ctx context.Context) (HealthStatus, error) {
	if s.app.networkServer == nil {
		return HealthStatus{
			State:   HealthUnknown,
			Message: "Network server not configured",
		}, nil
	}

	// Check if server has active connections to determine if it's running
	connectionCount := s.app.networkServer.GetConnectionCount()

	return HealthStatus{
		State:   HealthHealthy,
		Message: "Network server running",
		Data: map[string]interface{}{
			"connections": connectionCount,
		},
	}, nil
}

// ApplicationBuilder helps build and configure applications
type ApplicationBuilder struct {
	app    *DefaultApplication
	config map[string]interface{}
}

// NewApplicationBuilder creates a new application builder
func NewApplicationBuilder() *ApplicationBuilder {
	return &ApplicationBuilder{
		app:    NewApplication().(*DefaultApplication),
		config: make(map[string]interface{}),
	}
}

// WithConfig sets the configuration
func (b *ApplicationBuilder) WithConfig(cfg interface{}) *ApplicationBuilder {
	if configMap, ok := cfg.(map[string]interface{}); ok {
		for k, v := range configMap {
			b.config[k] = v
		}
	}
	return b
}

// WithConfigFile loads configuration from a file
func (b *ApplicationBuilder) WithConfigFile(filename string) *ApplicationBuilder {
	// For now, just return self - config file loading can be implemented later
	// when we have a clearer configuration structure
	return b
}

// WithService registers a service
func (b *ApplicationBuilder) WithService(name string, service Service, deps ...string) *ApplicationBuilder {
	b.app.lifecycleManager.Register(name, service, deps...)
	return b
}

// WithServiceFactory registers a service factory
func (b *ApplicationBuilder) WithServiceFactory(name string, factory ServiceFactory) *ApplicationBuilder {
	b.app.container.Register(name, factory)
	return b
}

// WithActorSystemConfig configures the actor system
func (b *ApplicationBuilder) WithActorSystemConfig() *ApplicationBuilder {
	b.config["actor_system"] = map[string]interface{}{
		"enabled": true,
	}
	return b
}

// WithNetworkConfig configures the network server
func (b *ApplicationBuilder) WithNetworkConfig(address string) *ApplicationBuilder {
	b.config["network"] = map[string]interface{}{
		"address": address,
	}
	return b
}

// Build builds the configured application
func (b *ApplicationBuilder) Build() (Application, error) {
	if len(b.config) > 0 {
		if err := b.app.Configure(b.config); err != nil {
			return nil, fmt.Errorf("failed to configure application: %w", err)
		}
	}
	return b.app, nil
}
