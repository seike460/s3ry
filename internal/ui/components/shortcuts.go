package components

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ShortcutAction represents a keyboard shortcut action
type ShortcutAction string

const (
	ActionQuit        ShortcutAction = "quit"
	ActionBack        ShortcutAction = "back"
	ActionRefresh     ShortcutAction = "refresh"
	ActionHelp        ShortcutAction = "help"
	ActionSettings    ShortcutAction = "settings"
	ActionLogs        ShortcutAction = "logs"
	ActionPreview     ShortcutAction = "preview"
	ActionConfirm     ShortcutAction = "confirm"
	ActionCancel      ShortcutAction = "cancel"
	ActionDownload    ShortcutAction = "download"
	ActionUpload      ShortcutAction = "upload"
	ActionDelete      ShortcutAction = "delete"
	ActionSelect      ShortcutAction = "select"
	ActionToggle      ShortcutAction = "toggle"
	ActionNext        ShortcutAction = "next"
	ActionPrevious    ShortcutAction = "previous"
	ActionFirst       ShortcutAction = "first"
	ActionLast        ShortcutAction = "last"
)

// ShortcutKey represents a keyboard shortcut
type ShortcutKey struct {
	Key         string         `json:"key"`
	Alt         bool           `json:"alt,omitempty"`
	Ctrl        bool           `json:"ctrl,omitempty"`
	Shift       bool           `json:"shift,omitempty"`
	Action      ShortcutAction `json:"action"`
	Description string         `json:"description"`
	Context     string         `json:"context,omitempty"` // Which view/context this applies to
}

// ShortcutManager manages keyboard shortcuts
type ShortcutManager struct {
	shortcuts    map[string]ShortcutKey
	contextMap   map[string][]ShortcutKey // shortcuts by context
	configPath   string
	
	// Styles
	keyStyle     lipgloss.Style
	actionStyle  lipgloss.Style
	contextStyle lipgloss.Style
}

// NewShortcutManager creates a new shortcut manager
func NewShortcutManager() *ShortcutManager {
	homeDir, _ := os.UserHomeDir()
	configPath := filepath.Join(homeDir, ".s3ry", "shortcuts.json")
	
	sm := &ShortcutManager{
		shortcuts:  make(map[string]ShortcutKey),
		contextMap: make(map[string][]ShortcutKey),
		configPath: configPath,
		
		keyStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			Background(lipgloss.Color("#2D2D2D")).
			Padding(0, 1),
		
		actionStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA")),
		
		contextStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#50FA7B")).
			MarginTop(1),
	}
	
	// Load default shortcuts
	sm.loadDefaultShortcuts()
	
	// Try to load custom shortcuts
	sm.loadCustomShortcuts()
	
	return sm
}

// loadDefaultShortcuts loads the default keyboard shortcuts
func (sm *ShortcutManager) loadDefaultShortcuts() {
	defaults := []ShortcutKey{
		// Global shortcuts
		{Key: "ctrl+c", Action: ActionQuit, Description: "Quit application", Context: "global"},
		{Key: "q", Action: ActionQuit, Description: "Quit application", Context: "global"},
		{Key: "esc", Action: ActionBack, Description: "Go back", Context: "global"},
		{Key: "?", Action: ActionHelp, Description: "Show help", Context: "global"},
		{Key: "r", Action: ActionRefresh, Description: "Refresh", Context: "global"},
		{Key: "s", Action: ActionSettings, Description: "Settings", Context: "global"},
		{Key: "l", Action: ActionLogs, Description: "Show logs", Context: "global"},
		
		// Navigation shortcuts
		{Key: "j", Action: ActionNext, Description: "Move down", Context: "navigation"},
		{Key: "k", Action: ActionPrevious, Description: "Move up", Context: "navigation"},
		{Key: "down", Action: ActionNext, Description: "Move down", Context: "navigation"},
		{Key: "up", Action: ActionPrevious, Description: "Move up", Context: "navigation"},
		{Key: "home", Action: ActionFirst, Description: "Go to first item", Context: "navigation"},
		{Key: "end", Action: ActionLast, Description: "Go to last item", Context: "navigation"},
		{Key: "g", Action: ActionFirst, Description: "Go to first item", Context: "navigation"},
		{Key: "G", Action: ActionLast, Description: "Go to last item", Context: "navigation"},
		
		// Object view shortcuts
		{Key: "enter", Action: ActionConfirm, Description: "Select/Confirm", Context: "object"},
		{Key: " ", Action: ActionSelect, Description: "Select item", Context: "object"},
		{Key: "p", Action: ActionPreview, Description: "Toggle preview", Context: "object"},
		{Key: "d", Action: ActionDownload, Description: "Download", Context: "object"},
		{Key: "u", Action: ActionUpload, Description: "Upload", Context: "object"},
		{Key: "x", Action: ActionDelete, Description: "Delete", Context: "object"},
		
		// Enhanced shortcuts for power users
		{Key: "ctrl+r", Action: ActionRefresh, Description: "Force refresh", Context: "global"},
		{Key: "ctrl+p", Action: ActionPreview, Description: "Toggle preview pane", Context: "object"},
		{Key: "ctrl+d", Action: ActionDownload, Description: "Batch download", Context: "object"},
		{Key: "ctrl+u", Action: ActionUpload, Description: "Batch upload", Context: "object"},
		{Key: "ctrl+x", Action: ActionDelete, Description: "Batch delete", Context: "object"},
		
		// Vi-style shortcuts for navigation
		{Key: "h", Action: ActionBack, Description: "Go back (vi-style)", Context: "navigation"},
		{Key: "0", Action: ActionFirst, Description: "Go to beginning", Context: "navigation"},
		{Key: "$", Action: ActionLast, Description: "Go to end", Context: "navigation"},
		
		// Toggle shortcuts
		{Key: "t", Action: ActionToggle, Description: "Toggle selection", Context: "object"},
		{Key: "ctrl+a", Action: ActionSelect, Description: "Select all", Context: "object"},
	}
	
	for _, shortcut := range defaults {
		key := sm.formatKey(shortcut)
		sm.shortcuts[key] = shortcut
		sm.contextMap[shortcut.Context] = append(sm.contextMap[shortcut.Context], shortcut)
	}
}

// loadCustomShortcuts loads custom shortcuts from config file
func (sm *ShortcutManager) loadCustomShortcuts() {
	if _, err := os.Stat(sm.configPath); os.IsNotExist(err) {
		return // No custom shortcuts file
	}
	
	data, err := os.ReadFile(sm.configPath)
	if err != nil {
		return
	}
	
	var customShortcuts []ShortcutKey
	if err := json.Unmarshal(data, &customShortcuts); err != nil {
		return
	}
	
	// Override or add custom shortcuts
	for _, shortcut := range customShortcuts {
		key := sm.formatKey(shortcut)
		sm.shortcuts[key] = shortcut
		
		// Update context map
		found := false
		for i, existing := range sm.contextMap[shortcut.Context] {
			if sm.formatKey(existing) == key {
				sm.contextMap[shortcut.Context][i] = shortcut
				found = true
				break
			}
		}
		if !found {
			sm.contextMap[shortcut.Context] = append(sm.contextMap[shortcut.Context], shortcut)
		}
	}
}

// SaveCustomShortcuts saves custom shortcuts to config file
func (sm *ShortcutManager) SaveCustomShortcuts(customShortcuts []ShortcutKey) error {
	// Ensure config directory exists
	dir := filepath.Dir(sm.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	
	data, err := json.MarshalIndent(customShortcuts, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal shortcuts: %w", err)
	}
	
	if err := os.WriteFile(sm.configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write shortcuts file: %w", err)
	}
	
	// Reload shortcuts
	sm.loadDefaultShortcuts()
	sm.loadCustomShortcuts()
	
	return nil
}

// formatKey formats a shortcut key for lookup
func (sm *ShortcutManager) formatKey(shortcut ShortcutKey) string {
	var parts []string
	
	if shortcut.Ctrl {
		parts = append(parts, "ctrl")
	}
	if shortcut.Alt {
		parts = append(parts, "alt")
	}
	if shortcut.Shift {
		parts = append(parts, "shift")
	}
	
	parts = append(parts, shortcut.Key)
	
	return strings.Join(parts, "+")
}

// GetAction returns the action for a given key combination
func (sm *ShortcutManager) GetAction(keyMsg tea.KeyMsg) (ShortcutAction, bool) {
	var keyParts []string
	
	if keyMsg.Type == tea.KeyCtrlC {
		return ActionQuit, true
	}
	
	// Handle modifier keys
	if keyMsg.Type == tea.KeyCtrlR {
		keyParts = append(keyParts, "ctrl", "r")
	} else if keyMsg.Type == tea.KeyCtrlP {
		keyParts = append(keyParts, "ctrl", "p")
	} else if keyMsg.Type == tea.KeyCtrlD {
		keyParts = append(keyParts, "ctrl", "d")
	} else if keyMsg.Type == tea.KeyCtrlU {
		keyParts = append(keyParts, "ctrl", "u")
	} else if keyMsg.Type == tea.KeyCtrlX {
		keyParts = append(keyParts, "ctrl", "x")
	} else if keyMsg.Type == tea.KeyCtrlA {
		keyParts = append(keyParts, "ctrl", "a")
	} else {
		// Handle regular keys
		keyStr := keyMsg.String()
		if keyStr == "ctrl+c" {
			return ActionQuit, true
		}
		keyParts = append(keyParts, keyStr)
	}
	
	key := strings.Join(keyParts, "+")
	if shortcut, exists := sm.shortcuts[key]; exists {
		return shortcut.Action, true
	}
	
	return "", false
}

// GetShortcutsForContext returns shortcuts for a specific context
func (sm *ShortcutManager) GetShortcutsForContext(context string) []ShortcutKey {
	shortcuts := make([]ShortcutKey, 0)
	
	// Add global shortcuts
	if context != "global" {
		shortcuts = append(shortcuts, sm.contextMap["global"]...)
	}
	
	// Add context-specific shortcuts
	if contextShortcuts, exists := sm.contextMap[context]; exists {
		shortcuts = append(shortcuts, contextShortcuts...)
	}
	
	return shortcuts
}

// RenderShortcutHelp renders a help display for shortcuts
func (sm *ShortcutManager) RenderShortcutHelp(context string) string {
	shortcuts := sm.GetShortcutsForContext(context)
	
	if len(shortcuts) == 0 {
		return "No shortcuts available"
	}
	
	// Group shortcuts by context
	contextGroups := make(map[string][]ShortcutKey)
	for _, shortcut := range shortcuts {
		contextGroups[shortcut.Context] = append(contextGroups[shortcut.Context], shortcut)
	}
	
	var output strings.Builder
	
	for ctx, ctxShortcuts := range contextGroups {
		if ctx == "global" {
			output.WriteString(sm.contextStyle.Render("Global Shortcuts"))
		} else {
			output.WriteString(sm.contextStyle.Render(strings.Title(ctx) + " Shortcuts"))
		}
		output.WriteString("\n")
		
		for _, shortcut := range ctxShortcuts {
			keyDisplay := sm.formatKeyDisplay(shortcut)
			line := fmt.Sprintf("%s %s",
				sm.keyStyle.Render(keyDisplay),
				sm.actionStyle.Render(shortcut.Description),
			)
			output.WriteString(line)
			output.WriteString("\n")
		}
		output.WriteString("\n")
	}
	
	return output.String()
}

// formatKeyDisplay formats a key for display
func (sm *ShortcutManager) formatKeyDisplay(shortcut ShortcutKey) string {
	var parts []string
	
	if shortcut.Ctrl {
		parts = append(parts, "Ctrl")
	}
	if shortcut.Alt {
		parts = append(parts, "Alt")
	}
	if shortcut.Shift {
		parts = append(parts, "Shift")
	}
	
	// Format special keys
	key := shortcut.Key
	switch key {
	case "enter":
		key = "↵"
	case " ":
		key = "Space"
	case "esc":
		key = "Esc"
	case "up":
		key = "↑"
	case "down":
		key = "↓"
	case "left":
		key = "←"
	case "right":
		key = "→"
	case "home":
		key = "Home"
	case "end":
		key = "End"
	}
	
	parts = append(parts, key)
	
	return strings.Join(parts, "+")
}

// RenderFooterShortcuts renders a compact footer with common shortcuts
func (sm *ShortcutManager) RenderFooterShortcuts(context string) string {
	// Get most important shortcuts for footer
	importantActions := []ShortcutAction{
		ActionRefresh, ActionPreview, ActionHelp, ActionSettings, ActionLogs, ActionBack, ActionQuit,
	}
	
	var parts []string
	shortcuts := sm.GetShortcutsForContext(context)
	
	for _, action := range importantActions {
		for _, shortcut := range shortcuts {
			if shortcut.Action == action {
				keyDisplay := sm.formatKeyDisplay(shortcut)
				// Shorten descriptions for footer
				desc := shortcut.Description
				switch action {
				case ActionRefresh:
					desc = "refresh"
				case ActionPreview:
					desc = "preview"
				case ActionHelp:
					desc = "help"
				case ActionSettings:
					desc = "settings"
				case ActionLogs:
					desc = "logs"
				case ActionBack:
					desc = "back"
				case ActionQuit:
					desc = "quit"
				}
				
				parts = append(parts, fmt.Sprintf("%s: %s", keyDisplay, desc))
				break
			}
		}
	}
	
	return strings.Join(parts, " • ")
}