package components

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// spinnerFrameRate is the tick cadence. The visible glyph advances every
// fourth tick so the animation stays readable at this rate.
const spinnerFrameRate = 16 * time.Millisecond

// SpinnerTickMsg represents a spinner tick message
type SpinnerTickMsg time.Time

// Spinner represents a loading spinner component
type Spinner struct {
	frames  []string
	current int
	message string
	active  bool

	frameCounter int

	// Styles
	spinnerStyle lipgloss.Style
	messageStyle lipgloss.Style
}

// NewSpinner creates a new Spinner component
func NewSpinner(message string) *Spinner {
	return &Spinner{
		frames: []string{
			"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏",
		},
		message: message,
		active:  true,

		spinnerStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorAccent)),

		messageStyle: lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorBright)).
			MarginLeft(1),
	}
}

// Update handles messages for the spinner component
func (s *Spinner) Update(msg tea.Msg) (*Spinner, tea.Cmd) {
	switch msg.(type) {
	case SpinnerTickMsg:
		if s.active {
			s.frameCounter++
			if s.frameCounter%4 == 0 {
				s.current = (s.current + 1) % len(s.frames)
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

// IsActive returns whether the spinner is active
func (s *Spinner) IsActive() bool {
	return s.active
}

// tick returns a command that will send a SpinnerTickMsg
func (s *Spinner) tick() tea.Cmd {
	return tea.Tick(spinnerFrameRate, func(t time.Time) tea.Msg {
		return SpinnerTickMsg(t)
	})
}
