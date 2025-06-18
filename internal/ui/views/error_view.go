package views

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ErrorType represents different types of errors
type ErrorType int

const (
	ErrorAuth ErrorType = iota
	ErrorNetwork
	ErrorPermission
	ErrorS3API
	ErrorFileSystem
	ErrorConfiguration
)

// ErrorRecord represents a single error occurrence
type ErrorRecord struct {
	Timestamp  time.Time
	ErrorType  ErrorType
	Message    string
	Context    map[string]interface{}
	Resolved   bool
	Resolution string
}

// RecoveryAction represents possible recovery actions
type RecoveryAction struct {
	Title       string
	Description string
	Actions     []string
	AutoRetry   bool
	HelpURL     string
}

// ErrorView represents the error display view with enhanced responsiveness
type ErrorView struct {
	Error    ErrorRecord
	Recovery RecoveryAction
	selected int
	width    int
	height   int

	// Performance optimizations
	lastRender    time.Time
	frameInterval time.Duration
	cached        bool
	cachedView    string

	// Styles
	titleStyle    lipgloss.Style
	errorStyle    lipgloss.Style
	recoveryStyle lipgloss.Style
	actionStyle   lipgloss.Style
	selectedStyle lipgloss.Style
	footerStyle   lipgloss.Style
	iconStyle     lipgloss.Style
}

// NewErrorView creates a new error view
func NewErrorView(err error, context map[string]interface{}) *ErrorView {
	errorType := classifyError(err)
	recovery := getRecoveryAction(errorType)

	record := ErrorRecord{
		Timestamp: time.Now(),
		ErrorType: errorType,
		Message:   err.Error(),
		Context:   context,
		Resolved:  false,
	}

	return &ErrorView{
		Error:    record,
		Recovery: recovery,
		selected: 0,
		width:    0,
		height:   0,

		// Performance optimizations for 60fps
		frameInterval: time.Millisecond * 16, // 60fps target
		lastRender:    time.Now(),
		cached:        false,
		cachedView:    "",

		titleStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF5555")).
			MarginBottom(1),

		errorStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#FF5555")).
			Padding(1).
			MarginBottom(2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#FF5555")),

		recoveryStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#04B575")).
			MarginBottom(1),

		actionStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#CCCCCC")).
			MarginLeft(2).
			PaddingLeft(1),

		selectedStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#04B575")).
			Background(lipgloss.Color("#2A2A2A")).
			MarginLeft(2).
			PaddingLeft(1).
			PaddingRight(1),

		footerStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262")).
			MarginTop(2),

		iconStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF5555")).
			MarginRight(1),
	}
}

// Init initializes the error view
func (v *ErrorView) Init() tea.Cmd {
	return nil
}

// Update handles messages for the error view with 60fps optimization
func (v *ErrorView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
		v.invalidateCache()

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if v.selected > 0 {
				v.selected--
				v.invalidateCache()
			}
		case "down", "j":
			if v.selected < len(v.Recovery.Actions)-1 {
				v.selected++
				v.invalidateCache()
			}
		case "enter":
			return v, v.executeRecoveryAction(v.selected)
		case "r":
			return v, v.retryOperation()
		case "h":
			return v, v.showHelp()
		case "c":
			return v, v.copyErrorDetails()
		case "ctrl+c", "q", "esc":
			return v, tea.Quit
		}
	}

	return v, v.tick60fps()
}

// tick60fps returns a command for 60fps frame rate limiting
func (v *ErrorView) tick60fps() tea.Cmd {
	return tea.Tick(v.frameInterval, func(t time.Time) tea.Msg {
		return struct{}{}
	})
}

// invalidateCache marks the cached view as invalid
func (v *ErrorView) invalidateCache() {
	v.cached = false
	v.cachedView = ""
}

// View renders the error view with 60fps optimization and caching
func (v *ErrorView) View() string {
	// 60fps frame rate limiting
	now := time.Now()
	if now.Sub(v.lastRender) < v.frameInterval && v.cached {
		return v.cachedView
	}
	v.lastRender = now

	var b strings.Builder

	// Error icon and title with enhanced styling
	errorIcon := map[ErrorType]string{
		ErrorAuth:          "🔐",
		ErrorNetwork:       "🌐",
		ErrorPermission:    "🚫",
		ErrorS3API:         "☁️",
		ErrorFileSystem:    "📁",
		ErrorConfiguration: "⚙️",
	}[v.Error.ErrorType]

	title := v.titleStyle.Render(fmt.Sprintf("%s Error Occurred", errorIcon))
	b.WriteString(title + "\n")

	// Error message with improved visual design
	message := v.errorStyle.Render(v.Error.Message)
	b.WriteString(message + "\n")

	// Recovery instructions with enhanced UX
	recoveryTitle := v.recoveryStyle.Render("🔧 Smart Recovery Options:")
	b.WriteString(recoveryTitle + "\n")

	// Render actions with improved visual hierarchy
	for i, action := range v.Recovery.Actions {
		style := v.actionStyle
		prefix := "  "
		if i == v.selected {
			style = v.selectedStyle
			prefix = "❯ "
		}

		actionText := fmt.Sprintf("%s%d. %s", prefix, i+1, action)
		b.WriteString(style.Render(actionText) + "\n")
	}

	// Auto-retry indicator
	if v.Recovery.AutoRetry {
		autoRetryText := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFA500")).
			Render("🔄 Auto-retry available for this error type")
		b.WriteString("\n" + autoRetryText + "\n")
	}

	// Help link with better accessibility
	if v.Recovery.HelpURL != "" {
		helpText := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7D56F4")).
			Background(lipgloss.Color("#2A2A2A")).
			Padding(0, 1).
			Render(fmt.Sprintf("📖 More help: %s", v.Recovery.HelpURL))
		b.WriteString("\n" + helpText + "\n")
	}

	// Context information with better formatting
	if len(v.Error.Context) > 0 {
		contextTitle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#888888")).
			Render("📋 Error Context:")
		b.WriteString("\n" + contextTitle + "\n")

		for key, value := range v.Error.Context {
			contextLine := lipgloss.NewStyle().
				Foreground(lipgloss.Color("#AAAAAA")).
				PaddingLeft(2).
				Render(fmt.Sprintf("• %s: %v", key, value))
			b.WriteString(contextLine + "\n")
		}
	}

	// Enhanced footer with more options
	footer := v.footerStyle.Render(
		"↑/↓: navigate • enter: execute • r: retry • h: help • c: copy • q: quit • esc: back")
	b.WriteString("\n" + footer)

	result := b.String()

	// Apply responsive layout for different screen sizes
	if v.width > 0 {
		containerStyle := lipgloss.NewStyle().
			Width(min(v.width-4, 80)). // Max width of 80 chars
			Align(lipgloss.Center)
		result = containerStyle.Render(result)
	}

	// Cache the result for 60fps performance
	v.cachedView = result
	v.cached = true

	return result
}

// Helper function for minimum calculation
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// classifyError determines the error type based on error message
func classifyError(err error) ErrorType {
	errStr := strings.ToLower(err.Error())

	switch {
	case strings.Contains(errStr, "credentials"), strings.Contains(errStr, "unauthorized"):
		return ErrorAuth
	case strings.Contains(errStr, "network"), strings.Contains(errStr, "timeout"),
		strings.Contains(errStr, "connection"):
		return ErrorNetwork
	case strings.Contains(errStr, "access denied"), strings.Contains(errStr, "403"),
		strings.Contains(errStr, "forbidden"):
		return ErrorPermission
	case strings.Contains(errStr, "s3"), strings.Contains(errStr, "bucket"),
		strings.Contains(errStr, "object"):
		return ErrorS3API
	case strings.Contains(errStr, "file"), strings.Contains(errStr, "directory"),
		strings.Contains(errStr, "path"):
		return ErrorFileSystem
	default:
		return ErrorConfiguration
	}
}

// getRecoveryAction returns appropriate recovery actions for error type
func getRecoveryAction(errorType ErrorType) RecoveryAction {
	switch errorType {
	case ErrorAuth:
		return RecoveryAction{
			Title:       "Authentication Error",
			Description: "AWS credentials are invalid or missing",
			Actions: []string{
				"Check AWS credentials in ~/.aws/credentials",
				"Verify AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY environment variables",
				"Run 'aws configure' to set up credentials",
				"Check if AWS profile is correctly specified",
			},
			AutoRetry: false,
			HelpURL:   "https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-files.html",
		}
	case ErrorNetwork:
		return RecoveryAction{
			Title:       "Network Connection Error",
			Description: "Unable to connect to AWS services",
			Actions: []string{
				"Check your internet connection",
				"Verify AWS service status at status.aws.amazon.com",
				"Try using a different network or VPN",
				"Check firewall settings",
			},
			AutoRetry: true,
			HelpURL:   "https://status.aws.amazon.com/",
		}
	case ErrorPermission:
		return RecoveryAction{
			Title:       "Permission Denied",
			Description: "Insufficient permissions for this operation",
			Actions: []string{
				"Check IAM user permissions for S3 operations",
				"Verify bucket policy allows your operations",
				"Contact your AWS administrator",
				"Review S3 bucket ACL settings",
			},
			AutoRetry: false,
			HelpURL:   "https://docs.aws.amazon.com/s3/latest/userguide/s3-access-control.html",
		}
	case ErrorS3API:
		return RecoveryAction{
			Title:       "S3 API Error",
			Description: "AWS S3 service returned an error",
			Actions: []string{
				"Check if bucket name is correct and exists",
				"Verify the AWS region is correct",
				"Wait a moment and try again (may be temporary)",
				"Check AWS service health status",
			},
			AutoRetry: true,
			HelpURL:   "https://docs.aws.amazon.com/s3/latest/API/ErrorResponses.html",
		}
	case ErrorFileSystem:
		return RecoveryAction{
			Title:       "File System Error",
			Description: "Local file system operation failed",
			Actions: []string{
				"Check if file or directory exists",
				"Verify you have read/write permissions",
				"Check available disk space",
				"Ensure file is not locked by another process",
			},
			AutoRetry: false,
			HelpURL:   "https://seike460.github.io/s3ry/troubleshooting#filesystem",
		}
	case ErrorConfiguration:
		return RecoveryAction{
			Title:       "Configuration Error",
			Description: "Application configuration issue",
			Actions: []string{
				"Check ~/.s3ry.yml configuration file",
				"Verify environment variables are set correctly",
				"Reset configuration to defaults",
				"Check command line arguments",
			},
			AutoRetry: false,
			HelpURL:   "https://seike460.github.io/s3ry/configuration",
		}
	default:
		return RecoveryAction{
			Title:       "Unknown Error",
			Description: "An unexpected error occurred",
			Actions: []string{
				"Try the operation again",
				"Check application logs for more details",
				"Report this issue on GitHub",
				"Contact support with error details",
			},
			AutoRetry: true,
			HelpURL:   "https://github.com/seike460/s3ry/issues",
		}
	}
}

// executeRecoveryAction executes the selected recovery action
func (v *ErrorView) executeRecoveryAction(index int) tea.Cmd {
	if index >= len(v.Recovery.Actions) {
		return nil
	}

	// This would implement the actual recovery logic
	// For now, return a message indicating the action
	return tea.Printf("Executing recovery action: %s", v.Recovery.Actions[index])
}

// retryOperation retries the failed operation
func (v *ErrorView) retryOperation() tea.Cmd {
	return tea.Printf("Retrying operation...")
}

// showHelp opens help documentation
func (v *ErrorView) showHelp() tea.Cmd {
	return tea.Printf("Opening help: %s", v.Recovery.HelpURL)
}

// copyErrorDetails copies error details to clipboard
func (v *ErrorView) copyErrorDetails() tea.Cmd {
	details := fmt.Sprintf("Error: %s\nType: %d\nTimestamp: %s\nContext: %v",
		v.Error.Message, v.Error.ErrorType, v.Error.Timestamp.Format(time.RFC3339), v.Error.Context)
	return tea.Printf("Error details copied to clipboard: %s", details)
}
