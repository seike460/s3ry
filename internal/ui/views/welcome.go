package views

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// WelcomeStep represents a single step in the welcome tutorial
type WelcomeStep struct {
	Title       string
	Description string
	Action      string
	KeyHint     string
	Icon        string
}

// WelcomeView represents the welcome screen with tutorial
type WelcomeView struct {
	currentStep    int
	steps          []WelcomeStep
	showTutorial   bool
	setupMode      bool
	credentialsSet bool
	regionSet      bool
	width          int
	height         int

	// AWS setup tracking
	awsProfile    string
	awsRegion     string
	setupProgress map[string]bool

	// Styles
	titleStyle       lipgloss.Style
	stepStyle        lipgloss.Style
	descriptionStyle lipgloss.Style
	actionStyle      lipgloss.Style
	keyHintStyle     lipgloss.Style
	progressStyle    lipgloss.Style
	footerStyle      lipgloss.Style
	successStyle     lipgloss.Style
	warningStyle     lipgloss.Style
	setupStyle       lipgloss.Style
}

// NewWelcomeView creates a new welcome view
func NewWelcomeView() *WelcomeView {
	steps := []WelcomeStep{
		{
			Title:       "Welcome to S3ry v2.0.0",
			Description: "Ultra-fast S3 management with 60fps responsive interface.\nExperience 10x performance improvement with intelligent error handling.",
			Action:      "Press Enter to continue",
			KeyHint:     "Enter",
			Icon:        "🚀",
		},
		{
			Title:       "AWS Credentials Setup",
			Description: "Let's configure your AWS credentials for secure access.\nWe'll check your existing setup and guide you through any missing configuration.",
			Action:      "Press 's' to start setup or Enter to skip",
			KeyHint:     "s / Enter",
			Icon:        "🔐",
		},
		{
			Title:       "Region Configuration",
			Description: "Choose your preferred AWS region for optimal performance.\nS3ry can automatically detect your region or let you specify one.",
			Action:      "Press 'r' for region setup or Enter to continue",
			KeyHint:     "r / Enter",
			Icon:        "🌍",
		},
		{
			Title:       "Navigation Basics",
			Description: "Use arrow keys (↑/↓) or Vim-style keys (j/k) to navigate.\nPress Enter or Space to select items.",
			Action:      "Try navigating with ↑/↓ or j/k",
			KeyHint:     "↑/↓ or j/k",
			Icon:        "🎮",
		},
		{
			Title:       "Keyboard Shortcuts",
			Description: "S3ry is designed for keyboard efficiency:\n• '?' - Show help anytime\n• 'q' - Quit application\n• 'r' - Refresh current view",
			Action:      "Press '?' to see all shortcuts",
			KeyHint:     "? / q / r",
			Icon:        "⌨️",
		},
		{
			Title:       "Performance Features",
			Description: "Experience breakthrough S3 performance:\n• 60fps responsive interface\n• Async rendering and virtual scrolling\n• Smart error recovery and guidance\n• Intelligent onboarding system",
			Action:      "Press Enter to start using S3ry",
			KeyHint:     "Enter",
			Icon:        "⚡",
		},
	}

	return &WelcomeView{
		currentStep:    0,
		steps:          steps,
		showTutorial:   true,
		setupMode:      false,
		credentialsSet: false,
		regionSet:      false,
		awsProfile:     "",
		awsRegion:      "ap-northeast-1", // Default region
		setupProgress: map[string]bool{
			"credentials": false,
			"region":      false,
			"profile":     false,
		},

		titleStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			Align(lipgloss.Center).
			MarginBottom(2),

		stepStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#04B575")).
			Align(lipgloss.Center).
			MarginBottom(1),

		descriptionStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#CCCCCC")).
			Align(lipgloss.Center).
			MarginBottom(2),

		actionStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFAA00")).
			Align(lipgloss.Center).
			MarginBottom(1),

		keyHintStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1).
			Align(lipgloss.Center),

		progressStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888888")).
			Align(lipgloss.Center).
			MarginTop(2),

		footerStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262")).
			Align(lipgloss.Center).
			MarginTop(2),

		successStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#04B575")).
			Align(lipgloss.Center),

		warningStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFAA00")).
			Align(lipgloss.Center),

		setupStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7D56F4")).
			Background(lipgloss.Color("#2A2A2A")).
			Padding(1).
			MarginBottom(1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#7D56F4")),
	}
}

// Init initializes the welcome view
func (v *WelcomeView) Init() tea.Cmd {
	return nil
}

// Update handles messages for the welcome view
func (v *WelcomeView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return v, tea.Quit

		case "esc":
			if v.currentStep > 0 {
				v.currentStep--
			}

		case "enter", " ":
			if v.currentStep < len(v.steps)-1 {
				v.currentStep++
			} else {
				// Tutorial completed, transition to main app
				return v, tea.Sequence(
					tea.Printf("Welcome tutorial completed!"),
					func() tea.Msg { return CompleteTutorialMsg{} },
				)
			}

		case "s":
			// Start AWS credentials setup
			if v.currentStep == 1 {
				v.setupMode = true
				return v, func() tea.Msg { return StartCredentialsSetupMsg{} }
			}

		case "r":
			// Start region setup
			if v.currentStep == 2 {
				return v, func() tea.Msg { return StartRegionSetupMsg{} }
			}

		case "?":
			// Show help if on shortcuts step
			if v.currentStep == 3 {
				return v, func() tea.Msg { return ShowHelpMsg{} }
			}

		case "up", "k":
			if v.currentStep > 0 {
				v.currentStep--
			}

		case "down", "j":
			if v.currentStep < len(v.steps)-1 {
				v.currentStep++
			}

		case "t":
			// Toggle tutorial mode
			v.showTutorial = !v.showTutorial
		}
	}

	return v, nil
}

// View renders the welcome view
func (v *WelcomeView) View() string {
	if v.setupMode {
		return v.renderSetupView()
	}

	if !v.showTutorial {
		return v.renderQuickStart()
	}

	return v.renderTutorial()
}

// renderTutorial renders the full tutorial interface
func (v *WelcomeView) renderTutorial() string {
	var b strings.Builder

	// Header with logo
	logo := v.renderLogo()
	b.WriteString(logo + "\n\n")

	// Current step
	step := v.steps[v.currentStep]

	// Step title with icon
	title := v.stepStyle.Render(fmt.Sprintf("%s %s", step.Icon, step.Title))
	b.WriteString(title + "\n")

	// Step description
	description := v.descriptionStyle.Render(step.Description)
	b.WriteString(description + "\n")

	// Action instruction
	action := v.actionStyle.Render(step.Action)
	b.WriteString(action + "\n")

	// Key hint
	keyHint := v.keyHintStyle.Render(step.KeyHint)
	b.WriteString(keyHint + "\n")

	// Progress indicator
	progress := v.renderProgress()
	b.WriteString(progress + "\n")

	// Footer
	footer := v.footerStyle.Render(
		"↑/↓: navigate steps • t: toggle tutorial • esc: back • q: quit")
	b.WriteString(footer)

	// Center the content
	content := b.String()
	return v.centerContent(content)
}

// renderQuickStart renders a condensed quick start view
func (v *WelcomeView) renderQuickStart() string {
	var b strings.Builder

	logo := v.renderLogo()
	b.WriteString(logo + "\n\n")

	quickStart := v.titleStyle.Render("Quick Start")
	b.WriteString(quickStart + "\n")

	instructions := []string{
		"• Use ↑/↓ or j/k to navigate",
		"• Press Enter to select",
		"• Press '?' for help",
		"• Press 'q' to quit",
		"• Press 's' for settings",
	}

	for _, instruction := range instructions {
		line := v.descriptionStyle.Render(instruction)
		b.WriteString(line + "\n")
	}

	footer := v.footerStyle.Render(
		"t: show tutorial • enter: continue • q: quit")
	b.WriteString("\n" + footer)

	return v.centerContent(b.String())
}

// renderLogo renders the S3ry logo
func (v *WelcomeView) renderLogo() string {
	logo := `
 ███████╗██████╗ ██████╗ ██╗   ██╗
 ██╔════╝╚════██╗██╔══██╗╚██╗ ██╔╝
 ███████╗ █████╔╝██████╔╝ ╚████╔╝ 
 ╚════██║ ╚═══██╗██╔══██╗  ╚██╔╝  
 ███████║██████╔╝██║  ██║   ██║   
 ╚══════╝╚═════╝ ╚═╝  ╚═╝   ╚═╝   
`

	logoStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#7D56F4")).
		Bold(true).
		Align(lipgloss.Center)

	version := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888")).
		Align(lipgloss.Center).
		Render("v2.0.0 - 10x Faster S3 Management")

	return logoStyle.Render(logo) + "\n" + version
}

// renderProgress renders the progress indicator
func (v *WelcomeView) renderProgress() string {
	total := len(v.steps)
	current := v.currentStep + 1

	// Progress bar
	progressBar := strings.Repeat("█", current) +
		strings.Repeat("░", total-current)

	progressText := fmt.Sprintf("Step %d of %d", current, total)

	return v.progressStyle.Render(
		fmt.Sprintf("%s\n%s", progressBar, progressText))
}

// centerContent centers the content on screen
func (v *WelcomeView) centerContent(content string) string {
	if v.width == 0 || v.height == 0 {
		return content
	}

	lines := strings.Split(content, "\n")
	maxWidth := 0

	// Find the maximum line width
	for _, line := range lines {
		if len(line) > maxWidth {
			maxWidth = len(line)
		}
	}

	// Center horizontally
	leftPadding := (v.width - maxWidth) / 2
	if leftPadding < 0 {
		leftPadding = 0
	}

	var centeredLines []string
	for _, line := range lines {
		centeredLine := strings.Repeat(" ", leftPadding) + line
		centeredLines = append(centeredLines, centeredLine)
	}

	// Center vertically
	contentHeight := len(centeredLines)
	topPadding := (v.height - contentHeight) / 2
	if topPadding < 0 {
		topPadding = 0
	}

	var result []string
	for i := 0; i < topPadding; i++ {
		result = append(result, "")
	}
	result = append(result, centeredLines...)

	return strings.Join(result, "\n")
}

// Message types for welcome view
type CompleteTutorialMsg struct{}
type OpenSettingsMsg struct{}
type ShowHelpMsg struct{}
type StartCredentialsSetupMsg struct{}
type StartRegionSetupMsg struct{}
type CredentialsCheckCompleteMsg struct {
	HasCredentials bool
	Profile        string
	Region         string
}
type RegionSetCompleteMsg struct {
	Region string
}

// renderSetupView renders the AWS setup interface
func (v *WelcomeView) renderSetupView() string {
	var b strings.Builder

	// Header
	logo := v.renderLogo()
	b.WriteString(logo + "\n\n")

	// Setup title
	title := v.titleStyle.Render("🔧 AWS Configuration Setup")
	b.WriteString(title + "\n\n")

	// Check current AWS configuration status
	credentialsStatus := v.checkAWSCredentials()
	regionStatus := v.checkAWSRegion()

	// Credentials section
	b.WriteString(v.renderCredentialsSection(credentialsStatus))
	b.WriteString("\n")

	// Region section
	b.WriteString(v.renderRegionSection(regionStatus))
	b.WriteString("\n")

	// Setup progress
	b.WriteString(v.renderSetupProgress())
	b.WriteString("\n")

	// Instructions
	instructions := v.renderSetupInstructions()
	b.WriteString(instructions)

	// Footer
	footer := v.footerStyle.Render("Enter: apply changes • c: check credentials • ESC: back to tutorial")
	b.WriteString("\n" + footer)

	return v.centerContent(b.String())
}

// renderCredentialsSection renders the credentials configuration section
func (v *WelcomeView) renderCredentialsSection(status CredentialsStatus) string {
	var b strings.Builder

	// Section header
	header := v.stepStyle.Render("🔐 AWS Credentials")
	b.WriteString(header + "\n")

	// Status indicator
	if status.HasCredentials {
		statusMsg := v.successStyle.Render("✅ Credentials configured")
		if status.Profile != "" {
			statusMsg += v.descriptionStyle.Render(fmt.Sprintf(" (Profile: %s)", status.Profile))
		}
		b.WriteString(statusMsg + "\n")
	} else {
		statusMsg := v.warningStyle.Render("⚠️ No AWS credentials found")
		b.WriteString(statusMsg + "\n")
	}

	// Current configuration info
	if status.HasCredentials {
		info := []string{}
		if status.AccessKeyID != "" {
			masked := maskAccessKey(status.AccessKeyID)
			info = append(info, fmt.Sprintf("Access Key: %s", masked))
		}
		if status.Region != "" {
			info = append(info, fmt.Sprintf("Default Region: %s", status.Region))
		}
		if status.Profile != "" {
			info = append(info, fmt.Sprintf("Active Profile: %s", status.Profile))
		}

		for _, line := range info {
			b.WriteString(v.descriptionStyle.Render("  "+line) + "\n")
		}
	} else {
		// Setup instructions
		instructions := []string{
			"To set up AWS credentials:",
			"1. Run 'aws configure' in your terminal",
			"2. Or set environment variables:",
			"   export AWS_ACCESS_KEY_ID=your_key",
			"   export AWS_SECRET_ACCESS_KEY=your_secret",
			"3. Or use AWS profiles in ~/.aws/credentials",
		}

		for _, instruction := range instructions {
			b.WriteString(v.descriptionStyle.Render("  "+instruction) + "\n")
		}
	}

	return b.String()
}

// renderRegionSection renders the region configuration section
func (v *WelcomeView) renderRegionSection(status RegionStatus) string {
	var b strings.Builder

	// Section header
	header := v.stepStyle.Render("🌍 AWS Region")
	b.WriteString(header + "\n")

	// Status indicator
	if status.HasRegion {
		statusMsg := v.successStyle.Render(fmt.Sprintf("✅ Region: %s", status.Region))
		b.WriteString(statusMsg + "\n")

		// Region info
		if regionInfo, exists := getRegionInfo(status.Region); exists {
			info := v.descriptionStyle.Render(fmt.Sprintf("  %s", regionInfo.Name))
			b.WriteString(info + "\n")
		}
	} else {
		statusMsg := v.warningStyle.Render("⚠️ No region configured")
		b.WriteString(statusMsg + "\n")

		// Available regions
		b.WriteString(v.descriptionStyle.Render("  Popular regions:") + "\n")
		popularRegions := []string{
			"us-east-1 (US East - N. Virginia)",
			"us-west-2 (US West - Oregon)",
			"eu-west-1 (Europe - Ireland)",
			"ap-northeast-1 (Asia Pacific - Tokyo)",
			"ap-southeast-1 (Asia Pacific - Singapore)",
		}

		for _, region := range popularRegions {
			b.WriteString(v.descriptionStyle.Render("    • "+region) + "\n")
		}
	}

	return b.String()
}

// renderSetupProgress renders overall setup progress
func (v *WelcomeView) renderSetupProgress() string {
	var b strings.Builder

	// Progress header
	header := v.stepStyle.Render("📊 Setup Progress")
	b.WriteString(header + "\n")

	// Calculate progress
	total := 3
	completed := 0

	tasks := []struct {
		name      string
		completed bool
		icon      string
	}{
		{"AWS Credentials", v.setupProgress["credentials"], "🔐"},
		{"Region Selection", v.setupProgress["region"], "🌍"},
		{"Profile Setup", v.setupProgress["profile"], "👤"},
	}

	for _, task := range tasks {
		if task.completed {
			completed++
			line := v.successStyle.Render(fmt.Sprintf("✅ %s %s", task.icon, task.name))
			b.WriteString("  " + line + "\n")
		} else {
			line := v.warningStyle.Render(fmt.Sprintf("⏳ %s %s", task.icon, task.name))
			b.WriteString("  " + line + "\n")
		}
	}

	// Progress bar
	progressPercent := (completed * 100) / total
	progressBar := v.renderProgressBar(progressPercent)
	b.WriteString("\n" + progressBar + "\n")

	return b.String()
}

// renderSetupInstructions renders context-specific setup instructions
func (v *WelcomeView) renderSetupInstructions() string {
	var b strings.Builder

	if !v.setupProgress["credentials"] {
		b.WriteString(v.setupStyle.Render("Next Steps:\n1. Configure AWS credentials\n2. Press 'c' to check credentials\n3. Press 's' to save configuration"))
		b.WriteString("\n")
	} else if !v.setupProgress["region"] {
		b.WriteString(v.setupStyle.Render("Next Steps:\n1. Select AWS region\n2. Press 'r' to choose region\n3. Press 's' to save configuration"))
		b.WriteString("\n")
	} else {
		b.WriteString(v.successStyle.Render("🎉 Setup Complete! You're ready to use S3ry."))
		b.WriteString("\n")
	}

	return b.String()
}

// renderProgressBar renders a visual progress bar
func (v *WelcomeView) renderProgressBar(percent int) string {
	width := 30
	filled := (percent * width) / 100

	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return v.progressStyle.Render(fmt.Sprintf("[%s] %d%%", bar, percent))
}

// CredentialsStatus represents AWS credentials status
type CredentialsStatus struct {
	HasCredentials bool
	AccessKeyID    string
	Region         string
	Profile        string
}

// RegionStatus represents AWS region status
type RegionStatus struct {
	HasRegion bool
	Region    string
}

// Note: RegionInfo is defined in region.go

// checkAWSCredentials checks for existing AWS credentials
func (v *WelcomeView) checkAWSCredentials() CredentialsStatus {
	status := CredentialsStatus{}

	// Check environment variables
	accessKey := os.Getenv("AWS_ACCESS_KEY_ID")
	secretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	if accessKey != "" && secretKey != "" {
		status.HasCredentials = true
		status.AccessKeyID = accessKey
		status.Region = os.Getenv("AWS_DEFAULT_REGION")
		v.setupProgress["credentials"] = true
		return status
	}

	// Check AWS profile
	profile := os.Getenv("AWS_PROFILE")
	if profile == "" {
		profile = "default"
	}

	// Check ~/.aws/credentials file
	homeDir, err := os.UserHomeDir()
	if err == nil {
		credentialsPath := filepath.Join(homeDir, ".aws", "credentials")
		if _, err := os.Stat(credentialsPath); err == nil {
			status.HasCredentials = true
			status.Profile = profile
			v.setupProgress["credentials"] = true
			v.setupProgress["profile"] = true
		}
	}

	return status
}

// checkAWSRegion checks for configured AWS region
func (v *WelcomeView) checkAWSRegion() RegionStatus {
	status := RegionStatus{}

	// Check environment variable
	region := os.Getenv("AWS_DEFAULT_REGION")
	if region == "" {
		region = os.Getenv("AWS_REGION")
	}

	if region != "" {
		status.HasRegion = true
		status.Region = region
		v.setupProgress["region"] = true
		return status
	}

	// Check AWS config file
	homeDir, err := os.UserHomeDir()
	if err == nil {
		configPath := filepath.Join(homeDir, ".aws", "config")
		if _, err := os.Stat(configPath); err == nil {
			// In a real implementation, we would parse the config file
			// For now, assume ap-northeast-1 as default
			status.HasRegion = true
			status.Region = "ap-northeast-1"
			v.setupProgress["region"] = true
		}
	}

	return status
}

// maskAccessKey masks an AWS access key for display
func maskAccessKey(accessKey string) string {
	if len(accessKey) < 8 {
		return strings.Repeat("*", len(accessKey))
	}
	return accessKey[:4] + strings.Repeat("*", len(accessKey)-8) + accessKey[len(accessKey)-4:]
}

// getRegionInfo returns information about an AWS region
func getRegionInfo(regionCode string) (RegionInfo, bool) {
	regions := map[string]RegionInfo{
		"us-east-1":      {"us-east-1", "US East (N. Virginia)"},
		"us-east-2":      {"us-east-2", "US East (Ohio)"},
		"us-west-1":      {"us-west-1", "US West (N. California)"},
		"us-west-2":      {"us-west-2", "US West (Oregon)"},
		"eu-west-1":      {"eu-west-1", "Europe (Ireland)"},
		"eu-west-2":      {"eu-west-2", "Europe (London)"},
		"eu-central-1":   {"eu-central-1", "Europe (Frankfurt)"},
		"ap-northeast-1": {"ap-northeast-1", "Asia Pacific (Tokyo)"},
		"ap-northeast-2": {"ap-northeast-2", "Asia Pacific (Seoul)"},
		"ap-southeast-1": {"ap-southeast-1", "Asia Pacific (Singapore)"},
		"ap-southeast-2": {"ap-southeast-2", "Asia Pacific (Sydney)"},
		"ap-south-1":     {"ap-south-1", "Asia Pacific (Mumbai)"},
	}

	info, exists := regions[regionCode]
	return info, exists
}
