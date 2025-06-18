package views

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/seike460/s3ry/internal/config"
	"github.com/seike460/s3ry/internal/ui/components"
)

// SettingInfo represents information about a setting
type SettingInfo struct {
	Key   string
	Name  string
	Value string
}

// SettingsView represents the settings configuration view
type SettingsView struct {
	list   *components.List
	config *config.Config
	status string

	// Styles
	headerStyle lipgloss.Style
	valueStyle  lipgloss.Style
	errorStyle  lipgloss.Style
}

// NewSettingsView creates a new settings view
func NewSettingsView() *SettingsView {
	// Load current configuration
	cfg, err := config.Load()
	if err != nil {
		cfg = config.Default()
	}

	settings := &SettingsView{
		config: cfg,
		headerStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			MarginBottom(2),

		valueStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#04B575")),

		errorStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF5555")),
	}

	settings.buildSettingsList()
	return settings
}

// Init initializes the settings view
func (v *SettingsView) Init() tea.Cmd {
	return nil
}

// Update handles messages for the settings view
func (v *SettingsView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if v.list != nil {
			v.list, _ = v.list.Update(msg)
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return v, tea.Quit
		case "r":
			// Refresh settings
			cfg, err := config.Load()
			if err != nil {
				cfg = config.Default()
			}
			v.config = cfg
			v.buildSettingsList()
			return v, nil
		case "enter", " ":
			if v.list != nil {
				selectedItem := v.list.GetCurrentItem()
				if selectedItem != nil && selectedItem.Data != nil {
					setting := selectedItem.Data.(SettingInfo)
					switch setting.Key {
					case "theme":
						// Navigate to theme settings
						return NewThemeSettingsView(), nil
					default:
						// For other settings, show a message
						v.status = fmt.Sprintf("Selected: %s", setting.Name)
					}
				}
			}
		}

		if v.list != nil {
			v.list, _ = v.list.Update(msg)
		}
	}

	return v, nil
}

// View renders the settings view
func (v *SettingsView) View() string {
	if v.list == nil {
		return v.errorStyle.Render("Settings not available")
	}

	footer := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262")).
		MarginTop(1).
		Render("r: refresh • esc/q: back • Edit ~/.s3ry.yaml to modify settings")

	return v.list.View() + "\n" + footer
}

// buildSettingsList creates the settings list items
func (v *SettingsView) buildSettingsList() {
	items := []components.ListItem{
		{
			Title:       "⚙️ Current Configuration",
			Description: "Active settings for S3ry application",
			Tag:         "Header",
		},
		{
			Title:       "",
			Description: "",
			Tag:         "Separator",
		},
		{
			Title:       "🌍 AWS Configuration",
			Description: "AWS service settings",
			Tag:         "Category",
		},
		{
			Title:       fmt.Sprintf("Region: %s", v.getConfigValue("Region", v.config.AWS.Region)),
			Description: "Current AWS region for S3 operations",
			Tag:         "Setting",
		},
		{
			Title:       fmt.Sprintf("Profile: %s", v.getConfigValue("Profile", v.config.AWS.Profile)),
			Description: "AWS CLI profile being used",
			Tag:         "Setting",
		},
		{
			Title:       fmt.Sprintf("Endpoint: %s", v.getConfigValue("Endpoint", v.config.AWS.Endpoint)),
			Description: "Custom S3 endpoint URL (if configured)",
			Tag:         "Setting",
		},
		{
			Title:       "",
			Description: "",
			Tag:         "Separator",
		},
		{
			Title:       "🎨 UI Configuration",
			Description: "User interface settings",
			Tag:         "Category",
		},
		{
			Title:       fmt.Sprintf("Language: %s", v.getConfigValue("Language", v.config.UI.Language)),
			Description: "Application language (en/ja)",
			Tag:         "Setting",
		},
		{
			Title:       fmt.Sprintf("Theme: %s", v.getConfigValue("Theme", v.config.UI.Theme)),
			Description: "Color theme for the interface",
			Tag:         "Setting",
			Data:        SettingInfo{Key: "theme", Name: "Theme", Value: v.config.UI.Theme},
		},
		{
			Title:       fmt.Sprintf("Mode: %s", v.getConfigValue("Mode", v.config.UI.Mode)),
			Description: "UI mode (legacy/bubbles)",
			Tag:         "Setting",
		},
		{
			Title:       "",
			Description: "",
			Tag:         "Separator",
		},
		{
			Title:       "🚀 Performance Settings",
			Description: "Optimization and performance configuration",
			Tag:         "Category",
		},
		{
			Title:       fmt.Sprintf("Workers: %d", v.config.Performance.Workers),
			Description: "Number of parallel workers",
			Tag:         "Setting",
		},
		{
			Title:       fmt.Sprintf("Chunk Size: %s", v.formatBytes(int64(v.config.Performance.ChunkSize))),
			Description: "Size of individual transfer chunks",
			Tag:         "Setting",
		},
		{
			Title:       fmt.Sprintf("Timeout: %d seconds", v.config.Performance.Timeout),
			Description: "Operation timeout duration",
			Tag:         "Setting",
		},
		{
			Title:       "",
			Description: "",
			Tag:         "Separator",
		},
		{
			Title:       "🔧 Environment Variables",
			Description: "System environment configuration",
			Tag:         "Category",
		},
		{
			Title:       fmt.Sprintf("AWS_REGION: %s", v.getEnvValue("AWS_REGION")),
			Description: "AWS region from environment",
			Tag:         "EnvVar",
		},
		{
			Title:       fmt.Sprintf("AWS_PROFILE: %s", v.getEnvValue("AWS_PROFILE")),
			Description: "AWS profile from environment",
			Tag:         "EnvVar",
		},
		{
			Title:       fmt.Sprintf("AWS_ACCESS_KEY_ID: %s", v.getMaskedEnvValue("AWS_ACCESS_KEY_ID")),
			Description: "AWS access key (masked for security)",
			Tag:         "EnvVar",
		},
		{
			Title:       fmt.Sprintf("AWS_SECRET_ACCESS_KEY: %s", v.getMaskedEnvValue("AWS_SECRET_ACCESS_KEY")),
			Description: "AWS secret key (masked for security)",
			Tag:         "EnvVar",
		},
		{
			Title:       "",
			Description: "",
			Tag:         "Separator",
		},
		{
			Title:       "📁 Configuration File",
			Description: "Settings file information",
			Tag:         "Category",
		},
		{
			Title:       fmt.Sprintf("Config Path: %s", v.getConfigPath()),
			Description: "Location of the configuration file",
			Tag:         "Info",
		},
		{
			Title:       "Edit Config File",
			Description: "Modify ~/.s3ry.yaml to change these settings",
			Tag:         "Action",
		},
	}

	v.list = components.NewList("⚙️ S3ry Settings & Configuration", items)
}

// Helper methods
func (v *SettingsView) getConfigValue(key, value string) string {
	if value == "" {
		return v.errorStyle.Render("(not set)")
	}
	return v.valueStyle.Render(value)
}

func (v *SettingsView) getBoolValue(value bool) string {
	if value {
		return v.valueStyle.Render("✓ Enabled")
	}
	return "✗ Disabled"
}

func (v *SettingsView) getEnvValue(key string) string {
	value := os.Getenv(key)
	if value == "" {
		return v.errorStyle.Render("(not set)")
	}
	return v.valueStyle.Render(value)
}

func (v *SettingsView) getMaskedEnvValue(key string) string {
	value := os.Getenv(key)
	if value == "" {
		return v.errorStyle.Render("(not set)")
	}
	if len(value) > 8 {
		return v.valueStyle.Render(value[:4] + strings.Repeat("*", len(value)-8) + value[len(value)-4:])
	}
	return v.valueStyle.Render(strings.Repeat("*", len(value)))
}

func (v *SettingsView) getConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return v.errorStyle.Render("(unknown)")
	}
	return v.valueStyle.Render(home + "/.s3ry.yaml")
}

func (v *SettingsView) formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
