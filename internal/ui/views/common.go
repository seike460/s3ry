// Package views contains the Bubble Tea screens: bucket selection,
// operation selection, object listing, upload, list generation, settings,
// and help. All views share the services carried by Deps.
package views

import (
	"context"
	"errors"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/seike460/s3ry/internal/config"
	"github.com/seike460/s3ry/internal/i18n"
	"github.com/seike460/s3ry/internal/s3"
	"github.com/seike460/s3ry/internal/ui/components"
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
	// Messages formats localized UI text. Nil falls back to English.
	Messages *i18n.Printer
	// StartURL, when set, skips the bucket picker and opens at this location.
	StartURL *s3.URL
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
// returns the translation for the active language. A nil Messages falls back
// to English.
func (d Deps) T(format string, args ...any) string {
	if d.Messages == nil {
		return i18n.NewPrinter("en").Sprintf(format, args...)
	}
	return d.Messages.Sprintf(format, args...)
}

var (
	headerStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(components.ColorAccent)).MarginBottom(1)
	contextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(components.ColorMuted))
	footerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color(components.ColorDisabled))
	errorStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(components.ColorDanger))
)

// errorList builds the single-item list shown when a load fails. The
// "Error" tag marks the item so pressing enter retries the load.
func errorList(d Deps, title, message string) *components.List {
	return components.NewList(title, []components.ListItem{{
		Title:       message,
		Description: d.T("Press 'r' to retry, 'esc' to go back, or 'q' to quit"),
		Tag:         "Error",
	}})
}

// listState is the shared scaffold for views that load a listing into a
// selectable list: the spinner shown while loading, the error display, the
// retryable error item, and the loading flag. Embedding it keeps each view
// limited to its own data mapping and selection behavior.
type listState struct {
	list    *components.List
	spinner *components.Spinner
	errors  *components.ErrorDisplay
	loading bool
}

func newListState(loadMessage string) listState {
	return listState{
		loading: true,
		spinner: components.NewSpinner(loadMessage),
		errors:  components.NewErrorDisplay(),
	}
}

// startLoading switches back to the loading state and returns the spinner
// plus load commands.
func (s *listState) startLoading(retryMessage string, load tea.Cmd) tea.Cmd {
	s.loading = true
	s.errors.ClearErrors()
	s.spinner = components.NewSpinner(retryMessage)
	return tea.Batch(s.spinner.Start(), load)
}

// fail stops loading, records the error, and shows the retryable error list.
func (s *listState) fail(d Deps, title, message string, err error) {
	s.loading = false
	s.spinner.Stop()
	s.errors.AddAWSError(err)
	s.list = errorList(d, title, message)
}

// loaded stops loading and installs the freshly built item list.
func (s *listState) loaded(title string, items []components.ListItem) {
	s.loading = false
	s.spinner.Stop()
	s.list = components.NewList(title, items)
}

// resize forwards a window size change to the list, when it exists.
func (s *listState) resize(msg tea.WindowSizeMsg) {
	if s.list != nil {
		s.list, _ = s.list.Update(msg)
	}
}

// onTick animates the spinner while a load is in flight.
func (s *listState) onTick(msg components.SpinnerTickMsg) tea.Cmd {
	if !s.loading {
		return nil
	}
	s.spinner, _ = s.spinner.Update(msg)
	return s.spinner.Start()
}

// retryRequested reports whether a keypress asks to reload: r, or enter on
// the error item.
func (s *listState) retryRequested(key string) bool {
	if key == "r" {
		return true
	}
	if key != "enter" && key != " " {
		return false
	}
	item := s.currentItem()
	return item != nil && item.Tag == "Error"
}

func (s *listState) currentItem() *components.ListItem {
	if s.list == nil {
		return nil
	}
	return s.list.GetCurrentItem()
}

// quitKey reports whether key asks to quit the program.
func quitKey(key string) bool {
	return key == "ctrl+c" || key == "q"
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
