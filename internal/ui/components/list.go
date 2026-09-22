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

// defaultMaxVisible is the viewport height used before a WindowSizeMsg
// arrives.
const defaultMaxVisible = 20

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
	items    []ListItem // filtered view of allItems
	allItems []ListItem
	cursor   int
	selected int
	width    int
	height   int
	showHelp bool

	// "/" opens the filter input; filtering narrows items by substring.
	filter    string
	filtering bool

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
		allItems: items,
		cursor:   0,
		selected: -1,
		showHelp: true,

		// Initialize virtual scrolling
		viewportTop: 0,
		maxVisible:  defaultMaxVisible,

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
		if l.filtering {
			l.onFilterKey(msg)
		} else {
			l.onKey(msg.String())
		}
	}

	return l, nil
}

// Filtering reports whether the filter input is capturing keystrokes.
// Views route every key to the list while this is true.
func (l *List) Filtering() bool {
	return l.filtering
}

// Filter returns the active filter text.
func (l *List) Filter() string {
	return l.filter
}

// onFilterKey edits the filter while input capture is active.
func (l *List) onFilterKey(msg tea.KeyMsg) {
	switch msg.Type {
	case tea.KeyEnter:
		l.filtering = false
	case tea.KeyEscape:
		l.filtering = false
		l.filter = ""
		l.applyFilter()
	case tea.KeyBackspace:
		if r := []rune(l.filter); len(r) > 0 {
			l.filter = string(r[:len(r)-1])
		}
		l.applyFilter()
	case tea.KeySpace:
		l.filter += " "
		l.applyFilter()
	case tea.KeyRunes:
		l.filter += string(msg.Runes)
		l.applyFilter()
	}
}

// applyFilter rebuilds the visible items from the filter text.
func (l *List) applyFilter() {
	l.items = filterItems(l.allItems, l.filter)
	l.cursor = 0
	l.selected = -1
	l.viewportTop = 0
	l.updateViewport()
}

// filterItems returns items whose title or description contains filter.
func filterItems(items []ListItem, filter string) []ListItem {
	if filter == "" {
		return items
	}
	needle := strings.ToLower(filter)
	var out []ListItem
	for _, item := range items {
		if strings.Contains(strings.ToLower(item.Title), needle) ||
			strings.Contains(strings.ToLower(item.Description), needle) {
			out = append(out, item)
		}
	}
	return out
}

// onKey moves the cursor or selects for navigation keys.
func (l *List) onKey(key string) {
	if l.moveKey(key) {
		return
	}
	switch key {
	case "enter", " ":
		l.selected = l.cursor
	case "/":
		l.filtering = true
	}
}

// moveKey applies cursor-movement keys and reports whether key was one.
func (l *List) moveKey(key string) bool {
	switch key {
	case "up", "k":
		l.moveCursor(-1)
	case "down", "j":
		l.moveCursor(1)
	case "pgup", "ctrl+b":
		l.moveCursor(-l.maxVisible)
	case "pgdown", "ctrl+f":
		l.moveCursor(l.maxVisible)
	case "home":
		l.cursor = 0
		l.updateViewport()
	case "end":
		l.cursor = len(l.items) - 1
		l.updateViewport()
	default:
		return false
	}
	return true
}

// moveCursor moves the cursor by delta, clamped to the item bounds.
func (l *List) moveCursor(delta int) {
	maxCursor := len(l.items) - 1
	if maxCursor < 0 {
		maxCursor = 0
	}
	l.cursor += delta
	if l.cursor < 0 {
		l.cursor = 0
	}
	if l.cursor > maxCursor {
		l.cursor = maxCursor
	}
	l.updateViewport()
}

// View renders the list component
func (l *List) View() string {
	var s strings.Builder
	l.renderTitle(&s)

	// Early return for empty lists
	if len(l.items) == 0 {
		l.renderEmpty(&s)
		return s.String()
	}

	// Calculate visible range using virtual scrolling
	start := l.viewportTop
	end := min(l.viewportTop+l.maxVisible, len(l.items))

	// Simple synchronous rendering to fix duplicate selection issue
	for i := start; i < end; i++ {
		s.WriteString(l.renderItem(i))
	}

	// Show scrolling indicators
	l.renderScrollIndicators(&s, start, end)
	l.renderHelp(&s)

	result := s.String()

	// Apply border if width is set
	if l.width > 0 {
		result = l.borderStyle.Width(l.width - borderPadding).Render(result)
	}

	return result
}

// renderTitle writes the title line plus the active filter indicator.
func (l *List) renderTitle(s *strings.Builder) {
	s.WriteString(l.titleStyle.Render(l.title))
	if l.filtering || l.filter != "" {
		s.WriteString(" ")
		s.WriteString(l.helpStyle.Render("filter: " + l.filter + "▏"))
	}
	s.WriteString("\n\n")
}

// renderEmpty writes the placeholder shown when no items remain.
func (l *List) renderEmpty(s *strings.Builder) {
	s.WriteString(l.helpStyle.Render("No items available"))
	if l.showHelp {
		s.WriteString("\n\n")
		s.WriteString(l.helpStyle.Render("No items to display"))
	}
}

// renderHelp writes the footer key hints.
func (l *List) renderHelp(s *strings.Builder) {
	if !l.showHelp {
		return
	}
	s.WriteString("\n")
	if len(l.items) > l.maxVisible {
		s.WriteString(l.helpStyle.Render("↑/↓: navigate • PgUp/PgDn: page • Home/End: jump • /: filter • enter/space: select"))
	} else {
		s.WriteString(l.helpStyle.Render("↑/↓: navigate • /: filter • enter/space: select • q: quit"))
	}
}

// GetCursor returns the current cursor position
func (l *List) GetCursor() int {
	return l.cursor
}

// SetCursor moves the cursor to index, clamped to the item bounds.
func (l *List) SetCursor(index int) {
	if index < 0 {
		index = 0
	}
	if maxCursor := len(l.items) - 1; index > maxCursor {
		index = max(maxCursor, 0)
	}
	l.cursor = index
	l.updateViewport()
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
	l.allItems = items
	l.applyFilter()
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
	l.filter = ""
	l.filtering = false
	l.applyFilter()
}
