package components

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// HelpContext represents different contexts where help can be shown
type HelpContext string

const (
	HelpContextMain      HelpContext = "main"
	HelpContextBucket    HelpContext = "bucket"
	HelpContextObject    HelpContext = "object"
	HelpContextOperation HelpContext = "operation"
	HelpContextUpload    HelpContext = "upload"
	HelpContextDownload  HelpContext = "download"
	HelpContextSettings  HelpContext = "settings"
	HelpContextError     HelpContext = "error"
	HelpContextLogs      HelpContext = "logs"
)

// HelpHint represents a contextual help hint
type HelpHint struct {
	Context     HelpContext
	Key         string
	Action      string
	Description string
	Priority    int         // Higher priority hints show first
	ShowWhen    func() bool // Optional condition to show hint
}

// HelpTooltip represents a tooltip with contextual information
type HelpTooltip struct {
	Title     string
	Content   string
	Context   HelpContext
	Shortcuts []HelpHint
	Tips      []string
	Visible   bool
	Timeout   time.Duration
	ShowTime  time.Time
}

// ContextualHelp manages contextual help and hints
type ContextualHelp struct {
	currentContext HelpContext
	hints          map[HelpContext][]HelpHint
	tooltip        *HelpTooltip
	showHints      bool
	compactMode    bool

	// Animation support
	animationManager *AnimationManager

	// Styles
	hintStyle        lipgloss.Style
	tooltipStyle     lipgloss.Style
	keyStyle         lipgloss.Style
	actionStyle      lipgloss.Style
	descriptionStyle lipgloss.Style
	titleStyle       lipgloss.Style
	tipStyle         lipgloss.Style
}

// NewContextualHelp creates a new contextual help system
func NewContextualHelp() *ContextualHelp {
	ch := &ContextualHelp{
		currentContext:   HelpContextMain,
		hints:            make(map[HelpContext][]HelpHint),
		showHints:        true,
		compactMode:      false,
		animationManager: NewAnimationManager(),

		hintStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888")).
			MarginTop(1).
			PaddingLeft(1),

		tooltipStyle: lipgloss.NewStyle().
			Background(lipgloss.Color("#2A2A2A")).
			Foreground(lipgloss.Color("#FFF")).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7D56F4")).
			Padding(1).
			MarginTop(1),

		keyStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#04B575")).
			Background(lipgloss.Color("#1A1A1A")).
			Padding(0, 1),

		actionStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#EE6FF8")),

		descriptionStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#CCC")),

		titleStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			Underline(true),

		tipStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFA500")).
			Italic(true),
	}

	ch.initializeDefaultHints()
	return ch
}

// initializeDefaultHints sets up default help hints for all contexts
func (ch *ContextualHelp) initializeDefaultHints() {
	// Main context hints
	ch.AddHint(HelpContextMain, "↑↓", "Navigate", "Move through menu items", 10)
	ch.AddHint(HelpContextMain, "Enter", "Select", "Choose current item", 9)
	ch.AddHint(HelpContextMain, "?", "Help", "Show comprehensive help", 8)
	ch.AddHint(HelpContextMain, "q", "Quit", "Exit application", 7)

	// Bucket context hints
	ch.AddHint(HelpContextBucket, "Enter", "Open", "Browse bucket contents", 10)
	ch.AddHint(HelpContextBucket, "r", "Refresh", "Reload bucket list", 9)
	ch.AddHint(HelpContextBucket, "s", "Settings", "Configure application", 8)
	ch.AddHint(HelpContextBucket, "l", "Logs", "View operation logs", 7)

	// Object context hints
	ch.AddHint(HelpContextObject, "d", "Download", "Download selected object(s)", 10)
	ch.AddHint(HelpContextObject, "u", "Upload", "Upload files to bucket", 9)
	ch.AddHint(HelpContextObject, "Del", "Delete", "Delete selected object(s)", 8)
	ch.AddHint(HelpContextObject, "Space", "Select", "Toggle object selection", 7)
	ch.AddHint(HelpContextObject, "Ctrl+A", "Select All", "Select all objects", 6)

	// Operation context hints
	ch.AddHint(HelpContextOperation, "Enter", "Execute", "Start the operation", 10)
	ch.AddHint(HelpContextOperation, "Esc", "Cancel", "Cancel and go back", 9)
	ch.AddHint(HelpContextOperation, "r", "Retry", "Retry failed operation", 8)

	// Upload context hints
	ch.AddHint(HelpContextUpload, "Ctrl+O", "Choose", "Select files to upload", 10)
	ch.AddHint(HelpContextUpload, "Enter", "Upload", "Start uploading files", 9)
	ch.AddHint(HelpContextUpload, "Ctrl+C", "Cancel", "Cancel upload operation", 8)

	// Settings context hints
	ch.AddHint(HelpContextSettings, "r", "Refresh", "Reload configuration", 10)
	ch.AddHint(HelpContextSettings, "e", "Edit", "Edit configuration file", 9)
	ch.AddHint(HelpContextSettings, "d", "Default", "Reset to defaults", 8)

	// Error context hints
	ch.AddHint(HelpContextError, "r", "Retry", "Retry the failed operation", 10)
	ch.AddHint(HelpContextError, "c", "Configure", "Open configuration", 9)
	ch.AddHint(HelpContextError, "l", "Logs", "View detailed logs", 8)
	ch.AddHint(HelpContextError, "h", "Help", "Get help for this error", 7)

	// Logs context hints
	ch.AddHint(HelpContextLogs, "r", "Refresh", "Refresh log entries", 10)
	ch.AddHint(HelpContextLogs, "c", "Clear", "Clear log history", 9)
	ch.AddHint(HelpContextLogs, "f", "Filter", "Filter log entries", 8)
	ch.AddHint(HelpContextLogs, "e", "Export", "Export logs to file", 7)
}

// SetContext changes the current help context
func (ch *ContextualHelp) SetContext(context HelpContext) {
	if ch.currentContext != context {
		ch.currentContext = context

		// Hide any existing tooltip when context changes
		if ch.tooltip != nil {
			ch.tooltip.Visible = false
		}

		// Create fade-in animation for new context hints
		ch.animationManager.CreateFadeInAnimation("context_hints", 300*time.Millisecond)
		ch.animationManager.StartAnimation("context_hints")
	}
}

// GetContext returns the current help context
func (ch *ContextualHelp) GetContext() HelpContext {
	return ch.currentContext
}

// AddHint adds a help hint for a specific context
func (ch *ContextualHelp) AddHint(context HelpContext, key, action, description string, priority int) {
	hint := HelpHint{
		Context:     context,
		Key:         key,
		Action:      action,
		Description: description,
		Priority:    priority,
	}

	ch.hints[context] = append(ch.hints[context], hint)
}

// AddConditionalHint adds a help hint that only shows when a condition is met
func (ch *ContextualHelp) AddConditionalHint(context HelpContext, key, action, description string, priority int, showWhen func() bool) {
	hint := HelpHint{
		Context:     context,
		Key:         key,
		Action:      action,
		Description: description,
		Priority:    priority,
		ShowWhen:    showWhen,
	}

	ch.hints[context] = append(ch.hints[context], hint)
}

// ShowTooltip displays a contextual tooltip
func (ch *ContextualHelp) ShowTooltip(title, content string, timeout time.Duration) {
	ch.tooltip = &HelpTooltip{
		Title:    title,
		Content:  content,
		Context:  ch.currentContext,
		Visible:  true,
		Timeout:  timeout,
		ShowTime: time.Now(),
	}

	// Create fade-in animation for tooltip
	ch.animationManager.CreateFadeInAnimation("tooltip", 200*time.Millisecond)
	ch.animationManager.StartAnimation("tooltip")
}

// ShowContextualTooltip shows a tooltip with context-specific shortcuts and tips
func (ch *ContextualHelp) ShowContextualTooltip(title, content string, tips []string, timeout time.Duration) {
	contextHints := ch.getVisibleHints(ch.currentContext)

	ch.tooltip = &HelpTooltip{
		Title:     title,
		Content:   content,
		Context:   ch.currentContext,
		Shortcuts: contextHints,
		Tips:      tips,
		Visible:   true,
		Timeout:   timeout,
		ShowTime:  time.Now(),
	}

	// Create fade-in animation for tooltip
	ch.animationManager.CreateFadeInAnimation("tooltip", 200*time.Millisecond)
	ch.animationManager.StartAnimation("tooltip")
}

// HideTooltip hides the current tooltip
func (ch *ContextualHelp) HideTooltip() {
	if ch.tooltip != nil {
		ch.tooltip.Visible = false
		ch.tooltip = nil
	}
}

// ToggleHints toggles the visibility of contextual hints
func (ch *ContextualHelp) ToggleHints() {
	ch.showHints = !ch.showHints
}

// SetCompactMode sets whether to use compact hint display
func (ch *ContextualHelp) SetCompactMode(compact bool) {
	ch.compactMode = compact
}

// Update handles messages for the contextual help system
func (ch *ContextualHelp) Update(msg tea.Msg) tea.Cmd {
	// Handle animation updates
	if cmd := ch.animationManager.Update(msg); cmd != nil {
		return cmd
	}

	// Handle tooltip timeout
	if ch.tooltip != nil && ch.tooltip.Visible && ch.tooltip.Timeout > 0 {
		if time.Since(ch.tooltip.ShowTime) > ch.tooltip.Timeout {
			ch.HideTooltip()
		}
	}

	return nil
}

// View renders the contextual help display
func (ch *ContextualHelp) View() string {
	var s strings.Builder

	// Render tooltip if visible
	if ch.tooltip != nil && ch.tooltip.Visible {
		tooltipContent := ch.renderTooltip()
		animatedTooltip := ch.animationManager.ApplyAnimationStyle("tooltip", ch.tooltipStyle, tooltipContent)
		s.WriteString(animatedTooltip)
		s.WriteString("\n")
	}

	// Render contextual hints if enabled
	if ch.showHints {
		hintsContent := ch.renderHints()
		if hintsContent != "" {
			animatedHints := ch.animationManager.ApplyAnimationStyle("context_hints", ch.hintStyle, hintsContent)
			s.WriteString(animatedHints)
		}
	}

	return s.String()
}

// renderTooltip renders the tooltip content
func (ch *ContextualHelp) renderTooltip() string {
	if ch.tooltip == nil {
		return ""
	}

	var s strings.Builder

	// Title
	s.WriteString(ch.titleStyle.Render(ch.tooltip.Title))
	s.WriteString("\n")

	// Content
	if ch.tooltip.Content != "" {
		s.WriteString(ch.tooltip.Content)
		s.WriteString("\n")
	}

	// Shortcuts if available
	if len(ch.tooltip.Shortcuts) > 0 {
		s.WriteString("\n")
		s.WriteString(ch.titleStyle.Render("Shortcuts:"))
		s.WriteString("\n")

		for _, hint := range ch.tooltip.Shortcuts[:min(5, len(ch.tooltip.Shortcuts))] { // Show max 5
			line := fmt.Sprintf("%s %s - %s",
				ch.keyStyle.Render(hint.Key),
				ch.actionStyle.Render(hint.Action),
				ch.descriptionStyle.Render(hint.Description))
			s.WriteString(line)
			s.WriteString("\n")
		}
	}

	// Tips if available
	if len(ch.tooltip.Tips) > 0 {
		s.WriteString("\n")
		s.WriteString(ch.titleStyle.Render("💡 Tips:"))
		s.WriteString("\n")

		for _, tip := range ch.tooltip.Tips {
			s.WriteString(ch.tipStyle.Render("• " + tip))
			s.WriteString("\n")
		}
	}

	return s.String()
}

// renderHints renders the contextual hints
func (ch *ContextualHelp) renderHints() string {
	hints := ch.getVisibleHints(ch.currentContext)
	if len(hints) == 0 {
		return ""
	}

	var s strings.Builder

	if ch.compactMode {
		// Compact mode: single line
		var parts []string
		for i, hint := range hints {
			if i >= 4 { // Show max 4 in compact mode
				break
			}
			part := fmt.Sprintf("%s:%s", ch.keyStyle.Render(hint.Key), hint.Action)
			parts = append(parts, part)
		}
		s.WriteString(strings.Join(parts, " • "))
	} else {
		// Full mode: multiple lines
		for i, hint := range hints {
			if i >= 6 { // Show max 6 in full mode
				break
			}

			line := fmt.Sprintf("%s %s",
				ch.keyStyle.Render(hint.Key),
				ch.actionStyle.Render(hint.Action))

			if hint.Description != "" {
				line += " - " + ch.descriptionStyle.Render(hint.Description)
			}

			s.WriteString(line)
			if i < len(hints)-1 && i < 5 {
				s.WriteString("\n")
			}
		}
	}

	return s.String()
}

// getVisibleHints returns hints for the current context that should be visible
func (ch *ContextualHelp) getVisibleHints(context HelpContext) []HelpHint {
	contextHints := ch.hints[context]
	if len(contextHints) == 0 {
		return nil
	}

	var visibleHints []HelpHint
	for _, hint := range contextHints {
		// Check if hint should be shown based on condition
		if hint.ShowWhen != nil && !hint.ShowWhen() {
			continue
		}

		visibleHints = append(visibleHints, hint)
	}

	// Sort by priority (higher first)
	for i := 0; i < len(visibleHints)-1; i++ {
		for j := i + 1; j < len(visibleHints); j++ {
			if visibleHints[i].Priority < visibleHints[j].Priority {
				visibleHints[i], visibleHints[j] = visibleHints[j], visibleHints[i]
			}
		}
	}

	return visibleHints
}

// GetContextualTips returns tips for the current context
func (ch *ContextualHelp) GetContextualTips() []string {
	switch ch.currentContext {
	case HelpContextBucket:
		return []string{
			"Use the region selector to change AWS regions",
			"Bucket names are globally unique across all AWS accounts",
			"Check IAM permissions if you can't see expected buckets",
		}
	case HelpContextObject:
		return []string{
			"Use Ctrl+A to select all objects for bulk operations",
			"Objects with '/' act as folders in the S3 console",
			"Download large files will show progress with ETA",
		}
	case HelpContextUpload:
		return []string{
			"Multiple files can be uploaded simultaneously",
			"Large files are automatically split into chunks",
			"Upload progress shows speed and estimated time remaining",
		}
	case HelpContextError:
		return []string{
			"Most errors are recoverable - try the suggested actions",
			"Check AWS credentials if you see permission errors",
			"Network errors usually resolve by retrying",
		}
	default:
		return []string{
			"Press '?' for comprehensive help anytime",
			"Use keyboard shortcuts for faster navigation",
			"Check settings to customize your experience",
		}
	}
}

// Utility function
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
