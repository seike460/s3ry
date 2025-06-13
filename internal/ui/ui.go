package ui

import (
	"os"
	
	tea "github.com/charmbracelet/bubbletea"
	"github.com/seike460/s3ry/internal/config"
	"github.com/seike460/s3ry/internal/ui/app"
)

// Run starts the Bubble Tea UI application
func Run(cfg *config.Config) error {
	// Create the main application
	app := app.New(cfg)
	
	// Configure tea program options for better compatibility in automated environments
	options := []tea.ProgramOption{
		tea.WithInput(os.Stdin),
		tea.WithOutput(os.Stdout),
		// Skip AltScreen and mouse for compatibility
	}
	
	// Create and run the program
	p := tea.NewProgram(app, options...)
	
	// Run the program
	_, err := p.Run()
	return err
}