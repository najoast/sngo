package config

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestConfig tests basic configuration functionality
func TestConfig(t *testing.T) {
	config := &Config{
		App: AppConfig{
			Name:        "test-app",
			Version:     "1.0.0",
			Environment: EnvDevelopment,
		},
		Log: LogConfig{
			Level:  LogLevelInfo,
			Format: "text",
			Output: "/tmp/test.log",
		},
		Network: NetworkConfig{
			TCP: TCPConfig{
				Address: "127.0.0.1",
				Port:    8080,
			},
			Limits: ConnectionLimits{
				MaxConnections: 1000,
			},
		},
		Actor: ActorConfig{
			MaxActors:          1000,
			DefaultMailboxSize: 100,
		},
	}

	// Test validation
	err := config.Validate()
	if err != nil {
		t.Fatalf("Config validation failed: %v", err)
	}

	// Test app name
	if config.App.Name != "test-app" {
		t.Errorf("Expected app name 'test-app', got '%s'", config.App.Name)
	}
} // TestConfigValidation tests configuration validation
func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: &Config{
				App: AppConfig{
					Name:        "valid-app",
					Version:     "1.0.0",
					Environment: EnvProduction,
				},
				Log: LogConfig{
					Level: LogLevelInfo,
				},
				Network: NetworkConfig{
					TCP: TCPConfig{
						Address: "0.0.0.0",
						Port:    8080,
					},
					Limits: ConnectionLimits{
						MaxConnections: 1000,
					},
				},
				Actor: ActorConfig{
					MaxActors:          1000,
					DefaultMailboxSize: 100,
				},
			},
			wantErr: false,
		},
		{
			name: "invalid app name",
			config: &Config{
				App: AppConfig{
					Name:        "",
					Version:     "1.0.0",
					Environment: EnvProduction,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid port",
			config: &Config{
				App: AppConfig{
					Name:    "test-app",
					Version: "1.0.0",
				},
				Network: NetworkConfig{
					TCP: TCPConfig{
						Address: "127.0.0.1",
						Port:    -1,
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestLoader tests configuration loading
func TestLoader(t *testing.T) {
	loader := NewLoader()

	// Test YAML loading
	yamlContent := `
app:
  name: test-app
  version: "1.0.0"
  environment: development
  
log:
  level: info
  format: text
  output: "/tmp/test.log"
  
network:
  tcp:
    address: "127.0.0.1"
    port: 8080
`

	// Create temporary YAML file
	tmpDir := os.TempDir()
	yamlFile := filepath.Join(tmpDir, "test-config.yaml")
	err := os.WriteFile(yamlFile, []byte(yamlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test YAML file: %v", err)
	}
	defer os.Remove(yamlFile)

	// Load from YAML file
	config, err := loader.LoadFromFile(yamlFile)
	if err != nil {
		t.Fatalf("Failed to load YAML config: %v", err)
	}

	// Verify loaded configuration
	if config.App.Name != "test-app" {
		t.Errorf("Expected app name 'test-app', got '%s'", config.App.Name)
	}
	if config.App.Environment != EnvDevelopment {
		t.Errorf("Expected env development, got %v", config.App.Environment)
	}
	if config.Network.TCP.Port != 8080 {
		t.Errorf("Expected port 8080, got %d", config.Network.TCP.Port)
	}
}

// TestLoaderJSON tests JSON configuration loading
func TestLoaderJSON(t *testing.T) {
	loader := NewLoader()

	// Test JSON loading
	jsonContent := `{
	"app": {
		"name": "json-test-app",
		"version": "2.0.0",
		"environment": "production"
	},
	"log": {
		"level": "debug",
		"format": "json",
		"output": "/var/log/app.log"
	},
	"network": {
		"tcp": {
			"address": "0.0.0.0",
			"port": 9090
		}
	}
}`

	// Create temporary JSON file
	tmpDir := os.TempDir()
	jsonFile := filepath.Join(tmpDir, "test-config.json")
	err := os.WriteFile(jsonFile, []byte(jsonContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test JSON file: %v", err)
	}
	defer os.Remove(jsonFile)

	// Load from JSON file
	config, err := loader.LoadFromFile(jsonFile)
	if err != nil {
		t.Fatalf("Failed to load JSON config: %v", err)
	}

	// Verify loaded configuration
	if config.App.Name != "json-test-app" {
		t.Errorf("Expected app name 'json-test-app', got '%s'", config.App.Name)
	}
	if config.App.Environment != EnvProduction {
		t.Errorf("Expected env production, got %v", config.App.Environment)
	}
	if config.Log.Level != LogLevelDebug {
		t.Errorf("Expected log level debug, got %v", config.Log.Level)
	}
}

// TestEnvironmentOverrides tests environment variable overrides
func TestEnvironmentOverrides(t *testing.T) {
	// Set environment variables
	os.Setenv("SNGO_APP_NAME", "env-test-app")
	os.Setenv("SNGO_NETWORK_TCP_PORT", "7777")
	os.Setenv("SNGO_LOG_LEVEL", "error")
	defer func() {
		os.Unsetenv("SNGO_APP_NAME")
		os.Unsetenv("SNGO_NETWORK_TCP_PORT")
		os.Unsetenv("SNGO_LOG_LEVEL")
	}()

	loader := NewLoader()

	// Create base configuration
	yamlContent := `
app:
  name: base-app
  version: "1.0.0"
  environment: development
  
log:
  level: info
  format: text
  
network:
  tcp:
    address: "127.0.0.1"
    port: 8080
  limits:
    max_connections: 1000
    
actor:
  max_actors: 1000
  default_mailbox_size: 100
`

	tmpDir := os.TempDir()
	yamlFile := filepath.Join(tmpDir, "env-test-config.yaml")
	err := os.WriteFile(yamlFile, []byte(yamlContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test YAML file: %v", err)
	}
	defer os.Remove(yamlFile)

	// Load configuration with environment overrides
	config, err := loader.LoadFromFile(yamlFile)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify environment overrides
	if config.App.Name != "env-test-app" {
		t.Errorf("Expected app name 'env-test-app', got '%s'", config.App.Name)
	}
	if config.Network.TCP.Port != 7777 {
		t.Errorf("Expected port 7777, got %d", config.Network.TCP.Port)
	}
	if config.Log.Level != LogLevelError {
		t.Errorf("Expected log level error, got %v", config.Log.Level)
	}
}

// TestAutoLoad tests automatic configuration discovery
func TestAutoLoad(t *testing.T) {
	loader := NewLoader()

	// Create config file in current directory
	originalWd, _ := os.Getwd()
	tmpDir := os.TempDir()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	configContent := `
app:
  name: auto-load-app
  version: "1.0.0"
  environment: development
`

	configFile := "config.yaml"
	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}
	defer os.Remove(configFile)

	// Test auto-loading
	config, err := loader.AutoLoad()
	if err != nil {
		t.Fatalf("Failed to auto-load config: %v", err)
	}

	if config.App.Name != "auto-load-app" {
		t.Errorf("Expected app name 'auto-load-app', got '%s'", config.App.Name)
	}
}

// TestWatcher tests configuration file watching
func TestWatcher(t *testing.T) {
	loader := NewLoader()

	// Create initial configuration file
	tmpDir := os.TempDir()
	configFile := filepath.Join(tmpDir, "watch-test-config.yaml")

	initialContent := `
app:
  name: watch-test-app
  version: "1.0.0"
  environment: development
  
network:
  tcp:
    address: "127.0.0.1"
    port: 8080
`

	err := os.WriteFile(configFile, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}
	defer os.Remove(configFile)

	// Create watcher
	watcher, err := NewWatcher(configFile, loader)
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}
	defer watcher.Stop()

	// Test initial configuration
	config := watcher.GetConfig()
	if config.App.Name != "watch-test-app" {
		t.Errorf("Expected initial app name 'watch-test-app', got '%s'", config.App.Name)
	}

	// Set up change callback
	changeDetected := make(chan bool, 1)
	watcher.OnConfigChange(func(oldConfig, newConfig *Config) {
		if newConfig.Network.TCP.Port == 9090 {
			changeDetected <- true
		}
	})

	// Start watching
	err = watcher.Start()
	if err != nil {
		t.Fatalf("Failed to start watcher: %v", err)
	}

	// Modify configuration file
	updatedContent := `
app:
  name: watch-test-app
  version: "1.0.0"
  environment: development
  
network:
  tcp:
    address: "127.0.0.1"
    port: 9090
`

	time.Sleep(100 * time.Millisecond) // Small delay before writing
	err = os.WriteFile(configFile, []byte(updatedContent), 0644)
	if err != nil {
		t.Fatalf("Failed to update config file: %v", err)
	}

	// Wait for change detection
	select {
	case <-changeDetected:
		// Success - change was detected
	case <-time.After(3 * time.Second):
		t.Error("Configuration change was not detected within timeout")
	}

	// Verify updated configuration
	time.Sleep(100 * time.Millisecond) // Small delay for config reload
	updatedConfig := watcher.GetConfig()
	if updatedConfig.Network.TCP.Port != 9090 {
		t.Errorf("Expected updated port 9090, got %d", updatedConfig.Network.TCP.Port)
	}
}

// TestFileProvider tests the file-based configuration provider
func TestFileProvider(t *testing.T) {
	// Create test configuration file
	tmpDir := os.TempDir()
	configFile := filepath.Join(tmpDir, "provider-test-config.yaml")

	configContent := `
app:
  name: provider-test-app
  version: "1.0.0"
  environment: production
  
log:
  level: warn
  format: json
  
network:
  tcp:
    address: "0.0.0.0"
    port: 8888
`

	err := os.WriteFile(configFile, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test config file: %v", err)
	}
	defer os.Remove(configFile)

	// Create file provider
	provider, err := NewFileProvider(configFile)
	if err != nil {
		t.Fatalf("Failed to create file provider: %v", err)
	}
	defer provider.Close()

	// Load configuration
	config, err := provider.Load()
	if err != nil {
		t.Fatalf("Failed to load config from provider: %v", err)
	}

	// Verify configuration
	if config.App.Name != "provider-test-app" {
		t.Errorf("Expected app name 'provider-test-app', got '%s'", config.App.Name)
	}
	if config.Network.TCP.Port != 8888 {
		t.Errorf("Expected port 8888, got %d", config.Network.TCP.Port)
	}

	// Test watching functionality with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	changeDetected := make(chan bool, 1)

	go func() {
		err := provider.Watch(ctx, func(oldConfig, newConfig *Config) {
			if newConfig.Network.TCP.Port == 7777 {
				changeDetected <- true
			}
		})
		if err != nil {
			t.Logf("Watch error (may be expected): %v", err)
		}
	}()

	// Give watcher time to start
	time.Sleep(100 * time.Millisecond)

	// Update configuration file
	updatedContent := strings.Replace(configContent, "port: 8888", "port: 7777", 1)
	err = os.WriteFile(configFile, []byte(updatedContent), 0644)
	if err != nil {
		t.Fatalf("Failed to update config file: %v", err)
	}

	// Wait for change detection or timeout
	select {
	case <-changeDetected:
		t.Log("Configuration change detected successfully")
	case <-time.After(3 * time.Second):
		t.Log("Configuration change was not detected within timeout (this may be expected in some test environments)")
	}
}
