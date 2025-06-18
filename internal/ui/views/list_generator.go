package views

import (
	"context"
	"fmt"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/seike460/s3ry/internal/s3"
	"github.com/seike460/s3ry/internal/ui/components"
	"github.com/seike460/s3ry/pkg/interfaces"
)

// ListGeneratorView represents the object list generator view
type ListGeneratorView struct {
	spinner    *components.Spinner
	progress   *components.Progress
	processing bool
	region     string
	bucket     string
	s3Client   interfaces.S3Client

	// Styles
	headerStyle lipgloss.Style
	errorStyle  lipgloss.Style
}

// NewListGeneratorView creates a new list generator view
func NewListGeneratorView(region, bucket string) *ListGeneratorView {
	// Create S3 client using the new architecture
	s3Client := s3.NewClient(region)

	return &ListGeneratorView{
		region:     region,
		bucket:     bucket,
		s3Client:   s3Client,
		processing: true,
		spinner:    components.NewSpinner("Generating object list..."),

		headerStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			MarginBottom(2),

		errorStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF5555")).
			MarginTop(1),
	}
}

// Init initializes the list generator view
func (v *ListGeneratorView) Init() tea.Cmd {
	return tea.Batch(
		v.spinner.Start(),
		v.generateList(),
	)
}

// Update handles messages for the list generator view
func (v *ListGeneratorView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if v.progress != nil {
			v.progress, _ = v.progress.Update(msg)
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return v, tea.Quit
		case "esc":
			if !v.processing {
				// Go back to operation selection
				return NewOperationView(v.region, v.bucket), nil
			}
		}

	case components.SpinnerTickMsg:
		if v.processing && v.spinner.IsActive() {
			v.spinner, _ = v.spinner.Update(msg)
			cmds = append(cmds, v.spinner.Start())
		}

	case components.ProgressMsg:
		if v.progress != nil {
			v.progress, _ = v.progress.Update(msg)
		}

	case components.CompletedMsg:
		if v.progress != nil {
			v.progress, _ = v.progress.Update(msg)
			v.processing = false
			v.spinner.Stop()
			// Wait a moment to show completion, then allow going back
			return v, tea.Tick(time.Second*3, func(t time.Time) tea.Msg {
				return tea.KeyMsg{Type: tea.KeyEsc}
			})
		}
	}

	return v, tea.Batch(cmds...)
}

// View renders the list generator view
func (v *ListGeneratorView) View() string {
	context := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888")).
		Render("Region: " + v.region + " | Bucket: " + v.bucket)

	var content string
	if v.progress != nil {
		content = v.progress.View()
	} else {
		content = v.headerStyle.Render("📝 Generating Object List") + "\n\n" + v.spinner.View()
	}

	help := ""
	if !v.processing {
		help = "\n\n" + lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262")).
			Render("esc: back • q: quit")
	}

	return context + "\n\n" + content + help
}

// generateList generates the object list file
func (v *ListGeneratorView) generateList() tea.Cmd {
	return func() tea.Msg {
		// Use interface method for MVP
		ctx := context.Background()

		// Create filename with timestamp
		t := time.Now()
		filename := fmt.Sprintf("ObjectList-%s.txt", t.Format("2006-01-02-15-04-05"))

		// Create file
		file, err := os.Create(filename)
		if err != nil {
			return components.CompletedMsg{
				Success: false,
				Message: fmt.Sprintf("Failed to create file: %v", err),
			}
		}
		defer file.Close()

		// List objects using interface method for MVP
		objects, err := v.s3Client.ListObjects(ctx, v.bucket, "", "")
		if err != nil {
			return components.CompletedMsg{
				Success: false,
				Message: fmt.Sprintf("Failed to list objects: %v", err),
			}
		}

		// Create progress tracker
		totalObjects := int64(len(objects))
		v.progress = components.NewProgress("📝 Writing object list", totalObjects)

		// Generate list with progress
		for i, obj := range objects {
			if !obj.IsPrefix { // Skip directories
				line := fmt.Sprintf("./%s,%d\n", obj.Key, obj.Size)
				_, writeErr := file.WriteString(line)
				if writeErr != nil {
					return components.CompletedMsg{
						Success: false,
						Message: fmt.Sprintf("Failed to write to file: %v", writeErr),
					}
				}

				processedObjects := int64(i + 1)
				// Update progress every 100 objects to avoid too many updates
				if processedObjects%100 == 0 || processedObjects == totalObjects {
					v.progress.SetProgress(processedObjects, totalObjects,
						fmt.Sprintf("Processed %d of %d objects", processedObjects, totalObjects))
				}
			}
		}

		return components.CompletedMsg{
			Success: true,
			Message: fmt.Sprintf("Object list created: %s (%d objects)", filename, totalObjects),
		}
	}
}
