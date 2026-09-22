package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

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
		e.renderError(&s, err)
		s.WriteString("\n")
	}

	return s.String()
}

// renderError renders one entry: header, details, and the recovery hint.
func (e *ErrorDisplay) renderError(s *strings.Builder, err ErrorMsg) {
	levelStyle := e.getLevelStyle(err.Level)
	icon := e.getLevelIcon(err.Level)

	header := fmt.Sprintf("%s %s", icon, err.Title)
	s.WriteString(levelStyle.Render(header))

	// Timestamp for non-critical errors
	if err.Level < ErrorLevelCritical {
		timestamp := err.Timestamp.Format("15:04:05")
		s.WriteString(" ")
		s.WriteString(e.timestampStyle.Render(fmt.Sprintf("(%s)", timestamp)))
	}

	s.WriteString("\n")

	if err.Message != "" {
		s.WriteString(e.messageStyle.Render(err.Message))
		s.WriteString("\n")
	}
	if err.Suggestion != "" {
		s.WriteString(e.suggestionStyle.Render(err.Suggestion))
		s.WriteString("\n")
	}
	if e.showTechnical && err.Technical != "" {
		s.WriteString(e.technicalStyle.Render(fmt.Sprintf("Technical: %s", err.Technical)))
		s.WriteString("\n")
	}

	e.renderRecovery(s, err)
}

// renderRecovery renders the recovery actions or the fallback hint.
func (e *ErrorDisplay) renderRecovery(s *strings.Builder, err ErrorMsg) {
	if len(err.RecoveryActions) == 0 {
		if err.Recoverable {
			s.WriteString(e.suggestionStyle.Render("🔄 Press 'r' to retry • ⬅️ Press 'esc' to go back • ❓ Press '?' for help"))
		} else {
			s.WriteString(e.errorStyle.Render("⚠️ This error requires manual intervention - check AWS configuration"))
		}
		return
	}

	s.WriteString(e.suggestionStyle.Render("📋 Available Actions:"))
	s.WriteString("\n")
	for _, action := range err.RecoveryActions {
		actionText := fmt.Sprintf("  %s: %s (%s)", action.Shortcut, action.Label, action.Description)
		s.WriteString(e.messageStyle.Render(actionText))
		s.WriteString("\n")
	}
}

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
