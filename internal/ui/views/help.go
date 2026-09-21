package views

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/seike460/s3ry/internal/ui/components"
)

// HelpView shows key bindings and usage information.
type HelpView struct {
	deps Deps
	list *components.List
}

// NewHelpView creates a new help view.
func NewHelpView(deps Deps) *HelpView {
	items := []components.ListItem{
		{Title: T("Navigation"), Tag: "Category"},
		{Title: "↑ / k", Description: T("Move cursor up"), Tag: "Key"},
		{Title: "↓ / j", Description: T("Move cursor down"), Tag: "Key"},
		{Title: "← / h", Description: T("Previous page"), Tag: "Key"},
		{Title: "→ / l", Description: T("Next page"), Tag: "Key"},
		{Title: "g", Description: T("Go to first item"), Tag: "Key"},
		{Title: "G", Description: T("Go to last item"), Tag: "Key"},
		{Title: T("Actions"), Tag: "Category"},
		{Title: "enter / space", Description: T("Select item"), Tag: "Key"},
		{Title: "esc", Description: T("Go back / cancel operation"), Tag: "Key"},
		{Title: "r", Description: T("Refresh current view"), Tag: "Key"},
		{Title: T("Application"), Tag: "Category"},
		{Title: "?", Description: T("Show this help"), Tag: "Key"},
		{Title: "s", Description: T("Show settings"), Tag: "Key"},
		{Title: "q", Description: T("Quit application"), Tag: "Key"},
		{Title: "ctrl+c", Description: T("Force quit"), Tag: "Key"},
		{Title: T("Operations"), Tag: "Category"},
		{Title: T("Download"), Description: T("Download selected S3 object to local file"), Tag: "Operation"},
		{Title: T("Upload"), Description: T("Upload local file to S3 bucket"), Tag: "Operation"},
		{Title: T("Delete"), Description: T("Delete selected S3 object"), Tag: "Operation"},
		{Title: T("Generate List"), Description: T("Create a list of all objects in bucket"), Tag: "Operation"},
	}

	return &HelpView{
		deps: deps,
		list: components.NewList(T("Help - s3ry S3 Browser"), items),
	}
}

// Init initializes the help view.
func (v *HelpView) Init() tea.Cmd {
	return nil
}

// Update handles messages for the help view.
func (v *HelpView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if v.list != nil {
			v.list, _ = v.list.Update(msg)
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return v, tea.Quit
		case "esc":
			return NewBucketView(v.deps), nil
		}

		if v.list != nil {
			v.list, _ = v.list.Update(msg)
		}
	}

	return v, nil
}

// View renders the help view.
func (v *HelpView) View() string {
	header := headerStyle.Render(T("s3ry - Interactive S3 Terminal Client"))

	description := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#626262")).
		Margin(1, 0).
		Render(T("Navigate your S3 buckets and objects with ease"))

	footer := footerStyle.Render(T("↑↓: navigate • esc: back • q: quit"))

	return header + "\n" + description + "\n\n" + v.list.View() + "\n" + footer
}
