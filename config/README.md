# SNGO Configuration System

The SNGO configuration system provides a comprehensive, flexible way to manage application configuration with support for multiple formats, environment variable overrides, hot-reload, and validation.

## Features

- **Multiple Format Support**: YAML and JSON configuration files
- **Environment Variable Overrides**: Override any configuration value via environment variables
- **Auto-Discovery**: Automatically find and load configuration files
- **Hot-Reload**: Watch configuration files for changes and reload automatically
- **Validation**: Built-in validation for all configuration values
- **Default Values**: Sensible defaults for all configuration options
- **Type Safety**: Strong typing for all configuration structures

## Quick Start

### Basic Usage

```go
import "github.com/najoast/sngo/config"

// Create a loader
loader := config.NewLoader()

// Auto-load configuration (looks for config.yaml, config.yml, config.json)
cfg, err := loader.AutoLoad()
if err != nil {
    log.Fatal(err)
}

// Use configuration
fmt.Printf("App: %s v%s\n", cfg.App.Name, cfg.App.Version)
fmt.Printf("TCP Port: %d\n", cfg.Network.TCP.Port)
```

### Load from Specific File

```go
// Load from specific file
cfg, err := loader.LoadFromFile("myapp.yaml")
if err != nil {
    log.Fatal(err)
}
```

### Configuration Watching

```go
// Create file provider with watching
provider, err := config.NewFileProvider("config.yaml")
if err != nil {
    log.Fatal(err)
}
defer provider.Close()

// Watch for changes
ctx := context.Background()
err = provider.Watch(ctx, func(oldConfig, newConfig *config.Config) {
    fmt.Println("Configuration changed!")
    // Handle configuration change
})
```

## Configuration Structure

### Application Configuration

```yaml
app:
  name: "my-service"           # Application name
  version: "1.0.0"             # Application version
  environment: "development"   # Environment: development, testing, staging, production
  debug: true                  # Enable debug mode
  description: "My service"    # Optional description
  metadata:                    # Optional metadata
    team: "backend"
    project: "microservices"
```

### Logging Configuration

```yaml
log:
  level: "info"                # Log level: trace, debug, info, warn, error, fatal
  format: "text"               # Format: text, json
  output: "stdout"             # Output: stdout, stderr, or file path
  color: true                  # Enable colored output
  rotation:                    # Log rotation settings
    enabled: false
    max_size: 100              # Maximum file size in MB
    max_backups: 3             # Number of backup files to keep
    max_age: 7                 # Maximum age in days
    compress: true             # Compress old files
  fields:                      # Additional fields to include
    service: "my-service"
```

### Network Configuration

```yaml
network:
  tcp:
    address: "0.0.0.0"         # TCP listening address
    port: 8080                 # TCP listening port
    keep_alive: true           # Enable TCP keep-alive
    keep_alive_interval: "60s" # Keep-alive interval
    buffer_size: 4096          # Buffer size for read/write
  
  udp:
    address: "0.0.0.0"         # UDP listening address
    port: 8081                 # UDP listening port
    buffer_size: 4096          # Buffer size
  
  limits:
    max_connections: 1000      # Maximum concurrent connections
    max_connections_per_ip: 100 # Maximum connections per IP
    rate_limit: 100            # Rate limit (connections per second)
  
  timeouts:
    read: "30s"                # Read timeout
    write: "30s"               # Write timeout
    idle: "5m"                 # Idle timeout
    handshake: "10s"           # Handshake timeout
```

### Actor System Configuration

```yaml
actor:
  max_actors: 10000            # Maximum number of actors
  default_mailbox_size: 1000   # Default mailbox size for actors
  timeouts:
    creation: "5s"             # Actor creation timeout
    shutdown: "10s"            # Actor shutdown timeout
    message_send: "1s"         # Message send timeout
    call: "30s"                # Actor call timeout
  routing:
    strategy: "direct"         # Routing strategy: direct, consistent_hash, round_robin
    persistence: false         # Enable message persistence
    message_ttl: "5m"          # Message time-to-live
```

### Service Discovery Configuration

```yaml
discovery:
  enabled: true                # Enable service discovery
  type: "local"                # Discovery type: local, consul, etcd, kubernetes
  registration:
    name: "my-service"         # Service name for registration
    version: "1.0.0"           # Service version
    ttl: "30s"                 # Registration TTL
    tags:                      # Service tags
      - "api"
      - "microservice"
    metadata:                  # Service metadata
      zone: "us-east-1"
  health_check:
    enabled: true              # Enable health checks
    interval: "10s"            # Health check interval
    timeout: "5s"              # Health check timeout
    http_endpoint: "/health"   # HTTP health check endpoint
  load_balancing:
    strategy: "round_robin"    # Load balancing strategy
    health_check: true         # Use health checks for load balancing
    circuit_breaker:
      enabled: false           # Enable circuit breaker
      failure_threshold: 5     # Failure threshold
      success_threshold: 3     # Success threshold for recovery
      timeout: "60s"           # Circuit breaker timeout
```

### Monitoring Configuration

```yaml
monitor:
  enabled: true                # Enable monitoring
  metrics_interval: "10s"      # Metrics collection interval
  http:
    enabled: true              # Enable HTTP monitoring endpoint
    address: "0.0.0.0"         # HTTP monitoring address
    port: 9090                 # HTTP monitoring port
    metrics_path: "/metrics"   # Metrics endpoint path
    health_path: "/health"     # Health check endpoint path
    pprof_enabled: false       # Enable pprof endpoints
  profiling:
    enabled: false             # Enable profiling
    cpu: false                 # Enable CPU profiling
    memory: false              # Enable memory profiling
    block: false               # Enable block profiling
    mutex: false               # Enable mutex profiling
```

### Custom Configuration

```yaml
custom:
  # Any custom application-specific configuration
  database:
    driver: "postgres"
    host: "localhost"
    port: 5432
    database: "myapp"
    username: "user"
    password: "secret"
  
  redis:
    address: "localhost:6379"
    password: ""
    database: 0
  
  features:
    enable_caching: true
    enable_metrics: true
```

## Environment Variable Overrides

Any configuration value can be overridden using environment variables with the prefix `SNGO_`:

```bash
# Override app name
export SNGO_APP_NAME="production-service"

# Override TCP port
export SNGO_NETWORK_TCP_PORT="9090"

# Override log level
export SNGO_LOG_LEVEL="debug"

# Override environment
export SNGO_APP_ENVIRONMENT="production"
```

Environment variable names are derived from the configuration path:
- `app.name` → `SNGO_APP_NAME`
- `network.tcp.port` → `SNGO_NETWORK_TCP_PORT`
- `log.level` → `SNGO_LOG_LEVEL`

## Configuration Validation

The configuration system automatically validates all values:

- **App name**: Must not be empty
- **Environment**: Must be one of: development, testing, staging, production
- **Log level**: Must be one of: trace, debug, info, warn, error, fatal
- **Ports**: Must be in range 1-65535
- **Connection limits**: Must be positive integers
- **Actor limits**: Must be positive integers

## Default Configuration

If no configuration file is found, the system uses sensible defaults:

```go
cfg := config.DefaultConfig()
// Returns fully populated configuration with default values
```

## File Auto-Discovery

The loader automatically searches for configuration files in this order:

1. `config.yaml`
2. `config.yml` 
3. `config.json`

## Hot-Reload

The configuration system supports hot-reload through file watching:

```go
watcher, err := config.NewWatcher("config.yaml", loader)
if err != nil {
    log.Fatal(err)
}

// Register change callback
watcher.OnConfigChange(func(oldConfig, newConfig *config.Config) {
    log.Println("Configuration reloaded!")
    // Update application state
})

// Start watching
err = watcher.Start()
if err != nil {
    log.Fatal(err)
}

// Stop watching when done
defer watcher.Stop()
```

## Examples

See the [config_demo](../examples/config_demo/) directory for a complete example demonstrating:

- Configuration loading
- Environment variable overrides
- Hot-reload capabilities
- Configuration validation

## Error Handling

The configuration system provides detailed error messages for common issues:

- Invalid configuration file format
- Missing required fields
- Invalid field values
- File system errors
- Validation failures

```go
cfg, err := loader.AutoLoad()
if err != nil {
    switch {
    case errors.Is(err, config.ErrConfigFileNotFound):
        log.Println("No config file found, using defaults")
    case errors.Is(err, config.ErrInvalidLogLevel):
        log.Println("Invalid log level specified")
    default:
        log.Fatalf("Configuration error: %v", err)
    }
}
```

## Best Practices

1. **Use environment variables for secrets**: Never store passwords or API keys in config files
2. **Validate early**: Load and validate configuration at application startup
3. **Handle hot-reload gracefully**: Ensure your application can handle configuration changes
4. **Use defaults**: Provide sensible defaults for all optional configuration
5. **Document custom config**: Document any custom configuration sections
6. **Test configuration**: Include configuration loading in your tests

## Testing

The configuration system includes comprehensive tests. Run them with:

```bash
go test ./config -v
```

Tests cover:
- Configuration loading (YAML/JSON)
- Environment variable overrides
- Validation
- File watching
- Error handling
- Default configuration
