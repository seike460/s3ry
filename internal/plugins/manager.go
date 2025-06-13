package plugins

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// Manager handles plugin lifecycle and execution
type Manager struct {
	plugins         map[string]S3Plugin
	pluginsByOp     map[S3Operation][]S3Plugin
	preProcessors   []PreProcessor
	postProcessors  []PostProcessor
	streamProcessors []StreamProcessor
	batchProcessors []BatchProcessor
	selectProcessors []SelectProcessor
	mu              sync.RWMutex
	logger          Logger
	config          *ManagerConfig
}

// ManagerConfig contains configuration for the plugin manager
type ManagerConfig struct {
	MaxConcurrentPlugins int           `json:"max_concurrent_plugins"`
	PluginTimeout        time.Duration `json:"plugin_timeout"`
	EnablePreProcessors  bool          `json:"enable_pre_processors"`
	EnablePostProcessors bool          `json:"enable_post_processors"`
	PluginDirectory      string        `json:"plugin_directory"`
	AllowedPlugins       []string      `json:"allowed_plugins,omitempty"`
	BlockedPlugins       []string      `json:"blocked_plugins,omitempty"`
}

// DefaultManagerConfig returns a default configuration
func DefaultManagerConfig() *ManagerConfig {
	return &ManagerConfig{
		MaxConcurrentPlugins: 10,
		PluginTimeout:        30 * time.Second,
		EnablePreProcessors:  true,
		EnablePostProcessors: true,
		PluginDirectory:      "./plugins",
		AllowedPlugins:       nil, // nil means all plugins allowed
		BlockedPlugins:       []string{},
	}
}

// NewManager creates a new plugin manager
func NewManager(config *ManagerConfig, logger Logger) *Manager {
	if config == nil {
		config = DefaultManagerConfig()
	}

	return &Manager{
		plugins:          make(map[string]S3Plugin),
		pluginsByOp:      make(map[S3Operation][]S3Plugin),
		preProcessors:    make([]PreProcessor, 0),
		postProcessors:   make([]PostProcessor, 0),
		streamProcessors: make([]StreamProcessor, 0),
		batchProcessors:  make([]BatchProcessor, 0),
		selectProcessors: make([]SelectProcessor, 0),
		logger:           logger,
		config:           config,
	}
}

// RegisterPlugin registers a plugin with the manager
func (m *Manager) RegisterPlugin(plugin S3Plugin) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	metadata := plugin.Metadata()
	
	// Check if plugin is blocked
	for _, blocked := range m.config.BlockedPlugins {
		if blocked == metadata.Name {
			return fmt.Errorf("plugin %s is blocked", metadata.Name)
		}
	}

	// Check if plugin is allowed (if allowlist is defined)
	if len(m.config.AllowedPlugins) > 0 {
		allowed := false
		for _, allowedPlugin := range m.config.AllowedPlugins {
			if allowedPlugin == metadata.Name {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("plugin %s is not in the allowed list", metadata.Name)
		}
	}

	// Initialize the plugin
	if err := plugin.Initialize(nil); err != nil {
		return fmt.Errorf("failed to initialize plugin %s: %w", metadata.Name, err)
	}

	// Register the plugin
	m.plugins[metadata.Name] = plugin

	// Register by supported operations
	for _, op := range plugin.SupportedOperations() {
		m.pluginsByOp[op] = append(m.pluginsByOp[op], plugin)
	}

	// Register specialized interfaces
	if pp, ok := plugin.(PreProcessor); ok {
		m.preProcessors = append(m.preProcessors, pp)
	}
	if pp, ok := plugin.(PostProcessor); ok {
		m.postProcessors = append(m.postProcessors, pp)
	}
	if sp, ok := plugin.(StreamProcessor); ok {
		m.streamProcessors = append(m.streamProcessors, sp)
	}
	if bp, ok := plugin.(BatchProcessor); ok {
		m.batchProcessors = append(m.batchProcessors, bp)
	}
	if sp, ok := plugin.(SelectProcessor); ok {
		m.selectProcessors = append(m.selectProcessors, sp)
	}

	// Sort plugins by priority for each operation
	for _, op := range plugin.SupportedOperations() {
		m.sortPluginsByPriority(op)
	}

	m.logger.Info("Plugin registered: %s v%s", metadata.Name, metadata.Version)
	return nil
}

// UnregisterPlugin removes a plugin from the manager
func (m *Manager) UnregisterPlugin(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	plugin, exists := m.plugins[name]
	if !exists {
		return fmt.Errorf("plugin %s not found", name)
	}

	// Cleanup plugin
	if err := plugin.Cleanup(); err != nil {
		m.logger.Warn("Error cleaning up plugin %s: %v", name, err)
	}

	// Remove from main registry
	delete(m.plugins, name)

	// Remove from operation mappings
	for op, plugins := range m.pluginsByOp {
		filtered := make([]S3Plugin, 0, len(plugins))
		for _, p := range plugins {
			if p.Metadata().Name != name {
				filtered = append(filtered, p)
			}
		}
		m.pluginsByOp[op] = filtered
	}

	// Remove from specialized interfaces
	m.removeFromPreProcessors(name)
	m.removeFromPostProcessors(name)
	m.removeFromStreamProcessors(name)
	m.removeFromBatchProcessors(name)
	m.removeFromSelectProcessors(name)

	m.logger.Info("Plugin unregistered: %s", name)
	return nil
}

// ExecuteOperation executes plugins for a specific operation
func (m *Manager) ExecuteOperation(ctx context.Context, operation S3Operation, opCtx OperationContext, args map[string]interface{}) (*OperationResult, error) {
	m.mu.RLock()
	plugins := m.pluginsByOp[operation]
	m.mu.RUnlock()

	if len(plugins) == 0 {
		return &OperationResult{
			Success: true,
			Message: "No plugins registered for operation",
		}, nil
	}

	// Pre-process
	if m.config.EnablePreProcessors {
		var err error
		args, err = m.executePreProcessors(opCtx, args)
		if err != nil {
			return nil, fmt.Errorf("pre-processing failed: %w", err)
		}
	}

	// Execute plugins
	var lastResult *OperationResult
	for _, plugin := range plugins {
		// Check if conditional plugin should execute
		if cp, ok := plugin.(ConditionalPlugin); ok {
			if !cp.ShouldExecute(opCtx, args) {
				continue
			}
		}

		// Execute with timeout
		timeoutCtx, cancel := context.WithTimeout(ctx, m.config.PluginTimeout)
		pluginCtx := opCtx
		pluginCtx.Context = timeoutCtx

		result, err := plugin.Execute(pluginCtx, args)
		cancel()

		if err != nil {
			m.logger.Error("Plugin %s failed: %v", plugin.Metadata().Name, err)
			continue
		}

		lastResult = result
		m.logger.Debug("Plugin %s executed successfully", plugin.Metadata().Name)
	}

	// Post-process
	if m.config.EnablePostProcessors && lastResult != nil {
		var err error
		lastResult, err = m.executePostProcessors(opCtx, lastResult)
		if err != nil {
			return nil, fmt.Errorf("post-processing failed: %w", err)
		}
	}

	if lastResult == nil {
		return &OperationResult{
			Success: true,
			Message: "No plugins executed",
		}, nil
	}

	return lastResult, nil
}

// GetPlugins returns all registered plugins
func (m *Manager) GetPlugins() map[string]S3Plugin {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]S3Plugin)
	for name, plugin := range m.plugins {
		result[name] = plugin
	}
	return result
}

// GetPluginsByOperation returns plugins for a specific operation
func (m *Manager) GetPluginsByOperation(operation S3Operation) []S3Plugin {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plugins := make([]S3Plugin, len(m.pluginsByOp[operation]))
	copy(plugins, m.pluginsByOp[operation])
	return plugins
}

// GetBatchProcessors returns registered batch processors
func (m *Manager) GetBatchProcessors() []BatchProcessor {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]BatchProcessor, len(m.batchProcessors))
	copy(result, m.batchProcessors)
	return result
}

// GetSelectProcessors returns registered S3 Select processors
func (m *Manager) GetSelectProcessors() []SelectProcessor {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]SelectProcessor, len(m.selectProcessors))
	copy(result, m.selectProcessors)
	return result
}

// Shutdown gracefully shuts down all plugins
func (m *Manager) Shutdown() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errors []error
	for name, plugin := range m.plugins {
		if err := plugin.Cleanup(); err != nil {
			errors = append(errors, fmt.Errorf("failed to cleanup plugin %s: %w", name, err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors during shutdown: %v", errors)
	}

	m.logger.Info("Plugin manager shutdown complete")
	return nil
}

// Helper methods

func (m *Manager) sortPluginsByPriority(operation S3Operation) {
	plugins := m.pluginsByOp[operation]
	sort.Slice(plugins, func(i, j int) bool {
		priA := PriorityMedium
		priB := PriorityMedium

		if pp, ok := plugins[i].(PrioritizedPlugin); ok {
			priA = pp.Priority()
		}
		if pp, ok := plugins[j].(PrioritizedPlugin); ok {
			priB = pp.Priority()
		}

		return priA > priB // Higher priority first
	})
	m.pluginsByOp[operation] = plugins
}

func (m *Manager) executePreProcessors(ctx OperationContext, args map[string]interface{}) (map[string]interface{}, error) {
	for _, processor := range m.preProcessors {
		var err error
		args, err = processor.PreProcess(ctx, args)
		if err != nil {
			return nil, err
		}
	}
	return args, nil
}

func (m *Manager) executePostProcessors(ctx OperationContext, result *OperationResult) (*OperationResult, error) {
	for _, processor := range m.postProcessors {
		var err error
		result, err = processor.PostProcess(ctx, result)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (m *Manager) removeFromPreProcessors(name string) {
	filtered := make([]PreProcessor, 0, len(m.preProcessors))
	for _, pp := range m.preProcessors {
		if pp.Metadata().Name != name {
			filtered = append(filtered, pp)
		}
	}
	m.preProcessors = filtered
}

func (m *Manager) removeFromPostProcessors(name string) {
	filtered := make([]PostProcessor, 0, len(m.postProcessors))
	for _, pp := range m.postProcessors {
		if pp.Metadata().Name != name {
			filtered = append(filtered, pp)
		}
	}
	m.postProcessors = filtered
}

func (m *Manager) removeFromStreamProcessors(name string) {
	filtered := make([]StreamProcessor, 0, len(m.streamProcessors))
	for _, sp := range m.streamProcessors {
		if sp.Metadata().Name != name {
			filtered = append(filtered, sp)
		}
	}
	m.streamProcessors = filtered
}

func (m *Manager) removeFromBatchProcessors(name string) {
	filtered := make([]BatchProcessor, 0, len(m.batchProcessors))
	for _, bp := range m.batchProcessors {
		if bp.Metadata().Name != name {
			filtered = append(filtered, bp)
		}
	}
	m.batchProcessors = filtered
}

func (m *Manager) removeFromSelectProcessors(name string) {
	filtered := make([]SelectProcessor, 0, len(m.selectProcessors))
	for _, sp := range m.selectProcessors {
		if sp.Metadata().Name != name {
			filtered = append(filtered, sp)
		}
	}
	m.selectProcessors = filtered
}