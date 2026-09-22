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
			s.WriteString(e.suggestionStyle.Render("🔄 Press 'r' to retry • ⬅️ Press 'esc' to go back • ❓ Press '?' for help"))
		} else {
			s.WriteString(e.errorStyle.Render("⚠️ This error requires manual intervention - check AWS configuration"))
		}

		s.WriteString("\n")
	}

	return s.String()
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
