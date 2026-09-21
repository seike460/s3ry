package views

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/seike460/s3ry/internal/ui/components"
)

// OperationView is the per-bucket operation selection view.
type OperationView struct {
	deps   Deps
	bucket string
	list   *components.List
}

// NewOperationView creates a new operation view.
func NewOperationView(deps Deps, bucket string) *OperationView {
	operations := []components.ListItem{
		{
			Title:       T("Download files"),
			Description: T("Download an object from the bucket (shortcut: d)"),
			Tag:         "Download",
		},
		{
			Title:       T("Upload files"),
			Description: T("Upload a local file to the bucket (shortcut: u)"),
			Tag:         "Upload",
		},
		{
			Title:       T("Delete objects"),
			Description: T("Delete an object from the bucket (shortcut: delete)"),
			Tag:         "Delete",
		},
		{
			Title:       T("Create object list"),
			Description: T("Export the bucket's object list to a local file"),
			Tag:         "List",
		},
	}

	return &OperationView{
		deps:   deps,
		bucket: bucket,
		list:   components.NewList(T("Select Operation"), operations),
	}
}

// Init initializes the operation view.
func (v *OperationView) Init() tea.Cmd {
	return nil
}

// Update handles messages for the operation view.
func (v *OperationView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.list, _ = v.list.Update(msg)

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return v, tea.Quit
		case "esc":
			return NewBucketView(v.deps), nil
		case "?":
			return NewHelpView(v.deps), nil
		case "s":
			return NewSettingsView(v.deps), nil
		case "d":
			return NewObjectView(v.deps, v.bucket, ModeDownload), nil
		case "u":
			return NewUploadView(v.deps, v.bucket), nil
		case "delete":
			return NewObjectView(v.deps, v.bucket, ModeDelete), nil
		case "enter", " ":
			if item := v.list.GetCurrentItem(); item != nil {
				switch item.Tag {
				case "Download":
					return NewObjectView(v.deps, v.bucket, ModeDownload), nil
				case "Upload":
					return NewUploadView(v.deps, v.bucket), nil
				case "Delete":
					return NewObjectView(v.deps, v.bucket, ModeDelete), nil
				case "List":
					return NewListGeneratorView(v.deps, v.bucket), nil
				}
			}
		}

		v.list, _ = v.list.Update(msg)
	}

	return v, nil
}

// View renders the operation view.
func (v *OperationView) View() string {
	header := headerStyle.Render(T("Select Operation"))
	context := contextStyle.Render(fmt.Sprintf("%s %s | %s %s",
		T("Region:"), v.deps.region(), T("Bucket:"), v.bucket))
	footer := footerStyle.Render(T("d: download • u: upload • delete: delete • ?: help • s: settings • esc: back • q: quit"))

	return header + "\n" + context + "\n\n" + v.list.View() + "\n\n" + footer
}
