// Package config provides configuration loading and parsing functionality
package config

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// ConfigFormat represents the configuration file format
type ConfigFormat string

const (
	FormatYAML ConfigFormat = "yaml"
	FormatJSON ConfigFormat = "json"
)

// Loader handles configuration loading from various sources
type Loader struct {
	// Configuration search paths
	searchPaths []string

	// Environment variable prefix
	envPrefix string

	// Default configuration
	defaultConfig *Config
}

// NewLoader creates a new configuration loader
func NewLoader() *Loader {
	return &Loader{
		searchPaths: []string{
			".",
			"./config",
			"./configs",
			"/etc/sngo",
			os.Getenv("HOME") + "/.sngo",
		},
		envPrefix:     "SNGO",
		defaultConfig: DefaultConfig(),
	}
}

// SetSearchPaths sets the configuration file search paths
func (l *Loader) SetSearchPaths(paths []string) *Loader {
	l.searchPaths = paths
	return l
}

// SetEnvPrefix sets the environment variable prefix
func (l *Loader) SetEnvPrefix(prefix string) *Loader {
	l.envPrefix = prefix
	return l
}

// SetDefaultConfig sets the default configuration
func (l *Loader) SetDefaultConfig(config *Config) *Loader {
	l.defaultConfig = config
	return l
}

// Load loads configuration from the specified file
func (l *Loader) Load(filename string) (*Config, error) {
	// Start with default configuration
	config := l.defaultConfig
	if config == nil {
		config = DefaultConfig()
	}

	// Try to load configuration file
	if filename != "" {
		fileConfig, err := l.loadFromFile(filename)
		if err != nil {
			return nil, fmt.Errorf("failed to load config from file %s: %w", filename, err)
		}
		config = fileConfig
	}

	// Override with environment variables
	err := l.loadFromEnv(config)
	if err != nil {
		return nil, fmt.Errorf("failed to load config from environment: %w", err)
	}

	// Validate configuration
	err = config.Validate()
	if err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return config, nil
}

// LoadFromFile loads configuration from a specific file
func (l *Loader) LoadFromFile(filename string) (*Config, error) {
	return l.loadFromFile(filename)
}

// LoadFromReader loads configuration from an io.Reader
func (l *Loader) LoadFromReader(reader io.Reader, format ConfigFormat) (*Config, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read configuration data: %w", err)
	}

	return l.parseConfig(data, format)
}

// AutoLoad automatically discovers and loads configuration
func (l *Loader) AutoLoad() (*Config, error) {
	// Try to find configuration file
	configFile, format, err := l.findConfigFile()
	if err != nil {
		// If no config file found, use default config
		if err == ErrConfigFileNotFound {
			config := l.defaultConfig
			if config == nil {
				config = DefaultConfig()
			}

			// Still apply environment variables
			err = l.loadFromEnv(config)
			if err != nil {
				return nil, fmt.Errorf("failed to load config from environment: %w", err)
			}

			// Validate configuration
			err = config.Validate()
			if err != nil {
				return nil, fmt.Errorf("configuration validation failed: %w", err)
			}

			return config, nil
		}
		return nil, err
	}

	// Load from found config file
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configFile, err)
	}

	config, err := l.parseConfig(data, format)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config file %s: %w", configFile, err)
	}

	// Merge with default config to fill missing fields
	defaultConfig := l.defaultConfig
	if defaultConfig == nil {
		defaultConfig = DefaultConfig()
	}
	config = l.mergeConfig(defaultConfig, config)

	// Override with environment variables
	err = l.loadFromEnv(config)
	if err != nil {
		return nil, fmt.Errorf("failed to load environment overrides: %w", err)
	}

	// Validate configuration
	err = config.Validate()
	if err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to load config from environment: %w", err)
	}

	// Validate configuration
	err = config.Validate()
	if err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return config, nil
}

// findConfigFile searches for configuration files in search paths
func (l *Loader) findConfigFile() (string, ConfigFormat, error) {
	filenames := []string{
		"sngo.yaml", "sngo.yml",
		"config.yaml", "config.yml",
		"sngo.json", "config.json",
	}

	for _, searchPath := range l.searchPaths {
		for _, filename := range filenames {
			fullPath := filepath.Join(searchPath, filename)
			if _, err := os.Stat(fullPath); err == nil {
				// Determine format from extension
				ext := strings.ToLower(filepath.Ext(filename))
				var format ConfigFormat
				switch ext {
				case ".yaml", ".yml":
					format = FormatYAML
				case ".json":
					format = FormatJSON
				default:
					continue
				}
				return fullPath, format, nil
			}
		}
	}

	return "", "", ErrConfigFileNotFound
}

// loadFromFile loads configuration from a file
func (l *Loader) loadFromFile(filename string) (*Config, error) {
	// Determine format from extension
	ext := strings.ToLower(filepath.Ext(filename))
	var format ConfigFormat
	switch ext {
	case ".yaml", ".yml":
		format = FormatYAML
	case ".json":
		format = FormatJSON
	default:
		return nil, fmt.Errorf("unsupported config file format: %s", ext)
	}

	// Read file
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	config, err := l.parseConfig(data, format)
	if err != nil {
		return nil, err
	}

	// Merge with default config to fill missing fields
	defaultConfig := l.defaultConfig
	if defaultConfig == nil {
		defaultConfig = DefaultConfig()
	}
	config = l.mergeConfig(defaultConfig, config)

	// Override with environment variables
	err = l.loadFromEnv(config)
	if err != nil {
		return nil, fmt.Errorf("failed to load environment overrides: %w", err)
	}

	// Validate configuration
	err = config.Validate()
	if err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return config, nil
}

// parseConfig parses configuration data based on format
func (l *Loader) parseConfig(data []byte, format ConfigFormat) (*Config, error) {
	config := &Config{}

	switch format {
	case FormatYAML:
		err := yaml.Unmarshal(data, config)
		if err != nil {
			return nil, fmt.Errorf("failed to parse YAML config: %w", err)
		}
	case FormatJSON:
		err := json.Unmarshal(data, config)
		if err != nil {
			return nil, fmt.Errorf("failed to parse JSON config: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported config format: %s", format)
	}

	return config, nil
}

// loadFromEnv loads configuration overrides from environment variables
func (l *Loader) loadFromEnv(config *Config) error {
	// App configuration
	if val := os.Getenv(l.envPrefix + "_APP_NAME"); val != "" {
		config.App.Name = val
	}
	if val := os.Getenv(l.envPrefix + "_APP_VERSION"); val != "" {
		config.App.Version = val
	}
	if val := os.Getenv(l.envPrefix + "_APP_ENVIRONMENT"); val != "" {
		config.App.Environment = Environment(val)
	}
	if val := os.Getenv(l.envPrefix + "_APP_DEBUG"); val != "" {
		config.App.Debug = strings.ToLower(val) == "true"
	}

	// Log configuration
	if val := os.Getenv(l.envPrefix + "_LOG_LEVEL"); val != "" {
		config.Log.Level = LogLevel(val)
	}
	if val := os.Getenv(l.envPrefix + "_LOG_FORMAT"); val != "" {
		config.Log.Format = val
	}
	if val := os.Getenv(l.envPrefix + "_LOG_OUTPUT"); val != "" {
		config.Log.Output = val
	}

	// Network configuration
	if val := os.Getenv(l.envPrefix + "_NETWORK_TCP_ADDRESS"); val != "" {
		config.Network.TCP.Address = val
	}
	if val := os.Getenv(l.envPrefix + "_NETWORK_TCP_PORT"); val != "" {
		if port, err := parsePort(val); err == nil {
			config.Network.TCP.Port = port
		}
	}

	// Discovery configuration
	if val := os.Getenv(l.envPrefix + "_DISCOVERY_ENABLED"); val != "" {
		config.Discovery.Enabled = strings.ToLower(val) == "true"
	}
	if val := os.Getenv(l.envPrefix + "_DISCOVERY_TYPE"); val != "" {
		config.Discovery.Type = val
	}
	if val := os.Getenv(l.envPrefix + "_SERVICE_NAME"); val != "" {
		config.Discovery.Registration.Name = val
	}

	// Monitor configuration
	if val := os.Getenv(l.envPrefix + "_MONITOR_ENABLED"); val != "" {
		config.Monitor.Enabled = strings.ToLower(val) == "true"
	}
	if val := os.Getenv(l.envPrefix + "_MONITOR_PORT"); val != "" {
		if port, err := parsePort(val); err == nil {
			config.Monitor.HTTP.Port = port
		}
	}

	return nil
}

// Helper function to parse port number
func parsePort(val string) (int, error) {
	var port int
	_, err := fmt.Sscanf(val, "%d", &port)
	if err != nil {
		return 0, err
	}
	if port <= 0 || port > 65535 {
		return 0, fmt.Errorf("invalid port number: %d", port)
	}
	return port, nil
}

// mergeConfig merges user config with default config
func (l *Loader) mergeConfig(defaultConfig, userConfig *Config) *Config {
	// Start with default config
	merged := *defaultConfig

	// Override with user config values where specified
	if userConfig.App.Name != "" {
		merged.App.Name = userConfig.App.Name
	}
	if userConfig.App.Version != "" {
		merged.App.Version = userConfig.App.Version
	}
	if userConfig.App.Environment != "" {
		merged.App.Environment = userConfig.App.Environment
	}
	if userConfig.App.Description != "" {
		merged.App.Description = userConfig.App.Description
	}
	merged.App.Debug = userConfig.App.Debug
	if userConfig.App.Metadata != nil {
		merged.App.Metadata = userConfig.App.Metadata
	}

	// Log config
	if userConfig.Log.Level != "" {
		merged.Log.Level = userConfig.Log.Level
	}
	if userConfig.Log.Format != "" {
		merged.Log.Format = userConfig.Log.Format
	}
	if userConfig.Log.Output != "" {
		merged.Log.Output = userConfig.Log.Output
	}
	merged.Log.Color = userConfig.Log.Color
	if userConfig.Log.Fields != nil {
		merged.Log.Fields = userConfig.Log.Fields
	}

	// Network config
	if userConfig.Network.TCP.Address != "" {
		merged.Network.TCP.Address = userConfig.Network.TCP.Address
	}
	if userConfig.Network.TCP.Port != 0 {
		merged.Network.TCP.Port = userConfig.Network.TCP.Port
	}
	if userConfig.Network.Limits.MaxConnections != 0 {
		merged.Network.Limits.MaxConnections = userConfig.Network.Limits.MaxConnections
	}

	// Actor config
	if userConfig.Actor.MaxActors != 0 {
		merged.Actor.MaxActors = userConfig.Actor.MaxActors
	}
	if userConfig.Actor.DefaultMailboxSize != 0 {
		merged.Actor.DefaultMailboxSize = userConfig.Actor.DefaultMailboxSize
	}

	// Discovery config
	if userConfig.Discovery.Registration.Name != "" {
		merged.Discovery.Registration.Name = userConfig.Discovery.Registration.Name
	}

	// Custom fields
	if userConfig.Custom != nil {
		if merged.Custom == nil {
			merged.Custom = make(map[string]interface{})
		}
		for k, v := range userConfig.Custom {
			merged.Custom[k] = v
		}
	}

	return &merged
}
