// Package components provides the reusable Bubble Tea widgets used by the
// views: lists, spinners, progress bars, previews, and error displays.
package components

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// borderPadding reserves cells for the border and its padding inside the
// configured list width.
const borderPadding = 4

// ListItem represents a selectable item in a list
type ListItem struct {
	Title       string
	Description string
	Tag         string
	Data        any // Additional data for the item
}

// List represents a selectable list component with virtual scrolling
type List struct {
	title    string
	items    []ListItem
	cursor   int
	selected int
	width    int
	height   int
	showHelp bool

	// Virtual scrolling for performance
	viewportTop int
	maxVisible  int

	// Styles
	titleStyle    lipgloss.Style
	itemStyle     lipgloss.Style
	selectedStyle lipgloss.Style
	descStyle     lipgloss.Style
	tagStyle      lipgloss.Style
	helpStyle     lipgloss.Style
	borderStyle   lipgloss.Style
}

// NewList creates a new List component
func NewList(title string, items []ListItem) *List {
	l := &List{
		title:    title,
		items:    items,
		cursor:   0,
		selected: -1,
		showHelp: true,

		// Initialize virtual scrolling
		viewportTop: 0,
		maxVisible:  20,

		titleStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorAccent)).
			MarginLeft(1).
			MarginBottom(1),

		itemStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorBright)).
			PaddingLeft(2),

		selectedStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorText)).
			Background(lipgloss.Color(ColorAccent)).
			PaddingLeft(1).
			PaddingRight(1).
			Border(lipgloss.RoundedBorder(), false, false, false, true).
			BorderForeground(lipgloss.Color(ColorSuccess)),

		descStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorMuted)).
			PaddingLeft(4),

		tagStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorHighlight)).
			Background(lipgloss.Color(ColorOverlay)).
			PaddingLeft(1).
			PaddingRight(1).
			MarginLeft(2),

		helpStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorDisabled)).
			MarginTop(1).
			PaddingLeft(1),

		borderStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color(ColorBorder)).
			Padding(1),
	}

	return l
}

// Update handles messages for the list component
func (l *List) Update(msg tea.Msg) (*List, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		l.width = msg.Width
		l.height = msg.Height

		// Update viewport parameters for virtual scrolling
		l.updateViewport()

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if l.cursor > 0 {
				l.cursor--
				l.updateViewport()
			}
		case "down", "j":
			if l.cursor < len(l.items)-1 {
				l.cursor++
				l.updateViewport()
			}
		case "pgup", "ctrl+b":
			// Page up navigation
			l.cursor -= l.maxVisible
			if l.cursor < 0 {
				l.cursor = 0
			}
			l.updateViewport()
		case "pgdown", "ctrl+f":
			// Page down navigation
			l.cursor += l.maxVisible
			if l.cursor >= len(l.items) {
				l.cursor = len(l.items) - 1
			}
			l.updateViewport()
		case "enter", " ":
			l.selected = l.cursor
		case "home":
			l.cursor = 0
			l.updateViewport()
		case "end":
			l.cursor = len(l.items) - 1
			l.updateViewport()
		}
	}

	return l, nil
}

// View renders the list component
func (l *List) View() string {
	var s strings.Builder

	// Title
	s.WriteString(l.titleStyle.Render(l.title))
	s.WriteString("\n\n")

	// Early return for empty lists
	if len(l.items) == 0 {
		s.WriteString(l.helpStyle.Render("No items available"))
		if l.showHelp {
			s.WriteString("\n\n")
			s.WriteString(l.helpStyle.Render("No items to display"))
		}
		return s.String()
	}

	// Calculate visible range using virtual scrolling
	start := l.viewportTop
	end := l.viewportTop + l.maxVisible
	if end > len(l.items) {
		end = len(l.items)
	}

	// Simple synchronous rendering to fix duplicate selection issue
	for i := start; i < end; i++ {
		s.WriteString(l.renderItem(i))
	}

	// Show scrolling indicators
	l.renderScrollIndicators(&s, start, end)

	// Help text
	if l.showHelp {
		s.WriteString("\n")
		if len(l.items) > l.maxVisible {
			s.WriteString(l.helpStyle.Render("↑/↓: navigate • PgUp/PgDn: page • Home/End: jump • enter/space: select"))
		} else {
			s.WriteString(l.helpStyle.Render("↑/↓: navigate • enter/space: select • q: quit"))
		}
	}

	result := s.String()

	// Apply border if width is set
	if l.width > 0 {
		result = l.borderStyle.Width(l.width - borderPadding).Render(result)
	}

	return result
}

// GetCursor returns the current cursor position
func (l *List) GetCursor() int {
	return l.cursor
}

// GetSelected returns the selected item index (-1 if none selected)
func (l *List) GetSelected() int {
	return l.selected
}

// GetSelectedItem returns the selected item (nil if none selected)
func (l *List) GetSelectedItem() *ListItem {
	if l.selected >= 0 && l.selected < len(l.items) {
		return &l.items[l.selected]
	}
	return nil
}

// GetCurrentItem returns the item at cursor position
func (l *List) GetCurrentItem() *ListItem {
	if l.cursor >= 0 && l.cursor < len(l.items) {
		return &l.items[l.cursor]
	}
	return nil
}

// SetItems updates the list items
func (l *List) SetItems(items []ListItem) {
	l.items = items
	l.cursor = 0
	l.selected = -1
	l.viewportTop = 0
	l.updateViewport()
}

// SetShowHelp sets whether to show help text
func (l *List) SetShowHelp(show bool) {
	l.showHelp = show
}

// Reset resets the list state
func (l *List) Reset() {
	l.cursor = 0
	l.selected = -1
	l.viewportTop = 0
	l.updateViewport()
}
