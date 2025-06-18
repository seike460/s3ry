package views

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/seike460/s3ry/internal/ui/components"
)

// HelpView represents the help view with comprehensive keyboard shortcuts
type HelpView struct {
	list *components.List

	// Styles
	headerStyle lipgloss.Style
	keyStyle    lipgloss.Style
	descStyle   lipgloss.Style
}

// NewHelpView creates a new help view
func NewHelpView() *HelpView {
	help := &HelpView{
		headerStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			MarginBottom(2),

		keyStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#04B575")),

		descStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888")),
	}

	// Create help items
	items := []components.ListItem{
		{
			Title:       "🌍 Navigation",
			Description: "Move around the application",
			Tag:         "Category",
		},
		{
			Title:       "↑/k - Move up",
			Description: "Navigate to previous item in lists",
			Tag:         "Key",
		},
		{
			Title:       "↓/j - Move down",
			Description: "Navigate to next item in lists",
			Tag:         "Key",
		},
		{
			Title:       "Enter/Space - Select",
			Description: "Select current item or confirm action",
			Tag:         "Key",
		},
		{
			Title:       "Esc - Back",
			Description: "Go back to previous view",
			Tag:         "Key",
		},
		{
			Title:       "",
			Description: "",
			Tag:         "Separator",
		},
		{
			Title:       "📁 File Operations",
			Description: "Work with S3 objects",
			Tag:         "Category",
		},
		{
			Title:       "d - Download",
			Description: "Download selected object(s) from S3",
			Tag:         "Key",
		},
		{
			Title:       "u - Upload",
			Description: "Upload file(s) to current bucket",
			Tag:         "Key",
		},
		{
			Title:       "Delete - Delete",
			Description: "Delete selected object(s) from S3",
			Tag:         "Key",
		},
		{
			Title:       "r - Refresh",
			Description: "Reload current list or retry failed operations",
			Tag:         "Key",
		},
		{
			Title:       "",
			Description: "",
			Tag:         "Separator",
		},
		{
			Title:       "⚙️ Application",
			Description: "General application controls",
			Tag:         "Category",
		},
		{
			Title:       "? - Help",
			Description: "Show this help page",
			Tag:         "Key",
		},
		{
			Title:       "s - Settings",
			Description: "Open application settings",
			Tag:         "Key",
		},
		{
			Title:       "l - Logs",
			Description: "View application logs and operations",
			Tag:         "Key",
		},
		{
			Title:       "Ctrl+C/q - Quit",
			Description: "Exit the application",
			Tag:         "Key",
		},
		{
			Title:       "",
			Description: "",
			Tag:         "Separator",
		},
		{
			Title:       "🚀 Pro Tips",
			Description: "Advanced usage",
			Tag:         "Category",
		},
		{
			Title:       "Modern UI Mode",
			Description: "Use --new-ui flag for enhanced TUI experience",
			Tag:         "Tip",
		},
		{
			Title:       "Parallel Downloads",
			Description: "Use --modern-backend for 5x faster operations",
			Tag:         "Tip",
		},
		{
			Title:       "Configuration",
			Description: "Set AWS_REGION, AWS_PROFILE, or use ~/.s3ry.yaml",
			Tag:         "Tip",
		},
		{
			Title:       "Keyboard Navigation",
			Description: "All operations can be performed via keyboard",
			Tag:         "Tip",
		},
	}

	help.list = components.NewList("📖 S3ry Help - Keyboard Shortcuts & Usage", items)

	return help
}

// Init initializes the help view
func (v *HelpView) Init() tea.Cmd {
	return nil
}

// Update handles messages for the help view
func (v *HelpView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if v.list != nil {
			v.list, _ = v.list.Update(msg)
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return v, tea.Quit
		case "esc":
			// Return to previous view - use a simple welcome for now
			return NewRegionView(), nil
		}

		if v.list != nil {
			v.list, _ = v.list.Update(msg)
		}
	}

	return v, nil
}

// View renders the help view
func (v *HelpView) View() string {
	if v.list == nil {
		return v.headerStyle.Render("Help not available")
	}

	footer := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262")).
		MarginTop(1).
		Render("esc/q: back • Use keyboard shortcuts shown above for efficient navigation")

	return v.list.View() + "\n" + footer
}
