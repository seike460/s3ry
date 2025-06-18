package views

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/seike460/s3ry/internal/ui/components"
)

// OperationView represents the operation selection view
type OperationView struct {
	list   *components.List
	region string
	bucket string

	// Styles
	headerStyle lipgloss.Style
}

// NewOperationView creates a new operation view
func NewOperationView(region, bucket string) *OperationView {
	operations := []components.ListItem{
		{
			Title:       "📥 Download files",
			Description: "Download objects from S3 bucket to local storage (shortcut: d)",
			Tag:         "Download",
		},
		{
			Title:       "📤 Upload files",
			Description: "Upload local files to S3 bucket (shortcut: u)",
			Tag:         "Upload",
		},
		{
			Title:       "🗑️ Delete objects",
			Description: "Delete objects from S3 bucket (shortcut: delete)",
			Tag:         "Delete",
		},
		{
			Title:       "📋 Create object list",
			Description: "Generate a list of all objects in the bucket",
			Tag:         "List",
		},
		{
			Title:       "☁️ Cloud integration",
			Description: "View related AWS services and their status",
			Tag:         "Cloud",
		},
	}

	return &OperationView{
		region: region,
		bucket: bucket,
		list:   components.NewList("Select Operation", operations),

		headerStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			MarginBottom(1),
	}
}

// Init initializes the operation view
func (v *OperationView) Init() tea.Cmd {
	return nil
}

// Update handles messages for the operation view
func (v *OperationView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.list, _ = v.list.Update(msg)

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return v, tea.Quit
		case "esc":
			// Go back to bucket selection
			return NewBucketView(v.region), nil
		case "?":
			// Show help
			return NewHelpView(), nil
		case "s":
			// Show settings
			return NewSettingsView(), nil
		case "l":
			// Show logs
			return NewLogsView(), nil
		case "d":
			// Direct download shortcut
			return NewObjectView(v.region, v.bucket, "download"), nil
		case "u":
			// Direct upload shortcut
			return NewUploadView(v.region, v.bucket), nil
		case "delete":
			// Direct delete shortcut
			return NewObjectView(v.region, v.bucket, "delete"), nil
		case "enter", " ":
			selectedItem := v.list.GetCurrentItem()
			if selectedItem != nil {
				fmt.Printf("🔍 DEBUG: OperationView selected item: %s\n", selectedItem.Tag)
				switch selectedItem.Tag {
				case "Download":
					fmt.Printf("🔍 DEBUG: Creating ObjectView for download\n")
					return NewObjectView(v.region, v.bucket, "download"), nil
				case "Upload":
					return NewUploadView(v.region, v.bucket), nil
				case "Delete":
					return NewObjectView(v.region, v.bucket, "delete"), nil
				case "List":
					return NewListGeneratorView(v.region, v.bucket), nil
				case "Cloud":
					return NewCloudIntegrationView(v.region, v.bucket), nil
				}
			}
		}

		v.list, _ = v.list.Update(msg)
	}

	return v, nil
}

// View renders the operation view
func (v *OperationView) View() string {
	header := v.headerStyle.Render("⚡ Select Operation")
	context := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888")).
		Render("Region: " + v.region + " | Bucket: " + v.bucket)

	footer := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262")).
		Render("d: download • u: upload • delete: delete • ?: help • s: settings • l: logs • esc: back • q: quit")

	return header + "\n" + context + "\n\n" + v.list.View() + "\n\n" + footer
}
