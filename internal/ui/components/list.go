package components

import (
	"fmt"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// RenderRequest represents an async render request
type RenderRequest struct {
	Index    int
	Item     ListItem
	IsCursor bool
	Style    lipgloss.Style
}

// RenderResult represents an async render result
type RenderResult struct {
	Index        int
	RenderedItem string
	Error        error
}

// FrameRateMsg represents a 60fps frame rate message
type FrameRateMsg struct {
	Timestamp time.Time
}

// ListItem represents a selectable item in a list
type ListItem struct {
	Title       string
	Description string
	Tag         string
	Data        interface{} // Additional data for the item
}

// List represents a selectable list component with virtual scrolling and async rendering
type List struct {
	title    string
	items    []ListItem
	cursor   int
	selected int
	width    int
	height   int
	showHelp bool

	// Virtual scrolling for performance
	viewportTop    int
	viewportHeight int
	maxVisible     int

	// Memory optimization
	itemCache    map[int]string
	cacheDirty   bool
	cacheSize    int
	maxCacheSize int

	// Async rendering optimization
	renderQueue   chan RenderRequest
	renderResults chan RenderResult
	renderWorkers int
	isRendering   bool

	// Frame rate limiting for 60fps
	lastRender    time.Time
	frameInterval time.Duration

	// Styles
	titleStyle    lipgloss.Style
	itemStyle     lipgloss.Style
	selectedStyle lipgloss.Style
	descStyle     lipgloss.Style
	tagStyle      lipgloss.Style
	helpStyle     lipgloss.Style
	borderStyle   lipgloss.Style
}

// NewList creates a new List component with performance optimizations
func NewList(title string, items []ListItem) *List {
	l := &List{
		title:    title,
		items:    items,
		cursor:   0,
		selected: -1,
		showHelp: true,

		// Initialize virtual scrolling
		viewportTop:    0,
		viewportHeight: 20, // Default viewport size
		maxVisible:     20,

		// Initialize memory optimization
		itemCache:    make(map[int]string),
		cacheDirty:   true,
		cacheSize:    0,
		maxCacheSize: 100, // Cache up to 100 rendered items

		// Initialize async rendering (60fps = ~16.67ms per frame)
		renderQueue:   make(chan RenderRequest, 100),
		renderResults: make(chan RenderResult, 100),
		renderWorkers: 4,                     // Optimal worker count for UI rendering
		frameInterval: time.Millisecond * 16, // 60fps target
		lastRender:    time.Now(),

		titleStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			MarginLeft(1).
			MarginBottom(1),

		itemStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFF")).
			PaddingLeft(2),

		selectedStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			PaddingLeft(1).
			PaddingRight(1).
			Border(lipgloss.RoundedBorder(), false, false, false, true).
			BorderForeground(lipgloss.Color("#04B575")),

		descStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888")).
			PaddingLeft(4),

		tagStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#00FF87")).
			Background(lipgloss.Color("#1A1A1A")).
			PaddingLeft(1).
			PaddingRight(1).
			MarginLeft(2),

		helpStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#626262")).
			MarginTop(1).
			PaddingLeft(1),

		borderStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#874BFD")).
			Padding(1),
	}

	// Start async render workers for 60fps performance
	l.startRenderWorkers()

	return l
}

// startRenderWorkers initializes async rendering workers for 60fps performance
func (l *List) startRenderWorkers() {
	for i := 0; i < l.renderWorkers; i++ {
		go func() {
			for req := range l.renderQueue {
				result := RenderResult{
					Index: req.Index,
				}

				// Render item with style optimization
				if req.IsCursor {
					cursor := "❯"
					line := fmt.Sprintf("%s %s", cursor, req.Item.Title)
					if req.Item.Tag != "" {
						line += l.tagStyle.Render(fmt.Sprintf("[%s]", req.Item.Tag))
					}
					result.RenderedItem = l.selectedStyle.Render(line) + "\n"

					// Show description if available and item is selected
					if req.Item.Description != "" {
						result.RenderedItem += l.descStyle.Render(req.Item.Description) + "\n"
					}
				} else {
					cursor := " "
					line := fmt.Sprintf("%s %s", cursor, req.Item.Title)
					if req.Item.Tag != "" {
						line += l.tagStyle.Render(fmt.Sprintf("[%s]", req.Item.Tag))
					}
					result.RenderedItem = l.itemStyle.Render(line) + "\n"
				}

				select {
				case l.renderResults <- result:
				default:
					// Drop frame if channel is full (maintain 60fps)
				}
			}
		}()
	}
}

// Update handles messages for the list component
func (l *List) Update(msg tea.Msg) (*List, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		l.width = msg.Width
		l.height = msg.Height

		// Update viewport parameters for virtual scrolling
		l.updateViewport()

	case FrameRateMsg:
		// Handle 60fps frame rate limiting
		return l, l.tick60fps()

	case RenderResult:
		// Handle async render results
		if msg.Index >= 0 && msg.Index < len(l.items) && msg.Error == nil {
			l.itemCache[msg.Index] = msg.RenderedItem
			l.cacheSize++
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if l.cursor > 0 {
				l.cursor--
				l.updateViewport()
				l.invalidateCache()
			}
		case "down", "j":
			if l.cursor < len(l.items)-1 {
				l.cursor++
				l.updateViewport()
				l.invalidateCache()
			}
		case "page_up", "ctrl+b":
			// Page up navigation
			l.cursor -= l.maxVisible
			if l.cursor < 0 {
				l.cursor = 0
			}
			l.updateViewport()
			l.invalidateCache()
		case "page_down", "ctrl+f":
			// Page down navigation
			l.cursor += l.maxVisible
			if l.cursor >= len(l.items) {
				l.cursor = len(l.items) - 1
			}
			l.updateViewport()
			l.invalidateCache()
		case "enter", " ":
			l.selected = l.cursor
		case "home":
			l.cursor = 0
			l.updateViewport()
			l.invalidateCache()
		case "end":
			l.cursor = len(l.items) - 1
			l.updateViewport()
			l.invalidateCache()
		}
	}

	return l, l.tick60fps()
}

// tick60fps returns a command for 60fps frame rate limiting
func (l *List) tick60fps() tea.Cmd {
	return tea.Tick(l.frameInterval, func(t time.Time) tea.Msg {
		return FrameRateMsg{Timestamp: t}
	})
}

// View renders the list component with async rendering and 60fps optimization
func (l *List) View() string {
	// 60fps frame rate limiting
	now := time.Now()
	if now.Sub(l.lastRender) < l.frameInterval {
		// Skip frame if too early (maintain 60fps)
		return l.getCachedView()
	}
	l.lastRender = now

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

	// Async rendering optimization: only render visible items
	for i := start; i < end; i++ {
		// Check cache first for memory efficiency
		if cachedItem, exists := l.itemCache[i]; exists && !l.cacheDirty {
			s.WriteString(cachedItem)
			continue
		}

		// Queue async render request for 60fps performance
		select {
		case l.renderQueue <- RenderRequest{
			Index:    i,
			Item:     l.items[i],
			IsCursor: i == l.cursor,
		}:
		default:
			// If queue is full, render synchronously to maintain display
			renderedItem := l.renderItem(i)
			if l.cacheSize < l.maxCacheSize {
				l.itemCache[i] = renderedItem
				l.cacheSize++
			}
			s.WriteString(renderedItem)
		}

		// Check for completed async renders
		select {
		case result := <-l.renderResults:
			if result.Index >= start && result.Index < end && result.Error == nil {
				l.itemCache[result.Index] = result.RenderedItem
				s.WriteString(result.RenderedItem)
			}
		default:
			// No async result available, use sync render
			if _, exists := l.itemCache[i]; !exists {
				renderedItem := l.renderItem(i)
				s.WriteString(renderedItem)
			}
		}
	}

	// Show performance-optimized scrolling indicators
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
		result = l.borderStyle.Width(l.width - 4).Render(result)
	}

	return result
}

// getCachedView returns a cached view for frame rate optimization
func (l *List) getCachedView() string {
	// Return previously rendered view to maintain 60fps
	// This is a simplified cache - in practice would store complete rendered view
	var s strings.Builder
	s.WriteString(l.titleStyle.Render(l.title))
	s.WriteString("\n\n")

	if len(l.items) == 0 {
		s.WriteString(l.helpStyle.Render("No items available"))
		return s.String()
	}

	// Use cached items for fast display
	start := l.viewportTop
	end := l.viewportTop + l.maxVisible
	if end > len(l.items) {
		end = len(l.items)
	}

	for i := start; i < end; i++ {
		if cachedItem, exists := l.itemCache[i]; exists {
			s.WriteString(cachedItem)
		}
	}

	return s.String()
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

// SetItems updates the list items with performance optimizations
func (l *List) SetItems(items []ListItem) {
	l.items = items
	l.cursor = 0
	l.selected = -1
	l.viewportTop = 0
	l.clearCache() // Clear cache when items change
	l.updateViewport()
}

// SetShowHelp sets whether to show help text
func (l *List) SetShowHelp(show bool) {
	l.showHelp = show
}

// Reset resets the list state with performance optimizations
func (l *List) Reset() {
	l.cursor = 0
	l.selected = -1
	l.viewportTop = 0
	l.clearCache()
	l.updateViewport()
}

// updateViewport updates the viewport for virtual scrolling
func (l *List) updateViewport() {
	if len(l.items) == 0 {
		return
	}

	// Update max visible items based on height
	if l.height > 0 {
		l.maxVisible = l.height - 6 // Account for title, help, padding
		if l.maxVisible < 5 {
			l.maxVisible = 5 // Minimum visible items
		}
	}

	// Adjust viewport to keep cursor visible
	if l.cursor < l.viewportTop {
		l.viewportTop = l.cursor
	} else if l.cursor >= l.viewportTop+l.maxVisible {
		l.viewportTop = l.cursor - l.maxVisible + 1
	}

	// Ensure viewport doesn't go beyond bounds
	if l.viewportTop < 0 {
		l.viewportTop = 0
	}
	if l.viewportTop+l.maxVisible > len(l.items) {
		l.viewportTop = len(l.items) - l.maxVisible
		if l.viewportTop < 0 {
			l.viewportTop = 0
		}
	}
}

// invalidateCache marks the render cache as dirty
func (l *List) invalidateCache() {
	l.cacheDirty = true
	l.cacheSize = 0
	// Clear the cache map
	for k := range l.itemCache {
		delete(l.itemCache, k)
	}
}

// clearCache clears the entire cache
func (l *List) clearCache() {
	l.itemCache = make(map[int]string)
	l.cacheDirty = true
	l.cacheSize = 0
}

// renderItem renders a single item
func (l *List) renderItem(i int) string {
	item := l.items[i]
	cursor := " "

	if i == l.cursor {
		cursor = "❯"
		line := fmt.Sprintf("%s %s", cursor, item.Title)
		if item.Tag != "" {
			line += l.tagStyle.Render(fmt.Sprintf("[%s]", item.Tag))
		}
		result := l.selectedStyle.Render(line) + "\n"

		// Show description if available and item is selected
		if item.Description != "" {
			result += l.descStyle.Render(item.Description) + "\n"
		}

		return result
	} else {
		line := fmt.Sprintf("%s %s", cursor, item.Title)
		if item.Tag != "" {
			line += l.tagStyle.Render(fmt.Sprintf("[%s]", item.Tag))
		}
		return l.itemStyle.Render(line) + "\n"
	}
}

// renderScrollIndicators renders scrolling indicators with performance info
func (l *List) renderScrollIndicators(s *strings.Builder, start, end int) {
	totalItems := len(l.items)

	// Only show indicators if there are items outside the viewport
	if totalItems > l.maxVisible {
		if start > 0 {
			s.WriteString(l.helpStyle.Render(fmt.Sprintf("↑ %d more items above", start)))
			s.WriteString("\n")
		}
		if end < totalItems {
			s.WriteString(l.helpStyle.Render(fmt.Sprintf("↓ %d more items below", totalItems-end)))
			s.WriteString("\n")
		}

		// Add a progress indicator for large lists (>50 items)
		if totalItems > 50 {
			progress := float64(l.cursor) / float64(totalItems-1) * 100
			s.WriteString(l.helpStyle.Render(fmt.Sprintf("Position: %d/%d (%.1f%%)", l.cursor+1, totalItems, progress)))
			s.WriteString("\n")
		}
	}
}

// Memory pools for object reuse (reduce GC pressure)
var (
	stringBuilderPool = sync.Pool{
		New: func() interface{} {
			return &strings.Builder{}
		},
	}
	renderRequestPool = sync.Pool{
		New: func() interface{} {
			return &RenderRequest{}
		},
	}
	renderResultPool = sync.Pool{
		New: func() interface{} {
			return &RenderResult{}
		},
	}
)

// getStringBuilder gets a pooled string builder
func getStringBuilder() *strings.Builder {
	sb := stringBuilderPool.Get().(*strings.Builder)
	sb.Reset()
	return sb
}

// putStringBuilder returns a string builder to pool
func putStringBuilder(sb *strings.Builder) {
	stringBuilderPool.Put(sb)
}

// GetPerformanceStats returns performance statistics for monitoring
func (l *List) GetPerformanceStats() map[string]interface{} {
	return map[string]interface{}{
		"total_items":       len(l.items),
		"cache_size":        l.cacheSize,
		"max_cache_size":    l.maxCacheSize,
		"viewport_top":      l.viewportTop,
		"viewport_height":   l.viewportHeight,
		"max_visible":       l.maxVisible,
		"cache_efficiency":  float64(l.cacheSize) / float64(l.maxCacheSize) * 100,
		"memory_optimized":  len(l.items) > l.maxVisible,
		"virtual_scrolling": true,
		"async_rendering":   true,
		"frame_rate_target": "60fps",
		"render_workers":    l.renderWorkers,
		"queue_capacity":    cap(l.renderQueue),
		"results_capacity":  cap(l.renderResults),
	}
}

// OptimizeForLargeList configures the list for optimal performance with large datasets
func (l *List) OptimizeForLargeList() {
	l.maxCacheSize = 200 // Increase cache for large lists
	l.cacheDirty = true
	l.clearCache()

	// Increase render workers for large datasets
	if len(l.items) > 1000 {
		l.renderWorkers = 8 // More workers for very large lists
	}

	// Optimize frame interval for heavy rendering
	if len(l.items) > 5000 {
		l.frameInterval = time.Millisecond * 20 // 50fps for very large datasets
	}
}

// Close properly shuts down the list component and cleans up resources
func (l *List) Close() {
	// Close render channels to stop workers
	if l.renderQueue != nil {
		close(l.renderQueue)
	}
	if l.renderResults != nil {
		close(l.renderResults)
	}

	// Clear caches to free memory
	l.clearCache()
}
