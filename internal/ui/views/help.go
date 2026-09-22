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
		{Title: deps.T("Navigation"), Tag: "Category"},
		{Title: "↑ / k", Description: deps.T("Move cursor up"), Tag: "Key"},
		{Title: "↓ / j", Description: deps.T("Move cursor down"), Tag: "Key"},
		{Title: "pgup / ctrl+b", Description: deps.T("Previous page"), Tag: "Key"},
		{Title: "pgdown / ctrl+f", Description: deps.T("Next page"), Tag: "Key"},
		{Title: "home", Description: deps.T("Go to first item"), Tag: "Key"},
		{Title: "end", Description: deps.T("Go to last item"), Tag: "Key"},
		{Title: deps.T("Actions"), Tag: "Category"},
		{Title: "enter / space", Description: deps.T("Select item"), Tag: "Key"},
		{Title: "esc", Description: deps.T("Go back / cancel operation"), Tag: "Key"},
		{Title: "r", Description: deps.T("Refresh current view"), Tag: "Key"},
		{Title: deps.T("Application"), Tag: "Category"},
		{Title: "?", Description: deps.T("Show this help"), Tag: "Key"},
		{Title: "s", Description: deps.T("Show settings"), Tag: "Key"},
		{Title: "q", Description: deps.T("Quit application"), Tag: "Key"},
		{Title: "ctrl+c", Description: deps.T("Force quit"), Tag: "Key"},
		{Title: deps.T("Operations"), Tag: "Category"},
		{Title: deps.T("Download"), Description: deps.T("Download selected S3 object to local file"), Tag: "Operation"},
		{Title: deps.T("Upload"), Description: deps.T("Upload local file to S3 bucket"), Tag: "Operation"},
		{Title: deps.T("Delete"), Description: deps.T("Delete selected S3 object"), Tag: "Operation"},
		{Title: deps.T("Generate List"), Description: deps.T("Create a list of all objects in bucket"), Tag: "Operation"},
	}

	return &HelpView{
		deps: deps,
		list: components.NewList(deps.T("Help - s3ry S3 Browser"), items),
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
		if v.list != nil && v.list.Filtering() {
			v.list, _ = v.list.Update(msg)
			return v, nil
		}
		if quitKey(msg.String()) {
			return v, tea.Quit
		}
		switch msg.String() {
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
	header := headerStyle.Render(v.deps.T("s3ry - Interactive S3 Terminal Client"))

	description := lipgloss.NewStyle().
		Foreground(lipgloss.Color(components.ColorDisabled)).
		Margin(1, 0).
		Render(v.deps.T("Navigate your S3 buckets and objects with ease"))

	footer := footerStyle.Render(v.deps.T("↑↓: navigate • esc: back • q: quit"))

	return header + "\n" + description + "\n\n" + v.list.View() + "\n" + footer
}
