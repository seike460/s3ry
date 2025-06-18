package components

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ViewType represents different application views
type ViewType int

const (
	WelcomeView ViewType = iota
	ListBucketsView
	ListObjectsView
	UploadView
	DownloadView
	ErrorView
	SettingsView
	HelpView
)

// KeyBinding represents a keyboard shortcut
type KeyBinding struct {
	Key         string
	Description string
	Action      string
	Context     ViewType
	Category    string
	Enabled     bool
	Global      bool // Available in all contexts
}

// KeyCategory groups related shortcuts
type KeyCategory struct {
	Name        string
	Description string
	Shortcuts   []KeyBinding
}

// HelpSystem manages contextual help display
type HelpSystem struct {
	shortcuts       map[ViewType][]KeyBinding
	globalShortcuts []KeyBinding
	categories      map[string]KeyCategory
	visible         bool
	currentView     ViewType

	// Styles for help display
	titleStyle    lipgloss.Style
	categoryStyle lipgloss.Style
	keyStyle      lipgloss.Style
	descStyle     lipgloss.Style
	footerStyle   lipgloss.Style
}

// KeyboardManager manages keyboard navigation and shortcuts
type KeyboardManager struct {
	shortcuts   map[string]KeyBinding
	contexts    map[ViewType][]KeyBinding
	help        *HelpSystem
	currentView ViewType
	enabled     bool
	customKeys  map[ViewType]map[string]KeyBinding

	// Event tracking
	keyPressCount map[string]int
	mutex         sync.RWMutex
}

// ShortcutExecutedMsg represents a shortcut execution event
type ShortcutExecutedMsg struct {
	Key     string
	Action  string
	Context ViewType
}

// ToggleHelpMsg toggles help display
type ToggleHelpMsg struct{}

// NewKeyboardManager creates a new keyboard navigation manager
func NewKeyboardManager() *KeyboardManager {
	help := &HelpSystem{
		shortcuts:       make(map[ViewType][]KeyBinding),
		globalShortcuts: make([]KeyBinding, 0),
		categories:      make(map[string]KeyCategory),
		visible:         false,

		titleStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			MarginBottom(1),

		categoryStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#04B575")).
			MarginBottom(1),

		keyStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1).
			MarginRight(1),

		descStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#CCCCCC")),

		footerStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262")).
			MarginTop(1),
	}

	km := &KeyboardManager{
		shortcuts:     make(map[string]KeyBinding),
		contexts:      make(map[ViewType][]KeyBinding),
		help:          help,
		enabled:       true,
		customKeys:    make(map[ViewType]map[string]KeyBinding),
		keyPressCount: make(map[string]int),
	}

	// Initialize default shortcuts
	km.initializeDefaultShortcuts()

	return km
}

// initializeDefaultShortcuts sets up the default keyboard shortcuts
func (km *KeyboardManager) initializeDefaultShortcuts() {
	// Global shortcuts (available in all views)
	globalShortcuts := []KeyBinding{
		{Key: "ctrl+c", Description: "Quit application", Action: "quit", Global: true, Category: "Application", Enabled: true},
		{Key: "q", Description: "Quit application", Action: "quit", Global: true, Category: "Application", Enabled: true},
		{Key: "?", Description: "Show/hide help", Action: "toggle_help", Global: true, Category: "Help", Enabled: true},
		{Key: "F1", Description: "Show help", Action: "show_help", Global: true, Category: "Help", Enabled: true},
		{Key: "ctrl+r", Description: "Refresh current view", Action: "refresh", Global: true, Category: "Navigation", Enabled: true},
		{Key: "r", Description: "Refresh", Action: "refresh", Global: true, Category: "Navigation", Enabled: true},
		{Key: "esc", Description: "Go back/Cancel", Action: "back", Global: true, Category: "Navigation", Enabled: true},
		{Key: "tab", Description: "Next field", Action: "next_field", Global: true, Category: "Navigation", Enabled: true},
		{Key: "shift+tab", Description: "Previous field", Action: "prev_field", Global: true, Category: "Navigation", Enabled: true},
		{Key: "ctrl+s", Description: "Settings", Action: "open_settings", Global: true, Category: "Application", Enabled: true},
	}

	for _, shortcut := range globalShortcuts {
		km.AddGlobalShortcut(shortcut)
	}

	// Welcome view shortcuts
	welcomeShortcuts := []KeyBinding{
		{Key: "enter", Description: "Continue/Next step", Action: "continue", Context: WelcomeView, Category: "Navigation", Enabled: true},
		{Key: "space", Description: "Continue/Next step", Action: "continue", Context: WelcomeView, Category: "Navigation", Enabled: true},
		{Key: "s", Description: "Open settings", Action: "open_settings", Context: WelcomeView, Category: "Setup", Enabled: true},
		{Key: "t", Description: "Toggle tutorial", Action: "toggle_tutorial", Context: WelcomeView, Category: "Help", Enabled: true},
		{Key: "up", Description: "Previous step", Action: "prev_step", Context: WelcomeView, Category: "Navigation", Enabled: true},
		{Key: "k", Description: "Previous step", Action: "prev_step", Context: WelcomeView, Category: "Navigation", Enabled: true},
		{Key: "down", Description: "Next step", Action: "next_step", Context: WelcomeView, Category: "Navigation", Enabled: true},
		{Key: "j", Description: "Next step", Action: "next_step", Context: WelcomeView, Category: "Navigation", Enabled: true},
	}

	for _, shortcut := range welcomeShortcuts {
		km.AddShortcut(shortcut)
	}

	// List buckets view shortcuts
	bucketShortcuts := []KeyBinding{
		{Key: "enter", Description: "Select bucket", Action: "select_bucket", Context: ListBucketsView, Category: "Selection", Enabled: true},
		{Key: "space", Description: "Select bucket", Action: "select_bucket", Context: ListBucketsView, Category: "Selection", Enabled: true},
		{Key: "up", Description: "Move up", Action: "move_up", Context: ListBucketsView, Category: "Navigation", Enabled: true},
		{Key: "k", Description: "Move up", Action: "move_up", Context: ListBucketsView, Category: "Navigation", Enabled: true},
		{Key: "down", Description: "Move down", Action: "move_down", Context: ListBucketsView, Category: "Navigation", Enabled: true},
		{Key: "j", Description: "Move down", Action: "move_down", Context: ListBucketsView, Category: "Navigation", Enabled: true},
		{Key: "home", Description: "Go to first", Action: "goto_first", Context: ListBucketsView, Category: "Navigation", Enabled: true},
		{Key: "end", Description: "Go to last", Action: "goto_last", Context: ListBucketsView, Category: "Navigation", Enabled: true},
		{Key: "page_up", Description: "Page up", Action: "page_up", Context: ListBucketsView, Category: "Navigation", Enabled: true},
		{Key: "page_down", Description: "Page down", Action: "page_down", Context: ListBucketsView, Category: "Navigation", Enabled: true},
		{Key: "ctrl+b", Description: "Page up", Action: "page_up", Context: ListBucketsView, Category: "Navigation", Enabled: true},
		{Key: "ctrl+f", Description: "Page down", Action: "page_down", Context: ListBucketsView, Category: "Navigation", Enabled: true},
		{Key: "n", Description: "New bucket", Action: "new_bucket", Context: ListBucketsView, Category: "Actions", Enabled: true},
		{Key: "d", Description: "Delete bucket", Action: "delete_bucket", Context: ListBucketsView, Category: "Actions", Enabled: true},
		{Key: "/", Description: "Search", Action: "search", Context: ListBucketsView, Category: "Search", Enabled: true},
		{Key: "ctrl+l", Description: "Filter by region", Action: "filter_region", Context: ListBucketsView, Category: "Search", Enabled: true},
	}

	for _, shortcut := range bucketShortcuts {
		km.AddShortcut(shortcut)
	}

	// List objects view shortcuts
	objectShortcuts := []KeyBinding{
		{Key: "enter", Description: "Select/Download object", Action: "select_object", Context: ListObjectsView, Category: "Selection", Enabled: true},
		{Key: "space", Description: "Toggle selection", Action: "toggle_selection", Context: ListObjectsView, Category: "Selection", Enabled: true},
		{Key: "up", Description: "Move up", Action: "move_up", Context: ListObjectsView, Category: "Navigation", Enabled: true},
		{Key: "k", Description: "Move up", Action: "move_up", Context: ListObjectsView, Category: "Navigation", Enabled: true},
		{Key: "down", Description: "Move down", Action: "move_down", Context: ListObjectsView, Category: "Navigation", Enabled: true},
		{Key: "j", Description: "Move down", Action: "move_down", Context: ListObjectsView, Category: "Navigation", Enabled: true},
		{Key: "u", Description: "Upload file", Action: "upload_file", Context: ListObjectsView, Category: "Actions", Enabled: true},
		{Key: "d", Description: "Download selected", Action: "download_selected", Context: ListObjectsView, Category: "Actions", Enabled: true},
		{Key: "delete", Description: "Delete selected", Action: "delete_selected", Context: ListObjectsView, Category: "Actions", Enabled: true},
		{Key: "x", Description: "Delete selected", Action: "delete_selected", Context: ListObjectsView, Category: "Actions", Enabled: true},
		{Key: "c", Description: "Copy objects", Action: "copy_objects", Context: ListObjectsView, Category: "Actions", Enabled: true},
		{Key: "m", Description: "Move objects", Action: "move_objects", Context: ListObjectsView, Category: "Actions", Enabled: true},
		{Key: "ctrl+a", Description: "Select all", Action: "select_all", Context: ListObjectsView, Category: "Selection", Enabled: true},
		{Key: "ctrl+d", Description: "Deselect all", Action: "deselect_all", Context: ListObjectsView, Category: "Selection", Enabled: true},
		{Key: "/", Description: "Search objects", Action: "search_objects", Context: ListObjectsView, Category: "Search", Enabled: true},
		{Key: "f", Description: "Filter by type", Action: "filter_type", Context: ListObjectsView, Category: "Search", Enabled: true},
		{Key: "ctrl+p", Description: "Preview object", Action: "preview_object", Context: ListObjectsView, Category: "View", Enabled: true},
		{Key: "p", Description: "Properties", Action: "show_properties", Context: ListObjectsView, Category: "View", Enabled: true},
	}

	for _, shortcut := range objectShortcuts {
		km.AddShortcut(shortcut)
	}

	// Error view shortcuts
	errorShortcuts := []KeyBinding{
		{Key: "enter", Description: "Execute recovery step", Action: "execute_recovery", Context: ErrorView, Category: "Recovery", Enabled: true},
		{Key: "r", Description: "Retry operation", Action: "retry_operation", Context: ErrorView, Category: "Recovery", Enabled: true},
		{Key: "h", Description: "Show help", Action: "show_error_help", Context: ErrorView, Category: "Help", Enabled: true},
		{Key: "t", Description: "Toggle technical details", Action: "toggle_technical", Context: ErrorView, Category: "View", Enabled: true},
		{Key: "up", Description: "Previous recovery step", Action: "prev_recovery", Context: ErrorView, Category: "Navigation", Enabled: true},
		{Key: "k", Description: "Previous recovery step", Action: "prev_recovery", Context: ErrorView, Category: "Navigation", Enabled: true},
		{Key: "down", Description: "Next recovery step", Action: "next_recovery", Context: ErrorView, Category: "Navigation", Enabled: true},
		{Key: "j", Description: "Next recovery step", Action: "next_recovery", Context: ErrorView, Category: "Navigation", Enabled: true},
		{Key: "c", Description: "Copy error details", Action: "copy_error", Context: ErrorView, Category: "Utility", Enabled: true},
		{Key: "s", Description: "Save error log", Action: "save_error_log", Context: ErrorView, Category: "Utility", Enabled: true},
	}

	for _, shortcut := range errorShortcuts {
		km.AddShortcut(shortcut)
	}

	// Settings view shortcuts
	settingsShortcuts := []KeyBinding{
		{Key: "enter", Description: "Edit setting", Action: "edit_setting", Context: SettingsView, Category: "Edit", Enabled: true},
		{Key: "space", Description: "Toggle setting", Action: "toggle_setting", Context: SettingsView, Category: "Edit", Enabled: true},
		{Key: "up", Description: "Previous setting", Action: "prev_setting", Context: SettingsView, Category: "Navigation", Enabled: true},
		{Key: "k", Description: "Previous setting", Action: "prev_setting", Context: SettingsView, Category: "Navigation", Enabled: true},
		{Key: "down", Description: "Next setting", Action: "next_setting", Context: SettingsView, Category: "Navigation", Enabled: true},
		{Key: "j", Description: "Next setting", Action: "next_setting", Context: SettingsView, Category: "Navigation", Enabled: true},
		{Key: "ctrl+s", Description: "Save settings", Action: "save_settings", Context: SettingsView, Category: "Actions", Enabled: true},
		{Key: "ctrl+r", Description: "Reset to defaults", Action: "reset_defaults", Context: SettingsView, Category: "Actions", Enabled: true},
		{Key: "e", Description: "Edit value", Action: "edit_value", Context: SettingsView, Category: "Edit", Enabled: true},
		{Key: "delete", Description: "Clear value", Action: "clear_value", Context: SettingsView, Category: "Edit", Enabled: true},
	}

	for _, shortcut := range settingsShortcuts {
		km.AddShortcut(shortcut)
	}

	// Initialize categories
	km.initializeCategories()
}

// initializeCategories organizes shortcuts into logical categories
func (km *KeyboardManager) initializeCategories() {
	categories := map[string]KeyCategory{
		"Application": {
			Name:        "Application",
			Description: "Global application controls",
		},
		"Navigation": {
			Name:        "Navigation",
			Description: "Move around the interface",
		},
		"Selection": {
			Name:        "Selection",
			Description: "Select and interact with items",
		},
		"Actions": {
			Name:        "Actions",
			Description: "Perform operations on files and buckets",
		},
		"Search": {
			Name:        "Search",
			Description: "Find and filter content",
		},
		"View": {
			Name:        "View",
			Description: "Change how information is displayed",
		},
		"Edit": {
			Name:        "Edit",
			Description: "Modify settings and values",
		},
		"Recovery": {
			Name:        "Error Recovery",
			Description: "Handle and resolve errors",
		},
		"Help": {
			Name:        "Help",
			Description: "Get assistance and information",
		},
		"Utility": {
			Name:        "Utilities",
			Description: "Additional utility functions",
		},
	}

	km.help.categories = categories
}

// AddShortcut adds a context-specific keyboard shortcut
func (km *KeyboardManager) AddShortcut(binding KeyBinding) {
	km.mutex.Lock()
	defer km.mutex.Unlock()

	key := fmt.Sprintf("%d:%s", binding.Context, binding.Key)
	km.shortcuts[key] = binding

	if _, exists := km.contexts[binding.Context]; !exists {
		km.contexts[binding.Context] = make([]KeyBinding, 0)
	}
	km.contexts[binding.Context] = append(km.contexts[binding.Context], binding)

	// Add to help system
	km.help.shortcuts[binding.Context] = append(km.help.shortcuts[binding.Context], binding)
}

// AddGlobalShortcut adds a global keyboard shortcut
func (km *KeyboardManager) AddGlobalShortcut(binding KeyBinding) {
	km.mutex.Lock()
	defer km.mutex.Unlock()

	binding.Global = true
	km.shortcuts[binding.Key] = binding
	km.help.globalShortcuts = append(km.help.globalShortcuts, binding)
}

// AddCustomShortcut allows users to define custom shortcuts
func (km *KeyboardManager) AddCustomShortcut(context ViewType, binding KeyBinding) {
	km.mutex.Lock()
	defer km.mutex.Unlock()

	if _, exists := km.customKeys[context]; !exists {
		km.customKeys[context] = make(map[string]KeyBinding)
	}
	km.customKeys[context][binding.Key] = binding
}

// SetCurrentView updates the current view context
func (km *KeyboardManager) SetCurrentView(view ViewType) {
	km.mutex.Lock()
	defer km.mutex.Unlock()

	km.currentView = view
	km.help.currentView = view
}

// HandleKeyPress processes keyboard input and returns appropriate action
func (km *KeyboardManager) HandleKeyPress(key string) (string, bool) {
	km.mutex.Lock()
	defer km.mutex.Unlock()

	if !km.enabled {
		return "", false
	}

	// Track key press frequency
	km.keyPressCount[key]++

	// Check for help toggle
	if key == "?" {
		km.help.visible = !km.help.visible
		return "toggle_help", true
	}

	// Check custom shortcuts first
	if customKeys, exists := km.customKeys[km.currentView]; exists {
		if binding, found := customKeys[key]; found && binding.Enabled {
			return binding.Action, true
		}
	}

	// Check context-specific shortcuts
	contextKey := fmt.Sprintf("%d:%s", km.currentView, key)
	if binding, exists := km.shortcuts[contextKey]; exists && binding.Enabled {
		return binding.Action, true
	}

	// Check global shortcuts
	if binding, exists := km.shortcuts[key]; exists && binding.Global && binding.Enabled {
		return binding.Action, true
	}

	return "", false
}

// Update handles messages for the keyboard manager
func (km *KeyboardManager) Update(msg tea.Msg) (*KeyboardManager, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if action, handled := km.HandleKeyPress(msg.String()); handled {
			return km, func() tea.Msg {
				return ShortcutExecutedMsg{
					Key:     msg.String(),
					Action:  action,
					Context: km.currentView,
				}
			}
		}
	case ToggleHelpMsg:
		km.help.visible = !km.help.visible
	}

	return km, nil
}

// GetContextualHelp returns help for the current context
func (km *KeyboardManager) GetContextualHelp() []KeyBinding {
	km.mutex.RLock()
	defer km.mutex.RUnlock()

	var help []KeyBinding

	// Add global shortcuts
	help = append(help, km.help.globalShortcuts...)

	// Add context-specific shortcuts
	if contextShortcuts, exists := km.help.shortcuts[km.currentView]; exists {
		help = append(help, contextShortcuts...)
	}

	return help
}

// GetShortcutsByCategory returns shortcuts organized by category
func (km *KeyboardManager) GetShortcutsByCategory() map[string][]KeyBinding {
	km.mutex.RLock()
	defer km.mutex.RUnlock()

	categories := make(map[string][]KeyBinding)

	// Get all relevant shortcuts
	allShortcuts := km.GetContextualHelp()

	// Group by category
	for _, shortcut := range allShortcuts {
		if shortcut.Enabled {
			categories[shortcut.Category] = append(categories[shortcut.Category], shortcut)
		}
	}

	// Sort shortcuts within each category
	for category := range categories {
		sort.Slice(categories[category], func(i, j int) bool {
			return categories[category][i].Key < categories[category][j].Key
		})
	}

	return categories
}

// IsHelpVisible returns whether help is currently displayed
func (km *KeyboardManager) IsHelpVisible() bool {
	return km.help.visible
}

// ToggleHelp toggles the help display
func (km *KeyboardManager) ToggleHelp() {
	km.help.visible = !km.help.visible
}

// RenderHelp generates the help display
func (km *KeyboardManager) RenderHelp() string {
	if !km.help.visible {
		return ""
	}

	var b strings.Builder

	// Title
	title := km.help.titleStyle.Render("⌨️ Keyboard Shortcuts")
	b.WriteString(title)
	b.WriteString("\n\n")

	// Current context
	contextName := km.getViewName(km.currentView)
	context := km.help.categoryStyle.Render(fmt.Sprintf("Current context: %s", contextName))
	b.WriteString(context)
	b.WriteString("\n\n")

	// Shortcuts by category
	categories := km.GetShortcutsByCategory()

	// Define category order for better organization
	categoryOrder := []string{"Application", "Navigation", "Selection", "Actions", "Search", "View", "Edit", "Recovery", "Help", "Utility"}

	for _, categoryName := range categoryOrder {
		if shortcuts, exists := categories[categoryName]; exists && len(shortcuts) > 0 {
			// Category header
			if categoryInfo, found := km.help.categories[categoryName]; found {
				categoryHeader := km.help.categoryStyle.Render(fmt.Sprintf("📂 %s", categoryInfo.Name))
				b.WriteString(categoryHeader)
				b.WriteString("\n")

				if categoryInfo.Description != "" {
					desc := km.help.descStyle.Render(fmt.Sprintf("   %s", categoryInfo.Description))
					b.WriteString(desc)
					b.WriteString("\n")
				}
			}

			// Shortcuts in this category
			for _, shortcut := range shortcuts {
				key := km.help.keyStyle.Render(shortcut.Key)
				desc := km.help.descStyle.Render(shortcut.Description)
				line := fmt.Sprintf("   %s %s", key, desc)
				b.WriteString(line)
				b.WriteString("\n")
			}
			b.WriteString("\n")
		}
	}

	// Footer
	footer := km.help.footerStyle.Render("Press ? to close help • ESC to go back")
	b.WriteString(footer)

	return b.String()
}

// getViewName returns human-readable name for view type
func (km *KeyboardManager) getViewName(view ViewType) string {
	names := map[ViewType]string{
		WelcomeView:     "Welcome",
		ListBucketsView: "Bucket List",
		ListObjectsView: "Object List",
		UploadView:      "Upload",
		DownloadView:    "Download",
		ErrorView:       "Error Recovery",
		SettingsView:    "Settings",
		HelpView:        "Help",
	}

	if name, exists := names[view]; exists {
		return name
	}
	return "Unknown"
}

// Enable enables keyboard navigation
func (km *KeyboardManager) Enable() {
	km.enabled = true
}

// Disable disables keyboard navigation
func (km *KeyboardManager) Disable() {
	km.enabled = false
}

// IsEnabled returns whether keyboard navigation is enabled
func (km *KeyboardManager) IsEnabled() bool {
	return km.enabled
}

// GetKeyPressStats returns statistics about key usage
func (km *KeyboardManager) GetKeyPressStats() map[string]int {
	km.mutex.RLock()
	defer km.mutex.RUnlock()

	stats := make(map[string]int)
	for key, count := range km.keyPressCount {
		stats[key] = count
	}

	return stats
}

// GetMostUsedShortcuts returns the most frequently used shortcuts
func (km *KeyboardManager) GetMostUsedShortcuts(limit int) []string {
	stats := km.GetKeyPressStats()

	type keyCount struct {
		key   string
		count int
	}

	var pairs []keyCount
	for key, count := range stats {
		pairs = append(pairs, keyCount{key, count})
	}

	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].count > pairs[j].count
	})

	var result []string
	for i, pair := range pairs {
		if i >= limit {
			break
		}
		result = append(result, pair.key)
	}

	return result
}
