package components

import (
	"fmt"
	"runtime"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// PerformanceMetrics represents real-time performance metrics
type PerformanceMetrics struct {
	FrameRate       float64
	MemoryUsage     uint64
	GoroutineCount  int
	RenderTime      time.Duration
	UpdateTime      time.Duration
	CacheHitRatio   float64
	VirtualScrolling bool
	ItemsVisible    int
	ItemsTotal      int
	Timestamp       time.Time
}

// PerformanceUpdateMsg represents a performance metric update
type PerformanceUpdateMsg PerformanceMetrics

// PerformanceMonitor represents a real-time performance monitoring component
type PerformanceMonitor struct {
	enabled       bool
	metrics       PerformanceMetrics
	history       []PerformanceMetrics
	maxHistory    int
	updateInterval time.Duration
	lastUpdate    time.Time
	frameCounter  int
	
	// Target performance thresholds
	targetFPS     float64
	maxMemoryMB   uint64
	maxGoroutines int
	
	// Display options
	showDetailed  bool
	showHistory   bool
	showWarnings  bool
	
	// Styles
	headerStyle   lipgloss.Style
	metricStyle   lipgloss.Style
	goodStyle     lipgloss.Style
	warningStyle  lipgloss.Style
	criticalStyle lipgloss.Style
	historyStyle  lipgloss.Style
}

// NewPerformanceMonitor creates a new performance monitoring component
func NewPerformanceMonitor() *PerformanceMonitor {
	return &PerformanceMonitor{
		enabled:        false, // Disabled by default for production
		maxHistory:     60,    // Keep 60 seconds of history
		updateInterval: time.Millisecond * 100, // Update every 100ms
		targetFPS:      60.0,
		maxMemoryMB:    100, // 100MB warning threshold
		maxGoroutines:  50,  // 50 goroutines warning threshold
		showDetailed:   false,
		showHistory:    false,
		showWarnings:   true,
		
		headerStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			MarginBottom(1),
		
		metricStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFF")),
		
		goodStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#04B575")),
		
		warningStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFA500")).
			Bold(true),
		
		criticalStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF5555")).
			Bold(true),
		
		historyStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888")).
			Faint(true),
	}
}

// Enable starts performance monitoring
func (p *PerformanceMonitor) Enable() {
	p.enabled = true
	p.lastUpdate = time.Now()
	p.frameCounter = 0
}

// Disable stops performance monitoring
func (p *PerformanceMonitor) Disable() {
	p.enabled = false
}

// Update handles messages and updates performance metrics
func (p *PerformanceMonitor) Update(msg tea.Msg) (*PerformanceMonitor, tea.Cmd) {
	if !p.enabled {
		return p, nil
	}
	
	now := time.Now()
	
	// Update metrics periodically
	if now.Sub(p.lastUpdate) >= p.updateInterval {
		p.updateMetrics(now)
		p.lastUpdate = now
	}
	
	switch msg := msg.(type) {
	case PerformanceUpdateMsg:
		p.metrics = PerformanceMetrics(msg)
		p.addToHistory(p.metrics)
	}
	
	return p, p.tick()
}

// View renders the performance monitor
func (p *PerformanceMonitor) View() string {
	if !p.enabled {
		return ""
	}
	
	var s strings.Builder
	
	// Header
	s.WriteString(p.headerStyle.Render("📊 Performance Monitor"))
	s.WriteString("\n")
	
	// Current metrics
	s.WriteString(p.renderCurrentMetrics())
	
	// Warnings if enabled
	if p.showWarnings {
		warnings := p.generateWarnings()
		if warnings != "" {
			s.WriteString("\n")
			s.WriteString(warnings)
		}
	}
	
	// Detailed metrics if enabled
	if p.showDetailed {
		s.WriteString("\n")
		s.WriteString(p.renderDetailedMetrics())
	}
	
	// History if enabled
	if p.showHistory && len(p.history) > 0 {
		s.WriteString("\n")
		s.WriteString(p.renderHistory())
	}
	
	return s.String()
}

// updateMetrics collects current performance metrics
func (p *PerformanceMonitor) updateMetrics(now time.Time) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	p.frameCounter++
	
	// Calculate frame rate
	elapsed := now.Sub(p.lastUpdate).Seconds()
	if elapsed > 0 {
		p.metrics.FrameRate = 1.0 / elapsed
	}
	
	p.metrics.MemoryUsage = m.Alloc
	p.metrics.GoroutineCount = runtime.NumGoroutine()
	p.metrics.Timestamp = now
	
	// Add to history
	p.addToHistory(p.metrics)
}

// addToHistory adds metrics to the history buffer
func (p *PerformanceMonitor) addToHistory(metrics PerformanceMetrics) {
	p.history = append(p.history, metrics)
	
	// Maintain history size
	if len(p.history) > p.maxHistory {
		p.history = p.history[1:]
	}
}

// renderCurrentMetrics renders the current performance metrics
func (p *PerformanceMonitor) renderCurrentMetrics() string {
	var s strings.Builder
	
	// Frame rate
	fps := p.metrics.FrameRate
	fpsStyle := p.goodStyle
	if fps < p.targetFPS*0.8 {
		fpsStyle = p.warningStyle
	}
	if fps < p.targetFPS*0.5 {
		fpsStyle = p.criticalStyle
	}
	
	s.WriteString(fmt.Sprintf("FPS: %s", fpsStyle.Render(fmt.Sprintf("%.1f", fps))))
	
	// Memory usage
	memMB := float64(p.metrics.MemoryUsage) / 1024 / 1024
	memStyle := p.goodStyle
	if memMB > float64(p.maxMemoryMB)*0.8 {
		memStyle = p.warningStyle
	}
	if memMB > float64(p.maxMemoryMB) {
		memStyle = p.criticalStyle
	}
	
	s.WriteString(fmt.Sprintf(" | Memory: %s", memStyle.Render(fmt.Sprintf("%.1f MB", memMB))))
	
	// Goroutines
	goroutineStyle := p.goodStyle
	if p.metrics.GoroutineCount > p.maxGoroutines {
		goroutineStyle = p.warningStyle
	}
	if p.metrics.GoroutineCount > p.maxGoroutines*2 {
		goroutineStyle = p.criticalStyle
	}
	
	s.WriteString(fmt.Sprintf(" | Goroutines: %s", goroutineStyle.Render(fmt.Sprintf("%d", p.metrics.GoroutineCount))))
	
	// Virtual scrolling status
	if p.metrics.VirtualScrolling {
		s.WriteString(fmt.Sprintf(" | Items: %s", p.goodStyle.Render(fmt.Sprintf("%d/%d", p.metrics.ItemsVisible, p.metrics.ItemsTotal))))
	}
	
	return s.String()
}

// renderDetailedMetrics renders detailed performance information
func (p *PerformanceMonitor) renderDetailedMetrics() string {
	var s strings.Builder
	
	s.WriteString(p.metricStyle.Render("Detailed Metrics:"))
	s.WriteString("\n")
	
	if p.metrics.RenderTime > 0 {
		s.WriteString(fmt.Sprintf("  Render Time: %v", p.metrics.RenderTime))
		s.WriteString("\n")
	}
	
	if p.metrics.UpdateTime > 0 {
		s.WriteString(fmt.Sprintf("  Update Time: %v", p.metrics.UpdateTime))
		s.WriteString("\n")
	}
	
	if p.metrics.CacheHitRatio > 0 {
		s.WriteString(fmt.Sprintf("  Cache Hit Ratio: %.1f%%", p.metrics.CacheHitRatio))
		s.WriteString("\n")
	}
	
	return s.String()
}

// renderHistory renders a simple performance history
func (p *PerformanceMonitor) renderHistory() string {
	var s strings.Builder
	
	s.WriteString(p.historyStyle.Render("Performance History (last 10s):"))
	s.WriteString("\n")
	
	// Show last 10 entries
	start := len(p.history) - 10
	if start < 0 {
		start = 0
	}
	
	for i := start; i < len(p.history); i++ {
		metrics := p.history[i]
		memMB := float64(metrics.MemoryUsage) / 1024 / 1024
		s.WriteString(p.historyStyle.Render(fmt.Sprintf("  %s: %.0f FPS, %.1f MB, %d goroutines",
			metrics.Timestamp.Format("15:04:05"),
			metrics.FrameRate,
			memMB,
			metrics.GoroutineCount)))
		s.WriteString("\n")
	}
	
	return s.String()
}

// generateWarnings generates performance warnings
func (p *PerformanceMonitor) generateWarnings() string {
	var warnings []string
	
	// FPS warnings
	if p.metrics.FrameRate < p.targetFPS*0.5 {
		warnings = append(warnings, "🚨 Critical: Very low frame rate detected")
	} else if p.metrics.FrameRate < p.targetFPS*0.8 {
		warnings = append(warnings, "⚠️ Warning: Frame rate below target")
	}
	
	// Memory warnings
	memMB := float64(p.metrics.MemoryUsage) / 1024 / 1024
	if memMB > float64(p.maxMemoryMB) {
		warnings = append(warnings, "🚨 Critical: High memory usage detected")
	} else if memMB > float64(p.maxMemoryMB)*0.8 {
		warnings = append(warnings, "⚠️ Warning: Memory usage approaching limit")
	}
	
	// Goroutine warnings
	if p.metrics.GoroutineCount > p.maxGoroutines*2 {
		warnings = append(warnings, "🚨 Critical: Too many goroutines")
	} else if p.metrics.GoroutineCount > p.maxGoroutines {
		warnings = append(warnings, "⚠️ Warning: High goroutine count")
	}
	
	if len(warnings) == 0 {
		return ""
	}
	
	var s strings.Builder
	s.WriteString(p.warningStyle.Render("Performance Warnings:"))
	s.WriteString("\n")
	
	for _, warning := range warnings {
		s.WriteString(p.warningStyle.Render(warning))
		s.WriteString("\n")
	}
	
	return s.String()
}

// tick returns a command for the next update
func (p *PerformanceMonitor) tick() tea.Cmd {
	if !p.enabled {
		return nil
	}
	
	return tea.Tick(p.updateInterval, func(t time.Time) tea.Msg {
		return PerformanceUpdateMsg(p.metrics)
	})
}

// Configuration methods
func (p *PerformanceMonitor) SetShowDetailed(show bool) {
	p.showDetailed = show
}

func (p *PerformanceMonitor) SetShowHistory(show bool) {
	p.showHistory = show
}

func (p *PerformanceMonitor) SetShowWarnings(show bool) {
	p.showWarnings = show
}

func (p *PerformanceMonitor) SetTargets(fps float64, memoryMB uint64, goroutines int) {
	p.targetFPS = fps
	p.maxMemoryMB = memoryMB
	p.maxGoroutines = goroutines
}

// UpdateMetrics allows external components to update specific metrics
func (p *PerformanceMonitor) UpdateMetrics(renderTime, updateTime time.Duration, cacheHitRatio float64, virtualScrolling bool, itemsVisible, itemsTotal int) {
	p.metrics.RenderTime = renderTime
	p.metrics.UpdateTime = updateTime
	p.metrics.CacheHitRatio = cacheHitRatio
	p.metrics.VirtualScrolling = virtualScrolling
	p.metrics.ItemsVisible = itemsVisible
	p.metrics.ItemsTotal = itemsTotal
}

// GetCurrentMetrics returns the current performance metrics
func (p *PerformanceMonitor) GetCurrentMetrics() PerformanceMetrics {
	return p.metrics
}

// GetAverageMetrics returns average metrics over the history
func (p *PerformanceMonitor) GetAverageMetrics() PerformanceMetrics {
	if len(p.history) == 0 {
		return PerformanceMetrics{}
	}
	
	var avgFPS, avgMemory float64
	var avgGoroutines int
	
	for _, metrics := range p.history {
		avgFPS += metrics.FrameRate
		avgMemory += float64(metrics.MemoryUsage)
		avgGoroutines += metrics.GoroutineCount
	}
	
	count := float64(len(p.history))
	return PerformanceMetrics{
		FrameRate:      avgFPS / count,
		MemoryUsage:    uint64(avgMemory / count),
		GoroutineCount: int(float64(avgGoroutines) / count),
	}
}