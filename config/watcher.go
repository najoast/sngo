// Package config provides configuration watching and hot-reload functionality
package config

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watcher watches configuration files for changes and provides hot-reload functionality
type Watcher struct {
	// Configuration file path
	configFile string

	// Configuration format
	format ConfigFormat

	// Configuration loader
	loader *Loader

	// Current configuration
	config   *Config
	configMu sync.RWMutex

	// File system watcher
	fsWatcher *fsnotify.Watcher

	// Event callbacks
	callbacks   []ConfigChangeCallback
	callbacksMu sync.RWMutex

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc

	// Wait group for goroutines
	wg sync.WaitGroup
}

// ConfigChangeCallback is called when configuration changes
type ConfigChangeCallback func(oldConfig, newConfig *Config)

// NewWatcher creates a new configuration watcher
func NewWatcher(configFile string, loader *Loader) (*Watcher, error) {
	// Determine format
	ext := filepath.Ext(configFile)
	var format ConfigFormat
	switch ext {
	case ".yaml", ".yml":
		format = FormatYAML
	case ".json":
		format = FormatJSON
	default:
		return nil, fmt.Errorf("unsupported config file format: %s", ext)
	}

	// Create file system watcher
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create file system watcher: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	watcher := &Watcher{
		configFile: configFile,
		format:     format,
		loader:     loader,
		fsWatcher:  fsWatcher,
		ctx:        ctx,
		cancel:     cancel,
	}

	// Load initial configuration
	config, err := loader.LoadFromFile(configFile)
	if err != nil {
		fsWatcher.Close()
		cancel()
		return nil, fmt.Errorf("failed to load initial config: %w", err)
	}
	watcher.config = config

	return watcher, nil
}

// Start starts watching the configuration file
func (w *Watcher) Start() error {
	// Add file to watcher
	err := w.fsWatcher.Add(w.configFile)
	if err != nil {
		return fmt.Errorf("failed to watch config file: %w", err)
	}

	// Start watching goroutine
	w.wg.Add(1)
	go w.watchLoop()

	return nil
}

// Stop stops watching the configuration file
func (w *Watcher) Stop() error {
	// Cancel context
	w.cancel()

	// Close file system watcher
	err := w.fsWatcher.Close()

	// Wait for goroutines to finish
	w.wg.Wait()

	return err
}

// GetConfig returns the current configuration
func (w *Watcher) GetConfig() *Config {
	w.configMu.RLock()
	defer w.configMu.RUnlock()
	return w.config
}

// OnConfigChange registers a callback for configuration changes
func (w *Watcher) OnConfigChange(callback ConfigChangeCallback) {
	w.callbacksMu.Lock()
	defer w.callbacksMu.Unlock()
	w.callbacks = append(w.callbacks, callback)
}

// Reload manually reloads the configuration
func (w *Watcher) Reload() error {
	return w.reloadConfig()
}

// watchLoop watches for file system events
func (w *Watcher) watchLoop() {
	defer w.wg.Done()

	// Debounce timer to avoid multiple reloads for rapid file changes
	var debounceTimer *time.Timer
	debounceDuration := 500 * time.Millisecond

	for {
		select {
		case <-w.ctx.Done():
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			return

		case event, ok := <-w.fsWatcher.Events:
			if !ok {
				return
			}

			// Check if this is our config file
			if event.Name != w.configFile {
				continue
			}

			// Handle different event types
			if event.Op&fsnotify.Write == fsnotify.Write ||
				event.Op&fsnotify.Create == fsnotify.Create {

				// Reset debounce timer
				if debounceTimer != nil {
					debounceTimer.Stop()
				}

				debounceTimer = time.AfterFunc(debounceDuration, func() {
					err := w.reloadConfig()
					if err != nil {
						log.Printf("Failed to reload config: %v", err)
					}
				})

			} else if event.Op&fsnotify.Remove == fsnotify.Remove ||
				event.Op&fsnotify.Rename == fsnotify.Rename {

				log.Printf("Config file %s was removed or renamed", w.configFile)
				// Try to re-add the file in case it was recreated
				time.AfterFunc(1*time.Second, func() {
					w.fsWatcher.Add(w.configFile)
				})
			}

		case err, ok := <-w.fsWatcher.Errors:
			if !ok {
				return
			}
			log.Printf("Config watcher error: %v", err)
		}
	}
}

// reloadConfig reloads the configuration from file
func (w *Watcher) reloadConfig() error {
	// Load new configuration
	newConfig, err := w.loader.LoadFromFile(w.configFile)
	if err != nil {
		return fmt.Errorf("failed to reload config: %w", err)
	}

	// Get old configuration
	w.configMu.RLock()
	oldConfig := w.config
	w.configMu.RUnlock()

	// Update configuration
	w.configMu.Lock()
	w.config = newConfig
	w.configMu.Unlock()

	// Notify callbacks
	w.notifyCallbacks(oldConfig, newConfig)

	log.Printf("Configuration reloaded from %s", w.configFile)
	return nil
}

// notifyCallbacks notifies all registered callbacks of configuration changes
func (w *Watcher) notifyCallbacks(oldConfig, newConfig *Config) {
	w.callbacksMu.RLock()
	callbacks := make([]ConfigChangeCallback, len(w.callbacks))
	copy(callbacks, w.callbacks)
	w.callbacksMu.RUnlock()

	for _, callback := range callbacks {
		// Call callback in a separate goroutine to avoid blocking
		go func(cb ConfigChangeCallback) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Config change callback panicked: %v", r)
				}
			}()
			cb(oldConfig, newConfig)
		}(callback)
	}
}

// Provider represents a configuration provider interface
type Provider interface {
	// Load loads configuration from the provider
	Load() (*Config, error)

	// Watch watches for configuration changes
	Watch(ctx context.Context, callback ConfigChangeCallback) error

	// Close closes the provider
	Close() error
}

// FileProvider provides configuration from files
type FileProvider struct {
	loader  *Loader
	watcher *Watcher
}

// NewFileProvider creates a new file-based configuration provider
func NewFileProvider(configFile string) (*FileProvider, error) {
	loader := NewLoader()

	provider := &FileProvider{
		loader: loader,
	}

	// If config file is specified, create watcher
	if configFile != "" {
		watcher, err := NewWatcher(configFile, loader)
		if err != nil {
			return nil, fmt.Errorf("failed to create config watcher: %w", err)
		}
		provider.watcher = watcher
	}

	return provider, nil
}

// Load loads configuration
func (fp *FileProvider) Load() (*Config, error) {
	if fp.watcher != nil {
		return fp.watcher.GetConfig(), nil
	}
	return fp.loader.AutoLoad()
}

// Watch watches for configuration changes
func (fp *FileProvider) Watch(ctx context.Context, callback ConfigChangeCallback) error {
	if fp.watcher == nil {
		return fmt.Errorf("watcher not available")
	}

	fp.watcher.OnConfigChange(callback)

	if err := fp.watcher.Start(); err != nil {
		return fmt.Errorf("failed to start config watcher: %w", err)
	}

	// Wait for context cancellation
	go func() {
		<-ctx.Done()
		fp.watcher.Stop()
	}()

	return nil
}

// Close closes the provider
func (fp *FileProvider) Close() error {
	if fp.watcher != nil {
		return fp.watcher.Stop()
	}
	return nil
}
