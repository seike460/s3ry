// Package views contains the Bubble Tea screens: bucket selection,
// operation selection, object listing, upload, list generation, settings,
// and help. All views share the services carried by Deps.
package views

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/seike460/s3ry/internal/config"
	"github.com/seike460/s3ry/internal/i18n"
	"github.com/seike460/s3ry/internal/s3"
)

// Deps carries the services shared by every view.
type Deps struct {
	// Session is the AWS session created once at startup. It owns the
	// credential chain, per-region clients, and the transfer managers.
	Session *s3.Session
	// Config is the resolved application configuration.
	Config *config.Config
	// Timeout bounds each blocking S3 request made by a view. Zero applies
	// no extra deadline.
	Timeout time.Duration
}

// errNoSession is reported when a view is created without an AWS session.
var errNoSession = errors.New("s3 session is not configured")

// sessionErr returns errNoSession when the dependencies carry no session.
func (d Deps) sessionErr() error {
	if d.Session == nil {
		return errNoSession
	}
	return nil
}

// region returns the session's default region, or "-" without a session.
func (d Deps) region() string {
	if d.Session == nil {
		return "-"
	}
	return d.Session.Region()
}

// listContext returns a context bounded by the configured timeout.
func (d Deps) listContext(parent context.Context) (context.Context, context.CancelFunc) {
	if d.Timeout > 0 {
		return context.WithTimeout(parent, d.Timeout)
	}
	return context.WithCancel(parent)
}

// T localizes a message key. Keys are written in English and the catalog
// returns the translation for the active language.
func T(format string, args ...any) string {
	return i18n.Sprintf(format, args...)
}

var (
	headerStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4")).MarginBottom(1)
	contextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#888"))
	footerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#626262"))
	errorStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FF5555"))
)

// formatBytes formats a byte count as a human-readable string.
func formatBytes(bytes int64) string {
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

// truncateShort middle-truncates s so it fits width runes.
func truncateShort(s string, width int) string {
	runes := []rune(s)
	if len(runes) <= width {
		return s
	}
	if width <= 3 {
		return string(runes[:width])
	}
	return string(runes[:width-3]) + "..."
}
