package main

import (
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/seike460/s3ry/internal/i18n"
	"github.com/seike460/s3ry/internal/ui/views"
)

func main() {
	// Initialize i18n system
	i18n.Init()
	
	// Create the initial view (region selection)
	initialView := views.NewRegionView()
	
	// Create the Bubble Tea program
	p := tea.NewProgram(
		initialView,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	
	// Run the program
	if _, err := p.Run(); err != nil {
		log.Printf("Error running program: %v", err)
		os.Exit(1)
	}
}