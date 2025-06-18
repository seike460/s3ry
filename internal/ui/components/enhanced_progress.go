package components

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ProgressBarStyle represents different progress bar visual styles
type ProgressBarStyle int

const (
	ProgressStyleDefault ProgressBarStyle = iota
	ProgressStyleMinimal
	ProgressStyleDetailed
	ProgressStyleGraphical
	ProgressStyleRainbow
)

// ProgressVisualConfig represents visual configuration for progress bars
type ProgressVisualConfig struct {
	Style       ProgressBarStyle
	ShowSpeed   bool
	ShowETA     bool
	ShowPercent bool
	ShowBytes   bool
	Animated    bool
	Colors      ProgressColors
	Characters  ProgressCharacters
}

// ProgressColors represents color scheme for progress bars
type ProgressColors struct {
	Complete   lipgloss.Color
	Incomplete lipgloss.Color
	Background lipgloss.Color
	Text       lipgloss.Color
	Speed      lipgloss.Color
	ETA        lipgloss.Color
}

// ProgressCharacters represents characters used in progress bars
type ProgressCharacters struct {
	Complete   string
	Incomplete string
	Edges      [2]string // [left, right]
}

// EnhancedProgress represents an enhanced progress component with multiple visual styles
type EnhancedProgress struct {
	*Progress // Embed base progress
	config    ProgressVisualConfig
	animation animationState
}

// animationState represents animation state for progress bars
type animationState struct {
	frame     int
	direction int
	enabled   bool
	lastTick  time.Time
}

// NewEnhancedProgress creates a new enhanced progress component
func NewEnhancedProgress(title string, total int64, config ProgressVisualConfig) *EnhancedProgress {
	base := NewProgress(title, total)

	// Set default config if not provided
	if config.Colors.Complete == "" {
		config.Colors = getDefaultColors()
	}
	if config.Characters.Complete == "" {
		config.Characters = getDefaultCharacters(config.Style)
	}

	return &EnhancedProgress{
		Progress: base,
		config:   config,
		animation: animationState{
			enabled:  config.Animated,
			lastTick: time.Now(),
		},
	}
}

// getDefaultColors returns default color scheme
func getDefaultColors() ProgressColors {
	return ProgressColors{
		Complete:   lipgloss.Color("#50FA7B"),
		Incomplete: lipgloss.Color("#44475A"),
		Background: lipgloss.Color("#282A36"),
		Text:       lipgloss.Color("#F8F8F2"),
		Speed:      lipgloss.Color("#8BE9FD"),
		ETA:        lipgloss.Color("#FFB86C"),
	}
}

// getDefaultCharacters returns default characters for a style
func getDefaultCharacters(style ProgressBarStyle) ProgressCharacters {
	switch style {
	case ProgressStyleMinimal:
		return ProgressCharacters{
			Complete:   "━",
			Incomplete: "╌",
			Edges:      [2]string{"", ""},
		}
	case ProgressStyleDetailed:
		return ProgressCharacters{
			Complete:   "█",
			Incomplete: "░",
			Edges:      [2]string{"▐", "▌"},
		}
	case ProgressStyleGraphical:
		return ProgressCharacters{
			Complete:   "▓",
			Incomplete: "▒",
			Edges:      [2]string{"▐", "▌"},
		}
	case ProgressStyleRainbow:
		return ProgressCharacters{
			Complete:   "█",
			Incomplete: "░",
			Edges:      [2]string{"▐", "▌"},
		}
	default: // ProgressStyleDefault
		return ProgressCharacters{
			Complete:   "█",
			Incomplete: "─",
			Edges:      [2]string{"│", "│"},
		}
	}
}

// Update updates the enhanced progress component
func (ep *EnhancedProgress) Update(msg tea.Msg) (*EnhancedProgress, tea.Cmd) {
	var cmd tea.Cmd
	ep.Progress, cmd = ep.Progress.Update(msg)

	// Handle animation updates
	if ep.animation.enabled {
		now := time.Now()
		if now.Sub(ep.animation.lastTick) > 100*time.Millisecond {
			ep.animation.frame++
			ep.animation.lastTick = now

			// Animate based on style
			if ep.config.Style == ProgressStyleRainbow {
				ep.animation.frame %= 6 // 6 colors in rainbow
			} else {
				ep.animation.frame %= 4 // 4 frame animation
			}
		}
	}

	return ep, cmd
}

// View renders the enhanced progress component
func (ep *EnhancedProgress) View() string {
	if ep.completed {
		return ep.renderCompleted()
	}

	switch ep.config.Style {
	case ProgressStyleMinimal:
		return ep.renderMinimal()
	case ProgressStyleDetailed:
		return ep.renderDetailed()
	case ProgressStyleGraphical:
		return ep.renderGraphical()
	case ProgressStyleRainbow:
		return ep.renderRainbow()
	default:
		return ep.renderDefault()
	}
}

// renderDefault renders the default progress style
func (ep *EnhancedProgress) renderDefault() string {
	var output strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ep.config.Colors.Text)
	output.WriteString(titleStyle.Render(ep.title))
	output.WriteString("\n")

	// Progress bar
	bar := ep.renderProgressBar()
	output.WriteString(bar)
	output.WriteString("\n")

	// Details
	if ep.config.ShowPercent || ep.config.ShowBytes || ep.config.ShowSpeed || ep.config.ShowETA {
		details := ep.renderDetails()
		output.WriteString(details)
		output.WriteString("\n")
	}

	// Message
	if ep.message != "" {
		messageStyle := lipgloss.NewStyle().
			Foreground(ep.config.Colors.Text).
			Italic(true)
		output.WriteString(messageStyle.Render(ep.message))
	}

	return output.String()
}

// renderMinimal renders a minimal progress style
func (ep *EnhancedProgress) renderMinimal() string {
	percent := float64(ep.current) / float64(ep.total) * 100
	bar := ep.renderProgressBar()

	return fmt.Sprintf("%s %.1f%%", bar, percent)
}

// renderDetailed renders a detailed progress style
func (ep *EnhancedProgress) renderDetailed() string {
	var output strings.Builder

	// Title with progress percentage
	percent := float64(ep.current) / float64(ep.total) * 100
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ep.config.Colors.Text)

	title := fmt.Sprintf("%s (%.1f%%)", ep.title, percent)
	output.WriteString(titleStyle.Render(title))
	output.WriteString("\n")

	// Progress bar
	output.WriteString(ep.renderProgressBar())
	output.WriteString("\n")

	// Detailed stats
	stats := ep.renderDetailedStats()
	output.WriteString(stats)

	if ep.message != "" {
		output.WriteString("\n")
		messageStyle := lipgloss.NewStyle().
			Foreground(ep.config.Colors.Text).
			Italic(true)
		output.WriteString(messageStyle.Render("📝 " + ep.message))
	}

	return output.String()
}

// renderGraphical renders a graphical progress style
func (ep *EnhancedProgress) renderGraphical() string {
	var output strings.Builder

	// ASCII art style title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ep.config.Colors.Complete).
		Background(ep.config.Colors.Background).
		Padding(0, 1)

	output.WriteString(titleStyle.Render("▓▒░ " + ep.title + " ░▒▓"))
	output.WriteString("\n")

	// Graphical progress bar with decorations
	bar := ep.renderGraphicalBar()
	output.WriteString(bar)
	output.WriteString("\n")

	// Graphical stats
	stats := ep.renderGraphicalStats()
	output.WriteString(stats)

	return output.String()
}

// renderRainbow renders a rainbow progress style
func (ep *EnhancedProgress) renderRainbow() string {
	var output strings.Builder

	// Rainbow title
	title := ep.renderRainbowText(ep.title)
	output.WriteString(title)
	output.WriteString("\n")

	// Rainbow progress bar
	bar := ep.renderRainbowBar()
	output.WriteString(bar)
	output.WriteString("\n")

	// Rainbow stats
	if ep.config.ShowSpeed || ep.config.ShowETA {
		stats := ep.renderRainbowStats()
		output.WriteString(stats)
	}

	return output.String()
}

// renderProgressBar renders the basic progress bar
func (ep *EnhancedProgress) renderProgressBar() string {
	if ep.width == 0 {
		ep.width = 50 // Default width
	}

	barWidth := ep.width - 2 // Account for edges
	if barWidth < 1 {
		barWidth = 1
	}

	filled := int(float64(barWidth) * (float64(ep.current) / float64(ep.total)))
	if filled > barWidth {
		filled = barWidth
	}

	completeStyle := lipgloss.NewStyle().Foreground(ep.config.Colors.Complete)
	incompleteStyle := lipgloss.NewStyle().Foreground(ep.config.Colors.Incomplete)

	var bar strings.Builder
	bar.WriteString(ep.config.Characters.Edges[0])

	for i := 0; i < filled; i++ {
		bar.WriteString(completeStyle.Render(ep.config.Characters.Complete))
	}

	for i := filled; i < barWidth; i++ {
		bar.WriteString(incompleteStyle.Render(ep.config.Characters.Incomplete))
	}

	bar.WriteString(ep.config.Characters.Edges[1])

	return bar.String()
}

// renderDetails renders progress details
func (ep *EnhancedProgress) renderDetails() string {
	var parts []string

	if ep.config.ShowPercent {
		percent := float64(ep.current) / float64(ep.total) * 100
		parts = append(parts, fmt.Sprintf("%.1f%%", percent))
	}

	if ep.config.ShowBytes {
		parts = append(parts, fmt.Sprintf("%s/%s",
			formatBytesProgress(ep.current),
			formatBytesProgress(ep.total)))
	}

	if ep.config.ShowSpeed && ep.avgSpeed > 0 {
		speedStyle := lipgloss.NewStyle().Foreground(ep.config.Colors.Speed)
		parts = append(parts, speedStyle.Render(fmt.Sprintf("%.1f MB/s", ep.avgSpeed/1024/1024)))
	}

	if ep.config.ShowETA && ep.avgSpeed > 0 {
		remaining := ep.total - ep.current
		eta := time.Duration(float64(remaining)/ep.avgSpeed) * time.Second
		etaStyle := lipgloss.NewStyle().Foreground(ep.config.Colors.ETA)
		parts = append(parts, etaStyle.Render("ETA: "+ep.Progress.formatDuration(eta)))
	}

	return strings.Join(parts, " • ")
}

// renderDetailedStats renders detailed statistics
func (ep *EnhancedProgress) renderDetailedStats() string {
	var output strings.Builder

	// Progress stats
	elapsed := time.Since(ep.startTime)

	statsStyle := lipgloss.NewStyle().
		Foreground(ep.config.Colors.Text).
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1)

	stats := fmt.Sprintf(`📊 Progress Statistics
┌─────────────────────────────────┐
│ Current:  %-20s │
│ Total:    %-20s │
│ Speed:    %-20s │
│ Elapsed:  %-20s │
│ ETA:      %-20s │
└─────────────────────────────────┘`,
		formatBytesProgress(ep.current),
		formatBytesProgress(ep.total),
		fmt.Sprintf("%.1f MB/s", ep.avgSpeed/1024/1024),
		ep.Progress.formatDuration(elapsed),
		func() string {
			if ep.avgSpeed > 0 {
				remaining := ep.total - ep.current
				eta := time.Duration(float64(remaining)/ep.avgSpeed) * time.Second
				return ep.Progress.formatDuration(eta)
			}
			return "calculating..."
		}())

	output.WriteString(statsStyle.Render(stats))

	return output.String()
}

// renderGraphicalBar renders a graphical progress bar
func (ep *EnhancedProgress) renderGraphicalBar() string {
	barWidth := 60
	filled := int(float64(barWidth) * (float64(ep.current) / float64(ep.total)))

	var bar strings.Builder
	bar.WriteString("▓▒░ ")

	// Use gradient colors
	for i := 0; i < filled; i++ {
		intensity := float64(i) / float64(barWidth)
		color := ep.getGradientColor(intensity)
		style := lipgloss.NewStyle().Foreground(color)
		bar.WriteString(style.Render("█"))
	}

	for i := filled; i < barWidth; i++ {
		bar.WriteString(lipgloss.NewStyle().Foreground(ep.config.Colors.Incomplete).Render("░"))
	}

	bar.WriteString(" ░▒▓")

	return bar.String()
}

// renderGraphicalStats renders graphical statistics
func (ep *EnhancedProgress) renderGraphicalStats() string {
	percent := float64(ep.current) / float64(ep.total) * 100

	// Create visual indicators
	var indicators strings.Builder
	indicators.WriteString("📈 ")

	// Speed indicator
	if ep.avgSpeed > 0 {
		speedMBs := ep.avgSpeed / 1024 / 1024
		if speedMBs > 10 {
			indicators.WriteString("🚀")
		} else if speedMBs > 5 {
			indicators.WriteString("⚡")
		} else {
			indicators.WriteString("🐌")
		}
		indicators.WriteString(fmt.Sprintf(" %.1f MB/s", speedMBs))
	}

	indicators.WriteString(fmt.Sprintf(" • %.1f%% complete", percent))

	return indicators.String()
}

// renderRainbowText renders text with rainbow colors
func (ep *EnhancedProgress) renderRainbowText(text string) string {
	colors := []lipgloss.Color{"#FF0000", "#FF7F00", "#FFFF00", "#00FF00", "#0000FF", "#4B0082"}

	var output strings.Builder
	for i, char := range text {
		colorIndex := (i + ep.animation.frame) % len(colors)
		style := lipgloss.NewStyle().Foreground(colors[colorIndex])
		output.WriteString(style.Render(string(char)))
	}

	return output.String()
}

// renderRainbowBar renders a rainbow progress bar
func (ep *EnhancedProgress) renderRainbowBar() string {
	colors := []lipgloss.Color{"#FF0000", "#FF7F00", "#FFFF00", "#00FF00", "#0000FF", "#4B0082"}
	barWidth := 60
	filled := int(float64(barWidth) * (float64(ep.current) / float64(ep.total)))

	var bar strings.Builder
	bar.WriteString("🌈 ")

	for i := 0; i < filled; i++ {
		colorIndex := (i + ep.animation.frame) % len(colors)
		style := lipgloss.NewStyle().Foreground(colors[colorIndex])
		bar.WriteString(style.Render("█"))
	}

	for i := filled; i < barWidth; i++ {
		bar.WriteString(lipgloss.NewStyle().Foreground(ep.config.Colors.Incomplete).Render("░"))
	}

	bar.WriteString(" 🌈")

	return bar.String()
}

// renderRainbowStats renders rainbow statistics
func (ep *EnhancedProgress) renderRainbowStats() string {
	var parts []string

	if ep.config.ShowSpeed && ep.avgSpeed > 0 {
		speed := fmt.Sprintf("🚀 %.1f MB/s", ep.avgSpeed/1024/1024)
		parts = append(parts, ep.renderRainbowText(speed))
	}

	if ep.config.ShowETA && ep.avgSpeed > 0 {
		remaining := ep.total - ep.current
		eta := time.Duration(float64(remaining)/ep.avgSpeed) * time.Second
		etaText := "⏱️ ETA: " + ep.Progress.formatDuration(eta)
		parts = append(parts, ep.renderRainbowText(etaText))
	}

	return strings.Join(parts, " • ")
}

// renderCompleted renders the completion message
func (ep *EnhancedProgress) renderCompleted() string {
	var output strings.Builder

	if ep.success {
		successStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#50FA7B")).
			Background(lipgloss.Color("#282A36")).
			Padding(0, 1)

		output.WriteString(successStyle.Render("✅ " + ep.title + " - COMPLETED"))
	} else {
		errorStyle := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF5555")).
			Background(lipgloss.Color("#282A36")).
			Padding(0, 1)

		output.WriteString(errorStyle.Render("❌ " + ep.title + " - FAILED"))
	}

	if ep.message != "" {
		output.WriteString("\n")
		messageStyle := lipgloss.NewStyle().
			Foreground(ep.config.Colors.Text)
		output.WriteString(messageStyle.Render(ep.message))
	}

	return output.String()
}

// getGradientColor returns a gradient color based on intensity
func (ep *EnhancedProgress) getGradientColor(intensity float64) lipgloss.Color {
	// Create a gradient from red to green
	if intensity < 0.5 {
		// Red to yellow
		red := 255
		green := int(255 * intensity * 2)
		return lipgloss.Color(fmt.Sprintf("#%02X%02X00", red, green))
	} else {
		// Yellow to green
		red := int(255 * (1 - intensity) * 2)
		green := 255
		return lipgloss.Color(fmt.Sprintf("#%02X%02X00", red, green))
	}
}

// formatBytesProgress formats bytes for progress display
func formatBytesProgress(bytes int64) string {
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
