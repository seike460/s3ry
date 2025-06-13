package components

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ProgressMsg represents a progress update message
type ProgressMsg struct {
	Current int64
	Total   int64
	Message string
}

// CompletedMsg represents a completion message
type CompletedMsg struct {
	Success bool
	Message string
}

// Progress represents a progress bar component with real-time updates
type Progress struct {
	title       string
	current     int64
	total       int64
	message     string
	completed   bool
	success     bool
	width       int
	startTime   time.Time
	lastUpdate  time.Time
	speed       float64
	avgSpeed    float64
	samples     []speedSample
	maxSamples  int
	
	// Styles
	titleStyle      lipgloss.Style
	progressStyle   lipgloss.Style
	completeStyle   lipgloss.Style
	errorStyle      lipgloss.Style
	messageStyle    lipgloss.Style
	speedStyle      lipgloss.Style
}

// speedSample represents a speed measurement sample
type speedSample struct {
	timestamp time.Time
	bytes     int64
}

// NewProgress creates a new Progress component with enhanced real-time tracking
func NewProgress(title string, total int64) *Progress {
	now := time.Now()
	return &Progress{
		title:      title,
		total:      total,
		startTime:  now,
		lastUpdate: now,
		maxSamples: 10, // Keep last 10 samples for average speed calculation
		samples:    make([]speedSample, 0, 10),
		
		titleStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#7D56F4")).
			MarginBottom(1),
		
		progressStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#04B575")),
		
		completeStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#04B575")),
		
		errorStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF5555")),
		
		messageStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#888")).
			MarginTop(1),
		
		speedStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFA500")).
			Bold(true),
	}
}

// Update handles messages for the progress component
func (p *Progress) Update(msg tea.Msg) (*Progress, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.width = msg.Width
		
	case ProgressMsg:
		now := time.Now()
		
		// Calculate instantaneous speed
		if p.current > 0 && !p.lastUpdate.IsZero() {
			deltaTime := now.Sub(p.lastUpdate).Seconds()
			deltaBytes := msg.Current - p.current
			if deltaTime > 0 && deltaBytes > 0 {
				p.speed = float64(deltaBytes) / deltaTime
				
				// Add sample for average speed calculation
				p.addSpeedSample(now, msg.Current)
				p.calculateAverageSpeed()
			}
		}
		
		p.current = msg.Current
		p.total = msg.Total
		p.message = msg.Message
		p.lastUpdate = now
		
	case CompletedMsg:
		p.completed = true
		p.success = msg.Success
		p.message = msg.Message
	}
	
	return p, nil
}

// View renders the progress component
func (p *Progress) View() string {
	var s strings.Builder
	
	// Title
	s.WriteString(p.titleStyle.Render(p.title))
	s.WriteString("\n\n")
	
	if p.completed {
		// Show completion status
		if p.success {
			s.WriteString(p.completeStyle.Render("✓ " + p.message))
		} else {
			s.WriteString(p.errorStyle.Render("✗ " + p.message))
		}
	} else {
		// Show progress bar
		barWidth := 40
		if p.width > 0 && p.width < 60 {
			barWidth = p.width - 20
		}
		
		var percentage float64
		if p.total > 0 {
			percentage = float64(p.current) / float64(p.total)
		}
		
		filled := int(percentage * float64(barWidth))
		bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
		
		s.WriteString(p.progressStyle.Render(fmt.Sprintf("[%s] %.1f%%", bar, percentage*100)))
		
		// Show size information if available
		if p.total > 0 {
			s.WriteString(fmt.Sprintf(" (%s / %s)", formatBytes(p.current), formatBytes(p.total)))
		}
		
		// Show enhanced speed and ETA information
		elapsed := time.Since(p.startTime)
		if elapsed > time.Second && p.current > 0 {
			// Use average speed for more stable display
			displaySpeed := p.avgSpeed
			if displaySpeed == 0 {
				displaySpeed = float64(p.current) / elapsed.Seconds()
			}
			
			s.WriteString(" | ")
			s.WriteString(p.speedStyle.Render(fmt.Sprintf("%s/s", formatBytes(int64(displaySpeed)))))
			
			// Show instantaneous speed if significantly different
			if p.speed > 0 && p.speed != displaySpeed {
				instantDiff := (p.speed - displaySpeed) / displaySpeed
				if instantDiff > 0.2 || instantDiff < -0.2 { // Show if >20% difference
					s.WriteString(fmt.Sprintf(" (now: %s/s)", formatBytes(int64(p.speed))))
				}
			}
			
			// Enhanced ETA calculation
			if p.total > 0 && displaySpeed > 0 {
				remaining := float64(p.total-p.current) / displaySpeed
				eta := time.Duration(remaining) * time.Second
				
				// Format ETA nicely
				if eta > time.Hour {
					s.WriteString(fmt.Sprintf(" | ETA: %dh%dm", int(eta.Hours()), int(eta.Minutes())%60))
				} else if eta > time.Minute {
					s.WriteString(fmt.Sprintf(" | ETA: %dm%ds", int(eta.Minutes()), int(eta.Seconds())%60))
				} else {
					s.WriteString(fmt.Sprintf(" | ETA: %ds", int(eta.Seconds())))
				}
			}
			
			// Show elapsed time
			if elapsed > time.Minute {
				s.WriteString(fmt.Sprintf(" | Elapsed: %dm%ds", int(elapsed.Minutes()), int(elapsed.Seconds())%60))
			} else {
				s.WriteString(fmt.Sprintf(" | Elapsed: %ds", int(elapsed.Seconds())))
			}
		}
	}
	
	// Show message if available
	if p.message != "" && !p.completed {
		s.WriteString("\n")
		s.WriteString(p.messageStyle.Render(p.message))
	}
	
	return s.String()
}

// SetProgress updates the progress with real-time speed calculation
func (p *Progress) SetProgress(current, total int64, message string) {
	now := time.Now()
	
	// Calculate speed if we have previous data
	if p.current > 0 && !p.lastUpdate.IsZero() {
		deltaTime := now.Sub(p.lastUpdate).Seconds()
		deltaBytes := current - p.current
		if deltaTime > 0 && deltaBytes > 0 {
			p.speed = float64(deltaBytes) / deltaTime
			p.addSpeedSample(now, current)
			p.calculateAverageSpeed()
		}
	}
	
	p.current = current
	p.total = total
	p.message = message
	p.lastUpdate = now
}

// Complete marks the progress as completed
func (p *Progress) Complete(success bool, message string) {
	p.completed = true
	p.success = success
	p.message = message
}

// IsCompleted returns whether the progress is completed
func (p *Progress) IsCompleted() bool {
	return p.completed
}

// IsSuccess returns whether the completed operation was successful
func (p *Progress) IsSuccess() bool {
	return p.success
}

// addSpeedSample adds a new speed measurement sample
func (p *Progress) addSpeedSample(timestamp time.Time, bytes int64) {
	sample := speedSample{
		timestamp: timestamp,
		bytes:     bytes,
	}
	
	// Add sample and maintain max size
	p.samples = append(p.samples, sample)
	if len(p.samples) > p.maxSamples {
		p.samples = p.samples[1:]
	}
}

// calculateAverageSpeed calculates the average speed from recent samples
func (p *Progress) calculateAverageSpeed() {
	if len(p.samples) < 2 {
		return
	}
	
	// Calculate average speed over the sample period
	first := p.samples[0]
	last := p.samples[len(p.samples)-1]
	
	deltaTime := last.timestamp.Sub(first.timestamp).Seconds()
	deltaBytes := last.bytes - first.bytes
	
	if deltaTime > 0 && deltaBytes > 0 {
		p.avgSpeed = float64(deltaBytes) / deltaTime
	}
}

// GetCurrentSpeed returns the current instantaneous speed
func (p *Progress) GetCurrentSpeed() float64 {
	return p.speed
}

// GetAverageSpeed returns the average speed over recent samples
func (p *Progress) GetAverageSpeed() float64 {
	return p.avgSpeed
}

// formatDuration formats a duration for display
func (p *Progress) formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	} else if d < time.Hour {
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	} else {
		return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
	}
}

// formatBytes formats byte count as human readable string
func formatBytes(bytes int64) string {
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