package components

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestNewList(t *testing.T) {
	items := []ListItem{
		{Title: "Item 1", Description: "First item", Tag: "tag1"},
		{Title: "Item 2", Description: "Second item", Tag: "tag2"},
	}

	list := NewList("Test List", items)

	assert.NotNil(t, list)
	assert.Equal(t, "Test List", list.title)
	assert.Equal(t, items, list.items)
	assert.Equal(t, 0, list.cursor)
	assert.Equal(t, -1, list.selected)
	assert.True(t, list.showHelp)
}

func TestListItem(t *testing.T) {
	item := ListItem{
		Title:       "Test Item",
		Description: "Test Description",
		Tag:         "test",
		Data:        "additional data",
	}

	assert.Equal(t, "Test Item", item.Title)
	assert.Equal(t, "Test Description", item.Description)
	assert.Equal(t, "test", item.Tag)
	assert.Equal(t, "additional data", item.Data)
}

func TestList_GetCursor(t *testing.T) {
	items := []ListItem{
		{Title: "Item 1"},
		{Title: "Item 2"},
	}

	list := NewList("Test", items)
	assert.Equal(t, 0, list.GetCursor())
}

func TestList_GetSelected(t *testing.T) {
	items := []ListItem{
		{Title: "Item 1"},
		{Title: "Item 2"},
	}

	list := NewList("Test", items)
	assert.Equal(t, -1, list.GetSelected())

	// Simulate selection
	list.selected = 0
	assert.Equal(t, 0, list.GetSelected())
}

func TestList_GetSelectedItem(t *testing.T) {
	items := []ListItem{
		{Title: "Item 1", Tag: "tag1"},
		{Title: "Item 2", Tag: "tag2"},
	}

	list := NewList("Test", items)

	// No selection initially
	selectedItem := list.GetSelectedItem()
	assert.Nil(t, selectedItem)

	// Set selection
	list.selected = 0
	selectedItem = list.GetSelectedItem()
	assert.NotNil(t, selectedItem)
	assert.Equal(t, "Item 1", selectedItem.Title)
	assert.Equal(t, "tag1", selectedItem.Tag)

	// Invalid selection
	list.selected = 10
	selectedItem = list.GetSelectedItem()
	assert.Nil(t, selectedItem)
}

func TestList_GetCurrentItem(t *testing.T) {
	items := []ListItem{
		{Title: "Item 1", Tag: "tag1"},
		{Title: "Item 2", Tag: "tag2"},
	}

	list := NewList("Test", items)

	// Default cursor position (0)
	currentItem := list.GetCurrentItem()
	assert.NotNil(t, currentItem)
	assert.Equal(t, "Item 1", currentItem.Title)

	// Move cursor
	list.cursor = 1
	currentItem = list.GetCurrentItem()
	assert.NotNil(t, currentItem)
	assert.Equal(t, "Item 2", currentItem.Title)

	// Invalid cursor
	list.cursor = 10
	currentItem = list.GetCurrentItem()
	assert.Nil(t, currentItem)
}

func TestList_SetItems(t *testing.T) {
	list := NewList("Test", []ListItem{{Title: "Old Item"}})

	newItems := []ListItem{
		{Title: "New Item 1"},
		{Title: "New Item 2"},
	}

	list.SetItems(newItems)

	assert.Equal(t, newItems, list.items)
	assert.Equal(t, 0, list.cursor)    // Should reset cursor
	assert.Equal(t, -1, list.selected) // Should reset selection
}

func TestList_SetShowHelp(t *testing.T) {
	list := NewList("Test", []ListItem{{Title: "Item"}})

	assert.True(t, list.showHelp) // Default is true

	list.SetShowHelp(false)
	assert.False(t, list.showHelp)

	list.SetShowHelp(true)
	assert.True(t, list.showHelp)
}

func TestList_Reset(t *testing.T) {
	list := NewList("Test", []ListItem{{Title: "Item 1"}, {Title: "Item 2"}})

	// Change state
	list.cursor = 1
	list.selected = 1

	// Reset
	list.Reset()

	assert.Equal(t, 0, list.cursor)
	assert.Equal(t, -1, list.selected)
}

func TestList_Update_KeyNavigation(t *testing.T) {
	items := []ListItem{
		{Title: "Item 1"},
		{Title: "Item 2"},
		{Title: "Item 3"},
	}

	list := NewList("Test", items)

	// Test down navigation
	list, _ = list.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	assert.Equal(t, 1, list.cursor)

	list, _ = list.Update(tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, 2, list.cursor)

	// Test can't go beyond last item
	list, _ = list.Update(tea.KeyMsg{Type: tea.KeyDown})
	assert.Equal(t, 2, list.cursor)

	// Test up navigation
	list, _ = list.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	assert.Equal(t, 1, list.cursor)

	list, _ = list.Update(tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, 0, list.cursor)

	// Test can't go above first item
	list, _ = list.Update(tea.KeyMsg{Type: tea.KeyUp})
	assert.Equal(t, 0, list.cursor)
}

func TestList_Update_HomeEnd(t *testing.T) {
	items := []ListItem{
		{Title: "Item 1"},
		{Title: "Item 2"},
		{Title: "Item 3"},
	}

	list := NewList("Test", items)
	list.cursor = 1 // Start in middle

	// Test home
	list, _ = list.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("home")})
	assert.Equal(t, 0, list.cursor)

	// Test end
	list, _ = list.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("end")})
	assert.Equal(t, 2, list.cursor)
}

func TestList_Update_Selection(t *testing.T) {
	items := []ListItem{
		{Title: "Item 1"},
		{Title: "Item 2"},
	}

	list := NewList("Test", items)
	list.cursor = 1

	// Test enter selection
	list, _ = list.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Equal(t, 1, list.selected)

	// Test space selection
	list.cursor = 0
	list, _ = list.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})
	assert.Equal(t, 0, list.selected)
}

func TestList_Update_WindowSize(t *testing.T) {
	list := NewList("Test", []ListItem{{Title: "Item"}})

	list, _ = list.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	assert.Equal(t, 80, list.width)
	assert.Equal(t, 24, list.height)
}

func TestList_View(t *testing.T) {
	items := []ListItem{
		{Title: "Item 1", Description: "First item", Tag: "tag1"},
		{Title: "Item 2", Description: "Second item", Tag: "tag2"},
	}

	list := NewList("Test List", items)

	view := list.View()

	// Should contain title
	assert.Contains(t, view, "Test List")

	// Should contain items
	assert.Contains(t, view, "Item 1")
	assert.Contains(t, view, "Item 2")

	// Should contain help text
	assert.Contains(t, view, "navigate")
	assert.Contains(t, view, "select")

	// Should contain cursor indicator for first item
	assert.Contains(t, view, "❯")
}

func TestList_View_WithoutHelp(t *testing.T) {
	items := []ListItem{
		{Title: "Item 1"},
	}

	list := NewList("Test", items)
	list.SetShowHelp(false)

	view := list.View()

	// Should not contain help text
	assert.NotContains(t, view, "navigate")
	assert.NotContains(t, view, "select")
}

func TestList_View_EmptyList(t *testing.T) {
	list := NewList("Empty List", []ListItem{})

	view := list.View()

	// Should contain title
	assert.Contains(t, view, "Empty List")

	// Should handle empty list gracefully
	assert.NotEmpty(t, view)
}

func TestList_View_LongList(t *testing.T) {
	var items []ListItem
	for i := 0; i < 20; i++ {
		items = append(items, ListItem{Title: "Item " + string(rune('A'+i))})
	}

	list := NewList("Long List", items)
	list.height = 10 // Simulate small terminal

	view := list.View()

	// Should contain title
	assert.Contains(t, view, "Long List")

	// Should contain some items but not all (due to scrolling)
	assert.Contains(t, view, "Item A")

	// Should be properly formatted
	assert.NotEmpty(t, view)
}

// Benchmark tests
func BenchmarkList_Update(b *testing.B) {
	items := make([]ListItem, 100)
	for i := range items {
		items[i] = ListItem{Title: "Item " + string(rune('A'+i%26))}
	}

	list := NewList("Benchmark", items)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		list.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
}

func BenchmarkList_View(b *testing.B) {
	items := make([]ListItem, 100)
	for i := range items {
		items[i] = ListItem{
			Title:       "Item " + string(rune('A'+i%26)),
			Description: "Description for item " + string(rune('A'+i%26)),
			Tag:         "tag" + string(rune('1'+i%9)),
		}
	}

	list := NewList("Benchmark", items)
	list.width = 80
	list.height = 24

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		list.View()
	}
}
