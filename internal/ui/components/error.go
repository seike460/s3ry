package components

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	// maxStoredErrors bounds the retained error history so a long session
	// cannot grow the display unboundedly.
	maxStoredErrors = 5
	// errorAutoHideDelay keeps transient errors visible long enough to read
	// without requiring a manual dismiss.
	errorAutoHideDelay = 10 * time.Second
)

// ErrorLevel represents the severity of an error
type ErrorLevel int

// Error levels ordered by severity.
const (
	ErrorLevelInfo ErrorLevel = iota
	ErrorLevelWarning
	ErrorLevelError
	ErrorLevelCritical
)

// ErrorMsg represents an error message with context
type ErrorMsg struct {
	Level           ErrorLevel
	Title           string
	Message         string
	Suggestion      string
	Technical       string
	Timestamp       time.Time
	Recoverable     bool
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
	errors        []ErrorMsg
	maxErrors     int
	showTechnical bool
	autoHide      bool
	hideAfter     time.Duration

	// Styles for different error levels
	infoStyle       lipgloss.Style
	warningStyle    lipgloss.Style
	errorStyle      lipgloss.Style
	criticalStyle   lipgloss.Style
	titleStyle      lipgloss.Style
	messageStyle    lipgloss.Style
	suggestionStyle lipgloss.Style
	technicalStyle  lipgloss.Style
	timestampStyle  lipgloss.Style
}

// NewErrorDisplay creates a new ErrorDisplay component
func NewErrorDisplay() *ErrorDisplay {
	return &ErrorDisplay{
		errors:        make([]ErrorMsg, 0),
		maxErrors:     maxStoredErrors,
		showTechnical: false,
		autoHide:      true,
		hideAfter:     errorAutoHideDelay,

		infoStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorSuccess)).
			Bold(true),

		warningStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorWarning)).
			Bold(true),

		errorStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorDanger)).
			Bold(true),

		criticalStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorCritical)).
			Bold(true).
			Background(lipgloss.Color(ColorDangerBg)).
			Padding(0, 1),

		titleStyle: lipgloss.NewStyle().
			Bold(true).
			Underline(true),

		messageStyle: lipgloss.NewStyle().
			MarginTop(1).
			MarginLeft(2),

		suggestionStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorAccent)).
			MarginTop(1).
			MarginLeft(2).
			Italic(true),

		technicalStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorMuted)).
			MarginTop(1).
			MarginLeft(2).
			Faint(true),

		timestampStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorFaint)).
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
func (e *ErrorDisplay) Update(_ tea.Msg) (*ErrorDisplay, tea.Cmd) {
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

// SetShowTechnical toggles the technical details section.
func (e *ErrorDisplay) SetShowTechnical(show bool) {
	e.showTechnical = show
}

// SetAutoHide toggles automatic hiding of old non-critical errors.
func (e *ErrorDisplay) SetAutoHide(autoHide bool, duration time.Duration) {
	e.autoHide = autoHide
	e.hideAfter = duration
}

// SetMaxErrors caps how many errors are retained.
func (e *ErrorDisplay) SetMaxErrors(limit int) {
	e.maxErrors = limit
	if len(e.errors) > limit {
		e.errors = e.errors[:limit]
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
