package plugins

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"plugin"
	"runtime"
	"strings"
	"sync"
	"time"
)

// PluginLoader handles dynamic loading and management of plugins
type PluginLoader struct {
	manager       *Manager
	pluginDirs    []string
	loadedPlugins map[string]*LoadedPlugin
	watchers      map[string]*PluginWatcher
	mu            sync.RWMutex
	logger        Logger
	config        *LoaderConfig
}

// LoaderConfig configures the plugin loader
type LoaderConfig struct {
	PluginDirectories   []string      `json:"plugin_directories"`
	AutoReload          bool          `json:"auto_reload"`
	ReloadInterval      time.Duration `json:"reload_interval"`
	AllowedExtensions   []string      `json:"allowed_extensions"`
	RequiredSymbols     []string      `json:"required_symbols"`
	MaxPlugins          int           `json:"max_plugins"`
	LoadTimeout         time.Duration `json:"load_timeout"`
	SandboxMode         bool          `json:"sandbox_mode"`
	VerifySignatures    bool          `json:"verify_signatures"`
}

// DefaultLoaderConfig returns default loader configuration
func DefaultLoaderConfig() *LoaderConfig {
	return &LoaderConfig{
		PluginDirectories: []string{"./plugins", "~/.s3ry/plugins", "/opt/s3ry/plugins"},
		AutoReload:        true,
		ReloadInterval:    30 * time.Second,
		AllowedExtensions: []string{".so", ".dylib", ".dll"},
		RequiredSymbols:   []string{"NewPlugin", "GetMetadata"},
		MaxPlugins:        50,
		LoadTimeout:       10 * time.Second,
		SandboxMode:       true,
		VerifySignatures:  false, // Disabled by default for development
	}
}

// LoadedPlugin represents a dynamically loaded plugin
type LoadedPlugin struct {
	Plugin      S3Plugin
	FilePath    string
	LoadTime    time.Time
	LastModified time.Time
	Metadata    PluginMetadata
	Handle      *plugin.Plugin
	Symbols     map[string]plugin.Symbol
}

// PluginWatcher monitors plugin files for changes
type PluginWatcher struct {
	FilePath     string
	LastModified time.Time
	IsActive     bool
}

// NewPluginLoader creates a new plugin loader
func NewPluginLoader(manager *Manager, config *LoaderConfig, logger Logger) *PluginLoader {
	if config == nil {
		config = DefaultLoaderConfig()
	}

	return &PluginLoader{
		manager:       manager,
		pluginDirs:    config.PluginDirectories,
		loadedPlugins: make(map[string]*LoadedPlugin),
		watchers:      make(map[string]*PluginWatcher),
		logger:        logger,
		config:        config,
	}
}

// LoadPlugins discovers and loads all plugins from configured directories
func (pl *PluginLoader) LoadPlugins(ctx context.Context) error {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	loadedCount := 0
	for _, dir := range pl.pluginDirs {
		count, err := pl.loadPluginsFromDirectory(ctx, dir)
		if err != nil {
			pl.logger.Warn("Failed to load plugins from directory %s: %v", dir, err)
			continue
		}
		loadedCount += count
	}

	pl.logger.Info("Loaded %d plugins from %d directories", loadedCount, len(pl.pluginDirs))

	// Start auto-reload if enabled
	if pl.config.AutoReload {
		go pl.startAutoReload(ctx)
	}

	return nil
}

// loadPluginsFromDirectory loads all plugins from a specific directory
func (pl *PluginLoader) loadPluginsFromDirectory(ctx context.Context, dir string) (int, error) {
	// Expand home directory
	if strings.HasPrefix(dir, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return 0, fmt.Errorf("failed to get home directory: %w", err)
		}
		dir = filepath.Join(homeDir, dir[2:])
	}

	// Check if directory exists
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		pl.logger.Debug("Plugin directory does not exist: %s", dir)
		return 0, nil
	}

	loadedCount := 0
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		// Check file extension
		if !pl.isValidPluginFile(path) {
			return nil
		}

		// Load plugin with timeout
		loadCtx, cancel := context.WithTimeout(ctx, pl.config.LoadTimeout)
		defer cancel()

		err = pl.loadPluginFile(loadCtx, path)
		if err != nil {
			pl.logger.Error("Failed to load plugin %s: %v", path, err)
			return nil // Continue loading other plugins
		}

		loadedCount++
		return nil
	})

	return loadedCount, err
}

// loadPluginFile loads a single plugin file
func (pl *PluginLoader) loadPluginFile(ctx context.Context, filePath string) error {
	// Check if plugin is already loaded
	if existing, exists := pl.loadedPlugins[filePath]; exists {
		// Check if file has been modified
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			return fmt.Errorf("failed to stat plugin file: %w", err)
		}

		if !fileInfo.ModTime().After(existing.LastModified) {
			pl.logger.Debug("Plugin %s already loaded and up to date", filePath)
			return nil
		}

		// Unload existing plugin
		pl.unloadPlugin(filePath)
	}

	// Verify plugin file
	if err := pl.verifyPluginFile(filePath); err != nil {
		return fmt.Errorf("plugin verification failed: %w", err)
	}

	// Load plugin
	p, err := plugin.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open plugin: %w", err)
	}

	// Load required symbols
	symbols := make(map[string]plugin.Symbol)
	for _, symbolName := range pl.config.RequiredSymbols {
		symbol, err := p.Lookup(symbolName)
		if err != nil {
			return fmt.Errorf("required symbol %s not found: %w", symbolName, err)
		}
		symbols[symbolName] = symbol
	}

	// Create plugin instance
	newPluginFunc, ok := symbols["NewPlugin"].(func() S3Plugin)
	if !ok {
		return fmt.Errorf("NewPlugin symbol has incorrect type")
	}

	pluginInstance := newPluginFunc()
	if pluginInstance == nil {
		return fmt.Errorf("NewPlugin returned nil")
	}

	// Get plugin metadata
	metadata := pluginInstance.Metadata()

	// Check plugin limits
	if pl.config.MaxPlugins > 0 && len(pl.loadedPlugins) >= pl.config.MaxPlugins {
		return fmt.Errorf("maximum number of plugins (%d) reached", pl.config.MaxPlugins)
	}

	// Get file info
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("failed to stat plugin file: %w", err)
	}

	// Create loaded plugin record
	loadedPlugin := &LoadedPlugin{
		Plugin:       pluginInstance,
		FilePath:     filePath,
		LoadTime:     time.Now(),
		LastModified: fileInfo.ModTime(),
		Metadata:     metadata,
		Handle:       p,
		Symbols:      symbols,
	}

	// Register with plugin manager
	err = pl.manager.RegisterPlugin(pluginInstance)
	if err != nil {
		return fmt.Errorf("failed to register plugin: %w", err)
	}

	// Store loaded plugin
	pl.loadedPlugins[filePath] = loadedPlugin

	// Set up file watcher
	pl.watchers[filePath] = &PluginWatcher{
		FilePath:     filePath,
		LastModified: fileInfo.ModTime(),
		IsActive:     true,
	}

	pl.logger.Info("Loaded plugin: %s v%s from %s", metadata.Name, metadata.Version, filePath)
	return nil
}

// unloadPlugin unloads a plugin
func (pl *PluginLoader) unloadPlugin(filePath string) error {
	loadedPlugin, exists := pl.loadedPlugins[filePath]
	if !exists {
		return fmt.Errorf("plugin not loaded: %s", filePath)
	}

	// Unregister from manager
	err := pl.manager.UnregisterPlugin(loadedPlugin.Metadata.Name)
	if err != nil {
		pl.logger.Warn("Failed to unregister plugin %s: %v", loadedPlugin.Metadata.Name, err)
	}

	// Remove from loaded plugins
	delete(pl.loadedPlugins, filePath)

	// Stop watcher
	if watcher, exists := pl.watchers[filePath]; exists {
		watcher.IsActive = false
		delete(pl.watchers, filePath)
	}

	pl.logger.Info("Unloaded plugin: %s", loadedPlugin.Metadata.Name)
	return nil
}

// isValidPluginFile checks if a file is a valid plugin file
func (pl *PluginLoader) isValidPluginFile(filePath string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	
	for _, allowedExt := range pl.config.AllowedExtensions {
		if ext == allowedExt {
			return true
		}
	}
	
	return false
}

// verifyPluginFile performs security verification on a plugin file
func (pl *PluginLoader) verifyPluginFile(filePath string) error {
	// Check file permissions
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	// On Unix systems, check that the file is not world-writable
	if runtime.GOOS != "windows" {
		mode := fileInfo.Mode()
		if mode.Perm()&0002 != 0 {
			return fmt.Errorf("plugin file is world-writable, which is a security risk")
		}
	}

	// Check file size (prevent loading extremely large files)
	maxSize := int64(100 * 1024 * 1024) // 100MB limit
	if fileInfo.Size() > maxSize {
		return fmt.Errorf("plugin file too large: %d bytes (max: %d)", fileInfo.Size(), maxSize)
	}

	// Signature verification (if enabled)
	if pl.config.VerifySignatures {
		if err := pl.verifyPluginSignature(filePath); err != nil {
			return fmt.Errorf("signature verification failed: %w", err)
		}
	}

	return nil
}

// verifyPluginSignature verifies the digital signature of a plugin (placeholder implementation)
func (pl *PluginLoader) verifyPluginSignature(filePath string) error {
	// Placeholder for signature verification
	// In a real implementation, this would:
	// 1. Check for a corresponding .sig file
	// 2. Verify the signature against a trusted public key
	// 3. Ensure the plugin hasn't been tampered with
	
	pl.logger.Debug("Signature verification not implemented for %s", filePath)
	return nil
}

// startAutoReload starts the auto-reload mechanism
func (pl *PluginLoader) startAutoReload(ctx context.Context) {
	ticker := time.NewTicker(pl.config.ReloadInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pl.checkForPluginUpdates(ctx)
		}
	}
}

// checkForPluginUpdates checks for plugin file changes and reloads if necessary
func (pl *PluginLoader) checkForPluginUpdates(ctx context.Context) {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	for filePath, watcher := range pl.watchers {
		if !watcher.IsActive {
			continue
		}

		fileInfo, err := os.Stat(filePath)
		if err != nil {
			if os.IsNotExist(err) {
				// Plugin file was deleted
				pl.logger.Info("Plugin file deleted, unloading: %s", filePath)
				pl.unloadPlugin(filePath)
			}
			continue
		}

		if fileInfo.ModTime().After(watcher.LastModified) {
			pl.logger.Info("Plugin file changed, reloading: %s", filePath)
			
			// Reload plugin
			err := pl.loadPluginFile(ctx, filePath)
			if err != nil {
				pl.logger.Error("Failed to reload plugin %s: %v", filePath, err)
			} else {
				watcher.LastModified = fileInfo.ModTime()
			}
		}
	}
}

// GetLoadedPlugins returns information about currently loaded plugins
func (pl *PluginLoader) GetLoadedPlugins() map[string]*LoadedPlugin {
	pl.mu.RLock()
	defer pl.mu.RUnlock()

	result := make(map[string]*LoadedPlugin)
	for path, plugin := range pl.loadedPlugins {
		result[path] = plugin
	}
	return result
}

// ReloadPlugin manually reloads a specific plugin
func (pl *PluginLoader) ReloadPlugin(ctx context.Context, filePath string) error {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	return pl.loadPluginFile(ctx, filePath)
}

// UnloadAllPlugins unloads all currently loaded plugins
func (pl *PluginLoader) UnloadAllPlugins() error {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	var errors []error
	for filePath := range pl.loadedPlugins {
		if err := pl.unloadPlugin(filePath); err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors during unload: %v", errors)
	}

	return nil
}

// GetPluginStatistics returns statistics about plugin loading
func (pl *PluginLoader) GetPluginStatistics() *PluginStatistics {
	pl.mu.RLock()
	defer pl.mu.RUnlock()

	stats := &PluginStatistics{
		TotalPlugins:    len(pl.loadedPlugins),
		PluginDirs:      len(pl.pluginDirs),
		AutoReloadEnabled: pl.config.AutoReload,
		LoadedPlugins:   make([]PluginInfo, 0, len(pl.loadedPlugins)),
	}

	for _, plugin := range pl.loadedPlugins {
		info := PluginInfo{
			Name:         plugin.Metadata.Name,
			Version:      plugin.Metadata.Version,
			FilePath:     plugin.FilePath,
			LoadTime:     plugin.LoadTime,
			LastModified: plugin.LastModified,
		}
		stats.LoadedPlugins = append(stats.LoadedPlugins, info)
	}

	return stats
}

// AddPluginDirectory adds a new directory to watch for plugins
func (pl *PluginLoader) AddPluginDirectory(ctx context.Context, dir string) error {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	// Check if directory already exists in the list
	for _, existingDir := range pl.pluginDirs {
		if existingDir == dir {
			return fmt.Errorf("directory already being watched: %s", dir)
		}
	}

	// Add directory
	pl.pluginDirs = append(pl.pluginDirs, dir)

	// Load plugins from the new directory
	_, err := pl.loadPluginsFromDirectory(ctx, dir)
	return err
}

// RemovePluginDirectory removes a directory from the watch list and unloads its plugins
func (pl *PluginLoader) RemovePluginDirectory(dir string) error {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	// Remove directory from list
	newDirs := make([]string, 0, len(pl.pluginDirs))
	found := false
	for _, existingDir := range pl.pluginDirs {
		if existingDir != dir {
			newDirs = append(newDirs, existingDir)
		} else {
			found = true
		}
	}

	if !found {
		return fmt.Errorf("directory not found: %s", dir)
	}

	pl.pluginDirs = newDirs

	// Unload plugins from this directory
	var errors []error
	for filePath := range pl.loadedPlugins {
		if strings.HasPrefix(filePath, dir) {
			if err := pl.unloadPlugin(filePath); err != nil {
				errors = append(errors, err)
			}
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors unloading plugins from directory: %v", errors)
	}

	return nil
}

// Supporting types

// PluginStatistics contains statistics about loaded plugins
type PluginStatistics struct {
	TotalPlugins      int          `json:"total_plugins"`
	PluginDirs        int          `json:"plugin_dirs"`
	AutoReloadEnabled bool         `json:"auto_reload_enabled"`
	LoadedPlugins     []PluginInfo `json:"loaded_plugins"`
}

// PluginInfo contains basic information about a loaded plugin
type PluginInfo struct {
	Name         string    `json:"name"`
	Version      string    `json:"version"`
	FilePath     string    `json:"file_path"`
	LoadTime     time.Time `json:"load_time"`
	LastModified time.Time `json:"last_modified"`
}

// SetAutoReload enables or disables auto-reload
func (pl *PluginLoader) SetAutoReload(ctx context.Context, enabled bool) {
	pl.mu.Lock()
	defer pl.mu.Unlock()

	pl.config.AutoReload = enabled
	
	if enabled {
		go pl.startAutoReload(ctx)
	}
}

// ValidatePluginInterface checks if a plugin properly implements required interfaces
func (pl *PluginLoader) ValidatePluginInterface(plugin S3Plugin) error {
	// Check required methods
	metadata := plugin.Metadata()
	if metadata.Name == "" {
		return fmt.Errorf("plugin name cannot be empty")
	}
	if metadata.Version == "" {
		return fmt.Errorf("plugin version cannot be empty")
	}

	// Check supported operations
	operations := plugin.SupportedOperations()
	if len(operations) == 0 {
		return fmt.Errorf("plugin must support at least one operation")
	}

	// Test initialization
	if err := plugin.Initialize(nil); err != nil {
		return fmt.Errorf("plugin initialization failed: %w", err)
	}

	// Test cleanup
	if err := plugin.Cleanup(); err != nil {
		return fmt.Errorf("plugin cleanup failed: %w", err)
	}

	return nil
}