package components

import (
	"fmt"
	"strings"
)

const (
	// listChromeLines reserves rows for the title, help line, and padding.
	listChromeLines = 6
	// minVisibleItems keeps the viewport usable on very small terminals.
	minVisibleItems = 5
	// positionIndicatorThreshold shows "Position: n/m" only on large lists.
	positionIndicatorThreshold = 50
)

// updateViewport updates the viewport for virtual scrolling
func (l *List) updateViewport() {
	if len(l.items) == 0 {
		return
	}

	// Update max visible items based on height
	if l.height > 0 {
		l.maxVisible = l.height - listChromeLines
		if l.maxVisible < minVisibleItems {
			l.maxVisible = minVisibleItems
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
	}
	line := fmt.Sprintf("%s %s", cursor, item.Title)
	if item.Tag != "" {
		line += l.tagStyle.Render(fmt.Sprintf("[%s]", item.Tag))
	}
	return l.itemStyle.Render(line) + "\n"
}

// renderScrollIndicators renders scrolling indicators
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
		if totalItems > positionIndicatorThreshold {
			progress := float64(l.cursor) / float64(totalItems-1) * 100
			s.WriteString(l.helpStyle.Render(fmt.Sprintf("Position: %d/%d (%.1f%%)", l.cursor+1, totalItems, progress)))
			s.WriteString("\n")
		}
	}
}
