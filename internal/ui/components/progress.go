package components

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// speedSampleWindow is how many recent samples feed the average speed.
const speedSampleWindow = 10

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
	title      string
	current    int64
	total      int64
	message    string
	completed  bool
	success    bool
	width      int
	startTime  time.Time
	lastUpdate time.Time
	speed      float64
	avgSpeed   float64
	samples    []speedSample
	maxSamples int

	// Styles
	titleStyle    lipgloss.Style
	progressStyle lipgloss.Style
	completeStyle lipgloss.Style
	errorStyle    lipgloss.Style
	messageStyle  lipgloss.Style
	speedStyle    lipgloss.Style
}

// NewProgress creates a new Progress component with enhanced real-time tracking
func NewProgress(title string, total int64) *Progress {
	now := time.Now()
	return &Progress{
		title:      title,
		total:      total,
		startTime:  now,
		lastUpdate: now,
		maxSamples: speedSampleWindow,
		samples:    make([]speedSample, 0, speedSampleWindow),

		titleStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorAccent)).
			MarginBottom(1),

		progressStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorSuccess)),

		completeStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorSuccess)),

		errorStyle: lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorDanger)),

		messageStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorMuted)).
			MarginTop(1),

		speedStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorWarning)).
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
