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

// SettingsView displays the resolved configuration for the running process.
type SettingsView struct {
	deps   Deps
	config *config.Config
	list   *components.List
}

// NewSettingsView creates a settings view bound to the active configuration.
func NewSettingsView(deps Deps) *SettingsView {
	cfg := deps.Config
	if cfg == nil {
		cfg = config.Default()
	}
	v := &SettingsView{deps: deps, config: cfg}
	v.buildSettingsList()
	return v
}

// Init initializes the settings view.
func (v *SettingsView) Init() tea.Cmd {
	return nil
}

// Update handles messages for the settings view.
func (v *SettingsView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			return NewBucketView(v.deps), nil
		}

		if v.list != nil {
			v.list, _ = v.list.Update(msg)
		}
	}

	return v, nil
}

// View renders the settings view.
func (v *SettingsView) View() string {
	if v.list == nil {
		return errorStyle.Render(v.deps.T("Settings not available"))
	}
	footer := footerStyle.Render(v.deps.T("esc: back • q: quit"))
	return v.list.View() + "\n" + footer
}

// buildSettingsList renders the current configuration into list items.
func (v *SettingsView) buildSettingsList() {
	items := []components.ListItem{
		{Title: v.deps.T("AWS Configuration"), Tag: "Category"},
		{
			Title:       fmt.Sprintf("%s %s", v.deps.T("Region:"), v.value(v.config.AWS.Region)),
			Description: v.deps.T("Empty follows the AWS SDK default chain"),
			Tag:         "Setting",
		},
		{
			Title:       fmt.Sprintf("%s %s", v.deps.T("Profile:"), v.value(v.config.AWS.Profile)),
			Description: v.deps.T("AWS shared config profile"),
			Tag:         "Setting",
		},
		{
			Title:       fmt.Sprintf("%s %s", v.deps.T("Endpoint:"), v.value(v.config.AWS.Endpoint)),
			Description: v.deps.T("Custom S3 endpoint URL, for example LocalStack"),
			Tag:         "Setting",
		},
		{Title: v.deps.T("UI Configuration"), Tag: "Category"},
		{
			Title:       fmt.Sprintf("%s %s", v.deps.T("Language:"), v.value(v.config.UI.Language)),
			Description: v.deps.T("Interface language (en/ja)"),
			Tag:         "Setting",
		},
		{Title: v.deps.T("Performance"), Tag: "Category"},
		{
			Title:       fmt.Sprintf("%s %d", v.deps.T("Concurrency:"), v.config.Performance.Concurrency),
			Description: v.deps.T("Parallel S3 workers for transfers and listing"),
			Tag:         "Setting",
		},
		{
			Title:       fmt.Sprintf("%s %s", v.deps.T("Part size:"), components.FormatBytes(v.config.Performance.PartSize)),
			Description: v.deps.T("Multipart chunk size in bytes"),
			Tag:         "Setting",
		},
		{
			Title:       fmt.Sprintf("%s %s", v.deps.T("Timeout:"), fmt.Sprintf("%ds", v.config.Performance.Timeout)),
			Description: v.deps.T("Timeout for each blocking S3 request"),
			Tag:         "Setting",
		},
		{Title: v.deps.T("Environment"), Tag: "Category"},
		{
			Title:       fmt.Sprintf("AWS_REGION: %s", v.envValue("AWS_REGION")),
			Description: v.deps.T("Region override from the environment"),
			Tag:         "EnvVar",
		},
		{
			Title:       fmt.Sprintf("AWS_PROFILE: %s", v.envValue("AWS_PROFILE")),
			Description: v.deps.T("Profile override from the environment"),
			Tag:         "EnvVar",
		},
		{
			Title:       fmt.Sprintf("AWS_ACCESS_KEY_ID: %s", v.maskedEnv("AWS_ACCESS_KEY_ID")),
			Description: v.deps.T("Access key from the environment (masked)"),
			Tag:         "EnvVar",
		},
		{
			Title:       fmt.Sprintf("AWS_SECRET_ACCESS_KEY: %s", v.maskedEnv("AWS_SECRET_ACCESS_KEY")),
			Description: v.deps.T("Secret key from the environment (masked)"),
			Tag:         "EnvVar",
		},
	}

	v.list = components.NewList(v.deps.T("Settings"), items)
}

var valueStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(components.ColorSuccess))

func (v *SettingsView) value(s string) string {
	if s == "" {
		return errorStyle.Render(v.deps.T("(not set)"))
	}
	return valueStyle.Render(s)
}

func (v *SettingsView) envValue(key string) string {
	return v.value(os.Getenv(key))
}

func (v *SettingsView) maskedEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		return errorStyle.Render(v.deps.T("(not set)"))
	}
	if len(value) > 8 {
		return valueStyle.Render(value[:4] + strings.Repeat("*", len(value)-8) + value[len(value)-4:])
	}
	return valueStyle.Render(strings.Repeat("*", len(value)))
}
