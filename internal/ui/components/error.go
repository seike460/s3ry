package components

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ErrorLevel represents the severity of an error
type ErrorLevel int

const (
	ErrorLevelInfo ErrorLevel = iota
	ErrorLevelWarning
	ErrorLevelError
	ErrorLevelCritical
)

// ErrorMsg represents an error message with context
type ErrorMsg struct {
	Level       ErrorLevel
	Title       string
	Message     string
	Suggestion  string
	Technical   string
	Timestamp   time.Time
	Recoverable bool
	RecoveryActions []RecoveryAction
}

// RecoveryAction represents a possible recovery action
type RecoveryAction struct {
	Label       string
	Description string
	Shortcut    string
	Action      func() error
}

// ErrorDisplay represents an enhanced error display component
type ErrorDisplay struct {
	errors      []ErrorMsg
	maxErrors   int
	showTechnical bool
	autoHide    bool
	hideAfter   time.Duration
	
	// Styles for different error levels
	infoStyle     lipgloss.Style
	warningStyle  lipgloss.Style
	errorStyle    lipgloss.Style
	criticalStyle lipgloss.Style
	titleStyle    lipgloss.Style
	messageStyle  lipgloss.Style
	suggestionStyle lipgloss.Style
	technicalStyle  lipgloss.Style
	timestampStyle  lipgloss.Style
}

// NewErrorDisplay creates a new ErrorDisplay component
func NewErrorDisplay() *ErrorDisplay {
	return &ErrorDisplay{
		errors:      make([]ErrorMsg, 0),
		maxErrors:   5, // Keep last 5 errors
		showTechnical: false,
		autoHide:    true,
		hideAfter:   time.Second * 10,
		
		infoStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#04B575")).
			Bold(true),
		
		warningStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFA500")).
			Bold(true),
		
		errorStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF5555")).
			Bold(true),
		
		criticalStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			Bold(true).
			Background(lipgloss.Color("#441111")).
			Padding(0, 1),
		
		titleStyle: lipgloss.NewStyle().
			Bold(true).
			Underline(true),
		
		messageStyle: lipgloss.NewStyle().
			MarginTop(1).
			MarginLeft(2),
		
		suggestionStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7D56F4")).
			MarginTop(1).
			MarginLeft(2).
			Italic(true),
		
		technicalStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888")).
			MarginTop(1).
			MarginLeft(2).
			Faint(true),
		
		timestampStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666")).
			Faint(true),
	}
}

// AddError adds a new error to the display
func (e *ErrorDisplay) AddError(level ErrorLevel, title, message, suggestion, technical string, recoverable bool) {
	e.AddErrorWithActions(level, title, message, suggestion, technical, recoverable, nil)
}

// AddErrorWithActions adds an error with recovery actions
func (e *ErrorDisplay) AddErrorWithActions(level ErrorLevel, title, message, suggestion, technical string, recoverable bool, actions []RecoveryAction) {
	errorMsg := ErrorMsg{
		Level:           level,
		Title:           title,
		Message:         message,
		Suggestion:      suggestion,
		Technical:       technical,
		Timestamp:       time.Now(),
		Recoverable:     recoverable,
		RecoveryActions: actions,
	}
	
	// Add to front of list
	e.errors = append([]ErrorMsg{errorMsg}, e.errors...)
	
	// Limit the number of stored errors
	if len(e.errors) > e.maxErrors {
		e.errors = e.errors[:e.maxErrors]
	}
}

// AddAWSError adds an AWS-specific error with intelligent suggestions
func (e *ErrorDisplay) AddAWSError(err error) {
	if err == nil {
		return
	}
	
	errStr := err.Error()
	var title, message, suggestion string
	var level ErrorLevel = ErrorLevelError
	var recoverable bool = true
	
	// Intelligent error categorization and suggestions
	switch {
	case strings.Contains(errStr, "NoCredentialsErr") || strings.Contains(errStr, "no credentials"):
		title = "AWS Credentials Not Found"
		message = "Unable to locate valid AWS credentials for authentication."
		suggestion = "💡 Run 'aws configure' or set AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY environment variables"
		
	case strings.Contains(errStr, "InvalidAccessKeyId"):
		title = "Invalid AWS Access Key"
		message = "The provided AWS access key ID is not valid."
		suggestion = "💡 Check your access key ID and run 'aws configure' to update credentials"
		
	case strings.Contains(errStr, "SignatureDoesNotMatch"):
		title = "Invalid AWS Secret Key"
		message = "The AWS secret access key doesn't match the access key ID."
		suggestion = "💡 Verify your secret access key and run 'aws configure' to update credentials"
		
	case strings.Contains(errStr, "TokenRefreshRequired"):
		title = "AWS Session Expired"
		message = "Your AWS session token has expired and needs to be refreshed."
		suggestion = "💡 Refresh your AWS session or re-run 'aws configure' if using temporary credentials"
		
	case strings.Contains(errStr, "RequestTimeTooSkewed"):
		title = "System Clock Incorrect"
		message = "Your system clock is not synchronized with AWS servers."
		suggestion = "💡 Synchronize your system time and try again"
		
	case strings.Contains(errStr, "AccessDenied"):
		title = "AWS Access Denied"
		message = "You don't have permission to perform this operation."
		suggestion = "💡 Check your IAM permissions for S3 access (ListBucket, GetObject, PutObject, DeleteObject)"
		
	case strings.Contains(errStr, "NoSuchBucket"):
		title = "S3 Bucket Not Found"
		message = "The specified S3 bucket does not exist or is not accessible."
		suggestion = "💡 Verify the bucket name and your access permissions to this bucket"
		
	case strings.Contains(errStr, "NoSuchKey"):
		title = "S3 Object Not Found"
		message = "The specified S3 object does not exist."
		suggestion = "💡 Check the object key and ensure it exists in the bucket"
		
	case strings.Contains(errStr, "BucketNotEmpty"):
		title = "S3 Bucket Not Empty"
		message = "Cannot delete a bucket that contains objects."
		suggestion = "💡 Delete all objects in the bucket first, then try deleting the bucket again"
		
	case strings.Contains(errStr, "network") || strings.Contains(errStr, "timeout") || strings.Contains(errStr, "connection"):
		title = "Network Connection Error"
		message = "Unable to connect to AWS services."
		suggestion = "🌐 Check your internet connection and try again. Consider using a different region if problems persist"
		
	case strings.Contains(errStr, "TooManyRequests") || strings.Contains(errStr, "RequestLimitExceeded"):
		title = "AWS Rate Limit Exceeded"
		message = "Too many requests sent to AWS in a short time."
		suggestion = "⏳ Wait a moment and try again. Consider reducing concurrent operations"
		level = ErrorLevelWarning
		
	default:
		title = "AWS Operation Failed"
		message = fmt.Sprintf("An AWS operation failed: %s", errStr)
		suggestion = "💡 Check AWS status page and your configuration. Contact support if the problem persists"
	}
	
	e.AddError(level, title, message, suggestion, errStr, recoverable)
}

// AddNetworkError adds a network-specific error
func (e *ErrorDisplay) AddNetworkError(operation string, err error) {
	title := fmt.Sprintf("Network Error: %s", operation)
	message := "A network error occurred while performing the operation."
	suggestion := "🌐 Check your internet connection and try again"
	
	if err != nil {
		e.AddError(ErrorLevelError, title, message, suggestion, err.Error(), true)
	} else {
		e.AddError(ErrorLevelError, title, message, suggestion, "", true)
	}
}

// AddValidationError adds a validation error
func (e *ErrorDisplay) AddValidationError(field, value, requirement string) {
	title := "Input Validation Error"
	message := fmt.Sprintf("Invalid value for %s: '%s'", field, value)
	suggestion := fmt.Sprintf("💡 %s", requirement)
	
	e.AddError(ErrorLevelWarning, title, message, suggestion, "", true)
}

// Update handles messages for the error display
func (e *ErrorDisplay) Update(msg tea.Msg) (*ErrorDisplay, tea.Cmd) {
	// Auto-hide old errors if enabled
	if e.autoHide {
		now := time.Now()
		filtered := make([]ErrorMsg, 0)
		
		for _, err := range e.errors {
			if now.Sub(err.Timestamp) < e.hideAfter || err.Level >= ErrorLevelError {
				filtered = append(filtered, err)
			}
		}
		
		e.errors = filtered
	}
	
	return e, nil
}

// View renders the error display
func (e *ErrorDisplay) View() string {
	if len(e.errors) == 0 {
		return ""
	}
	
	var s strings.Builder
	
	for i, err := range e.errors {
		if i > 0 {
			s.WriteString("\n")
		}
		
		// Render error based on level
		levelStyle := e.getLevelStyle(err.Level)
		icon := e.getLevelIcon(err.Level)
		
		// Header with icon and title
		header := fmt.Sprintf("%s %s", icon, err.Title)
		s.WriteString(levelStyle.Render(header))
		
		// Timestamp for non-critical errors
		if err.Level < ErrorLevelCritical {
			timestamp := err.Timestamp.Format("15:04:05")
			s.WriteString(" ")
			s.WriteString(e.timestampStyle.Render(fmt.Sprintf("(%s)", timestamp)))
		}
		
		s.WriteString("\n")
		
		// Message
		if err.Message != "" {
			s.WriteString(e.messageStyle.Render(err.Message))
			s.WriteString("\n")
		}
		
		// Suggestion
		if err.Suggestion != "" {
			s.WriteString(e.suggestionStyle.Render(err.Suggestion))
			s.WriteString("\n")
		}
		
		// Technical details (if enabled)
		if e.showTechnical && err.Technical != "" {
			s.WriteString(e.technicalStyle.Render(fmt.Sprintf("Technical: %s", err.Technical)))
			s.WriteString("\n")
		}
		
		// Recovery actions
		if len(err.RecoveryActions) > 0 {
			s.WriteString(e.suggestionStyle.Render("📋 Available Actions:"))
			s.WriteString("\n")
			for _, action := range err.RecoveryActions {
				actionText := fmt.Sprintf("  %s: %s (%s)", action.Shortcut, action.Label, action.Description)
				s.WriteString(e.messageStyle.Render(actionText))
				s.WriteString("\n")
			}
		} else if err.Recoverable {
			s.WriteString(e.suggestionStyle.Render("Press 'r' to retry or 'esc' to go back"))
		} else {
			s.WriteString(e.errorStyle.Render("This error requires manual intervention"))
		}
		
		s.WriteString("\n")
	}
	
	return s.String()
}

// Helper methods
func (e *ErrorDisplay) getLevelStyle(level ErrorLevel) lipgloss.Style {
	switch level {
	case ErrorLevelInfo:
		return e.infoStyle
	case ErrorLevelWarning:
		return e.warningStyle
	case ErrorLevelError:
		return e.errorStyle
	case ErrorLevelCritical:
		return e.criticalStyle
	default:
		return e.errorStyle
	}
}

func (e *ErrorDisplay) getLevelIcon(level ErrorLevel) string {
	switch level {
	case ErrorLevelInfo:
		return "ℹ️"
	case ErrorLevelWarning:
		return "⚠️"
	case ErrorLevelError:
		return "❌"
	case ErrorLevelCritical:
		return "🚨"
	default:
		return "❌"
	}
}

// Configuration methods
func (e *ErrorDisplay) SetShowTechnical(show bool) {
	e.showTechnical = show
}

func (e *ErrorDisplay) SetAutoHide(autoHide bool, duration time.Duration) {
	e.autoHide = autoHide
	e.hideAfter = duration
}

func (e *ErrorDisplay) SetMaxErrors(max int) {
	e.maxErrors = max
	if len(e.errors) > max {
		e.errors = e.errors[:max]
	}
}

// GetErrorCount returns the number of active errors
func (e *ErrorDisplay) GetErrorCount() int {
	return len(e.errors)
}

// GetLatestError returns the most recent error
func (e *ErrorDisplay) GetLatestError() *ErrorMsg {
	if len(e.errors) > 0 {
		return &e.errors[0]
	}
	return nil
}

// ClearErrors removes all errors
func (e *ErrorDisplay) ClearErrors() {
	e.errors = make([]ErrorMsg, 0)
}

// HasCriticalErrors returns true if any critical errors are present
func (e *ErrorDisplay) HasCriticalErrors() bool {
	for _, err := range e.errors {
		if err.Level == ErrorLevelCritical {
			return true
		}
	}
	return false
}