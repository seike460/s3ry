package views

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/seike460/s3ry/internal/history"
	"github.com/seike460/s3ry/internal/ui/components"
)

// HistoryView represents the history and bookmarks view
type HistoryView struct {
	historyManager *history.Manager
	list           *components.List
	currentTab     int // 0: History, 1: Bookmarks, 2: Frequent Locations
	filter         history.HistoryFilter
	searchTerm     string
	width          int
	height         int

	// Styles
	titleStyle     lipgloss.Style
	tabStyle       lipgloss.Style
	activeTabStyle lipgloss.Style
	subtitleStyle  lipgloss.Style
	helpStyle      lipgloss.Style
}

// NewHistoryView creates a new history view
func NewHistoryView() *HistoryView {
	// Initialize history manager
	historyManager, err := history.NewManager("")
	if err != nil {
		// Fallback to basic view if history manager fails
		historyManager = nil
	}

	view := &HistoryView{
		historyManager: historyManager,
		currentTab:     0,
		filter:         history.HistoryFilter{Limit: 50},

		titleStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			MarginBottom(1),

		tabStyle: lipgloss.NewStyle().
			Padding(0, 2).
			Background(lipgloss.Color("#F0F0F0")).
			Foreground(lipgloss.Color("#666")).
			MarginRight(1),

		activeTabStyle: lipgloss.NewStyle().
			Padding(0, 2).
			Background(lipgloss.Color("#7D56F4")).
			Foreground(lipgloss.Color("#FFFFFF")).
			MarginRight(1),

		subtitleStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888")).
			MarginBottom(1),

		helpStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666")).
			MarginTop(1),
	}

	view.updateList()
	return view
}

// Init initializes the history view
func (v *HistoryView) Init() tea.Cmd {
	return nil
}

// Update handles messages for the history view
func (v *HistoryView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		v.width = msg.Width
		v.height = msg.Height
		if v.list != nil {
			v.list, _ = v.list.Update(msg)
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return v, tea.Quit
		case "esc":
			// Go back to main view
			return NewOperationView("", ""), nil
		case "?":
			// Show help
			return NewHelpView(), nil
		case "tab":
			// Switch tabs
			v.currentTab = (v.currentTab + 1) % 3
			v.updateList()
		case "shift+tab":
			// Switch tabs backwards
			v.currentTab = (v.currentTab + 2) % 3
			v.updateList()
		case "r":
			// Refresh
			v.updateList()
		case "c":
			// Clear history (only on history tab)
			if v.currentTab == 0 && v.historyManager != nil {
				v.historyManager.ClearHistory()
				v.updateList()
			}
		case "/":
			// Start search
			// In a full implementation, this would switch to search mode
			return v, nil
		case "enter", " ":
			if v.list != nil {
				selectedItem := v.list.GetCurrentItem()
				if selectedItem != nil {
					return v.handleSelection(selectedItem)
				}
			}
		}

		if v.list != nil {
			v.list, _ = v.list.Update(msg)
		}
	}

	return v, tea.Batch(cmds...)
}

// View renders the history view
func (v *HistoryView) View() string {
	if v.historyManager == nil {
		return v.renderError("History manager not available")
	}

	// Render tabs
	tabs := v.renderTabs()

	// Render current tab content
	var content string
	if v.list != nil {
		content = v.list.View()
	} else {
		content = "No data available"
	}

	// Render help
	help := v.renderHelp()

	return tabs + "\n\n" + content + "\n" + help
}

// renderTabs renders the tab navigation
func (v *HistoryView) renderTabs() string {
	tabNames := []string{"📚 History", "🔖 Bookmarks", "📍 Frequent"}

	var tabs []string
	for i, name := range tabNames {
		if i == v.currentTab {
			tabs = append(tabs, v.activeTabStyle.Render(name))
		} else {
			tabs = append(tabs, v.tabStyle.Render(name))
		}
	}

	return v.titleStyle.Render("S3ry History & Bookmarks") + "\n" +
		strings.Join(tabs, "")
}

// updateList updates the list content based on current tab
func (v *HistoryView) updateList() {
	if v.historyManager == nil {
		return
	}

	var items []components.ListItem
	var title string

	switch v.currentTab {
	case 0: // History
		title = "📚 Operation History"
		entries := v.historyManager.GetHistory(v.filter)
		items = make([]components.ListItem, len(entries))

		for i, entry := range entries {
			status := "✅"
			if !entry.Success {
				status = "❌"
			}

			description := fmt.Sprintf("%s | %s | %s",
				entry.Timestamp.Format("2006-01-02 15:04:05"),
				entry.Bucket,
				entry.Key,
			)

			if entry.Duration > 0 {
				description += fmt.Sprintf(" | %v", entry.Duration)
			}

			if entry.Size > 0 {
				description += fmt.Sprintf(" | %s", formatBytesHistory(entry.Size))
			}

			items[i] = components.ListItem{
				Title:       fmt.Sprintf("%s %s", status, strings.Title(string(entry.Action))),
				Description: description,
				Tag:         "History",
				Data:        entry,
			}
		}

	case 1: // Bookmarks
		title = "🔖 Saved Bookmarks"
		bookmarks := v.historyManager.GetBookmarks(history.BookmarkFilter{})
		items = make([]components.ListItem, len(bookmarks))

		for i, bookmark := range bookmarks {
			typeIcon := "📁"
			switch bookmark.Type {
			case history.BookmarkOperation:
				typeIcon = "⚡"
			case history.BookmarkQuery:
				typeIcon = "🔍"
			}

			description := fmt.Sprintf("%s/%s | Used %d times | %s",
				bookmark.Bucket,
				bookmark.Prefix,
				bookmark.UseCount,
				bookmark.LastUsed.Format("2006-01-02"),
			)

			if len(bookmark.Tags) > 0 {
				description += " | Tags: " + strings.Join(bookmark.Tags, ", ")
			}

			items[i] = components.ListItem{
				Title:       fmt.Sprintf("%s %s", typeIcon, bookmark.Name),
				Description: description,
				Tag:         "Bookmark",
				Data:        bookmark,
			}
		}

	case 2: // Frequent Locations
		title = "📍 Frequently Accessed Locations"
		locations := v.historyManager.GetFrequentLocations(20)
		items = make([]components.ListItem, len(locations))

		for i, location := range locations {
			description := fmt.Sprintf("Accessed %d times | Last: %s",
				location.AccessCount,
				location.LastAccess.Format("2006-01-02 15:04"),
			)

			items[i] = components.ListItem{
				Title:       "📂 " + location.Location,
				Description: description,
				Tag:         "Location",
				Data:        location,
			}
		}
	}

	if len(items) == 0 {
		items = []components.ListItem{
			{
				Title:       "No items found",
				Description: "No data available for this view",
				Tag:         "Empty",
			},
		}
	}

	v.list = components.NewList(title, items)
}

// handleSelection handles item selection
func (v *HistoryView) handleSelection(item *components.ListItem) (tea.Model, tea.Cmd) {
	switch item.Tag {
	case "History":
		entry := item.Data.(history.HistoryEntry)
		// Navigate to the bucket/location from history
		if entry.Bucket != "" {
			return NewBucketView(""), nil
		}

	case "Bookmark":
		bookmark := item.Data.(history.Bookmark)
		// Use the bookmark (increment usage count)
		if v.historyManager != nil {
			v.historyManager.UseBookmark(bookmark.ID)
		}
		// Navigate to bookmarked location
		if bookmark.Bucket != "" {
			return NewBucketView(""), nil
		}

	case "Location":
		location := item.Data.(history.LocationStats)
		// Parse location and navigate
		parts := strings.SplitN(location.Location, "/", 2)
		if len(parts) > 0 {
			return NewBucketView(""), nil
		}
	}

	return v, nil
}

// renderHelp renders help text
func (v *HistoryView) renderHelp() string {
	switch v.currentTab {
	case 0: // History
		return v.helpStyle.Render("tab: switch views • r: refresh • c: clear history • enter: navigate • esc: back • q: quit")
	case 1: // Bookmarks
		return v.helpStyle.Render("tab: switch views • r: refresh • enter: use bookmark • esc: back • q: quit")
	case 2: // Frequent
		return v.helpStyle.Render("tab: switch views • r: refresh • enter: navigate • esc: back • q: quit")
	}
	return ""
}

// renderError renders an error message
func (v *HistoryView) renderError(message string) string {
	errorStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FF5555")).
		MarginTop(2).
		MarginBottom(2)

	return v.titleStyle.Render("📚 History & Bookmarks") + "\n\n" +
		errorStyle.Render("⚠️ Error: "+message) + "\n\n" +
		v.helpStyle.Render("esc: back • q: quit")
}

// Helper function to format bytes
func formatBytesHistory(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// BookmarkCreateView represents a view for creating bookmarks
type BookmarkCreateView struct {
	name         string
	description  string
	bucket       string
	prefix       string
	tags         string
	bookmarkType history.BookmarkType
	currentField int

	// Styles
	titleStyle lipgloss.Style
	fieldStyle lipgloss.Style
	helpStyle  lipgloss.Style
}

// NewBookmarkCreateView creates a new bookmark creation view
func NewBookmarkCreateView(bucket, prefix string) *BookmarkCreateView {
	return &BookmarkCreateView{
		bucket:       bucket,
		prefix:       prefix,
		bookmarkType: history.BookmarkLocation,
		currentField: 0,

		titleStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			MarginBottom(2),

		fieldStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(1).
			MarginBottom(1),

		helpStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666")).
			MarginTop(2),
	}
}

// Init initializes the bookmark create view
func (v *BookmarkCreateView) Init() tea.Cmd {
	return nil
}

// Update handles messages for the bookmark create view
func (v *BookmarkCreateView) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "esc":
			return NewHistoryView(), nil
		case "tab", "down":
			v.currentField = (v.currentField + 1) % 6
		case "shift+tab", "up":
			v.currentField = (v.currentField + 5) % 6
		case "enter":
			if v.currentField == 5 { // Save button
				return v.saveBookmark()
			}
		case "backspace":
			v.handleBackspace()
		default:
			if len(msg.String()) == 1 {
				v.handleInput(msg.String())
			}
		}
	}

	return v, nil
}

// View renders the bookmark create view
func (v *BookmarkCreateView) View() string {
	title := v.titleStyle.Render("🔖 Create New Bookmark")

	fields := []string{
		v.renderField("Name:", v.name, 0),
		v.renderField("Description:", v.description, 1),
		v.renderField("Bucket:", v.bucket, 2),
		v.renderField("Prefix:", v.prefix, 3),
		v.renderField("Tags (comma-separated):", v.tags, 4),
		v.renderSaveButton(),
	}

	help := v.helpStyle.Render("tab: next field • shift+tab: previous field • enter: save • esc: cancel")

	return title + "\n\n" + strings.Join(fields, "\n") + "\n" + help
}

// renderField renders an input field
func (v *BookmarkCreateView) renderField(label, value string, fieldIndex int) string {
	style := v.fieldStyle
	if v.currentField == fieldIndex {
		style = style.BorderForeground(lipgloss.Color("#7D56F4"))
	}

	content := fmt.Sprintf("%s\n%s", label, value)
	if v.currentField == fieldIndex {
		content += "_" // Cursor
	}

	return style.Render(content)
}

// renderSaveButton renders the save button
func (v *BookmarkCreateView) renderSaveButton() string {
	style := lipgloss.NewStyle().
		Background(lipgloss.Color("#7D56F4")).
		Foreground(lipgloss.Color("#FFFFFF")).
		Padding(1, 2).
		MarginTop(1)

	if v.currentField == 5 {
		style = style.Background(lipgloss.Color("#9333EA"))
	}

	return style.Render("💾 Save Bookmark")
}

// handleInput handles text input
func (v *BookmarkCreateView) handleInput(char string) {
	switch v.currentField {
	case 0:
		v.name += char
	case 1:
		v.description += char
	case 2:
		v.bucket += char
	case 3:
		v.prefix += char
	case 4:
		v.tags += char
	}
}

// handleBackspace handles backspace input
func (v *BookmarkCreateView) handleBackspace() {
	switch v.currentField {
	case 0:
		if len(v.name) > 0 {
			v.name = v.name[:len(v.name)-1]
		}
	case 1:
		if len(v.description) > 0 {
			v.description = v.description[:len(v.description)-1]
		}
	case 2:
		if len(v.bucket) > 0 {
			v.bucket = v.bucket[:len(v.bucket)-1]
		}
	case 3:
		if len(v.prefix) > 0 {
			v.prefix = v.prefix[:len(v.prefix)-1]
		}
	case 4:
		if len(v.tags) > 0 {
			v.tags = v.tags[:len(v.tags)-1]
		}
	}
}

// saveBookmark saves the bookmark and returns to history view
func (v *BookmarkCreateView) saveBookmark() (tea.Model, tea.Cmd) {
	if v.name == "" || v.bucket == "" {
		// Show error - name and bucket are required
		return v, nil
	}

	// Create bookmark
	bookmark := history.Bookmark{
		Name:        v.name,
		Description: v.description,
		Type:        v.bookmarkType,
		Bucket:      v.bucket,
		Prefix:      v.prefix,
		Tags:        strings.Split(v.tags, ","),
		CreatedAt:   time.Now(),
		LastUsed:    time.Now(),
	}

	// Clean up tags
	var cleanTags []string
	for _, tag := range bookmark.Tags {
		tag = strings.TrimSpace(tag)
		if tag != "" {
			cleanTags = append(cleanTags, tag)
		}
	}
	bookmark.Tags = cleanTags

	// Save to history manager
	historyManager, err := history.NewManager("")
	if err == nil {
		historyManager.AddBookmark(bookmark)
	}

	return NewHistoryView(), nil
}
