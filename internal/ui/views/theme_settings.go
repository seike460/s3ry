package views

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/seike460/s3ry/internal/ui/components"
)

// ThemeSettingsView represents the theme settings view
type ThemeSettingsView struct {
	themeManager *components.ThemeManager
	list         *components.List
	preview      string
	showPreview  bool
	width        int
	height       int

	// Styles
	titleStyle   lipgloss.Style
	previewStyle lipgloss.Style
}

// NewThemeSettingsView creates a new theme settings view
func NewThemeSettingsView() *ThemeSettingsView {
	themeManager := components.NewThemeManager()

	// Create list of themes
	themes := themeManager.GetAvailableThemes()
	items := make([]components.ListItem, len(themes))

	currentTheme := themeManager.GetTheme()

	for i, themeName := range themes {
		isSelected := currentTheme != nil && currentTheme.Name == themeName
		status := ""
		if isSelected {
			status = " (current)"
		}

		items[i] = components.ListItem{
			Title:       themeName + status,
			Description: fmt.Sprintf("Preview theme: %s", themeName),
			Tag:         "Theme",
			Data:        themeName,
		}
	}

	list := components.NewList("🎨 Theme Settings", items)

	return &ThemeSettingsView{
		themeManager: themeManager,
		list:         list,
		showPreview:  true,

		titleStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			MarginBottom(1),

		previewStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7D56F4")).
			Padding(1).
			MarginTop(1),
	}
}

// Init initializes the theme settings view
func (v *ThemeSettingsView) Init() tea.Cmd {
	// Generate initial preview
	if v.list != nil {
		selectedItem := v.list.GetCurrentItem()
		if selectedItem != nil {
			themeName := selectedItem.Data.(string)
			v.preview = v.themeManager.GetThemePreview(themeName)
		}
	}
	return nil
}

// Update handles messages for the theme settings view
func (v *ThemeSettingsView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
		if v.list != nil {
			v.list, _ = v.list.Update(msg)
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return v, tea.Quit
		case "esc":
			// Go back to main settings
			return NewSettingsView(), nil
		case "?":
			// Show help
			return NewHelpView(), nil
		case "p":
			// Toggle preview
			v.showPreview = !v.showPreview
		case "enter", " ":
			if v.list != nil {
				selectedItem := v.list.GetCurrentItem()
				if selectedItem != nil {
					themeName := selectedItem.Data.(string)
					if err := v.themeManager.SetTheme(themeName); err == nil {
						// Theme changed successfully, update list to show current theme
						return v.updateThemeList(), nil
					}
				}
			}
		case "r":
			// Reset to default theme
			v.themeManager.SetTheme("dark")
			return v.updateThemeList(), nil
		}

		if v.list != nil {
			oldItem := v.list.GetCurrentItem()
			v.list, _ = v.list.Update(msg)
			newItem := v.list.GetCurrentItem()

			// Update preview if selection changed
			if v.showPreview && oldItem != newItem && newItem != nil {
				themeName := newItem.Data.(string)
				v.preview = v.themeManager.GetThemePreview(themeName)
			}
		}
	}

	return v, tea.Batch(cmds...)
}

// View renders the theme settings view
func (v *ThemeSettingsView) View() string {
	if v.list == nil {
		return "Loading themes..."
	}

	// Header
	header := v.titleStyle.Render("🎨 Theme Settings")

	// Instructions
	instructions := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6272A4")).
		Render("Select a theme and press Enter to apply • p: toggle preview • r: reset to default • esc: back")

	if v.showPreview && v.preview != "" {
		// Split view: list on left, preview on right
		listWidth := v.width / 2
		previewWidth := v.width - listWidth - 2 // Account for spacing

		if listWidth < 30 {
			listWidth = 30
		}
		if previewWidth < 30 {
			previewWidth = 30
		}

		listStyle := lipgloss.NewStyle().Width(listWidth)
		previewStyled := v.previewStyle.Width(previewWidth)

		listView := listStyle.Render(v.list.View())
		previewView := previewStyled.Render(v.preview)

		content := lipgloss.JoinHorizontal(lipgloss.Top, listView, "  ", previewView)

		return header + "\n" + instructions + "\n\n" + content
	} else {
		// Full width list
		return header + "\n" + instructions + "\n\n" + v.list.View()
	}
}

// updateThemeList updates the theme list to reflect current selection
func (v *ThemeSettingsView) updateThemeList() tea.Model {
	themes := v.themeManager.GetAvailableThemes()
	items := make([]components.ListItem, len(themes))

	currentTheme := v.themeManager.GetTheme()

	for i, themeName := range themes {
		isSelected := currentTheme != nil && currentTheme.Name == themeName
		status := ""
		if isSelected {
			status = " (current)"
		}

		items[i] = components.ListItem{
			Title:       themeName + status,
			Description: fmt.Sprintf("Preview theme: %s", themeName),
			Tag:         "Theme",
			Data:        themeName,
		}
	}

	v.list = components.NewList("🎨 Theme Settings", items)

	// Update preview for current selection
	if v.showPreview && v.list != nil {
		selectedItem := v.list.GetCurrentItem()
		if selectedItem != nil {
			themeName := selectedItem.Data.(string)
			v.preview = v.themeManager.GetThemePreview(themeName)
		}
	}

	return v
}

// GetAvailableThemes returns available themes with descriptions
func GetAvailableThemes() map[string]string {
	return map[string]string{
		"dark":          "Dark theme with purple accents (default)",
		"light":         "Light theme with blue accents",
		"high-contrast": "High contrast theme for accessibility",
		"minimal":       "Minimal monochrome theme",
		"neon":          "Cyberpunk neon theme",
		"ocean":         "Ocean blue theme",
	}
}

// CreateCustomThemeView creates a view for creating custom themes
type CustomThemeView struct {
	themeManager *components.ThemeManager
	currentStep  int
	themeName    string
	colors       components.ThemeColors
	styles       components.ThemeStyles

	// Styles
	titleStyle lipgloss.Style
	stepStyle  lipgloss.Style
}

// NewCustomThemeView creates a new custom theme creation view
func NewCustomThemeView() *CustomThemeView {
	return &CustomThemeView{
		themeManager: components.NewThemeManager(),
		currentStep:  0,

		titleStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			MarginBottom(1),

		stepStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#50FA7B")).
			Bold(true),
	}
}

// Init initializes the custom theme view
func (v *CustomThemeView) Init() tea.Cmd {
	return nil
}

// Update handles messages for the custom theme view
func (v *CustomThemeView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return v, tea.Quit
		case "esc":
			// Go back to theme settings
			return NewThemeSettingsView(), nil
		case "enter":
			// Process current step
			return v.processStep(), nil
		}
	}

	return v, nil
}

// View renders the custom theme view
func (v *CustomThemeView) View() string {
	header := v.titleStyle.Render("🎨 Create Custom Theme")

	steps := []string{
		"Enter theme name",
		"Choose primary color",
		"Choose secondary color",
		"Choose background color",
		"Choose text color",
		"Review and save",
	}

	// Show current step
	currentStepText := v.stepStyle.Render(fmt.Sprintf("Step %d/%d: %s",
		v.currentStep+1, len(steps), steps[v.currentStep]))

	instructions := "Use arrow keys to navigate • Enter to confirm • Esc to go back"

	// Step-specific content
	var content string
	switch v.currentStep {
	case 0:
		content = "Enter a name for your custom theme:\n\n> " + v.themeName
	case 1:
		content = "Choose primary color (accent color for highlights):\n\n" +
			"Examples: #7D56F4, #0366D6, #FF6B6B, #4ECDC4\n\n> "
	default:
		content = "Theme creation in progress..."
	}

	return strings.Join([]string{
		header,
		currentStepText,
		"",
		content,
		"",
		instructions,
	}, "\n")
}

// processStep processes the current step
func (v *CustomThemeView) processStep() tea.Model {
	v.currentStep++

	// If all steps completed, save theme and return to settings
	if v.currentStep >= 6 {
		// Create and save theme
		theme := &components.Theme{
			Name:        v.themeName,
			Description: "Custom theme",
			Colors:      v.colors,
			Styles:      v.styles,
		}

		v.themeManager.AddCustomTheme(theme)
		v.themeManager.SaveCustomThemes()

		return NewThemeSettingsView()
	}

	return v
}
