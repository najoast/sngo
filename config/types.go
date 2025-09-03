// Package config provides configuration management for SNGO framework
package config

import (
	"time"
)

// Environment represents the deployment environment
type Environment string

const (
	EnvDevelopment Environment = "development"
	EnvTesting     Environment = "testing"
	EnvStaging     Environment = "staging"
	EnvProduction  Environment = "production"
)

// String returns the string representation of Environment
func (e Environment) String() string {
	return string(e)
}

// IsValid checks if the environment is valid
func (e Environment) IsValid() bool {
	switch e {
	case EnvDevelopment, EnvTesting, EnvStaging, EnvProduction:
		return true
	default:
		return false
	}
}

// LogLevel represents the logging level
type LogLevel string

const (
	LogLevelTrace LogLevel = "trace"
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
	LogLevelFatal LogLevel = "fatal"
)

// String returns the string representation of LogLevel
func (l LogLevel) String() string {
	return string(l)
}

// IsValid checks if the log level is valid
func (l LogLevel) IsValid() bool {
	switch l {
	case LogLevelTrace, LogLevelDebug, LogLevelInfo, LogLevelWarn, LogLevelError, LogLevelFatal:
		return true
	default:
		return false
	}
}

// Config represents the complete SNGO configuration
type Config struct {
	// Application configuration
	App AppConfig `yaml:"app" json:"app"`

	// Logging configuration
	Log LogConfig `yaml:"log" json:"log"`

	// Network configuration
	Network NetworkConfig `yaml:"network" json:"network"`

	// Actor system configuration
	Actor ActorConfig `yaml:"actor" json:"actor"`

	// Service discovery configuration
	Discovery DiscoveryConfig `yaml:"discovery" json:"discovery"`

	// Monitoring configuration
	Monitor MonitorConfig `yaml:"monitor" json:"monitor"`

	// Custom configurations (for user-defined services)
	Custom map[string]interface{} `yaml:"custom,omitempty" json:"custom,omitempty"`
}

// AppConfig contains application-level configuration
type AppConfig struct {
	// Application name
	Name string `yaml:"name" json:"name"`

	// Application version
	Version string `yaml:"version" json:"version"`

	// Deployment environment
	Environment Environment `yaml:"environment" json:"environment"`

	// Debug mode
	Debug bool `yaml:"debug" json:"debug"`

	// Application description
	Description string `yaml:"description,omitempty" json:"description,omitempty"`

	// Application metadata
	Metadata map[string]string `yaml:"metadata,omitempty" json:"metadata,omitempty"`
}

// LogConfig contains logging configuration
type LogConfig struct {
	// Log level
	Level LogLevel `yaml:"level" json:"level"`

	// Log format (json, text)
	Format string `yaml:"format" json:"format"`

	// Output destination (stdout, stderr, file path)
	Output string `yaml:"output" json:"output"`

	// Enable colored output
	Color bool `yaml:"color" json:"color"`

	// Log rotation configuration
	Rotation LogRotationConfig `yaml:"rotation" json:"rotation"`

	// Fields to include in log output
	Fields map[string]interface{} `yaml:"fields,omitempty" json:"fields,omitempty"`
}

// LogRotationConfig contains log rotation settings
type LogRotationConfig struct {
	// Enable log rotation
	Enabled bool `yaml:"enabled" json:"enabled"`

	// Maximum file size in MB
	MaxSize int `yaml:"max_size" json:"max_size"`

	// Maximum number of old files to retain
	MaxBackups int `yaml:"max_backups" json:"max_backups"`

	// Maximum age in days
	MaxAge int `yaml:"max_age" json:"max_age"`

	// Compress old files
	Compress bool `yaml:"compress" json:"compress"`
}

// NetworkConfig contains network-related configuration
type NetworkConfig struct {
	// TCP server configuration
	TCP TCPConfig `yaml:"tcp" json:"tcp"`

	// UDP server configuration (future)
	UDP UDPConfig `yaml:"udp" json:"udp"`

	// Connection limits
	Limits ConnectionLimits `yaml:"limits" json:"limits"`

	// Timeouts
	Timeouts TimeoutConfig `yaml:"timeouts" json:"timeouts"`
}

// TCPConfig contains TCP-specific configuration
type TCPConfig struct {
	// Listening address
	Address string `yaml:"address" json:"address"`

	// Listening port
	Port int `yaml:"port" json:"port"`

	// Enable TCP keep-alive
	KeepAlive bool `yaml:"keep_alive" json:"keep_alive"`

	// Keep-alive interval
	KeepAliveInterval time.Duration `yaml:"keep_alive_interval" json:"keep_alive_interval"`

	// Buffer size for reading/writing
	BufferSize int `yaml:"buffer_size" json:"buffer_size"`
}

// UDPConfig contains UDP-specific configuration (placeholder)
type UDPConfig struct {
	// Listening address
	Address string `yaml:"address" json:"address"`

	// Listening port
	Port int `yaml:"port" json:"port"`

	// Buffer size
	BufferSize int `yaml:"buffer_size" json:"buffer_size"`
}

// ConnectionLimits contains connection limit settings
type ConnectionLimits struct {
	// Maximum concurrent connections
	MaxConnections int `yaml:"max_connections" json:"max_connections"`

	// Maximum connections per IP
	MaxConnectionsPerIP int `yaml:"max_connections_per_ip" json:"max_connections_per_ip"`

	// Rate limiting (connections per second)
	RateLimit int `yaml:"rate_limit" json:"rate_limit"`
}

// TimeoutConfig contains timeout settings
type TimeoutConfig struct {
	// Read timeout
	Read time.Duration `yaml:"read" json:"read"`

	// Write timeout
	Write time.Duration `yaml:"write" json:"write"`

	// Idle timeout
	Idle time.Duration `yaml:"idle" json:"idle"`

	// Handshake timeout
	Handshake time.Duration `yaml:"handshake" json:"handshake"`
}

// ActorConfig contains actor system configuration
type ActorConfig struct {
	// Maximum number of actors
	MaxActors int `yaml:"max_actors" json:"max_actors"`

	// Default actor mailbox size
	DefaultMailboxSize int `yaml:"default_mailbox_size" json:"default_mailbox_size"`

	// Actor timeout settings
	Timeouts ActorTimeoutConfig `yaml:"timeouts" json:"timeouts"`

	// Message routing configuration
	Routing RoutingConfig `yaml:"routing" json:"routing"`
}

// ActorTimeoutConfig contains actor timeout settings
type ActorTimeoutConfig struct {
	// Actor creation timeout
	Creation time.Duration `yaml:"creation" json:"creation"`

	// Actor shutdown timeout
	Shutdown time.Duration `yaml:"shutdown" json:"shutdown"`

	// Message send timeout
	MessageSend time.Duration `yaml:"message_send" json:"message_send"`

	// Call timeout
	Call time.Duration `yaml:"call" json:"call"`
}

// RoutingConfig contains message routing configuration
type RoutingConfig struct {
	// Default routing strategy
	Strategy string `yaml:"strategy" json:"strategy"`

	// Enable message persistence
	Persistence bool `yaml:"persistence" json:"persistence"`

	// Message TTL
	MessageTTL time.Duration `yaml:"message_ttl" json:"message_ttl"`
}

// DiscoveryConfig contains service discovery configuration
type DiscoveryConfig struct {
	// Enable service discovery
	Enabled bool `yaml:"enabled" json:"enabled"`

	// Service registry type (local, consul, etcd)
	Type string `yaml:"type" json:"type"`

	// Service registration configuration
	Registration ServiceRegistrationConfig `yaml:"registration" json:"registration"`

	// Health check configuration
	HealthCheck HealthCheckConfig `yaml:"health_check" json:"health_check"`

	// Load balancing configuration
	LoadBalancing LoadBalancingConfig `yaml:"load_balancing" json:"load_balancing"`
}

// ServiceRegistrationConfig contains service registration settings
type ServiceRegistrationConfig struct {
	// Service name
	Name string `yaml:"name" json:"name"`

	// Service version
	Version string `yaml:"version" json:"version"`

	// Service tags
	Tags []string `yaml:"tags,omitempty" json:"tags,omitempty"`

	// Service metadata
	Metadata map[string]string `yaml:"metadata,omitempty" json:"metadata,omitempty"`

	// TTL for service registration
	TTL time.Duration `yaml:"ttl" json:"ttl"`
}

// HealthCheckConfig contains health check settings
type HealthCheckConfig struct {
	// Enable health checks
	Enabled bool `yaml:"enabled" json:"enabled"`

	// Health check interval
	Interval time.Duration `yaml:"interval" json:"interval"`

	// Health check timeout
	Timeout time.Duration `yaml:"timeout" json:"timeout"`

	// Health check endpoint
	Endpoint string `yaml:"endpoint,omitempty" json:"endpoint,omitempty"`
}

// LoadBalancingConfig contains load balancing settings
type LoadBalancingConfig struct {
	// Load balancing strategy
	Strategy string `yaml:"strategy" json:"strategy"`

	// Health check before routing
	HealthCheck bool `yaml:"health_check" json:"health_check"`

	// Circuit breaker settings
	CircuitBreaker CircuitBreakerConfig `yaml:"circuit_breaker" json:"circuit_breaker"`
}

// CircuitBreakerConfig contains circuit breaker settings
type CircuitBreakerConfig struct {
	// Enable circuit breaker
	Enabled bool `yaml:"enabled" json:"enabled"`

	// Failure threshold
	FailureThreshold int `yaml:"failure_threshold" json:"failure_threshold"`

	// Success threshold for recovery
	SuccessThreshold int `yaml:"success_threshold" json:"success_threshold"`

	// Timeout for open state
	Timeout time.Duration `yaml:"timeout" json:"timeout"`
}

// MonitorConfig contains monitoring configuration
type MonitorConfig struct {
	// Enable monitoring
	Enabled bool `yaml:"enabled" json:"enabled"`

	// Metrics collection interval
	MetricsInterval time.Duration `yaml:"metrics_interval" json:"metrics_interval"`

	// HTTP server for metrics
	HTTP HTTPMonitorConfig `yaml:"http" json:"http"`

	// Profiling configuration
	Profiling ProfilingConfig `yaml:"profiling" json:"profiling"`
}

// HTTPMonitorConfig contains HTTP monitoring server settings
type HTTPMonitorConfig struct {
	// Enable HTTP monitoring server
	Enabled bool `yaml:"enabled" json:"enabled"`

	// HTTP server address
	Address string `yaml:"address" json:"address"`

	// HTTP server port
	Port int `yaml:"port" json:"port"`

	// Metrics endpoint path
	MetricsPath string `yaml:"metrics_path" json:"metrics_path"`

	// Health endpoint path
	HealthPath string `yaml:"health_path" json:"health_path"`
}

// ProfilingConfig contains profiling settings
type ProfilingConfig struct {
	// Enable profiling
	Enabled bool `yaml:"enabled" json:"enabled"`

	// CPU profiling
	CPU bool `yaml:"cpu" json:"cpu"`

	// Memory profiling
	Memory bool `yaml:"memory" json:"memory"`

	// Block profiling
	Block bool `yaml:"block" json:"block"`

	// Mutex profiling
	Mutex bool `yaml:"mutex" json:"mutex"`
}

// DefaultConfig returns a default configuration
func DefaultConfig() *Config {
	return &Config{
		App: AppConfig{
			Name:        "sngo-app",
			Version:     "1.0.0",
			Environment: EnvDevelopment,
			Debug:       true,
			Description: "SNGO application",
		},
		Log: LogConfig{
			Level:  LogLevelInfo,
			Format: "text",
			Output: "stdout",
			Color:  true,
			Rotation: LogRotationConfig{
				Enabled:    false,
				MaxSize:    100,
				MaxBackups: 3,
				MaxAge:     7,
				Compress:   true,
			},
		},
		Network: NetworkConfig{
			TCP: TCPConfig{
				Address:           "0.0.0.0",
				Port:              8080,
				KeepAlive:         true,
				KeepAliveInterval: 60 * time.Second,
				BufferSize:        4096,
			},
			UDP: UDPConfig{
				Address:    "0.0.0.0",
				Port:       8081,
				BufferSize: 4096,
			},
			Limits: ConnectionLimits{
				MaxConnections:      1000,
				MaxConnectionsPerIP: 100,
				RateLimit:           100,
			},
			Timeouts: TimeoutConfig{
				Read:      30 * time.Second,
				Write:     30 * time.Second,
				Idle:      5 * time.Minute,
				Handshake: 10 * time.Second,
			},
		},
		Actor: ActorConfig{
			MaxActors:          10000,
			DefaultMailboxSize: 1000,
			Timeouts: ActorTimeoutConfig{
				Creation:    5 * time.Second,
				Shutdown:    10 * time.Second,
				MessageSend: 1 * time.Second,
				Call:        30 * time.Second,
			},
			Routing: RoutingConfig{
				Strategy:    "direct",
				Persistence: false,
				MessageTTL:  5 * time.Minute,
			},
		},
		Discovery: DiscoveryConfig{
			Enabled: true,
			Type:    "local",
			Registration: ServiceRegistrationConfig{
				Name:    "sngo-service",
				Version: "1.0.0",
				TTL:     30 * time.Second,
			},
			HealthCheck: HealthCheckConfig{
				Enabled:  true,
				Interval: 10 * time.Second,
				Timeout:  5 * time.Second,
			},
			LoadBalancing: LoadBalancingConfig{
				Strategy:    "round_robin",
				HealthCheck: true,
				CircuitBreaker: CircuitBreakerConfig{
					Enabled:          false,
					FailureThreshold: 5,
					SuccessThreshold: 3,
					Timeout:          60 * time.Second,
				},
			},
		},
		Monitor: MonitorConfig{
			Enabled:         true,
			MetricsInterval: 10 * time.Second,
			HTTP: HTTPMonitorConfig{
				Enabled:     true,
				Address:     "0.0.0.0",
				Port:        9090,
				MetricsPath: "/metrics",
				HealthPath:  "/health",
			},
			Profiling: ProfilingConfig{
				Enabled: false,
				CPU:     false,
				Memory:  false,
				Block:   false,
				Mutex:   false,
			},
		},
		Custom: make(map[string]interface{}),
	}
}

// Validate validates the configuration
func (c *Config) Validate() error {
	// Validate app config
	if c.App.Name == "" {
		return ErrInvalidAppName
	}
	if !c.App.Environment.IsValid() {
		return ErrInvalidEnvironment
	}

	// Validate log config
	if !c.Log.Level.IsValid() {
		return ErrInvalidLogLevel
	}

	// Validate network config
	if c.Network.TCP.Port <= 0 || c.Network.TCP.Port > 65535 {
		return ErrInvalidPort
	}
	if c.Network.Limits.MaxConnections <= 0 {
		return ErrInvalidMaxConnections
	}

	// Validate actor config
	if c.Actor.MaxActors <= 0 {
		return ErrInvalidMaxActors
	}
	if c.Actor.DefaultMailboxSize <= 0 {
		return ErrInvalidMailboxSize
	}

	return nil
}

// IsDevelopment returns true if the environment is development
func (c *Config) IsDevelopment() bool {
	return c.App.Environment == EnvDevelopment
}

// IsProduction returns true if the environment is production
func (c *Config) IsProduction() bool {
	return c.App.Environment == EnvProduction
}

// GetServiceName returns the service name for registration
func (c *Config) GetServiceName() string {
	if c.Discovery.Registration.Name != "" {
		return c.Discovery.Registration.Name
	}
	return c.App.Name
}

// GetLogLevel returns the log level
func (c *Config) GetLogLevel() LogLevel {
	return c.Log.Level
}

// IsDebugEnabled returns true if debug mode is enabled
func (c *Config) IsDebugEnabled() bool {
	return c.App.Debug || c.App.Environment == EnvDevelopment
}
