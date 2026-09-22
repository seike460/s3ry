// Package app hosts the root Bubble Tea model and program startup.
package app

import (
	"context"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/seike460/s3ry/internal/config"
	"github.com/seike460/s3ry/internal/i18n"
	"github.com/seike460/s3ry/internal/s3"
	"github.com/seike460/s3ry/internal/ui/views"
)

// App is the root Bubble Tea model. It owns the view stack and the shared
// dependencies every view receives.
type App struct {
	deps views.Deps
	view tea.Model
}

// New creates the application model with the shared dependencies.
func New(deps views.Deps) *App {
	return &App{
		deps: deps,
		view: initialView(deps),
	}
}

// initialView picks the first screen. An s3:// start URL skips the bucket
// picker: prefixes open the object browser, bare buckets open the operation
// menu.
func initialView(deps views.Deps) tea.Model {
	if deps.StartURL == nil {
		return views.NewBucketView(deps)
	}
	if deps.StartURL.IsPrefix() {
		return views.NewObjectViewAt(deps, deps.StartURL.Bucket, deps.StartURL.Key, views.ModeDownload)
	}
	return views.NewOperationView(deps, deps.StartURL.Bucket)
}

// Init initializes the application.
func (a *App) Init() tea.Cmd {
	return a.view.Init()
}

// abortableView is implemented by views that own a background transfer.
// The app aborts the transfer before replacing or quitting the view so the
// worker goroutine and its progress broker are released.
type abortableView interface {
	AbortTransfer()
}

// abortView cancels any in-flight transfer owned by view.
func abortView(view tea.Model) {
	if v, ok := view.(abortableView); ok {
		v.AbortTransfer()
	}
}

// Update handles global shortcuts and delegates to the current view.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "ctrl+c":
			abortView(a.view)
			return a, tea.Quit
		case "ctrl+h", "f1":
			abortView(a.view)
			a.view = views.NewHelpView(a.deps)
			return a, a.view.Init()
		case "ctrl+s":
			abortView(a.view)
			a.view = views.NewSettingsView(a.deps)
			return a, a.view.Init()
		}
	}

	var cmd tea.Cmd
	oldView := a.view
	oldViewType := fmt.Sprintf("%T", a.view)
	a.view, cmd = a.view.Update(msg)
	newViewType := fmt.Sprintf("%T", a.view)

	if oldViewType != newViewType {
		abortView(oldView)
		if initCmd := a.view.Init(); initCmd != nil {
			cmd = tea.Batch(cmd, initCmd)
		}
	}

	return a, cmd
}

// View renders the current view.
func (a *App) View() string {
	return a.view.View()
}

// Run creates the AWS session from the resolved configuration and starts the
// Bubble Tea program.
func Run(ctx context.Context, cfg *config.Config) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if cfg == nil {
		cfg = config.Default()
	}

	session, err := s3.NewSession(ctx, s3.Options{
		Profile:       cfg.AWS.Profile,
		Region:        cfg.AWS.Region,
		EndpointURL:   cfg.AWS.Endpoint,
		PathStyle:     cfg.AWS.PathStyle,
		NoSignRequest: cfg.AWS.NoSignRequest,
		Concurrency:   cfg.Performance.Concurrency,
		PartSize:      cfg.Performance.PartSize,
	})
	if err != nil {
		return err
	}

	deps := views.Deps{
		Session:  session,
		Config:   cfg,
		Timeout:  time.Duration(cfg.Performance.Timeout) * time.Second,
		Messages: i18n.NewPrinter(cfg.UI.Language),
		StartURL: startURL(cfg),
	}

	options := []tea.ProgramOption{
		tea.WithInput(os.Stdin),
		tea.WithOutput(os.Stdout),
	}
	if isTTYAvailable() {
		options = append(options,
			tea.WithAltScreen(),
			tea.WithMouseCellMotion(),
		)
	}

	p := tea.NewProgram(New(deps), options...)
	_, err = p.Run()
	return err
}

// startURL re-parses the configured start URL. The CLI already validated it,
// so a parse error here is impossible and returns nil.
func startURL(cfg *config.Config) *s3.URL {
	if cfg.StartURL == "" {
		return nil
	}
	u, err := s3.ParseURL(cfg.StartURL)
	if err != nil {
		return nil
	}
	return &u
}

// isTTYAvailable reports whether alternate-screen and mouse support are safe
// to enable.
func isTTYAvailable() bool {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return false
	}
	_ = tty.Close()

	fileInfo, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}
