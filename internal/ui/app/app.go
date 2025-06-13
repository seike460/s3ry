package app

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/seike460/s3ry/internal/config"
	"github.com/seike460/s3ry/internal/ui/views"
)

// AppState represents the current state of the application
type AppState int

const (
	StateInit AppState = iota
	StateRegionSelection
	StateBucketSelection
	StateOperationSelection
	StateObjectSelection
	StateUploading
	StateDownloading
	StateDeleting
	StateCreatingList
	StateError
	StateExit
)

// App represents the main application
type App struct {
	config *config.Config
	view   tea.Model
	
	// Styles
	titleStyle lipgloss.Style
	errorStyle lipgloss.Style
}

// New creates a new App instance with the given configuration
func New(cfg *config.Config) *App {
	app := &App{
		config: cfg,
		
		// Initialize styles
		titleStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			MarginLeft(2).
			MarginTop(1),
		
		errorStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF5555")).
			MarginLeft(2),
	}
	
	// Initialize with region view, using configured region if available
	if cfg.AWS.Region != "" {
		// If region is pre-configured, skip region selection and go directly to bucket selection
		app.view = views.NewBucketView(cfg.AWS.Region)
	} else {
		// Start with region selection
		app.view = views.NewRegionView()
	}
	
	return app
}

// Init initializes the application
func (a *App) Init() tea.Cmd {
	return a.view.Init()
}

// Update handles messages and updates the model
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Handle global keyboard shortcuts
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "ctrl+c":
			return a, tea.Quit
		case "ctrl+h", "f1":
			// Global help shortcut
			a.view = views.NewHelpView()
			return a, a.view.Init()
		case "ctrl+s":
			// Global settings shortcut
			a.view = views.NewSettingsView()
			return a, a.view.Init()
		case "ctrl+l":
			// Global logs shortcut
			a.view = views.NewLogsView()
			return a, a.view.Init()
		}
	}
	
	// Delegate to current view
	var cmd tea.Cmd
	a.view, cmd = a.view.Update(msg)
	
	return a, cmd
}


// View renders the current state of the application
func (a *App) View() string {
	return a.view.View()
}

// Run starts the Bubble Tea application with the given configuration
func Run(cfg *config.Config) error {
	app := New(cfg)
	p := tea.NewProgram(app, tea.WithAltScreen())
	_, err := p.Run()
	return err
}