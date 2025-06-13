package components

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SpinnerTickMsg represents a spinner tick message
type SpinnerTickMsg time.Time

// Spinner represents a loading spinner component with 60fps optimization
type Spinner struct {
	frames   []string
	current  int
	message  string
	active   bool
	
	// Performance optimization for 60fps
	frameRate      time.Duration
	lastUpdate     time.Time
	skipFrames     int
	frameCounter   int
	targetFPS      int
	
	// Styles
	spinnerStyle lipgloss.Style
	messageStyle lipgloss.Style
}

// NewSpinner creates a new Spinner component optimized for 60fps
func NewSpinner(message string) *Spinner {
	return &Spinner{
		frames: []string{
			"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏",
		},
		message: message,
		active:  true,
		
		// 60fps optimization settings
		targetFPS:   60,
		frameRate:   time.Millisecond * 16, // ~60fps (16.67ms per frame)
		lastUpdate:  time.Now(),
		skipFrames:  0,
		frameCounter: 0,
		
		spinnerStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#7D56F4")),
		
		messageStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFF")).
			MarginLeft(1),
	}
}

// NewDotSpinner creates a spinner with dot animation optimized for 60fps
func NewDotSpinner(message string) *Spinner {
	return &Spinner{
		frames: []string{
			"   ", ".  ", ".. ", "...", " ..", "  .", "   ",
		},
		message: message,
		active:  true,
		
		// 60fps optimization settings
		targetFPS:   60,
		frameRate:   time.Millisecond * 16, // ~60fps
		lastUpdate:  time.Now(),
		skipFrames:  0,
		frameCounter: 0,
		
		spinnerStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#04B575")),
		
		messageStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFF")),
	}
}

// Update handles messages for the spinner component with 60fps optimization
func (s *Spinner) Update(msg tea.Msg) (*Spinner, tea.Cmd) {
	switch msg.(type) {
	case SpinnerTickMsg:
		if s.active {
			now := time.Now()
			
			// Frame rate control to maintain smooth 60fps
			if now.Sub(s.lastUpdate) >= s.frameRate {
				// Update frame only when enough time has passed
				s.frameCounter++
				
				// Update spinner frame (slower animation for visual appeal)
				if s.frameCounter%4 == 0 { // Update spinner every 4 frames
					s.current = (s.current + 1) % len(s.frames)
				}
				
				s.lastUpdate = now
			}
			
			return s, s.tick()
		}
	}
	
	return s, nil
}

// View renders the spinner component
func (s *Spinner) View() string {
	if !s.active {
		return ""
	}
	
	frame := s.spinnerStyle.Render(s.frames[s.current])
	message := s.messageStyle.Render(s.message)
	
	return frame + message
}

// Start starts the spinner animation
func (s *Spinner) Start() tea.Cmd {
	s.active = true
	return s.tick()
}

// Stop stops the spinner animation
func (s *Spinner) Stop() {
	s.active = false
}

// SetMessage updates the spinner message
func (s *Spinner) SetMessage(message string) {
	s.message = message
}

// IsActive returns whether the spinner is active
func (s *Spinner) IsActive() bool {
	return s.active
}

// tick returns a command that will send a SpinnerTickMsg optimized for 60fps
func (s *Spinner) tick() tea.Cmd {
	return tea.Tick(s.frameRate, func(t time.Time) tea.Msg {
		return SpinnerTickMsg(t)
	})
}

// SetFrameRate adjusts the target frame rate for performance tuning
func (s *Spinner) SetFrameRate(fps int) {
	s.targetFPS = fps
	s.frameRate = time.Duration(1000/fps) * time.Millisecond
}

// GetFrameRate returns the current target frame rate
func (s *Spinner) GetFrameRate() int {
	return s.targetFPS
}

// GetPerformanceInfo returns performance statistics
func (s *Spinner) GetPerformanceInfo() map[string]interface{} {
	return map[string]interface{}{
		"target_fps":     s.targetFPS,
		"frame_rate_ms":  s.frameRate.Milliseconds(),
		"frame_counter":  s.frameCounter,
		"active":         s.active,
		"current_frame":  s.current,
		"total_frames":   len(s.frames),
	}
}